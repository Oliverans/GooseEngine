package engine

import (
	gm "chess-engine/goosemg"
)

// /////////////////////// Move pair struct /////////////////////////
type Pair struct {
	Key   gm.Move
	Value int
}
type PairList []Pair

// Masks for attacks
// In order: knight on A1, B1, C1, ... F8, G8, H8

func PawnCaptureBitboards(pawnBoard uint64, wToMove bool) (east uint64, west uint64) {
	if wToMove {
		west = (pawnBoard << 8 << 1) & ^bitboardFileA
		east = (pawnBoard << 8 >> 1) & ^bitboardFileH
	} else {
		west = (pawnBoard >> 8 << 1) & ^bitboardFileA
		east = (pawnBoard >> 8 >> 1) & ^bitboardFileH
	}
	return
}

// Include sorting struct pairs :)
// Thx @ https://medium.com/@kdnotes/how-to-sort-golang-maps-by-value-and-key-eedc1199d944
// Obviously you can't sort a map, but I am sorting a Slice instead!
func (p PairList) Len() int           { return len(p) }
func (p PairList) Swap(j, i int)      { p[i], p[j] = p[j], p[i] }
func (p PairList) Less(j, i int) bool { return p[i].Value < p[j].Value }

// A struct representing a principal variation line.
type PVLine struct {
	Moves []gm.Move
}

// Clear the principal variation line.
func (pvLine *PVLine) Clear() {
	pvLine.Moves = pvLine.Moves[:0]
}

// Update the principal variation line with a new best move,
// and a new line of best play after the best move.
func (pvLine *PVLine) Update(move gm.Move, newPVLine PVLine) {
	pvLine.Clear()
	pvLine.Moves = append(pvLine.Moves, move)
	pvLine.Moves = append(pvLine.Moves, newPVLine.Moves...)
}

// Get the best move from the principal variation line.
func (pvLine *PVLine) GetPVMove() gm.Move {
	return pvLine.Moves[0]
}

func (pvLine *PVLine) IsPVMove(move gm.Move) bool {
	for i := 0; i < len(pvLine.Moves); i++ {
		if pvLine.Moves[i] == move {
			return true
		}
	}
	return false
}

func (pvLine *PVLine) GetPVMoveAtDepth(depth int) gm.Move {
	if depth >= 0 && depth <= len(pvLine.Moves)-1 {
		return pvLine.Moves[depth]
	}
	var nullMove gm.Move
	return nullMove
}
