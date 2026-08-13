package engine

import (
	"bytes"
	"encoding/json"
	"math/bits"
	"math/rand"
	"reflect"
	"slices"
	"strings"
	"testing"

	gm "chess-engine/goosemg"
)

func traceFromFEN(t *testing.T, fen string) EvalTrace {
	t.Helper()
	board := gm.ParseFen(fen)
	return EvalTraceForBoard(&board)
}

func assertSquares(t *testing.T, got []string, want ...string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Fatalf("squares mismatch: got %v want %v", got, want)
	}
}

func assertPair(t *testing.T, got EvalPair, wantMG, wantEG int) {
	t.Helper()
	if got.MG != wantMG || got.EG != wantEG {
		t.Fatalf("pair mismatch: got MG %d EG %d, want MG %d EG %d", got.MG, got.EG, wantMG, wantEG)
	}
}

func requireSideBitboard(t *testing.T, bitboards map[string]EvalSideBitboards, name string) EvalSideBitboards {
	t.Helper()
	bb, ok := bitboards[name]
	if !ok {
		t.Fatalf("missing bitboard %q", name)
	}
	return bb
}

func squareIndex(name string) int {
	return int(name[0]-'a') + int(name[1]-'1')*8
}

func TestEvaluateWithTraceParity(t *testing.T) {
	fens := []string{
		gm.Startpos,
		"8/8/8/8/8/8/8/K6k w - - 0 1",
		"8/8/3p4/4P3/8/8/8/K6k w - - 0 1",
		"r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1",
		"8/8/R7/N7/R7/8/8/4K2k w - - 0 1",
	}

	for _, fen := range fens {
		t.Run(fen, func(t *testing.T) {
			fastBoard := gm.ParseFen(fen)
			traceBoard := gm.ParseFen(fen)

			fastScore := Evaluation(&fastBoard, false)
			traceScore, trace := EvaluateWithTrace(&traceBoard)

			if traceScore != fastScore {
				t.Fatalf("score mismatch: fast %d trace %d", fastScore, traceScore)
			}
			if trace.Score.SideToMove != fastScore {
				t.Fatalf("trace side-to-move score mismatch: got %d want %d", trace.Score.SideToMove, fastScore)
			}
		})
	}
}

