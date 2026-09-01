package engine

import (
	"testing"

	gm "chess-engine/goosemg"
)

func correctionTestMove(t *testing.T, b *gm.Board, uci string) gm.Move {
	t.Helper()
	for _, move := range b.GenerateLegalMoves() {
		if move.String() == uci {
			return move
		}
	}
	t.Fatalf("move %s is not legal in %s", uci, b.ToFen())
	return 0
}

func playCorrectionTestMove(t *testing.T, b *gm.Board, uci string) {
	t.Helper()
	move := correctionTestMove(t, b, uci)
	if ok, _ := b.MakeMove(move); !ok {
		t.Fatalf("move %s was rejected", uci)
	}
}

func seedCorrectionHistory(h *correctionHistory, b *gm.Board, value int16) {
	indices := correctionIndices(b)
	h.pawn[indices.side][indices.pawn] = value
	h.minor[indices.side][indices.minor] = value
	h.nonPawn[indices.side][0][indices.whiteNonPawn] = value
	h.nonPawn[indices.side][1][indices.blackNonPawn] = value
}

func TestCorrectionStructureKeyBoundaries(t *testing.T) {
	b := gm.ParseFen(gm.FENStartPos)
	pawnKey := pawnStructureKey(b.White.Pawns, b.Black.Pawns)
	minorKey := minorStructureKey(&b)
	whiteKey := nonPawnStructureKey(&b, gm.White)
	blackKey := nonPawnStructureKey(&b, gm.Black)

	playCorrectionTestMove(t, &b, "g1f3")
	if pawnStructureKey(b.White.Pawns, b.Black.Pawns) != pawnKey {
		t.Fatal("non-pawn move changed pawn key")
	}
	if minorStructureKey(&b) == minorKey || nonPawnStructureKey(&b, gm.White) == whiteKey {
		t.Fatal("knight move did not change its structural keys")
	}
	if nonPawnStructureKey(&b, gm.Black) != blackKey {
		t.Fatal("white move changed black non-pawn key")
	}

	whiteKey = nonPawnStructureKey(&b, gm.White)
	playCorrectionTestMove(t, &b, "g8f6")
	if nonPawnStructureKey(&b, gm.White) != whiteKey {
		t.Fatal("black move changed white non-pawn key")
	}

	pawnMove := gm.ParseFen(gm.FENStartPos)
	whiteKey = nonPawnStructureKey(&pawnMove, gm.White)
	blackKey = nonPawnStructureKey(&pawnMove, gm.Black)
	playCorrectionTestMove(t, &pawnMove, "e2e4")
	if nonPawnStructureKey(&pawnMove, gm.White) != whiteKey || nonPawnStructureKey(&pawnMove, gm.Black) != blackKey {
		t.Fatal("pawn move changed a non-pawn key")
	}

	for _, test := range []struct {
		name string
		fen  string
		move string
	}{
		{"rook", "4k3/8/8/8/8/2NB4/8/R3K3 w - - 0 1", "a1a2"},
		{"queen", "4k3/8/8/8/8/2NB4/8/3QK3 w - - 0 1", "d1d2"},
		{"king", "4k3/8/8/8/8/2NB4/8/4K3 w - - 0 1", "e1f1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			board := gm.ParseFen(test.fen)
			key := minorStructureKey(&board)
			playCorrectionTestMove(t, &board, test.move)
			if minorStructureKey(&board) != key {
				t.Fatalf("%s move changed minor key", test.name)
			}
		})
	}
}

func TestCorrectionHistoryIndexAndSideSeparation(t *testing.T) {
	if correctionHistoryIndex(0x1234) != correctionHistoryIndex(0x1234+correctionHistorySize) {
		t.Fatal("masked correction collision did not share an index")
	}

	white := gm.ParseFen(gm.FENStartPos)
	black := gm.ParseFen("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR b KQkq - 0 1")
	history := new(correctionHistory)
	seedCorrectionHistory(history, &white, 1024)
	if got := history.read(&white).correction; got != 4 {
		t.Fatalf("white correction = %d, want 4", got)
	}
	if got := history.read(&black).correction; got != 0 {
		t.Fatalf("black correction = %d, want 0", got)
	}
}

