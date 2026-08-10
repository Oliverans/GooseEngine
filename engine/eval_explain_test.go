package engine

import (
	"testing"

	gm "chess-engine/goosemg"
)

func TestEvalExplain_SplitAggregatesMatch(t *testing.T) {
	board := gm.ParseFen(gm.Startpos)
	_, b := EvalExplain(&board)

	if b.Activity != b.KnightActivity+b.BishopActivity+b.RookActivity+b.QueenActivity {
		t.Fatalf("activity aggregate mismatch: %d vs split sum %d", b.Activity, b.KnightActivity+b.BishopActivity+b.RookActivity+b.QueenActivity)
	}
	if b.KingSafety != b.KingAttackPressure+b.KingPawnShield+b.KingEndgame {
		t.Fatalf("king safety aggregate mismatch: %d vs split sum %d", b.KingSafety, b.KingAttackPressure+b.KingPawnShield+b.KingEndgame)
	}
}
