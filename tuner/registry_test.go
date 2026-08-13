package tuner

import (
	"strings"
	"testing"
)

func TestRegistryExpandsDeterministically(t *testing.T) {
	groups := []GroupSpec{
		{ID: "king", AnchorStrength: 0.02, LearningRateScale: 0.5},
		{ID: "bishop", AnchorStrength: 0.1, LearningRateScale: 1},
	}
	specs := []ParameterSpec{
		parameterTableForTest(),
		parameterScalarForTest(),
		parameterDiscreteForTest(),
	}

	registry, err := NewRegistry("test-v1", groups, specs)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	if got, want := len(registry.Elements), 6; got != want {
		t.Fatalf("element count = %d, want %d", got, want)
	}
	if got, want := registry.TrainableCount(), 5; got != want {
		t.Fatalf("trainable count = %d, want %d", got, want)
	}
	if got, want := registry.Specs[0].ID, ParameterID("bishop.pair.mg"); got != want {
		t.Fatalf("first canonical spec ID = %q, want %q", got, want)
	}

	bishop, ok := registry.Resolve("bishop.pair.mg")
	if !ok {
		t.Fatal("bishop.pair.mg handle not found")
	}
	if bishop.Offset != 0 || bishop.Length != 1 {
		t.Fatalf("bishop handle = offset %d, length %d; want 0, 1", bishop.Offset, bishop.Length)
	}
	if index, err := bishop.Index(); err != nil || index != 0 {
		t.Fatalf("bishop scalar index = %d, %v; want 0, nil", index, err)
	}

	shelter, ok := registry.Resolve("king.shelter.mg")
	if !ok {
		t.Fatal("king.shelter.mg handle not found")
	}
	if got, want := shelter.MustIndex(1, 0), shelter.Offset+2; got != want {
		t.Fatalf("shelter[1][0] index = %d, want %d", got, want)
	}
	element := registry.Elements[shelter.MustIndex(1, 0)]
	if got, want := strings.Join(element.CoordinateLabels, "/"), "1/none"; got != want {
		t.Fatalf("coordinate labels = %q, want %q", got, want)
	}
	if element.Initial != 30 || element.Anchor != 20 || element.DeviationScale != 5 {
		t.Fatalf("expanded cell = initial %v, anchor %v, scale %v", element.Initial, element.Anchor, element.DeviationScale)
	}
	if element.AnchorStrength != 0.02 || element.LearningRateScale != 0.25 {
		t.Fatalf("effective scales = anchor %v, learning rate %v; want 0.02, 0.25", element.AnchorStrength, element.LearningRateScale)
	}
	if got, want := element.ExportIndex, 2; got != want {
		t.Fatalf("default export index = %d, want %d", got, want)
	}

	discrete, ok := registry.Resolve("space.blocked_cap")
	if !ok {
		t.Fatal("space.blocked_cap handle not found")
	}
	if got := registry.Elements[discrete.Offset].TrainIndex; got != NoTrainIndex {
		t.Fatalf("discrete TrainIndex = %d, want %d", got, NoTrainIndex)
	}
}

func TestFrozenParameterRemainsInForwardVector(t *testing.T) {
	spec := parameterScalarForTest()
	spec.Training.Mode = TrainingFrozen
	registry, err := NewRegistry(
		"test-v1",
		[]GroupSpec{{ID: "bishop", AnchorStrength: 0.1, LearningRateScale: 1}},
		[]ParameterSpec{spec},
	)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	if got := len(registry.Elements); got != 1 {
		t.Fatalf("forward element count = %d, want 1", got)
	}
	if got := registry.TrainableCount(); got != 0 {
		t.Fatalf("trainable count = %d, want 0", got)
	}
	if got := registry.Elements[0].TrainIndex; got != NoTrainIndex {
		t.Fatalf("frozen TrainIndex = %d, want %d", got, NoTrainIndex)
	}
}

