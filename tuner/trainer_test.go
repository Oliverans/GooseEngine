package tuner

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
)

func TestTrainerEpochIsDeterministicAndHonorsCoverageFreezing(t *testing.T) {
	registry, binding, model := newDatasetTestSystem(t)
	root := t.TempDir()
	bookA := filepath.Join(root, "alpha.book")
	bookB := filepath.Join(root, "beta.book")
	if err := os.WriteFile(bookA, []byte(
		"8/8/8/8/8/8/8/K6k w - - 0 1 [0.5]\n"+
			"r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1 [1.0]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bookB, []byte(
		"8/8/4P3/8/8/8/8/K5k1 w - - 0 1 [1.0]\n"+
			"4r3/5pkp/2b3p1/1p6/8/2p1nPP1/PPN4P/1R2R2K b - - 3 25 [0.0]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	config := DefaultShardedCompileConfig()
	config.Compile.Split = SplitConfig{Seed: 19}
	config.Compile.ProgressEvery = 0
	config.MaxRecordsPerShard = 1
	output := filepath.Join(root, "dataset")
	if _, err := CompileBookFilesSharded(context.Background(), []string{bookB, bookA}, output, registry, binding, model, config); err != nil {
		t.Fatal(err)
	}
	manifest, err := LoadDatasetManifest(output, registry)
	if err != nil {
		t.Fatal(err)
	}
	statistics, err := LoadDatasetStatistics(manifest, registry)
	if err != nil {
		t.Fatal(err)
	}
	anchorA, err := NewParameterAnchorModel(registry, statistics.Coverage, ParameterAnchorConfig{})
	if err != nil {
		t.Fatal(err)
	}
	anchorB, err := NewParameterAnchorModel(registry, statistics.Coverage, ParameterAnchorConfig{})
	if err != nil {
		t.Fatal(err)
	}
	link, _ := NewTexelLink(0.007)
	trainerConfig := TrainerConfig{
		BatchSize: 2,
		Schedule: EpochLearningRateSchedule{
			Initial: 0.001,
			Drops:   []LearningRateDrop{{Epoch: 2, Factor: 0.5}},
		},
		Adam: DefaultAdamConfig(),
	}
	first, err := NewTrainer(registry, model, link, anchorA, trainerConfig)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewTrainer(registry, model, link, anchorB, trainerConfig)
	if err != nil {
		t.Fatal(err)
	}
	initial := registry.InitialValues()
	for epoch := uint64(0); epoch < 3; epoch++ {
		firstMetrics, err := first.TrainEpoch(manifest, epoch)
		if err != nil {
			t.Fatal(err)
		}
		secondMetrics, err := second.TrainEpoch(manifest, epoch)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(firstMetrics, secondMetrics) {
			t.Fatalf("epoch %d metrics differ:\n%+v\n%+v", epoch, firstMetrics, secondMetrics)
		}
		if !slices.Equal(first.Parameters(), second.Parameters()) {
			t.Fatalf("epoch %d parameters differ", epoch)
		}
		if firstMetrics.Records != manifest.Data.Statistics.Splits.Training || firstMetrics.Batches == 0 {
			t.Fatalf("epoch %d did not consume the training split: %+v", epoch, firstMetrics)
		}
	}
	trained := first.Parameters()
	changed := 0
	for _, element := range registry.Elements {
		if element.Mode != TrainingContinuous || statistics.Coverage[element.Index] == 0 {
			if trained[element.Index] != initial[element.Index] {
				t.Fatalf("frozen/uncovered parameter %s%v changed from %v to %v",
					element.ID, element.Coordinate, initial[element.Index], trained[element.Index])
			}
			continue
		}
		if trained[element.Index] != initial[element.Index] {
			changed++
		}
	}
	if changed == 0 {
		t.Fatal("training changed no active parameters")
	}
}

func TestAdamOverfitsOnePackedPosition(t *testing.T) {
	registry, binding, model := newForwardTestSystem(t)
	shard := nonlinearGradientTestShard(binding)
	link, _ := NewTexelLink(0.007)
	batch, err := NewPackedBatchAccumulator(model, link, nil)
	if err != nil {
		t.Fatal(err)
	}
	optimizer, err := NewAdamOptimizer(registry, nil, DefaultAdamConfig())
	if err != nil {
		t.Fatal(err)
	}
	parameters := registry.InitialValues()
	gradient := make([]float64, registry.TrainableCount())
	initial, err := batch.Compute(&shard, nil, nil, parameters, gradient)
	if err != nil {
		t.Fatal(err)
	}
	for step := 0; step < 100; step++ {
		if _, err := batch.Compute(&shard, nil, nil, parameters, gradient); err != nil {
			t.Fatal(err)
		}
		if _, err := optimizer.Update(parameters, gradient, 0.02); err != nil {
			t.Fatal(err)
		}
	}
	final, err := batch.Compute(&shard, nil, nil, parameters, gradient)
	if err != nil {
		t.Fatal(err)
	}
	if final.Data.Brier >= initial.Data.Brier*0.5 {
		t.Fatalf("single-position Brier did not substantially decrease: %v -> %v", initial.Data.Brier, final.Data.Brier)
	}
}
