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

var KnightMasks = [64]uint64{
	0x0000000000020400, 0x0000000000050800, 0x00000000000a1100, 0x0000000000142200,
	0x0000000000284400, 0x0000000000508800, 0x0000000000a01000, 0x0000000000402000,
	0x0000000002040004, 0x0000000005080008, 0x000000000a110011, 0x0000000014220022,
	0x0000000028440044, 0x0000000050880088, 0x00000000a0100010, 0x0000000040200020,
	0x0000000204000402, 0x0000000508000805, 0x0000000a1100110a, 0x0000001422002214,
	0x0000002844004428, 0x0000005088008850, 0x000000a0100010a0, 0x0000004020002040,
	0x0000020400040200, 0x0000050800080500, 0x00000a1100110a00, 0x0000142200221400,
	0x0000284400442800, 0x0000508800885000, 0x0000a0100010a000, 0x0000402000204000,
	0x0002040004020000, 0x0005080008050000, 0x000a1100110a0000, 0x0014220022140000,
	0x0028440044280000, 0x0050880088500000, 0x00a0100010a00000, 0x0040200020400000,
	0x0204000402000000, 0x0508000805000000, 0x0a1100110a000000, 0x1422002214000000,
	0x2844004428000000, 0x5088008850000000, 0xa0100010a0000000, 0x4020002040000000,
	0x0400040200000000, 0x0800080500000000, 0x1100110a00000000, 0x2200221400000000,
	0x4400442800000000, 0x8800885000000000, 0x100010a000000000, 0x2000204000000000,
	0x0004020000000000, 0x0008050000000000, 0x00110a0000000000, 0x0022140000000000,
	0x0044280000000000, 0x0088500000000000, 0x0010a00000000000, 0x0020400000000000,
}

// Include sorting struct pairs :)
// Thx @ https://medium.com/@kdnotes/how-to-sort-golang-maps-by-value-and-key-eedc1199d944
// Obviously you can't sort a map, but I am sorting a Slice instead!
func (p PairList) Len() int           { return len(p) }
func (p PairList) Swap(j, i int)      { p[i], p[j] = p[j], p[i] }
func (p PairList) Less(j, i int) bool { return p[i].Value < p[j].Value }

// TBD; DELETE ONCE I'M SURE I HATE THIS
//func sortStruct(board *dragontoothmg.Board, moves map[dragontoothmg.Move]int) PairList {
//	p := make(PairList, len(moves))
//	i := len(moves) - 1
//
//	for k, v := range moves {
//		p[i] = Pair{k, v}
//		i--
//	}
//
//	sort.Slice(p[:], func(i, j int) bool {
//		if p[i].Value == p[j].Value {
//			return p[i].Key > p[j].Key
//		} else {
//			return p[i].Value > p[j].Value
//		}
//	})
//	return p
//}

///////////////////////// Drawing board for debug //////////////////////

func Union(i ...uint64) uint64 {
	var u uint64
	for _, v := range i {
		u = u | v
	}
	return u
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
func (pvLine *PVLine) GetPVMove() dragontoothmg.Move {
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