func TestCoordinateTrainingOverrideResolvesByAxisLabels(t *testing.T) {
	spec := parameterTableForTest()
	spec.Training.Overrides = []CoordinateTrainingOverride{
		{
			AxisValues: map[string]string{"edgeDistance": "1", "relativeRank": "none"},
			Trainable:  false,
		},
	}
	registry, err := NewRegistry(
		"test-v1",
		[]GroupSpec{{ID: "king", AnchorStrength: 1, LearningRateScale: 1}},
		[]ParameterSpec{spec},
	)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	handle, _ := registry.Resolve("king.shelter.mg")
	frozenIndex := handle.MustIndex(1, 0)
	for _, element := range registry.Elements {
		if element.Index == frozenIndex {
			if element.Mode != TrainingFrozen || element.TrainIndex != NoTrainIndex {
				t.Fatalf("overridden element = mode %q train index %d, want frozen/%d", element.Mode, element.TrainIndex, NoTrainIndex)
			}
			continue
		}
		if element.Mode != TrainingContinuous || element.TrainIndex == NoTrainIndex {
			t.Fatalf("non-overridden element %v is not trainable", element.CoordinateLabels)
		}
	}
	if got, want := registry.TrainableCount(), 3; got != want {
		t.Fatalf("trainable count = %d, want %d", got, want)
	}
}

func TestRegistryFingerprintIsOrderIndependentAndExcludesValues(t *testing.T) {
	groups := []GroupSpec{
		{ID: "king", AnchorStrength: 0.02, LearningRateScale: 0.5},
		{ID: "bishop", AnchorStrength: 0.1, LearningRateScale: 1},
	}
	scalar := parameterScalarForTest()
	table := parameterTableForTest()

	first, err := NewRegistry("test-v1", groups, []ParameterSpec{table, scalar})
	if err != nil {
		t.Fatalf("first NewRegistry() error = %v", err)
	}

	changedValues := scalar
	changedValues.Initial = BroadcastValue(99)
	changedValues.Prior.Anchor = BroadcastValue(88)
	second, err := NewRegistry("test-v1", []GroupSpec{groups[1], groups[0]}, []ParameterSpec{changedValues, table})
	if err != nil {
		t.Fatalf("second NewRegistry() error = %v", err)
	}
	if first.Fingerprint != second.Fingerprint {
		t.Fatalf("value/order-only changes altered fingerprint:\n%s\n%s", first.Fingerprint, second.Fingerprint)
	}

	changedLayout := table
	changedLayout.Formula = FormulaKingDanger
	third, err := NewRegistry("test-v1", groups, []ParameterSpec{scalar, changedLayout})
	if err != nil {
		t.Fatalf("third NewRegistry() error = %v", err)
	}
	if first.Fingerprint == third.Fingerprint {
		t.Fatal("formula/layout change did not alter fingerprint")
	}

	changedOwnership := table
	changedOwnership.Training.Overrides = []CoordinateTrainingOverride{{
		AxisValues: map[string]string{"edgeDistance": "1", "relativeRank": "none"},
		Trainable:  false,
	}}
	fourth, err := NewRegistry("test-v1", groups, []ParameterSpec{scalar, changedOwnership})
	if err != nil {
		t.Fatalf("fourth NewRegistry() error = %v", err)
	}
	if first.Fingerprint != fourth.Fingerprint {
		t.Fatalf("optimizer-ownership-only change altered dataset layout fingerprint:\n%s\n%s", first.Fingerprint, fourth.Fingerprint)
	}
}

func TestRegistryCopiesCallerDefinitions(t *testing.T) {
	spec := parameterTableForTest()
	registry, err := NewRegistry("test-v1", []GroupSpec{{ID: "king", AnchorStrength: 1, LearningRateScale: 1}}, []ParameterSpec{spec})
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	spec.Shape.Axes[0].Labels[0] = "changed"
	spec.Initial.Values[0] = -100
	if got := registry.Specs[0].Shape.Axes[0].Labels[0]; got != "0" {
		t.Fatalf("registry axis label changed through caller slice: %q", got)
	}
	if got := registry.Elements[0].Initial; got != 10 {
		t.Fatalf("registry initial changed through caller slice: %v", got)
	}

	spec = parameterTableForTest()
	spec.Training.Overrides = []CoordinateTrainingOverride{{
		AxisValues: map[string]string{"edgeDistance": "1", "relativeRank": "none"},
		Trainable:  false,
	}}
	registry, err = NewRegistry("test-v1", []GroupSpec{{ID: "king", AnchorStrength: 1, LearningRateScale: 1}}, []ParameterSpec{spec})
	if err != nil {
		t.Fatalf("NewRegistry(with override) error = %v", err)
	}
	spec.Training.Overrides[0].AxisValues["edgeDistance"] = "0"
	if got := registry.Specs[0].Training.Overrides[0].AxisValues["edgeDistance"]; got != "1" {
		t.Fatalf("registry training override changed through caller map: %q", got)
	}
}

