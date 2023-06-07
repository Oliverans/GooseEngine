package engine

import (
	"math/bits"
	"time"

	"github.com/dylhunn/dragontoothmg"
)

var flipView = [64]int{
	56, 57, 58, 59, 60, 61, 62, 63,
	48, 49, 50, 51, 52, 53, 54, 55,
	40, 41, 42, 43, 44, 45, 46, 47,
	32, 33, 34, 35, 36, 37, 38, 39,
	24, 25, 26, 27, 28, 29, 30, 31,
	16, 17, 18, 19, 20, 21, 22, 23,
	8, 9, 10, 11, 12, 13, 14, 15,
	0, 1, 2, 3, 4, 5, 6, 7,
}

var DrawDivider = 12

var TotalEvalTime time.Duration

var PawnValueMG = 70
var PawnValueEG = 120
var KnightValueMG = 390
var KnightValueEG = 350
var BishopValueMG = 420
var BishopValueEG = 410
var RookValueMG = 540
var RookValueEG = 580
var QueenValueMG = 1020
var QueenValueEG = 950

var pieceList = [6]dragontoothmg.Piece{dragontoothmg.Pawn, dragontoothmg.Knight, dragontoothmg.Bishop, dragontoothmg.Rook, dragontoothmg.Queen, dragontoothmg.King}

var pieceValueMG = map[dragontoothmg.Piece]int{dragontoothmg.King: 0, dragontoothmg.Pawn: 82, dragontoothmg.Knight: 337, dragontoothmg.Bishop: 365, dragontoothmg.Rook: 477, dragontoothmg.Queen: 1025}
var pieceValueEG = map[dragontoothmg.Piece]int{dragontoothmg.King: 0, dragontoothmg.Pawn: 94, dragontoothmg.Knight: 281, dragontoothmg.Bishop: 297, dragontoothmg.Rook: 512, dragontoothmg.Queen: 936}

var mobilityValueMG = map[dragontoothmg.Piece]int{dragontoothmg.Pawn: 0, dragontoothmg.Knight: 2, dragontoothmg.Bishop: 2, dragontoothmg.Rook: 2, dragontoothmg.Queen: 2, dragontoothmg.King: 0}
var mobilityValueEG = map[dragontoothmg.Piece]int{dragontoothmg.Pawn: 0, dragontoothmg.Knight: 1, dragontoothmg.Bishop: 2, dragontoothmg.Rook: 5, dragontoothmg.Queen: 6, dragontoothmg.King: 0}

var attackerInner = map[dragontoothmg.Piece]int{dragontoothmg.Pawn: 0, dragontoothmg.Knight: 2, dragontoothmg.Bishop: 2, dragontoothmg.Rook: 4, dragontoothmg.Queen: 6, dragontoothmg.King: 0}
var attackerOuter = map[dragontoothmg.Piece]int{dragontoothmg.Pawn: 0, dragontoothmg.Knight: 0, dragontoothmg.Bishop: 0, dragontoothmg.Rook: 0, dragontoothmg.Queen: 0, dragontoothmg.King: 0}

var whiteOutposts uint64
var blackOutposts uint64

var seventhRankMask uint64 = 0xff000000000000
var secondRankMask uint64 = 0xff00
var wCentralSquaresMask uint64 = 0x3c3c3c00
var bCentralSquaresMask uint64 = 0x3c3c3c00000000

const (
	// Constants which map a piece to how much weight it should have on the phase of the game.
	PawnPhase   = 0
	KnightPhase = 1
	BishopPhase = 1
	RookPhase   = 2
	QueenPhase  = 4
	TotalPhase  = PawnPhase*16 + KnightPhase*4 + BishopPhase*4 + RookPhase*4 + QueenPhase*2
)

// Pawn variables
var DoubledPawnPenaltyMG = 3
var DoubledPawnPenaltyEG = 7
var IsolatedPawnMG = 7
var IsolatedPawnEG = 3
var PawnPushThreatMG = 10
var PawnPushThreatEG = 0
var ConnectedPawnsBonusMG = 15
var ConnectedPawnsBonusEG = 5
var PhalanxPawnsBonusMG = 5
var PhalanxPawnsBonusEG = 3
var BlockedPawn5thMG = 10
var BlockedPawn5thEG = 5
var BlockedPawn6thMG = 12
var BlockedPawn6thEG = 7

// Knight variables
var knightOutpost = 20
var knightCanReachOutpost = 0

// Bishop variables
var bishopPawnXrayPenaltyMG = 8
var bishopPawnXrayPenaltyEG = 2
var bishopOutpost = 15
var bishopPairBonusMG = 10
var bishopPairBonusEG = 40
var bishopCanReachOutpost = 0 // less usual for a bishop, but feels stronger when it happens!!!

// Rook variables
var RookXrayQueenOnOpenFileBonusMG = 4 // Small bonus better?
var ConnectedRooksBonusMG = 15
var SemiOpenBonusMG = 15
var OpenFileBonusMG = 25
var seventhRankBonusMG = 5
var seventhRankBonusEG = 20

// Queen variables

// King variables
var kingSemiOpenFileNextToPenalty = 15
var kingOpenFileNextToPenalty = 30
var kingOnOpenFilePenalty = 60

var kingSafetyTable = [100]int{
	0, 0, 1, 2, 3, 5, 7, 9, 12, 15,
	18, 22, 26, 30, 35, 39, 44, 50, 56, 62,
	68, 75, 82, 85, 89, 97, 105, 113, 122, 131,
	140, 150, 169, 180, 191, 202, 213, 225, 237, 248,
	260, 272, 283, 295, 307, 319, 330, 342, 354, 366,
	377, 389, 401, 412, 424, 436, 448, 459, 471, 483,
	494, 500, 500, 500, 500, 500, 500, 500, 500, 500,
	500, 500, 500, 500, 500, 500, 500, 500, 500, 500,
	500, 500, 500, 500, 500, 500, 500, 500, 500, 500,
	500, 500, 500, 500, 500, 500, 500, 500, 500, 500,
}

/* ============= HELPER VARIABLES ============= */
var passedPawnBonusMG = [8]int{0, 9, 4, 1, 13, 48, 109, 0}
var passedPawnBonusEG = [8]int{0, 1, 5, 25, 50, 103, 149, 0}

var centerManhattanDistance = [64]int{
	18, 12, 4, 3, 3, 4, 12, 18,
	12, 4, 3, 2, 2, 3, 4, 12,
	4, 3, 2, 1, 1, 2, 3, 4,
	3, 2, 1, 0, 0, 1, 2, 3,
	3, 2, 1, 0, 0, 1, 2, 3,
	4, 3, 2, 1, 1, 2, 3, 4,
	12, 4, 3, 2, 2, 3, 4, 12,
	18, 12, 4, 3, 3, 4, 12, 18,
}

var whiteKingPawnShelterBB = [2]uint64{0xe0e000, 0x70700}
var blackKingPawnShelterBB = [2]uint64{0xe0e00000000000, 0x7070000000000}

