package engine

import (
	"cmp"
	"fmt"
	"math"
	"math/bits"

	"github.com/dylhunn/dragontoothmg"
)

var FlipView = [64]int{
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

// List, for iterative purposes!
var pieceList = [6]dragontoothmg.Piece{dragontoothmg.Pawn, dragontoothmg.Knight, dragontoothmg.Bishop, dragontoothmg.Rook, dragontoothmg.Queen, dragontoothmg.King}

/* Helper variables */
// Outpost variables, updated each time evaluation is called
var whiteOutposts uint64
var blackOutposts uint64

var wPawnAttackBB uint64
var bPawnAttackBB uint64

var wPhalanxOrConnectedEndgameInvalidSquares uint64 = 0xffff00
var bPhalanxOrConnectedEndgameInvalidSquares uint64 = 0xffff0000000000

var wAllowedOutpostMask uint64 = 0xffff7e7e000000
var bAllowedOutpostMask uint64 = 0x7e7effff00

var seventhRankMask uint64 = 0xff000000000000
var secondRankMask uint64 = 0xff00

const (
	PawnPhase   = 0
	KnightPhase = 1
	BishopPhase = 1
	RookPhase   = 2
	QueenPhase  = 4
	TotalPhase  = PawnPhase*16 + KnightPhase*4 + BishopPhase*4 + RookPhase*4 + QueenPhase*2
)

/* General variables */

var DrawDivider = 12

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

var weakSquaresPenalty = 2
var weakKingSquaresPenalty = 5

var pieceValueMG = [7]int{dragontoothmg.King: 0, dragontoothmg.Pawn: 82, dragontoothmg.Knight: 337, dragontoothmg.Bishop: 365, dragontoothmg.Rook: 477, dragontoothmg.Queen: 1025}
var pieceValueEG = [7]int{dragontoothmg.King: 0, dragontoothmg.Pawn: 94, dragontoothmg.Knight: 281, dragontoothmg.Bishop: 297, dragontoothmg.Rook: 512, dragontoothmg.Queen: 936}

var mobilityValueMG = [7]int{dragontoothmg.Pawn: 0, dragontoothmg.Knight: 2, dragontoothmg.Bishop: 2, dragontoothmg.Rook: 2, dragontoothmg.Queen: 2, dragontoothmg.King: 0}
var mobilityValueEG = [7]int{dragontoothmg.Pawn: 0, dragontoothmg.Knight: 1, dragontoothmg.Bishop: 2, dragontoothmg.Rook: 5, dragontoothmg.Queen: 6, dragontoothmg.King: 0}

var attackerInner = [7]int{dragontoothmg.Pawn: 1, dragontoothmg.Knight: 2, dragontoothmg.Bishop: 2, dragontoothmg.Rook: 4, dragontoothmg.Queen: 6, dragontoothmg.King: 0}
var attackerOuter = [7]int{dragontoothmg.Pawn: 0, dragontoothmg.Knight: 1, dragontoothmg.Bishop: 1, dragontoothmg.Rook: 2, dragontoothmg.Queen: 2, dragontoothmg.King: 0}

/* Pawn variables */
var PassedPawnPSQT_MG = [64]int{
	0, 0, 0, 0, 0, 0, 0, 0,
	8, 9, 2, -8, -3, 8, 16, 9,
	5, 3, -3, -14, -3, 10, 13, 19,
	14, 0, -9, -7, -13, -7, 9, 16,
	28, 17, 13, 10, 10, 19, 6, 1,
	48, 43, 43, 30, 24, 31, 12, 2,
	45, 52, 42, 43, 28, 34, 19, 9,
	0, 0, 0, 0, 0, 0, 0, 0,
}
var PassedPawnPSQT_EG = [64]int{
	0, 0, 0, 0, 0, 0, 0, 0,
	2, 3, -4, 0, -2, -1, 7, 6,
	8, 6, 5, 1, 1, -1, 14, 7,
	29, 26, 21, 18, 17, 19, 34, 30,
	55, 52, 42, 35, 30, 34, 56, 52,
	91, 83, 66, 40, 30, 61, 67, 84,
	77, 74, 63, 53, 59, 60, 72, 77,
	0, 0, 0, 0, 0, 0, 0, 0,
}

var DoubledPawnPenaltyMG = 15
var DoubledPawnPenaltyEG = 30
var IsolatedPawnMG = 10
var IsolatedPawnEG = 20
var ConnectedPawnsBonusMG = 7
var ConnectedPawnsBonusEG = 9
var PhalanxPawnsBonusMG = 5
var PhalanxPawnsBonusEG = 7
var BlockedPawnBonusMG = 25
var BlockedPawnBonusEG = 15

/* Knight variables */
var KnightOutpostMG = 20
var KnightOutpostEG = 15
var KnightCanAttackPieceMG = 3
var KnightCanAttackPieceEG = 1

/* Bishop variables */
var BishopOutpostMG = 15
var BishopPairBonusMG = 20
var BishopPairBonusEG = 40
var BishopPawnSetupPenaltyMG = 5
var BishopPawnSetupPenaltyEG = 8
var BishopXrayKingMG = 20
var BishopXrayRookMG = 10
var BishopXrayQueenMG = 15

/* Rook variables */
var RookXrayQueenMG = 20
var ConnectedRooksBonusMG = 15
var RookSemiOpenFileBonusMG = 10
var RookOpenFileBonusMG = 15
var SeventhRankBonusEG = 20

/* Queen variables ... Pretty empty :'( */
var centralizedQueenSquares uint64 = 0x183c3c180000
var CentralizedQueenBonusEG = 30
var QueenInfiltrationBonusMG = -5
var QueenInfiltrationBonusEG = 25

/* King variables */
var KingSemiOpenFilePenalty = 10
var KingOpenFilePenalty = 7
var KingMinorPieceDefenseBonus = 3
var KingPawnDefenseMG = 3

var KingSafetyTable = [100]int{
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

var PSQT_MG = [7][64]int{
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

var PSQT_EG = [7][64]int{
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

// Taken from dragontooth chess engine!
var isolatedPawnTable = [8]uint64{
	0x303030303030303, 0x707070707070707, 0xe0e0e0e0e0e0e0e, 0x1c1c1c1c1c1c1c1c,
	0x3838383838383838, 0x7070707070707070, 0xe0e0e0e0e0e0e0e0, 0xc0c0c0c0c0c0c0c0,
}

/* ============= HELPER VARIABLES ============= */
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

var onlyFile = [8]uint64{
	0x0101010101010101, 0x0202020202020202, 0x0404040404040404, 0x0808080808080808,
	0x1010101010101010, 0x2020202020202020, 0x4040404040404040, 0x8080808080808080,
}

var onlyRank = [8]uint64{
	0xFF, 0xFF00, 0xFF0000, 0xFF000000,
	0xFF00000000, 0xFF0000000000, 0xFF000000000000, 0xFF00000000000000,
}

/* ============= HELPER FUNCTIONS ============= */

func min[T cmp.Ordered](a, b T) T {
	if a < b {
		return a
	}
	return b
}
func max[T cmp.Ordered](a, b T) T {
	if a > b {
		return a
	}
	return b
}

/* ============= EVALUATION FUNCTIONS ============= */

func GetPiecePhase(b *dragontoothmg.Board) (phase int) {
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
		//var idx = bits.TrailingZeros64(x)
		revView := FlipView[bits.TrailingZeros64(x)]
		mgScore -= ptm[revView]
		egScore -= pte[revView]
	}
	return mgScore, egScore
}

func getWeakSquares(movementBB [2][5]uint64, kingInnerRing [2]uint64) (weakSquares [2]uint64, weakSquaresKing [2]uint64) {

	var wSide uint64 = 0x3c3c3c7e00     //0xffffffff //(full half side)
	var bSide uint64 = 0x7e3c3c3c000000 //0xffffffff00000000 (full half side)

	// Squares attacked by bishops, knights and rooks
	var wAttackers uint64 = (movementBB[0][0] | movementBB[0][1] | movementBB[0][2])
	var bAttackers uint64 = (movementBB[1][0] | movementBB[1][1] | movementBB[1][2])

	// Undefended squares attacked by opponent pieces
	var wPotentialWeakSquares uint64 = (wSide & bAttackers) // (movementBB[0][0] | movementBB[0][1] | movementBB[0][2])) | (movementBB[0][3] | movementBB[0][4])
	var bPotentialWeakSquares uint64 = (bSide & wAttackers) //(movementBB[1][0] | movementBB[1][1] | movementBB[1][2])) | (movementBB[1][3] | movementBB[1][4])

	// If the squares are defended by friendly pieces, they're not weak anymore
	var wWeakSquares uint64 = wPotentialWeakSquares &^ wAttackers
	var bWeakSquares uint64 = bPotentialWeakSquares &^ bAttackers

	weakSquares[0] = wWeakSquares
	weakSquares[1] = bWeakSquares
	weakSquaresKing[0] = wWeakSquares & kingInnerRing[0]
	weakSquaresKing[1] = bWeakSquares & kingInnerRing[1]

	return weakSquares, weakSquaresKing
}

/*
	PAWN FUNCTIONS
*/

func connectedOrPhalanxPawnBonus(b *dragontoothmg.Board) (connectedMG, connectedEG, phalanxMG, phalanxEG int) {

	var wConnectedMG = bits.OnesCount64(b.White.Pawns & wPawnAttackBB)
	var wConnectedEG = bits.OnesCount64((b.White.Pawns & wPawnAttackBB) &^ wPhalanxOrConnectedEndgameInvalidSquares)
	var bConnectedMG = bits.OnesCount64(b.Black.Pawns & bPawnAttackBB)
	var bConnectedEG = bits.OnesCount64((b.Black.Pawns & bPawnAttackBB) &^ bPhalanxOrConnectedEndgameInvalidSquares)
	connectedMG = (wConnectedMG * ConnectedPawnsBonusMG) - (bConnectedMG * ConnectedPawnsBonusMG)
	connectedEG = (wConnectedEG * ConnectedPawnsBonusEG) - (bConnectedEG * ConnectedPawnsBonusEG)
	var wPhalanxBB uint64
	var bPhalanxBB uint64

	for x := b.White.Pawns; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		wPhalanxBB = wPhalanxBB | ((PositionBB[sq-1]) & b.White.Pawns &^ bitboardFileH) | ((PositionBB[sq+1]) & b.White.Pawns &^ bitboardFileA)
	}
	for x := b.Black.Pawns; x != 0; x &= x - 1 {
		bPhalanxBB = bPhalanxBB | (((PositionBB[bits.TrailingZeros64(x)-1]) & b.Black.Pawns &^ bitboardFileH) | ((PositionBB[bits.TrailingZeros64(x)+1]) & b.Black.Pawns &^ bitboardFileA))
	}

	phalanxMG += (bits.OnesCount64(wPhalanxBB&^secondRankMask) * PhalanxPawnsBonusMG) - (bits.OnesCount64(bPhalanxBB&^seventhRankMask) * PhalanxPawnsBonusMG)
	phalanxEG += (bits.OnesCount64(wPhalanxBB&^secondRankMask) * PhalanxPawnsBonusEG) - (bits.OnesCount64(bPhalanxBB&^seventhRankMask) * PhalanxPawnsBonusEG)

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

	doubledMG = (bDoubledPawnCount * DoubledPawnPenaltyMG) - (wDoubledPawnCount * DoubledPawnPenaltyMG)
	doubledEG = (bDoubledPawnCount * DoubledPawnPenaltyEG) - (wDoubledPawnCount * DoubledPawnPenaltyEG)
	return doubledMG, doubledEG
}

func isolatedPawnPenalty(b *dragontoothmg.Board) (isolatedMG, isolatedEG int) {

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

func passedPawnBonus(b *dragontoothmg.Board, wPawnAttackBB uint64, bPawnAttackBB uint64) (passedMG, passedEG int) {
	var squaresChecked uint64
	for x := b.White.Pawns; x != 0; x &= x - 1 {
		var sq = bits.TrailingZeros64(x)
		var pawnFile = onlyFile[sq%8]
		var pawnRank = onlyRank[sq/8]
		squaresChecked |= (pawnFile & pawnRank)
		var checkAbove = ranksAbove[(sq/8)+1]

		if bits.OnesCount64(bPawnAttackBB&(pawnFile&checkAbove)) == 0 && bits.OnesCount64(b.Black.Pawns&(pawnFile&checkAbove)) == 0 {
			// Connected passed pawns
			passedMG += PassedPawnPSQT_MG[sq]
			passedEG += PassedPawnPSQT_EG[sq]
		}
	}

	for x := b.Black.Pawns; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		pawnFile := onlyFile[sq%8]
		pawnRank := onlyRank[sq/8]
		squaresChecked |= (pawnFile & pawnRank)
		var checkBelow = ranksBelow[(sq/8)-1]

		if bits.OnesCount64(wPawnAttackBB&(pawnFile&checkBelow)) == 0 && bits.OnesCount64(b.White.Pawns&(pawnFile&checkBelow)) == 0 {
			revSQ := FlipView[sq]
			passedMG -= PassedPawnPSQT_MG[revSQ]
			passedEG -= PassedPawnPSQT_EG[revSQ]
		}
	}

	return passedMG, passedEG
}

func blockedPawnBonus(b *dragontoothmg.Board) (blockedBonusMG int, blockedBonusEG int) {
	thirdAndFourthRank := onlyRank[2] | onlyRank[3]
	fifthAndSixthRank := onlyRank[4] | onlyRank[5]

	for x := b.White.Pawns; x != 0; x &= x - 1 {
		squareBB := PositionBB[bits.TrailingZeros64(x)]
		abovePawnBB := squareBB << 8
		if (fifthAndSixthRank&squareBB) > 0 && (b.Black.Pawns&abovePawnBB) > 0 {
			blockedBonusMG += BlockedPawnBonusMG
			blockedBonusEG += BlockedPawnBonusEG
		}
	}
	for x := b.Black.Pawns; x != 0; x &= x - 1 {
		squareBB := PositionBB[bits.TrailingZeros64(x)]
		abovePawnBB := squareBB >> 8
		if (thirdAndFourthRank&squareBB) > 0 && (b.White.Pawns&abovePawnBB) > 0 {
			blockedBonusMG -= BlockedPawnBonusMG
			blockedBonusEG -= BlockedPawnBonusEG
		}
	}
	return blockedBonusMG, blockedBonusEG
}

/*
	KNIGHT FUNCTIONS
*/

func knightThreats(b *dragontoothmg.Board) (knightThreatsMG int, knightThreatsEG int) {
	wPieces := (b.White.Bishops | b.White.Rooks | b.White.Queens)
	bPieces := (b.Black.Bishops | b.Black.Rooks | b.Black.Queens)
	for x := b.White.Knights; x != 0; x &= x - 1 {
		knightMovement := KnightMasks[bits.TrailingZeros64(x)]
		for y := knightMovement; y != 0; y &= y - 1 {
			knightThreatBB := KnightMasks[bits.TrailingZeros64(y)]
			if bits.OnesCount64(knightThreatBB&bPieces) > 0 {
				bPieces &^= knightThreatBB
				knightThreatsMG += KnightCanAttackPieceMG
				knightThreatsEG += KnightCanAttackPieceEG
			}
		}
	}

	for x := b.Black.Knights; x != 0; x &= x - 1 {
		knightMovement := KnightMasks[bits.TrailingZeros64(x)]
		for y := knightMovement; y != 0; y &= y - 1 {
			knightThreatBB := KnightMasks[bits.TrailingZeros64(y)]
			if bits.OnesCount64(knightThreatBB&wPieces) > 0 {
				wPieces &^= knightThreatBB
				knightThreatsMG -= KnightCanAttackPieceMG
				knightThreatsEG -= KnightCanAttackPieceEG
			}
		}
	}

	return knightThreatsMG, knightThreatsEG
}

/*
	BISHOP FUNCTIONS
*/

func bishopPairBonuses(b *dragontoothmg.Board) (bishopPairMG, bishopPairEG int) {

	whiteBishops := bits.OnesCount64(b.White.Bishops)
	blackBishops := bits.OnesCount64(b.Black.Bishops)
	if whiteBishops > 1 && blackBishops < 2 {
		bishopPairMG += BishopPairBonusMG
		bishopPairEG += BishopPairBonusEG
	}
	if blackBishops > 1 && whiteBishops < 2 {
		bishopPairMG -= BishopPairBonusMG
		bishopPairEG -= BishopPairBonusEG
	}
	return bishopPairMG, bishopPairEG
}

func bishopXrayAttacks(b *dragontoothmg.Board) (bishopXrayMG int) {

	for x := b.White.Bishops; x != 0; x &= x - 1 {
		var sq = bits.TrailingZeros64(x)
		var bishopMovementBoard = dragontoothmg.CalculateBishopMoveBitboard(uint8(sq), (b.White.Pawns | b.Black.Pawns)) // We can't xray our own pawns
		if bits.OnesCount64(bishopMovementBoard&b.Black.Kings) > 0 {
			bishopXrayMG += BishopXrayKingMG
		} else if bits.OnesCount64(bishopMovementBoard&b.Black.Rooks) > 0 {
			bishopXrayMG += BishopXrayRookMG
		} else if bits.OnesCount64(bishopMovementBoard&b.Black.Queens) > 0 {
			bishopXrayMG += BishopXrayQueenMG
		}
	}

	for x := b.Black.Bishops; x != 0; x &= x - 1 {
		var sq = bits.TrailingZeros64(x)
		var bishopMovementBoard = dragontoothmg.CalculateBishopMoveBitboard(uint8(sq), (b.White.Pawns | b.Black.Pawns)) // We can't xray our own pawns
		if bits.OnesCount64(bishopMovementBoard&b.Black.Kings) > 0 {
			bishopXrayMG += BishopXrayKingMG
		} else if bits.OnesCount64(bishopMovementBoard&b.White.Rooks) > 0 {
			bishopXrayMG -= BishopXrayRookMG
		} else if bits.OnesCount64(bishopMovementBoard&b.White.Queens) > 0 {
			bishopXrayMG -= BishopXrayQueenMG
		}
	}

	return bishopXrayMG
}

/*
	ROOK FUNCTIONS
*/

func rookFilesBonus(b *dragontoothmg.Board, openFiles uint64, wSemiOpenFiles uint64, bSemiOpenFiles uint64) (semiOpen, open int) {
	whiteRooks := b.White.Rooks
	blackRooks := b.Black.Rooks

	semiOpen = RookSemiOpenFileBonusMG * bits.OnesCount64(wSemiOpenFiles&whiteRooks)
	semiOpen -= RookSemiOpenFileBonusMG * bits.OnesCount64(bSemiOpenFiles&blackRooks)

	open = RookOpenFileBonusMG * bits.OnesCount64(openFiles&whiteRooks)
	open -= RookOpenFileBonusMG * bits.OnesCount64(openFiles&blackRooks)

	return semiOpen, open
}

func rookAttacks(b *dragontoothmg.Board) (xrayMG int) {
	for x := b.White.Rooks; x != 0; x &= x - 1 {
		var sq = bits.TrailingZeros64(x)
		rookMovementBoard := dragontoothmg.CalculateRookMoveBitboard(uint8(sq), (b.White.Pawns | b.Black.Pawns))
		if bits.OnesCount64(rookMovementBoard&b.Black.Queens) > 0 {
			xrayMG += RookXrayQueenMG
		}
	}
	for x := b.Black.Rooks; x != 0; x &= x - 1 {
		var sq = bits.TrailingZeros64(x)
		rookMovementBoard := dragontoothmg.CalculateRookMoveBitboard(uint8(sq), (b.White.Pawns | b.Black.Pawns))
		if bits.OnesCount64(rookMovementBoard&b.White.Queens) > 0 {
			xrayMG -= RookXrayQueenMG
		}
	}
	return xrayMG
}

/*
	QUEEN FUNCTIONS
*/

func centralizedQueen(b *dragontoothmg.Board) (centralizedBonus int) {
	if bits.OnesCount64(b.White.Queens) > 0 && bits.OnesCount64(b.White.Queens&centralizedQueenSquares) > 0 {
		centralizedBonus += CentralizedQueenBonusEG
	}
	if bits.OnesCount64(b.Black.Queens) > 0 && bits.OnesCount64(b.Black.Queens&centralizedQueenSquares) > 0 {
		centralizedBonus += CentralizedQueenBonusEG
	}
	return centralizedBonus
}

func queenInfiltrationBonus(b *dragontoothmg.Board, wPawnAttackSpan uint64, bPawnAttackSpan uint64) (queenInfiltrationBonusMG int, queenInfiltrationBonusEG int) {
	if b.White.Queens&ranksAbove[4] > 0 && b.White.Queens&bPawnAttackSpan == 0 {
		queenInfiltrationBonusMG += QueenInfiltrationBonusMG
		queenInfiltrationBonusEG += QueenInfiltrationBonusEG
	}

	if b.Black.Queens&ranksBelow[4] > 0 && b.Black.Queens&wPawnAttackSpan == 0 {
		queenInfiltrationBonusMG -= QueenInfiltrationBonusMG
		queenInfiltrationBonusEG -= QueenInfiltrationBonusEG
	}

	return queenInfiltrationBonusMG, queenInfiltrationBonusEG
}

/*
	KING FUNCTIONS
*/

func kingMinorPieceDefences(kingInnerRing [2]uint64, knightMovementBB [2]uint64, bishopMovementBB [2]uint64) int {
	wDefendingPiecesCount := bits.OnesCount64(kingInnerRing[0] & (knightMovementBB[0] | bishopMovementBB[0]))
	bDefendingPiecesCount := bits.OnesCount64(kingInnerRing[1] & (knightMovementBB[1] | bishopMovementBB[1]))

	return (wDefendingPiecesCount * KingMinorPieceDefenseBonus) - (bDefendingPiecesCount * KingMinorPieceDefenseBonus)
}

func kingPawnDefense(b *dragontoothmg.Board, kingZoneBBInner [2]uint64) int {
	wPawnsCloseToKing := min(3, bits.OnesCount64(b.White.Pawns&kingZoneBBInner[0]))
	bPawnsCloseToKing := min(3, bits.OnesCount64(b.Black.Pawns&kingZoneBBInner[1]))
	return (wPawnsCloseToKing * KingPawnDefenseMG) - (bPawnsCloseToKing * KingPawnDefenseMG)
}

func getkingEndGamePositionValue(b *dragontoothmg.Board, whiteWithAdvantage bool) (score int) {

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
		friendlyKingFile = bKingSq % 8
		friendlyKingRank = bKingSq / 8
		enemyKingFile = wKingSq % 8
		enemyKingRank = wKingSq / 8
	}

	// Max of either distance by rank or by file; either way we close the distance
	r2r1 := math.Abs(float64(enemyKingRank) - float64(friendlyKingRank))
	f2f1 := math.Abs(float64(enemyKingFile) - float64(friendlyKingFile))

	if whiteWithAdvantage {
		score = Max(int(r2r1), int(f2f1)) * -20
	} else {
		score = Max(int(r2r1), int(f2f1)) * 20
	}

	return score
}

func kingFilesPenalty(b *dragontoothmg.Board, openFiles uint64, wSemiOpenFiles uint64, bSemiOpenFiles uint64) (score int) {
	// Get the king's files
	wKingFile := onlyFile[bits.TrailingZeros64(b.White.Kings)%8]
	bKingFile := onlyFile[bits.TrailingZeros64(b.Black.Kings)%8]

	// Left & right files of the king
	wKingFiles := ((wKingFile & ^bitboardFileA) >> 1) | ((wKingFile & ^bitboardFileH) << 1)
	bKingFiles := ((bKingFile & ^bitboardFileA) >> 1) | ((bKingFile & ^bitboardFileH) << 1)

	wSemiOpenFilesCount := bits.OnesCount64(wKingFiles & bSemiOpenFiles)
	wOpenFilesCount := bits.OnesCount64(wKingFiles & openFiles)
	bSemiOpenFilesCount := bits.OnesCount64(bKingFiles & wSemiOpenFiles)
	bOpenFilesCount := bits.OnesCount64(bKingFiles & openFiles)

	if wSemiOpenFilesCount > 0 {
		score -= (wSemiOpenFilesCount / 8) * KingSemiOpenFilePenalty
	}
	if wOpenFilesCount > 0 {
		score -= (wOpenFilesCount / 8) * KingOpenFilePenalty
	}
	if bSemiOpenFilesCount > 0 {
		score += (bSemiOpenFilesCount / 8) * KingSemiOpenFilePenalty
	}
	if bOpenFilesCount > 0 {
		score += (bOpenFilesCount / 8) * KingOpenFilePenalty
	}

	return score
}

func kingAttackCountPenalty(attackUnitCount *[2]int) (kingAttacksPenaltyMG int, kingATtacksPenaltyEG int) {

	if attackUnitCount[0] > 99 {
		attackUnitCount[0] = 99
	}
	if attackUnitCount[1] > 99 {
		attackUnitCount[1] = 99
	}

	return (KingSafetyTable[attackUnitCount[0]]) - (KingSafetyTable[attackUnitCount[1]]), (KingSafetyTable[attackUnitCount[0]] / 4) - (KingSafetyTable[attackUnitCount[1]/4])
}

func kingEndGameCentralizationPenalty(b *dragontoothmg.Board) (kingCmdEG int) {
	return (centerManhattanDistance[bits.TrailingZeros64(b.Black.Kings)] * 10) - (centerManhattanDistance[bits.TrailingZeros64(b.White.Kings)] * 10)
}

func Evaluation(b *dragontoothmg.Board, debug bool, isQuiescence bool) (score int) {
	// UPDATE & INIT VARIABLES FOR EVAL
	// Get pawn attack bitboards
	var wPawnAttackBBEast, wPawnAttackBBWest = PawnCaptureBitboards(b.White.Pawns, true)
	var bPawnAttackBBEast, bPawnAttackBBWest = PawnCaptureBitboards(b.Black.Pawns, false)

	var wPawnAttackSpan = calculatePawnFileFill((wPawnAttackBBEast|wPawnAttackBBWest), true) & ranksBelow[4]
	var bPawnAttackSpan = calculatePawnFileFill((bPawnAttackBBEast|bPawnAttackBBWest), false) & ranksAbove[4]

	var pawnFillWhite = calculatePawnFileFill(b.White.Pawns, true)
	var pawnFillBlack = calculatePawnFileFill(b.Black.Pawns, false)

	var wSemiOpenFiles = pawnFillBlack &^ pawnFillWhite
	var bSemiOpenFiles = pawnFillWhite &^ pawnFillBlack

	var openFiles = ^pawnFillWhite & ^pawnFillBlack

	var wPawnAttackBB = wPawnAttackBBEast | wPawnAttackBBWest
	var bPawnAttackBB = bPawnAttackBBEast | bPawnAttackBBWest

	// Prepare weak square arrays
	var knightMovementBB = [2]uint64{}
	var bishopMovementBB = [2]uint64{}
	var rookMovementBB = [2]uint64{}
	var queenMovementBB = [2]uint64{}
	var kingMovementBB = [2]uint64{}

	// Get outpost bitboards
	var outposts = getOutpostsBB(b)
	whiteOutposts = outposts[0]
	blackOutposts = outposts[1]

	// Get game phase
	var piecePhase = GetPiecePhase(b)
	var currPhase = TotalPhase - piecePhase

	var pawnMG, pawnEG int
	var knightMG, knightEG int
	var bishopMG, bishopEG int
	var rookMG, rookEG int
	var queenMG, queenEG int
	var kingMG, kingEG int

	var psqtMG, psqtEG int

	var wMaterialMG, wMaterialEG = countMaterial(&b.White)
	var bMaterialMG, bMaterialEG = countMaterial(&b.Black)

	// For king safety ...
	var attackUnitCounts = [2]int{
		0: 0,
		1: 0,
	}

	var innerKingSafetyZones = getKingSafetyTable(b, true, wPawnAttackBB, bPawnAttackBB)
	var outerKingSafetyZones = getKingSafetyTable(b, false, 0, 0)

	if debug {
		println("################### FEN ###################")
		println("FEN: ", b.ToFen())
		println("################### HELPER VARIABLES ###################")
		println("Pawn attacks: ", wPawnAttackBB, " <||> ", bPawnAttackBB)
		println("Pawn attack spans: ", wPawnAttackSpan, " <||> ", bPawnAttackSpan)
		println("Pawn attacks: ", wPawnAttackBB, " <||> ", bPawnAttackBB)
		println("Open files: ", openFiles)
		println("Semi-Open files: ", wSemiOpenFiles, " <||> ", bSemiOpenFiles)
		println("Outposts: ", outposts[0], " <||> ", outposts[1])
		println("King safety tables inner: ", innerKingSafetyZones[0], " <||> ", innerKingSafetyZones[1])
		println("King safety tables outer: ", outerKingSafetyZones[0], " <||> ", outerKingSafetyZones[1])
		println("################### TACTICAL PIECE VALUES ###################")
	}

	wPieceCount := bits.OnesCount64(b.White.Bishops | b.White.Knights | b.White.Rooks | b.White.Queens)
	bPieceCount := bits.OnesCount64(b.Black.Bishops | b.Black.Knights | b.Black.Rooks | b.Black.Queens)

	/*
		Variable explanation:
		General:
			- Material explains itself
			- PSQT is a "generalized" positional array, to say what's "generally good and bad" squares for our pieces
			- Mobility means squares a piece (that's not a pawn) can move to, that's not attacked by a pawn
			- Outpost means a square where no pawns can attack or push to attack, that's defended by a friendly pawn
				- Only relevant for bishops and knights
			- Attacking Unit Counts is gathered from each piece, which attacks a square in the proximity of the opponent king
			- Weak squares is any square attacked by enemy knight, bishop or rook and not defended by knight, bishop or rook

		Pawns:
			- Isolated pawns means a pawn that has no opponent pawns that can attack it, nor block its path
			- Doubled pawns means two (or more) pawns on the same file, and no friendly pawns next to it nor on its file
			- Passed pawn means a pawn that has no opponent pawns that can attack nor block its path
			- Connected pawns means a pawn defending another
			- Phalanx pawns means a pawn that has a pawn next to it
				- The idea of Phalanx & Connected is to try and prevent passed pawns, and also open up more space on the board

		Knights:
			- Knight threat means, when the knight has the possibility to attack a opponents queen or rook

		Bishops:
			- Bishop pair means, when you have two bishops and your opponent doesn't - giving a slight general positional edge
			- Bishop attacks means the bishop attacking any opponent rook or queen - includes xrays through own/opponent pieces

		Rooks:
			- Semiopen file means a file that's blocked only by an opponent pawn
			- Open file means a file that's not blocked by any pawns
			- Seventh rank bonus is a bonus for being on the seventh rank ("becoming a pig"), which is good in the endgame
			- Rook attacks means the rook attacking any opponent rook or queen - includes xrays through own/opponent pieces

		Queens:
			- Centralized Queen means that the queen control key squares (central squares ...) in the endgame
			- Queen infiltration means that the queen is on the opponents side of the board, and can't be pushed away by enemy pawns
		King:
			- King attack penalty means a penalty for the amount of enemy pieces attacking squares around the king
			- King Pawn Shield Penalty means a penalty for having open or semi-open files next to the king
			- Central manhattan distance is the distance from the center for the king
			- King distance penalty is, in a totally winning endgame, how for the opponent king is (to win ex. KR v K endgames)
			- King minor piece defense is knights or bishops defending squares around the king
			- King pawn defense is how many pawns are close to the king in the midgame
	*/
	for _, piece := range pieceList {
		switch piece {
		case dragontoothmg.Pawn:
			pawnPsqtMG, pawnPsqtEG := countPieceTables(&b.White.Pawns, &b.Black.Pawns, &PSQT_MG[dragontoothmg.Pawn], &PSQT_EG[dragontoothmg.Pawn])
			isolatedMG, isolatedEG := isolatedPawnPenalty(b)
			doubledMG, doubledEG := pawnDoublingPenalties(b)
			connectedMG, connectedEG, phalanxMG, phalanxEG := connectedOrPhalanxPawnBonus(b)
			passedMG, passedEG := passedPawnBonus(b, wPawnAttackBB, bPawnAttackBB)
			blockedPawnBonusMG, blockedPawnBonusEG := blockedPawnBonus(b)

			// Transition from more complex pawn structures to just prioritizing passers as endgame nears...
			// Not sure if it's good, but it's something?
			pawnMG += pawnPsqtMG + passedMG + doubledMG + isolatedMG + connectedMG + phalanxMG
			pawnEG += pawnPsqtEG + passedEG + doubledEG + isolatedEG + connectedEG + phalanxEG
			if debug {
				println("Pawn MG:\t", "PSQT: ", pawnPsqtMG, "\tIsolated: ", isolatedMG, "\tDoubled:", doubledMG, "\tPassed: ", passedMG, "\tConnected: ", connectedMG, "\tPhalanx: ", phalanxMG, "\tBlocked: ", blockedPawnBonusMG)
				println("Pawn EG:\t", "PSQT: ", pawnPsqtEG, "\tIsolated: ", isolatedEG, "\tDoubled:", doubledEG, "\tPassed: ", passedEG, "\tConnected: ", connectedEG, "\tPhalanx: ", phalanxEG, "\tBlocked: ", blockedPawnBonusEG)
			}
		case dragontoothmg.Knight:
			knightPsqtMG, knightPsqtEG := countPieceTables(&b.White.Knights, &b.Black.Knights, &PSQT_MG[dragontoothmg.Knight], &PSQT_EG[dragontoothmg.Knight])
			var knightMobilityMG, knightMobilityEG int
			for x := b.White.Knights; x != 0; x &= x - 1 {
				square := bits.TrailingZeros64(x)
				movementBB := (KnightMasks[square] &^ b.White.All) &^ bPawnAttackBB
				knightMovementBB[0] |= movementBB
				knightMobilityMG += bits.OnesCount64(movementBB) * mobilityValueMG[dragontoothmg.Knight]
				knightMobilityEG += bits.OnesCount64(movementBB) * mobilityValueEG[dragontoothmg.Knight]
				attackUnitCounts[0] += (bits.OnesCount64(movementBB&innerKingSafetyZones[1]) * attackerInner[dragontoothmg.Knight])
				attackUnitCounts[0] += (bits.OnesCount64(movementBB&outerKingSafetyZones[1]) * attackerOuter[dragontoothmg.Knight])
			}
			for x := b.Black.Knights; x != 0; x &= x - 1 {
				square := bits.TrailingZeros64(x)
				movementBB := (KnightMasks[square] &^ b.Black.All) &^ wPawnAttackBB
				knightMovementBB[1] |= movementBB
				knightMobilityMG -= bits.OnesCount64(movementBB) * mobilityValueMG[dragontoothmg.Knight]
				knightMobilityEG -= bits.OnesCount64(movementBB) * mobilityValueEG[dragontoothmg.Knight]
				attackUnitCounts[1] += (bits.OnesCount64(movementBB&innerKingSafetyZones[0]) * attackerInner[dragontoothmg.Knight])
				attackUnitCounts[1] += (bits.OnesCount64(movementBB&outerKingSafetyZones[0]) * attackerOuter[dragontoothmg.Knight])
			}
			var knightOutpostMG = (KnightOutpostMG * bits.OnesCount64(b.White.Knights&whiteOutposts)) - (KnightOutpostMG * bits.OnesCount64(b.Black.Knights&blackOutposts))
			var knightOutpostEG = (KnightOutpostEG * bits.OnesCount64(b.White.Knights&whiteOutposts)) - (KnightOutpostEG * bits.OnesCount64(b.Black.Knights&blackOutposts))
			var knightThreatsBonusMG, knightThreatsBonusEG = knightThreats(b)
			knightMG += knightPsqtMG + knightOutpostMG + knightMobilityMG + knightThreatsBonusMG
			knightEG += knightPsqtEG + knightOutpostEG + knightMobilityEG // + knightThreatsBonusEG
			if debug {
				println("Knight MG:\t", "PSQT: ", knightPsqtMG, "\tMobility: ", knightMobilityMG, "\tOutpost:", knightOutpostMG, "\tKnight threats: ", knightThreatsBonusMG)
				println("Knight EG:\t", "PSQT: ", knightPsqtEG, "\tMobility: ", knightMobilityEG, "\tOutpost:", knightOutpostEG, "\tKnight threats: ", knightThreatsBonusEG)
			}
		case dragontoothmg.Bishop:
			bishopPsqtMG, bishopPsqtEG := countPieceTables(&b.White.Bishops, &b.Black.Bishops, &PSQT_MG[dragontoothmg.Bishop], &PSQT_EG[dragontoothmg.Bishop])

			var bishopMobilityMG, bishopMobilityEG int
			for x := b.White.Bishops; x != 0; x &= x - 1 {
				square := bits.TrailingZeros64(x)
				movementBB := (dragontoothmg.CalculateBishopMoveBitboard(uint8(square), (b.White.All|b.Black.All)) & ^b.White.All) &^ bPawnAttackBB
				bishopMovementBB[0] |= movementBB
				bishopMobilityMG += bits.OnesCount64(movementBB) * mobilityValueMG[dragontoothmg.Bishop]
				bishopMobilityEG += bits.OnesCount64(movementBB) * mobilityValueEG[dragontoothmg.Bishop]
				attackUnitCounts[0] += (bits.OnesCount64(movementBB&innerKingSafetyZones[1]) * attackerInner[dragontoothmg.Bishop])
				attackUnitCounts[0] += (bits.OnesCount64(movementBB&outerKingSafetyZones[1]) * attackerOuter[dragontoothmg.Bishop])
			}
			for x := b.Black.Bishops; x != 0; x &= x - 1 {
				square := bits.TrailingZeros64(x)
				movementBB := (dragontoothmg.CalculateBishopMoveBitboard(uint8(square), (b.White.All|b.Black.All)) & ^b.Black.All) &^ wPawnAttackBB
				bishopMovementBB[1] |= movementBB
				bishopMobilityMG -= bits.OnesCount64(movementBB) * mobilityValueMG[dragontoothmg.Bishop]
				bishopMobilityEG -= bits.OnesCount64(movementBB) * mobilityValueEG[dragontoothmg.Bishop]
				attackUnitCounts[1] += (bits.OnesCount64(movementBB&innerKingSafetyZones[0]) * attackerInner[dragontoothmg.Bishop])
				attackUnitCounts[1] += (bits.OnesCount64(movementBB&outerKingSafetyZones[0]) * attackerOuter[dragontoothmg.Bishop])
			}
			var bishopOutpostMG = (BishopOutpostMG * bits.OnesCount64(b.White.Bishops&whiteOutposts)) - (BishopOutpostMG * bits.OnesCount64(b.Black.Bishops&blackOutposts))
			var bishopPairMG, bishopPairEG = bishopPairBonuses(b)
			var bishopXrayAttackMG = bishopXrayAttacks(b)

			bishopMG += bishopPsqtMG + bishopMobilityMG + bishopPairMG + bishopOutpostMG
			bishopEG += bishopPsqtEG + bishopMobilityEG + bishopPairEG
			if debug {
				println("Bishop MG:\t", "PSQT: ", bishopPsqtMG, "\tMobility: ", bishopMobilityMG, "\tOutpost:", bishopOutpostMG, "\tPair: ", bishopPairMG, "\tBishop attacks: ", bishopXrayAttackMG)
				println("Bishop EG:\t", "PSQT: ", bishopPsqtEG, "\tMobility: ", bishopMobilityEG, "\t\t\tPair: ", bishopPairEG)
			}
		case dragontoothmg.Rook:
			var rookPsqtMG, rookPsqtEG = countPieceTables(&b.White.Rooks, &b.Black.Rooks, &PSQT_MG[dragontoothmg.Rook], &PSQT_EG[dragontoothmg.Rook])
			var rookSemiOpenMG, rookOpenMG = rookFilesBonus(b, openFiles, wSemiOpenFiles, bSemiOpenFiles)
			var rookSeventhRankBonus = (bits.OnesCount64(b.White.Rooks&seventhRankMask) * SeventhRankBonusEG) - (bits.OnesCount64(b.Black.Rooks&secondRankMask) * SeventhRankBonusEG)
			var rookMobilityMG int
			for x := b.White.Rooks; x != 0; x &= x - 1 {
				square := bits.TrailingZeros64(x)
				movementBB := (dragontoothmg.CalculateRookMoveBitboard(uint8(square), (b.White.All|b.Black.All)) & ^b.White.All) &^ bPawnAttackBB
				rookMovementBB[0] |= movementBB
				rookMobilityMG += bits.OnesCount64(movementBB) * mobilityValueMG[dragontoothmg.Rook]
				attackUnitCounts[0] += (bits.OnesCount64(movementBB&innerKingSafetyZones[1]) * attackerInner[dragontoothmg.Rook])
				attackUnitCounts[0] += (bits.OnesCount64(movementBB&outerKingSafetyZones[1]) * attackerOuter[dragontoothmg.Rook])
			}
			for x := b.Black.Rooks; x != 0; x &= x - 1 {
				square := bits.TrailingZeros64(x)
				movementBB := (dragontoothmg.CalculateRookMoveBitboard(uint8(square), (b.White.All|b.Black.All)) & ^b.Black.All) &^ wPawnAttackBB
				rookMovementBB[1] |= movementBB
				rookMobilityMG -= bits.OnesCount64(movementBB) * mobilityValueMG[dragontoothmg.Rook]
				attackUnitCounts[1] += (bits.OnesCount64(movementBB&innerKingSafetyZones[0]) * attackerInner[dragontoothmg.Rook])
				attackUnitCounts[1] += (bits.OnesCount64(movementBB&outerKingSafetyZones[0]) * attackerOuter[dragontoothmg.Rook])
			}

			rookXrayAttack := rookAttacks(b)
			rookMG += rookPsqtMG + rookMobilityMG + rookOpenMG + rookSemiOpenMG + rookXrayAttack
			rookEG += rookPsqtEG + rookSeventhRankBonus
			if debug {
				println("Rook MG:\t", "PSQT: ", rookPsqtMG, "\tMobility: ", rookMobilityMG, "\tOpen: ", rookOpenMG, "\tSemiOpen: ", rookSemiOpenMG, "\tRook Xray: ", rookXrayAttack)
				println("Rook EG:\t", "PSQT: ", rookPsqtEG, "\tSeventh: ", rookSeventhRankBonus)
			}
		case dragontoothmg.Queen:
			queenPsqtMG, queenPsqtEG := countPieceTables(&b.White.Queens, &b.Black.Queens, &PSQT_MG[dragontoothmg.Queen], &PSQT_EG[dragontoothmg.Queen])
			var queenMobilityMG, queenMobilityEG int
			for x := b.White.Queens; x != 0; x &= x - 1 {
				square := bits.TrailingZeros64(x)
				movementBB := dragontoothmg.CalculateBishopMoveBitboard(uint8(square), (b.White.All|b.Black.All)) & ^b.White.All
				movementBB = (movementBB | ((dragontoothmg.CalculateRookMoveBitboard(uint8(square), (b.White.All | b.Black.All))) & ^b.White.All)) &^ bPawnAttackBB
				queenMovementBB[0] |= movementBB
				queenMobilityMG += bits.OnesCount64(movementBB) * mobilityValueMG[dragontoothmg.Queen]
				queenMobilityEG += bits.OnesCount64(movementBB) * mobilityValueEG[dragontoothmg.Queen]
				attackUnitCounts[0] += (bits.OnesCount64(movementBB&innerKingSafetyZones[1]) * attackerInner[dragontoothmg.Queen])
				attackUnitCounts[0] += (bits.OnesCount64(movementBB&outerKingSafetyZones[1]) * attackerOuter[dragontoothmg.Queen])
			}
			for x := b.Black.Queens; x != 0; x &= x - 1 {
				square := bits.TrailingZeros64(x)
				movementBB := dragontoothmg.CalculateBishopMoveBitboard(uint8(square), (b.White.All|b.Black.All)) & ^b.Black.All
				movementBB = (movementBB | ((dragontoothmg.CalculateRookMoveBitboard(uint8(square), (b.White.All | b.Black.All))) & ^b.Black.All)) &^ wPawnAttackBB
				queenMovementBB[1] |= movementBB
				queenMobilityMG -= bits.OnesCount64(movementBB) * mobilityValueMG[dragontoothmg.Queen]
				queenMobilityEG -= bits.OnesCount64(movementBB) * mobilityValueEG[dragontoothmg.Queen]
				attackUnitCounts[1] += (bits.OnesCount64(movementBB&innerKingSafetyZones[0]) * attackerInner[dragontoothmg.Queen])
				attackUnitCounts[1] += (bits.OnesCount64(movementBB&outerKingSafetyZones[0]) * attackerOuter[dragontoothmg.Queen])
			}

			var centralizedQueenBonus = centralizedQueen(b)
			var queenInfiltrationBonusMG, queenInfiltrationBonusEG = queenInfiltrationBonus(b, wPawnAttackSpan, bPawnAttackSpan)

			queenMG += queenPsqtMG + queenMobilityMG + queenInfiltrationBonusMG
			queenEG += queenPsqtEG + queenMobilityEG + centralizedQueenBonus + queenInfiltrationBonusEG

			if debug {
				println("Queen MG:\t", "PSQT: ", queenPsqtMG, "\tMobility: ", queenMobilityMG, "\tInfiltration: ", queenInfiltrationBonusMG)
				println("Queen EG:\t", "PSQT: ", queenPsqtEG, "\tMobility: ", queenMobilityEG, "\tInfiltration: ", queenInfiltrationBonusEG, "\tCentralized Queen bonus", centralizedQueenBonus)
			}
		case dragontoothmg.King:
			kingPsqtMG, kingPsqtEG := countPieceTables(&b.White.Kings, &b.Black.Kings, &PSQT_MG[dragontoothmg.King], &PSQT_EG[dragontoothmg.King])
			kingAttackPenaltyMG, kingAttackPenaltyEG := kingAttackCountPenalty(&attackUnitCounts)
			kingPawnShieldPenaltyMG := kingFilesPenalty(b, openFiles, wSemiOpenFiles, bSemiOpenFiles)
			kingCentralManhattanPenalty := 0
			kingDistancePenalty := 0
			KingMinorPieceDefenseBonusMG := kingMinorPieceDefences(innerKingSafetyZones, knightMovementBB, bishopMovementBB)
			kingPawnDefenseMG := kingPawnDefense(b, innerKingSafetyZones)

			kingMovementBB[0] = (innerKingSafetyZones[0] &^ b.White.All) &^ (knightMovementBB[1] | bishopMovementBB[1] | rookMovementBB[1] | queenMovementBB[1])
			kingMovementBB[1] = (innerKingSafetyZones[1] &^ b.Black.All) &^ (knightMovementBB[0] | bishopMovementBB[0] | rookMovementBB[0] | queenMovementBB[0])
			/*
				If we're below a certain count of pieces (excluding pawns), we try to centralize our king
				We're more likely to centralize queens are traded off
				If our opponent has no pieces left, we try to follow the enemy king to find a faster mating sequence
			*/
			if (piecePhase < 16 && bits.OnesCount64(b.White.Queens|b.Black.Queens) == 0) || piecePhase < 10 { // Moving closer to endgame, try to centralize king somewhat for activity
				/*
					Let's figure out if we're in a specific endgame; either for draw or winning position ...
					King v King+Bishop+Knight requires us to know the color complex
				*/

				kingCentralManhattanPenalty = kingEndGameCentralizationPenalty(b)

				if wPieceCount > 0 && bPieceCount == 0 {
					kingDistancePenalty = getkingEndGamePositionValue(b, true)
				} else if wPieceCount == 0 && bPieceCount > 0 {
					kingDistancePenalty = getkingEndGamePositionValue(b, false)
				}
			}

			kingMG += kingPsqtMG + kingAttackPenaltyMG + kingPawnShieldPenaltyMG + KingMinorPieceDefenseBonusMG + kingPawnDefenseMG
			kingEG += kingPsqtEG + kingAttackPenaltyEG + kingDistancePenalty + kingCentralManhattanPenalty
			if debug {
				println("King MG:\t", "PSQT: ", kingPsqtMG, "\tAttack: ", kingAttackPenaltyMG, "\tFile: ", kingPawnShieldPenaltyMG, "\tKing pawn defense: ", kingPawnDefenseMG, "\tMinor defense: ", KingMinorPieceDefenseBonusMG)
				println("King EG:\t", "PSQT: ", kingPsqtEG, "\tAttack: ", kingAttackPenaltyEG, "\tCentralization: ", kingCentralManhattanPenalty, "\tDistance penalty: ", kingDistancePenalty)
			}
		}
	}

	/*
		Weak square control - based on how well squares in ones own ""zone"" is defended
		Squares attacked by opponent pieces, that are undefended or only defended by king/queen is ""weak""
		Idea is to prioritize space control; to manage what squares are important to defend, change the bitmask in the getWeakSquares function
	*/
	var movementBB [2][5]uint64 = [2][5]uint64{
		{
			knightMovementBB[0], bishopMovementBB[0], rookMovementBB[0], queenMovementBB[0], kingMovementBB[0],
		},
		{
			knightMovementBB[1], bishopMovementBB[1], rookMovementBB[1], queenMovementBB[1], kingMovementBB[1],
		},
	}
	var weakSquares, weakKingSquares = getWeakSquares(movementBB, innerKingSafetyZones)

	//var weakSquaresPenalty = 2
	//var weakKingSquaresPenalty = 5

	var wWeakSquarePenalty = (bits.OnesCount64(weakSquares[0]&^weakKingSquares[0]) * weakSquaresPenalty) * -1
	var bWeakSquarePenalty = (bits.OnesCount64(weakSquares[1]&^weakKingSquares[1]) * weakSquaresPenalty)

	var wWeakKingSquarePenalty = (bits.OnesCount64(weakKingSquares[0]) * weakKingSquaresPenalty) * -1
	var bWeakKingSquarePenalty = (bits.OnesCount64(weakKingSquares[1]) * weakKingSquaresPenalty)

	var weakSquareMG int = wWeakSquarePenalty + wWeakKingSquarePenalty + bWeakSquarePenalty + bWeakKingSquarePenalty

	/* Calculate score from all variables */
	var materialScoreMG = (wMaterialMG - bMaterialMG)
	var materialScoreEG = (wMaterialEG - bMaterialEG)

	// Tempo bonus for side to move
	var toMoveBonus = 10
	if !b.Wtomove {
		toMoveBonus = -10
	}

	var variableScoreMG = pawnMG + knightMG + bishopMG + rookMG + queenMG + kingMG + weakSquareMG + toMoveBonus
	var variableScoreEG = pawnEG + knightEG + bishopEG + rookEG + queenEG + kingEG + toMoveBonus

	var mgScore = variableScoreMG + materialScoreMG
	var egScore = variableScoreEG + materialScoreEG

	var mgPhase = 1 - float64(currPhase)/24.0
	var egPhase = float64(currPhase) / 24.0
	score = int((float64(mgScore) * mgPhase) + (float64(egScore) * egPhase))

	if debug {
		println("################### MOBILITY ###################")
		println("Knights: ", movementBB[0][0], " : ", movementBB[1][0])
		println("Bishops: ", movementBB[0][1], " : ", movementBB[1][1])
		println("Rooks: ", movementBB[0][2], " : ", movementBB[1][2])
		println("Queens: ", movementBB[0][3], " : ", movementBB[1][3])
		println("################### START PHASE ###################")
		println("Piece phase: \t\t", piecePhase)
		fmt.Printf("Midgame phase: %.2f\n", mgPhase)
		println("Total phase: \t\t", TotalPhase)
		println("Reduced phase: \t\t", (currPhase*256+12)/TotalPhase)
		println("Weak squares: ", weakSquares[0], " : ", weakSquares[1])
		println("Weak king squares: ", weakKingSquares[0], " : ", weakKingSquares[1])
	}

	if isTheoreticalDraw(b, debug) {
		score = score / DrawDivider
	}

	if debug {
		println("################### MIDGAME_EVAL : ENDGAME_EVAL  ###################")
		println("PSQT eval: \t\t\t", psqtMG, ":", psqtEG)
		println("Weak Squares eval: \t\t", weakSquareMG, ":")
		println("Pawn eval: \t\t\t", pawnMG, ":", pawnEG)
		println("Knight eval: \t\t\t", knightMG, ":", knightEG)
		println("Bishop eval: \t\t\t", bishopMG, ":", bishopEG)
		println("Rook eval: \t\t\t", rookMG, ":", rookEG)
		println("Queen eval: \t\t\t", queenMG, ":", queenEG)
		println("King eval: \t\t\t", kingMG, ":", kingEG)
		println("Non-material eval: \t\t", variableScoreMG, ":", variableScoreEG)
		println("Material eval: \t\t\t", materialScoreMG, ":", materialScoreEG)
		println("White attacking unit count: \t", attackUnitCounts[0])
		println("Black attacking unit count: \t", attackUnitCounts[1])
		println("Phase eval: \t\t\t", mgScore, " : ", egScore)
		println("Total score: \t\t\t", score)
	}

	if isQuiescence && b.Halfmoveclock > 8 {
		println("Quiescence eval: ", score, " ---- FEN: ", b.ToFen())
	}

	if !b.Wtomove {
		score = -score
	}

	return score
}
