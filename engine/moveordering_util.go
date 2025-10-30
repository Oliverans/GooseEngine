package engine

import (
	"github.com/dylhunn/dragontoothmg"
)

///////////////////////// Move pair struct /////////////////////////
type Pair struct {
	Key   dragontoothmg.Move
	Value int
}
type PairList []Pair

// Include sorting struct pairs :)
// Thx @ https://medium.com/@kdnotes/how-to-sort-golang-maps-by-value-and-key-eedc1199d944
// Obviously you can't sort a map, but I am sorting a Slice instead!
func (p PairList) Len() int           { return len(p) }
func (p PairList) Swap(j, i int)      { p[i], p[j] = p[j], p[i] }
func (p PairList) Less(j, i int) bool { return p[i].Value < p[j].Value }

/*
	HISTORY/COUNTER MOVES
	If a move was a cut-node (above beta), and not a capture, we keep track of two things:
	The move that countered us (previous move made) - a counter move
	A historical score of the move - since we know it was a good move to keep track of, we make sure we can use this for move ordering later
*/

var counterMove [2][64][64]dragontoothmg.Move
var historyMove [2][64][64]int
var historyMaxVal = 2000 // Ensure we stay below the captures, countermoves etc

func storeCounter(sideToMove bool, prevMove dragontoothmg.Move, move dragontoothmg.Move) {
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
			historyMove[0][move.From()][move.To()] /= 4
		}
	} else {
		if historyMove[1][move.From()][move.To()] > 0 {
			historyMove[1][move.From()][move.To()] /= 4
		}
	}
}

// Age the values in the history table by halving them.
func ageHistoryTable(sideToMove bool) {
	for sq1 := 0; sq1 < 64; sq1++ {
		for sq2 := 0; sq2 < 64; sq2++ {
			if sideToMove {
				historyMove[0][sq1][sq2] /= 8
			} else {
				historyMove[1][sq1][sq2] /= 8
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
