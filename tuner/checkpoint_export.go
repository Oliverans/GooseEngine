package tuner

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

type CheckpointParameterSnapshot struct {
	TrainingDefinitionSHA256 string
	Cursor                   TrainingCursor
	AdamStep                 uint64
	Parameters               []float64
}

// LoadCheckpointParameterSnapshot reads the forward vector from a compatible
// dataset/registry layout without requiring the current optimizer policy to
// match. This permits export after coordinate ownership policy changes; the
// source training-definition identity remains available for verification.
func LoadCheckpointParameterSnapshot(path string, dataset LoadedDatasetManifest, registry *Registry) (CheckpointParameterSnapshot, error) {
	if registry == nil {
		return CheckpointParameterSnapshot{}, errors.New("checkpoint export requires a registry")
	}
	file, err := os.Open(path)
	if err != nil {
		return CheckpointParameterSnapshot{}, err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var checkpoint trainerCheckpoint
	if err := decoder.Decode(&checkpoint); err != nil {
		return CheckpointParameterSnapshot{}, fmt.Errorf("decode checkpoint: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return CheckpointParameterSnapshot{}, err
	}
	if checkpoint.Format != TrainerCheckpointFormat || checkpoint.Version != TrainerCheckpointVersion {
		return CheckpointParameterSnapshot{}, fmt.Errorf("unsupported checkpoint %q version %d", checkpoint.Format, checkpoint.Version)
	}
	if checkpoint.DatasetManifestSHA256 != dataset.SHA256 {
		return CheckpointParameterSnapshot{}, errors.New("checkpoint dataset manifest does not match export dataset")
	}
	if checkpoint.RegistryVersion != registry.Version || checkpoint.RegistryFingerprint != registry.Fingerprint {
		return CheckpointParameterSnapshot{}, errors.New("checkpoint registry layout does not match export registry")
	}
	if len(checkpoint.Parameters) != len(registry.Elements) {
		return CheckpointParameterSnapshot{}, fmt.Errorf("checkpoint has %d parameters, want %d", len(checkpoint.Parameters), len(registry.Elements))
	}
	for index, value := range checkpoint.Parameters {
		if !finite(value) {
			return CheckpointParameterSnapshot{}, fmt.Errorf("checkpoint parameter %d is not finite", index)
		}
		if err := validateBoundedValue(registry.Elements[index].Bounds, value, "checkpoint", index); err != nil {
			return CheckpointParameterSnapshot{}, err
		}
	}
	return CheckpointParameterSnapshot{
		TrainingDefinitionSHA256: checkpoint.TrainingDefinitionSHA256,
		Cursor:                   checkpoint.Cursor, AdamStep: checkpoint.AdamStep,
		Parameters: append([]float64(nil), checkpoint.Parameters...),
	}, nil
}