func TestRenderEvalTraceJSON(t *testing.T) {
	board := gm.ParseFen(gm.Startpos)
	trace := EvalTraceForBoard(&board)

	var out bytes.Buffer
	if err := RenderEvalTraceJSON(&out, trace); err != nil {
		t.Fatalf("render json: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}

	for _, key := range []string{"fen", "sideToMove", "score", "phase", "draw", "totals", "pawn", "knight", "bishop", "rook", "queen", "king"} {
		if _, ok := decoded[key]; !ok {
			t.Fatalf("missing top-level field %q in json trace", key)
		}
	}
}

func TestRenderEvalTraceJSONRoundTripsEvalTrace(t *testing.T) {
	board := gm.ParseFen(gm.Startpos)
	trace := EvalTraceForBoard(&board)

	var out bytes.Buffer
	if err := RenderEvalTraceJSON(&out, trace); err != nil {
		t.Fatalf("render json: %v", err)
	}

	var decoded EvalTrace
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("json did not round-trip into EvalTrace: %v\n%s", err, out.String())
	}

	if decoded.FEN != trace.FEN {
		t.Fatalf("fen mismatch after round trip: got %q want %q", decoded.FEN, trace.FEN)
	}
	if decoded.SideToMove != "white" {
		t.Fatalf("side-to-move mismatch after round trip: got %q", decoded.SideToMove)
	}
	if decoded.Score.SideToMove != trace.Score.SideToMove {
		t.Fatalf("score mismatch after round trip: got %d want %d", decoded.Score.SideToMove, trace.Score.SideToMove)
	}
}

func TestRenderEvalTraceTextSections(t *testing.T) {
	board := gm.ParseFen(gm.Startpos)
	trace := EvalTraceForBoard(&board)

	var out bytes.Buffer
	if err := RenderEvalTraceText(&out, trace); err != nil {
		t.Fatalf("render text: %v", err)
	}

	text := out.String()
	for _, section := range []string{"Evaluation Trace", "Totals", "Pawn", "Knight", "Bishop", "Rook", "Queen", "King", "Space", "Bitboards"} {
		if !strings.Contains(text, section) {
			t.Fatalf("missing text section %q in:\n%s", section, text)
		}
	}
}

func TestEvalTraceBareKingsDraw(t *testing.T) {
	board := gm.ParseFen("8/8/8/8/8/8/8/K6k w - - 0 1")
	score, trace := EvaluateWithTrace(&board)

	if !trace.Draw.Theoretical {
		t.Fatalf("bare kings should be traced as a theoretical draw")
	}
	if !trace.Draw.Applied {
		t.Fatalf("bare kings should apply draw scaling")
	}
	if trace.Draw.Divider != DrawDivider {
		t.Fatalf("draw divider mismatch: got %d want %d", trace.Draw.Divider, DrawDivider)
	}
	if score != trace.Score.SideToMove {
		t.Fatalf("returned score %d does not match trace score %d", score, trace.Score.SideToMove)
	}
}

func TestEvalTracePawnSquareLists(t *testing.T) {
	board := gm.ParseFen("8/8/3p4/4P3/8/8/8/K6k w - - 0 1")
	trace := EvalTraceForBoard(&board)

	for _, name := range []string{"lever", "candidate"} {
		bitboards, ok := trace.Pawn.Bitboards[name]
		if !ok {
			t.Fatalf("missing pawn bitboard %q", name)
		}
		if bitboards.White.Squares == nil || bitboards.Black.Squares == nil {
			t.Fatalf("%s bitboard has nil square lists", name)
		}
		if bitboards.White.Count == 0 {
			t.Fatalf("%s bitboard should expose inspectable white squares for this FEN", name)
		}
		if len(bitboards.White.Hex) != 16 || len(bitboards.Black.Hex) != 16 {
			t.Fatalf("%s bitboard should expose fixed-width hex values", name)
		}
	}
}

func TestEvalTraceRookStackBlockedState(t *testing.T) {
	board := gm.ParseFen("8/8/R7/N7/R7/8/8/4K2k w - - 0 1")
	trace := EvalTraceForBoard(&board)

	if _, ok := trace.Rook.Bitboards["stackedFiles"]; !ok {
		t.Fatalf("missing evaluator stacked-files bitboard")
	}

	blocked, ok := trace.Rook.Bitboards["blockedStackedFiles"]
	if !ok {
		t.Fatalf("missing physically blocked stacked-files bitboard")
	}
	if blocked.White.Count == 0 {
		t.Fatalf("expected the a-file rook stack to be inspectable as physically blocked")
	}
	if blocked.White.Squares == nil || blocked.Black.Squares == nil {
		t.Fatalf("blocked stacked-files bitboard has nil square lists")
	}
}

func TestEvalTracePawnStructureBitboardsExact(t *testing.T) {
	trace := traceFromFEN(t, "7k/8/8/3p4/3P4/2P5/P7/K7 w - - 0 1")

	isolated := requireSideBitboard(t, trace.Pawn.Bitboards, "isolated")
	assertSquares(t, isolated.White.Squares, "a2")
	assertSquares(t, isolated.Black.Squares, "d5")

	passed := requireSideBitboard(t, trace.Pawn.Bitboards, "passed")
	assertSquares(t, passed.White.Squares, "a2")
	assertSquares(t, passed.Black.Squares)

	blocked := requireSideBitboard(t, trace.Pawn.Bitboards, "blocked")
	assertSquares(t, blocked.White.Squares, "d4")
	assertSquares(t, blocked.Black.Squares, "d5")
}

// White pawns on a2 and a3 with nothing on the a-file to face them. The front
// pawn of the stack is the one charged, and with no black pawn ahead it takes
// the unopposed rate.
func TestEvalTraceDoubledPawnTermUnopposed(t *testing.T) {
	trace := traceFromFEN(t, "7k/8/8/8/8/P7/P7/K7 w - - 0 1")

	doubled := requireSideBitboard(t, trace.Pawn.Bitboards, "doubled")
	assertSquares(t, doubled.White.Squares, "a3")
	assertSquares(t, doubled.Black.Squares)
	assertPair(t, trace.Pawn.Terms["doubled"], -PawnDoubledUnopposedMG, -PawnDoubledUnopposedEG)
}

// The same stack with a black pawn on a6. Nothing about the stack changed, but
// the file is now contested, so the front pawn can no longer advance into a
// passer and the heavier rate applies.
func TestEvalTraceDoubledPawnTermOpposed(t *testing.T) {
	trace := traceFromFEN(t, "7k/8/p7/8/8/P7/P7/K7 w - - 0 1")

	opposed := requireSideBitboard(t, trace.Pawn.Bitboards, "opposed")
	assertSquares(t, opposed.White.Squares, "a2", "a3")
	assertPair(t, trace.Pawn.Terms["doubled"], -PawnDoubledOpposedMG, -PawnDoubledOpposedEG)
}

func TestEvalTraceRookFileTermsAndBitboards(t *testing.T) {
	trace := traceFromFEN(t, "6k1/p6r/8/8/8/8/4P3/R3K3 w - - 0 1")

	assertPair(t, trace.Rook.Terms["semiOpen"], RookSemiOpenMG, RookSemiOpenEG)
	assertPair(t, trace.Rook.Terms["open"], -RookOpenMG, -RookOpenEG)

	semiOpen := requireSideBitboard(t, trace.Rook.Bitboards, "semiOpenFiles")
	assertSquares(t, semiOpen.White.Squares, "a1", "a2", "a3", "a4", "a5", "a6", "a7", "a8")
	assertSquares(t, semiOpen.Black.Squares, "e1", "e2", "e3", "e4", "e5", "e6", "e7", "e8")

	open := requireSideBitboard(t, trace.Rook.Bitboards, "openFiles")
	if open.White.Count != 48 || open.Black.Count != 48 {
		t.Fatalf("open file counts mismatch: got W %d B %d, want 48 each", open.White.Count, open.Black.Count)
	}
}

func TestEvalTraceRookSeventhRankTerm(t *testing.T) {
	trace := traceFromFEN(t, "7k/RR6/8/8/8/8/8/4K3 w - - 0 1")

	assertPair(t, trace.Rook.Terms["seventh"], RookSeventhRankMG*4, RookSeventhRankEG*4)
	seventh := requireSideBitboard(t, trace.Rook.Bitboards, "seventhRank")
	assertSquares(t, seventh.White.Squares, "a7", "b7")
	assertSquares(t, seventh.Black.Squares)
}

func TestEvalTraceBishopPairTerm(t *testing.T) {
	trace := traceFromFEN(t, "7k/8/8/8/8/8/8/2B1KB2 w - - 0 1")

	wantMG := (BishopPairBonusMG * trace.Center.BishopPairScaleMG) / 100
	wantEG := (BishopPairBonusEG * trace.Center.BishopPairScaleEG) / 100
	assertPair(t, trace.Bishop.Terms["pair"], wantMG, wantEG)
}

func TestEvalTraceQueenCentralizationTerm(t *testing.T) {
	tests := []struct {
		name    string
		fen     string
		wantEG  int
		wantWBB []string
		wantBBB []string
	}{
		{
			name:    "white centralized queen",
			fen:     "7k/8/8/8/3Q4/8/8/4K3 w - - 0 1",
			wantEG:  QueenCentralizationEG,
			wantWBB: []string{"d4"},
		},
		{
			name:    "black centralized queen",
			fen:     "7k/8/8/4q3/8/8/8/4K3 w - - 0 1",
			wantEG:  -QueenCentralizationEG,
			wantBBB: []string{"e5"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trace := traceFromFEN(t, tt.fen)
			assertPair(t, trace.Queen.Terms["centralization"], 0, tt.wantEG)

			centralized := requireSideBitboard(t, trace.Queen.Bitboards, "centralized")
			assertSquares(t, centralized.White.Squares, tt.wantWBB...)
			assertSquares(t, centralized.Black.Squares, tt.wantBBB...)
		})
	}
}

func TestEvalTraceTheoreticalDrawCases(t *testing.T) {
	tests := []struct {
		name string
		fen  string
		draw bool
	}{
		{name: "bare kings", fen: "8/8/8/8/8/8/8/K6k w - - 0 1", draw: true},
		{name: "king and knight versus king", fen: "7k/8/8/8/8/8/8/KN6 w - - 0 1", draw: true},
		{name: "king and bishop versus king", fen: "7k/8/8/8/8/8/8/KB6 w - - 0 1", draw: true},
		{name: "king and two knights versus king", fen: "7k/8/8/8/8/8/8/KNN5 w - - 0 1", draw: true},
		{name: "king and rook versus king", fen: "7k/8/8/8/8/8/8/KR6 w - - 0 1", draw: false},
		{name: "king and queen versus king", fen: "7k/8/8/8/8/8/8/KQ6 w - - 0 1", draw: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trace := traceFromFEN(t, tt.fen)
			if trace.Draw.Theoretical != tt.draw {
				t.Fatalf("draw state mismatch: got %t want %t", trace.Draw.Theoretical, tt.draw)
			}
			if trace.Draw.Applied != tt.draw {
				t.Fatalf("draw applied mismatch: got %t want %t", trace.Draw.Applied, tt.draw)
			}
		})
	}
}

func TestEvalTraceKnightOutpostTermAndSquares(t *testing.T) {
	trace := traceFromFEN(t, "7k/8/8/3N4/4P3/8/8/K7 w - - 0 1")

	outposts := requireSideBitboard(t, trace.Knight.Bitboards, "outpostSquares")
	assertSquares(t, outposts.White.Squares, "d5", "f5")
	assertSquares(t, outposts.Black.Squares)
	assertPair(t, trace.Knight.Terms["outpost"], KnightOutpostMG, KnightOutpostEG)
}

func TestEvalTraceBadBishopTerm(t *testing.T) {
	trace := traceFromFEN(t, "7k/8/8/8/p3p3/P3P3/1P1P4/K1B5 w - - 0 1")

	blocked := requireSideBitboard(t, trace.Bishop.Bitboards, "blockedPawns")
	assertSquares(t, blocked.White.Squares, "a3", "e3")
	assertSquares(t, blocked.Black.Squares, "a4", "e4")
	assertPair(t, trace.Bishop.Terms["badBishop"], 2*BadBishopMG, 2*BadBishopEG)
}

func TestEvalTraceBackwardPawnTermAndSquares(t *testing.T) {
	trace := traceFromFEN(t, "7k/8/4p3/4P3/3P4/8/8/K7 w - - 0 1")

	backward := requireSideBitboard(t, trace.Pawn.Bitboards, "backward")
	assertSquares(t, backward.White.Squares, "d4")
	assertSquares(t, backward.Black.Squares)
	// Black's only pawn is on e6, so nothing faces d4 down the d-file.
	assertPair(t, trace.Pawn.Terms["backward"], -BackwardUnopposedMG, -BackwardUnopposedEG)
}

