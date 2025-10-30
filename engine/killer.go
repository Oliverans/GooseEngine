package engine

import (
	"github.com/dylhunn/dragontoothmg"
)

var History HistoryStruct

type KillerStruct struct {
	KillerMoves [MaxDepth + 1][2]dragontoothmg.Move
}

func (k *KillerStruct) InsertKiller(move dragontoothmg.Move, ply int8) {
	if move != k.KillerMoves[ply][0] {
		k.KillerMoves[ply][1] = k.KillerMoves[ply][0]
		k.KillerMoves[ply][0] = move
	}
}

// Clear the killer moves table.
func (k *KillerStruct) ClearKillers() {
	for depth := 0; depth < MaxDepth+1; depth++ {
		k.KillerMoves[depth][0] = EmptyMove
		k.KillerMoves[depth][1] = EmptyMove
	}
}