var PieceSquareTablesMidGame = map[dragontoothmg.Piece][64]int{
	dragontoothmg.Pawn: {
		0, 0, 0, 0, 0, 0, 0, 0,
		-35, -1, -20, -23, -15, 24, 38, -22,
		-26, -4, -4, -10, 3, 3, 33, -12,
		-27, -2, -5, 12, 17, 6, 10, -25,
		-14, 13, 6, 21, 23, 12, 17, -23,
		-6, 7, 26, 31, 65, 56, 25, -20,
		98, 134, 61, 95, 68, 126, 34, -11,
		0, 0, 0, 0, 0, 0, 0, 0,
	},
	dragontoothmg.Knight: {
		-105, -21, -58, -33, -17, -28, -19, -23,
		-29, -53, -12, -3, -1, 18, -14, -19,
		-23, -9, 12, 10, 19, 17, 25, -16,
		-13, 4, 16, 13, 28, 19, 21, -8,
		-9, 17, 19, 53, 37, 69, 18, 22,
		-47, 60, 37, 65, 84, 129, 73, 44,
		-73, -41, 72, 36, 23, 62, 7, -17,
		-167, -89, -34, -49, 61, -97, -15, -107,
	},
	dragontoothmg.Bishop: {
		-33, -3, -14, -21, -13, -12, -39, -21,
		4, 15, 16, 0, 7, 21, 33, 1,
		0, 15, 15, 15, 14, 27, 18, 10,
		-6, 13, 13, 26, 34, 12, 10, 4,
		-4, 5, 19, 50, 37, 37, 7, -2,
		-16, 37, 43, 40, 35, 50, 37, -2,
		-26, 16, -18, -13, 30, 59, 18, -47,
		-29, 4, -82, -37, -25, -42, 7, -8,
	},
	dragontoothmg.Rook: {
		-19, -13, 1, 17, 16, 7, -37, -26,
		-44, -16, -20, -9, -1, 11, -6, -71,
		-45, -25, -16, -17, 3, 0, -5, -33,
		-36, -26, -12, -1, 9, -7, 6, -23,
		-24, -11, 7, 26, 24, 35, -8, -20,
		-5, 19, 26, 36, 17, 45, 61, 16,
		27, 32, 58, 62, 80, 67, 26, 44,
		32, 42, 32, 51, 63, 9, 31, 43,
	},
	dragontoothmg.Queen: {
		-1, -18, -9, 10, -15, -25, -31, -50,
		-35, -8, 11, 2, 8, 15, -3, 1,
		-14, 2, -11, -2, -5, 2, 14, 5,
		-9, -26, -9, -10, -2, -4, 3, -3,
		-27, -27, -16, -16, -1, 17, -2, 1,
		-13, -17, 7, 8, 29, 56, 47, 57,
		-24, -39, -5, 1, -16, 57, 28, 54,
		-28, 0, 29, 12, 59, 44, 43, 45,
	},
	dragontoothmg.King: {
		-15, 36, 12, -54, 8, -28, 24, 14,
		1, 7, -8, -64, -43, -16, 9, 8,
		-14, -14, -22, -46, -44, -30, -15, -27,
		-49, -1, -27, -39, -46, -44, -33, -51,
		-17, -20, -12, -27, -30, -25, -14, -36,
		-9, 24, 2, -16, -20, 6, 22, -22,
		29, -1, -20, -7, -8, -4, -38, -29,
		-65, 23, 16, -15, -56, -34, 2, 13,
	},
}

var PieceSquareTablesEndgame = map[dragontoothmg.Piece][64]int{
	dragontoothmg.Pawn: {
		0, 0, 0, 0, 0, 0, 0, 0,
		13, 8, 8, 10, 13, 0, 2, -7,
		4, 7, -6, 1, 0, -5, -1, -8,
		13, 9, -3, -7, -7, -8, 3, -1,
		32, 24, 13, 5, -2, 4, 17, 17,
		94, 100, 85, 67, 56, 53, 82, 84,
		178, 173, 158, 134, 147, 132, 165, 187,
		0, 0, 0, 0, 0, 0, 0, 0,
	},
	dragontoothmg.Knight: {
		-29, -51, -23, -15, -22, -18, -50, -64,
		-42, -20, -10, -5, -2, -20, -23, -44,
		-23, -3, -1, 15, 10, -3, -20, -22,
		-18, -6, 16, 25, 16, 17, 4, -18,
		-17, 3, 22, 22, 22, 11, 8, -18,
		-24, -20, 10, 9, -1, -9, -19, -41,
		-25, -8, -25, -2, -9, -25, -24, -52,
		-58, -38, -13, -28, -31, -27, -63, -99,
	},
	dragontoothmg.Bishop: {
		-23, -9, -23, -5, -9, -16, -5, -17,
		-14, -18, -7, -1, 4, -9, -15, -27,
		-12, -3, 8, 10, 13, 3, -7, -15,
		-6, 3, 13, 19, 7, 10, -3, -9,
		-3, 9, 12, 9, 14, 10, 3, 2,
		2, -8, 0, -1, -2, 6, 0, 4,
		-8, -4, 7, -12, -3, -13, -4, -14,
		-14, -21, -11, -8, -7, -9, -17, -24,
	},
	dragontoothmg.Rook: {
		-9, 2, 3, -1, -5, -13, 4, -20,
		-6, -6, 0, 2, -9, -9, -11, -3,
		-4, 0, -5, -1, -7, -12, -8, -16,
		3, 5, 8, 4, -5, -6, -8, -11,
		4, 3, 13, 1, 2, 1, -1, 2,
		7, 7, 7, 5, 4, -3, -5, -3,
		11, 13, 13, 11, -3, 3, 8, 3,
		13, 10, 18, 15, 12, 12, 8, 5,
	},
	dragontoothmg.Queen: {
		-33, -28, -22, -43, -5, -32, -20, -41,
		-22, -23, -30, -16, -16, -23, -36, -32,
		-16, -27, 15, 6, 9, 17, 10, 5,
		-18, 28, 19, 47, 31, 34, 39, 23,
		3, 22, 24, 45, 57, 40, 57, 36,
		-20, 6, 9, 49, 47, 35, 19, 9,
		-17, 20, 32, 41, 58, 25, 30, 0,
		-9, 22, 22, 27, 27, 19, 10, 20,
	},
	dragontoothmg.King: {
		-53, -34, -21, -11, -28, -14, -24, -43,
		-27, -11, 4, 13, 14, 4, -5, -17,
		-19, -3, 11, 21, 23, 16, 7, -9,
		-18, -4, 21, 24, 27, 23, 9, -11,
		-8, 22, 24, 27, 26, 33, 26, 3,
		10, 17, 23, 15, 20, 45, 44, 13,
		-12, 17, 14, 17, 17, 38, 23, 11,
		-74, -35, -18, -18, -11, 15, 4, -17,
	},
}

var whitePassedPawnTable = [64]uint64{
	0x303030303030300, 0x707070707070700, 0xe0e0e0e0e0e0e00, 0x1c1c1c1c1c1c1c00, 0x3838383838383800, 0x7070707070707000, 0xe0e0e0e0e0e0e000, 0xc0c0c0c0c0c0c000,
	0x303030303030000, 0x707070707070000, 0xe0e0e0e0e0e0000, 0x1c1c1c1c1c1c0000, 0x3838383838380000, 0x7070707070700000, 0xe0e0e0e0e0e00000, 0xc0c0c0c0c0c00000,
	0x303030303000000, 0x707070707000000, 0xe0e0e0e0e000000, 0x1c1c1c1c1c000000, 0x3838383838000000, 0x7070707070000000, 0xe0e0e0e0e0000000, 0xc0c0c0c0c0000000,
	0x303030300000000, 0x707070700000000, 0xe0e0e0e00000000, 0x1c1c1c1c00000000, 0x3838383800000000, 0x7070707000000000, 0xe0e0e0e000000000, 0xc0c0c0c000000000,
	0x303030000000000, 0x707070000000000, 0xe0e0e0000000000, 0x1c1c1c0000000000, 0x3838380000000000, 0x7070700000000000, 0xe0e0e00000000000, 0xc0c0c00000000000,
	0x303000000000000, 0x707000000000000, 0xe0e000000000000, 0x1c1c000000000000, 0x3838000000000000, 0x7070000000000000, 0xe0e0000000000000, 0xc0c0000000000000,
	0x300000000000000, 0x700000000000000, 0xe00000000000000, 0x1c00000000000000, 0x3800000000000000, 0x7000000000000000, 0xe000000000000000, 0xc000000000000000,
	0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
}

