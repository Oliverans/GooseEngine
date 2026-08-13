package tuner

import (
	"math"
	"testing"
)

func TestAdamOptimizerBiasCorrectionBoundsAndInactiveCoordinates(t *testing.T) {
	spec := parameterScalarForTest()
	spec.Training.LearningRateScale = 0.5
	registry, err := NewRegistry(
		"adam-test-v1",
		[]GroupSpec{{ID: "bishop", AnchorStrength: 0, LearningRateScale: 1}},
		[]ParameterSpec{spec},
	)
	if err != nil {
		t.Fatal(err)
	}
	optimizer, err := NewAdamOptimizer(registry, nil, AdamConfig{Beta1: 0, Beta2: 0, Epsilon: 1})
	if err != nil {
		t.Fatal(err)
	}
	parameters := registry.InitialValues()
	metrics, err := optimizer.Update(parameters, []float64{2}, 0.1)
	if err != nil {
		t.Fatal(err)
	}
	assertNear(t, parameters[0], 30-0.1*0.5*2/3, 1e-15)
	if metrics.Step != 1 || optimizer.Step() != 1 || metrics.ActiveCoordinates != 1 || metrics.BoundHits != 0 {
		t.Fatalf("unexpected first Adam metrics: %+v", metrics)
	}
	metrics, err = optimizer.Update(parameters, []float64{2}, 100)
	if err != nil {
		t.Fatal(err)
	}
	if parameters[0] != 0 || metrics.BoundHits != 1 {
		t.Fatalf("bounded Adam value=%v metrics=%+v", parameters[0], metrics)
	}

	inactive, err := NewAdamOptimizer(registry, []int{}, DefaultAdamConfig())
	if err != nil {
		t.Fatal(err)
	}
	parameters = registry.InitialValues()
	if _, err := inactive.Update(parameters, []float64{100}, 1); err != nil {
		t.Fatal(err)
	}
	if parameters[0] != registry.Elements[0].Initial || inactive.firstMoment[0] != 0 || inactive.secondMoment[0] != 0 {
		t.Fatalf("inactive coordinate changed: parameter=%v m=%v v=%v", parameters[0], inactive.firstMoment[0], inactive.secondMoment[0])
	}
}

func TestAdamOptimizerRejectsInvalidStepAtomically(t *testing.T) {
	registry, err := NewRegistry(
		"adam-test-v1",
		[]GroupSpec{{ID: "bishop", AnchorStrength: 0, LearningRateScale: 1}},
		[]ParameterSpec{parameterScalarForTest()},
	)
	if err != nil {
		t.Fatal(err)
	}
	optimizer, err := NewAdamOptimizer(registry, nil, DefaultAdamConfig())
	if err != nil {
		t.Fatal(err)
	}
	parameters := registry.InitialValues()
	before := append([]float64(nil), parameters...)
	if _, err := optimizer.Update(parameters, []float64{math.NaN()}, 0.1); err == nil {
		t.Fatal("non-finite gradient was accepted")
	}
	if parameters[0] != before[0] || optimizer.Step() != 0 || optimizer.firstMoment[0] != 0 {
		t.Fatalf("failed Adam step mutated state: parameter=%v step=%d m=%v", parameters[0], optimizer.Step(), optimizer.firstMoment[0])
	}
}

func TestAdamOptimizerHasNoSteadyStateAllocations(t *testing.T) {
	registry, err := NewRegistry(
		"adam-test-v1",
		[]GroupSpec{{ID: "bishop", AnchorStrength: 0, LearningRateScale: 1}},
		[]ParameterSpec{parameterScalarForTest()},
	)
	if err != nil {
		t.Fatal(err)
	}
	optimizer, err := NewAdamOptimizer(registry, nil, DefaultAdamConfig())
	if err != nil {
		t.Fatal(err)
	}
	parameters := registry.InitialValues()
	gradient := []float64{0.25}
	var updateErr error
	allocations := testing.AllocsPerRun(100, func() {
		_, updateErr = optimizer.Update(parameters, gradient, 0.001)
	})
	if updateErr != nil {
		t.Fatal(updateErr)
	}
	if allocations != 0 {
		t.Fatalf("steady-state Adam allocations = %v, want 0", allocations)
	}
}

func TestEpochLearningRateSchedule(t *testing.T) {
	schedule := EpochLearningRateSchedule{
		Initial: 0.01,
		Drops: []LearningRateDrop{
			{Epoch: 3, Factor: 0.1},
			{Epoch: 7, Factor: 0.5},
		},
	}
	for epoch, want := range map[uint64]float64{0: 0.01, 2: 0.01, 3: 0.001, 6: 0.001, 7: 0.0005} {
		got, err := schedule.Rate(epoch)
		if err != nil {
			t.Fatal(err)
		}
		assertNear(t, got, want, 1e-15)
	}
	bad := schedule
	bad.Drops[1].Epoch = 3
	if err := bad.Validate(); err == nil {
		t.Fatal("duplicate learning-rate epoch was accepted")
	}
}
