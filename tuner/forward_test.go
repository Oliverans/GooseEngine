package tuner

import (
	"math"
	"math/rand"
	"strings"
	"testing"

	eng "chess-engine/engine"
	gm "chess-engine/goosemg"
)

func TestEngineExactForwardMatchesEngineOnRandomLegalPositions(t *testing.T) {
	registry, binding, model := newForwardTestSystem(t)
	parameters, err := InitialExactParameters(registry)
	if err != nil {
		t.Fatalf("InitialExactParameters() error = %v", err)
	}
	continuousParameters := registry.InitialValues()
	if err := model.ValidateContinuousParameters(continuousParameters); err != nil {
		t.Fatalf("ValidateContinuousParameters() error = %v", err)
	}

	rng := rand.New(rand.NewSource(0x666f7277617264))
	board := gm.ParseFen(gm.Startpos)
	for i := 0; i < 500; i++ {
		trace := eng.TuningTraceForBoard(&board)
		bound, err := binding.BindTrace(trace)
		if err != nil {
			t.Fatalf("position %d (%s): BindTrace() error = %v", i, board.ToFen(), err)
		}
		got, err := model.EngineExact(bound, parameters)
		if err != nil {
			t.Fatalf("position %d (%s): EngineExact() error = %v", i, board.ToFen(), err)
		}
		if got.Buckets != trace.Reference.Buckets ||
			got.WhitePerspective != trace.Reference.WhitePerspective ||
			got.SideToMove != trace.Reference.SideToMove {
			t.Fatalf(
				"position %d (%s): exact result %+v, want buckets=%+v white=%d stm=%d",
				i,
				board.ToFen(),
				got,
				trace.Reference.Buckets,
				trace.Reference.WhitePerspective,
				trace.Reference.SideToMove,
			)
		}
		if _, err := model.Continuous(bound, continuousParameters); err != nil {
			t.Fatalf("position %d (%s): Continuous() error = %v", i, board.ToFen(), err)
		}

		moves := board.GenerateLegalMoves()
		if len(moves) == 0 || i%80 == 79 {
			board = gm.ParseFen(gm.Startpos)
			continue
		}
		move := moves[rng.Intn(len(moves))]
		if ok, _ := board.MakeMove(move); !ok {
			t.Fatalf("position %d: generated legal move %s was rejected", i, move.String())
		}
	}
}

func TestEngineExactForwardMatchesCuratedNonlinearPositions(t *testing.T) {
	registry, binding, model := newForwardTestSystem(t)
	parameters, err := InitialExactParameters(registry)
	if err != nil {
		t.Fatalf("InitialExactParameters() error = %v", err)
	}
	fens := []string{
		"8/8/8/8/8/8/8/K6k w - - 0 1",
		"Qnk4r/1pprq1np/8/1P2p3/1bNpp1p1/1Q1P2P1/1B2PP1P/R4RK1 b - - 0 23",
		"r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1",
		"4r3/5pkp/2b3p1/1p6/8/2p1nPP1/PPN4P/1R2R2K b - - 3 25",
		"7k/8/8/8/2P5/1P6/8/K7 w - - 0 1",
		"7k/8/3p4/2P1P3/8/8/8/K7 w - - 0 1",
		"5rk1/5ppp/8/7P/8/8/5PP1/5RK1 w - - 0 1",
		"8/8/4P3/8/8/8/8/K5k1 w - - 0 1",
	}
	for _, fen := range fens {
		t.Run(fen, func(t *testing.T) {
			board := gm.ParseFen(fen)
			trace := eng.TuningTraceForBoard(&board)
			bound, err := binding.BindTrace(trace)
			if err != nil {
				t.Fatalf("BindTrace() error = %v", err)
			}
			got, err := model.EngineExact(bound, parameters)
			if err != nil {
				t.Fatalf("EngineExact() error = %v", err)
			}
			if got.Buckets != trace.Reference.Buckets || got.SideToMove != trace.Reference.SideToMove {
				t.Fatalf("exact result %+v, want buckets %+v score %d", got, trace.Reference.Buckets, trace.Reference.SideToMove)
			}
		})
	}
}

func TestContinuousForwardUsesFractionalParametersWithoutRounding(t *testing.T) {
	registry, binding, model := newForwardTestSystem(t)
	parameters := registry.InitialValues()

	trace := eng.TuningTrace{
		SchemaVersion: eng.TuningTraceSchemaVersion,
		SideToMove:    1,
		PiecePhase:    24,
		TotalPhase:    24,
	}
	trace.Units.Material[gm.PieceTypePawn] = 1
	bound, err := binding.BindTrace(trace)
	if err != nil {
		t.Fatalf("BindTrace() error = %v", err)
	}
	pawnMG := binding.Material[gm.PieceTypePawn].MG.Offset
	parameters[pawnMG] += 0.5

	got, err := model.Continuous(bound, parameters)
	if err != nil {
		t.Fatalf("Continuous() error = %v", err)
	}
	want := parameters[pawnMG]
	if got.MG != want || got.WhitePerspective != want || got.SideToMove != want {
		t.Fatalf("continuous result %+v, want full-MG score %.2f", got, want)
	}
}

