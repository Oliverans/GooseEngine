package tuner

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	TrainerCheckpointFormat  = "goose-tuner-checkpoint"
	TrainerCheckpointVersion = 1
)

type trainerCheckpoint struct {
	Format                   string             `json:"format"`
	Version                  int                `json:"version"`
	DatasetManifestSHA256    string             `json:"datasetManifestSha256"`
	RegistryVersion          string             `json:"registryVersion"`
	RegistryFingerprint      string             `json:"registryFingerprint"`
	TrainingDefinitionSHA256 string             `json:"trainingDefinitionSha256"`
	Cursor                   TrainingCursor     `json:"cursor"`
	Parameters               []float64          `json:"parameters"`
	FirstMoment              []float64          `json:"firstMoment"`
	SecondMoment             []float64          `json:"secondMoment"`
	LastUpdate               []float64          `json:"lastUpdate"`
	AdamStep                 uint64             `json:"adamStep"`
	EarlyStopping            EarlyStoppingState `json:"earlyStopping"`
}

type definitionAnchorGroup struct {
	ID       GroupID
	Strength float64
	Active   int
}

type definitionAnchorCoordinate struct {
	ParameterIndex int
	TrainIndex     int
	GroupIndex     int
	Anchor         float64
	DeviationScale float64
	StrengthScale  float64
}

