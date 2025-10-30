package engine

import (
	"math"
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

// Disguting Golang helper...
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
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
var ranksAbove = [8]uint64{0xffffffffffffff00, 0xffffffffffff0000, 0xffffffffff000000, 0xffffffff00000000, 0xffffff0000000000, 0xffff000000000000, 0xff00000000000000, 0}
var ranksBelow = [8]uint64{0, 0xff, 0xffff, 0xffffff, 0xffffffff, 0xffffffffff, 0xffffffffffff, 0xffffffffffffff}

var fifthAndSixthRank uint64 = 0xffff00000000
var thirdAndFourthRank uint64 = 0xffff0000

/*
	Helper function for multi-piece purposes
		- Outposts
		- Open/Semi-open files
*/

func getOutpostsBB(wPawnAttacksBB, bPawnAttacksBB, wPawnAttackSpanBB, bPawnAttackSpanBB uint64) (outpostSquares [2]uint64) {
	// Generate allowed ranks & files for outposts to be on
	var wPotentialOutposts uint64 = (wPawnAttacksBB & wAllowedOutpostMask)
	var bPotentialOutposts uint64 = (bPawnAttacksBB & bAllowedOutpostMask)

	var wOutpostBB uint64
	var bOutpostBB uint64

	for x := wPotentialOutposts; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		sqBB := PositionBB[sq]

		if bPawnAttackSpanBB&sqBB == 0 {
			wOutpostBB |= sqBB
		}
	}

	for x := bPotentialOutposts; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		sqBB := PositionBB[sq]
		if wPawnAttackSpanBB&sqBB == 0 {
			bOutpostBB |= sqBB
		}
	}

	outpostSquares[0] = wOutpostBB
	outpostSquares[1] = bOutpostBB
	return
}

func getOpenFiles(b *dragontoothmg.Board) (openFiles, wSemiOpenFiles, bSemiOpenFiles uint64) {
	for i := 0; i < 8; i++ {
		var currFile uint64 = onlyFile[i]

		// Open files
		if currFile&(b.White.Pawns|b.Black.Pawns) == 0 {
			openFiles |= currFile
		}

		// White semi-open
		if currFile&(b.White.Pawns) == 0 && currFile&(b.Black.Pawns) > 0 {
			wSemiOpenFiles |= currFile
		}

		// Black semi-open
		if currFile&(b.Black.Pawns) == 0 && currFile&(b.White.Pawns) > 0 {
			bSemiOpenFiles |= currFile
		}
	}
	return openFiles, wSemiOpenFiles, bSemiOpenFiles
}

/*
	Helper functions for king safety evaluation
*/

func getInnerKingSafetyTable(b *dragontoothmg.Board) [2]uint64 {
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

		kingZoneTable[i] = kingZoneBBInner
	}
	return kingZoneTable
}

func getOuterKingSafetyTable(kingInnerRing [2]uint64) [2]uint64 {
	var kingZoneTable [2]uint64

	for i, board := range kingInnerRing {
		kingZoneBBOuter := board
		kingZoneBBOuter = kingZoneBBOuter | (kingZoneBBOuter << 8) | (kingZoneBBOuter >> 8)
		kingZoneBBOuter = kingZoneBBOuter | (((kingZoneBBOuter & ^bitboardFileA) >> 1) | ((kingZoneBBOuter & ^bitboardFileH) << 1))
		kingZoneBBOuter = kingZoneBBOuter &^ kingZoneTable[i]
		kingZoneTable[i] = kingZoneBBOuter &^ board
	}
	return kingZoneTable
}

/*
	Helper functions to get bitboards for the purpose of pawn evaluation
*/

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

func pawnAttackSpan(wPawnAttackBB, bPawnAttackBB uint64) (wPawnAttackSpanBB, bPawnAttackSpanBB uint64) {
	for x := wPawnAttackBB; x != 0; x &= x - 1 {
		var sq int = bits.TrailingZeros64(x)
		var pawnRankBB uint64 = ranksAbove[(sq/8)-1]
		var pawnFileBB uint64 = onlyFile[sq%8]

		wPawnAttackSpanBB |= pawnRankBB & pawnFileBB
	}

	for x := bPawnAttackBB; x != 0; x &= x - 1 {
		var sq int = bits.TrailingZeros64(x)
		var pawnRankBB uint64 = ranksBelow[(sq/8)+1]
		var pawnFileBB uint64 = onlyFile[sq%8]

		bPawnAttackSpanBB |= pawnRankBB & pawnFileBB
	}

	return wPawnAttackSpanBB, bPawnAttackSpanBB
}