func TestCorrectionHistoryCombination(t *testing.T) {
	b := gm.ParseFen(gm.FENStartPos)
	history := new(correctionHistory)
	indices := correctionIndices(&b)
	history.pawn[indices.side][indices.pawn] = 256
	history.minor[indices.side][indices.minor] = 512
	history.nonPawn[indices.side][0][indices.whiteNonPawn] = -256
	history.nonPawn[indices.side][1][indices.blackNonPawn] = 512

	read := history.read(&b)
	if read.correction != 1 {
		t.Fatalf("combined correction = %d, want 1", read.correction)
	}
	if applyStaticCorrection(Checkmate-2, 10) != Checkmate-1 || applyStaticCorrection(-Checkmate+2, -10) != -Checkmate+1 {
		t.Fatal("static correction escaped non-mate range")
	}
}

func TestCorrectionHistoryEMAAndBounds(t *testing.T) {
	b := gm.ParseFen(gm.FENStartPos)
	history := new(correctionHistory)
	for range 200 {
		update := history.update(&b, 15, 0, 20)
		if update.direction != 1 || update.saturated {
			t.Fatalf("ordinary update = %+v", update)
		}
	}
	if got := history.read(&b).correction; got < 19 || got > 20 {
		t.Fatalf("converged correction = %d, want 19..20", got)
	}

	update := history.update(&b, 15, 0, 1000)
	if !update.saturated || update.direction != 1 {
		t.Fatalf("saturated update = %+v", update)
	}
	for range 200 {
		history.update(&b, 15, 0, -1000)
	}
	indices := correctionIndices(&b)
	for _, value := range []int16{
		history.pawn[indices.side][indices.pawn],
		history.minor[indices.side][indices.minor],
		history.nonPawn[indices.side][0][indices.whiteNonPawn],
		history.nonPawn[indices.side][1][indices.blackNonPawn],
	} {
		if int32(value) < -correctionHistoryMax || int32(value) > correctionHistoryMax {
			t.Fatalf("entry exceeded bound: %d", value)
		}
	}
}

func TestCorrectionHistoryUpdateGuards(t *testing.T) {
	valid := func(stopped, root, check, null bool, excluded gm.Move, legal int, score, corrected int32,
		bound int8, capture, promotion bool) bool {
		return correctionHistoryUpdateEligible(stopped, root, check, null, excluded, legal,
			score, corrected, bound, capture, promotion)
	}
	if !valid(false, false, false, false, 0, 1, 10, 0, ExactFlag, false, false) {
		t.Fatal("valid exact update was rejected")
	}
	for name, got := range map[string]bool{
		"stopped":     valid(true, false, false, false, 0, 1, 10, 0, ExactFlag, false, false),
		"root":        valid(false, true, false, false, 0, 1, 10, 0, ExactFlag, false, false),
		"check":       valid(false, false, true, false, 0, 1, 10, 0, ExactFlag, false, false),
		"null":        valid(false, false, false, true, 0, 1, 10, 0, ExactFlag, false, false),
		"excluded":    valid(false, false, false, false, gm.Move(1), 1, 10, 0, ExactFlag, false, false),
		"no moves":    valid(false, false, false, false, 0, 0, 10, 0, ExactFlag, false, false),
		"mate":        valid(false, false, false, false, 0, 1, Checkmate, 0, ExactFlag, false, false),
		"capture":     valid(false, false, false, false, 0, 1, 10, 0, ExactFlag, true, false),
		"promotion":   valid(false, false, false, false, 0, 1, 10, 0, ExactFlag, false, true),
		"lower wrong": valid(false, false, false, false, 0, 1, 0, 0, BetaFlag, false, false),
		"upper wrong": valid(false, false, false, false, 0, 1, 0, 0, AlphaFlag, false, false),
	} {
		if got {
			t.Fatalf("%s guard accepted update", name)
		}
	}
	if !valid(false, false, false, false, 0, 1, 1, 0, BetaFlag, false, false) {
		t.Fatal("supporting lower bound was rejected")
	}
	if !valid(false, false, false, false, 0, 1, -1, 0, AlphaFlag, false, false) {
		t.Fatal("supporting upper bound was rejected")
	}
}

