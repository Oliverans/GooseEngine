package main

import (
	"chess-engine/engine"
	"fmt"
	"testing"

	gm "chess-engine/goosemg"
)

func BenchmarkMain(b *testing.B) {
	board := gm.ParseFen(gm.Startpos) // the game board
	var bestmove = engine.StartSearch(&board, 50, 1000, 500, 0, false, false, false, false)
	engine.SearchState.ResetForNewGame()
	fmt.Println("bestmove ", bestmove)
}
