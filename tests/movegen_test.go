package goose_engine_mg_test

import (
	"testing"
	// Replace "chess-engine/goosemg" with the module path of the engine package when integrating.
	myengine "chess-engine/goosemg"
)

func TestMoveGenerationInitial(t *testing.T) {
	board, err := myengine.ParseFEN(myengine.FENStartPos)
	if err != nil {
		t.Fatalf("ParseFEN failed for initial position: %v", err)
	}
	moves := board.GenerateMoves()
	if len(moves) != 20 {
		t.Errorf("Initial position: expected 20 moves, got %d", len(moves))
	}
}
