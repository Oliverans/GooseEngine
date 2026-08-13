package engine

import (
	"bytes"
	"encoding/json"
	"math/rand"
	"reflect"
	"testing"

	gm "chess-engine/goosemg"
)

func TestTuningTraceReconstructsRandomLegalPositions(t *testing.T) {
	rng := rand.New(rand.NewSource(0x6f6f7365))
	board := gm.ParseFen(gm.Startpos)
	for i := 0; i < 500; i++ {
		trace := TuningTraceForBoard(&board)
		score, buckets := ScoreTuningTraceCurrent(trace)
		if buckets != trace.Reference.Buckets || score != trace.Reference.SideToMove {
			t.Fatalf("position %d (%s): reconstructed score/buckets %d/%+v, want %d/%+v",
				i, board.ToFen(), score, buckets, trace.Reference.SideToMove, trace.Reference.Buckets)
		}
		moves := board.GenerateLegalMoves()
		if len(moves) == 0 || i%80 == 79 {
			board = gm.ParseFen(gm.Startpos)
			continue
		}
		move := moves[rng.Intn(len(moves))]
		if ok, _ := board.MakeMove(move); !ok {
			t.Fatalf("generated legal move %s was rejected", move.String())
		}
	}
}

func TestTuningTraceReconstructsCurrentEvaluation(t *testing.T) {
	fens := []string{
		gm.Startpos,
		"8/8/8/8/8/8/8/K6k w - - 0 1",
		"8/8/3p4/4P3/8/8/8/K6k w - - 0 1",
		"r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1",
		"r1bq1rk1/ppp2ppp/2nb1n2/3pp3/2B1P3/2NP1N2/PPP2PPP/R1BQ1RK1 w - - 0 1",
		"7k/8/p7/8/8/P7/P7/K7 w - - 0 1",
		"7k/4p3/3p4/2p5/2P5/3P4/8/K7 w - - 0 1",
		"7k/8/8/8/2P5/1P6/8/K7 w - - 0 1",
		"7k/8/3p4/2P1P3/8/8/8/K7 w - - 0 1",
		"4r3/5pkp/2b3p1/1p6/8/2p1nPP1/PPN4P/1R2R2K b - - 3 25",
		"7k/8/RR6/N7/R7/8/8/4K3 w - - 0 1",
		"r5k1/1pp2ppp/8/8/8/8/1PP2PPP/R5KR w - - 0 1",
		"6k1/8/4N3/8/8/8/Q7/K7 w - - 0 1",
		"3r2k1/8/8/8/8/2N5/8/4K3 w - - 0 1",
		"k7/8/8/8/8/6p1/6P1/6K1 w - - 0 1",
		"5rk1/5ppp/8/7P/8/8/5PP1/5RK1 w - - 0 1",
		"8/8/4P3/8/8/8/8/K5k1 w - - 0 1",
		"4k3/8/8/8/8/8/8/1N2K1N1 w - - 0 1",
		"8/8/8/8/8/8/8/K5Qk w - - 0 1",
	}

	for _, fen := range fens {
		t.Run(fen, func(t *testing.T) {
			board := gm.ParseFen(fen)
			trace := TuningTraceForBoard(&board)
			score, buckets := ScoreTuningTraceCurrent(trace)
			if buckets != trace.Reference.Buckets {
				eval := EvalTraceForBoard(&board)
				t.Logf("eval pawn=%v knight=%v bishop=%v rook=%v queen=%v king=%v space=%+v imbalance=%+v tempo=%d fixed=%+v", eval.Pawn.Terms, eval.Knight.Terms, eval.Bishop.Terms, eval.Rook.Terms, eval.Queen.Terms, eval.King.Terms, eval.Space, eval.Imbalance, eval.Tempo, trace.Fixed)
				t.Logf("units=%+v", trace.Units)
				t.Fatalf("bucket mismatch: reconstructed %+v, evaluator %+v", buckets, trace.Reference.Buckets)
			}
			if score != trace.Reference.SideToMove {
				t.Fatalf("score mismatch: reconstructed %d, evaluator %d", score, trace.Reference.SideToMove)
			}
			if got := Evaluation(&board, false); got != score {
				t.Fatalf("fast evaluation %d, reconstructed %d", got, score)
			}
		})
	}
}

