package engine

import (
	"fmt"
	"math/bits"

	"github.com/dylhunn/dragontoothmg"
)

var nodesChecked = 0
var LMR = [MaxDepth + 1][100]int8{}

func isThreefoldRepetition(posHash uint64) bool {
	var repeats int
	for _, h := range History.History {
		if h == posHash {
			repeats++
		}
	}
	return repeats >= 3
}

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

// A struct representing a principal variation line.
type PVLine struct {
	Moves []dragontoothmg.Move
}

// Clear the principal variation line.
func (pvLine *PVLine) Clear() {
	pvLine.Moves = nil
}

// Update the principal variation line with a new best move,
// and a new line of best play after the best move.
func (pvLine *PVLine) Update(move dragontoothmg.Move, newPVLine PVLine) {
	pvLine.Clear()
	pvLine.Moves = append(pvLine.Moves, move)
	pvLine.Moves = append(pvLine.Moves, newPVLine.Moves...)
}

// Get the best move from the principal variation line.
func (pvLine *PVLine) GetPVMove(board *dragontoothmg.Board) (move dragontoothmg.Move) {
	return pvLine.Moves[0]
}

func (pvLine *PVLine) IsPVMove(move dragontoothmg.Move) bool {
	for i := 0; i < len(pvLine.Moves); i++ {
		if pvLine.Moves[i] == move {
			return true
		}
	}
	return false
}

func (pvLine *PVLine) GetPVMoveAtDepth(depth int) dragontoothmg.Move {
	if depth >= 0 && depth <= len(pvLine.Moves)-1 {
		return pvLine.Moves[depth]
	}
	var nullMove dragontoothmg.Move
	return nullMove
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
	var nilMove dragontoothmg.Move
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
}
