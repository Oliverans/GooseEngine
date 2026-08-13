package tuner

import (
	"math/rand"
	"strings"
	"testing"

	eng "chess-engine/engine"
	gm "chess-engine/goosemg"
)

func TestTraceBindingHasCompleteRegistryAndSchemaCoverage(t *testing.T) {
	registry, err := NewEngineRegistry()
	if err != nil {
		t.Fatalf("NewEngineRegistry() error = %v", err)
	}
	binding, err := NewTraceBinding(registry)
	if err != nil {
		t.Fatalf("NewTraceBinding() error = %v", err)
	}
	if binding.RegistryFingerprint != registry.Fingerprint {
		t.Fatalf("binding fingerprint = %q, want %q", binding.RegistryFingerprint, registry.Fingerprint)
	}

	fields := binding.FieldBindings()
	if len(fields) == 0 {
		t.Fatal("trace field coverage manifest is empty")
	}
	counts := map[TraceFieldUse]int{}
	previous := ""
	for _, field := range fields {
		if field.Path <= previous {
			t.Fatalf("trace fields are not uniquely sorted: %q after %q", field.Path, previous)
		}
		previous = field.Path
		counts[field.Use]++
	}
	if counts[TraceFormulaInput] == 0 || counts[TraceForwardInput] == 0 || counts[TraceDiagnostic] == 0 {
		t.Fatalf("trace field dispositions are incomplete: %v", counts)
	}
}

func TestTraceBindingRejectsUnownedRegistryParameter(t *testing.T) {
	registry, err := NewEngineRegistry()
	if err != nil {
		t.Fatalf("NewEngineRegistry() error = %v", err)
	}
	extra := registry.Specs[0]
	extra.ID = "test.unowned"
	extra.EngineName = "TestUnowned"
	extra.Export.GoSymbol = "TestUnowned"
	withExtra, err := NewRegistry(registry.Version, registry.Groups, append(registry.Specs, extra))
	if err != nil {
		t.Fatalf("NewRegistry(with extra) error = %v", err)
	}

	_, err = NewTraceBinding(withExtra)
	if err == nil || !strings.Contains(err.Error(), "without trace bindings") {
		t.Fatalf("NewTraceBinding() error = %v, want unowned-registry error", err)
	}
}

func TestTraceBindingRejectsFormulaMismatch(t *testing.T) {
	registry, err := NewEngineRegistry()
	if err != nil {
		t.Fatalf("NewEngineRegistry() error = %v", err)
	}
	specs := cloneSpecs(registry.Specs)
	for i := range specs {
		if specs[i].ID == "bishop.pair.mg" {
			specs[i].Formula = FormulaLinear
			break
		}
	}
	changed, err := NewRegistry(registry.Version, registry.Groups, specs)
	if err != nil {
		t.Fatalf("NewRegistry(changed formula) error = %v", err)
	}

	_, err = NewTraceBinding(changed)
	if err == nil || !strings.Contains(err.Error(), "has formula") {
		t.Fatalf("NewTraceBinding() error = %v, want formula mismatch", err)
	}
}