func TestRegistryRejectsInvalidDefinitions(t *testing.T) {
	validGroup := []GroupSpec{{ID: "bishop", AnchorStrength: 0.1, LearningRateScale: 1}}
	tests := []struct {
		name    string
		groups  []GroupSpec
		specs   []ParameterSpec
		wantErr string
	}{
		{
			name:    "missing group",
			groups:  validGroup,
			specs:   []ParameterSpec{withGroup(parameterScalarForTest(), "missing")},
			wantErr: "unknown parameter group",
		},
		{
			name:    "duplicate ID",
			groups:  validGroup,
			specs:   []ParameterSpec{parameterScalarForTest(), parameterScalarForTest()},
			wantErr: "duplicate parameter ID",
		},
		{
			name:   "shape mismatch",
			groups: validGroup,
			specs: []ParameterSpec{withShape(parameterScalarForTest(), Shape{
				Dimensions: []int{2},
				Axes:       []AxisSpec{{Name: "file", Labels: []string{"a"}}},
			})},
			wantErr: "has 1 labels, want 2",
		},
		{
			name:    "missing values",
			groups:  validGroup,
			specs:   []ParameterSpec{withInitial(parameterScalarForTest(), ValueSpec{})},
			wantErr: "initial has 0 values",
		},
		{
			name:    "non-positive scale",
			groups:  validGroup,
			specs:   []ParameterSpec{withDeviationScale(parameterScalarForTest(), BroadcastValue(0))},
			wantErr: "deviation scale at cell 0 must be positive",
		},
		{
			name:    "outside bounds",
			groups:  validGroup,
			specs:   []ParameterSpec{withBounds(parameterScalarForTest(), ClosedBounds(0, 20))},
			wantErr: "initial at cell 0 is above upper bound",
		},
		{
			name:   "incomplete training override",
			groups: validGroup,
			specs: []ParameterSpec{withTrainingOverrides(withGroup(parameterTableForTest(), "bishop"),
				CoordinateTrainingOverride{AxisValues: map[string]string{"edgeDistance": "1"}, Trainable: false})},
			wantErr: "names 1 axes, want 2",
		},
		{
			name:   "unknown training override label",
			groups: validGroup,
			specs: []ParameterSpec{withTrainingOverrides(withGroup(parameterTableForTest(), "bishop"),
				CoordinateTrainingOverride{AxisValues: map[string]string{"edgeDistance": "edge", "relativeRank": "none"}, Trainable: false})},
			wantErr: "unknown label",
		},
		{
			name:   "duplicate training override",
			groups: validGroup,
			specs: []ParameterSpec{withTrainingOverrides(withGroup(parameterTableForTest(), "bishop"),
				CoordinateTrainingOverride{AxisValues: map[string]string{"edgeDistance": "1", "relativeRank": "none"}, Trainable: false},
				CoordinateTrainingOverride{AxisValues: map[string]string{"edgeDistance": "1", "relativeRank": "none"}, Trainable: true})},
			wantErr: "duplicates override",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewRegistry("test-v1", test.groups, test.specs)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("NewRegistry() error = %v, want error containing %q", err, test.wantErr)
			}
		})
	}
}

func TestParameterHandleRejectsInvalidCoordinates(t *testing.T) {
	registry, err := NewRegistry(
		"test-v1",
		[]GroupSpec{{ID: "king", AnchorStrength: 1, LearningRateScale: 1}},
		[]ParameterSpec{parameterTableForTest()},
	)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	handle, _ := registry.Resolve("king.shelter.mg")

	if _, err := handle.Index(0); err == nil {
		t.Fatal("Index(0) succeeded for a two-dimensional parameter")
	}
	if _, err := handle.Index(0, 2); err == nil {
		t.Fatal("Index(0, 2) succeeded outside the second axis")
	}
}

