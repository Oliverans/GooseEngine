package goose_engine_mg_test

import (
	myengine "chess-engine/goosemg"
	"testing"
)

func TestCapturesInitialZero(t *testing.T) {
	b, err := myengine.ParseFEN(myengine.FENStartPos)
	if err != nil {
		t.Fatal(err)
	}
	got := b.GenerateCaptures()
	if len(got) != 0 {
		t.Fatalf("initial captures: got %d want 0", len(got))
	}
}

func TestCapturesEnPassant(t *testing.T) {
	fen := "k7/8/8/3pP3/8/8/8/7K w - d6 0 2"
	b, err := myengine.ParseFEN(fen)
	if err != nil {
		t.Fatal(err)
	}
	caps := b.GenerateCaptures()
	var epCount int
	for _, m := range caps {
		if m.Flags() == myengine.FlagEnPassant {
			epCount++
		}
	}
	if epCount != 1 {
		t.Fatalf("expected exactly 1 en passant capture, got %d (total captures=%d)", epCount, len(caps))
	}
}

func TestPromotionCapturesAndQuiets(t *testing.T) {
	fen := "1n5k/P7/8/8/8/8/8/7K w - - 0 1"
	b, err := myengine.ParseFEN(fen)
	if err != nil {
		t.Fatal(err)
	}

	caps := b.GenerateCaptures()
	// Expect 4 capture promotions from a7xb8=Q/R/B/N
	wantCap := map[string]bool{"a7b8Q": true, "a7b8R": true, "a7b8B": true, "a7b8N": true}
	var haveCap = map[string]bool{}
	for _, m := range caps {
		haveCap[m.String()] = true
	}
	for s := range wantCap {
		if !haveCap[s] {
			t.Fatalf("missing capture promotion %s; got=%v", s, haveCap)
		}
	}

	quiets := b.GenerateQuiets()
	// Expect 4 quiet promotions from a7a8=Q/R/B/N
	wantQuiet := map[string]bool{"a7a8Q": true, "a7a8R": true, "a7a8B": true, "a7a8N": true}
	var haveQuiet = map[string]bool{}
	for _, m := range quiets {
		haveQuiet[m.String()] = true
	}
	for s := range wantQuiet {
		if !haveQuiet[s] {
			t.Fatalf("missing quiet promotion %s; got=%v", s, haveQuiet)
		}
	}
}
