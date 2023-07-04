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

var PositionBB [65]uint64

// Score penalty for moving towards a theoretical draw
var DrawDivider = 12

var TotalEvalTime time.Duration

// Values we use to set
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

// List, for iterative purposes!
var pieceList = [6]dragontoothmg.Piece{dragontoothmg.Pawn, dragontoothmg.Knight, dragontoothmg.Bishop, dragontoothmg.Rook, dragontoothmg.Queen, dragontoothmg.King}

// Piece values
var pieceValueMG = [7]int{dragontoothmg.King: 0, dragontoothmg.Pawn: 82, dragontoothmg.Knight: 337, dragontoothmg.Bishop: 365, dragontoothmg.Rook: 477, dragontoothmg.Queen: 1025}
var pieceValueEG = [7]int{dragontoothmg.King: 0, dragontoothmg.Pawn: 94, dragontoothmg.Knight: 281, dragontoothmg.Bishop: 297, dragontoothmg.Rook: 512, dragontoothmg.Queen: 936}

// Mobility bonuses
var mobilityValueMG = [7]int{dragontoothmg.Pawn: 0, dragontoothmg.Knight: 2, dragontoothmg.Bishop: 2, dragontoothmg.Rook: 2, dragontoothmg.Queen: 2, dragontoothmg.King: 0}
var mobilityValueEG = [7]int{dragontoothmg.Pawn: 0, dragontoothmg.Knight: 1, dragontoothmg.Bishop: 2, dragontoothmg.Rook: 5, dragontoothmg.Queen: 6, dragontoothmg.King: 0}

// King safety variables
var attackerInner = [7]int{dragontoothmg.Pawn: 1, dragontoothmg.Knight: 2, dragontoothmg.Bishop: 2, dragontoothmg.Rook: 4, dragontoothmg.Queen: 6, dragontoothmg.King: 0}
var attackerOuter = [7]int{dragontoothmg.Pawn: 0, dragontoothmg.Knight: 1, dragontoothmg.Bishop: 1, dragontoothmg.Rook: 2, dragontoothmg.Queen: 2, dragontoothmg.King: 0}

// Outpost variables, updated each time evaluation is called
var whiteOutposts uint64
var blackOutposts uint64

var wPawnAttackBB uint64
var bPawnAttackBB uint64

// Helpful masks
var wPassedRankBB uint64 = 0xffffffffffffff00
var bPassedRankBB uint64 = 0xffffffffffffff

var wAllowedOutpostMask uint64 = 0xffff7e7e000000
var bAllowedOutpostMask uint64 = 0x7e7effff00

var seventhRankMask uint64 = 0xff000000000000
var secondRankMask uint64 = 0xff00

var wCentralSquaresMask uint64 = 0x3c3c3c00
var bCentralSquaresMask uint64 = 0x3c3c3c00000000