func TestRegistryMapsActiveSubshapeIntoEngineStorage(t *testing.T) {
	spec := parameterTableForTest()
	spec.Export.StorageDimensions = []int{2, 3}
	spec.Export.StorageOffset = 1
	spec.Export.StorageStrides = []int{3, 1}
	registry, err := NewRegistry(
		"test-v1",
		[]GroupSpec{{ID: "king", AnchorStrength: 1, LearningRateScale: 1}},
		[]ParameterSpec{spec},
	)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	got := []int{
		registry.Elements[0].ExportIndex,
		registry.Elements[1].ExportIndex,
		registry.Elements[2].ExportIndex,
		registry.Elements[3].ExportIndex,
	}
	want := []int{1, 2, 4, 5}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("export indexes = %v, want %v", got, want)
		}
	}
}

func parameterScalarForTest() ParameterSpec {
	return ParameterSpec{
		ID:         "bishop.pair.mg",
		EngineName: "BishopPairMG",
		Group:      "bishop",
		Shape:      ScalarShape(),
		Phase:      PhaseMG,
		Formula:    FormulaLinear,
		Role:       RoleCoefficient,
		Initial:    BroadcastValue(30),
		Prior: PriorSpec{
			Anchor:         BroadcastValue(28),
			DeviationScale: BroadcastValue(8),
			StrengthScale:  1,
		},
		Training: TrainingSpec{
			Mode:              TrainingContinuous,
			LearningRateScale: 1,
			Bounds:            ClosedBounds(0, 100),
		},
		Export: ExportSpec{
			GoSymbol: "BishopPairMG",
			GoType:   EngineInt,
			Rounding: RoundNearest,
		},
	}
}

func parameterTableForTest() ParameterSpec {
	return ParameterSpec{
		ID:         "king.shelter.mg",
		EngineName: "KingShelterBonusMG",
		Group:      "king",
		Shape: TableShape(
			AxisSpec{Name: "edgeDistance", Labels: []string{"0", "1"}},
			AxisSpec{Name: "relativeRank", Labels: []string{"none", "1"}},
		),
		Phase:   PhaseMG,
		Formula: FormulaLinear,
		Role:    RoleCoefficient,
		Initial: ElementValues(10, 20, 30, 40),
		Prior: PriorSpec{
			Anchor:         BroadcastValue(20),
			DeviationScale: BroadcastValue(5),
			StrengthScale:  1,
		},
		Training: TrainingSpec{
			Mode:              TrainingContinuous,
			LearningRateScale: 0.5,
			Bounds:            ClosedBounds(-100, 100),
		},
		Export: ExportSpec{
			GoSymbol: "KingShelterBonusMG",
			GoType:   EngineInt,
			Rounding: RoundNearest,
		},
	}
}

func parameterDiscreteForTest() ParameterSpec {
	return ParameterSpec{
		ID:         "space.blocked_cap",
		EngineName: "SpaceBlockedCap",
		Group:      "king",
		Shape:      ScalarShape(),
		Phase:      PhaseShared,
		Formula:    FormulaSpace,
		Role:       RoleCap,
		Initial:    BroadcastValue(4),
		Prior: PriorSpec{
			Anchor:         BroadcastValue(4),
			DeviationScale: BroadcastValue(1),
			StrengthScale:  1,
		},
		Training: TrainingSpec{
			Mode:              TrainingDiscrete,
			LearningRateScale: 1,
			Bounds:            ClosedBounds(1, 8),
		},
		Export: ExportSpec{
			GoSymbol: "SpaceBlockedCap",
			GoType:   EngineInt,
			Rounding: RoundNearest,
		},
	}
}

func withGroup(spec ParameterSpec, group GroupID) ParameterSpec {
	spec.Group = group
	return spec
}

func withShape(spec ParameterSpec, shape Shape) ParameterSpec {
	spec.Shape = shape
	return spec
}

func withInitial(spec ParameterSpec, initial ValueSpec) ParameterSpec {
	spec.Initial = initial
	return spec
}

func withDeviationScale(spec ParameterSpec, scale ValueSpec) ParameterSpec {
	spec.Prior.DeviationScale = scale
	return spec
}

func withBounds(spec ParameterSpec, bounds Bounds) ParameterSpec {
	spec.Training.Bounds = bounds
	return spec
}

func withTrainingOverrides(spec ParameterSpec, overrides ...CoordinateTrainingOverride) ParameterSpec {
	spec.Training.Overrides = overrides
	return spec
}
