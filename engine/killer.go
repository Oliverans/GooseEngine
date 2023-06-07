package engine

import (
	"github.com/dylhunn/dragontoothmg"
)

type KillerStruct struct {
	KillerMoves map[int8]map[int]dragontoothmg.Move
}

var KillerMoveLength = 2
var KillerMoveScore = 10

func InsertKiller(move dragontoothmg.Move, ply int8, k KillerStruct) {
	for i := 0; i < (KillerMoveLength - 1); i++ {
		// Shift the moves
		k.KillerMoves[ply][0] = k.KillerMoves[ply][i+1]
		k.KillerMoves[ply][KillerMoveLength-1] = move
	}
}
