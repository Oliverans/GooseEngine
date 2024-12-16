package engine

import (
	"math/bits"

	"github.com/dylhunn/dragontoothmg"
)

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

func InBetween(i, min, max int) bool {
	if (i >= min) && (i <= max) {
		return true
	} else {
		return false
	}
}

var (
	bitboardFileA uint64 = 0x0101010101010101
	bitboardFileH uint64 = 0x8080808080808080
)
var ClearRank [8]uint64
var MaskRank [8]uint64
var ranksAbove = [8]uint64{0xffffffffffffffff, 0xffffffffffffff00, 0xffffffffffff0000, 0xffffffffff000000, 0xffffffff00000000, 0xffffff0000000000, 0xffff000000000000, 0xff00000000000000}
var ranksBelow = [8]uint64{0xff, 0xffff, 0xffffff, 0xffffffff, 0xffffffffff, 0xffffffffffff, 0xffffffffffffff, 0xffffffffffffffff}

func getFileOfSquare(sq int) uint64 {
	return onlyFile[sq%8]
}

func getKingSafetyTable(b *dragontoothmg.Board, inner bool, wPawnAttackBB uint64, bPawnAttackBB uint64) [2]uint64 {
	var kingZoneTable [2]uint64
	kingBoards := [2]uint64{
		0: b.White.Kings,
		1: b.Black.Kings,
	}

	for i, board := range kingBoards {
		kingZoneBBInner := board
		kingSquare := bits.TrailingZeros64(kingZoneBBInner)
		rank := kingSquare / 8
		file := kingSquare % 8

		// If we're at the bottom or top rank, we should still keep the size of the king zone at a minimum of 3 wide/high
		if rank == 0 {
			kingZoneBBInner = kingZoneBBInner | (kingZoneBBInner << 8) | (kingZoneBBInner << 16)
		} else if rank == 7 {
			kingZoneBBInner = kingZoneBBInner | (kingZoneBBInner >> 8) | (kingZoneBBInner >> 16)
		} else {
			kingZoneBBInner = kingZoneBBInner | (kingZoneBBInner << 8) | (kingZoneBBInner >> 8)
		}

		if file == 0 {
			kingZoneBBInner = kingZoneBBInner | (kingZoneBBInner << 1) | (kingZoneBBInner << 2)
		} else if file == 7 {
			kingZoneBBInner = kingZoneBBInner | (kingZoneBBInner >> 1) | (kingZoneBBInner >> 2)
		} else {
			kingZoneBBInner = kingZoneBBInner | (((kingZoneBBInner & ^bitboardFileA) >> 1) | ((kingZoneBBInner & ^bitboardFileH) << 1))
		}

		if i == 0 {
			kingZoneBBInner &^= wPawnAttackBB
		} else {
			kingZoneBBInner &^= bPawnAttackBB
		}

		kingZoneTable[i] = kingZoneBBInner
	}

	if !inner {
		for i, board := range kingZoneTable {
			kingZoneBBOuter := board
			kingZoneBBOuter = kingZoneBBOuter | (kingZoneBBOuter << 8) | (kingZoneBBOuter >> 8)
			kingZoneBBOuter = kingZoneBBOuter | (((kingZoneBBOuter & ^bitboardFileA) >> 1) | ((kingZoneBBOuter & ^bitboardFileH) << 1))
			kingZoneBBOuter = kingZoneBBOuter &^ kingZoneTable[i]
			kingZoneTable[i] = kingZoneBBOuter
		}
	}
	return kingZoneTable
}

