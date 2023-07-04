package engine

import (
	"math"

	"github.com/dylhunn/dragontoothmg"
)

func initVariables(board *dragontoothmg.Board) {
	// Search tables ...
	InitLMRTable()
	initPositionBB()
	setPieceValues()
}

func initPositionBB() {
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
	for depth := 3; depth < 100; depth++ {
		for moveCnt := 3; moveCnt < 100; moveCnt++ {
			LMR[depth][moveCnt] = max(2, depth/4) + moveCnt/12
		}
	}
}

func setPieceValues() {
	for _, pieceType := range pieceList {
		switch pieceType {
		case dragontoothmg.Pawn:
			pieceValueMG[pieceType] = PawnValueMG
			pieceValueEG[pieceType] = PawnValueEG
		case dragontoothmg.Knight:
			pieceValueMG[pieceType] = KnightValueMG
			pieceValueEG[pieceType] = KnightValueEG
		case dragontoothmg.Bishop:
			pieceValueMG[pieceType] = BishopValueMG
			pieceValueEG[pieceType] = BishopValueMG
		case dragontoothmg.Rook:
			pieceValueMG[pieceType] = RookValueMG
			pieceValueEG[pieceType] = RookValueEG
		case dragontoothmg.Queen:
			pieceValueMG[pieceType] = QueenValueMG
			pieceValueEG[pieceType] = QueenValueEG
		}
	}
}
