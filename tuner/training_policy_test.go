package tuner

import (
	"strings"
	"testing"
)

func TestTrainingPolicyFreezesLabelledCoordinates(t *testing.T) {
	specs, err := applyTrainingPolicy(
		[]ParameterSpec{parameterTableForTest()},
		TrainingPolicy{Overrides: []ParameterTrainingOverride{
			Freeze("king.shelter.mg", At("relativeRank", "none")),
		}},
	)
	if err != nil {
		t.Fatalf("applyTrainingPolicy() error = %v", err)
	}
	registry, err := NewRegistry(
		"test-v1",
		[]GroupSpec{{ID: "king", AnchorStrength: 1, LearningRateScale: 1}},
		specs,
	)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	handle, _ := registry.Resolve("king.shelter.mg")
	for edgeDistance := 0; edgeDistance < 2; edgeDistance++ {
		frozen := registry.Elements[handle.MustIndex(edgeDistance, 0)]
		if frozen.Mode != TrainingFrozen || frozen.TrainIndex != NoTrainIndex {
			t.Errorf("edgeDistance=%d relativeRank=none is not frozen", edgeDistance)
		}
		active := registry.Elements[handle.MustIndex(edgeDistance, 1)]
		if active.Mode != TrainingContinuous || active.TrainIndex == NoTrainIndex {
			t.Errorf("edgeDistance=%d relativeRank=1 is not trainable", edgeDistance)
		}
	}
	if got, want := registry.TrainableCount(), 2; got != want {
		t.Fatalf("trainable count = %d, want %d", got, want)
	}
}

func TestTrainingPolicyWithoutSelectionsAppliesToWholeParameter(t *testing.T) {
	specs, err := applyTrainingPolicy(
		[]ParameterSpec{parameterScalarForTest()},
		TrainingPolicy{Overrides: []ParameterTrainingOverride{Freeze("bishop.pair.mg")}},
	)
	if err != nil {
		t.Fatalf("applyTrainingPolicy() error = %v", err)
	}
	registry, err := NewRegistry(
		"test-v1",
		[]GroupSpec{{ID: "bishop", AnchorStrength: 1, LearningRateScale: 1}},
		specs,
	)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	if element := registry.Elements[0]; element.Mode != TrainingFrozen || element.TrainIndex != NoTrainIndex {
		t.Fatalf("whole scalar override = mode %q train index %d, want frozen/%d", element.Mode, element.TrainIndex, NoTrainIndex)
	}
}

func TestTrainingPolicyCanEnableOneCoordinate(t *testing.T) {
	spec := parameterTableForTest()
	spec.Training.Mode = TrainingFrozen
	specs, err := applyTrainingPolicy(
		[]ParameterSpec{spec},
		TrainingPolicy{Overrides: []ParameterTrainingOverride{
			Train("king.shelter.mg", At("edgeDistance", "1"), At("relativeRank", "none")),
		}},
	)
	if err != nil {
		t.Fatalf("applyTrainingPolicy() error = %v", err)
	}
	registry, err := NewRegistry(
		"test-v1",
		[]GroupSpec{{ID: "king", AnchorStrength: 1, LearningRateScale: 1}},
		specs,
	)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	handle, _ := registry.Resolve("king.shelter.mg")
	activeIndex := handle.MustIndex(1, 0)
	if got, want := registry.TrainableCount(), 1; got != want {
		t.Fatalf("trainable count = %d, want %d", got, want)
	}
	for _, element := range registry.Elements {
		if element.Index == activeIndex {
			if element.Mode != TrainingContinuous || element.TrainIndex == NoTrainIndex {
				t.Fatal("selected coordinate was not made trainable")
			}
		} else if element.Mode != TrainingFrozen || element.TrainIndex != NoTrainIndex {
			t.Fatalf("unselected coordinate %v did not retain frozen default", element.CoordinateLabels)
		}
	}
}

func TestTrainingPolicySelectsGroupsThenAppliesParameterOverrides(t *testing.T) {
	table := parameterTableForTest()
	scalar := parameterScalarForTest()
	specs, err := applyTrainingPolicy(
		[]ParameterSpec{table, scalar},
		TrainingPolicy{
			Default: FreezeEligibleParameters,
			Groups:  []GroupTrainingOverride{TrainGroup("king")},
			Overrides: []ParameterTrainingOverride{
				Freeze("king.shelter.mg", At("edgeDistance", "1"), At("relativeRank", "none")),
			},
		},
	)
	if err != nil {
		t.Fatalf("applyTrainingPolicy() error = %v", err)
	}
	registry, err := NewRegistry(
		"test-v1",
		[]GroupSpec{
			{ID: "king", AnchorStrength: 1, LearningRateScale: 1},
			{ID: "bishop", AnchorStrength: 1, LearningRateScale: 1},
		},
		specs,
	)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	handle, _ := registry.Resolve("king.shelter.mg")
	frozenIndex := handle.MustIndex(1, 0)
	if got, want := registry.TrainableCount(), 3; got != want {
		t.Fatalf("trainable count = %d, want %d", got, want)
	}
	for _, element := range registry.Elements {
		switch {
		case element.ID == "bishop.pair.mg":
			if element.Mode != TrainingFrozen {
				t.Fatal("unselected bishop group remained trainable")
			}
		case element.Index == frozenIndex:
			if element.Mode != TrainingFrozen {
				t.Fatal("specific parameter override did not win over group rule")
			}
		case element.ID == "king.shelter.mg" && element.Mode != TrainingContinuous:
			t.Fatalf("selected king coordinate %v is not trainable", element.CoordinateLabels)
		}
	}
}

