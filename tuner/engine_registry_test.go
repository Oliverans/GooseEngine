package tuner

import (
	"strings"
	"testing"

	eng "chess-engine/engine"
	gm "chess-engine/goosemg"
)

func TestEngineRegistryInventory(t *testing.T) {
	registry, err := NewEngineRegistry()
	if err != nil {
		t.Fatalf("NewEngineRegistry() error = %v", err)
	}

	if got, want := len(registry.Specs), 126; got != want {
		t.Fatalf("spec count = %d, want %d", got, want)
	}
	if got, want := len(registry.Elements), 1131; got != want {
		t.Fatalf("active element count = %d, want %d", got, want)
	}
	const wantFingerprint = "890ca7ff1ddeed35b719c018175304f62c95876c1931fd9af0066ec45c50a3e4"
	if registry.Fingerprint != wantFingerprint {
		t.Fatalf("layout fingerprint = %s, want %s; bump EngineRegistryVersion if this layout change is intentional", registry.Fingerprint, wantFingerprint)
	}

	wantGroups := map[GroupID]int{
		groupMaterial:      10,
		groupPSQT:          14,
		groupMobility:      8,
		groupPawnStructure: 19,
		groupCenter:        6,
		groupPieceActivity: 10,
		groupRook:          12,
		groupKingSafety:    31,
		groupKingPasser:    4,
		groupSpace:         7,
		groupImbalance:     3,
		groupTempo:         1,
		groupFinalScale:    1,
	}
	gotGroups := make(map[GroupID]int, len(wantGroups))
	for _, spec := range registry.Specs {
		gotGroups[spec.Group]++
	}
	for group, want := range wantGroups {
		if got := gotGroups[group]; got != want {
			t.Errorf("group %q spec count = %d, want %d", group, got, want)
		}
	}
	if len(gotGroups) != len(wantGroups) {
		t.Errorf("group count = %d, want %d: %v", len(gotGroups), len(wantGroups), gotGroups)
	}

	for _, element := range registry.Elements {
		if element.Initial != element.Anchor {
			t.Errorf("element %q%v initial %v != anchor %v", element.ID, element.Coordinate, element.Initial, element.Anchor)
		}
		if element.DeviationScale <= 0 {
			t.Errorf("element %q%v has non-positive deviation scale %v", element.ID, element.Coordinate, element.DeviationScale)
		}
		if element.AnchorStrength != 0 {
			t.Errorf("element %q%v anchor strength = %v, want provisional 0", element.ID, element.Coordinate, element.AnchorStrength)
		}
	}

}

// TestEngineRegistryStage4TrainingPolicy records the next cumulative ownership
// transition before engineTrainingPolicy enables it. The Stage 4 test is
// intentionally RED while the Stage 3 policy remains in force.
func TestEngineRegistryStage4TrainingPolicy(t *testing.T) {
	registry, err := NewEngineRegistry()
	if err != nil {
		t.Fatalf("NewEngineRegistry() error = %v", err)
	}

	if got, want := registry.TrainableCount(), 1026; got != want {
		t.Errorf("trainable count = %d, want %d", got, want)
	}
	if got, want := len(registry.Elements)-registry.TrainableCount(), 105; got != want {
		t.Errorf("frozen count = %d, want %d", got, want)
	}
	if got, want := len(registry.Elements), 1131; got != want {
		t.Errorf("element count = %d, want %d", got, want)
	}

	stage4Groups := map[GroupID]bool{
		groupMaterial:      true,
		groupPSQT:          true,
		groupMobility:      true,
		groupPawnStructure: true,
		groupKingPasser:    true,
		groupCenter:        true,
		groupPieceActivity: true,
		groupRook:          true,
		groupSpace:         true,
		groupImbalance:     true,
		groupTempo:         true,
	}
	wantTrainableByGroup := map[GroupID]int{
		groupMaterial:      10,
		groupPSQT:          832,
		groupMobility:      120,
		groupPawnStructure: 25,
		groupKingPasser:    3,
		groupCenter:        6,
		groupPieceActivity: 10,
		groupRook:          12,
		groupSpace:         5,
		groupImbalance:     2,
		groupTempo:         1,
	}
	gotTrainableByGroup := make(map[GroupID]int, len(wantTrainableByGroup))
	kingSafetyFrozen := 0
	for _, element := range registry.Elements {
		group := registry.Specs[element.SpecIndex].Group
		if stage4Groups[group] && !stage4ProtectedFrozen(element, group) {
			if element.Mode != TrainingContinuous || element.TrainIndex == NoTrainIndex {
				t.Errorf("Stage 4 group %q element %q%v = mode %q train index %d, want continuous", group, element.ID, element.CoordinateLabels, element.Mode, element.TrainIndex)
			}
			if element.Mode == TrainingContinuous && element.TrainIndex != NoTrainIndex {
				gotTrainableByGroup[group]++
			}
			continue
		}
		if element.Mode != TrainingFrozen || element.TrainIndex != NoTrainIndex {
			t.Errorf("Stage 4 protected/frozen group %q element %q%v = mode %q train index %d, want frozen/%d", group, element.ID, element.CoordinateLabels, element.Mode, element.TrainIndex, NoTrainIndex)
		}
		if group == groupKingSafety && element.Mode == TrainingFrozen && element.TrainIndex == NoTrainIndex {
			kingSafetyFrozen++
		}
	}
	for group, want := range wantTrainableByGroup {
		if got := gotTrainableByGroup[group]; got != want {
			t.Errorf("Stage 4 trainable group %q count = %d, want %d", group, got, want)
		}
	}
	if got, want := kingSafetyFrozen, 99; got != want {
		t.Errorf("Stage 4 king-safety frozen element count = %d, want %d", got, want)
	}

	connected := mustResolve(t, registry, "pawn.connected.mg")
	if element := registry.Elements[connected.MustIndex(0)]; element.Mode != TrainingFrozen || element.TrainIndex != NoTrainIndex {
		t.Errorf("Stage 4 pawn.connected.mg relativeRank=1 = mode %q train index %d, want frozen/%d", element.Mode, element.TrainIndex, NoTrainIndex)
	}
	kingPasserDivisor := mustResolve(t, registry, "king.passer.divisor")
	if element := registry.Elements[kingPasserDivisor.Offset]; element.Mode != TrainingFrozen || element.TrainIndex != NoTrainIndex {
		t.Errorf("Stage 4 king.passer.divisor = mode %q train index %d, want frozen/%d", element.Mode, element.TrainIndex, NoTrainIndex)
	}
}

