package engine

import (
	"fmt"
	"math/bits"

	"github.com/dylhunn/dragontoothmg"
	"golang.org/x/exp/constraints"
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
var whiteOutposts uint64
var blackOutposts uint64

var wAllowedOutpostMask uint64 = 0xffff7e7e000000
var bAllowedOutpostMask uint64 = 0x7e7effff00

var seventhRankMask uint64 = 0xff000000000000
var secondRankMask uint64 = 0xff00

var blackSquaresBB uint64 = 0xaa55aa55aa55aa55

var wSide uint64 = 0x3c7e7e3c00     //0x3c3c3c0000   //0x7e7e3c00       //0xffffffff //(full half side)
var bSide uint64 = 0x3c7e7e3c000000 //0x3c3c3c000000 //0x3c7e7e00000000 //0xffffffff00000000 (full half side)

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

// 84, 333, 346, 441, 921
// 106, 244, 268, 478, 886
var PawnValueMG = 82
var PawnValueEG = 94
var KnightValueMG = 337
var KnightValueEG = 281
var BishopValueMG = 365
var BishopValueEG = 297
var RookValueMG = 477
var RookValueEG = 512
var QueenValueMG = 1025
var QueenValueEG = 936

var weakSquaresPenalty = 4

var PieceValueMG = [7]int{dragontoothmg.King: 0, dragontoothmg.Pawn: 95, dragontoothmg.Knight: 337, dragontoothmg.Bishop: 365, dragontoothmg.Rook: 477, dragontoothmg.Queen: 1025} // Original estimation
var PieceValueEG = [7]int{dragontoothmg.King: 0, dragontoothmg.Pawn: 110, dragontoothmg.Knight: 281, dragontoothmg.Bishop: 297, dragontoothmg.Rook: 512, dragontoothmg.Queen: 936} // Original estimation

var MobilityValueMG = [7]int{dragontoothmg.Pawn: 0, dragontoothmg.Knight: 5, dragontoothmg.Bishop: 3, dragontoothmg.Rook: 3, dragontoothmg.Queen: 2, dragontoothmg.King: 0}
var MobilityValueEG = [7]int{dragontoothmg.Pawn: 0, dragontoothmg.Knight: 1, dragontoothmg.Bishop: 2, dragontoothmg.Rook: 3, dragontoothmg.Queen: 4, dragontoothmg.King: 0}

var attackerInner = [7]int{dragontoothmg.Pawn: 1, dragontoothmg.Knight: 2, dragontoothmg.Bishop: 2, dragontoothmg.Rook: 4, dragontoothmg.Queen: 5, dragontoothmg.King: 0}
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
var DoubledPawnPenaltyEG = 20
var IsolatedPawnMG = 7
var IsolatedPawnEG = 15
var ConnectedPawnsBonusMG = 2
var ConnectedPawnsBonusEG = 3
var PhalanxPawnsBonusMG = 1
var PhalanxPawnsBonusEG = 2
var BlockedPawnBonusMG = 25
var BlockedPawnBonusEG = 15

/* Knight variables */
var KnightOutpostMG = 25
var KnightOutpostEG = 20
var KnightCanAttackPieceMG = 3
var KnightCanAttackPieceEG = 1

/* Bishop variables */
var BishopOutpostMG = 15
var BishopOutpostEG = 5
var BishopPairBonusMG = 20
var BishopPairBonusEG = 30
var BishopPawnSetupPenaltyMG = 5
var BishopPawnSetupPenaltyEG = 2
var BishopXrayRookMG = 8
var BishopXrayQueenMG = 15
var BishopXrayKingMG = 5

/* Rook variables */
var RookXrayAttacksMG = 12
var ConnectedRooksBonusMG = 20
var ConnectedRooksBonusEG = 5
var RookSemiOpenFileBonusMG = 10
var RookOpenFileBonusMG = 15
var RookStackedSemiOpenFileBonusMG = 10
var RookStackedOpenFileBonusMG = 45
var SeventhRankBonusEG = 20

/* Queen variables ... Pretty empty :'( */
var centralizedQueenSquares uint64 = 0x183c3c180000
var CentralizedQueenBonusEG = 30
var QueenInfiltrationBonusMG = -2
var QueenInfiltrationBonusEG = 15

/* King variables */
var KingPawnDistancePenalty = 3
var KingSemiOpenFilePenalty = 7
var KingOpenFilePenalty = 5
var KingMinorPieceDefenseBonus = 3
var KingPawnDefenseMG = 10

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
		-26, -4, -20, -10, 3, 3, 33, -12,
		-27, -2, -5, 13, 17, 6, 10, -25,
		-14, 13, 6, 21, 23, 12, 17, -23,
		-6, 7, 26, 31, 65, 56, 25, -20,
		98, 134, 61, 95, 68, 126, 34, -11,
		0, 0, 0, 0, 0, 0, 0, 0,
	},
	dragontoothmg.Knight: {
		-43, -11, -8, -5, 1, -20, -4, -22,
		-31, -22, 19, 7, 5, 13, -8, -11,
		-21, 21, 8, 16, 36, 33, 19, 6,
		-6, 2, 0, 23, 8, 27, 4, 14,
		-3, 10, 12, 8, 16, 10, 19, 1,
		-19, -4, 3, 7, 22, 12, 15, -11,
		-21, -20, -9, 8, 9, 11, -5, 0,
		-19, -13, -20, -14, -2, 3, -11, -8,
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
		-10, 0, 0, 0, 10, 9, 5, 7,
		-19, -35, -5, 2, -9, 7, 1, 15,
		-10, -7, -4, -9, 15, 29, 24, 22,
		-14, -14, -15, -11, -1, -5, 3, -6,
		-8, -20, -8, -5, -4, -2, 2, -2,
		-13, 5, 2, 1, -1, 8, 4, 2,
		-20, 0, 10, 16, 16, 16, -6, 6,
		-3, -1, 7, 19, 5, -10, -9, -17,
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
		-36, -16, -7, -14, -4, -20, -20, -29,
		-17, 2, -7, 14, 2, -7, -9, -19,
		-13, -7, 14, 12, 4, 6, 0, -13,
		-5, 8, 24, 18, 22, 15, 11, -4,
		-3, 4, 20, 30, 22, 25, 15, -2,
		-7, 1, 3, 19, 10, -2, -4, -4,
		-10, -2, -1, 0, 6, -8, -3, -13,
		-12, -28, -8, 1, -5, -12, -27, -12,
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
		-12, 4, 8, 4, 10, 9, 3, 6,
		-17, -7, -1, 7, 3, 6, 1, 0,
		-5, -1, -4, 12, 14, 20, 12, 14,
		-2, 2, 2, 9, 13, 7, 18, 22,
		-9, 3, 1, 15, 5, 10, 12, 10,
		-6, -20, 0, -15, 0, -1, 10, 7,
		-6, -14, -31, -27, -19, -12, -11, -4,
		-12, -22, -19, -30, -8, -13, -6, -15,
	},
	dragontoothmg.King: {
		-43, -34, -20, -5, -26, -9, -35, -55,
		-27, -10, 2, 9, 9, 1, -12, -26,
		-21, -6, 5, 13, 15, 9, -2, -12,
		-23, -6, 14, 21, 20, 18, 5, -16,
		-12, 14, 21, 25, 19, 25, 18, -5,
		-1, 18, 19, 15, 16, 35, 34, 4,
		-9, 14, 11, 13, 13, 28, 19, 1,
		-15, -11, -11, -6, -2, 3, 4, -9,
	},
}