var blackPassedPawnTable = [64]uint64{
	0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
	0x3, 0x7, 0xe, 0x1c, 0x38, 0x70, 0xe0, 0xc0,
	0x303, 0x707, 0xe0e, 0x1c1c, 0x3838, 0x7070, 0xe0e0, 0xc0c0,
	0x30303, 0x70707, 0xe0e0e, 0x1c1c1c, 0x383838, 0x707070, 0xe0e0e0, 0xc0c0c0,
	0x3030303, 0x7070707, 0xe0e0e0e, 0x1c1c1c1c, 0x38383838, 0x70707070, 0xe0e0e0e0, 0xc0c0c0c0,
	0x303030303, 0x707070707, 0xe0e0e0e0e, 0x1c1c1c1c1c, 0x3838383838, 0x7070707070, 0xe0e0e0e0e0, 0xc0c0c0c0c0,
	0x30303030303, 0x70707070707, 0xe0e0e0e0e0e, 0x1c1c1c1c1c1c, 0x383838383838, 0x707070707070, 0xe0e0e0e0e0e0, 0xc0c0c0c0c0c0,
	0x3030303030303, 0x7070707070707, 0xe0e0e0e0e0e0e, 0x1c1c1c1c1c1c1c, 0x38383838383838, 0x70707070707070, 0xe0e0e0e0e0e0e0, 0xc0c0c0c0c0c0c0,
}

// Taken from dragontooth chess engine!
var isolatedPawnTable = [8]uint64{
	0x303030303030303, 0x707070707070707, 0xe0e0e0e0e0e0e0e, 0x1c1c1c1c1c1c1c1c,
	0x3838383838383838, 0x7070707070707070, 0xe0e0e0e0e0e0e0e0, 0xc0c0c0c0c0c0c0c0,
}

var onlyFile = [8]uint64{
	0x0101010101010101, 0x0202020202020202, 0x0404040404040404, 0x0808080808080808,
	0x1010101010101010, 0x2020202020202020, 0x4040404040404040, 0x8080808080808080,
}

var onlyRank = [8]uint64{
	0xFF, 0xFF00, 0xFF0000, 0xFF000000,
	0xFF00000000, 0xFF0000000000, 0xFF000000000000, 0xFF00000000000000,
}

/* ============= HELPER FUNCTIONS ============= */

