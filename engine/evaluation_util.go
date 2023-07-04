package engine

import (
	"math/bits"

	"github.com/dylhunn/dragontoothmg"
)

func InBetween(i, min, max int) bool {
	if (i >= min) && (i <= max) {
		return true
	} else {
		return false
	}
}

var SquareBB [65]uint64

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

func getOutpostsBB(b *dragontoothmg.Board) (outpostSquares [2]uint64) {
	// Generate allowed ranks & files for outposts to be on
	wPotentialOutposts := wPawnAttackBB & wAllowedOutpostMask
	bPotentialOutposts := bPawnAttackBB & bAllowedOutpostMask

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
			var ranksToCheckForEnemyPawns = ranksBelow[(sq/8)+1]
			if bits.OnesCount64(b.White.Pawns&(filesToCheck&ranksToCheckForEnemyPawns)) == 0 {
				bOutpostBB = bOutpostBB | sqBB
			}
		}
	}

	outpostSquares[0] = wOutpostBB
	outpostSquares[1] = bOutpostBB
	return
}

func OutpostBonus(b *dragontoothmg.Board, pieceType dragontoothmg.Piece) (outpostMG, outpostEG int) {
	var onOutpostBonus = 0
	if pieceType == dragontoothmg.Bishop {
		// White
		outpostMG += bishopOutpost * bits.OnesCount64(b.White.Bishops&whiteOutposts)
		outpostEG += bishopOutpost * bits.OnesCount64(b.White.Bishops&whiteOutposts)

		// Black
		outpostMG -= bishopOutpost * bits.OnesCount64(b.Black.Bishops&blackOutposts)
		outpostEG -= bishopOutpost * bits.OnesCount64(b.Black.Bishops&blackOutposts)

	} else {
		outpostMG += knightOutpost * bits.OnesCount64(b.White.Knights&whiteOutposts)
		outpostEG += knightOutpost * bits.OnesCount64(b.White.Knights&whiteOutposts)

		outpostMG -= onOutpostBonus * bits.OnesCount64(b.Black.Knights&blackOutposts)
		outpostEG -= onOutpostBonus * bits.OnesCount64(b.Black.Knights&blackOutposts)
	}
	return outpostMG, outpostEG
}

func calculatePawnFileFill(pawnBitboard uint64) uint64 {
	pawnBitboard |= calculatePawnNorthFill(pawnBitboard)
	pawnBitboard |= calculatePawnSouthFill(pawnBitboard)
	return pawnBitboard
}

func calculatePawnNorthFill(pawnBitboard uint64) uint64 {
	pawnBitboard |= (pawnBitboard << 8)
	pawnBitboard |= (pawnBitboard << 16)
	pawnBitboard |= (pawnBitboard << 32)
	return pawnBitboard
}

func calculatePawnSouthFill(pawnBitboard uint64) uint64 {
	pawnBitboard |= (pawnBitboard >> 8)
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
			if (wKnights == 2 || bKnights == 2) || ((wBishops == 1 || wKnights == 1) && (bBishops == 1 || bKnights == 1)) {
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
