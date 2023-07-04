package main

import (
	"chess-engine/engine"
	"testing"

	"github.com/dylhunn/dragontoothmg"
)

func BenchmarkMain(b *testing.B) {
	board := dragontoothmg.ParseFen(dragontoothmg.Startpos) // the game board
	var bestmove = engine.StartSearch(&board, 50, 1000, 500, false, false)
	_ = bestmove
	//fmt.Println("bestmove ", bestmove)
}