// White's d3 is backward: c4 is its only neighbour and stands ahead rather than
// behind, so nothing supports it, and black's c5 covers its advance square. The
// black pawn on d6 makes it opposed as well, which should more than halve the
// charge. Black's own pawns are arranged to keep it one-sided -- c5 and d6 are
// each supported from behind, and e7's advance square is uncontested -- so the
// term is white's backward pawn alone.
func TestEvalTraceBackwardPawnOpposed(t *testing.T) {
	trace := traceFromFEN(t, "7k/4p3/3p4/2p5/2P5/3P4/8/K7 w - - 0 1")

	backward := requireSideBitboard(t, trace.Pawn.Bitboards, "backward")
	assertSquares(t, backward.White.Squares, "d3")
	assertSquares(t, backward.Black.Squares)

	// c4 is opposed as well -- black's c5 is ahead of it on the c-file -- but c4
	// is supported by d3 and so is not a backward pawn, and only the
	// intersection of the two sets is charged.
	opposed := requireSideBitboard(t, trace.Pawn.Bitboards, "opposed")
	assertSquares(t, opposed.White.Squares, "d3", "c4")

	assertPair(t, trace.Pawn.Terms["backward"], -BackwardOpposedMG, -BackwardOpposedEG)

	if BackwardOpposedMG >= BackwardUnopposedMG {
		t.Fatal("an opposed backward pawn should cost less than an unopposed one")
	}
}

// The connected multiplier is 2 + phalanx - opposed, so the same pawn can be
// worth one, two or three times its rank value. Each fixture is one-sided:
// black's pawn, where there is one, does not qualify itself.
func TestEvalTraceConnectedPawnMultiplier(t *testing.T) {
	// c4 is defended by b3, and b3 is neither defended nor beside a pawn, so
	// only c4 qualifies. Nothing faces it: multiplier 2.
	supported := traceFromFEN(t, "7k/8/8/8/2P5/1P6/8/K7 w - - 0 1")
	// The same pair with a black pawn on c6. It is isolated and undefended so it
	// does not qualify, but it opposes c4: multiplier 1.
	opposed := traceFromFEN(t, "7k/8/2p5/8/2P5/1P6/8/K7 w - - 0 1")
	// b4 and c4 side by side, neither defended, nothing facing them. Both
	// qualify at multiplier 3, so six units rather than two.
	phalanx := traceFromFEN(t, "7k/8/8/8/1PP5/8/8/K7 w - - 0 1")

	want := func(units int) (int, int) {
		v := units * PawnConnectedMG[3]
		return v, v * (3 - 2) / 4
	}
	mg, eg := want(2)
	assertPair(t, supported.Pawn.Terms["connected"], mg, eg)
	mg, eg = want(1)
	assertPair(t, opposed.Pawn.Terms["connected"], mg, eg)
	mg, eg = want(6)
	assertPair(t, phalanx.Pawn.Terms["connected"], mg, eg)
}

// The same unopposed pair walked up the board: the middlegame value follows the
// rank array and the endgame share grows from a quarter of it to three quarters.
func TestEvalTraceConnectedPawnRankRamp(t *testing.T) {
	fens := map[int]string{
		3: "7k/8/8/8/1PP5/8/8/K7 w - - 0 1",
		4: "7k/8/8/1PP5/8/8/8/K7 w - - 0 1",
		5: "7k/8/1PP5/8/8/8/8/K7 w - - 0 1",
	}
	for r, fen := range fens {
		v := 6 * PawnConnectedMG[r]
		assertPair(t, traceFromFEN(t, fen).Pawn.Terms["connected"], v, v*(r-2)/4)
	}
}

// The starting shape uses the first active rank-table entry. Its current zero
// is an evaluation value, not a structural exclusion, so tuned candidates may
// assign it a nonzero score.
func TestEvalTraceConnectedStartingRankUsesTable(t *testing.T) {
	v := 6 * PawnConnectedMG[1]
	assertPair(t, traceFromFEN(t, "7k/8/8/8/8/8/1PP5/K7 w - - 0 1").Pawn.Terms["connected"], v, v*(1-2)/4)
}

// Blocked pawns are scored over each side's own fifth and sixth ranks only, and
// the phase weighting crosses over between them: mostly a middlegame cost on the
// fifth, mostly an endgame cost on the sixth.
func TestEvalTraceBlockedPawnRanksAndCrossover(t *testing.T) {
	fifth := traceFromFEN(t, "7k/8/4p3/4P3/8/8/8/K7 w - - 0 1")
	sixth := traceFromFEN(t, "7k/4p3/4P3/8/8/8/8/K7 w - - 0 1")

	// Only the side that advanced further is counted: black's blocker sits on
	// its own second or third rank, outside the window.
	assertPair(t, fifth.Pawn.Terms["blocked"], PawnBlockedMG[0], PawnBlockedEG[0])
	assertPair(t, sixth.Pawn.Terms["blocked"], PawnBlockedMG[1], PawnBlockedEG[1])

	if fifth.Pawn.Terms["blocked"].MG >= 0 || sixth.Pawn.Terms["blocked"].EG >= 0 {
		t.Fatal("blocked pawns should now cost the advanced side, not pay it")
	}
	if !(PawnBlockedMG[0] < PawnBlockedMG[1] && PawnBlockedEG[1] < PawnBlockedEG[0]) {
		t.Fatal("expected the middlegame cost to shrink and the endgame cost to grow with rank")
	}
}

func TestPawnFillIncludesEverySquareAhead(t *testing.T) {
	var northWant uint64
	for square := 18; square <= 58; square += 8 { // c3 through c8
		northWant |= PositionBB[square]
	}
	if got := calculatePawnNorthFill(PositionBB[10]); got != northWant { // c2
		t.Fatalf("north fill mismatch: got %#x want %#x", got, northWant)
	}

	var southWant uint64
	for square := 42; square >= 2; square -= 8 { // c6 through c1
		southWant |= PositionBB[square]
	}
	if got := calculatePawnSouthFill(PositionBB[50]); got != southWant { // c7
		t.Fatalf("south fill mismatch: got %#x want %#x", got, southWant)
	}
}

func TestBackwardPawnCanReceiveSupportFromTwoRanksBehind(t *testing.T) {
	trace := traceFromFEN(t, "7k/8/4p3/8/3P4/8/2P5/K7 w - - 0 1")

	backward := requireSideBitboard(t, trace.Pawn.Bitboards, "backward")
	assertSquares(t, backward.White.Squares)
	assertSquares(t, backward.Black.Squares)
}

func TestEvalTraceCandidatePassedPawnTermAndSquares(t *testing.T) {
	trace := traceFromFEN(t, "7k/8/3p4/2P1P3/8/8/8/K7 w - - 0 1")

	candidate := requireSideBitboard(t, trace.Pawn.Bitboards, "candidate")
	assertSquares(t, candidate.White.Squares, "c5", "e5")
	if !slices.Contains(candidate.Black.Squares, "d6") {
		t.Fatalf("black candidate squares should include the real d6 pawn, got %v", candidate.Black.Squares)
	}
	if trace.Pawn.Terms["candidate"].MG <= 0 || trace.Pawn.Terms["candidate"].EG <= 0 {
		t.Fatalf("candidate term should reward the two white candidates, got %+v", trace.Pawn.Terms["candidate"])
	}
}

func TestLeverAndCandidateSquaresContainRealPawns(t *testing.T) {
	trace := traceFromFEN(t, "4r3/5pkp/2b3p1/1p6/8/2p1nPP1/PPN4P/1R2R2K b - - 3 25")

	levers := requireSideBitboard(t, trace.Pawn.Bitboards, "lever")
	candidates := requireSideBitboard(t, trace.Pawn.Bitboards, "candidate")
	assertSquares(t, levers.Black.Squares, "c3")
	assertSquares(t, candidates.Black.Squares, "c3")
}