func TestContinuousForwardRemovesIntermediateIntegerTruncation(t *testing.T) {
	registry, binding, model := newForwardTestSystem(t)
	floatParameters := registry.InitialValues()
	exactParameters, err := InitialExactParameters(registry)
	if err != nil {
		t.Fatalf("InitialExactParameters() error = %v", err)
	}

	trace := eng.TuningTrace{
		SchemaVersion: eng.TuningTraceSchemaVersion,
		SideToMove:    1,
		PiecePhase:    0,
		TotalPhase:    24,
	}
	trace.Units.Pawn.Connected.White[3] = 1
	bound, err := binding.BindTrace(trace)
	if err != nil {
		t.Fatalf("BindTrace() error = %v", err)
	}
	exact, err := model.EngineExact(bound, exactParameters)
	if err != nil {
		t.Fatalf("EngineExact() error = %v", err)
	}
	continuous, err := model.Continuous(bound, floatParameters)
	if err != nil {
		t.Fatalf("Continuous() error = %v", err)
	}

	connected := floatParameters[binding.Pawn.Connected.MustIndex(2)]
	wantContinuousEG := connected / 4
	if continuous.EG != wantContinuousEG {
		t.Fatalf("continuous connected EG = %v, want %v", continuous.EG, wantContinuousEG)
	}
	if float64(exact.Buckets.EG) == continuous.EG {
		t.Fatalf("exact and continuous EG unexpectedly equal at truncation boundary: %v", continuous.EG)
	}
}

func TestForwardModelsRejectInvalidInputs(t *testing.T) {
	registry, binding, model := newForwardTestSystem(t)
	exact, err := InitialExactParameters(registry)
	if err != nil {
		t.Fatalf("InitialExactParameters() error = %v", err)
	}
	continuous := registry.InitialValues()
	trace := BoundTrace{
		SchemaVersion: eng.TuningTraceSchemaVersion,
		SideToMove:    1,
		TotalPhase:    24,
	}

	if _, err := model.EngineExact(trace, exact[:len(exact)-1]); err == nil || !strings.Contains(err.Error(), "length") {
		t.Fatalf("short exact vector error = %v", err)
	}
	badExact := append([]int(nil), exact...)
	badExact[binding.Space.WeightDivisor.Offset] = 0
	if _, err := model.EngineExact(trace, badExact); err == nil || !strings.Contains(err.Error(), "space divisor") {
		t.Fatalf("zero exact divisor error = %v", err)
	}

	badContinuous := append([]float64(nil), continuous...)
	badContinuous[binding.Pawn.Connected.Offset] = math.NaN()
	if err := model.ValidateContinuousParameters(badContinuous); err == nil || !strings.Contains(err.Error(), "not finite") {
		t.Fatalf("NaN continuous parameter validation error = %v", err)
	}
	badContinuous = append([]float64(nil), continuous...)
	badContinuous[binding.Danger.Divisor.MG.Offset] = 0
	if _, err := model.Continuous(trace, badContinuous); err == nil || !strings.Contains(err.Error(), "king danger MG divisor") {
		t.Fatalf("zero continuous divisor error = %v", err)
	}
}

func TestForwardModelRejectsBindingFromDifferentRegistry(t *testing.T) {
	registry, err := NewEngineRegistry()
	if err != nil {
		t.Fatalf("NewEngineRegistry() error = %v", err)
	}
	binding, err := NewTraceBinding(registry)
	if err != nil {
		t.Fatalf("NewTraceBinding() error = %v", err)
	}
	other, err := NewRegistry(registry.Version+"-different", registry.Groups, registry.Specs)
	if err != nil {
		t.Fatalf("NewRegistry(other) error = %v", err)
	}
	if _, err := NewForwardModel(other, binding); err == nil || !strings.Contains(err.Error(), "fingerprint") {
		t.Fatalf("mismatched NewForwardModel() error = %v", err)
	}
}

func newForwardTestSystem(t *testing.T) (*Registry, *TraceBinding, *ForwardModel) {
	t.Helper()
	// Forward and gradient tests exercise every structurally eligible formula,
	// independently of the narrower production-stage optimizer policy.
	registry, err := newEngineRegistryWithPolicy(TrainingPolicy{})
	if err != nil {
		t.Fatalf("newEngineRegistryWithPolicy() error = %v", err)
	}
	binding, err := NewTraceBinding(registry)
	if err != nil {
		t.Fatalf("NewTraceBinding() error = %v", err)
	}
	model, err := NewForwardModel(registry, binding)
	if err != nil {
		t.Fatalf("NewForwardModel() error = %v", err)
	}
	return registry, binding, model
}