func TestBindTraceCompilesLinearAndNonlinearInputs(t *testing.T) {
	registry, err := NewEngineRegistry()
	if err != nil {
		t.Fatalf("NewEngineRegistry() error = %v", err)
	}
	binding, err := NewTraceBinding(registry)
	if err != nil {
		t.Fatalf("NewTraceBinding() error = %v", err)
	}

	trace := eng.TuningTrace{
		SchemaVersion: eng.TuningTraceSchemaVersion,
		FEN:           "test",
		SideToMove:    1,
		PiecePhase:    12,
		TotalPhase:    24,
		Fixed:         eng.EvalPair{MG: 3, EG: -2},
	}
	trace.Units.Material[gm.PieceTypePawn] = 2
	trace.Units.Pawn.IsolatedOpposed = -1
	trace.Units.Mobility.Rook[3] = 4
	trace.Units.Piece.BishopPair = 1
	trace.Units.Center.Openness = -2
	trace.Units.Tempo = 1
	trace.Units.Pawn.CandidatePassers = []eng.TuningCandidatePasser{
		{Side: 1, Source: 16, Targets: []int{25, 27}},
	}

	bound, err := binding.BindTrace(trace)
	if err != nil {
		t.Fatalf("BindTrace() error = %v", err)
	}
	if bound.Fixed != trace.Fixed || bound.PiecePhase != 12 || bound.TotalPhase != 24 {
		t.Fatalf("bound metadata = fixed %+v phase %d/%d", bound.Fixed, bound.PiecePhase, bound.TotalPhase)
	}
	if bound.Nonlinear.CenterOpenness != -2 || bound.Nonlinear.BishopPair != 1 {
		t.Fatalf("bound center inputs = openness %d bishop pair %d", bound.Nonlinear.CenterOpenness, bound.Nonlinear.BishopPair)
	}
	if len(bound.Nonlinear.CandidatePassers) != 1 {
		t.Fatalf("candidate passer count = %d, want 1", len(bound.Nonlinear.CandidatePassers))
	}
	firstTarget := bound.Nonlinear.CandidatePassers[0].Targets[0]
	wantMG := binding.Pawn.Passed.MG.MustIndex(25/8-1, 25%8)
	wantEG := binding.Pawn.Passed.EG.MustIndex(25/8-1, 25%8)
	if firstTarget.MGParameterIndex != wantMG || firstTarget.EGParameterIndex != wantEG {
		t.Fatalf("bound candidate target = %+v, want MG/EG indexes %d/%d", firstTarget, wantMG, wantEG)
	}
	trace.Units.Pawn.CandidatePassers[0].Targets[0] = 40
	if got := bound.Nonlinear.CandidatePassers[0].Targets[0]; got != firstTarget {
		t.Fatalf("bound candidate targets alias input trace: got %+v, want %+v", got, firstTarget)
	}

	mg := linearUnitsByIndex(bound.LinearMG)
	eg := linearUnitsByIndex(bound.LinearEG)
	if got := mg[binding.Material[gm.PieceTypePawn].MG.Offset]; got != 2 {
		t.Errorf("material MG units = %d, want 2", got)
	}
	if got := eg[binding.Material[gm.PieceTypePawn].EG.Offset]; got != 2 {
		t.Errorf("material EG units = %d, want 2", got)
	}
	if got := mg[binding.Pawn.IsolatedOpposed.MG.Offset]; got != -1 {
		t.Errorf("isolated MG units = %d, want -1", got)
	}
	if got := mg[binding.Mobility.Rook.MG.MustIndex(3)]; got != 4 {
		t.Errorf("rook mobility MG units = %d, want 4", got)
	}
	if _, exists := mg[binding.Piece.BishopPair.MG.Offset]; exists {
		t.Error("center-scaled bishop pair was incorrectly compiled as linear")
	}
	if got := mg[binding.Tempo.Offset]; got != 1 {
		t.Errorf("tempo MG units = %d, want 1", got)
	}
	if got := eg[binding.Tempo.Offset]; got != 1 {
		t.Errorf("tempo EG units = %d, want 1", got)
	}
	for _, term := range append(append([]BoundLinearTerm(nil), bound.LinearMG...), bound.LinearEG...) {
		if term.Units == 0 {
			t.Error("bound linear list contains a zero-unit term")
		}
		if term.ParameterIndex < 0 || term.ParameterIndex >= len(registry.Elements) {
			t.Errorf("bound parameter index %d outside registry vector", term.ParameterIndex)
		}
	}
}

func TestBindTraceAcceptsRandomLegalEngineTraces(t *testing.T) {
	registry, err := NewEngineRegistry()
	if err != nil {
		t.Fatalf("NewEngineRegistry() error = %v", err)
	}
	binding, err := NewTraceBinding(registry)
	if err != nil {
		t.Fatalf("NewTraceBinding() error = %v", err)
	}

	rng := rand.New(rand.NewSource(0x7472616365))
	board := gm.ParseFen(gm.Startpos)
	for i := 0; i < 300; i++ {
		trace := eng.TuningTraceForBoard(&board)
		bound, err := binding.BindTrace(trace)
		if err != nil {
			t.Fatalf("position %d (%s): BindTrace() error = %v", i, board.ToFen(), err)
		}
		if bound.Reference != trace.Reference {
			t.Fatalf("position %d: reference checksum changed while binding", i)
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

func TestBindTraceRejectsInvalidSchemaAndSentinels(t *testing.T) {
	registry, err := NewEngineRegistry()
	if err != nil {
		t.Fatalf("NewEngineRegistry() error = %v", err)
	}
	binding, err := NewTraceBinding(registry)
	if err != nil {
		t.Fatalf("NewTraceBinding() error = %v", err)
	}
	base := eng.TuningTrace{
		SchemaVersion: eng.TuningTraceSchemaVersion,
		SideToMove:    1,
		PiecePhase:    0,
		TotalPhase:    24,
	}

	badSchema := base
	badSchema.SchemaVersion++
	if _, err := binding.BindTrace(badSchema); err == nil || !strings.Contains(err.Error(), "schema") {
		t.Fatalf("bad schema error = %v", err)
	}

	badShelter := base
	badShelter.Units.ShelterStorm.Shelter[0][7] = 1
	if _, err := binding.BindTrace(badShelter); err == nil || !strings.Contains(err.Error(), "shelter rank seven") {
		t.Fatalf("bad shelter error = %v", err)
	}

	badConnected := base
	badConnected.Units.Pawn.Connected.White[0] = 1
	if _, err := binding.BindTrace(badConnected); err == nil || !strings.Contains(err.Error(), "rank zero") {
		t.Fatalf("bad connected error = %v", err)
	}

	badPassed := base
	badPassed.Units.Pawn.Passed = []eng.TuningIndexedUnit{{Index: 0, Units: 1}}
	if _, err := binding.BindTrace(badPassed); err == nil || !strings.Contains(err.Error(), "passed-pawn") {
		t.Fatalf("bad passed-pawn error = %v", err)
	}
}

func linearUnitsByIndex(terms []BoundLinearTerm) map[int]int {
	out := make(map[int]int, len(terms))
	for _, term := range terms {
		out[term.ParameterIndex] += term.Units
	}
	return out
}