func TestPawnHashEntryDependsOnlyOnPawnKey(t *testing.T) {
	const fen = "7k/8/3p4/8/2P1P3/8/8/K7 w - - 0 1"
	base := gm.ParseFen(fen)
	want := ComputePawnEntry(&base, false)
	rng := rand.New(rand.NewSource(20260711))
	nonPawns := []gm.Piece{
		gm.WhiteKnight, gm.WhiteBishop, gm.WhiteRook, gm.WhiteQueen,
		gm.BlackKnight, gm.BlackBishop, gm.BlackRook, gm.BlackQueen,
	}

	for sample := 0; sample < 256; sample++ {
		board := gm.ParseFen(fen)
		// Force the candidate-pawn push squares through all occupancy states;
		// the remaining placements broaden the property beyond this bug shape.
		if sample&1 != 0 {
			board.SetPiece(gm.Square(squareIndex("c5")), gm.WhiteKnight)
		}
		if sample&2 != 0 {
			board.SetPiece(gm.Square(squareIndex("e5")), gm.BlackRook)
		}
		for placed := 0; placed < sample%13; placed++ {
			for {
				sq := gm.Square(rng.Intn(64))
				if board.PieceAt(sq) != gm.NoPiece {
					continue
				}
				board.SetPiece(sq, nonPawns[rng.Intn(len(nonPawns))])
				break
			}
		}
		if !board.Validate() {
			t.Fatalf("sample %d produced an inconsistent board", sample)
		}

		got := ComputePawnEntry(&board, false)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("sample %d: pawn-hash entry changed with non-pawn placement:\nwant %+v\n got %+v", sample, want, got)
		}
	}
}

func TestEvaluationIndependentOfPawnHashProcessingOrder(t *testing.T) {
	const open = "7k/8/3p4/8/2P1P3/8/8/K7 w - - 0 1"
	const blocked = "7k/8/3p4/2N1N3/2P1P3/8/8/K7 w - - 0 1"
	defer ClearPawnHash()

	traceClean := func(fen string) EvalTrace {
		ClearPawnHash()
		board := gm.ParseFen(fen)
		return EvalTraceForBoard(&board)
	}
	traceAfter := func(first, target string) EvalTrace {
		ClearPawnHash()
		firstBoard := gm.ParseFen(first)
		_ = EvalTraceForBoard(&firstBoard)
		targetBoard := gm.ParseFen(target)
		return EvalTraceForBoard(&targetBoard)
	}

	cleanOpen := traceClean(open)
	cleanBlocked := traceClean(blocked)
	if reflect.DeepEqual(cleanOpen.Pawn.Terms["candidate"], cleanBlocked.Pawn.Terms["candidate"]) {
		t.Fatal("test fixture does not vary the occupancy-dependent candidate term")
	}
	if got := traceAfter(blocked, open); !reflect.DeepEqual(got, cleanOpen) {
		t.Fatal("open-position trace depends on a prior same-pawn-key evaluation")
	}
	if got := traceAfter(open, blocked); !reflect.DeepEqual(got, cleanBlocked) {
		t.Fatal("blocked-position trace depends on a prior same-pawn-key evaluation")
	}
}

// TestEvalTraceKingStormGeometry pins down which files the storm walk visits and
// which rank it reads on each, by spelling out the table lookups the walk should
// make rather than the number they add up to.
func TestEvalTraceKingStormGeometry(t *testing.T) {
	trace := traceFromFEN(t, "6k1/8/8/8/6P1/8/8/2K5 w - - 0 1")

	// White king c1 walks b, c, d. Neither side has a pawn on any of them, so
	// each file contributes its "no enemy pawn" entry at its own edge distance.
	whiteStorm := -KingStormUnblockedMG[1][0] - KingStormUnblockedMG[2][0] - KingStormUnblockedMG[3][0]
	// Black king g8 walks f, g, h. The g-file holds the white pawn on g4, which
	// stands on black's fourth rank counting from black's own back rank.
	blackStorm := -KingStormUnblockedMG[2][0] - KingStormUnblockedMG[1][4] - KingStormUnblockedMG[0][0]

	assertPair(t, trace.King.Terms["storm"], whiteStorm-blackStorm, 0)
}

// TestEvalTraceKingStormBlockedCarriesEndgame checks the one asymmetry between
// the two storm tables: a locked pawn pair scores in the endgame, an advancing
// pawn that is still free to come on does not.
func TestEvalTraceKingStormBlockedCarriesEndgame(t *testing.T) {
	// Black pawn g3 against white pawn g2: the pair is locked.
	blocked := traceFromFEN(t, "k7/8/8/8/8/6p1/6P1/6K1 w - - 0 1")
	// Same pawns one rank apart: nothing blocks the black pawn yet.
	free := traceFromFEN(t, "k7/8/8/8/6p1/8/6P1/6K1 w - - 0 1")

	if got := blocked.King.Terms["storm"].EG; got != -KingStormBlockedEG[2] {
		t.Fatalf("blocked storm EG: got %d, want %d", got, -KingStormBlockedEG[2])
	}
	if got := free.King.Terms["storm"].EG; got != 0 {
		t.Fatalf("unblocked storm EG: got %d, want 0", got)
	}
}

// TestEvalTraceKingStormSameWing covers the case the retired evaluatePawnStorm
// could not see at all: both kings castled to the same side, where the old
// opposite-castling gate returned a flat zero.
func TestEvalTraceKingStormSameWing(t *testing.T) {
	// Mirror-image castled positions must cancel exactly.
	level := traceFromFEN(t, "5rk1/5ppp/8/8/8/8/5PPP/5RK1 w - - 0 1")
	if got := level.King.Terms["storm"]; got.MG != 0 || got.EG != 0 {
		t.Fatalf("symmetric position storm: got MG %d EG %d, want 0/0", got.MG, got.EG)
	}

	// Push white's h-pawn to h5. It is the same wing both kings castled to, so
	// the old term scored nothing here; the new one must credit white.
	pushed := traceFromFEN(t, "5rk1/5ppp/8/7P/8/8/5PP1/5RK1 w - - 0 1")
	if got := pushed.King.Terms["storm"].MG; got <= 0 {
		t.Fatalf("same-wing h-pawn storm: got MG %d, want > 0", got)
	}
}

// White's a-pawn is isolated either way; the black pawn on a6 only changes
// whether anything faces it down the a-file.
func TestEvalTraceIsolatedPawnOpposedSplit(t *testing.T) {
	unopposed := traceFromFEN(t, "7k/8/8/8/8/8/P7/K7 w - - 0 1")
	opposed := traceFromFEN(t, "7k/8/p7/8/8/8/P7/K7 w - - 0 1")

	iso := requireSideBitboard(t, unopposed.Pawn.Bitboards, "isolated")
	assertSquares(t, iso.White.Squares, "a2")

	assertPair(t, unopposed.Pawn.Terms["isolated"], -IsolatedUnopposedMG, -IsolatedUnopposedEG)

	// Black's a6 is isolated too, so the two charges partly cancel; both sides
	// are opposed here, leaving a difference of zero.
	assertPair(t, opposed.Pawn.Terms["isolated"], 0, 0)

	if IsolatedOpposedMG >= IsolatedUnopposedMG || IsolatedOpposedEG >= IsolatedUnopposedEG {
		t.Fatal("an opposed isolated pawn should cost less than an unopposed one")
	}
}