func getPawnBBs(b *dragontoothmg.Board, wPawnAttacksBB uint64, bPawnAttacksBB uint64) (wPhalanxPawnsBB uint64, bPhalanxPawnsBB uint64, wBlockedPawnsBB uint64, bBlockedPawnsBB uint64, wConnectedPawns uint64, bConnectedPawns uint64, wPassedPawnsBB uint64, bPassedPawnsBB uint64, wDoubledPawnsBB uint64, bDoubledPawnsBB uint64, wIsolatedPawnsBB uint64, bIsolatedPawnsBB uint64, wClosestPawn int, bClosestPawn int) {
	var wCheckedFiles uint64
	var bCheckedFiles uint64

	var wKingSquare = bits.TrailingZeros64(b.White.Kings)
	var bKingSquare = bits.TrailingZeros64(b.Black.Kings)

	var wKingFile = wKingSquare % 8
	var wKingRank = wKingSquare / 8
	var bKingFile = bKingSquare % 8
	var bKingRank = bKingSquare / 8

	wClosestPawn = 50
	bClosestPawn = 50

	for x := b.White.Pawns; x != 0; x &= x - 1 {
		var sq int = bits.TrailingZeros64(x)
		var pawnRank int = sq / 8
		var pawnFile int = sq % 8
		var pawnRankBB uint64 = onlyRank[sq/8]
		var pawnFileBB uint64 = onlyFile[sq%8]

		// Get distance to closest pawn from king
		var r2r1 = math.Abs(float64(pawnRank) - float64(wKingRank))
		var f2f1 = math.Abs(float64(pawnFile) - float64(wKingFile))
		wClosestPawn = Min(Max(int(r2r1), int(f2f1)), wClosestPawn)

		// PHALANX
		wPhalanxPawnsBB |= (((PositionBB[sq-1]) & b.White.Pawns &^ bitboardFileH) | ((PositionBB[sq+1]) & b.White.Pawns &^ bitboardFileA)) &^ secondRankMask

		// BLOCKED PAWNS
		var squareBB uint64 = PositionBB[sq]
		var abovePawnBB uint64 = squareBB << 8
		if b.Black.Pawns&abovePawnBB > 0 {
			wBlockedPawnsBB |= squareBB
		}

		// PASSED PAWNS
		var checkAbove = ranksAbove[(sq / 8)]
		if bits.OnesCount64(bPawnAttacksBB&(pawnFileBB&checkAbove)) == 0 && bits.OnesCount64(b.Black.Pawns&(pawnFileBB&checkAbove)) == 0 {
			wPassedPawnsBB |= (pawnFileBB & pawnRankBB)
		}

		// DOUBLED PAWNS
		if bits.OnesCount64((pawnFileBB&b.White.Pawns)&^wCheckedFiles) >= 2 {
			wDoubledPawnsBB |= pawnFileBB & b.White.Pawns
			wCheckedFiles |= pawnFileBB
		}

		// ISOLATED PAWNS
		var wPawnsOnFile uint64 = (b.White.Pawns & pawnFileBB) &^ wCheckedFiles
		if wPawnsOnFile > 0 {
			var neighbors uint64 = actualIsolatedPawnTable[bits.TrailingZeros64(wPawnsOnFile)%8]
			if b.White.Pawns&neighbors == 0 {
				wIsolatedPawnsBB |= wPawnsOnFile
				wCheckedFiles |= pawnFileBB
			}
		}
	}
	for x := b.Black.Pawns; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		var pawnRank int = sq / 8
		var pawnFile int = sq % 8
		var pawnRankBB uint64 = onlyRank[sq/8]
		var pawnFileBB uint64 = onlyFile[sq%8]

		// Get distance to closest pawn from king
		var r2r1 = math.Abs(float64(pawnRank) - float64(bKingRank))
		var f2f1 = math.Abs(float64(pawnFile) - float64(bKingFile))
		bClosestPawn = Min(Max(int(r2r1), int(f2f1)), bClosestPawn)

		// PHALANX
		bPhalanxPawnsBB |= (((PositionBB[sq-1]) & b.Black.Pawns &^ bitboardFileH) | ((PositionBB[sq+1]) & b.Black.Pawns &^ bitboardFileA)) &^ seventhRankMask

		// BLOCKED PAWNS
		squareBB := PositionBB[sq]
		belowPawnBB := squareBB >> 8
		if b.White.Pawns&belowPawnBB > 0 {
			bBlockedPawnsBB |= squareBB
		}

		// PASSED PAWNS
		var checkBelow = ranksBelow[(sq / 8)]

		if bits.OnesCount64(wPawnAttacksBB&(pawnFileBB&checkBelow)) == 0 && bits.OnesCount64(b.White.Pawns&(pawnFileBB&checkBelow)) == 0 {
			bPassedPawnsBB |= (pawnFileBB & pawnRankBB)
		}

		// DOUBLED PAWNS
		if bits.OnesCount64((pawnFileBB&b.Black.Pawns)&^bCheckedFiles) >= 2 {
			bDoubledPawnsBB |= (pawnFileBB & b.Black.Pawns)
			bCheckedFiles |= pawnFileBB
		}

		// ISOLATED PAWNS
		var bPawnsOnFile uint64 = (b.Black.Pawns & pawnFileBB) &^ bCheckedFiles
		if bPawnsOnFile > 0 {
			var neighbors uint64 = actualIsolatedPawnTable[bits.TrailingZeros64(bPawnsOnFile)%8]
			if b.Black.Pawns&neighbors == 0 {
				bIsolatedPawnsBB |= bPawnsOnFile
				bCheckedFiles |= pawnFileBB
			}
		}
	}

	if wClosestPawn == 50 {
		wClosestPawn = 0
	}
	if bClosestPawn == 50 {
		bClosestPawn = 0
	}

	// CONNECTED PAWNS
	wConnectedPawns |= (b.White.Pawns & wPawnAttacksBB)
	bConnectedPawns |= (b.Black.Pawns & bPawnAttacksBB)

	return wPhalanxPawnsBB, bPhalanxPawnsBB, wBlockedPawnsBB, bBlockedPawnsBB, wConnectedPawns, bConnectedPawns, wPassedPawnsBB, bPassedPawnsBB, wDoubledPawnsBB, bDoubledPawnsBB, wIsolatedPawnsBB, bIsolatedPawnsBB, wClosestPawn, bClosestPawn
}

