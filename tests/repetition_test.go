package goose_engine_mg_test

import (
    myengine "github.com/Oliverans/GooseEngineMG/goosemg"
    "testing"
)

// findMove finds a move by from/to squares.
func findMove(t *testing.T, b *myengine.Board, from, to myengine.Square) (myengine.Move, bool) {
	t.Helper()
	moves := b.GenerateMoves()
	for _, m := range moves {
		if m.From() == from && m.To() == to {
			return m, true
		}
	}
	return 0, false
}

func TestThreefoldRepetition_KnightShuffle(t *testing.T) {
	b, err := myengine.ParseFEN(myengine.FENStartPos)
	if err != nil {
		t.Fatal(err)
	}

	// Track zobrist history (positions before the current)
	var hist []uint64
	hist = append(hist, b.ComputeZobrist())

	// Helper to play a specific move and record history
	play := func(from, to myengine.Square) {
		m, ok := findMove(t, b, from, to)
		if !ok {
			t.Fatalf("move %v->%v not found", from, to)
		}
		ok2, st := b.MakeMove(m)
		if !ok2 {
			t.Fatalf("move %v->%v illegal unexpectedly", from, to)
		}
		hist = append(hist, b.ComputeZobrist())
		// Do not unmake; we build a sequence, then test repetition at the end
		_ = st
	}

	g1 := myengine.Square(6)
	f3 := myengine.Square(2*8 + 5) // 21
	g8 := myengine.Square(7*8 + 6) // 62
	f6 := myengine.Square(5*8 + 5) // 45

	// One cycle (returns to initial position)
	play(g1, f3) // W: Ng1-f3
	play(g8, f6) // B: Ng8-f6
	play(f3, g1) // W: Nf3-g1
	play(f6, g8) // B: Nf6-g8 (position equals initial)

	if b.IsDrawByRepetition(hist) {
		t.Fatalf("should not be threefold yet after one cycle")
	}

	// Second cycle
	play(g1, f3)
	play(g8, f6)
	play(f3, g1)
	play(f6, g8) // Third occurrence of initial position

	if !b.IsDrawByRepetition(hist) {
		t.Fatalf("expected threefold repetition after two cycles")
	}
}

func TestFiftyMoveRuleWithPushes(t *testing.T) {
	b, err := myengine.ParseFEN(myengine.FENStartPos)
	if err != nil {
		t.Fatal(err)
	}

	var stack []myengine.MoveState
	var hist []uint64

	g1 := myengine.Square(6)
	f3 := myengine.Square(2*8 + 5)
	g8 := myengine.Square(7*8 + 6)
	f6 := myengine.Square(5*8 + 5)

	// Perform 100 half-moves without pawn moves or captures
	for i := 0; i < 25; i++ {
		m, ok := findMovePP(t, b, g1, f3)
		if !ok {
			t.Fatalf("Ng1-f3 not found at i=%d", i)
		}
		if !b.PushMove(m, &stack, &hist) {
			t.Fatalf("push Ng1-f3 failed at i=%d", i)
		}
		m, ok = findMovePP(t, b, g8, f6)
		if !ok {
			t.Fatalf("Ng8-f6 not found at i=%d", i)
		}
		if !b.PushMove(m, &stack, &hist) {
			t.Fatalf("push Ng8-f6 failed at i=%d", i)
		}
		m, ok = findMovePP(t, b, f3, g1)
		if !ok {
			t.Fatalf("Nf3-g1 not found at i=%d", i)
		}
		if !b.PushMove(m, &stack, &hist) {
			t.Fatalf("push Nf3-g1 failed at i=%d", i)
		}
		m, ok = findMovePP(t, b, f6, g8)
		if !ok {
			t.Fatalf("Nf6-g8 not found at i=%d", i)
		}
		if !b.PushMove(m, &stack, &hist) {
			t.Fatalf("push Nf6-g8 failed at i=%d", i)
		}
	}

	if !b.IsDrawBy50() {
		t.Fatalf("expected 50-move rule draw after 100 halfmoves, got halfmoveClock=%d", b.HalfmoveClock())
	}
}