func getOutpostsBB(b *dragontoothmg.Board) (outpostSquares [2]uint64) {
	// Generate allowed ranks & files for outposts to be on
	wPotentialOutposts := (wPawnAttackBB & wAllowedOutpostMask) &^ b.White.Pawns
	bPotentialOutposts := (bPawnAttackBB & bAllowedOutpostMask) &^ b.Black.Pawns

	var wOutpostBB uint64
	var bOutpostBB uint64

	for x := wPotentialOutposts; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		sqBB := PositionBB[sq]
		if bits.OnesCount64(sqBB&wPotentialOutposts) > 0 {
			filesToCheck := (getFileOfSquare(sq-1) &^ bitboardFileH) | (getFileOfSquare(sq+1) &^ bitboardFileA)
			var ranksToCheckForEnemyPawns = ranksAbove[(sq/8)+1]
			if bits.OnesCount64(b.Black.Pawns&(filesToCheck&ranksToCheckForEnemyPawns)) == 0 {
				wOutpostBB = wOutpostBB | sqBB
			}
		}
	}

	for x := bPotentialOutposts; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		sqBB := PositionBB[sq]
		if bits.OnesCount64(sqBB&bPotentialOutposts) > 0 {
			filesToCheck := (getFileOfSquare(sq-1) &^ bitboardFileH) | (getFileOfSquare(sq+1) &^ bitboardFileA)
			var ranksToCheckForEnemyPawns = ranksBelow[(sq/8)-1]
			if bits.OnesCount64(b.White.Pawns&(filesToCheck&ranksToCheckForEnemyPawns)) == 0 {
				bOutpostBB = bOutpostBB | sqBB
			}
		}
	}

	outpostSquares[0] = wOutpostBB
	outpostSquares[1] = bOutpostBB
	return
}

func calculatePawnFileFill(pawnBitboard uint64, isWhite bool) uint64 {
	if isWhite {
		pawnBitboard |= calculatePawnNorthFill(pawnBitboard)
	} else {
		pawnBitboard |= calculatePawnSouthFill(pawnBitboard)
	}
	return pawnBitboard
}

func calculatePawnNorthFill(pawnBitboard uint64) uint64 {
	pawnBitboard = (pawnBitboard << 8)
	pawnBitboard |= (pawnBitboard << 16)
	pawnBitboard |= (pawnBitboard << 32)
	return pawnBitboard
}

func calculatePawnSouthFill(pawnBitboard uint64) uint64 {
	pawnBitboard = (pawnBitboard >> 8)
	pawnBitboard |= (pawnBitboard >> 16)
	pawnBitboard |= (pawnBitboard >> 32)
	return pawnBitboard
}

func isTheoreticalDraw(board *dragontoothmg.Board, debug bool) bool {
	pawnCount := bits.OnesCount64(board.White.Pawns | board.Black.Pawns)

	wKnights := bits.OnesCount64(board.White.Knights)
	wBishops := bits.OnesCount64(board.White.Bishops)
	wRooks := bits.OnesCount64(board.White.Rooks)
	wQueens := bits.OnesCount64(board.White.Queens)

	bKnights := bits.OnesCount64(board.Black.Knights)
	bBishops := bits.OnesCount64(board.Black.Bishops)
	bRooks := bits.OnesCount64(board.Black.Rooks)
	bQueens := bits.OnesCount64(board.Black.Queens)

	allPieces := bits.OnesCount64((board.White.All | board.Black.All) & ^(board.White.Kings | board.Black.Kings))
	if debug {
		println("All: ", allPieces, "\twQueen: ", wQueens, "\twRooks: ", wRooks, "\twKnights: ", wKnights, "\twBishops: ", wBishops)
		println("All: ", allPieces, "\tbQueen: ", bQueens, "\tbRooks: ", bRooks, "\tbKnights: ", bKnights, "\tbBishops: ", bBishops)
	}

	/*
		GENERAL DRAWS:
			ONE PIECE:
				- One knight				✓
				- One bishop				✓
			TWO PIECES:
				- two knights (same side)	✓
				- knight v knight			✓
				- bishop v bishop			✓
				- bishop v knight			✓
				- rook v rook				✓
				- queen v queen				✓

	*/
	if pawnCount == 0 {
		if allPieces == 1 { // single piece draw
			if wKnights == 1 || wBishops == 1 || bKnights == 1 || bBishops == 1 {
				return true
			}
		} else if allPieces == 2 { // Draws with only two major/minor pieces (where it generally is a draw)
			if (wKnights == 2 || bKnights == 2) || ((wBishops+wKnights > 0 && wBishops+wKnights < 2) && (bBishops+bKnights > 0 && bBishops+bKnights < 2)) {
				return true
			} else if (wRooks == 1 && (bBishops == 1 || bKnights == 1 || bRooks == 1)) || (bRooks == 1 && (wBishops == 1 || wKnights == 1 || wRooks == 1)) {
				return true
			} else if wQueens == 1 && bQueens == 1 {
				return true
			}
		}
	}

	return false
}
