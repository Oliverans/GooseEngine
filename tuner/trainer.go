package tuner

import (
	"errors"
	"fmt"
)

type TrainerConfig struct {
	BatchSize       int
	RecordsPerEpoch uint64
	Schedule        EpochLearningRateSchedule
	Adam            AdamConfig
	EarlyStopping   EarlyStoppingConfig
}

func DefaultTrainerConfig() TrainerConfig {
	// The learning rate is a provisional smoke-run value. The first controlled
	// baseline run must calibrate it before treating this as a production preset.
	return TrainerConfig{
		BatchSize: 16384,
		Schedule:  ConstantLearningRate(0.01),
		Adam:      DefaultAdamConfig(),
	}
}

func (c TrainerConfig) Validate() error {
	if c.BatchSize <= 0 {
		return fmt.Errorf("trainer batch size must be positive, got %d", c.BatchSize)
	}
	if err := c.Schedule.Validate(); err != nil {
		return err
	}
	if err := c.Adam.Validate(); err != nil {
		return err
	}
	return c.EarlyStopping.Validate()
}

type EpochTrainingMetrics struct {
	Epoch          uint64
	LearningRate   float64
	Shards         int
	Batches        uint64
	Records        uint64
	Weight         float64
	Data           DataLossMetrics
	MeanAnchorLoss float64
	MeanTotalLoss  float64
	BoundHits      uint64
	LastAdamStep   uint64
	ShardOrder     []string
}

// TrainingCursor identifies the next batch boundary to process. RecordOffset
// is an offset into the deterministic permutation for the current shard, not
// the shard's physical record number.
type TrainingCursor struct {
	Epoch        uint64 `json:"epoch"`
	Shard        int    `json:"shard"`
	RecordOffset int    `json:"recordOffset"`
}

// TrainingProgressMetrics describes one resumable segment. A segment ends
// either after maxBatches or at the end of its current epoch.
type TrainingProgressMetrics struct {
	Epoch          uint64
	LearningRate   float64
	ShardsVisited  int
	Batches        uint64
	Records        uint64
	Weight         float64
	Data           DataLossMetrics
	MeanAnchorLoss float64
	MeanTotalLoss  float64
	BoundHits      uint64
	LastAdamStep   uint64
	ShardOrder     []string
	StartCursor    TrainingCursor
	EndCursor      TrainingCursor
	EpochComplete  bool
}

// Trainer is the deterministic single-threaded training core.
type Trainer struct {
	registry         *Registry
	model            *ForwardModel
	batch            *PackedBatchAccumulator
	optimizer        *AdamOptimizer
	schedule         EpochLearningRateSchedule
	batchSize        int
	parameters       []float64
	gradient         []float64
	cursor           TrainingCursor
	link             TexelLink
	anchor           *ParameterAnchorModel
	config           TrainerConfig
	earlyStop        EarlyStoppingState
	definitionSHA256 string
}