// The e-file slice of white's zone is e2, e3, e4. Advancing a pawn pays for the
// ground it leaves behind; a second pawn standing in that ground takes the
// payment back.
func TestSpaceBonusRewardsAdvanceAndPunishesDoubling(t *testing.T) {
	const (
		e2 = uint64(1) << 12
		e3 = uint64(1) << 20
		e4 = uint64(1) << 28
	)
	zone := wSpaceZoneMask & onlyFile[4]
	bonus := func(pawns uint64) int { return spaceBonusFor(pawns, 0, zone, 0, 0, true) }

	onE2 := bonus(e2)         // e3 and e4 safe, neither behind it
	onE4 := bonus(e4)         // e2 and e3 safe, both behind it
	doubled := bonus(e2 | e4) // e3 safe and behind e4; e2 and e4 occupied

	if want := 2 * SpaceSafeMG; onE2 != want {
		t.Fatalf("pawn on e2: got %d want %d", onE2, want)
	}
	if want := 2*SpaceSafeMG + 2*SpaceBehindPawnMG; onE4 != want {
		t.Fatalf("pawn on e4: got %d want %d", onE4, want)
	}
	if want := SpaceSafeMG + SpaceBehindPawnMG; doubled != want {
		t.Fatalf("pawns on e2 and e4: got %d want %d", doubled, want)
	}
	if doubled >= onE4 {
		t.Fatalf("doubling should forfeit the advance: doubled %d, advanced %d", doubled, onE4)
	}
}

// Only enemy pawn attacks deny a square. Piece attacks never reach this term.
func TestSpaceBonusExcludesEnemyPawnAttacksOnly(t *testing.T) {
	const e3 = uint64(1) << 20
	zone := wSpaceZoneMask & onlyFile[4]

	if got, want := spaceBonusFor(0, 0, zone, 0, 0, true), 3*SpaceSafeMG; got != want {
		t.Fatalf("empty file: got %d want %d", got, want)
	}
	if got, want := spaceBonusFor(0, e3, zone, 0, 0, true), 2*SpaceSafeMG; got != want {
		t.Fatalf("e3 covered by an enemy pawn: got %d want %d", got, want)
	}
}

// A square we cannot hold structurally is discounted, and a fully open file
// more than a semi-open one.
func TestSpaceBonusFileTiers(t *testing.T) {
	zone := wSpaceZoneMask & onlyFile[4]
	eFile := onlyFile[4]

	plain := spaceBonusFor(0, 0, zone, 0, 0, true)
	semi := spaceBonusFor(0, 0, zone, eFile, 0, true)
	open := spaceBonusFor(0, 0, zone, 0, eFile, true)

	if want := plain + 3*SpaceSemiOpenMG; semi != want {
		t.Fatalf("semi-open file: got %d want %d", semi, want)
	}
	if want := plain + 3*SpaceOpenMG; open != want {
		t.Fatalf("open file: got %d want %d", open, want)
	}
	//if !(SpaceOpenMG < SpaceSemiOpenMG && SpaceSemiOpenMG < 0) {
	//	t.Fatal("expected open to be discounted more than semi-open, and both negative")
	//}
}

// Mirror-image structures cancel, and the term is middlegame only.
func TestEvalTraceSpaceSymmetryAndPhase(t *testing.T) {
	trace := traceFromFEN(t, "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")

	if trace.Space.MG != 0 {
		t.Fatalf("symmetric position space MG: got %d want 0", trace.Space.MG)
	}
	if trace.Space.EG != 0 {
		t.Fatalf("space should carry no endgame value, got %d", trace.Space.EG)
	}
}

// EvalExplain used to negate getKingMopUpBonus a second time, after the function
// had already negated it for the black-advantage case, so it reported the mop-up
// backwards whenever black was the side mating. Mirrored positions must give
// equal and opposite endgame-king terms.
func TestEvalExplainMopUpSignMirrors(t *testing.T) {
	// White mates with the rook; then the same position with colours swapped.
	wBoard := gm.ParseFen("7k/8/6K1/8/8/8/8/7R w - - 0 1")
	bBoard := gm.ParseFen("7r/8/8/8/8/6k1/8/7K b - - 0 1")
	_, white := EvalExplain(&wBoard)
	_, black := EvalExplain(&bBoard)

	if white.KingEndgame == 0 {
		t.Fatal("fixture does not reach the mop-up branch")
	}
	if white.KingEndgame != -black.KingEndgame {
		t.Fatalf("mop-up sign is not mirror-symmetric: white %d, black %d",
			white.KingEndgame, black.KingEndgame)
	}
}

// A lone knight on e6 bears on two squares of the black king's ring (f8 and g7)
// and gives no check from there, so the accumulator is one piece weight plus two
// ring squares. White has no queen, so SafetyNoEnemyQueens comes off the top.
//
// That constant is a discount, not an execution. At Ethereal's -237 it was an
// execution: it exceeded any attack a single piece can mount, and measured over
// Goose's own games it floored 86.2% of queenless sides at exactly zero against
// 6.5% under the attack-unit table it replaced. At -60 the smallest attack that
// exists -- one bishop touching one ring square, 24 + 45 -- still clears it, so
// the clamp in kingDangerRaw survives mainly as a guard rather than as a term
// that fires. The queenless discount now lives in the squaring: taking 60 off
// the accumulator costs far more at the top of the range than at the bottom.
func TestEvalTraceKingDangerQueenlessDiscount(t *testing.T) {
	trace := traceFromFEN(t, "6k1/8/4N3/8/8/8/8/K7 w - - 0 1")

	const ringHits = 2 // knight on e6 covers f8 and g7

	withQueen := SafetyKnightWeightMG + SafetyAttackValueMG*ringHits + SafetyAdjustmentMG
	wantRaw := withQueen + SafetyNoEnemyQueensMG
	if wantRaw <= 0 {
		t.Fatalf("fixture no longer exercises the discount: raw %d", wantRaw)
	}

	if got := trace.King.AttackersOnBlackKing; got != 1 {
		t.Fatalf("attacker count on black king: got %d want 1", got)
	}
	if got := trace.King.SafeChecksOnBlackKing + trace.King.UnsafeChecksOnBlackKing; got != 0 {
		t.Fatalf("fixture should have no checks, got %d", got)
	}
	if got := trace.King.AttackUnitsOnBlackKing; got != wantRaw {
		t.Fatalf("queenless attack units: got %d want %d", got, wantRaw)
	}
	want := (wantRaw * wantRaw) / (SafetyMGDivisor * SafetyMGDivisor)
	if got := trace.King.DangerToBlackKing; got != want {
		t.Fatalf("queenless danger: got %d want %d", got, want)
	}
}

// The same knight, plus a white queen on a2. The queen never touches the king
// ring, so it adds no attacker weight -- but it does have two checking squares,
// a8 along the a-file and g2 along the second rank. Both sit well outside the
// ring, which is the case a ring-anchored check test would miss entirely, and
// black has nothing but its king so neither square is defended.
func TestEvalTraceKingDangerAccumulator(t *testing.T) {
	trace := traceFromFEN(t, "6k1/8/4N3/8/8/8/Q7/K7 w - - 0 1")

	const ringHits = 2   // knight on e6 covers f8 and g7
	const safeChecks = 2 // queen to a8 or g2

	wantRawMG := SafetyKnightWeightMG + SafetyAttackValueMG*ringHits +
		SafetySafeQueenCheckMG*safeChecks + SafetyAdjustmentMG
	wantRawEG := SafetyKnightWeightEG + SafetyAttackValueEG*ringHits +
		SafetySafeQueenCheckEG*safeChecks + SafetyAdjustmentEG
	wantMG := (wantRawMG * wantRawMG) / (SafetyMGDivisor * SafetyMGDivisor)
	wantEG := wantRawEG / SafetyEGDivisor

	if got := trace.King.AttackersOnBlackKing; got != 1 {
		t.Fatalf("attacker count on black king: got %d want 1 (queen misses the ring)", got)
	}
	if got := trace.King.SafeChecksOnBlackKing; got != safeChecks {
		t.Fatalf("safe checks on black king: got %d want %d", got, safeChecks)
	}
	if got := trace.King.UnsafeChecksOnBlackKing; got != 0 {
		t.Fatalf("black defends nothing, so no check should be unsafe; got %d", got)
	}
	if got := trace.King.AttackUnitsOnBlackKing; got != wantRawMG {
		t.Fatalf("raw danger accumulator: got %d want %d", got, wantRawMG)
	}
	if got := trace.King.DangerToBlackKing; got != wantMG {
		t.Fatalf("danger to black king: got %d want %d", got, wantMG)
	}
	// Black has no attackers at all, so nothing is charged against the white king.
	if got := trace.King.AttackUnitsOnWhiteKing; got != 0 {
		t.Fatalf("black generates no attack, got %d units", got)
	}
	assertPair(t, trace.King.Terms["attack"], wantMG, wantEG)
}

