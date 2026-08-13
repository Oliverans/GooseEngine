package tuner

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

const CandidateFormat = "goose-tuner-export-v1"

// CandidateProvenance is the immutable identity carried by an exported integer
// candidate. The vector is deliberately separate from a trainer checkpoint: a
// chained run starts from the exact exported engine integers with fresh Adam
// state, rather than resuming the parent optimizer.
type CandidateProvenance struct {
	Path                  string `json:"path"`
	SHA256                string `json:"sha256"`
	DatasetManifestSHA256 string `json:"datasetManifestSha256"`
	RegistryVersion       string `json:"registryVersion"`
	RegistryFingerprint   string `json:"registryFingerprint"`
	Checkpoint            string `json:"checkpoint"`
	CheckpointSHA256      string `json:"checkpointSha256"`
}

// Candidate is the compatible subset of tunerexport's immutable candidate.json
// report required to seed a later training stage.
type Candidate struct {
	Format                   string `json:"format"`
	DatasetRoot              string `json:"datasetRoot"`
	DatasetManifestSHA256    string `json:"datasetManifestSha256"`
	RegistryVersion          string `json:"registryVersion"`
	RegistryFingerprint      string `json:"registryFingerprint"`
	TrainingDefinitionSHA256 string `json:"trainingDefinitionSha256"`
	Checkpoint               string `json:"checkpoint"`
	CheckpointSHA256         string `json:"checkpointSha256"`
	BuildTag                 string `json:"buildTag"`
	QuantizedValues          []int  `json:"quantizedValues"`
	candidatePath            string
	candidateSHA256          string
}

// LoadCandidate hashes and decodes a tunerexport candidate report. The report
// remains the immutable source of truth; callers must validate it against their
// live dataset and compiled registry before using the vector.
func LoadCandidate(path string) (Candidate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Candidate{}, err
	}
	var candidate Candidate
	if err := json.Unmarshal(data, &candidate); err != nil {
		return Candidate{}, fmt.Errorf("decode candidate: %w", err)
	}
	if candidate.Format != CandidateFormat {
		return Candidate{}, fmt.Errorf("unsupported candidate format %q", candidate.Format)
	}
	if candidate.BuildTag != EngineCandidateBuildTag {
		return Candidate{}, fmt.Errorf("candidate build tag %q, want %q", candidate.BuildTag, EngineCandidateBuildTag)
	}
	if err := validateCandidateIdentity(candidate); err != nil {
		return Candidate{}, err
	}
	digest := sha256.Sum256(data)
	candidate.candidatePath = path
	candidate.candidateSHA256 = hex.EncodeToString(digest[:])
	candidate.QuantizedValues = append([]int(nil), candidate.QuantizedValues...)
	return candidate, nil
}

func validateCandidateIdentity(candidate Candidate) error {
	for _, field := range []struct {
		name  string
		value string
	}{
		{"dataset manifest SHA-256", candidate.DatasetManifestSHA256},
		{"registry version", candidate.RegistryVersion},
		{"registry fingerprint", candidate.RegistryFingerprint},
		{"training definition SHA-256", candidate.TrainingDefinitionSHA256},
		{"checkpoint", candidate.Checkpoint},
		{"checkpoint SHA-256", candidate.CheckpointSHA256},
	} {
		if field.value == "" {
			return fmt.Errorf("candidate %s is required", field.name)
		}
	}
	if !isSHA256(candidate.DatasetManifestSHA256) || !isSHA256(candidate.RegistryFingerprint) || !isSHA256(candidate.TrainingDefinitionSHA256) || !isSHA256(candidate.CheckpointSHA256) {
		return errors.New("candidate contains an invalid SHA-256 identity")
	}
	if len(candidate.QuantizedValues) == 0 {
		return errors.New("candidate quantized vector is empty")
	}
	return nil
}

func isSHA256(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func (c Candidate) Provenance() CandidateProvenance {
	return CandidateProvenance{
		Path: c.candidatePath, SHA256: c.candidateSHA256,
		DatasetManifestSHA256: c.DatasetManifestSHA256,
		RegistryVersion:       c.RegistryVersion, RegistryFingerprint: c.RegistryFingerprint,
		Checkpoint: c.Checkpoint, CheckpointSHA256: c.CheckpointSHA256,
	}
}

// VerifySeedRegistry fails closed unless the seed candidate describes the
// current registry layout and the compiled engine has applied every vector cell
// as both its effective initial value and anchor.
func (c Candidate) VerifySeedRegistry(registry *Registry) error {
	if registry == nil {
		return errors.New("candidate seed requires a registry")
	}
	if c.RegistryVersion != registry.Version || c.RegistryFingerprint != registry.Fingerprint {
		return fmt.Errorf("candidate registry %s/%s does not match compiled registry %s/%s", c.RegistryVersion, c.RegistryFingerprint, registry.Version, registry.Fingerprint)
	}
	if len(c.QuantizedValues) != len(registry.Elements) {
		return fmt.Errorf("candidate has %d quantized values, compiled registry has %d", len(c.QuantizedValues), len(registry.Elements))
	}
	for index, element := range registry.Elements {
		want := float64(c.QuantizedValues[index])
		if element.Initial != want {
			return fmt.Errorf("compiled registry initial value %d for %q is %g, want seed %g", index, element.ID, element.Initial, want)
		}
		if element.Anchor != want {
			return fmt.Errorf("compiled registry anchor value %d for %q is %g, want seed %g", index, element.ID, element.Anchor, want)
		}
	}
	return nil
}
