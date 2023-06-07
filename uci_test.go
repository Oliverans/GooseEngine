package main

import (
	"chess-engine/engine"
	"fmt"
	"testing"

	"github.com/dylhunn/dragontoothmg"
)

func BenchmarkXxx(b *testing.B) {
	board := dragontoothmg.ParseFen(dragontoothmg.Startpos) // the game board
	var bestmove = engine.StartSearch(&board, 50, 1000, 500, false, false)
	fmt.Println("bestmove ", bestmove)
}