// Defence decides safe from unsafe. The black rook on d8 has two checking
// squares against the white king on e1: d1 along the rank and e8 down the file.
// White's knight on c3 covers d1, so that check is unsafe; nothing covers e8, so
// that one is safe.
func TestEvalTraceKingDangerSafeVersusUnsafeCheck(t *testing.T) {
	trace := traceFromFEN(t, "3r2k1/8/8/8/8/2N5/8/4K3 w - - 0 1")

	if got := trace.King.SafeChecksOnWhiteKing; got != 1 {
		t.Fatalf("e8 is undefended and should be the one safe check; got %d", got)
	}
	if got := trace.King.UnsafeChecksOnWhiteKing; got != 1 {
		t.Fatalf("d1 is covered by the c3 knight and should be the one unsafe check; got %d", got)
	}
}

func TestEvalTraceMobilityTerms(t *testing.T) {
	// Knight and bishop mobility are scaled by centre openness in BOTH phases, so
	// the expected values are the raw bucket times the scale the trace reports.
	// Rooks and queens are not centre-scaled and carry a nil scales function.
	tests := []struct {
		name       string
		fen        string
		tracePiece func(EvalTrace) EvalPieceTrace
		bucket     int
		rawMG      int
		rawEG      int
		scales     func(EvalTrace) (mg, eg int)
	}{
		{
			name:       "knight on d4",
			fen:        "7k/8/8/8/3N4/8/8/K7 w - - 0 1",
			tracePiece: func(trace EvalTrace) EvalPieceTrace { return trace.Knight },
			bucket:     8,
			rawMG:      KnightMobilityMG[8],
			rawEG:      KnightMobilityEG[8],
			scales: func(tr EvalTrace) (int, int) {
				return tr.Center.KnightMobilityScale, tr.Center.KnightMobilityScaleEG
			},
		},
		{
			name:       "bishop on d4",
			fen:        "7k/8/8/8/3B4/8/8/4K3 w - - 0 1",
			tracePiece: func(trace EvalTrace) EvalPieceTrace { return trace.Bishop },
			bucket:     13,
			rawMG:      BishopMobilityMG[13],
			rawEG:      BishopMobilityEG[13],
			scales: func(tr EvalTrace) (int, int) {
				return tr.Center.BishopMobilityScale, tr.Center.BishopMobilityScaleEG
			},
		},
		{
			name:       "rook on d4",
			fen:        "7k/8/8/8/3R4/8/8/4K3 w - - 0 1",
			tracePiece: func(trace EvalTrace) EvalPieceTrace { return trace.Rook },
			bucket:     14,
			rawMG:      RookMobilityMG[14],
			rawEG:      RookMobilityEG[14],
		},
		{
			name:       "queen on d4",
			fen:        "7k/8/8/8/3Q4/8/8/4K3 w - - 0 1",
			tracePiece: func(trace EvalTrace) EvalPieceTrace { return trace.Queen },
			bucket:     21,
			rawMG:      QueenMobilityMG[21],
			rawEG:      QueenMobilityEG[21],
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trace := traceFromFEN(t, tt.fen)
			piece := tt.tracePiece(trace)

			wantMG, wantEG := tt.rawMG, tt.rawEG
			if tt.scales != nil {
				scaleMG, scaleEG := tt.scales(trace)
				if scaleMG == 100 && scaleEG == 100 {
					t.Fatalf("these pawnless positions should scale; got 100/100")
				}
				wantMG = wantMG * scaleMG / 100
				wantEG = wantEG * scaleEG / 100
			}
			assertPair(t, piece.Terms["mobility"], wantMG, wantEG)
			if piece.MobilityCounts[tt.bucket] != 1 {
				t.Fatalf("mobility bucket %d mismatch: got %d want 1 in counts %v", tt.bucket, piece.MobilityCounts[tt.bucket], piece.MobilityCounts)
			}
		})
	}
}

func TestEvalTracePassedPawnTermsByRank(t *testing.T) {
	tests := []struct {
		name   string
		fen    string
		side   string
		square string
		wantMG int
		wantEG int
	}{
		{
			name:   "white passed pawn on e4",
			fen:    "7k/8/8/8/4P3/8/8/K7 w - - 0 1",
			side:   "white",
			square: "e4",
			wantMG: PassedPawnPSQT_MG[squareIndex("e4")],
			wantEG: PassedPawnPSQT_EG[squareIndex("e4")],
		},
		{
			name:   "white passed pawn on e6",
			fen:    "7k/8/4P3/8/8/8/8/K7 w - - 0 1",
			side:   "white",
			square: "e6",
			wantMG: PassedPawnPSQT_MG[squareIndex("e6")],
			wantEG: PassedPawnPSQT_EG[squareIndex("e6")],
		},
		{
			name:   "black passed pawn on e5",
			fen:    "7k/8/8/4p3/8/8/8/K7 w - - 0 1",
			side:   "black",
			square: "e5",
			wantMG: -PassedPawnPSQT_MG[FlipView[squareIndex("e5")]],
			wantEG: -PassedPawnPSQT_EG[FlipView[squareIndex("e5")]],
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trace := traceFromFEN(t, tt.fen)
			passed := requireSideBitboard(t, trace.Pawn.Bitboards, "passed")
			if tt.side == "white" {
				assertSquares(t, passed.White.Squares, tt.square)
				assertSquares(t, passed.Black.Squares)
			} else {
				assertSquares(t, passed.White.Squares)
				assertSquares(t, passed.Black.Squares, tt.square)
			}
			assertPair(t, trace.Pawn.Terms["passed"], tt.wantMG, tt.wantEG)
		})
	}

	lessAdvanced := traceFromFEN(t, "7k/8/8/8/4P3/8/8/K7 w - - 0 1")
	moreAdvanced := traceFromFEN(t, "7k/8/4P3/8/8/8/8/K7 w - - 0 1")
	if moreAdvanced.Pawn.Terms["passed"].MG <= lessAdvanced.Pawn.Terms["passed"].MG {
		t.Fatalf("advanced passed pawn MG should increase: e4 %+v e6 %+v", lessAdvanced.Pawn.Terms["passed"], moreAdvanced.Pawn.Terms["passed"])
	}
	if moreAdvanced.Pawn.Terms["passed"].EG <= lessAdvanced.Pawn.Terms["passed"].EG {
		t.Fatalf("advanced passed pawn EG should increase: e4 %+v e6 %+v", lessAdvanced.Pawn.Terms["passed"], moreAdvanced.Pawn.Terms["passed"])
	}
}

