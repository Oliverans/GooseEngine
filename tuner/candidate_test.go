package tuner

import (
	"strings"
	"testing"
)

func TestLoadCandidateHashesAndValidatesIdentity(t *testing.T) {
	const candidatePath = "../data/tuner-runs/stage3-full-center-pieceactivity/export-epoch20/candidate.json"
	loaded, err := LoadCandidate(candidatePath)
	if err != nil {
		t.Fatalf("LoadCandidate: %v", err)
	}
	if got, want := loaded.Provenance().Path, candidatePath; got != want {
		t.Fatalf("candidate provenance path = %q, want %q", got, want)
	}
	if got, want := loaded.Provenance().SHA256, "ae8bbc21a09d5ef6e5896349a47de56ed54b1af807e0e22579174ec59cad630c"; got != want {
		t.Fatalf("candidate SHA-256 = %q, want %q", got, want)
	}
	loaded.RegistryFingerprint = "not-a-hash"
	if err := validateCandidateIdentity(loaded); err == nil {
		t.Fatal("invalid candidate fingerprint was accepted")
	}
}

func TestCandidateVerifySeedRegistryRequiresEveryInitialAndAnchor(t *testing.T) {
	registry, err := NewEngineRegistry()
	if err != nil {
		t.Fatal(err)
	}
	values, err := InitialExactParameters(registry)
	if err != nil {
		t.Fatal(err)
	}
	candidate := Candidate{
		RegistryVersion: registry.Version, RegistryFingerprint: registry.Fingerprint,
		QuantizedValues: values,
	}
	if err := candidate.VerifySeedRegistry(registry); err != nil {
		t.Fatalf("engine-seeded registry rejected: %v", err)
	}
	registry.Elements[0].Initial++
	if err := candidate.VerifySeedRegistry(registry); err == nil || !strings.Contains(err.Error(), "initial") {
		t.Fatalf("initial mismatch error = %v, want rejection", err)
	}
	registry.Elements[0].Initial--
	registry.Elements[0].Anchor++
	if err := candidate.VerifySeedRegistry(registry); err == nil || !strings.Contains(err.Error(), "anchor") {
		t.Fatalf("anchor mismatch error = %v, want rejection", err)
	}
}

func TestGenerateEngineSeedSourceCarriesImmutableProvenance(t *testing.T) {
	registry, err := NewEngineRegistry()
	if err != nil {
		t.Fatal(err)
	}
	values, err := InitialExactParameters(registry)
	if err != nil {
		t.Fatal(err)
	}
	digest := strings.Repeat("c", 64)
	source, err := GenerateEngineSeedSource(registry, values, EngineSeedMetadata{
		CandidateSHA256: digest, DatasetManifestSHA256: strings.Repeat("d", 64),
		CheckpointSHA256: strings.Repeat("e", 64), RegistryFingerprint: registry.Fingerprint,
		Checkpoint: "parent/checkpoint.json",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{EngineSeedGeneratedHeader, "//go:build " + EngineSeedBuildTag, "Parent candidate SHA-256: " + digest, "pieceValueMG[1] = 84"} {
		if !strings.Contains(string(source), want) {
			t.Errorf("seed source missing %q", want)
		}
	}
}
