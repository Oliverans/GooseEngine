package engine

import (
	gm "chess-engine/goosemg"
)

var History HistoryStruct

type KillerStruct struct {
	KillerMoves [MaxDepth + 1][2]gm.Move
}

var KillerMoveLength = 2
var KillerMoveScore = 10

func InsertKiller(move gm.Move, ply int8, k *KillerStruct) {
	index := int(ply)
	if index >= len(k.KillerMoves) {
		index = len(k.KillerMoves) - 1
	}
	if move != k.KillerMoves[index][0] {
		k.KillerMoves[index][1] = k.KillerMoves[index][0]
		k.KillerMoves[index][0] = move
	}
}

func ClearKillers(k *KillerStruct) {
	for i := range k.KillerMoves {
		k.KillerMoves[i][0] = 0
		k.KillerMoves[i][1] = 0
	}
}