func stage4ProtectedFrozen(element ParameterElement, group GroupID) bool {
	if group == groupKingSafety || group == groupFinalScale {
		return true
	}
	if element.ID == "pawn.connected.mg" && element.CoordinateLabels[0] == "1" {
		return true
	}
	switch element.ID {
	case "king.passer.divisor", "space.blocked_cap", "space.weight_divisor", "imbalance.reference_pawn_count":
		return true
	default:
		return false
	}
}

func TestEngineRegistrySensitiveControlsStartFrozen(t *testing.T) {
	registry, err := NewEngineRegistry()
	if err != nil {
		t.Fatalf("NewEngineRegistry() error = %v", err)
	}

	wantFrozen := map[ParameterID]bool{
		"final.draw_divider":             true,
		"imbalance.reference_pawn_count": true,
		"king.danger.divisor.eg":         true,
		"king.danger.divisor.mg":         true,
		"king.passer.divisor":            true,
		"space.blocked_cap":              true,
		"space.weight_divisor":           true,
	}
	gotFrozen := make(map[ParameterID]bool)
	for _, element := range registry.Elements {
		if element.Mode == TrainingFrozen {
			gotFrozen[element.ID] = true
			if element.TrainIndex != NoTrainIndex {
				t.Errorf("frozen element %q has train index %d", element.ID, element.TrainIndex)
			}
		}
		if element.Mode == TrainingDiscrete {
			t.Errorf("engine baseline unexpectedly classifies %q as discrete", element.ID)
		}
	}
	for id := range wantFrozen {
		if !gotFrozen[id] {
			t.Errorf("sensitive parameter %q is not frozen", id)
		}
	}
}

func TestEngineRegistryOmitsRetiredParameters(t *testing.T) {
	registry, err := NewEngineRegistry()
	if err != nil {
		t.Fatalf("NewEngineRegistry() error = %v", err)
	}

	retiredEngineNames := []string{
		"KingSafetyTable",
		"PawnStormBaseMG",
		"PawnStormFreePct",
		"PawnStormLeverPct",
		"PawnStormWeakLeverPct",
		"PawnStormBlockedPct",
		"PawnStormOppositeMultiplier",
		"IsolatedPawnMG",
		"IsolatedPawnEG",
		"BackwardPawnMG",
		"BackwardPawnEG",
		"DoubledPawnPenaltyMG",
		"DoubledPawnPenaltyEG",
		"ConnectedPawnsBonusMG",
		"ConnectedPawnsBonusEG",
		"PhalanxPawnsBonusMG",
		"PhalanxPawnsBonusEG",
		"BlockedPawnBonusMG",
		"BlockedPawnBonusEG",
		"KingSemiOpenFilePenalty",
		"KingOpenFilePenalty",
		"KingPawnDefenseMG",
		"SpaceBonusMG",
		"SpaceBonusEG",
		"WeakKingSquarePenaltyMG",
		"ImbalanceBishopPerPawnMG",
		"ImbalanceBishopPerPawnEG",
	}
	for _, spec := range registry.Specs {
		for _, retired := range retiredEngineNames {
			if spec.EngineName == retired || strings.HasPrefix(spec.EngineName, retired+"[") {
				t.Errorf("retired engine parameter %q registered as %q", retired, spec.ID)
			}
		}
	}
}

