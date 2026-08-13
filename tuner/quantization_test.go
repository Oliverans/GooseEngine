package tuner

import (
	"bytes"
	"strings"
	"testing"
)

func TestQuantizeParametersAppliesBoundsRoundingAndEngineTypes(t *testing.T) {
	registry, err := NewEngineRegistry()
	if err != nil {
		t.Fatal(err)
	}
	parameters := registry.InitialValues()
	adjustment := mustResolve(t, registry, "king.danger.adjustment.mg")
	divisor := mustResolve(t, registry, "king.danger.divisor.mg")
	drawDivider := mustResolve(t, registry, "final.draw_divider")
	parameters[adjustment.Offset] = 0.5
	parameters[divisor.Offset] = -4.2
	parameters[drawDivider.Offset] = 7
	registry.Elements[adjustment.Offset].Mode = TrainingContinuous
	registry.Elements[divisor.Offset].Mode = TrainingContinuous
	result, err := QuantizeParameters(registry, parameters)
	if err != nil {
		t.Fatal(err)
	}
	if result.Values[adjustment.Offset] != 1 {
		t.Fatalf("nearest half rounded to %d, want 1", result.Values[adjustment.Offset])
	}
	if result.Values[divisor.Offset] != 1 || !result.Entries[divisor.Offset].BoundClamped {
		t.Fatalf("bounded divisor = %+v, want quantized 1 with clamp", result.Entries[divisor.Offset])
	}
	if result.Values[drawDivider.Offset] != int(registry.Elements[drawDivider.Offset].Initial) ||
		!result.Entries[drawDivider.Offset].PolicyReset || result.Metrics.PolicyResets != 1 {
		t.Fatalf("frozen draw divider was not reset: %+v", result.Entries[drawDivider.Offset])
	}
	if result.Metrics.BoundClamps != 1 || result.Metrics.NonzeroRoundingErrors == 0 {
		t.Fatalf("unexpected quantization metrics: %+v", result.Metrics)
	}
}

func TestGenerateEngineCandidateSourceUsesBuildTagAndIndexedAssignments(t *testing.T) {
	registry, err := NewEngineRegistry()
	if err != nil {
		t.Fatal(err)
	}
	values, err := InitialExactParameters(registry)
	if err != nil {
		t.Fatal(err)
	}
	source, err := GenerateEngineCandidateSource(registry, values, EngineCandidateMetadata{
		DatasetManifestSHA256: "dataset", RegistryFingerprint: registry.Fingerprint, Checkpoint: "checkpoint.json",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"//go:build " + EngineCandidateBuildTag,
		"pieceValueMG[1] = 84",
		"PSQT_MG[1][8] = -5",
		"KingShelterMG[0][0] = -1",
		"DrawDivider = 8",
	} {
		if !bytes.Contains(source, []byte(expected)) {
			t.Errorf("generated source does not contain %q", expected)
		}
	}
	if strings.Contains(string(source), "PSQT_MG[1][0] =") {
		t.Fatal("generated source overwrites an unowned pawn PSQT storage cell")
	}
}

func TestRoundingPolicies(t *testing.T) {
	tests := []struct {
		policy RoundingPolicy
		value  float64
		want   int64
	}{{RoundNearest, -1.5, -2}, {RoundTowardZero, -1.9, -1}, {RoundFloor, -1.1, -2}, {RoundCeil, -1.9, -1}}
	for _, test := range tests {
		got, err := roundParameter(test.value, test.policy)
		if err != nil || got != test.want {
			t.Errorf("round %v with %s = %d, %v; want %d", test.value, test.policy, got, err, test.want)
		}
	}
}
