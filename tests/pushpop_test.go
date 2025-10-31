package goose_engine_mg_test

import (
	myengine "chess-engine/goosemg"
	"testing"
)

// local helper duplicate
func findMovePP(t *testing.T, b *myengine.Board, from, to myengine.Square) (myengine.Move, bool) {
	t.Helper()
	moves := b.GenerateMoves()
	for _, m := range moves {
		if m.From() == from && m.To() == to {
			return m, true
		}
	}
	return 0, false
}

func TestPushPopRoundTrip(t *testing.T) {
	b, err := myengine.ParseFEN(myengine.FENStartPos)
	if err != nil {
		t.Fatal(err)
	}
	startFEN := b.ToFEN()
	startZ := b.ComputeZobrist()

	var stack []myengine.MoveState
	var hist []uint64

	// e2e4, e7e5
	e2 := myengine.Square(1*8 + 4)
	e4 := myengine.Square(3*8 + 4)
	e7 := myengine.Square(6*8 + 4)
	e5 := myengine.Square(4*8 + 4)

	m1, ok := findMovePP(t, b, e2, e4)
	if !ok {
		t.Fatalf("e2e4 not found")
	}
	if !b.PushMove(m1, &stack, &hist) {
		t.Fatalf("PushMove e2e4 failed")
	}

	m2, ok := findMovePP(t, b, e7, e5)
	if !ok {
		t.Fatalf("e7e5 not found")
	}
	if !b.PushMove(m2, &stack, &hist) {
		t.Fatalf("PushMove e7e5 failed")
	}

	// Pop twice
	b.PopMove(&stack, &hist)
	b.PopMove(&stack, &hist)

	if b.ToFEN() != startFEN {
		t.Fatalf("FEN mismatch after pop: got %q want %q", b.ToFEN(), startFEN)
	}
	if b.ComputeZobrist() != startZ {
		t.Fatalf("Zobrist mismatch after pop")
	}
	if len(stack) != 0 || len(hist) != 0 {
		t.Fatalf("stack/history not empty after pops")
	}
}