func TestEngineRegistryActiveSubshapes(t *testing.T) {
	registry, err := NewEngineRegistry()
	if err != nil {
		t.Fatalf("NewEngineRegistry() error = %v", err)
	}

	pawnPSQT := mustResolve(t, registry, "psqt.pawn.mg")
	if pawnPSQT.Length != 48 || len(pawnPSQT.Dimensions) != 2 || pawnPSQT.Dimensions[0] != 6 || pawnPSQT.Dimensions[1] != 8 {
		t.Fatalf("pawn PSQT handle = length %d dimensions %v, want 48 [6 8]", pawnPSQT.Length, pawnPSQT.Dimensions)
	}
	if got, want := registry.Elements[pawnPSQT.Offset].ExportIndex, int(gm.PieceTypePawn)*64+8; got != want {
		t.Errorf("first pawn PSQT export index = %d, want %d", got, want)
	}
	if got, want := registry.Elements[pawnPSQT.Offset+pawnPSQT.Length-1].ExportIndex, int(gm.PieceTypePawn)*64+55; got != want {
		t.Errorf("last pawn PSQT export index = %d, want %d", got, want)
	}

	shelter := mustResolve(t, registry, "king.shelter.mg")
	if shelter.Length != 28 {
		t.Fatalf("king shelter length = %d, want 28", shelter.Length)
	}
	for row := 0; row < 4; row++ {
		for column := 0; column < 7; column++ {
			element := registry.Elements[shelter.MustIndex(row, column)]
			if got, want := element.ExportIndex, row*8+column; got != want {
				t.Errorf("shelter[%d][%d] export index = %d, want %d", row, column, got, want)
			}
		}
	}

	connected := mustResolve(t, registry, "pawn.connected.mg")
	if connected.Length != 6 {
		t.Fatalf("connected-pawn length = %d, want 6", connected.Length)
	}
	if got := registry.Elements[connected.Offset].ExportIndex; got != 1 {
		t.Errorf("first connected-pawn export index = %d, want 1", got)
	}

	blocked := mustResolve(t, registry, "king.storm.blocked.eg")
	if blocked.Length != 6 {
		t.Fatalf("blocked-storm length = %d, want 6", blocked.Length)
	}
	if got := registry.Elements[blocked.Offset].ExportIndex; got != 2 {
		t.Errorf("first blocked-storm export index = %d, want 2", got)
	}

	if _, exists := registry.Resolve("material.king.mg"); exists {
		t.Error("king material sentinel unexpectedly registered")
	}
	if _, exists := registry.Resolve("psqt.king.mg"); !exists {
		t.Error("active king PSQT missing")
	}
}

func TestEngineRegistrySeedsCurrentValues(t *testing.T) {
	registry, err := NewEngineRegistry()
	if err != nil {
		t.Fatalf("NewEngineRegistry() error = %v", err)
	}

	materialMG, _ := eng.CurrentTuningPieceValues()
	assertScalarInitial(t, registry, "material.pawn.mg", float64(materialMG[gm.PieceTypePawn]))
	assertScalarInitial(t, registry, "bishop.pair.mg", float64(eng.BishopPairBonusMG))
	assertScalarInitial(t, registry, "space.weight_divisor", float64(eng.SpaceWeightDiv))
	assertScalarInitial(t, registry, "final.draw_divider", float64(eng.DrawDivider))
}

func mustResolve(t *testing.T, registry *Registry, id ParameterID) ParameterHandle {
	t.Helper()
	handle, ok := registry.Resolve(id)
	if !ok {
		t.Fatalf("parameter %q not found", id)
	}
	return handle
}

func assertScalarInitial(t *testing.T, registry *Registry, id ParameterID, want float64) {
	t.Helper()
	handle := mustResolve(t, registry, id)
	if handle.Length != 1 {
		t.Fatalf("parameter %q length = %d, want scalar", id, handle.Length)
	}
	if got := registry.Elements[handle.Offset].Initial; got != want {
		t.Fatalf("parameter %q initial = %v, want %v", id, got, want)
	}
}