func NewTrainer(registry *Registry, model *ForwardModel, link TexelLink, anchor *ParameterAnchorModel, config TrainerConfig) (*Trainer, error) {
	if registry == nil || model == nil {
		return nil, errors.New("trainer requires a registry and forward model")
	}
	if model.parameterCount != len(registry.Elements) || model.trainableCount != registry.TrainableCount() {
		return nil, errors.New("trainer registry dimensions do not match the forward model")
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	batch, err := NewPackedBatchAccumulator(model, link, anchor)
	if err != nil {
		return nil, err
	}
	var active []int
	if anchor != nil {
		active = anchor.ActiveTrainIndexes()
	}
	optimizer, err := NewAdamOptimizer(registry, active, config.Adam)
	if err != nil {
		return nil, err
	}
	trainer := &Trainer{
		registry: registry, model: model, batch: batch, optimizer: optimizer,
		schedule: config.Schedule, batchSize: config.BatchSize,
		parameters: registry.InitialValues(), gradient: make([]float64, registry.TrainableCount()),
		link: link, anchor: anchor, config: config,
	}
	trainer.definitionSHA256, err = trainerDefinitionSHA256(trainer)
	if err != nil {
		return nil, err
	}
	return trainer, nil
}

func (t *Trainer) Parameters() []float64 {
	if t == nil {
		return nil
	}
	return append([]float64(nil), t.parameters...)
}

func (t *Trainer) OptimizerStep() uint64 {
	if t == nil || t.optimizer == nil {
		return 0
	}
	return t.optimizer.Step()
}

func (t *Trainer) TrainingDefinitionSHA256() string {
	if t == nil {
		return ""
	}
	return t.definitionSHA256
}

func (t *Trainer) Cursor() TrainingCursor {
	if t == nil {
		return TrainingCursor{}
	}
	return t.cursor
}

// TrainEpoch visits every training record exactly once in deterministic
// epoch-specific shard and record order. It is a convenience wrapper around
// the resumable batch runner and therefore requires an epoch-boundary cursor.
func (t *Trainer) TrainEpoch(dataset LoadedDatasetManifest, epoch uint64) (EpochTrainingMetrics, error) {
	if t == nil {
		return EpochTrainingMetrics{}, errors.New("cannot train with a nil trainer")
	}
	want := TrainingCursor{Epoch: epoch}
	if t.cursor != want {
		return EpochTrainingMetrics{}, fmt.Errorf("TrainEpoch %d requires cursor %+v, got %+v", epoch, want, t.cursor)
	}
	progress, err := t.TrainBatches(dataset, ^uint64(0))
	if err != nil {
		return EpochTrainingMetrics{}, err
	}
	if !progress.EpochComplete {
		return EpochTrainingMetrics{}, fmt.Errorf("epoch %d did not complete", epoch)
	}
	return EpochTrainingMetrics{
		Epoch: progress.Epoch, LearningRate: progress.LearningRate,
		Shards: progress.ShardsVisited, Batches: progress.Batches,
		Records: progress.Records, Weight: progress.Weight, Data: progress.Data,
		MeanAnchorLoss: progress.MeanAnchorLoss, MeanTotalLoss: progress.MeanTotalLoss,
		BoundHits: progress.BoundHits, LastAdamStep: progress.LastAdamStep,
		ShardOrder: progress.ShardOrder,
	}, nil
}

// TrainBatches advances from the current cursor by at most maxBatches. The
// cursor is advanced only after a complete optimizer update, making every
// externally visible cursor a safe checkpoint boundary.
func (t *Trainer) TrainBatches(dataset LoadedDatasetManifest, maxBatches uint64) (TrainingProgressMetrics, error) {
	if t == nil || t.registry == nil || t.model == nil || t.optimizer == nil {
		return TrainingProgressMetrics{}, errors.New("cannot train with a nil trainer")
	}
	if maxBatches == 0 {
		return TrainingProgressMetrics{}, errors.New("maximum batch count must be positive")
	}
	if dataset.Data.RegistryFingerprint != t.registry.Fingerprint {
		return TrainingProgressMetrics{}, fmt.Errorf("dataset registry fingerprint %q, want %q", dataset.Data.RegistryFingerprint, t.registry.Fingerprint)
	}
	epoch := t.cursor.Epoch
	learningRate, err := t.schedule.Rate(epoch)
	if err != nil {
		return TrainingProgressMetrics{}, err
	}
	shards, err := deterministicEpochTrainingSelection(dataset, t.config.RecordsPerEpoch, epoch)
	if err != nil {
		return TrainingProgressMetrics{}, err
	}
	if t.cursor.Shard < 0 || t.cursor.Shard >= len(shards) {
		return TrainingProgressMetrics{}, fmt.Errorf("training cursor shard %d outside [0,%d)", t.cursor.Shard, len(shards))
	}
	if t.cursor.RecordOffset < 0 {
		return TrainingProgressMetrics{}, fmt.Errorf("training cursor record offset must be non-negative, got %d", t.cursor.RecordOffset)
	}
	metrics := TrainingProgressMetrics{
		Epoch: epoch, LearningRate: learningRate, StartCursor: t.cursor,
		ShardOrder: make([]string, 0, len(shards)-t.cursor.Shard),
	}
	var dataSums weightedDataMetrics
	anchorSum, totalSum := 0.0, 0.0
	for shardIndex := t.cursor.Shard; shardIndex < len(shards); shardIndex++ {
		selection := shards[shardIndex]
		metadata := selection.Metadata
		metrics.ShardOrder = append(metrics.ShardOrder, metadata.Path)
		metrics.ShardsVisited++
		shard, err := LoadPackedDatasetShard(dataset, metadata, t.registry)
		if err != nil {
			return TrainingProgressMetrics{}, fmt.Errorf("load training shard %q: %w", metadata.Path, err)
		}
		permutation, err := deterministicSelectedRecordPermutation(dataset, selection, epoch)
		if err != nil {
			return TrainingProgressMetrics{}, err
		}
		startOffset := 0
		if shardIndex == t.cursor.Shard {
			startOffset = t.cursor.RecordOffset
		}
		if startOffset >= len(permutation) {
			return TrainingProgressMetrics{}, fmt.Errorf("training cursor record offset %d outside shard %q with %d records", startOffset, metadata.Path, len(permutation))
		}
		if startOffset%t.batchSize != 0 {
			return TrainingProgressMetrics{}, fmt.Errorf("training cursor record offset %d is not a batch boundary for size %d", startOffset, t.batchSize)
		}
		for start := startOffset; start < len(permutation); start += t.batchSize {
			end := min(start+t.batchSize, len(permutation))
			batchMetrics, err := t.batch.Compute(&shard, permutation[start:end], nil, t.parameters, t.gradient)
			if err != nil {
				return TrainingProgressMetrics{}, fmt.Errorf("shard %q batch %d: %w", metadata.Path, metrics.Batches, err)
			}
			stepMetrics, err := t.optimizer.Update(t.parameters, t.gradient, learningRate)
			if err != nil {
				return TrainingProgressMetrics{}, fmt.Errorf("shard %q batch %d Adam: %w", metadata.Path, metrics.Batches, err)
			}
			dataSums.add(batchMetrics.Data)
			anchorSum += batchMetrics.AnchorLoss
			totalSum += batchMetrics.TotalLoss
			metrics.Batches++
			metrics.Records += batchMetrics.Records
			metrics.Weight += batchMetrics.Weight
			metrics.BoundHits += uint64(stepMetrics.BoundHits)
			metrics.LastAdamStep = stepMetrics.Step
			t.cursor = TrainingCursor{Epoch: epoch, Shard: shardIndex, RecordOffset: end}
			if end == len(permutation) {
				t.cursor.Shard++
				t.cursor.RecordOffset = 0
			}
			if metrics.Batches == maxBatches {
				if t.cursor.Shard == len(shards) {
					t.cursor = TrainingCursor{Epoch: epoch + 1}
					metrics.EpochComplete = true
				}
				metrics.EndCursor = t.cursor
				return finalizeTrainingProgress(metrics, dataSums, anchorSum, totalSum)
			}
		}
	}
	t.cursor = TrainingCursor{Epoch: epoch + 1}
	metrics.EndCursor = t.cursor
	metrics.EpochComplete = true
	return finalizeTrainingProgress(metrics, dataSums, anchorSum, totalSum)
}

func finalizeTrainingProgress(metrics TrainingProgressMetrics, dataSums weightedDataMetrics, anchorSum, totalSum float64) (TrainingProgressMetrics, error) {
	if metrics.Batches == 0 {
		return TrainingProgressMetrics{}, errors.New("training progress contains no batches")
	}
	data, err := dataSums.metrics()
	if err != nil {
		return TrainingProgressMetrics{}, err
	}
	metrics.Data = data
	metrics.MeanAnchorLoss = anchorSum / float64(metrics.Batches)
	metrics.MeanTotalLoss = totalSum / float64(metrics.Batches)
	return metrics, nil
}

type weightedDataMetrics struct {
	total     weightedLossSums
	byOutcome [3]weightedLossSums
}

func (s *weightedDataMetrics) add(metrics DataLossMetrics) {
	s.total.addMetrics(metrics.OutcomeLossMetrics)
	for outcome := range s.byOutcome {
		s.byOutcome[outcome].addMetrics(metrics.ByOutcome[outcome])
	}
}

func (s *weightedLossSums) addMetrics(metrics OutcomeLossMetrics) {
	s.samples += metrics.Samples
	s.weight += metrics.Weight
	s.brier += metrics.Brier * metrics.Weight
	s.logLoss += metrics.LogLoss * metrics.Weight
	s.prediction += metrics.MeanExpectedWhiteScore * metrics.Weight
	s.target += metrics.MeanTargetWhiteScore * metrics.Weight
}

func (s *weightedDataMetrics) metrics() (DataLossMetrics, error) {
	if s.total.weight <= 0 {
		return DataLossMetrics{}, errors.New("epoch metrics require positive data weight")
	}
	metrics := DataLossMetrics{OutcomeLossMetrics: s.total.metrics()}
	for outcome := range metrics.ByOutcome {
		metrics.ByOutcome[outcome] = s.byOutcome[outcome].metrics()
	}
	return metrics, nil
}
