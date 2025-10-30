package goose_engine_mg_test

import (
    "strings"
    "testing"

    myengine "github.com/Oliverans/GooseEngineMG/goosemg"
)

// parseCoord converts a coordinate like "d2" into a Square.
func parseCoordD50(t *testing.T, sq string) myengine.Square {
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

// findMoveD50 finds a move by from/to squares (local helper for this file).
func findMoveD50(t *testing.T, b *myengine.Board, from, to myengine.Square) (myengine.Move, bool) {
	t.Helper()
	moves := b.GenerateMoves()
	for _, m := range moves {
		if m.From() == from && m.To() == to {
			return m, true
		}
	}
	return 0, false
}

func TestFiftyMoveRule_Scenario(t *testing.T) {
	b, err := myengine.ParseFEN(myengine.FENStartPos)
	if err != nil {
		t.Fatal(err)
	}

	var stack []myengine.MoveState
	var hist []uint64
	var applied []myengine.Move

	// Provided coordinate moves for the 50-move rule scenario
	seq := "d2d4 d7d5 f2f4 f7f5 e2e3 e7e6 g2g3 g7g6 h2h4 h7h5 c2c3 c7c6 b2b4 b7b5 a2a3 a7a6 b1d2 g8e7 f1g2 c8b7 e1f2 e8f7 d1e2 f8g7 h1h3 a8a7 c1b2 b8d7 a1c1 b7c8 c1b1 d7f8 g1f3 f8h7 d2f1 e7g8 f1d2 g8e7 d2f1 e7g8 f1h2 g8h6 f3g5 f7f8 e2c2 f8e7 b1d1 c8b7 f2e2 g7f8 g2f3 h7f6 c2c1 d8c8 c1a1 c8a8 d1g1 b7c8 h2f1 h8h7 h3h2 h7h8 f1d2 f8g7 d2f1 c8d7 a1c1 a8b7 b2a1 a7a8 f1d2 h8c8 g1g2 c8f8 h2h1 f8g8 g2g1 g8h8 g5h3 h6g8 d2f1 g8h6 f1h2 f6g4 h2f1 g4f6 f1d2 g7f8 g1e1 b7c7 h1g1 f8g7 f3h1 h8b8 e1f1 d7e8 d2b3 e8d7 b3c5 f6e4 h3g5 h6g4 c5b3 e4f6 g5h3 g4h6 h1f3 f6g8 g1h1 g7f6 f1f2 e7d8 e2f1 d8c8 f1g2 c8b7"

	for i, mv := range strings.Split(seq, " ") {
		if len(mv) != 4 {
			t.Fatalf("invalid move token %q at %d", mv, i)
		}
		from := parseCoordD50(t, mv[:2])
		to := parseCoordD50(t, mv[2:])
		m, ok := findMoveD50(t, b, from, to)
		if !ok {
			t.Fatalf("move %s not found at ply %d", mv, i)
		}
		if !b.PushMove(m, &stack, &hist) {
			t.Fatalf("illegal move %s at ply %d", mv, i)
		}
		applied = append(applied, m)
	}

	if !b.IsDrawBy50() {
		// Diagnostic: locate last irreversible move (pawn move or capture)
		isPawn := func(p myengine.Piece) bool { return (p & 7) == 1 }
		last := -1
		for i := len(applied) - 1; i >= 0; i-- {
			m := applied[i]
			if isPawn(m.MovedPiece()) || m.CapturedPiece() != myengine.NoPiece {
				last = i
				break
			}
		}
		sq := func(s myengine.Square) string {
			f := int(s) % 8
			r := int(s) / 8
			return string('a'+byte(f)) + string('1'+byte(r))
		}
		if last >= 0 {
			start := last - 4
			if start < 0 {
				start = 0
			}
			end := last + 4
			if end > len(applied)-1 {
				end = len(applied) - 1
			}
			t.Logf("HalfmoveClock=%d; last irreversible ply=%d; since=%d", b.HalfmoveClock(), last, len(applied)-1-last)
			for j := start; j <= end; j++ {
				m := applied[j]
				t.Logf("%3d: %s->%s moved=%d captured=%d promo=%d", j, sq(m.From()), sq(m.To()), m.MovedPiece(), m.CapturedPiece(), m.PromotionPiece())
			}
		} else {
			t.Logf("HalfmoveClock=%d; no irreversible move found in applied list", b.HalfmoveClock())
		}
		t.Fatalf("expected 50-move rule draw, got halfmoveClock=%d", b.HalfmoveClock())
	}
}
