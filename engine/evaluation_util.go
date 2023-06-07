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

func getRankOfSquare(sq int) uint64 {
	return onlyRank[sq%8]
}

func getFileOfSquare(sq int) uint64 {
	return onlyFile[sq%8]
}

func getOutpostsBB(b *dragontoothmg.Board) (outpostSquares [2]uint64) {
	wEast, wWest := PawnCaptureBitboards(b.White.Pawns, true)
	bEast, bWest := PawnCaptureBitboards(b.Black.Pawns, false)
	wPawnCaptureBoard := wEast | wWest
	bPawnCaptureBoard := bEast | bWest

	// Generate allowed ranks & files for outposts to be on

	var wAllowedOutpostMask uint64 = 0xffff7e7e000000
	var bAllowedOutpostMask uint64 = 0x7e7effff00
	wPotentialOutposts := wPawnCaptureBoard & wAllowedOutpostMask
	bPotentialOutposts := bPawnCaptureBoard & bAllowedOutpostMask

	var wOutpostBB uint64
	var bOutpostBB uint64

	for x := wPotentialOutposts; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		sqBB := PositionBB[sq]
		// Match potential outpost positions with the square
		if bits.OnesCount64(sqBB&wPotentialOutposts) > 0 {
			leftFile := getFileOfSquare(sq-1) &^ bitboardFileH
			rightFile := getFileOfSquare(sq+1) &^ bitboardFileA
			if bits.OnesCount64(b.Black.Pawns&(leftFile)) == 0 {
				wOutpostBB = wOutpostBB | sqBB
			}
			if bits.OnesCount64(b.Black.Pawns&(rightFile)) == 0 {
				wOutpostBB = wOutpostBB | sqBB
			}
		}
	}

	for x := bPawnCaptureBoard; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		sqBB := PositionBB[sq]
		// Match potential outpost positions with the square
		if bits.OnesCount64(sqBB&bPotentialOutposts) > 0 {
			leftFile := getFileOfSquare(sq-1) &^ bitboardFileH
			rightFile := getFileOfSquare(sq+1) &^ bitboardFileA
			if bits.OnesCount64(b.White.Pawns&(leftFile)) == 0 {
				bOutpostBB = bOutpostBB | sqBB
			}
			if bits.OnesCount64(b.White.Pawns&(rightFile)) == 0 {
				bOutpostBB = bOutpostBB | sqBB
			}
		}
	}

	outpostSquares[0] = wOutpostBB
	outpostSquares[1] = bOutpostBB
	return
}

func OutpostBonus(b *dragontoothmg.Board, pieceType dragontoothmg.Piece) (outpostMG, outpostEG int) {
	var reachableOutpostBonus = 0
	var onOutpostBonus = 0
	var wPieceBB uint64
	var bPieceBB uint64
	if pieceType == dragontoothmg.Bishop {
		reachableOutpostBonus = bishopCanReachOutpost
		onOutpostBonus = bishopOutpost
		wPieceBB = b.White.Bishops
		bPieceBB = b.Black.Bishops
	} else {
		reachableOutpostBonus = knightOutpost
		onOutpostBonus = knightCanReachOutpost
		wPieceBB = b.White.Knights
		bPieceBB = b.Black.Knights
	}

	for x := wPieceBB; x != 0; x &= x - 1 {
		sqBB := PositionBB[bits.TrailingZeros64(x)]
		var movementBB uint64
		if pieceType == dragontoothmg.Bishop {
			movementBB = dragontoothmg.CalculateBishopMoveBitboard(uint8(bits.TrailingZeros64(x)), (b.White.All|b.Black.All)) & ^b.White.All
		} else {
			movementBB = KnightMasks[bits.TrailingZeros64(x)] & ^b.White.All
		}

		if bits.OnesCount64(sqBB&whiteOutposts) > 0 { // If we are on an outpost, give a bonus
			outpostMG += onOutpostBonus * bits.OnesCount64(sqBB&whiteOutposts)
			outpostEG += onOutpostBonus * bits.OnesCount64(sqBB&whiteOutposts)
		} else if bits.OnesCount64(movementBB&whiteOutposts) > 0 { // else if we can reach it, give a smaller bonus
			outpostMG += reachableOutpostBonus * bits.OnesCount64(sqBB&whiteOutposts)
			outpostEG += reachableOutpostBonus * bits.OnesCount64(movementBB&whiteOutposts)
		}
	}

	for x := bPieceBB; x != 0; x &= x - 1 {
		sqBB := PositionBB[bits.TrailingZeros64(x)]
		var movementBB uint64
		if pieceType == dragontoothmg.Bishop {
			movementBB = dragontoothmg.CalculateBishopMoveBitboard(uint8(bits.TrailingZeros64(x)), (b.White.All|b.Black.All)) &^ b.Black.All
		} else {
			movementBB = KnightMasks[bits.TrailingZeros64(x)] &^ b.Black.All
		}

		if bits.OnesCount64(sqBB&blackOutposts) > 0 { // If we are on an outpost, give a bonus
			outpostMG -= (onOutpostBonus * bits.OnesCount64(sqBB&blackOutposts))
			outpostEG -= (onOutpostBonus * bits.OnesCount64(sqBB&blackOutposts))
		} else if bits.OnesCount64(movementBB&blackOutposts) > 0 { // else if we can reach it, give a smaller bonus
			outpostMG -= (reachableOutpostBonus * bits.OnesCount64(movementBB&blackOutposts))
			outpostEG -= (reachableOutpostBonus * bits.OnesCount64(movementBB&blackOutposts))
		}
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