func getQueenInfiltrationBB(wPawnAttackSpanBB uint64, bPawnAttackSpanBB uint64) (wInfiltrationBB uint64, bInfiltrationBB uint64) {
	wInfiltrationBB = bPawnAttackSpanBB & ranksAbove[3]
	bInfiltrationBB = wPawnAttackSpanBB & ranksBelow[3]
	return wInfiltrationBB, bInfiltrationBB
}

func isTheoreticalDraw(board *dragontoothmg.Board, debug bool) bool {
	pawnCount := bits.OnesCount64(board.White.Pawns | board.Black.Pawns)

	wKnights := bits.OnesCount64(board.White.Knights)
	wBishops := bits.OnesCount64(board.White.Bishops)
	wRooks := bits.OnesCount64(board.White.Rooks)
	wQueens := bits.OnesCount64(board.White.Queens)
	wMinorPieces := wKnights + wBishops

	bKnights := bits.OnesCount64(board.Black.Knights)
	bBishops := bits.OnesCount64(board.Black.Bishops)
	bRooks := bits.OnesCount64(board.Black.Rooks)
	bQueens := bits.OnesCount64(board.Black.Queens)
	bMinorPieces := bKnights + bBishops

	allPieces := bits.OnesCount64((board.White.All | board.Black.All) & ^(board.White.Kings | board.Black.Kings))
	if debug {
		println("All: ", allPieces, "\twQueen: ", wQueens, "\twRooks: ", wRooks, "\twKnights: ", wKnights, "\twBishops: ")
		println("All: ", allPieces, "\tbQueen: ", bQueens, "\tbRooks: ", bRooks, "\tbKnights: ", bKnights, "\tbBishops: ")
	}

	/*
		GENERAL DRAWS:
			ONLY KINGS:	??????????
			ONE PIECE:
				- One knight				✓
				- One bishop				✓
			TWO PIECES:
				- two knights (same side)	✓
				- one minor piece each		✓
				- rook vs minor piece		✓
				- rook v rook				✓
				- queen v queen				✓
			THREE PIECES:
				- rook+minor vs rook		✓
				- 2v1 only minor pieces		✓
				- Queen vs 2 knights		✓
	*/
	if pawnCount == 0 {
		if allPieces == 1 { // single piece draw
			if wMinorPieces == 1 || bMinorPieces == 1 {
				return true
			}
		} else if allPieces == 2 { // Draws with only two major/minor pieces (where it generally is a draw)
			if wKnights == 2 || bKnights == 2 {
				return true
			} else if wMinorPieces == 1 && bMinorPieces == 1 {
				return true
			} else if (wRooks == 1 && (bMinorPieces == 1 || bRooks == 1)) || (bRooks == 1 && (wMinorPieces == 1 || wRooks == 1)) {
				return true
			} else if wQueens == 1 && bQueens == 1 {
				return true
			}
		} else if allPieces == 3 {
			if ((wRooks == 1 && wMinorPieces == 1) && bRooks == 1) || (bRooks == 1 && bMinorPieces == 1) && wRooks == 1 {
				return true
			} else if (wMinorPieces == 2 && bMinorPieces == 1) || (bMinorPieces == 2 && wMinorPieces == 1) {
				return true
			} else if (wQueens == 1 && bKnights == 2) || (bQueens == 1 && wKnights == 2) {
				return true
			}
		}
	}

	return false
}
