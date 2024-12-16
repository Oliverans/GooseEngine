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
	for depth := 1; depth < 100; depth++ {
		for moveCnt := 3; moveCnt < 100; moveCnt++ {
			// Current calculations comes from Weiss engine ....
			/*
				Modern engines have a different reduction of search depending on whether there's a capture or a quiet move

				Blunder engine uses the following (more simple) formula:
				max(2, depth/4) + moveCnt/12

				Weiss engine uses the following formula for quiet and captures respectively:
				1.82 + log10(depth) * log10(moveCnt) / 2.68
				0.38 + log10(depth) * log10(moveCnt) / 2.93

			*/
			//LMR[0][depth][moveCnt] = int8(max(2, depth/4) + moveCnt/12) //int8(1.82 + math.Log10(float64(depth))*math.Log10(float64(moveCnt))/2.68)
			LMR[depth][moveCnt] = int8(max(2, depth/4) + moveCnt/12) // int8(0.38 + math.Log10(float64(depth))*math.Log10(float64(moveCnt))/2.93)
		}
	}
}

// If we set any of the piece values to a custom value, we apply it here...
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