func max(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

/* ============= EVALUATION FUNCTIONS ============= */

func spaceArea(b *dragontoothmg.Board) (spaceBonus int) {

	wEast, wWest := PawnCaptureBitboards(b.White.Pawns, true)
	wPawnAttackBB := wEast | wWest
	wPawnBB := b.White.Pawns
	var wBehindPawnArea uint64

	bEast, bWest := PawnCaptureBitboards(b.Black.Pawns, false)
	bPawnAttackBB := bEast | bWest
	bPawnBB := b.Black.Pawns
	var bBehindPawnArea uint64

	for x := wPawnBB; x != 0; x &= x - 1 {
		square := bits.TrailingZeros64(x)
		position := PositionBB[square]
		tmpPosition := position
		rankSquare := square / 8
		if rankSquare > 1 && rankSquare < 4 { // Pawn not on a starting square
			numberOfSquaresBehind := rankSquare - 1
			for i := numberOfSquaresBehind; i >= 1; i-- {
				tmpPos := tmpPosition
				tmpPos = tmpPos >> (8 * i)
				position = position | tmpPos
			}
			wBehindPawnArea = wBehindPawnArea | (position &^ tmpPosition)
		}
	}

	for x := bPawnBB; x != 0; x &= x - 1 {
		square := bits.TrailingZeros64(x)
		position := PositionBB[square]
		tmpPosition := position
		rankSquare := square / 8
		if rankSquare < 6 && rankSquare > 3 { // Pawn not on a starting square
			numberOfSquaresBehind := rankSquare + 1
			for i := numberOfSquaresBehind; i < 7; i++ {
				tmpPos := tmpPosition
				tmpPos = tmpPos << (8 * (7 - i))
				position = position | tmpPos
			}
			bBehindPawnArea = bBehindPawnArea | (position &^ tmpPosition)
		}
	}

	wBehindPawnArea = (wBehindPawnArea & wCentralSquaresMask) &^ (bPawnAttackBB | wPawnBB)
	bBehindPawnArea = (bBehindPawnArea & bCentralSquaresMask) &^ (wPawnAttackBB | bPawnBB)

	wSpace := wCentralSquaresMask &^ (wBehindPawnArea | wPawnBB | bPawnAttackBB)
	bSpace := bCentralSquaresMask &^ (bBehindPawnArea | bPawnBB | wPawnAttackBB)

	wPieceCount := bits.OnesCount64(b.White.All)
	bPieceCount := bits.OnesCount64(b.Black.All)

	wScore := (wPieceCount) * (bits.OnesCount64(wSpace) + (bits.OnesCount64(wBehindPawnArea) * 2))
	bScore := (bPieceCount) * (bits.OnesCount64(bSpace) + (bits.OnesCount64(bBehindPawnArea) * 2))

	//var weight = pieceCount - 3 + Math.min(blockedCount, 9);
	//return ((space_area(pos, square) * weight * weight / 16) << 0);

	return wScore - bScore
}

func pieceMobilityBonus(b *dragontoothmg.Board, pieceType dragontoothmg.Piece, attackUnitsCount *[2]int, innerKingSafetyZones [2]uint64, outerKingSafetyZones [2]uint64) (mobilityMG, mobilityEG int) {
	var wPieceBB uint64
	var bPieceBB uint64

	inCheck := b.OurKingInCheck()

	if pieceType == dragontoothmg.Knight {
		wPieceBB = b.White.Knights
		bPieceBB = b.Black.Knights
	} else if pieceType == dragontoothmg.Bishop {
		wPieceBB = b.White.Bishops
		bPieceBB = b.Black.Bishops
	} else if pieceType == dragontoothmg.Rook {
		wPieceBB = b.White.Rooks
		bPieceBB = b.Black.Rooks
	} else if pieceType == dragontoothmg.Queen {
		wPieceBB = b.White.Queens
		bPieceBB = b.Black.Queens
	} else {
		wPieceBB = b.White.Pawns
		bPieceBB = b.Black.Pawns
	}

	var wPawnCaptureBoard uint64 = 0
	var bPawnCaptureBoard uint64 = 0

	for x := wPieceBB; x != 0; x &= x - 1 {
		square := bits.TrailingZeros64(x)
		var movementBoard uint64
		var movesCount = 0
		if pieceType == dragontoothmg.Knight {
			movementBoard = KnightMasks[square] &^ b.White.All
			movesCount = bits.OnesCount64(movementBoard&^bPawnCaptureBoard) - 4
		} else if pieceType == dragontoothmg.Bishop {
			movementBoard = dragontoothmg.CalculateBishopMoveBitboard(uint8(square), (b.White.All|b.Black.All)) & ^b.White.All
			movesCount = bits.OnesCount64(movementBoard&^bPawnCaptureBoard) - 7
		} else if pieceType == dragontoothmg.Rook {
			movementBoard = dragontoothmg.CalculateRookMoveBitboard(uint8(square), (b.White.All|b.Black.All)) & ^b.White.All
			movesCount = bits.OnesCount64(movementBoard&^bPawnCaptureBoard) - 7
			if !b.Wtomove && inCheck {
				attackUnitsCount[0] += 4
			}
		} else if pieceType == dragontoothmg.Queen {
			movementBoard = dragontoothmg.CalculateRookMoveBitboard(uint8(square), (b.White.All|b.Black.All)) & ^b.White.All
			movementBoard = movementBoard | ((dragontoothmg.CalculateRookMoveBitboard(uint8(square), (b.White.All | b.Black.All))) & ^b.White.All)
			movesCount = bits.OnesCount64(movementBoard&^bPawnCaptureBoard) - 14
			if !b.Wtomove && inCheck {
				attackUnitsCount[0] += 8
			}
		}

		attackUnitsCount[0] += (bits.OnesCount64(movementBoard&innerKingSafetyZones[1]) * attackerInner[pieceType])
		attackUnitsCount[0] += (bits.OnesCount64(movementBoard&outerKingSafetyZones[1]) * attackerOuter[pieceType])

		mobilityMG += movesCount * mobilityValueMG[pieceType]
		mobilityEG += movesCount * mobilityValueEG[pieceType]
	}

	for x := bPieceBB; x != 0; x &= x - 1 {
		square := bits.TrailingZeros64(x)
		var movementBoard uint64
		var movesCount = 0
		if pieceType == dragontoothmg.Knight {
			movementBoard = KnightMasks[square] &^ b.Black.All
			movesCount = bits.OnesCount64(movementBoard&^wPawnCaptureBoard) - 4
		} else if pieceType == dragontoothmg.Bishop {
			movementBoard = dragontoothmg.CalculateBishopMoveBitboard(uint8(square), (b.White.All|b.Black.All)) & ^b.Black.All
			movesCount = bits.OnesCount64(movementBoard&^wPawnCaptureBoard) - 7
		} else if pieceType == dragontoothmg.Rook {
			movementBoard = dragontoothmg.CalculateRookMoveBitboard(uint8(square), (b.White.All|b.Black.All)) & ^b.Black.All
			movesCount = bits.OnesCount64(movementBoard&^wPawnCaptureBoard) - 7
			if b.Wtomove && inCheck {
				attackUnitsCount[1] += 4
			}
		} else if pieceType == dragontoothmg.Queen {
			movementBoard = dragontoothmg.CalculateBishopMoveBitboard(uint8(square), (b.White.All|b.Black.All)) & ^b.Black.All
			movementBoard = movementBoard | (dragontoothmg.CalculateRookMoveBitboard(uint8(square), (b.White.All|b.Black.All)) & ^b.Black.All)
			movesCount = bits.OnesCount64(movementBoard&^wPawnCaptureBoard) - 14
			if b.Wtomove && inCheck {
				attackUnitsCount[1] += 8
			}
		}
		attackUnitsCount[1] += (bits.OnesCount64(movementBoard&innerKingSafetyZones[0]) * attackerInner[pieceType])
		attackUnitsCount[1] += (bits.OnesCount64(movementBoard&outerKingSafetyZones[0]) * attackerOuter[pieceType])

		mobilityMG -= movesCount * mobilityValueMG[pieceType]
		mobilityEG -= movesCount * mobilityValueEG[pieceType]
	}

	return mobilityMG, mobilityEG
}

func getKingSafetyTable(b *dragontoothmg.Board, inner bool) [2]uint64 {
	var kingZoneTable [2]uint64
	kingBoards := [2]uint64{
		0: b.White.Kings,
		1: b.Black.Kings,
	}

	for i, board := range kingBoards {
		kingZoneBB := board
		sq := bits.TrailingZeros64(kingZoneBB)
		kingZoneBB = kingZoneBB | (kingZoneBB << 8) | (kingZoneBB >> 8)
		if sq == 0 {
			kingZoneBB = kingZoneBB | (kingZoneBB << 8)
			kingZoneBB = kingZoneBB | (kingZoneBB << 1)
		} else if sq == 7 {
			kingZoneBB = kingZoneBB | (kingZoneBB << 8)
			kingZoneBB = kingZoneBB | (kingZoneBB >> 1)
		} else if sq >= 1 && sq <= 6 {
			kingZoneBB = kingZoneBB | (kingZoneBB << 8) | (kingZoneBB >> 8)
		} else if sq == 63 {
			kingZoneBB = kingZoneBB | (kingZoneBB >> 8)
			kingZoneBB = kingZoneBB | (kingZoneBB >> 1)
		} else if sq == 56 {
			kingZoneBB = kingZoneBB | (kingZoneBB >> 8)
			kingZoneBB = kingZoneBB | (kingZoneBB << 1)
		} else if sq >= 57 && sq <= 62 {
			kingZoneBB = kingZoneBB | (kingZoneBB << 8) | (kingZoneBB >> 8)
		}
		kingZoneBB = kingZoneBB | ((kingZoneBB & ^bitboardFileA) >> 1) | ((kingZoneBB & ^bitboardFileH) << 1)
		kingZoneTable[i] = kingZoneBB
	}
	if inner {
		for i, kingZoneBB := range kingZoneTable {
			if i == 0 {
				tmpKingZone := kingZoneBB
				kingZoneBB = kingZoneBB | (((kingZoneBB & ^bitboardFileA) >> 1) | ((kingZoneBB & ^bitboardFileH) << 1))
				kingZoneBB = kingZoneBB | ((kingZoneBB >> 8) | (kingZoneBB << 8))
				kingZoneTable[i] = kingZoneBB &^ tmpKingZone
			} else if i == 1 {
				tmpKingZone := kingZoneBB
				kingZoneBB = kingZoneBB | (((kingZoneBB & ^bitboardFileA) >> 1) | ((kingZoneBB & ^bitboardFileH) << 1))
				kingZoneBB = kingZoneBB | ((kingZoneBB >> 8) | (kingZoneBB << 8))
				kingZoneTable[i] = kingZoneBB &^ tmpKingZone
			}
		}
	}
	return kingZoneTable
}

func getkingEndGamePositionValue(b *dragontoothmg.Board, whiteWithAdvantage bool) int {

	var friendlyKingFile = 0
	var friendlyKingRank = 0
	var enemyKingFile = 0
	wKingSq := bits.TrailingZeros64(b.White.Kings)
	bKingSq := bits.TrailingZeros64(b.Black.Kings)
	var enemyKingRank = 0
	if whiteWithAdvantage { // White
		friendlyKingFile = wKingSq % 8
		friendlyKingRank = wKingSq / 8
		enemyKingFile = bKingSq % 8
		enemyKingRank = bKingSq / 8
	} else { // Black
		friendlyKingFile = wKingSq % 8
		friendlyKingRank = bKingSq / 8
		enemyKingFile = wKingSq % 8
		enemyKingRank = wKingSq / 8
	}
	score := 0

	// Add score based on the distance betwwn the kings
	// The longer the distance, the worse it is for us
	dstBetweenKingsFile := friendlyKingFile - enemyKingFile
	dstBetweenKingsRank := friendlyKingRank - enemyKingRank
	dstBetweenKings := dstBetweenKingsFile + dstBetweenKingsRank
	score -= 5 * dstBetweenKings

	return score
}

func getPiecePhase(b *dragontoothmg.Board) (phase int) {
	phase += bits.OnesCount64(b.White.Pawns|b.Black.Pawns) * PawnPhase
	phase += bits.OnesCount64(b.White.Knights|b.Black.Knights) * KnightPhase
	phase += bits.OnesCount64(b.White.Bishops|b.Black.Bishops) * BishopPhase
	phase += bits.OnesCount64(b.White.Rooks|b.Black.Rooks) * RookPhase
	phase += bits.OnesCount64(b.White.Queens|b.Black.Queens) * QueenPhase
	return phase
}

func CountMaterial(bb *dragontoothmg.Bitboards) (materialMG, materialEG int) {
	materialMG += bits.OnesCount64(bb.Pawns) * pieceValueMG[dragontoothmg.Pawn]
	materialEG += bits.OnesCount64(bb.Pawns) * pieceValueEG[dragontoothmg.Pawn]

	materialMG += bits.OnesCount64(bb.Knights) * pieceValueMG[dragontoothmg.Knight]
	materialEG += bits.OnesCount64(bb.Knights) * pieceValueEG[dragontoothmg.Knight]

	materialMG += bits.OnesCount64(bb.Bishops) * pieceValueMG[dragontoothmg.Bishop]
	materialEG += bits.OnesCount64(bb.Bishops) * pieceValueEG[dragontoothmg.Bishop]

	materialMG += bits.OnesCount64(bb.Rooks) * pieceValueMG[dragontoothmg.Rook]
	materialEG += bits.OnesCount64(bb.Rooks) * pieceValueEG[dragontoothmg.Rook]

	materialMG += bits.OnesCount64(bb.Queens) * pieceValueMG[dragontoothmg.Queen]
	materialEG += bits.OnesCount64(bb.Queens) * pieceValueEG[dragontoothmg.Queen]

	return materialMG, materialEG
}

func countPieceTables(b *dragontoothmg.Board, pieceType dragontoothmg.Piece, ptm [64]int, pte [64]int) (mgScore int, egScore int) {
	var wbb uint64
	var bbb uint64
	switch pieceType {
	case dragontoothmg.Pawn:
		wbb = b.White.Pawns
		bbb = b.Black.Pawns
	case dragontoothmg.Knight:
		wbb = b.White.Knights
		bbb = b.Black.Knights
	case dragontoothmg.Bishop:
		wbb = b.White.Bishops
		bbb = b.Black.Bishops
	case dragontoothmg.Rook:
		wbb = b.White.Rooks
		bbb = b.Black.Rooks
	case dragontoothmg.Queen:
		wbb = b.White.Queens
		bbb = b.Black.Queens
	case dragontoothmg.King:
		wbb = b.White.Kings
		bbb = b.Black.Kings
	}
	for wbb != 0 {
		var idx = bits.TrailingZeros64(wbb)
		wbb &= wbb - 1
		mgScore += ptm[idx]
		egScore += pte[idx]
		//println("Piece type: ", pieceType, " @score: ", score, " on square: ", idx)
	}
	for bbb != 0 {
		var idx = bits.TrailingZeros64(bbb)
		revView := flipView[idx]
		bbb &= bbb - 1
		mgScore -= ptm[revView]
		egScore -= pte[revView]
	}
	return mgScore, egScore
}

func blockedPawnPenalty(b *dragontoothmg.Board) (blockedMG, blockedEG int) {
	var bbs = [2]uint64{b.White.Pawns, b.Black.Pawns}

	for color, bb := range bbs {
		for x := bb; x != 0; x &= x - 1 {
			sq := bits.TrailingZeros64(x)
			if color == 0 {
				pawnBB := PositionBB[sq] << 8
				if bits.OnesCount64(pawnBB&(b.Black.Pawns&onlyRank[5])) > 0 {
					blockedMG += BlockedPawn5thMG
					blockedEG += BlockedPawn5thEG
				} else if bits.OnesCount64(pawnBB&b.Black.Pawns&onlyRank[6]) > 0 {
					blockedMG += BlockedPawn6thMG
					blockedEG += BlockedPawn6thEG
				}
			} else {
				pawnBB := PositionBB[sq] >> 8
				if bits.OnesCount64(pawnBB&b.White.Pawns&onlyRank[2]) > 0 {
					blockedMG -= BlockedPawn5thMG
					blockedEG -= BlockedPawn5thEG
				} else if bits.OnesCount64(pawnBB&b.White.Pawns&onlyRank[1]) > 0 {
					blockedMG -= BlockedPawn6thMG
					blockedEG -= BlockedPawn6thEG
				}
			}
		}
	}

	return blockedMG, blockedEG
}

func connectedOrPhalanxPawnBonus(b *dragontoothmg.Board) (connectedMG, connectedEG, phalanxMG, phalanxEG int) {
	// The idea of phalanx is from Stockfish Evaluation Guide
	// I've however not optimized this value what-so-ever, just spitballed the rough value :)
	var wConnectedPawns = 0
	var bConnectedPawns = 0

	var wPhalanxBB uint64
	var bPhalanxBB uint64

	wEast, wWest := PawnCaptureBitboards(b.White.Pawns, true)
	wPawnAttackBB := wEast | wWest
	wPawnBB := b.White.Pawns

	bEast, bWest := PawnCaptureBitboards(b.Black.Pawns, true)
	bpawnAttackBB := bEast | bWest
	bPawnBB := b.Black.Pawns

	for x := wPawnBB; x != 0; x &= x - 1 {
		pawnBB := PositionBB[bits.TrailingZeros64(x)]
		wConnectedPawns += bits.OnesCount64(wPawnAttackBB & pawnBB)
		wPhalanxBB = wPhalanxBB | (((PositionBB[bits.TrailingZeros64(x)-1]) & b.White.Pawns &^ bitboardFileH) | ((PositionBB[bits.TrailingZeros64(x)+1]) & b.White.Pawns &^ bitboardFileA))
	}
	for x := bPawnBB; x != 0; x &= x - 1 {
		pawnBB := PositionBB[bits.TrailingZeros64(x)]
		bConnectedPawns += bits.OnesCount64(bpawnAttackBB & pawnBB)
		bPhalanxBB = bPhalanxBB | (((PositionBB[bits.TrailingZeros64(x)-1]) & b.Black.Pawns &^ bitboardFileH) | ((PositionBB[bits.TrailingZeros64(x)+1]) & b.Black.Pawns &^ bitboardFileA))
	}
	connectedMG += (wConnectedPawns * ConnectedPawnsBonusMG) - (bConnectedPawns * ConnectedPawnsBonusMG)
	connectedEG += (wConnectedPawns * ConnectedPawnsBonusEG) - (bConnectedPawns * ConnectedPawnsBonusEG)

	phalanxMG += (bits.OnesCount64(wPhalanxBB) * PhalanxPawnsBonusMG) - (bits.OnesCount64(bPhalanxBB) * PhalanxPawnsBonusMG)
	phalanxEG += (bits.OnesCount64(wPhalanxBB) * PhalanxPawnsBonusEG) - (bits.OnesCount64(bPhalanxBB) * PhalanxPawnsBonusEG)

	return connectedMG, connectedEG, phalanxMG, phalanxEG
}

func pawnDoublingPenalties(b *dragontoothmg.Board) (doubledMG, doubledEG int) {
	var wDoubledPawnCount int
	var bDoubledPawnCount int
	for i := 0; i < 8; i++ {
		currFile := onlyFile[i]
		wDoubledPawnCount += max(bits.OnesCount64(b.White.Pawns&currFile)-1, 0)
		bDoubledPawnCount += max(bits.OnesCount64(b.Black.Pawns&currFile)-1, 0)
	}

	doubledMG += (wDoubledPawnCount * DoubledPawnPenaltyMG) - (bDoubledPawnCount * DoubledPawnPenaltyMG)
	doubledEG += (wDoubledPawnCount * DoubledPawnPenaltyEG) - (bDoubledPawnCount * DoubledPawnPenaltyEG)
	return doubledMG, doubledEG
}

func isolatedPawnBonus(b *dragontoothmg.Board) (isolatedMG, isolatedEG int) {
	var bbs = [2]uint64{0: b.White.Pawns, 1: b.Black.Pawns}

	// Taken (and slightly modified ..) from dragontooth chess bot
	for color, bb := range bbs {
		for x := bb; x != 0; x &= x - 1 {
			idx := bits.TrailingZeros64(bb)
			file := idx % 8
			if color == 0 {
				neighbors := bits.OnesCount64(isolatedPawnTable[file]&b.White.Pawns) - 1
				if neighbors == 0 {
					isolatedMG -= IsolatedPawnMG
					isolatedEG -= IsolatedPawnEG
				}
			} else {
				neighbors := bits.OnesCount64(isolatedPawnTable[file]&b.Black.Pawns) - 1
				if neighbors == 0 {
					isolatedMG += IsolatedPawnMG
					isolatedEG += IsolatedPawnEG
				}
			}
		}
	}
	return isolatedMG, isolatedEG
}

func passedPawnBonus(b *dragontoothmg.Board) (passedMG, passedEG int) {
	var bbs = [2]uint64{0: b.White.Pawns, 1: b.Black.Pawns}

	wEast, wWest := PawnCaptureBitboards(b.White.Pawns, true)
	wPawnAttackBB := wEast | wWest

	bEast, bWest := PawnCaptureBitboards(b.Black.Pawns, false)
	bPawnAttackBB := bEast | bWest

	for color, bb := range bbs {
		for x := bb; x != 0; x &= x - 1 {
			sq := bits.TrailingZeros64(x)
			if color == 0 {
				rank := sq / 8
				pawnFile := onlyFile[sq%8]
				var passedRankBB uint64
				for x := rank; x < 8; x++ {
					passedRankBB = passedRankBB | onlyRank[x]
				}
				if bits.OnesCount64(bPawnAttackBB&pawnFile&passedRankBB) == 0 && bits.OnesCount64(b.Black.Pawns&pawnFile&passedRankBB) == 0 {
					passedMG += passedPawnBonusMG[rank]
					passedEG += passedPawnBonusEG[rank]
				}
			} else {
				rank := sq / 8
				pawnFile := onlyFile[sq%8]
				var passedRankBB uint64
				for x := rank; x > 0; x-- {
					passedRankBB = passedRankBB | onlyRank[x]
				}
				if bits.OnesCount64(wPawnAttackBB&pawnFile&passedRankBB) == 0 && bits.OnesCount64(b.White.Pawns&pawnFile) == 0 {
					revSQ := flipView[sq]
					rank = revSQ / 8
					passedMG -= passedPawnBonusMG[rank]
					passedEG -= passedPawnBonusEG[rank]
				}
			}
		}
	}

	return passedMG, passedEG
}

func rookOpenFileBonus(b *dragontoothmg.Board) (mgScore int) { // We only return a MG-score, assuming it'll cause issues
	var bbs = [2]uint64{0: b.White.Rooks, 1: b.Black.Rooks}

	// So we don't stack rooks twice; this also assumed two rooks per file
	var wStackFile int = -1
	var bStackFile int = -1

	for color, bb := range bbs {
		for x := bb; x != 0; x &= x - 1 {
			sq := bits.TrailingZeros64(x)
			file := getFileOfSquare(sq)
			if color == 0 {
				if bits.OnesCount64(b.White.Pawns&file) == 0 { // Prerequisite
					if bits.OnesCount64(b.Black.Pawns&file) != 0 { // Semi-open
						mgScore += SemiOpenBonusMG
						if bits.OnesCount64(b.Black.Queens&file) > 0 {
							mgScore += RookXrayQueenOnOpenFileBonusMG
						}
					} else { // Open
						if bits.OnesCount64(b.White.Rooks&file) > 1 && wStackFile != -1 {
							wStackFile = sq % 8 // Update so we don't count rooks that are stacked twice
							mgScore += ConnectedRooksBonusMG
						}
						mgScore += OpenFileBonusMG
					}
				}
			} else {
				if bits.OnesCount64(b.Black.Pawns&file) == 0 { // Prerequisite
					if bits.OnesCount64(b.White.Pawns&file) != 0 { // Semi-Open
						mgScore -= SemiOpenBonusMG
						if bits.OnesCount64(b.White.Queens&file) > 0 {
							mgScore -= RookXrayQueenOnOpenFileBonusMG
						}
					} else { // Open
						if bits.OnesCount64(b.Black.Rooks&file) > 1 && bStackFile != -1 {
							bStackFile = sq % 8 // Update so we don't count rooks that are stacked twice
							mgScore -= ConnectedRooksBonusMG
						}
						mgScore -= OpenFileBonusMG
					}
				}
			}
		}
	}
	return mgScore
}

func rookSeventhRankBonus(b *dragontoothmg.Board) (mgScore, egScore int) {
	seventhRankRooks := bits.OnesCount64(b.White.Rooks & secondRankMask)
	secondRankRooks := bits.OnesCount64(b.Black.Rooks & seventhRankMask)
	if seventhRankRooks > 0 {
		if seventhRankRooks == 1 {
			mgScore += seventhRankBonusMG // baby oink
			egScore += seventhRankBonusEG // twin oink
		} else {
			mgScore += seventhRankBonusMG * 2 // single oink
			egScore += seventhRankBonusEG * 2 // double oink
		}
	}

	if secondRankRooks > 0 {
		if secondRankRooks == 1 {
			mgScore -= seventhRankBonusMG // baby oink
			egScore -= seventhRankBonusEG // twin oink
		} else {
			mgScore -= seventhRankBonusMG * 2 // single oink
			egScore -= seventhRankBonusEG * 2 // double oink
		}
	}
	return mgScore, egScore
}

func bishopXrayPenalty(b *dragontoothmg.Board) (xrayMG, xrayEG int) {
	for x := b.White.Bishops; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		bishopMovementBB := dragontoothmg.CalculateBishopMoveBitboard(uint8(sq), (b.White.All|b.Black.All)) & ^b.White.All
		bishopMovementBB = bishopMovementBB & ((onlyFile[3] | onlyFile[4]) & (onlyRank[3] | onlyRank[4]))
		if bits.OnesCount64(bishopMovementBB&(b.White.Pawns|b.Black.Pawns)) > 0 {
			xrayMG -= bits.OnesCount64(bishopMovementBB&(b.White.Pawns|b.Black.Pawns)) * bishopPawnXrayPenaltyMG
			xrayEG -= bits.OnesCount64(bishopMovementBB&(b.White.Pawns|b.Black.Pawns)) * bishopPawnXrayPenaltyEG
		}
	}

	for x := b.Black.Bishops; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		bishopMovementBB := dragontoothmg.CalculateBishopMoveBitboard(uint8(sq), (b.White.All|b.Black.All)) & ^b.Black.All
		bishopMovementBB = bishopMovementBB & ((onlyFile[3] | onlyFile[4]) & (onlyRank[3] | onlyRank[4]))
		if bits.OnesCount64(bishopMovementBB&(b.White.Pawns|b.Black.Pawns)) > 0 {
			xrayMG += bits.OnesCount64(bishopMovementBB&(b.White.Pawns|b.Black.Pawns)) * bishopPawnXrayPenaltyMG
			xrayEG += bits.OnesCount64(bishopMovementBB&(b.White.Pawns|b.Black.Pawns)) * bishopPawnXrayPenaltyEG
		}
	}
	return xrayMG, xrayEG
}

