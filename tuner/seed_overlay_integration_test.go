//go:build tuner_seed

package tuner

import "testing"

func TestStage3CandidateSeedMatchesCompiledRegistry(t *testing.T) {
	candidate, err := LoadCandidate("../data/tuner-runs/stage3-full-center-pieceactivity/export-epoch20/candidate.json")
	if err != nil {
		t.Fatalf("load immutable Stage 3 candidate: %v", err)
	}
	if got, want := candidate.Provenance().SHA256, "ae8bbc21a09d5ef6e5896349a47de56ed54b1af807e0e22579174ec59cad630c"; got != want {
		t.Fatalf("candidate SHA-256 = %s, want %s", got, want)
	}
	registry, err := NewEngineRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if err := candidate.VerifySeedRegistry(registry); err != nil {
		t.Fatalf("compiled tuner_seed registry does not exactly match Stage 3 candidate: %v", err)
	}
	binding, err := NewTraceBinding(registry)
	if err != nil {
		t.Fatal(err)
	}
	model, err := NewForwardModel(registry, binding)
	if err != nil {
		t.Fatal(err)
	}
	link, err := NewTexelLink(0.00704896324708)
	if err != nil {
		t.Fatal(err)
	}
	trainer, err := NewTrainer(registry, model, link, nil, DefaultTrainerConfig())
	if err != nil {
		t.Fatal(err)
	}
	if got := trainer.OptimizerStep(); got != 0 {
		t.Fatalf("fresh chained trainer Adam step = %d, want 0", got)
	}
	if got := trainer.Cursor(); got != (TrainingCursor{}) {
		t.Fatalf("fresh chained trainer cursor = %+v, want zero cursor", got)
	}
	for index, got := range trainer.Parameters() {
		if want := float64(candidate.QuantizedValues[index]); got != want {
			t.Fatalf("fresh chained trainer parameter %d = %g, want Stage 3 integer %g", index, got, want)
		}
	}
}
