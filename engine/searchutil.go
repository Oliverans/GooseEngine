package engine

import (
	"fmt"
	"math/bits"

	"github.com/dylhunn/dragontoothmg"
)

var nodesChecked = 0
var cutNodes = 0
var TTNodes = 0
var KillerNodes = 0
var counterMove [2][64][64]dragontoothmg.Move
var historyMove [2][64][64]int
var historyMaxVal = 10000 // Ensure we stay below the captures, countermoves etc

func storeCounter(counterList *[2][64][64]dragontoothmg.Move, sideToMove bool, prevMove dragontoothmg.Move, move dragontoothmg.Move) {
	from := dragontoothmg.Square(prevMove.From())
	to := dragontoothmg.Square(prevMove.To())
	if sideToMove {
		counterMove[0][from][to] = move
	} else {
		counterMove[1][from][to] = move
	}
}

// Increment the history score for the given move if it caused a beta-cutoff and is quiet.
func incrementHistoryScore(b *dragontoothmg.Board, move dragontoothmg.Move, depth int8) {
	if b.Wtomove {
		historyMove[0][move.From()][move.To()] += int(depth * depth)
		if historyMove[0][move.From()][move.To()] >= historyMaxVal {
			ageHistoryTable(b.Wtomove)
		}
	} else {
		historyMove[1][move.From()][move.To()] += int(depth * depth)
		if historyMove[1][move.From()][move.To()] >= historyMaxVal {
			ageHistoryTable(b.Wtomove)
		}
	}
}

// Decrement the history score for the given move if it didn't cause a beta-cutoff and is quiet.
func decrementHistoryScore(b *dragontoothmg.Board, move dragontoothmg.Move) {
	if b.Wtomove {
		if historyMove[0][move.From()][move.To()] > 0 {
			historyMove[0][move.From()][move.To()] -= 1
		}
	} else {
		if historyMove[1][move.From()][move.To()] > 0 {
			historyMove[1][move.From()][move.To()] -= 1
		}
	}
}

// Age the values in the history table by halving them.
func ageHistoryTable(sideToMove bool) {
	for sq1 := 0; sq1 < 64; sq1++ {
		for sq2 := 0; sq2 < 64; sq2++ {
			if sideToMove {
				historyMove[0][sq1][sq2] /= 2
			} else {
				historyMove[1][sq1][sq2] /= 2
			}
		}
	}
}

// Clear the values in the history table.
func ClearHistoryTable() {
	for sq1 := 0; sq1 < 64; sq1++ {
		for sq2 := 0; sq2 < 64; sq2++ {
			historyMove[0][sq1][sq2] = 0
			historyMove[1][sq1][sq2] = 0
		}
	}
}

func Min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

func Max(x, y int) int {
	if x > y {
		return x
	}
	return y
}

func hasMinorOrMajorPiece(b *dragontoothmg.Board) (wCount int, bCount int) {
	wCount += bits.OnesCount64(b.White.Bishops | b.White.Knights | b.White.Rooks | b.White.Queens)
	bCount += bits.OnesCount64(b.Black.Bishops | b.Black.Knights | b.Black.Rooks | b.Black.Queens)
	return wCount, bCount
}

// Precomputed reductions
const MaxDepth = 100

var LMR = [MaxDepth + 1][100]int{}

func InitSearchTables() {
	for depth := 3; depth < 100; depth++ {
		for moveCnt := 3; moveCnt < 100; moveCnt++ {
			LMR[depth][moveCnt] = max(2, depth/4) + moveCnt/12
		}
	}

}

func getPVLineString(pvLine PVLine) (theMoves string) {
	for _, move := range pvLine.Moves {
		theMoves += " "
		theMoves += move.String()
	}
	return theMoves
}

// Taken from Blunder chess engine, since I'm very lazy
func getMateOrCPScore(score int) string {
	if int16(score) > (MaxScore - 50) {
		pliesToMate := int(MaxScore) - score
		mateInN := (pliesToMate / 2) + (pliesToMate % 2)
		return fmt.Sprintf("mate %d", mateInN)
	}

	if int16(score) < (MinScore + 50) {
		pliesToMate := int(MinScore) - score
		mateInN := (pliesToMate / 2) + (pliesToMate % 2)
		return fmt.Sprintf("mate %d", mateInN)
	}

	return fmt.Sprintf("cp %d", score)
}