func bishopPairBonuses(b *dragontoothmg.Board) (bishopPairMG, bishopPairEG int) {
	whiteBishops := bits.OnesCount64(b.White.Bishops)
	blackBishops := bits.OnesCount64(b.Black.Bishops)
	if whiteBishops > 1 {
		bishopPairMG += bishopPairBonusMG
		bishopPairEG += bishopPairBonusEG
	}
	if blackBishops > 1 {
		bishopPairMG -= bishopPairBonusMG
		bishopPairEG -= bishopPairBonusEG
	}
	return bishopPairMG, bishopPairEG
}

func openFilesNextToKing(b *dragontoothmg.Board) (int, int) {
	var wOpenFilePenaltyCount = 0
	var wSemiOpenFilePenaltyCount = 0
	var bOpenFilePenaltyCount = 0
	var bSemiOpenFilePenaltyCount = 0
	pawnFillWhite := calculatePawnFileFill(b.White.Pawns)
	pawnFillBlack := calculatePawnFileFill(b.Black.Pawns)
	openFilesWhite := ^pawnFillWhite & ^pawnFillBlack
	openFilesBlack := ^pawnFillBlack & ^pawnFillWhite
	halfOpenFilesWhite := ^pawnFillWhite ^ openFilesBlack
	halfOpenFilesBlack := ^pawnFillBlack ^ openFilesWhite

	// Variables for getting white & black king files
	wKingSquare := bits.TrailingZeros64(b.White.Kings)
	wKingBB := PositionBB[wKingSquare]
	var wLeftFile uint64
	var wRightFile uint64
	if wKingSquare == 64 {
		wLeftFile = PositionBB[wKingSquare-1] &^ bitboardFileH
	} else if wKingSquare == 0 {
		wRightFile = PositionBB[wKingSquare+1] &^ bitboardFileA
	} else {
		wLeftFile = PositionBB[wKingSquare-1] &^ bitboardFileH
		wRightFile = PositionBB[wKingSquare+1] &^ bitboardFileA
	}

	bKingSquare := bits.TrailingZeros64(b.Black.Kings)
	bKingBB := PositionBB[bKingSquare]
	var bLeftFile uint64
	var bRightFile uint64
	if bKingSquare == 64 {
		bLeftFile = PositionBB[wKingSquare-1] &^ bitboardFileH
	} else if wKingSquare == 0 {
		bRightFile = PositionBB[wKingSquare+1] &^ bitboardFileA
	} else {
		bLeftFile = PositionBB[wKingSquare-1] &^ bitboardFileH
		bRightFile = PositionBB[wKingSquare+1] &^ bitboardFileA
	}

	// White king penalties
	wSemiOpenFilePenaltyCount += bits.OnesCount64(halfOpenFilesWhite & wRightFile)
	wSemiOpenFilePenaltyCount += bits.OnesCount64(halfOpenFilesWhite & wLeftFile)
	wOpenFilePenaltyCount += bits.OnesCount64(openFilesWhite & wRightFile)
	wOpenFilePenaltyCount += bits.OnesCount64(openFilesWhite & wLeftFile)
	wOnOpenFilePenaltyCount := bits.OnesCount64(openFilesWhite & wKingBB)

	// Black king penalties
	bSemiOpenFilePenaltyCount += bits.OnesCount64(halfOpenFilesBlack & bLeftFile)
	bSemiOpenFilePenaltyCount += bits.OnesCount64(halfOpenFilesBlack & bRightFile)
	bOpenFilePenaltyCount += bits.OnesCount64(openFilesBlack & bLeftFile)
	bOpenFilePenaltyCount += bits.OnesCount64(openFilesBlack & bRightFile)
	bOnOpenFilePenaltyCount := bits.OnesCount64(openFilesBlack & bKingBB)

	semiOpenPenalty := (wSemiOpenFilePenaltyCount*kingSemiOpenFileNextToPenalty)*-1 + (bSemiOpenFilePenaltyCount * kingSemiOpenFileNextToPenalty)
	openPenalty := (bOpenFilePenaltyCount*kingOpenFileNextToPenalty)*-1 + (bOpenFilePenaltyCount * kingOpenFileNextToPenalty)
	openPenalty += (wOnOpenFilePenaltyCount*kingOnOpenFilePenalty)*-1 + (bOnOpenFilePenaltyCount * kingOnOpenFilePenalty)

	return semiOpenPenalty, openPenalty
}