func trainerDefinitionSHA256(t *Trainer) (string, error) {
	if t == nil || t.registry == nil || t.optimizer == nil {
		return "", errors.New("training definition requires a complete trainer")
	}
	definition := struct {
		Schema              string
		RegistryVersion     string
		RegistryFingerprint string
		Elements            []ParameterElement
		TexelK              float64
		Config              TrainerConfig
		Active              []bool
		AnchorGroups        []definitionAnchorGroup
		AnchorCoordinates   []definitionAnchorCoordinate
	}{
		Schema:          "goose-tuner-training-definition-v1",
		RegistryVersion: t.registry.Version, RegistryFingerprint: t.registry.Fingerprint,
		Elements: t.registry.Elements, TexelK: t.link.K, Config: t.config,
		Active: make([]bool, len(t.optimizer.coordinates)),
	}
	for index, coordinate := range t.optimizer.coordinates {
		definition.Active[index] = coordinate.active
	}
	if t.anchor != nil {
		definition.AnchorGroups = make([]definitionAnchorGroup, len(t.anchor.groups))
		for index, group := range t.anchor.groups {
			definition.AnchorGroups[index] = definitionAnchorGroup{ID: group.id, Strength: group.strength, Active: group.active}
		}
		definition.AnchorCoordinates = make([]definitionAnchorCoordinate, len(t.anchor.coordinates))
		for index, coordinate := range t.anchor.coordinates {
			definition.AnchorCoordinates[index] = definitionAnchorCoordinate{
				ParameterIndex: coordinate.parameterIndex, TrainIndex: coordinate.trainIndex,
				GroupIndex: coordinate.groupIndex, Anchor: coordinate.anchor,
				DeviationScale: coordinate.deviationScale, StrengthScale: coordinate.strengthScale,
			}
		}
	}
	encoded, err := json.Marshal(definition)
	if err != nil {
		return "", fmt.Errorf("encode training definition: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

// SaveCheckpoint atomically publishes a new checkpoint file. It deliberately
// refuses to overwrite an existing path; callers should use unique step/epoch
// names and update any human-facing "latest" convention separately.
func (t *Trainer) SaveCheckpoint(path string, dataset LoadedDatasetManifest) error {
	if t == nil || t.registry == nil || t.optimizer == nil {
		return errors.New("cannot checkpoint a nil trainer")
	}
	if path == "" {
		return errors.New("checkpoint path is required")
	}
	if dataset.SHA256 == "" {
		return errors.New("checkpoint requires a dataset manifest identity")
	}
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("checkpoint path %q already exists", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	checkpoint := trainerCheckpoint{
		Format: TrainerCheckpointFormat, Version: TrainerCheckpointVersion,
		DatasetManifestSHA256: dataset.SHA256,
		RegistryVersion:       t.registry.Version, RegistryFingerprint: t.registry.Fingerprint,
		TrainingDefinitionSHA256: t.definitionSHA256, Cursor: t.cursor,
		Parameters:   append([]float64(nil), t.parameters...),
		FirstMoment:  append([]float64(nil), t.optimizer.firstMoment...),
		SecondMoment: append([]float64(nil), t.optimizer.secondMoment...),
		LastUpdate:   append([]float64(nil), t.optimizer.lastUpdate...),
		AdamStep:     t.optimizer.step, EarlyStopping: t.earlyStop,
	}
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".tuner-checkpoint-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	published := false
	defer func() {
		_ = temporary.Close()
		if !published {
			_ = os.Remove(temporaryPath)
		}
	}()
	encoder := json.NewEncoder(temporary)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(checkpoint); err != nil {
		return fmt.Errorf("encode checkpoint: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync checkpoint: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close checkpoint: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("publish checkpoint: %w", err)
	}
	published = true
	return nil
}

// LoadCheckpoint validates the complete snapshot before mutating the trainer.
func (t *Trainer) LoadCheckpoint(path string, dataset LoadedDatasetManifest) error {
	if t == nil || t.registry == nil || t.optimizer == nil {
		return errors.New("cannot load a checkpoint into a nil trainer")
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var checkpoint trainerCheckpoint
	if err := decoder.Decode(&checkpoint); err != nil {
		return fmt.Errorf("decode checkpoint: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return err
	}
	if err := t.validateCheckpoint(checkpoint, dataset); err != nil {
		return err
	}
	copy(t.parameters, checkpoint.Parameters)
	copy(t.optimizer.firstMoment, checkpoint.FirstMoment)
	copy(t.optimizer.secondMoment, checkpoint.SecondMoment)
	copy(t.optimizer.lastUpdate, checkpoint.LastUpdate)
	t.optimizer.step = checkpoint.AdamStep
	t.cursor = checkpoint.Cursor
	t.earlyStop = checkpoint.EarlyStopping
	return nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("checkpoint contains trailing JSON")
		}
		return fmt.Errorf("checkpoint trailing data: %w", err)
	}
	return nil
}

func (t *Trainer) validateCheckpoint(c trainerCheckpoint, dataset LoadedDatasetManifest) error {
	if c.Format != TrainerCheckpointFormat || c.Version != TrainerCheckpointVersion {
		return fmt.Errorf("unsupported checkpoint %q version %d", c.Format, c.Version)
	}
	if c.DatasetManifestSHA256 != dataset.SHA256 {
		return fmt.Errorf("checkpoint dataset manifest %q, want %q", c.DatasetManifestSHA256, dataset.SHA256)
	}
	if c.RegistryVersion != t.registry.Version || c.RegistryFingerprint != t.registry.Fingerprint {
		return errors.New("checkpoint registry does not match trainer registry")
	}
	if c.TrainingDefinitionSHA256 != t.definitionSHA256 {
		return errors.New("checkpoint training definition does not match trainer configuration")
	}
	if len(c.Parameters) != len(t.parameters) || len(c.FirstMoment) != len(t.optimizer.firstMoment) ||
		len(c.SecondMoment) != len(t.optimizer.secondMoment) || len(c.LastUpdate) != len(t.optimizer.lastUpdate) {
		return errors.New("checkpoint vector dimensions do not match trainer dimensions")
	}
	activeParameter := make([]bool, len(c.Parameters))
	for _, coordinate := range t.optimizer.coordinates {
		activeParameter[coordinate.parameterIndex] = coordinate.active
	}
	for index, value := range c.Parameters {
		if !finite(value) {
			return fmt.Errorf("checkpoint parameter %d is not finite", index)
		}
		element := t.registry.Elements[index]
		if err := validateBoundedValue(element.Bounds, value, "checkpoint", index); err != nil {
			return err
		}
		if !activeParameter[index] && value != element.Initial {
			return fmt.Errorf("checkpoint inactive parameter %d changed", index)
		}
	}
	for index := range c.FirstMoment {
		if !finite(c.FirstMoment[index]) || !finite(c.SecondMoment[index]) || c.SecondMoment[index] < 0 || !finite(c.LastUpdate[index]) {
			return fmt.Errorf("checkpoint Adam coordinate %d is invalid", index)
		}
		coordinate := t.optimizer.coordinates[index]
		if !coordinate.active && (c.FirstMoment[index] != 0 || c.SecondMoment[index] != 0 || c.LastUpdate[index] != 0) {
			return fmt.Errorf("checkpoint inactive Adam coordinate %d changed", index)
		}
	}
	if err := t.validateCheckpointCursor(c.Cursor, c.AdamStep, dataset); err != nil {
		return err
	}
	if c.EarlyStopping.Initialized && !finite(c.EarlyStopping.BestBrier) {
		return errors.New("checkpoint early-stopping best Brier is invalid")
	}
	return nil
}

func (t *Trainer) validateCheckpointCursor(cursor TrainingCursor, step uint64, dataset LoadedDatasetManifest) error {
	shards, err := deterministicEpochTrainingSelection(dataset, t.config.RecordsPerEpoch, cursor.Epoch)
	if err != nil {
		return err
	}
	if cursor.Shard < 0 || cursor.Shard >= len(shards) || cursor.RecordOffset < 0 {
		return fmt.Errorf("checkpoint training cursor %+v is invalid", cursor)
	}
	if cursor.RecordOffset != 0 {
		if cursor.RecordOffset%t.batchSize != 0 || uint64(cursor.RecordOffset) >= shards[cursor.Shard].Records {
			return fmt.Errorf("checkpoint record offset %d is not a valid batch boundary", cursor.RecordOffset)
		}
	}
	var batchesPerEpoch uint64
	for _, shard := range shards {
		batchesPerEpoch += (shard.Records + uint64(t.batchSize) - 1) / uint64(t.batchSize)
	}
	if cursor.Epoch != 0 && batchesPerEpoch > ^uint64(0)/cursor.Epoch {
		return errors.New("checkpoint expected Adam step overflows")
	}
	expected := cursor.Epoch * batchesPerEpoch
	for index := 0; index < cursor.Shard; index++ {
		expected += (shards[index].Records + uint64(t.batchSize) - 1) / uint64(t.batchSize)
	}
	expected += uint64(cursor.RecordOffset / t.batchSize)
	if step != expected {
		return fmt.Errorf("checkpoint Adam step %d does not match cursor-derived step %d", step, expected)
	}
	return nil
}
