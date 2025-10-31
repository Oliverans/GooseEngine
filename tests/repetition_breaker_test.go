package goose_engine_mg_test

import (
	"strings"
	"testing"

	myengine "chess-engine/goosemg"
)

// parseCoord converts a coordinate like "d2" into a Square.
func parseCoord(t *testing.T, sq string) myengine.Square {
	t.Helper()
	if len(sq) != 2 {
		t.Fatalf("invalid coord %q", sq)
	}
	file := int(sq[0] - 'a')
	rank := int(sq[1] - '1')
	if file < 0 || file > 7 || rank < 0 || rank > 7 {
		t.Fatalf("coord out of range: %q", sq)
	}
	return myengine.Square(rank*8 + file)
}

// findMoveRB finds a move by from/to squares (local helper for this file).
func findMoveRB(t *testing.T, b *myengine.Board, from, to myengine.Square) (myengine.Move, bool) {
	t.Helper()
	moves := b.GenerateMoves()
	for _, m := range moves {
		if m.From() == from && m.To() == to {
			return m, true
		}
	}
	return 0, false
}

func TestThreefoldRepetition_WithBreaker(t *testing.T) {
	b, err := myengine.ParseFEN(myengine.FENStartPos)
	if err != nil {
		t.Fatal(err)
	}

	var stack []myengine.MoveState
	var hist []uint64

	// Provided coordinate moves for threefold repetition scenario
	seq := "d2d4 g8f6 c2c4 g7g6 f2f3 d7d6 e2e4 e7e5 d4d5 f6h5 c1e3 f8g7 b1c3 e8g8 d1d2 f7f5 e1c1 f5f4 e3f2 g7f6 d2e1 b8d7 c1b1 f6e7 g2g3 c7c5 d5c6 b7c6 c4c5 d6c5 c3a4 d8c7 e1c3 a8b8 f1h3 d7b6 a4c5 f8f7 b2b3 f4g3 h2g3 e7c5 c3c5 h5g7 d1c1 c8e6 c5c6 c7e7 c6c5 e7f6 h3g2 f7b7 b1a1 b6d7 c5d6 g7e8 d6a6 e6b3 a6f6 e8f6 a2b3 b7b3 c1c2 b3b1 a1a2 b1b4 a2a1 b4b1 a1a2 b1b4 a2a1 b4b1"

	// Push moves and track repetition history using PushMove
	hist = append(hist, b.ComputeZobrist())
	for i, mv := range strings.Split(seq, " ") {
		if len(mv) != 4 {
			t.Fatalf("invalid move token %q at %d", mv, i)
		}
		from := parseCoord(t, mv[:2])
		to := parseCoord(t, mv[2:])
		m, ok := findMoveRB(t, b, from, to)
		if !ok {
			t.Fatalf("move %s not found at ply %d", mv, i)
		}
		if !b.PushMove(m, &stack, &hist) {
			t.Fatalf("illegal move %s at ply %d", mv, i)
		}
	}

	if !b.IsDrawByRepetition(hist) {
		t.Fatalf("expected threefold repetition after provided sequence")
	}
}
