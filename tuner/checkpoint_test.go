package tuner

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
)

type trainerCheckpointFixture struct {
	registry *Registry
	model    *ForwardModel
	manifest LoadedDatasetManifest
	link     TexelLink
	anchor   *ParameterAnchorModel
	config   TrainerConfig
}

func newTrainerCheckpointFixture(t *testing.T) trainerCheckpointFixture {
	t.Helper()
	registry, binding, model := newDatasetTestSystem(t)
	root := t.TempDir()
	lines := []string{
		"8/8/8/8/8/8/8/K6k w - - 0 1 [0.5]",
		"r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1 [1.0]",
		"8/8/4P3/8/8/8/8/K5k1 w - - 0 1 [1.0]",
		"4r3/5pkp/2b3p1/1p6/8/2p1nPP1/PPN4P/1R2R2K b - - 3 25 [0.0]",
	}
	parsed := make([]BookPosition, len(lines))
	for index, line := range lines {
		position, err := ParseBookLine(line)
		if err != nil {
			t.Fatal(err)
		}
		parsed[index] = position
	}
	var split SplitConfig
	found := false
	for seed := uint64(1); seed < 10000; seed++ {
		candidate := SplitConfig{Seed: seed, ValidationBasisPoints: 5000}
		training, validation := 0, 0
		for _, position := range parsed {
			assigned, err := AssignSplit(position.IdentityFEN, candidate)
			if err != nil {
				t.Fatal(err)
			}
			if assigned == SplitTraining {
				training++
			} else if assigned == SplitValidation {
				validation++
			}
		}
		if training >= 2 && validation >= 1 {
			split, found = candidate, true
			break
		}
	}
	if !found {
		t.Fatal("could not find deterministic fixture split")
	}
	book := filepath.Join(root, "fixture.book")
	data := []byte(lines[0] + "\n" + lines[1] + "\n" + lines[2] + "\n" + lines[3] + "\n")
	if err := os.WriteFile(book, data, 0o644); err != nil {
		t.Fatal(err)
	}
	compileConfig := DefaultShardedCompileConfig()
	compileConfig.Compile.Split = split
	compileConfig.Compile.ProgressEvery = 0
	compileConfig.MaxRecordsPerShard = 8
	output := filepath.Join(root, "dataset")
	if _, err := CompileBookFilesSharded(context.Background(), []string{book}, output, registry, binding, model, compileConfig); err != nil {
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
	anchor, err := NewParameterAnchorModel(registry, statistics.Coverage, ParameterAnchorConfig{})
	if err != nil {
		t.Fatal(err)
	}
	link, err := NewTexelLink(0.007)
	if err != nil {
		t.Fatal(err)
	}
	config := TrainerConfig{
		BatchSize: 1, RecordsPerEpoch: 2,
		Schedule: EpochLearningRateSchedule{Initial: 0.001, Drops: []LearningRateDrop{{Epoch: 2, Factor: 0.5}}},
		Adam:     DefaultAdamConfig(), EarlyStopping: EarlyStoppingConfig{Patience: 2, MinDelta: 1e-7},
	}
	return trainerCheckpointFixture{registry: registry, model: model, manifest: manifest, link: link, anchor: anchor, config: config}
}

func (f trainerCheckpointFixture) newTrainer(t *testing.T) *Trainer {
	t.Helper()
	trainer, err := NewTrainer(f.registry, f.model, f.link, f.anchor, f.config)
	if err != nil {
		t.Fatal(err)
	}
	return trainer
}

func TestCheckpointResumeExactlyMatchesUninterruptedTraining(t *testing.T) {
	fixture := newTrainerCheckpointFixture(t)
	uninterrupted := fixture.newTrainer(t)
	interrupted := fixture.newTrainer(t)

	initialValidation, err := uninterrupted.Validate(fixture.manifest, SplitValidation)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := uninterrupted.ObserveValidation(0, initialValidation); err != nil {
		t.Fatal(err)
	}
	if _, err := interrupted.ObserveValidation(0, initialValidation); err != nil {
		t.Fatal(err)
	}
	for epoch := uint64(0); epoch < 3; epoch++ {
		if _, err := uninterrupted.TrainEpoch(fixture.manifest, epoch); err != nil {
			t.Fatal(err)
		}
	}
	progress, err := interrupted.TrainBatches(fixture.manifest, 1)
	if err != nil {
		t.Fatal(err)
	}
	if progress.EpochComplete || interrupted.Cursor() == (TrainingCursor{}) {
		t.Fatalf("one batch did not leave a mid-epoch cursor: %+v", progress)
	}
	checkpointPath := filepath.Join(t.TempDir(), "step-1.json")
	if err := interrupted.SaveCheckpoint(checkpointPath, fixture.manifest); err != nil {
		t.Fatal(err)
	}
	snapshot, err := LoadCheckpointParameterSnapshot(checkpointPath, fixture.manifest, fixture.registry)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.TrainingDefinitionSHA256 != interrupted.TrainingDefinitionSHA256() ||
		snapshot.Cursor != interrupted.Cursor() || snapshot.AdamStep != interrupted.OptimizerStep() ||
		!slices.Equal(snapshot.Parameters, interrupted.parameters) {
		t.Fatalf("checkpoint parameter snapshot differs from trainer state: %+v", snapshot)
	}
	if err := interrupted.SaveCheckpoint(checkpointPath, fixture.manifest); err == nil {
		t.Fatal("checkpoint unexpectedly overwrote an existing path")
	}

	resumed := fixture.newTrainer(t)
	if err := resumed.LoadCheckpoint(checkpointPath, fixture.manifest); err != nil {
		t.Fatal(err)
	}
	for resumed.Cursor().Epoch < 3 {
		if _, err := resumed.TrainBatches(fixture.manifest, 1); err != nil {
			t.Fatal(err)
		}
	}
	if !slices.Equal(uninterrupted.parameters, resumed.parameters) ||
		!slices.Equal(uninterrupted.optimizer.firstMoment, resumed.optimizer.firstMoment) ||
		!slices.Equal(uninterrupted.optimizer.secondMoment, resumed.optimizer.secondMoment) ||
		!slices.Equal(uninterrupted.optimizer.lastUpdate, resumed.optimizer.lastUpdate) {
		t.Fatal("resumed parameters or Adam state differ from uninterrupted training")
	}
	if uninterrupted.cursor != resumed.cursor || uninterrupted.optimizer.step != resumed.optimizer.step ||
		uninterrupted.earlyStop != resumed.earlyStop {
		t.Fatalf("resumed scalar state differs: uninterrupted=%+v/%d resumed=%+v/%d",
			uninterrupted.cursor, uninterrupted.optimizer.step, resumed.cursor, resumed.optimizer.step)
	}
}

func TestValidationDiagnosticsAndCheckpointRejectionAreNonMutating(t *testing.T) {
	fixture := newTrainerCheckpointFixture(t)
	trainer := fixture.newTrainer(t)
	if _, err := trainer.TrainBatches(fixture.manifest, 1); err != nil {
		t.Fatal(err)
	}
	parameters := trainer.Parameters()
	first := append([]float64(nil), trainer.optimizer.firstMoment...)
	second := append([]float64(nil), trainer.optimizer.secondMoment...)
	last := append([]float64(nil), trainer.optimizer.lastUpdate...)
	cursor := trainer.Cursor()
	step := trainer.OptimizerStep()

	validation, err := trainer.Validate(fixture.manifest, SplitValidation)
	if err != nil {
		t.Fatal(err)
	}
	if validation.Data.Samples == 0 || len(validation.BySource) == 0 || validation.TotalLoss != validation.Data.Brier+validation.Anchor.Loss {
		t.Fatalf("unexpected validation metrics: %+v", validation)
	}
	diagnostics, err := trainer.Diagnostics(5)
	if err != nil {
		t.Fatal(err)
	}
	if diagnostics.AdamStep != step || diagnostics.Cursor != cursor || len(diagnostics.Groups) != len(fixture.registry.Groups) || len(diagnostics.TopDisplacements) != 5 {
		t.Fatalf("unexpected diagnostics: %+v", diagnostics)
	}
	if !slices.Equal(parameters, trainer.parameters) || !slices.Equal(first, trainer.optimizer.firstMoment) ||
		!slices.Equal(second, trainer.optimizer.secondMoment) || !slices.Equal(last, trainer.optimizer.lastUpdate) ||
		trainer.Cursor() != cursor || trainer.OptimizerStep() != step {
		t.Fatal("validation or diagnostics mutated training state")
	}

	path := filepath.Join(t.TempDir(), "checkpoint.json")
	if err := trainer.SaveCheckpoint(path, fixture.manifest); err != nil {
		t.Fatal(err)
	}
	changedConfig := fixture.config
	changedConfig.BatchSize++
	incompatible, err := NewTrainer(fixture.registry, fixture.model, fixture.link, fixture.anchor, changedConfig)
	if err != nil {
		t.Fatal(err)
	}
	before := incompatible.Parameters()
	if err := incompatible.LoadCheckpoint(path, fixture.manifest); err == nil {
		t.Fatal("incompatible training definition unexpectedly loaded")
	}
	if !reflect.DeepEqual(before, incompatible.Parameters()) || incompatible.OptimizerStep() != 0 || incompatible.Cursor() != (TrainingCursor{}) {
		t.Fatal("rejected checkpoint mutated the target trainer")
	}
}

func TestEarlyStoppingUsesUnregularizedValidationBrier(t *testing.T) {
	fixture := newTrainerCheckpointFixture(t)
	trainer := fixture.newTrainer(t)
	values := []float64{0.25, 0.24, 0.24000005, 0.241}
	var final EarlyStoppingDecision
	for epoch, value := range values {
		decision, err := trainer.ObserveValidation(uint64(epoch), ValidationMetrics{Data: DataLossMetrics{OutcomeLossMetrics: OutcomeLossMetrics{Brier: value}}, TotalLoss: value + 100})
		if err != nil {
			t.Fatal(err)
		}
		final = decision
	}
	if !final.Stop || final.State.BestEpoch != 1 || final.State.BadEpochs != 2 {
		t.Fatalf("unexpected early-stopping decision: %+v", final)
	}
}
