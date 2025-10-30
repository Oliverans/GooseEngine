package main

import (
	"chess-engine/engine"
	"fmt"
	"testing"

	"github.com/dylhunn/dragontoothmg"
)

func BenchmarkMain(b *testing.B) {
	board := dragontoothmg.ParseFen(dragontoothmg.Startpos) // the game board
	var bestmove = engine.StartSearch(&board, 10, 1000, 500, true, false)
	engine.ResetForNewGame()
	fmt.Println("bestmove ", bestmove)
}