// Constants which map a piece to how much weight it should have on the phase of the game.
const (
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

// Bishop variables
var bishopOutpost = 15
var bishopPairBonusMG = 10
var bishopPairBonusEG = 40

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

var pieceSquareTablesMidGame = [7][64]int{
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

var pieceSquareTablesEndGame = [7][64]int{
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

	var wBehindPawnArea uint64
	var bBehindPawnArea uint64

	for x := b.White.Pawns; x != 0; x &= x - 1 {
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

	for x := b.Black.Pawns; x != 0; x &= x - 1 {
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

	wBehindPawnArea = (wBehindPawnArea & wCentralSquaresMask) &^ (bPawnAttackBB | b.White.Pawns)
	bBehindPawnArea = (bBehindPawnArea & bCentralSquaresMask) &^ (wPawnAttackBB | b.Black.Pawns)

	wSpace := wCentralSquaresMask &^ (wBehindPawnArea | b.White.Pawns | bPawnAttackBB)
	bSpace := bCentralSquaresMask &^ (bBehindPawnArea | b.Black.Pawns | wPawnAttackBB)

	wPieceCount := bits.OnesCount64(b.White.All)
	bPieceCount := bits.OnesCount64(b.Black.All)

	wScore := (wPieceCount) * (bits.OnesCount64(wSpace) + (bits.OnesCount64(wBehindPawnArea) * 2))
	bScore := (bPieceCount) * (bits.OnesCount64(bSpace) + (bits.OnesCount64(bBehindPawnArea) * 2))

	//var weight = pieceCount - 3 + Math.min(blockedCount, 9)
	//return ((space_area(pos, square) * weight * weight / 16) << 0)

	return wScore - bScore
}

func pieceMobilityBonus(b *dragontoothmg.Board, pieceType dragontoothmg.Piece, attackUnitsCount *[2]int, innerKingSafetyZones [2]uint64, outerKingSafetyZones [2]uint64) (mobilityMG, mobilityEG int) {
	var wPieceBB uint64
	var bPieceBB uint64

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

	for x := wPieceBB; x != 0; x &= x - 1 {
		square := bits.TrailingZeros64(x)
		var movementBoard uint64
		var movesCount = 0
		if pieceType == dragontoothmg.Knight {
			movementBoard = KnightMasks[square] &^ b.White.All
			movesCount = bits.OnesCount64(movementBoard&bPawnAttackBB) - 4
		} else if pieceType == dragontoothmg.Bishop {
			movementBoard = dragontoothmg.CalculateBishopMoveBitboard(uint8(square), (b.White.All|b.Black.All)) & ^b.White.All
			movesCount = bits.OnesCount64(movementBoard) - 7
		} else if pieceType == dragontoothmg.Rook {
			movementBoard = dragontoothmg.CalculateRookMoveBitboard(uint8(square), (b.White.All|b.Black.All)) & ^b.White.All
			movesCount = bits.OnesCount64(movementBoard) - 7
		} else if pieceType == dragontoothmg.Queen {
			movementBoard = dragontoothmg.CalculateRookMoveBitboard(uint8(square), (b.White.All|b.Black.All)) & ^b.White.All
			movementBoard = movementBoard | ((dragontoothmg.CalculateRookMoveBitboard(uint8(square), (b.White.All | b.Black.All))) & ^b.White.All)
			movesCount = bits.OnesCount64(movementBoard) - 14
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
			movesCount = bits.OnesCount64(movementBoard&wPawnAttackBB) - 4
		} else if pieceType == dragontoothmg.Bishop {
			movementBoard = dragontoothmg.CalculateBishopMoveBitboard(uint8(square), (b.White.All|b.Black.All)) & ^b.Black.All
			movesCount = bits.OnesCount64(movementBoard) - 7
		} else if pieceType == dragontoothmg.Rook {
			movementBoard = dragontoothmg.CalculateRookMoveBitboard(uint8(square), (b.White.All|b.Black.All)) & ^b.Black.All
			movesCount = bits.OnesCount64(movementBoard) - 7
		} else if pieceType == dragontoothmg.Queen {
			movementBoard = dragontoothmg.CalculateBishopMoveBitboard(uint8(square), (b.White.All|b.Black.All)) & ^b.Black.All
			movementBoard = movementBoard | (dragontoothmg.CalculateRookMoveBitboard(uint8(square), (b.White.All|b.Black.All)) & ^b.Black.All)
			movesCount = bits.OnesCount64(movementBoard) - 14
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

func countMaterial(bb *dragontoothmg.Bitboards) (materialMG, materialEG int) {
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

func countPieceTables(wPieceBB *uint64, bPieceBB *uint64, ptm *[64]int, pte *[64]int) (mgScore int, egScore int) {

	for x := *wPieceBB; x != 0; x &= x - 1 {
		var idx = bits.TrailingZeros64(x)
		mgScore += ptm[idx]
		egScore += pte[idx]
	}
	for x := *bPieceBB; x != 0; x &= x - 1 {
		var idx = bits.TrailingZeros64(x)
		revView := flipView[idx]
		mgScore -= ptm[revView]
		egScore -= pte[revView]
	}
	return mgScore, egScore
}

func blockedPawnPenalty(b *dragontoothmg.Board) (blockedMG, blockedEG int) {
	for x := b.White.Pawns; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		pawnBB := PositionBB[sq] << 8
		if bits.OnesCount64(pawnBB&(b.Black.Pawns&onlyRank[5])) > 0 {
			blockedMG += BlockedPawn5thMG
			blockedEG += BlockedPawn5thEG
		} else if bits.OnesCount64(pawnBB&b.Black.Pawns&onlyRank[6]) > 0 {
			blockedMG += BlockedPawn6thMG
			blockedEG += BlockedPawn6thEG
		}
	}

	for x := b.Black.Pawns; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		pawnBB := PositionBB[sq] >> 8
		if bits.OnesCount64(pawnBB&b.White.Pawns&onlyRank[2]) > 0 {
			blockedMG -= BlockedPawn5thMG
			blockedEG -= BlockedPawn5thEG
		} else if bits.OnesCount64(pawnBB&b.White.Pawns&onlyRank[1]) > 0 {
			blockedMG -= BlockedPawn6thMG
			blockedEG -= BlockedPawn6thEG
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

	for x := b.White.Pawns; x != 0; x &= x - 1 {
		pawnBB := PositionBB[bits.TrailingZeros64(x)]
		wConnectedPawns += bits.OnesCount64(wPawnAttackBB & pawnBB)
		wPhalanxBB = wPhalanxBB | (((PositionBB[bits.TrailingZeros64(x)-1]) & b.White.Pawns &^ bitboardFileH) | ((PositionBB[bits.TrailingZeros64(x)+1]) & b.White.Pawns &^ bitboardFileA))
	}
	for x := b.Black.Pawns; x != 0; x &= x - 1 {
		pawnBB := PositionBB[bits.TrailingZeros64(x)]
		bConnectedPawns += bits.OnesCount64(bPawnAttackBB & pawnBB)
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

	for x := b.White.Pawns; x != 0; x &= x - 1 {
		idx := bits.TrailingZeros64(x)
		file := idx % 8
		neighbors := bits.OnesCount64(isolatedPawnTable[file]&b.White.Pawns) - 1
		if neighbors == 0 {
			isolatedMG -= IsolatedPawnMG
			isolatedEG -= IsolatedPawnEG
		}
	}

	for x := b.Black.Pawns; x != 0; x &= x - 1 {
		idx := bits.TrailingZeros64(x)
		file := idx % 8
		neighbors := bits.OnesCount64(isolatedPawnTable[file]&b.Black.Pawns) - 1
		if neighbors == 0 {
			isolatedMG += IsolatedPawnMG
			isolatedEG += IsolatedPawnEG
		}
	}

	return isolatedMG, isolatedEG
}

func passedPawnBonus(b *dragontoothmg.Board) (passedMG, passedEG int) {
	for x := b.White.Pawns; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		rank := sq / 8
		pawnFile := onlyFile[sq%8]
		var checkAbove = ranksAbove[(sq/8)+1]

		if bits.OnesCount64(bPawnAttackBB&(pawnFile&checkAbove)) == 0 && bits.OnesCount64(b.Black.Pawns&(pawnFile&checkAbove)) == 0 {
			passedMG += passedPawnBonusMG[rank]
			passedEG += passedPawnBonusEG[rank]
		}
	}

	for x := b.Black.Pawns; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		//rank := sq / 8
		pawnFile := onlyFile[sq%8]
		var checkBelow = ranksBelow[(sq/8)+1]

		if bits.OnesCount64(wPawnAttackBB&(pawnFile&checkBelow)) == 0 && bits.OnesCount64(b.White.Pawns&(pawnFile&checkBelow)) == 0 {
			revSQ := flipView[sq]
			rank := revSQ / 8
			passedMG -= passedPawnBonusMG[rank]
			passedEG -= passedPawnBonusEG[rank]
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
	wSecondRankMG := bits.OnesCount64(b.White.Rooks&seventhRankMask) * (seventhRankBonusMG * 2)
	wSecondRankEG := bits.OnesCount64(b.White.Rooks&secondRankMask) * seventhRankBonusEG

	bSecondRankMG := bits.OnesCount64(b.Black.Rooks&seventhRankMask) * (seventhRankBonusMG * 2)
	bSecondRankEG := bits.OnesCount64(b.Black.Rooks&secondRankMask) * seventhRankBonusEG

	mgScore += wSecondRankMG - bSecondRankMG
	egScore += wSecondRankEG - bSecondRankEG

	return mgScore, egScore
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
	// UPDATE & INIT VARIABLES FOR EVAL
	score := 0

	// Prepare pawn attack bitboards
	wPawnAttackBBEast, wPawnAttackBBWest := PawnCaptureBitboards(b.White.Pawns, true)
	bPawnAttackBBEast, bPawnAttackBBWest := PawnCaptureBitboards(b.Black.Pawns, false)

	wPawnAttackBB = wPawnAttackBBEast | wPawnAttackBBWest
	bPawnAttackBB = bPawnAttackBBEast | bPawnAttackBBWest

	// Update outpost bitboards
	outposts := getOutpostsBB(b)
	whiteOutposts = outposts[0]
	blackOutposts = outposts[1]

	if debug {
		println("################### HELPER VARIABLES ###################")
		println("Pawn attacks: ", wPawnAttackBB, " <||> ", bPawnAttackBB)
		println("Outposts: ", outposts[0], " <||> ", outposts[1])
	}

	var piecePhase = getPiecePhase(b)
	var currPhase = TotalPhase - piecePhase
	phase := (currPhase*256 + (TotalPhase / 2)) / TotalPhase

	var pawnMG, pawnEG int
	var knightMG, knightEG int
	var bishopMG, bishopEG int
	var rookMG, rookEG int
	var queenMG, queenEG int
	var kingMG, kingEG int

	wMaterialMG, wMaterialEG := countMaterial(&b.White)
	bMaterialMG, bMaterialEG := countMaterial(&b.Black)

	// For king safety ...
	var attackUnitCounts = [2]int{
		0: 0,
		1: 0,
	}

	//var outerKingSafetyZones = getKingSafetyTable(b, true)
	var innerKingSafetyZones = getKingSafetyTable(b, false)

	if debug {
		println("################### TACTICAL PIECE VALUES ###################")
		println("FEN: ", b.ToFen())
	}
	for _, piece := range pieceList {
		switch piece {
		case dragontoothmg.Pawn:
			pawnPsqtMG, pawnPsqtEG := countPieceTables(&b.White.Pawns, &b.Black.Pawns, &pieceSquareTablesMidGame[dragontoothmg.Pawn], &pieceSquareTablesEndGame[dragontoothmg.Pawn])
			isolatedMG, isolatedEG := isolatedPawnBonus(b)
			//doubledMG, doubledEG := pawnDoublingPenalties(b)
			//connectedMG, connectedEG, phalanxMG, phalanxEG := connectedOrPhalanxPawnBonus(b)
			//blockedMG, blockedEG := blockedPawnPenalty(b)
			passedMG, passedEG := passedPawnBonus(b)
			if debug {
				println("Pawn MG:\t", "PSQT: ", pawnPsqtMG, "\tIsolted: ", isolatedMG) // , "\tDoubled:", doubledMG) //"\tConnected: ", connectedMG, "\tPhalanx: ", phalanxMG, "\tblocked: ", blockedMG, "\tPassed: ", passedMG)
				println("Pawn EG:\t", "PSQT: ", pawnPsqtEG, "\tIsolted: ", isolatedEG) // , "\tDoubled:", doubledEG) // "\tConnected: ", connectedEG, "\tPhalanx: ", phalanxEG, "\tblocked: ", blockedEG, "\tPassed: ", passedEG)
			}
			pawnMG += pawnPsqtMG + passedMG + isolatedMG // + doubledMG //+ blockedMG + phalanxMG + connectedMG
			pawnEG += pawnPsqtEG + passedEG + isolatedEG // + doubledEG //+ connectedEG + phalanxEG + blockedEG
		case dragontoothmg.Knight:
			knightPsqtMG, knightPsqtEG := countPieceTables(&b.White.Knights, &b.Black.Knights, &pieceSquareTablesMidGame[dragontoothmg.Knight], &pieceSquareTablesEndGame[dragontoothmg.Knight])
			var knightMobilityMG, knightMobilityEG int //:= pieceMobilityBonus(b, dragontoothmg.Knight, &attackUnitCounts, innerKingSafetyZones, outerKingSafetyZones)
			for x := b.White.Knights; x != 0; x &= x - 1 {
				square := bits.TrailingZeros64(x)
				movementBB := (KnightMasks[square] &^ b.White.All) &^ bPawnAttackBB
				knightMobilityMG += bits.OnesCount64(movementBB) * mobilityValueMG[dragontoothmg.Knight]
				knightMobilityEG += bits.OnesCount64(movementBB) * mobilityValueEG[dragontoothmg.Knight]
				attackUnitCounts[0] += (bits.OnesCount64(movementBB&innerKingSafetyZones[1]) * attackerInner[dragontoothmg.Knight])
			}
			for x := b.Black.Knights; x != 0; x &= x - 1 {
				square := bits.TrailingZeros64(x)
				movementBB := (KnightMasks[square] &^ b.Black.All) &^ wPawnAttackBB
				knightMobilityMG -= bits.OnesCount64(movementBB) * mobilityValueMG[dragontoothmg.Knight]
				knightMobilityEG -= bits.OnesCount64(movementBB) * mobilityValueEG[dragontoothmg.Knight]
				attackUnitCounts[1] += (bits.OnesCount64(movementBB&innerKingSafetyZones[0]) * attackerInner[dragontoothmg.Knight])
			}
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
			bishopPsqtMG, bishopPsqtEG := countPieceTables(&b.White.Bishops, &b.Black.Bishops, &pieceSquareTablesMidGame[dragontoothmg.Bishop], &pieceSquareTablesEndGame[dragontoothmg.Bishop])
			var bishopMobilityMG, bishopMobilityEG int //:= pieceMobilityBonus(b, dragontoothmg.Knight, &attackUnitCounts, innerKingSafetyZones, outerKingSafetyZones)
			for x := b.White.Bishops; x != 0; x &= x - 1 {
				square := bits.TrailingZeros64(x)
				movementBB := (dragontoothmg.CalculateBishopMoveBitboard(uint8(square), (b.White.All|b.Black.All)) & ^b.White.All) &^ bPawnAttackBB
				bishopMobilityMG += bits.OnesCount64(movementBB) * mobilityValueMG[dragontoothmg.Bishop]
				bishopMobilityEG += bits.OnesCount64(movementBB) * mobilityValueEG[dragontoothmg.Bishop]
				attackUnitCounts[0] += (bits.OnesCount64(movementBB&innerKingSafetyZones[1]) * attackerInner[dragontoothmg.Bishop])
			}
			for x := b.Black.Bishops; x != 0; x &= x - 1 {
				square := bits.TrailingZeros64(x)
				movementBB := (dragontoothmg.CalculateBishopMoveBitboard(uint8(square), (b.White.All|b.Black.All)) & ^b.Black.All) &^ wPawnAttackBB
				bishopMobilityMG -= bits.OnesCount64(movementBB) * mobilityValueMG[dragontoothmg.Bishop]
				bishopMobilityEG -= bits.OnesCount64(movementBB) * mobilityValueEG[dragontoothmg.Bishop]
				attackUnitCounts[1] += (bits.OnesCount64(movementBB&innerKingSafetyZones[0]) * attackerInner[dragontoothmg.Bishop])
			}
			bishopOutpostMG, bishopOutpostEG := OutpostBonus(b, dragontoothmg.Bishop)
			bishopPairMG, bishopPairEG := bishopPairBonuses(b)
			if debug {
				println("Bishop MG:\t", "PSQT: ", bishopPsqtMG, "\tMobility: ", bishopMobilityMG, "\tOutpost:", bishopOutpostMG, "\tPair: ", bishopPairMG)
				println("Bishop EG:\t", "PSQT: ", bishopPsqtEG, "\tMobility: ", bishopMobilityEG, "\tOutpost:", bishopOutpostEG, "\tPair: ", bishopPairEG)
			}
			bishopMG += bishopPsqtMG + bishopOutpostMG + bishopPairMG + bishopMobilityMG
			bishopEG += bishopPsqtEG + bishopOutpostEG + bishopPairEG + bishopMobilityEG
		case dragontoothmg.Rook:
			rookPsqtMG, rookPsqtEG := countPieceTables(&b.White.Rooks, &b.Black.Rooks, &pieceSquareTablesMidGame[dragontoothmg.Rook], &pieceSquareTablesEndGame[dragontoothmg.Rook])
			rookOpenMG := rookOpenFileBonus(b)
			rookSeventhBonusMG, rookSeventhBonusEG := rookSeventhRankBonus(b)
			var rookMobilityMG, rookMobilityEG int //:= pieceMobilityBonus(b, dragontoothmg.Rook, &attackUnitCounts, innerKingSafetyZones, outerKingSafetyZones)
			for x := b.White.Rooks; x != 0; x &= x - 1 {
				square := bits.TrailingZeros64(x)
				movementBB := dragontoothmg.CalculateRookMoveBitboard(uint8(square), (b.White.All|b.Black.All)) & ^b.White.All
				rookMobilityMG += bits.OnesCount64(movementBB) * mobilityValueMG[dragontoothmg.Rook]
				rookMobilityEG += bits.OnesCount64(movementBB) * mobilityValueEG[dragontoothmg.Rook]
				attackUnitCounts[0] += (bits.OnesCount64(movementBB&innerKingSafetyZones[1]) * attackerInner[dragontoothmg.Rook])
			}
			for x := b.Black.Rooks; x != 0; x &= x - 1 {
				square := bits.TrailingZeros64(x)
				movementBB := dragontoothmg.CalculateRookMoveBitboard(uint8(square), (b.White.All|b.Black.All)) & ^b.Black.All
				rookMobilityMG -= bits.OnesCount64(movementBB) * mobilityValueMG[dragontoothmg.Rook]
				rookMobilityEG -= bits.OnesCount64(movementBB) * mobilityValueEG[dragontoothmg.Rook]
				attackUnitCounts[1] += (bits.OnesCount64(movementBB&innerKingSafetyZones[0]) * attackerInner[dragontoothmg.Rook])
			}

			if debug {
				println("Rook MG:\t", "PSQT: ", rookPsqtMG, "\tMobility: ", rookMobilityMG, "\tSeventh: ", rookSeventhBonusMG, "\tOpen: ", rookOpenMG)
				println("Rook EG:\t", "PSQT: ", rookPsqtEG, "\tMobility: ", rookMobilityEG, "\tSeventh: ", rookSeventhBonusEG)
			}
			rookMG += rookPsqtMG + rookMobilityMG + rookOpenMG
			rookEG += rookPsqtEG + rookMobilityEG + rookSeventhBonusEG
		case dragontoothmg.Queen:
			queenPsqtMG, queenPsqtEG := countPieceTables(&b.White.Queens, &b.Black.Queens, &pieceSquareTablesMidGame[dragontoothmg.Queen], &pieceSquareTablesEndGame[dragontoothmg.Queen])
			var queenMobilityMG, queenMobilityEG int // := pieceMobilityBonus(b, dragontoothmg.Queen, &attackUnitCounts, innerKingSafetyZones, outerKingSafetyZones)
			for x := b.White.Queens; x != 0; x &= x - 1 {
				square := bits.TrailingZeros64(x)
				movementBB := dragontoothmg.CalculateBishopMoveBitboard(uint8(square), (b.White.All|b.Black.All)) & ^b.White.All
				movementBB = movementBB | ((dragontoothmg.CalculateRookMoveBitboard(uint8(square), (b.White.All | b.Black.All))) & ^b.White.All)
				queenMobilityMG += bits.OnesCount64(movementBB) * mobilityValueMG[dragontoothmg.Queen]
				queenMobilityEG += bits.OnesCount64(movementBB) * mobilityValueEG[dragontoothmg.Queen]
				attackUnitCounts[0] += (bits.OnesCount64(movementBB&innerKingSafetyZones[1]) * attackerInner[dragontoothmg.Queen])
			}
			for x := b.Black.Queens; x != 0; x &= x - 1 {
				square := bits.TrailingZeros64(x)
				movementBB := dragontoothmg.CalculateBishopMoveBitboard(uint8(square), (b.White.All|b.Black.All)) & ^b.Black.All
				movementBB = movementBB | ((dragontoothmg.CalculateRookMoveBitboard(uint8(square), (b.White.All | b.Black.All))) & ^b.Black.All)
				queenMobilityMG -= bits.OnesCount64(movementBB) * mobilityValueMG[dragontoothmg.Queen]
				queenMobilityEG -= bits.OnesCount64(movementBB) * mobilityValueEG[dragontoothmg.Queen]
				attackUnitCounts[1] += (bits.OnesCount64(movementBB&innerKingSafetyZones[0]) * attackerInner[dragontoothmg.Queen])
			}

			if debug {
				println("Queen MG:\t", "PSQT: ", queenPsqtMG, "\tMobility: ", queenMobilityMG)
				println("Queen EG:\t", "PSQT: ", queenPsqtEG, "\tMobility: ", queenMobilityEG)
			}
			queenMG += queenPsqtMG + queenMobilityMG
			queenEG += queenPsqtEG + queenMobilityEG
		case dragontoothmg.King:
			kingPqstMG, kingPqstEG := countPieceTables(&b.White.Kings, &b.Black.Kings, &pieceSquareTablesMidGame[dragontoothmg.King], &pieceSquareTablesEndGame[dragontoothmg.King])
			kingSemiOpenFilePenalty, kingOpenFilePenalty := openFilesNextToKing(b)
			attackPenalty := kingAttackCountPenalty(&attackUnitCounts)
			kingCentralManhattanPenalty := 0
			if piecePhase < 8 && ((bits.OnesCount64(b.White.Queens) == 0) || bits.OnesCount64(b.Black.Queens) == 0) {
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

	//pawnMG, pawnEG = countPieceTables(&b.White.Pawns, &b.Black.Pawns, &pieceSquareTablesMidGame[dragontoothmg.Pawn], &pieceSquareTablesEndGame[dragontoothmg.Pawn])
	//knightMG, knightEG = countPieceTables(&b.White.Knights, &b.Black.Knights, &pieceSquareTablesMidGame[dragontoothmg.Knight], &pieceSquareTablesEndGame[dragontoothmg.Knight])
	//bishopMG, bishopEG = countPieceTables(&b.White.Bishops, &b.Black.Bishops, &pieceSquareTablesMidGame[dragontoothmg.Bishop], &pieceSquareTablesEndGame[dragontoothmg.Bishop])
	//rookMG, rookEG = countPieceTables(&b.White.Rooks, &b.Black.Rooks, &pieceSquareTablesMidGame[dragontoothmg.Rook], &pieceSquareTablesEndGame[dragontoothmg.Rook])
	//queenMG, queenEG = countPieceTables(&b.White.Queens, &b.Black.Queens, &pieceSquareTablesMidGame[dragontoothmg.Queen], &pieceSquareTablesEndGame[dragontoothmg.Queen])
	//kingMG, kingEG = countPieceTables(&b.White.Kings, &b.Black.Kings, &pieceSquareTablesMidGame[dragontoothmg.King], &pieceSquareTablesEndGame[dragontoothmg.King])

	var variableScoreMG = pawnMG + knightMG + bishopMG + rookMG + queenMG + kingMG + spaceBonus
	var variableScoreEG = pawnEG + knightEG + bishopEG + rookEG + queenEG + kingEG

	var mgScore = variableScoreMG + materialScoreMG
	var egScore = variableScoreEG + materialScoreEG

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

	//if isTheoreticalDraw(b, debug) {
	//	score = int(score / DrawDivider)
	//}

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

	return score
}
