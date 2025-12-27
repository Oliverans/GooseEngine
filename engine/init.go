package engine

import (
	"math"
	"sync"

	gm "chess-engine/goosemg"
)

var positionBBOnce sync.Once
var lmrTableOnce sync.Once
var passedPawnMasksOnce sync.Once

func initVariables(_ *gm.Board) {
	// Search tables ...
	lmrTableOnce.Do(InitLMRTable)
	positionBBOnce.Do(InitPositionBB)
	passedPawnMasksOnce.Do(InitPassedPawnMasks)
}

func InitPassedPawnMasks() {
	for sq := 0; sq < 64; sq++ {
		file := sq & 7
		rank := sq / 8

		// White: squares ahead on same and adjacent files
		for r := rank + 1; r <= 7; r++ {
			if file > 0 {
				PassedMaskWhite[sq] |= PositionBB[r*8+file-1]
			}
			PassedMaskWhite[sq] |= PositionBB[r*8+file]
			if file < 7 {
				PassedMaskWhite[sq] |= PositionBB[r*8+file+1]
			}
		}

		// Black: squares behind (from white's view) on same and adjacent files
		for r := 0; r < rank; r++ {
			if file > 0 {
				PassedMaskBlack[sq] |= PositionBB[r*8+file-1]
			}
			PassedMaskBlack[sq] |= PositionBB[r*8+file]
			if file < 7 {
				PassedMaskBlack[sq] |= PositionBB[r*8+file+1]
			}
		}
	}
}

func InitPositionBB() {
	for i := 0; i <= 64; i++ {
		PositionBB[i] = uint64(math.Pow(float64(2), float64(i)))
		sqBB := PositionBB[i]

		// Generate king moves lookup table.

		top := sqBB >> 8
		topRight := (sqBB >> 8 >> 1) & ^bitboardFileH
		topLeft := (sqBB >> 8 << 1) & ^bitboardFileA

		right := (sqBB >> 1) & ^bitboardFileH
		left := (sqBB << 1) & ^bitboardFileA

		bottom := sqBB << 8
		bottomRight := (sqBB << 8 >> 1) & ^bitboardFileH
		bottomLeft := (sqBB << 8 << 1) & ^bitboardFileA

		kingMoves := top | topRight | topLeft | right | left | bottom | bottomRight | bottomLeft

		KingMoves[i] = kingMoves
	}
}

// Late-move reduction tables
func InitLMRTable() {
	for depth := 1; depth < 64; depth++ {
		for moveCnt := 1; moveCnt < 64; moveCnt++ {
			base := 0.0
			if depth >= 2 && moveCnt >= 2 {
				base = 0.8 + math.Log(float64(depth))*math.Log(float64(moveCnt))/2.5
			}
			r := int(base + 0.5) // round

			if r < 0 {
				r = 0
			}
			if r > depth-1 {
				r = depth - 1
			}
			LMR[depth][moveCnt] = int8(r)
		}
	}
}