// Taken from dragontooth chess engine!
var isolatedPawnTable = [8]uint64{
	0x303030303030303, 0x707070707070707, 0xe0e0e0e0e0e0e0e, 0x1c1c1c1c1c1c1c1c,
	0x3838383838383838, 0x7070707070707070, 0xe0e0e0e0e0e0e0e0, 0xc0c0c0c0c0c0c0c0,
}
var actualIsolatedPawnTable = [8]uint64{
	0x202020202020202, 0x505050505050505, 0xa0a0a0a0a0a0a0a, 0x1414141414141414,
	0x2828282828282828, 0x5050505050505050, 0xa0a0a0a0a0a0a0a0, 0x4040404040404040,
}

/* ============= HELPER VARIABLES ============= */
var CenterManhattanDistance = [64]int{
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

func min[T constraints.Ordered](a, b T) T {
	if a < b {
		return a
	}
	return b
}

func max[T constraints.Ordered](a, b T) T {
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
	materialMG += bits.OnesCount64(bb.Pawns) * PieceValueMG[dragontoothmg.Pawn]
	materialEG += bits.OnesCount64(bb.Pawns) * PieceValueEG[dragontoothmg.Pawn]

	materialMG += bits.OnesCount64(bb.Knights) * PieceValueMG[dragontoothmg.Knight]
	materialEG += bits.OnesCount64(bb.Knights) * PieceValueEG[dragontoothmg.Knight]

	materialMG += bits.OnesCount64(bb.Bishops) * PieceValueMG[dragontoothmg.Bishop]
	materialEG += bits.OnesCount64(bb.Bishops) * PieceValueEG[dragontoothmg.Bishop]

	materialMG += bits.OnesCount64(bb.Rooks) * PieceValueMG[dragontoothmg.Rook]
	materialEG += bits.OnesCount64(bb.Rooks) * PieceValueEG[dragontoothmg.Rook]

	materialMG += bits.OnesCount64(bb.Queens) * PieceValueMG[dragontoothmg.Queen]
	materialEG += bits.OnesCount64(bb.Queens) * PieceValueEG[dragontoothmg.Queen]

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

func getWeakSquares(movementBB [2][6]uint64, wPawnAttackBB uint64, bPawnAttackBB uint64) (weakSquares [2]uint64) {
	// Squares attacked by bishops, knights and rooks
	var wAttackers uint64 = (movementBB[0][0] | movementBB[0][1] | movementBB[0][2])
	var bAttackers uint64 = (movementBB[1][0] | movementBB[1][1] | movementBB[1][2])

	var wDefenders uint64 = movementBB[0][0] | movementBB[0][1] | wPawnAttackBB
	var bDefenders uint64 = movementBB[1][0] | movementBB[1][1] | bPawnAttackBB

	// Undefended squares attacked by opponent pieces
	var wPotentialWeakSquares uint64 = (wSide & bAttackers)
	var bPotentialWeakSquares uint64 = (bSide & wAttackers)

	// If the squares are defended by friendly pieces, they're not weak anymore!
	var wWeakSquares uint64 = wPotentialWeakSquares &^ (wDefenders | wPawnAttackBB)
	var bWeakSquares uint64 = bPotentialWeakSquares &^ (bDefenders | bPawnAttackBB)

	//fmt.Printf("Attackers: %v | %v \t Defenders: %v | %v \t Potential weak: %v | %v \t Weak squares: %v | %v \n", wAttackers, bAttackers, wDefenders, bDefenders, wPotentialWeakSquares, bPotentialWeakSquares, wWeakSquares, bWeakSquares)

	weakSquares[0] = wWeakSquares
	weakSquares[1] = bWeakSquares

	return weakSquares
}

/*
	PAWN FUNCTIONS
*/

func phalanxOrConnectedPawnBonus(wPhalanxPawnsBB uint64, bPhalanxPawnsBB uint64, wConnectedPawnsBB uint64, bConnectedPawnsBB uint64) (phalanxMG, phalanxEG, connectedMG, connectedEG int) {
	phalanxMG = (bits.OnesCount64(wPhalanxPawnsBB&^secondRankMask) * PhalanxPawnsBonusMG) - (bits.OnesCount64(bPhalanxPawnsBB&^seventhRankMask) * PhalanxPawnsBonusMG)
	phalanxEG = (bits.OnesCount64(wPhalanxPawnsBB&^secondRankMask) * PhalanxPawnsBonusEG) - (bits.OnesCount64(bPhalanxPawnsBB&^seventhRankMask) * PhalanxPawnsBonusEG)

	connectedMG = (bits.OnesCount64(wConnectedPawnsBB) * ConnectedPawnsBonusMG) - (bits.OnesCount64(bConnectedPawnsBB) * ConnectedPawnsBonusMG)
	connectedEG = (bits.OnesCount64(wConnectedPawnsBB) * ConnectedPawnsBonusEG) - (bits.OnesCount64(bConnectedPawnsBB) * ConnectedPawnsBonusEG)

	return phalanxMG, phalanxEG, connectedMG, connectedEG
}

func isolatedPawnPenalty(wIsolatedPawnsBB uint64, bIsolatedPawnsBB uint64) (isolatedMG, isolatedEG int) {
	// White
	isolatedMG -= bits.OnesCount64(wIsolatedPawnsBB) * IsolatedPawnMG
	isolatedEG -= bits.OnesCount64(wIsolatedPawnsBB) * IsolatedPawnEG

	// Black
	isolatedMG += bits.OnesCount64(bIsolatedPawnsBB) * IsolatedPawnMG
	isolatedEG += bits.OnesCount64(bIsolatedPawnsBB) * IsolatedPawnEG

	return isolatedMG, isolatedEG
}

func passedPawnBonus(wPassedPawnsBB uint64, bPassedPawnsBB uint64, wConnectedPawns uint64, bConnectedPawns uint64) (passedMG, passedEG int) {
	for x := wPassedPawnsBB; x != 0; x &= x - 1 {
		var squareBB uint64 = PositionBB[bits.TrailingZeros64(x)]
		if squareBB&wConnectedPawns > 0 {
			passedMG += PassedPawnPSQT_MG[bits.TrailingZeros64(x)] + (ConnectedPawnsBonusMG * 2)
			passedEG += PassedPawnPSQT_EG[bits.TrailingZeros64(x)] + (ConnectedPawnsBonusEG * 2)
		} else {
			passedMG += PassedPawnPSQT_MG[bits.TrailingZeros64(x)]
			passedEG += PassedPawnPSQT_EG[bits.TrailingZeros64(x)]
		}
	}

	for x := bPassedPawnsBB; x != 0; x &= x - 1 {
		revSq := FlipView[bits.TrailingZeros64(x)]
		var squareBB uint64 = PositionBB[bits.TrailingZeros64(x)]
		if squareBB&bConnectedPawns > 0 {
			passedMG -= (PassedPawnPSQT_MG[revSq])
			passedEG -= (PassedPawnPSQT_EG[revSq])
		} else {
			passedMG -= PassedPawnPSQT_MG[revSq]
			passedEG -= PassedPawnPSQT_EG[revSq]
		}
	}

	return passedMG, passedEG
}

func pawnDoublingPenalties(wDoubledPawnBB uint64, bDoubledPawnBB uint64, wIsolatedPawnsBB uint64, bIsolatedPawnsBB uint64) (doubledMG, doubledEG int) {
	if wIsolatedPawnsBB&wDoubledPawnBB > 0 {
		doubledMG -= (bits.OnesCount64(wDoubledPawnBB) / 2) * (DoubledPawnPenaltyMG + IsolatedPawnMG)
		doubledEG -= (bits.OnesCount64(wDoubledPawnBB) / 2) * (DoubledPawnPenaltyEG + IsolatedPawnEG)
	} else {
		doubledMG -= (bits.OnesCount64(wDoubledPawnBB) / 2) * DoubledPawnPenaltyMG
		doubledEG -= (bits.OnesCount64(wDoubledPawnBB) / 2) * DoubledPawnPenaltyEG
	}

	if bIsolatedPawnsBB&bDoubledPawnBB > 0 {
		doubledMG += (bits.OnesCount64(bDoubledPawnBB) / 2) * (DoubledPawnPenaltyMG + IsolatedPawnMG)
		doubledEG += (bits.OnesCount64(bDoubledPawnBB) / 2) * (DoubledPawnPenaltyEG + IsolatedPawnEG)
	} else {
		doubledMG += (bits.OnesCount64(bDoubledPawnBB) / 2) * DoubledPawnPenaltyMG
		doubledEG += (bits.OnesCount64(bDoubledPawnBB) / 2) * DoubledPawnPenaltyEG
	}
	return doubledMG, doubledEG
}

func blockedPawnBonus(wBlockedPawnsBB uint64, bBlockedPawnsBB uint64) (blockedBonusMG int, blockedBonusEG int) {
	// White
	blockedBonusMG += (bits.OnesCount64(wBlockedPawnsBB&fifthAndSixthRank) * BlockedPawnBonusMG)
	blockedBonusEG += (bits.OnesCount64(wBlockedPawnsBB&fifthAndSixthRank) * BlockedPawnBonusEG)
	// Black
	blockedBonusMG -= (bits.OnesCount64(bBlockedPawnsBB&thirdAndFourthRank) * BlockedPawnBonusMG)
	blockedBonusEG -= (bits.OnesCount64(bBlockedPawnsBB&thirdAndFourthRank) * BlockedPawnBonusEG)

	return blockedBonusMG, blockedBonusEG
}

/*
	KNIGHT FUNCTIONS
*/

func knightThreats(b *dragontoothmg.Board) (knightThreatsMG int) {
	wPieces := (b.White.Bishops | b.White.Rooks | b.White.Queens)
	bPieces := (b.Black.Bishops | b.Black.Rooks | b.Black.Queens)
	for x := b.White.Knights; x != 0; x &= x - 1 {
		knightMovement := KnightMasks[bits.TrailingZeros64(x)] &^ (wPieces | b.White.Pawns)
		// We only count one way of attacking a piece (often knight can reach the same piece via different squares)
		bTmpPieces := bPieces
		for y := knightMovement; y != 0; y &= y - 1 {
			knightThreatBB := KnightMasks[bits.TrailingZeros64(y)] &^ (wPieces | b.White.Pawns)
			if knightThreatBB&bTmpPieces > 0 {
				bTmpPieces &^= knightThreatBB
				knightThreatsMG += KnightCanAttackPieceMG
			}
		}
	}

	for x := b.Black.Knights; x != 0; x &= x - 1 {
		knightMovement := KnightMasks[bits.TrailingZeros64(x)] &^ (bPieces | b.Black.Pawns)
		// We only count one way of attacking a piece (often knight can reach the same piece via different squares)
		wTmpPieces := bPieces
		for y := knightMovement; y != 0; y &= y - 1 {
			knightThreatBB := KnightMasks[bits.TrailingZeros64(y)] &^ (bPieces | b.Black.Pawns)
			if bits.OnesCount64(knightThreatBB&wTmpPieces) > 0 {
				wTmpPieces &^= knightThreatBB
				knightThreatsMG -= KnightCanAttackPieceMG
			}
		}
	}

	return knightThreatsMG
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
		var bishopMovementBoard = dragontoothmg.CalculateBishopMoveBitboard(uint8(sq), (b.White.All|b.Black.Pawns)) & ^b.White.All // We can't xray our own pieces
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
		var bishopMovementBoard = dragontoothmg.CalculateBishopMoveBitboard(uint8(sq), (b.Black.All|b.White.Pawns)) & ^b.Black.All // We can't xray our own pieces
		if bits.OnesCount64(bishopMovementBoard&b.White.Kings) > 0 {
			bishopXrayMG -= BishopXrayKingMG
		} else if bits.OnesCount64(bishopMovementBoard&b.White.Rooks) > 0 {
			bishopXrayMG -= BishopXrayRookMG
		} else if bits.OnesCount64(bishopMovementBoard&b.White.Queens) > 0 {
			bishopXrayMG -= BishopXrayQueenMG
		}
	}

	return bishopXrayMG
}

func bishopPawnColorRatio(b *dragontoothmg.Board, wBlockedPawnsBB, bBlockedPawnsBB uint64) (bishopPawnSetupMG, bishopPawnSetupEG int) {
	// Pawn square color count
	var wPawnWhiteSquareCount int = bits.OnesCount64(wBlockedPawnsBB &^ blackSquaresBB)
	var wPawnBlackSquareCount int = bits.OnesCount64(wBlockedPawnsBB & blackSquaresBB)

	var bPawnWhiteSquareCount int = bits.OnesCount64(bBlockedPawnsBB &^ blackSquaresBB)
	var bPawnBlackSquareCount int = bits.OnesCount64(bBlockedPawnsBB & blackSquaresBB)

	for x := b.White.Bishops; x != 0; x &= x - 1 {
		var isWhiteSquare bool = (PositionBB[bits.TrailingZeros64(x)] & blackSquaresBB) == 0
		if isWhiteSquare {
			bishopPawnSetupMG -= wPawnWhiteSquareCount * BishopPawnSetupPenaltyMG
			bishopPawnSetupEG -= wPawnWhiteSquareCount * BishopPawnSetupPenaltyEG
		} else {
			bishopPawnSetupMG -= wPawnBlackSquareCount * BishopPawnSetupPenaltyMG
			bishopPawnSetupEG -= wPawnBlackSquareCount * BishopPawnSetupPenaltyEG
		}
	}

	for x := b.Black.Bishops; x != 0; x &= x - 1 {
		var isWhiteSquare bool = (PositionBB[bits.TrailingZeros64(x)] & blackSquaresBB) == 0
		if isWhiteSquare {
			bishopPawnSetupMG += bPawnWhiteSquareCount * BishopPawnSetupPenaltyMG
			bishopPawnSetupEG += bPawnWhiteSquareCount * BishopPawnSetupPenaltyEG
		} else {
			bishopPawnSetupMG += bPawnBlackSquareCount * BishopPawnSetupPenaltyMG
			bishopPawnSetupEG += bPawnBlackSquareCount * BishopPawnSetupPenaltyEG
		}
	}

	return bishopPawnSetupMG, bishopPawnSetupEG
}

/*
	ROOK FUNCTIONS
*/

func rookFilesBonus(b *dragontoothmg.Board, openFiles uint64, wSemiOpenFiles uint64, bSemiOpenFiles uint64) (semiOpen, open int) {
	wSemiOpen := bits.OnesCount64(wSemiOpenFiles & b.White.Rooks)
	bSemiOpen := bits.OnesCount64(bSemiOpenFiles & b.Black.Rooks)
	wOpen := bits.OnesCount64(openFiles & b.White.Rooks)
	bOpen := bits.OnesCount64(openFiles & b.Black.Rooks)

	if wSemiOpen > 0 || wOpen > 0 {
		if wSemiOpen >= 2 || wOpen >= 2 {
			semiOpen += RookStackedSemiOpenFileBonusMG * wSemiOpen
			open += RookStackedOpenFileBonusMG * wOpen
		} else {
			semiOpen += RookSemiOpenFileBonusMG * wSemiOpen
			open += RookOpenFileBonusMG * wOpen
		}
	}

	if bSemiOpen > 0 || bOpen > 0 {
		if bSemiOpen >= 2 || bOpen >= 2 {
			semiOpen -= RookStackedSemiOpenFileBonusMG * bSemiOpen
			open -= RookStackedOpenFileBonusMG * bOpen
		} else {
			semiOpen -= RookSemiOpenFileBonusMG * bSemiOpen
			open -= RookOpenFileBonusMG * bOpen
		}
	}

	return semiOpen, open
}

func rookAttacks(b *dragontoothmg.Board) (xrayMG int) {
	var wBlockingPieces = b.White.Pawns | b.White.Rooks | b.White.Queens | b.White.Kings
	var bBlockingPieces = b.Black.Pawns | b.Black.Rooks | b.Black.Queens | b.Black.Kings

	for x := b.White.Rooks; x != 0; x &= x - 1 {
		var sq = bits.TrailingZeros64(x)
		rookMovementBoard := dragontoothmg.CalculateRookMoveBitboard(uint8(sq), (wBlockingPieces|(b.Black.Pawns))) &^ b.White.All
		if bits.OnesCount64(rookMovementBoard&(b.Black.Queens|b.Black.Kings)) > 0 {
			xrayMG += RookXrayAttacksMG
		}
	}
	for x := b.Black.Rooks; x != 0; x &= x - 1 {
		var sq = bits.TrailingZeros64(x)
		rookMovementBoard := dragontoothmg.CalculateRookMoveBitboard(uint8(sq), (bBlockingPieces|b.White.Pawns)) &^ b.White.All
		if bits.OnesCount64(rookMovementBoard&(b.White.Queens|b.White.Kings)) > 0 {
			xrayMG -= RookXrayAttacksMG
		}
	}
	return xrayMG
}

func rookOnSecondOrSeventh(b *dragontoothmg.Board) (rookOnSeventhOrSecondMG int) {
	return (bits.OnesCount64(b.White.Rooks&seventhRankMask) * SeventhRankBonusEG) - (bits.OnesCount64(b.Black.Rooks&secondRankMask) * SeventhRankBonusEG)
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

func queenInfiltrationBonus(b *dragontoothmg.Board, wQueenInfiltrationBB uint64, bQueenInfiltrationBB uint64, knightMovementBB [2]uint64, bishopMovementBB [2]uint64, rookMovementBB [2]uint64) (queenInfiltrationBonusMG int, queenInfiltrationBonusEG int) {
	wQueenInfil := wQueenInfiltrationBB &^ (knightMovementBB[1] | bishopMovementBB[1] | rookMovementBB[1])
	bQueenInfil := bQueenInfiltrationBB &^ (knightMovementBB[0] | bishopMovementBB[0] | rookMovementBB[0])
	//fmt.Printf("Queen Infiltration: %v | %v\n", wQueenInfil, bQueenInfil)
	queenInfiltrationBonusMG = (bits.OnesCount64(b.White.Queens&wQueenInfil) * QueenInfiltrationBonusMG) - (bits.OnesCount64(b.Black.Queens&bQueenInfil) * QueenInfiltrationBonusMG)
	queenInfiltrationBonusEG = (bits.OnesCount64(b.White.Queens&wQueenInfil) * QueenInfiltrationBonusEG) - (bits.OnesCount64(b.Black.Queens&bQueenInfil) * QueenInfiltrationBonusEG)
	return queenInfiltrationBonusMG, queenInfiltrationBonusEG
}

/*
	KING FUNCTIONS
*/

func kingPawnDistance(wClosestPawnDistance, bClosestPawnDistance int) (kingPawnDistancePenaltyEG int) {
	if wClosestPawnDistance > 1 {
		kingPawnDistancePenaltyEG -= KingPawnDistancePenalty * wClosestPawnDistance
	}

	if bClosestPawnDistance > 1 {
		kingPawnDistancePenaltyEG += KingPawnDistancePenalty * bClosestPawnDistance
	}

	return kingPawnDistancePenaltyEG
}

func kingMinorPieceDefences(kingInnerRing [2]uint64, knightMovementBB [2]uint64, bishopMovementBB [2]uint64) int {
	wDefendingPiecesCount := bits.OnesCount64(kingInnerRing[0] & (knightMovementBB[0] | bishopMovementBB[0]))
	bDefendingPiecesCount := bits.OnesCount64(kingInnerRing[1] & (knightMovementBB[1] | bishopMovementBB[1]))

	return (wDefendingPiecesCount * KingMinorPieceDefenseBonus) - (bDefendingPiecesCount * KingMinorPieceDefenseBonus)
}

func kingPawnDefense(b *dragontoothmg.Board) int {
	wKingMoves := KingMoves[bits.TrailingZeros64(b.White.Kings)]
	bKingMoves := KingMoves[bits.TrailingZeros64(b.Black.Kings)]

	wPawnsCloseToKing := min(3, bits.OnesCount64(wKingMoves&b.White.Pawns)) // 3, so we don't overgrow our fortress with like 5 pawns
	bPawnsCloseToKing := min(3, bits.OnesCount64(bKingMoves&b.Black.Pawns))

	//fmt.Printf("Above | %v -- %v | Below\n", wKingMoves, bKingMoves)
	return (wPawnsCloseToKing * KingPawnDefenseMG) - (bPawnsCloseToKing * KingPawnDefenseMG)
}

//func getkingEndGamePositionValue(b *dragontoothmg.Board, whiteWithAdvantage bool) (score int) {
//	var friendlyKingFile = 0
//	var friendlyKingRank = 0
//	var enemyKingFile = 0
//	var enemyKingRank = 0
//	wKingSq := bits.TrailingZeros64(b.White.Kings)
//	bKingSq := bits.TrailingZeros64(b.Black.Kings)
//	if whiteWithAdvantage { // White
//		friendlyKingFile = wKingSq % 8
//		friendlyKingRank = wKingSq / 8
//		enemyKingFile = bKingSq % 8
//		enemyKingRank = bKingSq / 8
//	} else { // Black
//		friendlyKingFile = bKingSq % 8
//		friendlyKingRank = bKingSq / 8
//		enemyKingFile = wKingSq % 8
//		enemyKingRank = wKingSq / 8
//	}
//
//	// Max of either distance by rank or by file; either way we close the distance
//	r2r1 := math.Abs(float64(enemyKingRank) - float64(friendlyKingRank))
//	f2f1 := math.Abs(float64(enemyKingFile) - float64(friendlyKingFile))
//
//	if whiteWithAdvantage {
//		score = Max(int(r2r1), int(f2f1)) * -35
//	} else {
//		score = Max(int(r2r1), int(f2f1)) * 35
//	}
//
//	return score
//}

func kingFilesPenalty(b *dragontoothmg.Board, openFiles uint64, wSemiOpenFiles uint64, bSemiOpenFiles uint64) (score int) {
	// Get the king's files
	wKingFiles := onlyFile[bits.TrailingZeros64(b.White.Kings)%8]
	bKingFiles := onlyFile[bits.TrailingZeros64(b.Black.Kings)%8]

	// Left & right files of the king
	wKingFiles = wKingFiles | ((wKingFiles & ^bitboardFileA) >> 1) | ((wKingFiles & ^bitboardFileH) << 1)
	bKingFiles = bKingFiles | ((bKingFiles & ^bitboardFileA) >> 1) | ((bKingFiles & ^bitboardFileH) << 1)

	// onlyFile returns a full file, so we need to /8 to get the count
	wSemiOpenFilesCount := bits.OnesCount64(wKingFiles&bSemiOpenFiles) / 8
	wOpenFilesCount := bits.OnesCount64(wKingFiles&openFiles) / 8
	bSemiOpenFilesCount := bits.OnesCount64(bKingFiles&wSemiOpenFiles) / 8
	bOpenFilesCount := bits.OnesCount64(bKingFiles&openFiles) / 8

	//fmt.Printf("King files: %v | %v \t Semi-Open: %v | %v \t Open: %v | %v \n", wKingFiles, bKingFiles, wSemiOpenFilesCount, bSemiOpenFilesCount, wOpenFilesCount, bOpenFilesCount)

	if wSemiOpenFilesCount > 0 {
		score -= wSemiOpenFilesCount * KingSemiOpenFilePenalty
	}
	if wOpenFilesCount > 0 {
		score -= wOpenFilesCount * KingOpenFilePenalty
	}
	if bSemiOpenFilesCount > 0 {
		score += bSemiOpenFilesCount * KingSemiOpenFilePenalty
	}
	if bOpenFilesCount > 0 {
		score += bOpenFilesCount * KingOpenFilePenalty
	}

	return score
}

func kingAttackCountPenalty(attackUnitCount *[2]int) (kingAttacksPenaltyMG int) {

	if attackUnitCount[0] > 99 {
		attackUnitCount[0] = 99
	}
	if attackUnitCount[1] > 99 {
		attackUnitCount[1] = 99
	}

	return KingSafetyTable[attackUnitCount[0]] - KingSafetyTable[attackUnitCount[1]]
}

func kingActivity(b *dragontoothmg.Board, piecePhase int, wMaterial int, bMaterial int) int {
	var score int = 0
	//println("PIece phase: ", piecePhase)
	if piecePhase <= 12 {
		wKingSq := bits.TrailingZeros64(b.White.Kings)
		bKingSq := bits.TrailingZeros64(b.Black.Kings)

		wKingRank := wKingSq / 8
		wKingFile := wKingSq % 8
		bKingRank := bKingSq / 8
		bKingFile := bKingSq % 8

		// Chebyshev distance between kings (king proximity)
		kingDist := Max(abs(wKingRank-bKingRank), abs(wKingFile-bKingFile))

		// Center manhattan distance from center
		wCenterDist := CenterManhattanDistance[wKingSq]
		bCenterDist := CenterManhattanDistance[bKingSq]

		//fmt.Printf("Center Dst: %v | %v       King dist: %v", wCenterDist, bCenterDist, kingDist)

		// Phase-scaled bonuses
		phaseScale := (24 - piecePhase)

		if wMaterial > bMaterial+200 {
			score -= kingDist * 12 * phaseScale / 24   // Get closer to enemy king
			score -= wCenterDist * 4 * phaseScale / 24 // De-Centralize Enemy king
		}
		if bMaterial > wMaterial+200 {
			score += kingDist * 12 * phaseScale / 24
			score += bCenterDist * 4 * phaseScale / 24
		}
	}
	return score
}

func Evaluation(b *dragontoothmg.Board, debug bool, isQuiescence bool) (score int) {
	// UPDATE & INIT VARIABLES FOR EVAL
	// Prepare pawn attacks and pawn attack spans

	var wPawnAttackBBEast, wPawnAttackBBWest = PawnCaptureBitboards(b.White.Pawns, true)
	var bPawnAttackBBEast, bPawnAttackBBWest = PawnCaptureBitboards(b.Black.Pawns, false)
	var wPawnAttackBB = wPawnAttackBBEast | wPawnAttackBBWest
	var bPawnAttackBB = bPawnAttackBBEast | bPawnAttackBBWest

	var wPawnAttackSpan, bPawnAttackSpan = pawnAttackSpan(wPawnAttackBB, bPawnAttackBB)

	// Pawn bitboards
	var wPhalanxsPawnsBB, bPhalanxsPawnsBB,
		wBlockedPawnsBB, bBlockedPawnsBB,
		wConnectedPawnsBB, bConnectedPawnsBB,
		wPassedPawnsBB, bPassedPawnsBB,
		wDoubledPawnsBB, bDoubledPawnsBB,
		wIsolatedPawnsBB, bIsolatedPawnsBB,
		wClosestPawn, bClosestPawn = getPawnBBs(b, wPawnAttackBB, bPawnAttackBB) // Last two are integers are not bitboards!

	var openFiles, wSemiOpenFiles, bSemiOpenFiles uint64 = getOpenFiles(b)

	// Queen infiltration BB
	var wQueenInfiltrationBB, bQueenInfiltrationBB = getQueenInfiltrationBB(wPawnAttackSpan, bPawnAttackSpan)

	// Prepare movement bitboard slots
	// For space control calculations
	var knightMovementBB = [2]uint64{}
	var bishopMovementBB = [2]uint64{}
	var rookMovementBB = [2]uint64{}
	var queenMovementBB = [2]uint64{}
	var kingMovementBB = [2]uint64{}

	// Get outpost bitboards
	var outposts = getOutpostsBB(wPawnAttackBB, bPawnAttackBB, wPawnAttackSpan, bPawnAttackSpan)
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

	var innerKingSafetyZones = getInnerKingSafetyTable(b)
	var outerKingSafetyZones = getOuterKingSafetyTable(innerKingSafetyZones)

	if debug {
		fmt.Printf("################### FEN ###################\n")
		fmt.Printf("FEN: %v\n", b.ToFen())
		fmt.Printf("################### HELPER VARIABLES ###################\n")
		fmt.Printf("Pawn attacks: %v || %v\n", wPawnAttackBB, bPawnAttackBB)
		fmt.Printf("Pawn attack spans: %v || %v\n", wPawnAttackSpan, bPawnAttackSpan)
		fmt.Printf("Open files: %v\n", openFiles)
		fmt.Printf("Semi-Open files: %v || %v\n", wSemiOpenFiles, bSemiOpenFiles)
		fmt.Printf("Outposts: %v || %v\n", outposts[0], outposts[1])
		fmt.Printf("Queen infiltration BB: %v || %v\n", wQueenInfiltrationBB, bQueenInfiltrationBB)
		fmt.Printf("King safety tables inner: %v || %v\n", innerKingSafetyZones[0], innerKingSafetyZones[1])
		fmt.Printf("King safety tables outer: %v || %v\n", outerKingSafetyZones[0], outerKingSafetyZones[1])
		fmt.Printf("################### PAWN HELPER VARS ###################\n")
		fmt.Printf("Doubled:\t %v || %v\n", wDoubledPawnsBB, bDoubledPawnsBB)
		fmt.Printf("Isolated:\t %v || %v\n", wIsolatedPawnsBB, bIsolatedPawnsBB)
		fmt.Printf("Passed: \t %v || %v\n", wPassedPawnsBB, bPassedPawnsBB)
		fmt.Printf("Connected:\t %v || %v\n", wConnectedPawnsBB, bConnectedPawnsBB)
		fmt.Printf("Blocked:\t %v || %v\n", wBlockedPawnsBB, bBlockedPawnsBB)
		fmt.Printf("Phalanx:\t %v || %v\n", wPhalanxsPawnsBB, bPhalanxsPawnsBB)
		fmt.Printf("Closest pawn:\t %v || %v\n", wClosestPawn, bClosestPawn)
		fmt.Printf("################### TACTICAL PIECE VALUES ###################\n")
	}

	//wPieceCount := bits.OnesCount64(b.White.Bishops | b.White.Knights | b.White.Rooks | b.White.Queens)
	//bPieceCount := bits.OnesCount64(b.Black.Bishops | b.Black.Knights | b.Black.Rooks | b.Black.Queens)

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
			- Connected pawns means a pawn defending another
				- If also a passed pawn, gives double the bonus
			- Passed pawn means a pawn that has no opponent pawns that can attack nor block it
			- Phalanx pawns means a pawn that has a pawn next to it

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
			passedMG, passedEG := passedPawnBonus(wPassedPawnsBB, bPassedPawnsBB, wConnectedPawnsBB, bConnectedPawnsBB)
			doubledMG, doubledEG := pawnDoublingPenalties(wDoubledPawnsBB, bDoubledPawnsBB, wIsolatedPawnsBB, bIsolatedPawnsBB)
			isolatedMG, isolatedEG := isolatedPawnPenalty(wIsolatedPawnsBB, bIsolatedPawnsBB)
			//phalanxMG, phalanxEG, connectedMG, connectedEG := phalanxOrConnectedPawnBonus(wPhalanxsPawnsBB, bPhalanxsPawnsBB, wConnectedPawnsBB, bConnectedPawnsBB)
			//blockedPawnBonusMG, blockedPawnBonusEG := blockedPawnBonus(wBlockedPawnsBB, bBlockedPawnsBB)

			// Transition from more complex pawn structures to just prioritizing passers as endgame nears...
			// Not sure if it's good, but it's something?
			pawnMG += pawnPsqtMG + passedMG + doubledMG + isolatedMG //+ blockedPawnBonusMG //+ phalanxMG + connectedMG
			pawnEG += pawnPsqtEG + passedEG + doubledEG + isolatedEG //+ blockedPawnBonusEG //+ phalanxEG + connectedEG
			if debug {
				println("Pawn MG:\t", "PSQT: ", pawnPsqtMG, "\tPassed: ", passedMG, "\tDoubled:", doubledMG, "\tIsolated: ", isolatedMG) //, "\tBlocked: ", blockedPawnBonusMG) //, "\tPhalanx: ", phalanxMG, "\tConnected: ", connectedMG)
				println("Pawn EG:\t", "PSQT: ", pawnPsqtEG, "\tPassed: ", passedEG, "\tDoubled:", doubledEG, "\tIsolated: ", isolatedEG) //, "\tBlocked: ", blockedPawnBonusEG) //, "\tPhalanx: ", phalanxEG, "\tConnected: ", connectedEG)
			}
		case dragontoothmg.Knight:
			knightPsqtMG, knightPsqtEG := countPieceTables(&b.White.Knights, &b.Black.Knights, &PSQT_MG[dragontoothmg.Knight], &PSQT_EG[dragontoothmg.Knight])
			var knightMobilityMG, knightMobilityEG int
			for x := b.White.Knights; x != 0; x &= x - 1 {
				square := bits.TrailingZeros64(x)
				movementBB := (KnightMasks[square] &^ b.White.All)
				knightMovementBB[0] |= movementBB
				knightMobilityMG += bits.OnesCount64(movementBB&^bPawnAttackBB) * MobilityValueMG[dragontoothmg.Knight]
				knightMobilityEG += bits.OnesCount64(movementBB&^bPawnAttackBB) * MobilityValueEG[dragontoothmg.Knight]
				attackUnitCounts[0] += (bits.OnesCount64(movementBB&innerKingSafetyZones[1]) * attackerInner[dragontoothmg.Knight])
				attackUnitCounts[0] += (bits.OnesCount64(movementBB&outerKingSafetyZones[1]) * attackerOuter[dragontoothmg.Knight])
			}
			for x := b.Black.Knights; x != 0; x &= x - 1 {
				square := bits.TrailingZeros64(x)
				movementBB := (KnightMasks[square] &^ b.Black.All)
				knightMovementBB[1] |= movementBB
				knightMobilityMG -= bits.OnesCount64(movementBB&^wPawnAttackBB) * MobilityValueMG[dragontoothmg.Knight]
				knightMobilityEG -= bits.OnesCount64(movementBB&^wPawnAttackBB) * MobilityValueEG[dragontoothmg.Knight]
				attackUnitCounts[1] += (bits.OnesCount64(movementBB&innerKingSafetyZones[0]) * attackerInner[dragontoothmg.Knight])
				attackUnitCounts[1] += (bits.OnesCount64(movementBB&outerKingSafetyZones[0]) * attackerOuter[dragontoothmg.Knight])
			}
			var knightOutpostMG = (KnightOutpostMG * bits.OnesCount64(b.White.Knights&whiteOutposts)) - (KnightOutpostMG * bits.OnesCount64(b.Black.Knights&blackOutposts))
			var knightOutpostEG = (KnightOutpostEG * bits.OnesCount64(b.White.Knights&whiteOutposts)) - (KnightOutpostEG * bits.OnesCount64(b.Black.Knights&blackOutposts))
			var knightThreatsBonusMG = knightThreats(b)
			knightMG += knightPsqtMG + knightOutpostMG + knightMobilityMG + knightThreatsBonusMG
			knightEG += knightPsqtEG + knightOutpostEG + knightMobilityEG
			if debug {
				fmt.Printf("Knight MG:\t PSQT: %v\t Mobility: %v\t Outpost: %v\t Threats: %v\t\n", knightPsqtMG, knightMobilityMG, knightOutpostMG, knightThreatsBonusMG)
				fmt.Printf("Knight EG:\t PSQT: %v\t Mobility: %v\t Outpost: %v\t\n", knightPsqtEG, knightMobilityEG, knightOutpostEG)
			}
		case dragontoothmg.Bishop:
			bishopPsqtMG, bishopPsqtEG := countPieceTables(&b.White.Bishops, &b.Black.Bishops, &PSQT_MG[dragontoothmg.Bishop], &PSQT_EG[dragontoothmg.Bishop])
			var bishopMobilityMG, bishopMobilityEG int
			for x := b.White.Bishops; x != 0; x &= x - 1 {
				square := bits.TrailingZeros64(x)
				movementBB := dragontoothmg.CalculateBishopMoveBitboard(uint8(square), (b.White.All|b.Black.All)) &^ b.White.All
				bishopMovementBB[0] |= movementBB
				bishopMobilityMG += bits.OnesCount64(movementBB&^bPawnAttackBB) * MobilityValueMG[dragontoothmg.Bishop]
				bishopMobilityEG += bits.OnesCount64(movementBB&^bPawnAttackBB) * MobilityValueEG[dragontoothmg.Bishop]
				attackUnitCounts[0] += (bits.OnesCount64(movementBB&innerKingSafetyZones[1]) * attackerInner[dragontoothmg.Bishop])
				attackUnitCounts[0] += (bits.OnesCount64(movementBB&outerKingSafetyZones[1]) * attackerOuter[dragontoothmg.Bishop])
			}
			for x := b.Black.Bishops; x != 0; x &= x - 1 {
				square := bits.TrailingZeros64(x)
				movementBB := dragontoothmg.CalculateBishopMoveBitboard(uint8(square), (b.White.All|b.Black.All)) &^ b.Black.All
				bishopMovementBB[1] |= movementBB
				bishopMobilityMG -= bits.OnesCount64(movementBB&^wPawnAttackBB) * MobilityValueMG[dragontoothmg.Bishop]
				bishopMobilityEG -= bits.OnesCount64(movementBB&^wPawnAttackBB) * MobilityValueEG[dragontoothmg.Bishop]
				attackUnitCounts[1] += (bits.OnesCount64(movementBB&innerKingSafetyZones[0]) * attackerInner[dragontoothmg.Bishop])
				attackUnitCounts[1] += (bits.OnesCount64(movementBB&outerKingSafetyZones[0]) * attackerOuter[dragontoothmg.Bishop])
			}
			var bishopOutpostMG = (BishopOutpostMG * bits.OnesCount64(b.White.Bishops&whiteOutposts)) - (BishopOutpostMG * bits.OnesCount64(b.Black.Bishops&blackOutposts))
			var bishopPairMG, bishopPairEG = bishopPairBonuses(b)
			var bishopXrayAttackMG = bishopXrayAttacks(b)
			var bishopColorSetupMG, bishopColorSetupEG = bishopPawnColorRatio(b, wBlockedPawnsBB, bBlockedPawnsBB)

			bishopMG += bishopPsqtMG + bishopMobilityMG + bishopPairMG + bishopXrayAttackMG + bishopOutpostMG //+ bishopColorSetupMG
			bishopEG += bishopPsqtEG + bishopMobilityEG + bishopPairEG                                        //+ bishopColorSetupEG
			if debug {
				println("Bishop MG:\t", "PSQT: ", bishopPsqtMG, "\tMobility: ", bishopMobilityMG, "\tColor: ", bishopColorSetupMG, "\tOutpost:", bishopOutpostMG, "\tPair: ", bishopPairMG, "\tBishop attacks: ", bishopXrayAttackMG)
				println("Bishop EG:\t", "PSQT: ", bishopPsqtEG, "\tMobility: ", bishopMobilityEG, "\tColor: ", bishopColorSetupEG, "\tPair: ", bishopPairEG)
			}
		case dragontoothmg.Rook:
			var rookPsqtMG, rookPsqtEG = countPieceTables(&b.White.Rooks, &b.Black.Rooks, &PSQT_MG[dragontoothmg.Rook], &PSQT_EG[dragontoothmg.Rook])
			var rookSemiOpenMG, rookOpenMG = rookFilesBonus(b, openFiles, wSemiOpenFiles, bSemiOpenFiles)
			var rookSeventhRankBonus = rookOnSecondOrSeventh(b)
			var rookXrayAttackMG = rookAttacks(b)
			var rookMobilityMG, rookMobilityEG int
			for x := b.White.Rooks; x != 0; x &= x - 1 {
				square := bits.TrailingZeros64(x)
				movementBB := dragontoothmg.CalculateRookMoveBitboard(uint8(square), (b.White.All|b.Black.All)) &^ b.White.All
				rookMovementBB[0] |= movementBB
				rookMobilityMG += bits.OnesCount64(movementBB&^bPawnAttackBB) * MobilityValueMG[dragontoothmg.Rook]
				rookMobilityEG += bits.OnesCount64(movementBB&^bPawnAttackBB) * MobilityValueEG[dragontoothmg.Rook]
				attackUnitCounts[0] += (bits.OnesCount64(movementBB&innerKingSafetyZones[1]) * attackerInner[dragontoothmg.Rook])
				attackUnitCounts[0] += (bits.OnesCount64(movementBB&outerKingSafetyZones[1]) * attackerOuter[dragontoothmg.Rook])
			}
			for x := b.Black.Rooks; x != 0; x &= x - 1 {
				square := bits.TrailingZeros64(x)
				movementBB := dragontoothmg.CalculateRookMoveBitboard(uint8(square), (b.White.All|b.Black.All)) & ^b.Black.All
				rookMovementBB[1] |= movementBB
				rookMobilityMG -= bits.OnesCount64(movementBB&^wPawnAttackBB) * MobilityValueMG[dragontoothmg.Rook]
				rookMobilityEG -= bits.OnesCount64(movementBB&^wPawnAttackBB) * MobilityValueEG[dragontoothmg.Rook]
				attackUnitCounts[1] += (bits.OnesCount64(movementBB&innerKingSafetyZones[0]) * attackerInner[dragontoothmg.Rook])
				attackUnitCounts[1] += (bits.OnesCount64(movementBB&outerKingSafetyZones[0]) * attackerOuter[dragontoothmg.Rook])
			}
			rookMG += rookPsqtMG + rookMobilityMG + rookOpenMG + rookSemiOpenMG + rookXrayAttackMG
			rookEG += rookPsqtEG + rookMobilityEG + rookSeventhRankBonus
			if debug {
				println("Rook MG:\t", "PSQT: ", rookPsqtMG, "\tMobility: ", rookMobilityMG, "\tOpen: ", rookOpenMG, "\tSemiOpen: ", rookSemiOpenMG, "\tRook Xray: ", rookXrayAttackMG)
				println("Rook EG:\t", "PSQT: ", rookPsqtEG, "\tMobility: ", rookMobilityEG, "\tSeventh: ", rookSeventhRankBonus)
			}
		case dragontoothmg.Queen:
			var queenPsqtMG, queenPsqtEG int = countPieceTables(&b.White.Queens, &b.Black.Queens, &PSQT_MG[dragontoothmg.Queen], &PSQT_EG[dragontoothmg.Queen])
			var queenMobilityMG, queenMobilityEG int
			for x := b.White.Queens; x != 0; x &= x - 1 {
				square := bits.TrailingZeros64(x)
				movementBB := dragontoothmg.CalculateBishopMoveBitboard(uint8(square), (b.White.All|b.Black.All)) & ^b.White.All
				movementBB |= dragontoothmg.CalculateRookMoveBitboard(uint8(square), (b.White.All|b.Black.All)) & ^b.White.All
				queenMovementBB[0] |= movementBB
				queenMobilityMG += bits.OnesCount64(movementBB&^bPawnAttackBB) * MobilityValueMG[dragontoothmg.Queen]
				queenMobilityEG += bits.OnesCount64(movementBB&^bPawnAttackBB) * MobilityValueEG[dragontoothmg.Queen]
				attackUnitCounts[0] += (bits.OnesCount64(movementBB&innerKingSafetyZones[1]) * attackerInner[dragontoothmg.Queen])
				attackUnitCounts[0] += (bits.OnesCount64(movementBB&outerKingSafetyZones[1]) * attackerOuter[dragontoothmg.Queen])
			}
			for x := b.Black.Queens; x != 0; x &= x - 1 {
				square := bits.TrailingZeros64(x)
				movementBB := dragontoothmg.CalculateBishopMoveBitboard(uint8(square), (b.White.All|b.Black.All)) & ^b.Black.All
				movementBB |= dragontoothmg.CalculateRookMoveBitboard(uint8(square), (b.White.All|b.Black.All)) & ^b.Black.All
				queenMovementBB[1] |= movementBB
				queenMobilityMG -= bits.OnesCount64(movementBB&^wPawnAttackBB) * MobilityValueMG[dragontoothmg.Queen]
				queenMobilityEG -= bits.OnesCount64(movementBB&^wPawnAttackBB) * MobilityValueEG[dragontoothmg.Queen]
				attackUnitCounts[1] += (bits.OnesCount64(movementBB&innerKingSafetyZones[0]) * attackerInner[dragontoothmg.Queen])
				attackUnitCounts[1] += (bits.OnesCount64(movementBB&outerKingSafetyZones[0]) * attackerOuter[dragontoothmg.Queen])
			}

			var centralizedQueenBonus = centralizedQueen(b)
			var queenInfiltrationBonusMG, queenInfiltrationBonusEG = queenInfiltrationBonus(b, wQueenInfiltrationBB, bQueenInfiltrationBB, knightMovementBB, bishopMovementBB, rookMovementBB)

			queenMG += queenPsqtMG + queenMobilityMG + queenInfiltrationBonusMG
			queenEG += queenPsqtEG + queenMobilityEG + centralizedQueenBonus + queenInfiltrationBonusEG

			if debug {
				println("Queen MG:\t", "PSQT: ", queenPsqtMG, "\tMobility: ", queenMobilityMG, "\tInfiltration: ", queenInfiltrationBonusMG)
				println("Queen EG:\t", "PSQT: ", queenPsqtEG, "\tMobility: ", queenMobilityEG, "\tInfiltration: ", queenInfiltrationBonusEG, "\tCentralized Queen bonus", centralizedQueenBonus)
			}
		case dragontoothmg.King:
			kingPsqtMG, kingPsqtEG := countPieceTables(&b.White.Kings, &b.Black.Kings, &PSQT_MG[dragontoothmg.King], &PSQT_EG[dragontoothmg.King])
			kingAttackPenaltyMG := kingAttackCountPenalty(&attackUnitCounts)
			kingFilesPenaltyMG := kingFilesPenalty(b, openFiles, wSemiOpenFiles, bSemiOpenFiles)
			KingMinorPieceDefenseBonusMG := kingMinorPieceDefences(innerKingSafetyZones, knightMovementBB, bishopMovementBB)
			kingPawnDefenseMG := kingPawnDefense(b)
			kingPawnDistancePenaltyEG := kingPawnDistance(wClosestPawn, bClosestPawn)

			kingMovementBB[0] = (innerKingSafetyZones[0] &^ b.White.All) &^ (knightMovementBB[1] | bishopMovementBB[1] | rookMovementBB[1] | queenMovementBB[1])
			kingMovementBB[1] = (innerKingSafetyZones[1] &^ b.Black.All) &^ (knightMovementBB[0] | bishopMovementBB[0] | rookMovementBB[0] | queenMovementBB[0])
			/*
				If we're below a certain count of pieces (excluding pawns), we try to centralize our king
				We're more likely to centralize queens are traded off
				If our opponent has no pieces left, we try to follow the enemy king to find a faster mating sequence
			*/
			var kingActivityEG int
			kingActivityEG = kingActivity(b, piecePhase, wMaterialEG, bMaterialEG)

			kingMG += kingPsqtMG + kingAttackPenaltyMG + kingFilesPenaltyMG + KingMinorPieceDefenseBonusMG + kingPawnDefenseMG
			kingEG += kingPsqtEG + kingPawnDistancePenaltyEG + kingActivityEG
			if debug {
				fmt.Printf("King MG:\t PSQT: %v \t Attack: %v \t Shield: %v \t King pawn defense: %v \t Minor defense: %v\n", kingPsqtMG, kingAttackPenaltyMG, kingFilesPenaltyMG, kingPawnDefenseMG, KingMinorPieceDefenseBonusMG)
				fmt.Printf("King EG:\t PSQT: %v \t Pawn distance: %v \t Activity: %v \n", kingPsqtEG, kingPawnDistancePenaltyEG, kingActivityEG)
				// Test of phasing out kingAttackPenaltyEG, will remove fully if this makes the engine stronger
			}
		}
	}

	/*
		Weak square control - based on how well squares in ones own ""zone"" is defended
		Squares attacked by opponent pieces, that are undefended or only defended by king/queen is ""weak""
		Idea is to prioritize space control; to manage what squares are important to defend, change the bitmask in the getWeakSquares function
	*/
	var movementBB [2][6]uint64 = [2][6]uint64{
		{
			knightMovementBB[0], bishopMovementBB[0], rookMovementBB[0], queenMovementBB[0], kingMovementBB[0],
		},
		{
			knightMovementBB[1], bishopMovementBB[1], rookMovementBB[1], queenMovementBB[1], kingMovementBB[1],
		},
	}
	var weakSquares = getWeakSquares(movementBB, wPawnAttackBB, bPawnAttackBB)

	var wWeakSquarePenalty = bits.OnesCount64(weakSquares[0]) * weakSquaresPenalty
	var bWeakSquarePenalty = bits.OnesCount64(weakSquares[1]) * weakSquaresPenalty

	var weakSquareMG int = (bWeakSquarePenalty - wWeakSquarePenalty)
	_ = weakSquareMG

	/* Calculate score from all variables */
	var materialScoreMG = (wMaterialMG - bMaterialMG)
	var materialScoreEG = (wMaterialEG - bMaterialEG)

	// Tempo bonus for side to move
	var toMoveBonus = 20
	if !b.Wtomove {
		toMoveBonus = -20
	}

	var variableScoreMG = pawnMG + knightMG + bishopMG + rookMG + queenMG + kingMG + toMoveBonus //+ weakSquareMG
	var variableScoreEG = pawnEG + knightEG + bishopEG + rookEG + queenEG + kingEG + toMoveBonus

	var mgScore = variableScoreMG + materialScoreMG
	var egScore = variableScoreEG + materialScoreEG

	var mgPhase = 1 - (float64(currPhase) / 24.0)
	var egPhase = float64(currPhase) / 24.0
	score = int((float64(mgScore) * mgPhase) + (float64(egScore) * egPhase))

	if debug {
		fmt.Printf("################### MOBILITY ###################\n")
		fmt.Printf("Knights: %v || %v\n", movementBB[0][0], movementBB[1][0])
		fmt.Printf("Bishops: %v || %v\n", movementBB[0][1], movementBB[1][1])
		fmt.Printf("Rooks: %v || %v\n", movementBB[0][2], movementBB[1][2])
		fmt.Printf("Queens: %v || %v\n", movementBB[0][3], movementBB[1][3])
		fmt.Printf("Weak squares: %v || %v\n", weakSquares[0], weakSquares[1])
		fmt.Printf("Weak squares scores: %v || %v\n", wWeakSquarePenalty, bWeakSquarePenalty)
		fmt.Printf("################### START PHASE ###################\n")
		fmt.Printf("Piece phase: \t\t %v", piecePhase)
		fmt.Printf("Midgame phase: %.2f\n", mgPhase)
		fmt.Printf("Total phase: \t\t %v", TotalPhase)
		fmt.Printf("Reduced phase: \t\t %v", (currPhase*256+12)/TotalPhase)
	}

	if isTheoreticalDraw(b, debug) {
		score = score / DrawDivider
	}

	if debug {
		println("################### MIDGAME_EVAL : ENDGAME_EVAL  ###################")
		println("PSQT eval: \t\t\t", psqtMG, ":", psqtEG)
		//println("Weak Squares eval: \t\t", weakSquareMG, ":")
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

	if isQuiescence && b.Fullmoveno > 8 {
		println("Quiescence eval: ", score, " ---- FEN: ", b.ToFen())
	}

	if !b.Wtomove {
		score = -score
	}

	return score
}
