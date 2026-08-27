package engine

import (
	"testing"

	gm "chess-engine/goosemg"
)

func TestStoppedBeforeFirstIterationReturnsLegalMove(t *testing.T) {
	board := gm.ParseFen(gm.Startpos)
	legalMoves := board.GenerateLegalMoves()
	SearchState.ResetForNewGame()
	SearchState.RequestStop()

	bestMove := StartSearch(&board, SearchLimits{Infinite: true}, false, false, false)
	found := false
	for _, move := range legalMoves {
		if move.String() == bestMove {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("immediately stopped search returned %q, want a legal root move", bestMove)
	}
}

func TestTerminalPositionReturnsUCINullMove(t *testing.T) {
	board := gm.ParseFen("7k/6Q1/7K/8/8/8/8/8 b - - 0 1")
	SearchState.ResetForNewGame()

	bestMove := StartSearch(&board, SearchLimits{Depth: 1}, false, false, false)
	if bestMove != "0000" {
		t.Fatalf("terminal search returned %q, want 0000", bestMove)
	}
}