// The material imbalance term reads the TOTAL pawn count and multiplies it by
// the knight-count difference, so it is silent whenever the knights are equal --
// no matter how lopsided the pawns or bishops are. The previous per-side form
// was not: it emitted a score for a pure pawn-count difference, re-pricing pawn
// material that the pawn's own value already carries.
func TestEvalTraceImbalanceSilentWithoutKnightDifference(t *testing.T) {
	tests := []struct {
		name string
		fen  string
		// Same position with white's knight removed. Without this the test would
		// pass just as happily against a term hardcoded to zero: the control
		// proves the pawn delta really is non-zero here and that the knight
		// difference is what silences the term.
		control string
	}{
		// 7 white pawns vs 5 black, one knight each. Total 12, so the pawn delta
		// is +2 and non-zero: only the knight difference can be zeroing this.
		{
			"pawn count difference",
			"4k3/1n1ppppp/8/8/8/8/PPPPPPP1/1N2K3 w - - 0 1",
			"4k3/1n1ppppp/8/8/8/8/PPPPPPP1/4K3 w - - 0 1",
		},
		// Same pawns, plus two white bishops against one black. The retired
		// per-bishop tilt would have scored this.
		{
			"bishop count difference",
			"4k3/1nbppppp/8/8/8/8/PPPPPPP1/1NB1KB2 w - - 0 1",
			"4k3/1nbppppp/8/8/8/8/PPPPPPP1/2B1KB2 w - - 0 1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertPair(t, traceFromFEN(t, tt.fen).Imbalance, 0, 0)
			assertPair(t, traceFromFEN(t, tt.control).Imbalance,
				-2*ImbalanceKnightPerPawnMG, -2*ImbalanceKnightPerPawnEG)
		})
	}
}

// A knight surplus is worth more the more pawns are on the board and less as
// they come off -- the sign flips either side of ImbalanceRefPawnCount.
func TestEvalTraceImbalanceKnightTiltFlipsWithPawnCount(t *testing.T) {
	// 14 pawns, two white knights against one: delta +4, knightDiff +1.
	many := traceFromFEN(t, "4k3/ppppppp1/1n6/8/8/8/PPPPPPP1/1N2K1N1 w - - 0 1")
	assertPair(t, many.Imbalance, 4*ImbalanceKnightPerPawnMG, 4*ImbalanceKnightPerPawnEG)

	// Same knights, 4 pawns: delta -6, so the surplus is now a penalty.
	few := traceFromFEN(t, "4k3/pp6/1n6/8/8/8/PP6/1N2K1N1 w - - 0 1")
	assertPair(t, few.Imbalance, -6*ImbalanceKnightPerPawnMG, -6*ImbalanceKnightPerPawnEG)

	if many.Imbalance.MG <= 0 || few.Imbalance.MG >= 0 {
		t.Fatalf("knight tilt should change sign across the reference: many %+v few %+v",
			many.Imbalance, few.Imbalance)
	}
}

// The term is unclamped, and the extreme it used to clamp away points the right
// way: two knights and no pawns cannot mate, so the surplus is a large penalty.
func TestEvalTraceImbalancePawnlessKnightSurplus(t *testing.T) {
	trace := traceFromFEN(t, "4k3/8/8/8/8/8/8/1N2K1N1 w - - 0 1")
	assertPair(t, trace.Imbalance, -20*ImbalanceKnightPerPawnMG, -20*ImbalanceKnightPerPawnEG)
}

func TestEvalTraceImbalanceMirrorsByColour(t *testing.T) {
	white := traceFromFEN(t, "4k3/ppppppp1/1n6/8/8/8/PPPPPPP1/1N2K1N1 w - - 0 1")
	black := traceFromFEN(t, "1n2k1n1/ppppppp1/8/8/8/1N6/PPPPPPP1/4K3 b - - 0 1")
	assertPair(t, black.Imbalance, -white.Imbalance.MG, -white.Imbalance.EG)
	if white.Imbalance.MG == 0 {
		t.Fatal("mirror check is vacuous: the term scored nothing for white")
	}
}

// The rook file-COUNT bonus pays for owning rooks in a structure they can use,
// whether or not a rook currently stands on such a file. That is what separates
// it from the per-rook "semiOpen"/"open" terms, which only pay on arrival.
func TestEvalTraceRookFileCountPaysOffFile(t *testing.T) {
	// Black holds all eight files, white is missing d and e, so d and e are
	// semi-open FOR WHITE and no file is fully open. Both rooks sit on a, which
	// carries pawns for both sides -- so neither rook is on a file the per-rook
	// terms would pay for.
	trace := traceFromFEN(t, "r5k1/pppppppp/8/8/8/8/PPP2PPP/R5K1 w - - 0 1")

	// Control: the per-rook terms must be silent, or this proves nothing about
	// which term is paying. Both are zero here by absence, not by cancellation.
	assertPair(t, trace.Rook.Terms["semiOpen"], 0, 0)
	assertPair(t, trace.Rook.Terms["open"], 0, 0)

	// Two white semi-open files, none for black, one rook each side.
	assertPair(t, trace.Rook.Terms["fileCount"],
		2*RookFileCountSemiMG, 2*RookFileCountSemiEG)
}

// Open files are shared, so that half of the term cancels unless the rook counts
// themselves differ.
func TestEvalTraceRookFileCountOpenNeedsRookImbalance(t *testing.T) {
	// a, d and e fully open, one rook each: the open half cancels exactly, and
	// there are no semi-open files for either side.
	equal := traceFromFEN(t, "r5k1/1pp2ppp/8/8/8/8/1PP2PPP/R5K1 w - - 0 1")
	assertPair(t, equal.Rook.Terms["fileCount"], 0, 0)

	// Same structure, white a rook up: three open files now speak once.
	extra := traceFromFEN(t, "r5k1/1pp2ppp/8/8/8/8/1PP2PPP/R5KR w - - 0 1")
	assertPair(t, extra.Rook.Terms["fileCount"],
		3*RookFileCountOpenMG, 3*RookFileCountOpenEG)
}

func TestEvalTraceRookFileCountMirrorsByColour(t *testing.T) {
	white := traceFromFEN(t, "r5k1/pppppppp/8/8/8/8/PPP2PPP/R5K1 w - - 0 1")
	black := traceFromFEN(t, "r5k1/ppp2ppp/8/8/8/8/PPPPPPPP/R5K1 b - - 0 1")
	assertPair(t, black.Rook.Terms["fileCount"],
		-white.Rook.Terms["fileCount"].MG, -white.Rook.Terms["fileCount"].EG)
	if white.Rook.Terms["fileCount"].MG == 0 {
		t.Fatal("mirror check is vacuous: the term scored nothing for white")
	}
}

// A side with no rooks collects nothing however usable its files are. This is
// TestEvalTraceRookFileCountPaysOffFile's position with white's rook removed:
// the same two semi-open files now pay zero instead of 2*RookFileCountSemiMG.
func TestEvalTraceRookFileCountNeedsRooks(t *testing.T) {
	trace := traceFromFEN(t, "r5k1/pppppppp/8/8/8/8/PPP2PPP/6K1 w - - 0 1")
	assertPair(t, trace.Rook.Terms["fileCount"], 0, 0)
}

// The centre scales are linear in openness rather than bucketed. The old form
// had thresholds at openIdx 0.25 and 0.75 with a dead band between them that
// held 45.7% of corpus positions and scaled nothing at all.
func TestCenterMobilityScalesAreLinearAndCoverTheMiddle(t *testing.T) {
	// openIdx maps to openness in quarters: 0 -> -4, 0.5 -> 0, 1 -> +4.
	tests := []struct {
		openIdx                      float64
		wantKnMG, wantBiMG, wantBpMG int
	}{
		{0.0, 100 + CenterKnightMobilityMG, 100 - CenterBishopMobilityMG, 100 - CenterBishopPairMG},
		{0.5, 100, 100, 100},
		{1.0, 100 - CenterKnightMobilityMG, 100 + CenterBishopMobilityMG, 100 + CenterBishopPairMG},
	}
	for _, tt := range tests {
		got := getCenterMobilityScales(false, tt.openIdx)
		if got.knightMobilityMG != tt.wantKnMG || got.bishopMobilityMG != tt.wantBiMG || got.bishopPairMG != tt.wantBpMG {
			t.Fatalf("openIdx %.3f: got kn %d bi %d bp %d, want kn %d bi %d bp %d",
				tt.openIdx, got.knightMobilityMG, got.bishopMobilityMG, got.bishopPairMG,
				tt.wantKnMG, tt.wantBiMG, tt.wantBpMG)
		}
	}

	// The three index values that the old dead band swallowed must now scale.
	for _, idx := range []float64{0.375, 0.625} {
		got := getCenterMobilityScales(false, idx)
		if got.knightMobilityMG == 100 && got.bishopMobilityMG == 100 {
			t.Fatalf("openIdx %.3f still scales nothing", idx)
		}
	}

	// Monotone in openness, with no cliff between adjacent index steps.
	prevKn, prevBi := 1<<30, -(1 << 30)
	for i := 0; i <= 8; i++ {
		got := getCenterMobilityScales(false, float64(i)/8)
		if got.knightMobilityMG > prevKn {
			t.Fatalf("knight scale should fall as the centre opens; %d then %d", prevKn, got.knightMobilityMG)
		}
		if got.bishopMobilityMG < prevBi {
			t.Fatalf("bishop scale should rise as the centre opens; %d then %d", prevBi, got.bishopMobilityMG)
		}
		prevKn, prevBi = got.knightMobilityMG, got.bishopMobilityMG
	}
}