func TestTuningTracePawnHashHistoryIndependent(t *testing.T) {
	const open = "4k3/8/8/8/8/3N4/4P3/4K3 w - - 0 1"
	const occupied = "4k3/8/8/8/8/8/4P3/3NK3 w - - 0 1"
	clean := func(fen string) TuningTrace {
		ClearPawnHash()
		board := gm.ParseFen(fen)
		return TuningTraceForBoard(&board)
	}
	wantOpen, wantOccupied := clean(open), clean(occupied)

	ClearPawnHash()
	first := gm.ParseFen(occupied)
	_ = TuningTraceForBoard(&first)
	second := gm.ParseFen(open)
	if got := TuningTraceForBoard(&second); !reflect.DeepEqual(got, wantOpen) {
		t.Fatal("open trace depends on prior same-pawn-key position")
	}

	ClearPawnHash()
	first = gm.ParseFen(open)
	_ = TuningTraceForBoard(&first)
	second = gm.ParseFen(occupied)
	if got := TuningTraceForBoard(&second); !reflect.DeepEqual(got, wantOccupied) {
		t.Fatal("occupied trace depends on prior same-pawn-key position")
	}
}

func TestTuningTraceDoesNotMutateBoardOrFastEvaluation(t *testing.T) {
	const fen = "r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1"
	board := gm.ParseFen(fen)
	wantFEN := board.ToFen()
	wantEval := Evaluation(&board, false)
	_ = TuningTraceForBoard(&board)
	if got := board.ToFen(); got != wantFEN {
		t.Fatalf("tuning trace mutated board:\n got  %s\n want %s", got, wantFEN)
	}
	if got := Evaluation(&board, false); got != wantEval {
		t.Fatalf("evaluation changed after trace: got %d, want %d", got, wantEval)
	}
}

func TestEvalTraceRookMobilityUsesEvaluationOccupancy(t *testing.T) {
	board := gm.ParseFen("7k/8/RR6/N7/R7/8/8/4K3 w - - 0 1")
	trace := EvalTraceForBoard(&board)
	sum := EvalPair{}
	for name, term := range trace.Rook.Terms {
		if name != "total" {
			sum.MG += term.MG
			sum.EG += term.EG
		}
	}
	if sum != trace.Rook.Total {
		t.Fatalf("rook trace terms %+v do not sum to evaluator total %+v", sum, trace.Rook.Total)
	}
}

func TestRenderTuningTraceJSONRoundTrip(t *testing.T) {
	board := gm.ParseFen("r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1")
	want := TuningTraceForBoard(&board)
	var buf bytes.Buffer
	if err := RenderTuningTraceJSON(&buf, want); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(buf.Bytes(), []byte("\n  \"")) {
		t.Fatal("tuning JSON should be compact rather than indented")
	}
	var got TuningTrace
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip mismatch:\n got  %#v\n want %#v", got, want)
	}
	if got.SchemaVersion != tuningTraceSchema {
		t.Fatalf("schema version %d, want %d", got.SchemaVersion, tuningTraceSchema)
	}
}

func TestTuningTraceContainsNonlinearPrimitives(t *testing.T) {
	board := gm.ParseFen("r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1")
	trace := TuningTraceForBoard(&board)
	if trace.Units.Center.Openness < -4 || trace.Units.Center.Openness > 4 {
		t.Fatalf("center openness out of range: %d", trace.Units.Center.Openness)
	}
	if trace.Units.Danger.White.Attackers == [4]int{} && trace.Units.Danger.Black.Attackers == [4]int{} {
		t.Fatal("expected king-danger attacker primitives")
	}
	if trace.Units.Space.White.PieceCount == 0 || trace.Units.Space.Black.PieceCount == 0 {
		t.Fatal("space material weights are missing")
	}
	visits := 0
	for _, row := range trace.Units.ShelterStorm.Shelter {
		for _, n := range row {
			visits += absInt(n)
		}
	}
	if visits == 0 {
		t.Fatal("shelter table visits are missing")
	}
}
