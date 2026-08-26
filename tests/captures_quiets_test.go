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
	wantCap := map[string]bool{"a7b8q": true, "a7b8r": true, "a7b8b": true, "a7b8n": true}
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
	wantQuiet := map[string]bool{"a7a8q": true, "a7a8r": true, "a7a8b": true, "a7a8n": true}
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

func TestTacticalsIncludeCapturesAndQueenPromotions(t *testing.T) {
	tests := []struct {
		fen  string
		want map[string]bool
	}{
		{
			fen: "1n5k/P7/8/8/8/8/8/7K w - - 0 1",
			want: map[string]bool{
				"a7a8q": true,
				"a7b8q": true,
				"a7b8r": true,
				"a7b8b": true,
				"a7b8n": true,
			},
		},
		{
			fen: "7k/8/8/8/8/8/p7/1N5K b - - 0 1",
			want: map[string]bool{
				"a2a1q": true,
				"a2b1q": true,
				"a2b1r": true,
				"a2b1b": true,
				"a2b1n": true,
			},
		},
	}

	for _, test := range tests {
		b, err := myengine.ParseFEN(test.fen)
		if err != nil {
			t.Fatal(err)
		}

		moves := b.GenerateTacticalsInto(make([]myengine.Move, 0, 16))
		if len(moves) != len(test.want) {
			t.Fatalf("tacticals: got %v want %v", moves, test.want)
		}
		for _, move := range moves {
			if !test.want[move.String()] {
				t.Fatalf("unexpected tactical move %s", move.String())
			}
		}
	}
}

func TestTacticalsMatchCapturesPlusQuietQueenPromotions(t *testing.T) {
	fens := []string{
		myengine.FENStartPos,
		"r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1",
		"k7/8/8/3pP3/8/8/8/7K w - d6 0 2",
		"1n5k/P7/8/8/8/8/8/7K w - - 0 1",
		"7k/8/8/8/8/8/p7/1N5K b - - 0 1",
	}

	for _, fen := range fens {
		b, err := myengine.ParseFEN(fen)
		if err != nil {
			t.Fatal(err)
		}

		want := make(map[myengine.Move]bool)
		for _, move := range b.GenerateCaptures() {
			want[move] = true
		}
		for _, move := range b.GenerateQuiets() {
			if move.PromotionPieceType() == myengine.PieceTypeQueen {
				want[move] = true
			}
		}

		moves := b.GenerateTacticalsInto(make([]myengine.Move, 0, 128))
		if len(moves) != len(want) {
			t.Fatalf("%s: got %v want %v", fen, moves, want)
		}
		for _, move := range moves {
			if !want[move] {
				t.Fatalf("%s: unexpected tactical move %s", fen, move.String())
			}
		}
	}
}
