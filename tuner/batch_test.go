package tuner

import (
	"math"
	"math/rand"
	"testing"
)

func TestPackedBatchAccumulatorMatchesWeightedObjectiveDerivative(t *testing.T) {
	registry, binding, model := newForwardTestSystem(t)
	shard := nonlinearGradientTestShard(binding)
	second := shard.Records[0]
	second.Outcome = OutcomeBlackWin
	second.FixedMG -= 23
	second.FixedEG += 31
	second.SideToMove = -1
	shard.Records = append(shard.Records, second)

	coverage := make([]uint64, len(registry.Elements))
	for index := range coverage {
		coverage[index] = 1
	}
	anchor, err := NewParameterAnchorModel(registry, coverage, ParameterAnchorConfig{
		GroupStrengths: map[GroupID]float64{groupMaterial: 0.01},
	})
	if err != nil {
		t.Fatal(err)
	}
	link, _ := NewTexelLink(0.007)
	batch, err := NewPackedBatchAccumulator(model, link, anchor)
	if err != nil {
		t.Fatal(err)
	}
	parameters := registry.InitialValues()
	parameters[binding.Material[1].MG.Offset] += 2.5
	indexes := []uint32{0, 1, 0}
	weights := []float64{1, 2, 0.5}
	gradient := make([]float64, registry.TrainableCount())
	metrics, err := batch.Compute(&shard, indexes, weights, parameters, gradient)
	if err != nil {
		t.Fatal(err)
	}
	if metrics.Records != 3 || metrics.Weight != 3.5 || metrics.TotalLoss != metrics.Data.Brier+metrics.AnchorLoss {
		t.Fatalf("unexpected batch metrics: %+v", metrics)
	}

	rng := rand.New(rand.NewSource(0x6261746368677261))
	plus := append([]float64(nil), parameters...)
	minus := append([]float64(nil), parameters...)
	const epsilon = 1e-5
	analytic := 0.0
	for _, element := range registry.Elements {
		if element.TrainIndex == NoTrainIndex {
			continue
		}
		value := rng.Float64()*2 - 1
		plus[element.Index] += epsilon * value
		minus[element.Index] -= epsilon * value
		analytic += gradient[element.TrainIndex] * value
	}
	workGradient := make([]float64, registry.TrainableCount())
	plusMetrics, err := batch.Compute(&shard, indexes, weights, plus, workGradient)
	if err != nil {
		t.Fatal(err)
	}
	minusMetrics, err := batch.Compute(&shard, indexes, weights, minus, workGradient)
	if err != nil {
		t.Fatal(err)
	}
	numeric := (plusMetrics.TotalLoss - minusMetrics.TotalLoss) / (2 * epsilon)
	assertNear(t, analytic, numeric, 3e-7*math.Max(1, math.Abs(numeric)))
}

func TestPackedBatchAccumulatorHasNoSteadyStateAllocations(t *testing.T) {
	registry, binding, model := newForwardTestSystem(t)
	shard := nonlinearGradientTestShard(binding)
	link, _ := NewTexelLink(0.007)
	batch, err := NewPackedBatchAccumulator(model, link, nil)
	if err != nil {
		t.Fatal(err)
	}
	parameters := registry.InitialValues()
	gradient := make([]float64, registry.TrainableCount())
	indexes := []uint32{0}
	if _, err := batch.Compute(&shard, indexes, nil, parameters, gradient); err != nil {
		t.Fatal(err)
	}
	var computeErr error
	allocations := testing.AllocsPerRun(100, func() {
		_, computeErr = batch.Compute(&shard, indexes, nil, parameters, gradient)
	})
	if computeErr != nil {
		t.Fatal(computeErr)
	}
	if allocations != 0 {
		t.Fatalf("steady-state batch allocations = %v, want 0", allocations)
	}
}