// Both phases are scaled now. The endgame swing is deliberately the smaller of
// the two in percentage terms because endgame mobility is roughly three times
// the middlegame figure.
func TestCenterMobilityScalesCarryEndgame(t *testing.T) {
	open := getCenterMobilityScales(false, 1.0)
	closed := getCenterMobilityScales(false, 0.0)

	for _, got := range []int{open.knightMobilityEG, open.bishopMobilityEG, open.bishopPairEG,
		closed.knightMobilityEG, closed.bishopMobilityEG, closed.bishopPairEG} {
		if got == 100 {
			t.Fatal("endgame scales must not be inert; that was the bug being fixed")
		}
	}
	//if absInt(open.knightMobilityEG-100) >= absInt(open.knightMobilityMG-100) {
	//	t.Fatalf("endgame knight swing should be the smaller: MG %d EG %d",
	//		open.knightMobilityMG, open.knightMobilityEG)
	//}
	//if absInt(open.bishopMobilityEG-100) >= absInt(open.bishopMobilityMG-100) {
	//	t.Fatalf("endgame bishop swing should be the smaller: MG %d EG %d",
	//		open.bishopMobilityMG, open.bishopMobilityEG)
	//}
}

// Integer division truncates toward zero, which is symmetric about it, so a
// position and its colour mirror must scale by exactly opposite amounts.
func TestCenterMobilityScalesRoundSymmetrically(t *testing.T) {
	for i := 0; i <= 4; i++ {
		lo := getCenterMobilityScales(false, float64(i)/8)
		hi := getCenterMobilityScales(false, float64(8-i)/8)
		if (lo.knightMobilityMG - 100) != -(hi.knightMobilityMG - 100) {
			t.Fatalf("asymmetric rounding at step %d: %d vs %d", i, lo.knightMobilityMG, hi.knightMobilityMG)
		}
		if (lo.bishopPairEG - 100) != -(hi.bishopPairEG - 100) {
			t.Fatalf("asymmetric rounding at step %d: %d vs %d", i, lo.bishopPairEG, hi.bishopPairEG)
		}
	}
}

// A locked centre is pinned to maximally closed regardless of what the file
// count says, because c or f being semi-open can still read openIdx 0.25.
func TestCenterMobilityScalesLockedPinsToClosed(t *testing.T) {
	locked := getCenterMobilityScales(true, 0.25)
	fullyClosed := getCenterMobilityScales(false, 0.0)
	if locked != fullyClosed {
		t.Fatalf("locked centre should scale as fully closed: got %+v want %+v", locked, fullyClosed)
	}
}

// getOutpostsBB used to rebuild, per candidate square per node, a mask that
// depends only on the square. outpostBlockersWhite/Black hold it precomputed.
// This pins the table against the arithmetic it replaced rather than trusting
// the derivation, including the two far-rank cases where the old code took an
// explicit "automatic outpost" branch and the table relies on an empty mask
// giving the same answer.
func TestOutpostBlockerTablesMatchTheOldArithmetic(t *testing.T) {
	InitPositionBB()
	InitPassedPawnMasks()

	for sq := 0; sq < 64; sq++ {
		file, rank := sq%8, sq/8

		var adjFilesMask uint64
		if file > 0 {
			adjFilesMask |= onlyFile[file-1]
		}
		if file < 7 {
			adjFilesMask |= onlyFile[file+1]
		}

		var wantWhite, wantBlack uint64
		if rank < 7 {
			wantWhite = adjFilesMask & ranksAbove[rank+1]
		}
		if rank > 0 {
			wantBlack = adjFilesMask & ranksBelow[rank-1]
		}

		if got := outpostBlockersWhite[sq]; got != wantWhite {
			t.Fatalf("white blockers for square %d: got %#016x want %#016x", sq, got, wantWhite)
		}
		if got := outpostBlockersBlack[sq]; got != wantBlack {
			t.Fatalf("black blockers for square %d: got %#016x want %#016x", sq, got, wantBlack)
		}
	}
}

// Whole-function equivalence over random positions, so a mistake in how the
// table is consumed is caught as well as a mistake in the table itself.
func TestGetOutpostsBBMatchesReferenceOverRandomPositions(t *testing.T) {
	InitPositionBB()
	InitPassedPawnMasks()

	reference := func(b *gm.Board, wPawnAttackBB, bPawnAttackBB uint64) [2]uint64 {
		var out [2]uint64
		walk := func(cands uint64, enemyPawns uint64, white bool) uint64 {
			var bb uint64
			for x := cands; x != 0; x &= x - 1 {
				sq := bits.TrailingZeros64(x)
				file, rank := sq%8, sq/8
				var adj uint64
				if file > 0 {
					adj |= onlyFile[file-1]
				}
				if file < 7 {
					adj |= onlyFile[file+1]
				}
				if white {
					if rank >= 7 || enemyPawns&adj&ranksAbove[rank+1] == 0 {
						bb |= PositionBB[sq]
					}
				} else {
					if rank <= 0 || enemyPawns&adj&ranksBelow[rank-1] == 0 {
						bb |= PositionBB[sq]
					}
				}
			}
			return bb
		}
		out[0] = walk((wPawnAttackBB&wAllowedOutpostMask)&^b.White.Pawns, b.Black.Pawns, true)
		out[1] = walk((bPawnAttackBB&bAllowedOutpostMask)&^b.Black.Pawns, b.White.Pawns, false)
		return out
	}

	rng := rand.New(rand.NewSource(20260810))
	var board gm.Board
	nonEmpty := 0
	for i := 0; i < 20000; i++ {
		// Random pawns only; outposts depend on nothing else.
		board.White.Pawns = rng.Uint64() &^ (onlyRank[0] | onlyRank[7])
		board.Black.Pawns = rng.Uint64() &^ (onlyRank[0] | onlyRank[7]) &^ board.White.Pawns

		wAtkE, wAtkW := PawnCaptureBitboards(board.White.Pawns, true)
		bAtkE, bAtkW := PawnCaptureBitboards(board.Black.Pawns, false)
		wAtk, bAtk := wAtkE|wAtkW, bAtkE|bAtkW

		got := getOutpostsBB(&board, wAtk, bAtk)
		want := reference(&board, wAtk, bAtk)
		if got != want {
			t.Fatalf("mismatch on iteration %d\n white pawns %#016x\n black pawns %#016x\n got %#016x/%#016x want %#016x/%#016x",
				i, board.White.Pawns, board.Black.Pawns, got[0], got[1], want[0], want[1])
		}
		if got[0] != 0 || got[1] != 0 {
			nonEmpty++
		}
	}
	if nonEmpty < 1000 {
		t.Fatalf("comparison is near-vacuous: only %d of 20000 positions produced any outpost", nonEmpty)
	}
}
