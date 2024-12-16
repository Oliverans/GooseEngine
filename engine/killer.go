package engine

import (
	"github.com/dylhunn/dragontoothmg"
)

var HistoryMap map[uint64]int = make(map[uint64]int, 5000)
var History HistoryStruct

type KillerStruct struct {
	KillerMoves [MaxDepth + 1][2]dragontoothmg.Move
}

var KillerMoveLength = 2
var KillerMoveScore = 10

func InsertKiller(move dragontoothmg.Move, ply int8, k *KillerStruct) {
	if move != k.KillerMoves[ply][0] {
		k.KillerMoves[ply][1] = k.KillerMoves[ply][0]
		k.KillerMoves[ply][0] = move
	}
}