func TestCorrectionHistoryGateAccounting(t *testing.T) {
	ResetCutStats()
	gates := [][2]*uint64{
		{&SearchState.cutStats.CorrectionImprovingEnabled, &SearchState.cutStats.CorrectionImprovingSuppressed},
		{&SearchState.cutStats.CorrectionRFPEnabled, &SearchState.cutStats.CorrectionRFPSuppressed},
		{&SearchState.cutStats.CorrectionNullMoveEnabled, &SearchState.cutStats.CorrectionNullMoveSuppressed},
		{&SearchState.cutStats.CorrectionRazoringEnabled, &SearchState.cutStats.CorrectionRazoringSuppressed},
		{&SearchState.cutStats.CorrectionFutilityEnabled, &SearchState.cutStats.CorrectionFutilitySuppressed},
		{&SearchState.cutStats.CorrectionCaptureFutilityEnabled, &SearchState.cutStats.CorrectionCaptureFutilitySuppressed},
	}
	for _, gate := range gates {
		recordCorrectionGate(false, true, gate[0], gate[1])
		recordCorrectionGate(true, false, gate[0], gate[1])
		recordCorrectionGate(false, false, gate[0], gate[1])
		if *gate[0] != 1 || *gate[1] != 1 {
			t.Fatalf("gate counts = %d/%d, want 1/1", *gate[0], *gate[1])
		}
	}
}

func TestCorrectionHistoryLifecycle(t *testing.T) {
	b := gm.ParseFen(gm.FENStartPos)
	SearchState.correction.clear()
	t.Cleanup(func() { SearchState.correction.clear() })
	seedCorrectionHistory(&SearchState.correction, &b, 1024)
	UpdateBetweenSearches()
	if SearchState.correction.read(&b).correction == 0 {
		t.Fatal("between-search maintenance cleared correction history")
	}
	ResetForNewGame()
	if SearchState.correction.read(&b).correction != 0 {
		t.Fatal("new-game reset retained correction history")
	}
}

func TestCorrectionHistoryKeepsTTStaticEvalRaw(t *testing.T) {
	b := gm.ParseFen(gm.FENStartPos)
	tt := newTestTT()
	prepareAlphaBetaTest(t, &b, tt)
	seedCorrectionHistory(&SearchState.correction, &b, 2560)
	raw := Evaluation(&b, false)

	var pv PVLine
	alphabeta(&b, -MaxScore, MaxScore, 1, 0, &pv, 0, false, false, 0, false, 0)
	entry, found := SearchState.tt.ProbeEntry(b.Hash())
	if !found || int32(entry.StaticEval) != raw {
		t.Fatalf("stored static eval = %d, found %v; want raw %d", entry.StaticEval, found, raw)
	}
	if SearchState.evalStack[0] != raw+10 {
		t.Fatalf("corrected eval stack = %d, want %d", SearchState.evalStack[0], raw+10)
	}
}

func TestCorrectionHistoryEnablesRFPBeforeTTRefinement(t *testing.T) {
	previousScale, previousMaxDepth := RFPScale, RFPMaxDepth
	t.Cleanup(func() {
		RFPScale = previousScale
		RFPMaxDepth = previousMaxDepth
	})
	RFPScale = 50
	RFPMaxDepth = 1

	b := gm.ParseFen(gm.FENStartPos)
	prepareAlphaBetaTest(t, &b, newTestTT())
	seedCorrectionHistory(&SearchState.correction, &b, int16(correctionHistoryMax))
	raw := Evaluation(&b, false)

	var pv PVLine
	got := alphabeta(&b, raw+15, raw+16, 1, 1, &pv, 0, false, false, 0, false, 0)
	if got != raw+32 {
		t.Fatalf("RFP score = %d, want %d", got, raw+32)
	}
	if SearchState.cutStats.CorrectionRFPEnabled != 1 || SearchState.cutStats.RFPRefinements != 0 || SearchState.cutStats.RFPCutoffs != 1 {
		t.Fatalf("correction/TT/cutoff counts = %d/%d/%d, want 1/0/1",
			SearchState.cutStats.CorrectionRFPEnabled,
			SearchState.cutStats.RFPRefinements,
			SearchState.cutStats.RFPCutoffs)
	}
}

func TestCorrectionHistoryDoesNotChangeQuiescenceStandPat(t *testing.T) {
	b := gm.ParseFen("7k/8/8/8/8/8/8/K7 w - - 0 1")
	prepareQuiescenceTest(t, &b, newTestTT())
	seedCorrectionHistory(&SearchState.correction, &b, int16(correctionHistoryMax))
	raw := Evaluation(&b, false)

	var pv PVLine
	if got := quiescence(&b, -MaxScore, MaxScore, &pv, 30, 0, -1); got != raw {
		t.Fatalf("qsearch score = %d, want raw stand-pat %d", got, raw)
	}
}
