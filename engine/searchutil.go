package engine

import (
	"fmt"
	"math/bits"

	gm "chess-engine/goosemg"
)

var nodesChecked = 0
var LMR = [MaxDepth + 1][100]int8{}
var counterMove [2][64][64]gm.Move
var historyMove [2][64][64]int
var historyMaxVal = 10000 // Ensure we stay below the captures, countermoves etc

// God damn it Golang, why do I need to write my own Clamp function :(
func Clamp(f, low, high int8) int8 {
	if f < low {
		return low
	}
	if f > high {
		return high
	}
	return f
}

// To keep track of 3-fold repetition and/or 50 move draw
type HistoryStruct struct {
	History             []uint64
	HalfclockRepetition int
}

/*
HISTORY/COUNTER MOVES
If a move was a cut-node (above beta), and not a capture, we keep track of two things:
The move that countered us (previous move made) - a counter move
A historical score of the move - since we know it was a good move to keep track of, we make sure we can use this for move ordering later
*/
func storeCounter(sideToMove bool, prevMove gm.Move, move gm.Move) {
	from := gm.Square(prevMove.From())
	to := gm.Square(prevMove.To())
	if sideToMove {
		counterMove[0][from][to] = move
	} else {
		counterMove[1][from][to] = move
	}
}

// Increment the history score for the given move if it caused a beta-cutoff and is quiet.
func incrementHistoryScore(b *gm.Board, move gm.Move, depth int8) {
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
func decrementHistoryScore(b *gm.Board, move gm.Move) {
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

func hasMinorOrMajorPiece(b *gm.Board) (wCount int, bCount int) {
	wCount += bits.OnesCount64(b.White.Bishops | b.White.Knights | b.White.Rooks | b.White.Queens)
	bCount += bits.OnesCount64(b.Black.Bishops | b.Black.Knights | b.Black.Rooks | b.Black.Queens)
	return wCount, bCount
}

// Precomputed reductions
const MaxDepth = 100

func getPVLineString(pvLine PVLine) (theMoves string) {
	for _, move := range pvLine.Moves {
		theMoves += " "
		theMoves += move.String()
	}
	return theMoves
}

// Taken from Blunder chess engine and just slightly modified, since I'm very lazy; works great though :)
func getMateOrCPScore(score int) string {
	if int16(score) > Checkmate {
		pliesToMate := int(MaxScore) - score
		mateInN := (pliesToMate / 2) + (pliesToMate % 2)
		return fmt.Sprintf("mate %d", mateInN)
	} else if int16(score) < -Checkmate {
		pliesToMate := -int(MaxScore) - score
		mateInN := (pliesToMate / 2) + (pliesToMate % 2)
		return fmt.Sprintf("mate %d", -mateInN)
	}

	return fmt.Sprintf("cp %d", score)
}

func ResetForNewGame() {
	TT.clearTT()
	var nilMove gm.Move
	for i := 0; i < 64; i++ {
		for z := 0; z < 64; z++ {
			counterMove[0][i][z] = nilMove
			counterMove[1][i][z] = nilMove
		}
	}

	for i := 0; i < 64; i++ {
		for z := 0; z < 64; z++ {
			historyMove[0][i][z] = 0
			historyMove[1][i][z] = 0
		}
	}
	prevSearchScore = 0
}