func kingAttackCountPenalty(attackUnitCount *[2]int) int {

	if attackUnitCount[0] > 99 {
		attackUnitCount[0] = 99
	}
	if attackUnitCount[1] > 99 {
		attackUnitCount[1] = 99
	}

	return (kingSafetyTable[attackUnitCount[0]]) - (kingSafetyTable[attackUnitCount[1]])
}

func kingEndGameCentralization(b *dragontoothmg.Board) (kingCmdEG int) {
	kingCmdEG -= centerManhattanDistance[bits.TrailingZeros64(b.White.Kings)] * 10
	kingCmdEG += centerManhattanDistance[bits.TrailingZeros64(b.Black.Kings)] * 10
	return kingCmdEG
}

func Evaluation(b *dragontoothmg.Board, debug bool) int {
	timeStarted := time.Now()

	//######	START UPDATE & INIT VARIABLES FOR EVAL
	score := 0

	var piecePhase = getPiecePhase(b)
	var currPhase = TotalPhase - piecePhase
	phase := (currPhase*256 + (TotalPhase / 2)) / TotalPhase

	// Update outpost bitboards
	outposts := getOutpostsBB(b)
	whiteOutposts = outposts[0]
	blackOutposts = outposts[1]

	var pawnMG, pawnEG int
	var knightMG, knightEG int
	var bishopMG, bishopEG int
	var rookMG, rookEG int
	var queenMG, queenEG int
	var kingMG, kingEG int

	wMaterialMG, wMaterialEG := CountMaterial(&b.White)
	bMaterialMG, bMaterialEG := CountMaterial(&b.Black)

	// For king safety ...
	var attackUnitCounts = [2]int{
		0: 0,
		1: 0,
	}

	var innerKingSafetyZones = getKingSafetyTable(b, false)
	var outerKingSafetyZones = getKingSafetyTable(b, true)

	if debug {
		println("################### TACTICAL PIECE VALUE ###################")
	}
	for _, piece := range pieceList {
		switch piece {
		case dragontoothmg.Pawn:
			pawnPsqtMG, pawnPsqtEG := countPieceTables(b, dragontoothmg.Pawn, PieceSquareTablesMidGame[dragontoothmg.Pawn], PieceSquareTablesEndgame[dragontoothmg.Pawn])
			isolatedMG, isolatedEG := isolatedPawnBonus(b)
			doubledMG, doubledEG := pawnDoublingPenalties(b)
			connectedMG, connectedEG, phalanxMG, phalanxEG := connectedOrPhalanxPawnBonus(b)
			blockedMG, blockedEG := blockedPawnPenalty(b)
			passedMG, passedEG := passedPawnBonus(b)
			if debug {
				println("Pawn MG:\t", "PSQT: ", pawnPsqtMG, "\tIsolted: ", isolatedMG, "\tDoubled:", doubledMG, "\tConnected: ", connectedMG, "\tPhalanx: ", phalanxMG, "\tblocked: ", blockedMG, "\tPassed: ", passedMG)
				println("Pawn EG:\t", "PSQT: ", pawnPsqtEG, "\tIsolted: ", isolatedEG, "\tDoubled:", doubledEG, "\tConnected: ", connectedEG, "\tPhalanx: ", phalanxEG, "\tblocked: ", blockedEG, "\tPassed: ", passedEG)
			}
			pawnMG += pawnPsqtMG + passedMG + isolatedMG //+ doubledMG + phalanxMG + blockedMG //+ connectedMG
			pawnEG += pawnPsqtEG + passedEG + isolatedEG //+ doubledEG                         //+ connectedEG + phalanxEG + blockedEG
		case dragontoothmg.Knight:
			knightPsqtMG, knightPsqtEG := countPieceTables(b, dragontoothmg.Knight, PieceSquareTablesMidGame[dragontoothmg.Knight], PieceSquareTablesEndgame[dragontoothmg.Knight])
			knightMobilityMG, knightMobilityEG := pieceMobilityBonus(b, dragontoothmg.Knight, &attackUnitCounts, innerKingSafetyZones, outerKingSafetyZones)
			knightOutpostMG, knightOutpostEG := OutpostBonus(b, dragontoothmg.Knight)
			if debug {
				println("Knight MG:\t", "PSQT: ", knightPsqtMG, "\tMobility: ", knightMobilityMG, "\tOutpost:", knightOutpostMG)
				println("Knight EG:\t", "PSQT: ", knightPsqtEG, "\tMobility: ", knightMobilityEG, "\tOutpost:", knightOutpostEG)
			}
			//knightMobilityMG = 0
			//knightMobilityEG = 0
			knightMG += knightPsqtMG + knightOutpostMG + knightMobilityMG
			knightEG += knightPsqtEG + knightOutpostEG + knightMobilityEG
		case dragontoothmg.Bishop:
			bishopPsqtMG, bishopPsqtEG := countPieceTables(b, dragontoothmg.Bishop, PieceSquareTablesMidGame[dragontoothmg.Bishop], PieceSquareTablesEndgame[dragontoothmg.Bishop])
			bishopXrayMG, bishopXrayEG := bishopXrayPenalty(b)
			bishopMobilityMG, bishopMobilityEG := pieceMobilityBonus(b, dragontoothmg.Bishop, &attackUnitCounts, innerKingSafetyZones, outerKingSafetyZones)
			bishopOutpostMG, bishopOutpostEG := OutpostBonus(b, dragontoothmg.Bishop)
			bishopPairMG, bishopPairEG := bishopPairBonuses(b)
			if debug {
				println("Bishop MG:\t", "PSQT: ", bishopPsqtMG, "\tMobility: ", bishopMobilityMG, "\tOutpost:", bishopOutpostMG, "\tPair: ", bishopPairMG, "\tXray: ", bishopXrayMG)
				println("Bishop EG:\t", "PSQT: ", bishopPsqtEG, "\tMobility: ", bishopMobilityEG, "\tOutpost:", bishopOutpostEG, "\tPair: ", bishopPairEG, "\tXray: ", bishopXrayEG)
			}
			bishopMG += bishopPsqtMG + bishopOutpostMG + bishopPairMG + bishopMobilityMG //+ bishopXrayMG
			bishopEG += bishopPsqtEG + bishopOutpostEG + bishopPairEG + bishopMobilityEG //+ bishopXrayEG
		case dragontoothmg.Rook:
			rookPsqtMG, rookPsqtEG := countPieceTables(b, dragontoothmg.Rook, PieceSquareTablesMidGame[dragontoothmg.Rook], PieceSquareTablesEndgame[dragontoothmg.Rook])
			rookOpenMG := rookOpenFileBonus(b)
			rookSeventhBonusMG, rookSeventhBonusEG := rookSeventhRankBonus(b)
			rookMobilityMG, rookMobilityEG := pieceMobilityBonus(b, dragontoothmg.Rook, &attackUnitCounts, innerKingSafetyZones, outerKingSafetyZones)
			if debug {
				println("Rook MG:\t", "PSQT: ", rookPsqtMG, "\tMobility: ", rookMobilityMG, "\tSeventh: ", rookSeventhBonusMG, "\tOpen: ", rookOpenMG)
				println("Rook EG:\t", "PSQT: ", rookPsqtEG, "\tMobility: ", rookMobilityEG, "\tSeventh: ", rookSeventhBonusEG)
			}
			rookMobilityMG = 0
			rookMobilityMG = 0
			rookMG += rookPsqtMG + rookMobilityMG + rookSeventhBonusMG + rookOpenMG
			rookEG += rookPsqtEG + rookMobilityEG + rookSeventhBonusEG
		case dragontoothmg.Queen:
			queenPsqtMG, queenPsqtEG := countPieceTables(b, dragontoothmg.Queen, PieceSquareTablesMidGame[dragontoothmg.Queen], PieceSquareTablesEndgame[dragontoothmg.Queen])
			queenMobilityMG, queenMobilityEG := pieceMobilityBonus(b, dragontoothmg.Queen, &attackUnitCounts, innerKingSafetyZones, outerKingSafetyZones)
			if debug {
				println("Queen MG:\t", "PSQT: ", queenPsqtMG, "\tMobility: ", queenMobilityMG)
				println("Queen EG:\t", "PSQT: ", queenPsqtEG, "\tMobility: ", queenMobilityEG)
			}
			queenMG += queenPsqtMG + queenMobilityMG
			queenEG += queenPsqtEG + queenMobilityEG
		case dragontoothmg.King:
			kingPqstMG, kingPqstEG := countPieceTables(b, dragontoothmg.King, PieceSquareTablesMidGame[dragontoothmg.King], PieceSquareTablesEndgame[dragontoothmg.King])
			kingSemiOpenFilePenalty, kingOpenFilePenalty := openFilesNextToKing(b)
			attackPenalty := kingAttackCountPenalty(&attackUnitCounts)
			//kingMopUpValue := 0
			kingCentralManhattanPenalty := 0
			if piecePhase < 8 && (bits.OnesCount64(b.White.Queens) == 0) || bits.OnesCount64(b.Black.Queens) == 0 {
				kingCentralManhattanPenalty = kingEndGameCentralization(b)
			}
			if debug {
				println("King MG:\t", "PSQT: ", kingPqstMG, "\tSemiOpen: ", kingSemiOpenFilePenalty, "\tOpen: ", kingOpenFilePenalty, "\tAttack: ", attackPenalty)
				println("King EG:\t", "PSQT: ", kingPqstEG, "\tCentralization: ", kingCentralManhattanPenalty, "\tAttack: ", attackPenalty)
			}
			kingMG += kingPqstMG + attackPenalty + kingSemiOpenFilePenalty + kingOpenFilePenalty
			kingEG += kingPqstEG + kingCentralManhattanPenalty + attackPenalty
		}
	}

	var spaceBonus = spaceArea(b)

	var materialScoreMG = (wMaterialMG - bMaterialMG)
	var materialScoreEG = (wMaterialEG - bMaterialEG)
	var variableScoreMG = pawnMG + knightMG + bishopMG + rookMG + queenMG + kingMG + spaceBonus
	var variableScoreEG = pawnEG + knightEG + bishopEG + rookEG + queenEG + kingEG
	var mgScore = variableScoreMG + materialScoreMG
	var egScore = variableScoreEG + materialScoreEG

	/* TEMPO BONUS; never seem to make any difference.
	var tempoBonusMG = 10
	var tempoBonusEG = 5
	if b.Wtomove {
		mgScore += tempoBonusMG
		egScore += tempoBonusEG
	} else {
		mgScore -= tempoBonusMG
		egScore -= tempoBonusEG
	}
	*/

	if debug {
		println("################### START PHASE ###################")
		println("Piece phase: \t\t", piecePhase)
		println("Calculated phase: \t", phase)
		println("Total phase: \t\t", TotalPhase)
		println("Reduced phase: \t\t", (currPhase*256+(TotalPhase/2))/TotalPhase)
		println("Phase MG: \t\t", mgScore)
		println("Phase EG: \t\t", egScore)
	}

	score = int(((float64(mgScore) * (float64(256) - float64(phase))) + (float64(egScore) * float64(phase))) / float64(256))
	if isTheoreticalDraw(b, debug) {
		score = int(score / DrawDivider)
	}

	if debug {
		println("################### MIDGAME_EVAL:ENDGAME_EVAL  ###################")
		println("Pawn eval: \t\t", pawnMG, ":", pawnEG)
		println("Knight eval: \t\t", knightMG, ":", knightEG)
		println("Bishop eval: \t\t", bishopMG, ":", bishopEG)
		println("Rook eval: \t\t", rookMG, ":", rookEG)
		println("Queen eval: \t\t", queenMG, ":", queenEG)
		println("King eval: \t\t", kingMG, ":", kingEG)
		println("Non-material eval: \t", variableScoreMG, ":", variableScoreEG)
		println("Material eval: \t\t", materialScoreMG, ":", materialScoreEG)
		println("White attacking unit count: \t", attackUnitCounts[0])
		println("Black attacking unit count: \t", attackUnitCounts[1])
		println("Total score: \t\t", score)
	}

	if !b.Wtomove {
		score = -score
	}

	TotalEvalTime += time.Since(timeStarted)
	return score
}
