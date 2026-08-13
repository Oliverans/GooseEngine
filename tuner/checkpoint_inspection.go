package tuner

import (
	"encoding/json"
	"fmt"
	"os"
)

// CheckpointInspection is the immutable, read-only portion of a trainer
// checkpoint. It intentionally exposes no mutation path and is suitable for
// reporting tools that need to inspect artifacts without constructing a trainer.
type CheckpointInspection struct {
	Path                     string
	DatasetManifestSHA256    string
	RegistryVersion          string
	RegistryFingerprint      string
	TrainingDefinitionSHA256 string
	Cursor                   TrainingCursor
	Parameters               []float64
	FirstMoment              []float64
	SecondMoment             []float64
	LastUpdate               []float64
	AdamStep                 uint64
	EarlyStopping            EarlyStoppingState
}

// InspectCheckpoint decodes one checkpoint artifact and verifies its wire
// format. It deliberately does not apply the checkpoint or require a dataset;
// callers can compare its immutable identities to a run report as appropriate.
func InspectCheckpoint(path string) (CheckpointInspection, error) {
	file, err := os.Open(path)
	if err != nil {
		return CheckpointInspection{}, err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var checkpoint trainerCheckpoint
	if err := decoder.Decode(&checkpoint); err != nil {
		return CheckpointInspection{}, fmt.Errorf("decode checkpoint: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return CheckpointInspection{}, err
	}
	if checkpoint.Format != TrainerCheckpointFormat || checkpoint.Version != TrainerCheckpointVersion {
		return CheckpointInspection{}, fmt.Errorf("unsupported checkpoint %q version %d", checkpoint.Format, checkpoint.Version)
	}
	return CheckpointInspection{
		Path:                  path,
		DatasetManifestSHA256: checkpoint.DatasetManifestSHA256,
		RegistryVersion:       checkpoint.RegistryVersion, RegistryFingerprint: checkpoint.RegistryFingerprint,
		TrainingDefinitionSHA256: checkpoint.TrainingDefinitionSHA256,
		Cursor:                   checkpoint.Cursor, Parameters: append([]float64(nil), checkpoint.Parameters...),
		FirstMoment:  append([]float64(nil), checkpoint.FirstMoment...),
		SecondMoment: append([]float64(nil), checkpoint.SecondMoment...),
		LastUpdate:   append([]float64(nil), checkpoint.LastUpdate...),
		AdamStep:     checkpoint.AdamStep, EarlyStopping: checkpoint.EarlyStopping,
	}, nil
}