func TestTrainGroupDoesNotEnableStructurallyFrozenCoordinates(t *testing.T) {
	spec := parameterScalarForTest()
	spec.Training.Mode = TrainingFrozen
	specs, err := applyTrainingPolicy(
		[]ParameterSpec{spec},
		TrainingPolicy{
			Default: FreezeEligibleParameters,
			Groups:  []GroupTrainingOverride{TrainGroup("bishop")},
		},
	)
	if err != nil {
		t.Fatalf("applyTrainingPolicy() error = %v", err)
	}
	registry, err := NewRegistry(
		"test-v1",
		[]GroupSpec{{ID: "bishop", AnchorStrength: 1, LearningRateScale: 1}},
		specs,
	)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	if got := registry.TrainableCount(); got != 0 {
		t.Fatalf("trainable count = %d, want structurally frozen group member to remain frozen", got)
	}
}

func TestTrainingPolicyGroupRulesApplyInOrder(t *testing.T) {
	specs, err := applyTrainingPolicy(
		[]ParameterSpec{parameterScalarForTest()},
		TrainingPolicy{Groups: []GroupTrainingOverride{
			FreezeGroup("bishop"),
			TrainGroup("bishop"),
		}},
	)
	if err != nil {
		t.Fatalf("applyTrainingPolicy() error = %v", err)
	}
	registry, err := NewRegistry(
		"test-v1",
		[]GroupSpec{{ID: "bishop", AnchorStrength: 1, LearningRateScale: 1}},
		specs,
	)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	if got := registry.TrainableCount(); got != 1 {
		t.Fatalf("trainable count = %d, want later TrainGroup rule to restore ownership", got)
	}
}

func TestTrainingPolicyRejectsInvalidSelectors(t *testing.T) {
	table := parameterTableForTest()
	scalar := parameterScalarForTest()
	discrete := parameterDiscreteForTest()
	tests := []struct {
		name    string
		specs   []ParameterSpec
		policy  TrainingPolicy
		wantErr string
	}{
		{
			name: "unknown parameter", specs: []ParameterSpec{table},
			policy:  TrainingPolicy{Overrides: []ParameterTrainingOverride{Freeze("missing")}},
			wantErr: "unknown parameter",
		},
		{
			name: "unknown axis", specs: []ParameterSpec{table},
			policy:  TrainingPolicy{Overrides: []ParameterTrainingOverride{Freeze("king.shelter.mg", At("rank", "1"))}},
			wantErr: "unknown axis",
		},
		{
			name: "unknown label", specs: []ParameterSpec{table},
			policy:  TrainingPolicy{Overrides: []ParameterTrainingOverride{Freeze("king.shelter.mg", At("relativeRank", "2"))}},
			wantErr: "unknown label",
		},
		{
			name: "duplicate axis", specs: []ParameterSpec{table},
			policy:  TrainingPolicy{Overrides: []ParameterTrainingOverride{Freeze("king.shelter.mg", At("relativeRank", "none"), At("relativeRank", "1"))}},
			wantErr: "more than once",
		},
		{
			name: "scalar axes", specs: []ParameterSpec{scalar},
			policy:  TrainingPolicy{Overrides: []ParameterTrainingOverride{Freeze("bishop.pair.mg", At("rank", "1"))}},
			wantErr: "scalar",
		},
		{
			name: "discrete parameter", specs: []ParameterSpec{discrete},
			policy:  TrainingPolicy{Overrides: []ParameterTrainingOverride{Train("space.blocked_cap")}},
			wantErr: "discrete parameter",
		},
		{
			name: "unknown group", specs: []ParameterSpec{table},
			policy:  TrainingPolicy{Groups: []GroupTrainingOverride{TrainGroup("missing")}},
			wantErr: "unknown group",
		},
		{
			name: "empty group", specs: []ParameterSpec{table},
			policy:  TrainingPolicy{Groups: []GroupTrainingOverride{TrainGroup("")}},
			wantErr: "no group ID",
		},
		{
			name: "unknown default", specs: []ParameterSpec{table},
			policy:  TrainingPolicy{Default: TrainingDefault("bogus")},
			wantErr: "unknown training policy default",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := applyTrainingPolicy(test.specs, test.policy)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("applyTrainingPolicy() error = %v, want error containing %q", err, test.wantErr)
			}
		})
	}
}
