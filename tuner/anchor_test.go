package tuner

import (
	"slices"
	"testing"
)

func TestParameterAnchorModelNormalizesByScaleAndGroupSize(t *testing.T) {
	registry, err := NewRegistry(
		"anchor-test-v1",
		[]GroupSpec{
			{ID: "king", AnchorStrength: 0.2, LearningRateScale: 1},
		},
		[]ParameterSpec{parameterTableForTest(), parameterDiscreteForTest()},
	)
	if err != nil {
		t.Fatal(err)
	}
	model, err := NewParameterAnchorModel(registry, nil, ParameterAnchorConfig{})
	if err != nil {
		t.Fatal(err)
	}
	parameters := registry.InitialValues()
	parameters[0], parameters[1], parameters[2], parameters[3] = 25, 30, 15, 20
	gradient := make([]float64, registry.TrainableCount())
	metrics, err := model.Evaluate(parameters, gradient)
	if err != nil {
		t.Fatal(err)
	}
	// Normalized displacements are [1,2,-1,0]. With lambda 0.2 and
	// four active cells, the coefficient is 0.05 per squared displacement.
	assertNear(t, metrics.Loss, 0.3, 1e-15)
	assertNear(t, gradient[0], 0.02, 1e-15)
	assertNear(t, gradient[1], 0.04, 1e-15)
	assertNear(t, gradient[2], -0.02, 1e-15)
	assertNear(t, gradient[3], 0, 1e-15)
	hotGradient := make([]float64, registry.TrainableCount())
	hotLoss, err := model.Penalty(parameters, hotGradient)
	if err != nil {
		t.Fatal(err)
	}
	assertNear(t, hotLoss, metrics.Loss, 1e-15)
	for index := range gradient {
		assertNear(t, hotGradient[index], gradient[index], 1e-15)
	}
	if metrics.ActiveParameters != 4 || metrics.FrozenParameters != 1 || metrics.UncoveredParameters != 0 {
		t.Fatalf("unexpected anchor counts: %+v", metrics)
	}
	if len(metrics.Groups) != 1 || metrics.Groups[0].Group != "king" || metrics.Groups[0].ActiveParameters != 4 {
		t.Fatalf("unexpected group metrics: %+v", metrics.Groups)
	}
}

func TestParameterAnchorModelCoverageFreezesUnobservedCoordinates(t *testing.T) {
	registry, err := NewRegistry(
		"anchor-test-v1",
		[]GroupSpec{{ID: "king", AnchorStrength: 0.2, LearningRateScale: 1}},
		[]ParameterSpec{parameterTableForTest(), parameterDiscreteForTest()},
	)
	if err != nil {
		t.Fatal(err)
	}
	coverage := []uint64{1, 0, 1, 1, 1}
	model, err := NewParameterAnchorModel(registry, coverage, ParameterAnchorConfig{
		GroupStrengths: map[GroupID]float64{"king": 0.4},
	})
	if err != nil {
		t.Fatal(err)
	}
	parameters := registry.InitialValues()
	parameters[0], parameters[1], parameters[2], parameters[3] = 25, 30, 15, 20
	gradient := make([]float64, registry.TrainableCount())
	metrics, err := model.Evaluate(parameters, gradient)
	if err != nil {
		t.Fatal(err)
	}
	// The uncovered second cell is absent. Active normalized displacements are
	// [1,-1,0], producing 0.4/3*(1+1).
	assertNear(t, metrics.Loss, 0.8/3, 1e-15)
	assertNear(t, gradient[0], 0.16/3, 1e-15)
	assertNear(t, gradient[1], 0, 1e-15)
	assertNear(t, gradient[2], -0.16/3, 1e-15)
	if metrics.ActiveParameters != 3 || metrics.UncoveredParameters != 1 || metrics.FrozenParameters != 1 {
		t.Fatalf("unexpected anchor counts: %+v", metrics)
	}
	if got, want := model.ActiveTrainIndexes(), []int{0, 2, 3}; !slices.Equal(got, want) {
		t.Fatalf("active indexes = %v, want %v", got, want)
	}
}

func TestParameterAnchorModelRejectsInvalidConfiguration(t *testing.T) {
	registry, err := NewRegistry(
		"anchor-test-v1",
		[]GroupSpec{{ID: "king", AnchorStrength: 0.2, LearningRateScale: 1}},
		[]ParameterSpec{parameterTableForTest()},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewParameterAnchorModel(registry, []uint64{1}, ParameterAnchorConfig{}); err == nil {
		t.Fatal("invalid coverage length was accepted")
	}
	if _, err := NewParameterAnchorModel(registry, nil, ParameterAnchorConfig{
		GroupStrengths: map[GroupID]float64{"missing": 1},
	}); err == nil {
		t.Fatal("unknown group override was accepted")
	}
}
