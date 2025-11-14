package engine

import (
	"cmp"
	"fmt"
	"math/bits"

	gm "chess-engine/goosemg"
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
var pieceList = [6]gm.PieceType{gm.PieceTypePawn, gm.PieceTypeKnight, gm.PieceTypeBishop, gm.PieceTypeRook, gm.PieceTypeQueen, gm.PieceTypeKing}

/* Helper variables */
// Outpost variables, updated each time evaluation is called
var whiteOutposts uint64
var blackOutposts uint64

var wPhalanxOrConnectedEndgameInvalidSquares uint64 = 0xffff00
var bPhalanxOrConnectedEndgameInvalidSquares uint64 = 0xffff0000000000

var wAllowedOutpostMask uint64 = 0xffff7e7e000000
var bAllowedOutpostMask uint64 = 0x7e7effff00

var seventhRankMask uint64 = 0xff000000000000
var secondRankMask uint64 = 0xff00

/* Queen variables ... Pretty empty :'( */
var centralizedQueenSquares uint64 = 0x183c3c180000

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

var weakSquaresPenaltyMG = 2
var weakKingSquaresPenaltyMG = 5
var PSQT_MG = [7][64]int{
	gm.PieceTypePawn: {
		0, 0, 0, 0, 0, 0, 0, 0,
		-14, -16, -17, -10, -12, 29, 29, 5,
		-21, -29, -22, -21, -10, 0, 7, -7,
		-14, -16, -8, -10, 3, 18, 13, 0,
		-5, 0, -1, 15, 27, 45, 28, 9,
		-2, 16, 30, 35, 48, 82, 40, 5,
		77, 72, 70, 68, 53, 40, 14, 21,
		0, 0, 0, 0, 0, 0, 0, 0,
	},
	gm.PieceTypeKnight: {
		-17, -12, -25, -7, -2, 2, -12, -29,
		-20, -18, -3, 7, 3, 4, 0, -1,
		-14, 4, 6, 15, 19, 6, 5, -8,
		3, 13, 21, 23, 28, 22, 33, 13,
		5, 20, 40, 45, 29, 46, 22, 31,
		-15, 18, 43, 44, 61, 62, 33, 12,
		-7, 0, 32, 35, 34, 36, -2, 9,
		-53, -3, -9, 1, 3, -4, 0, -13,
	},
	gm.PieceTypeBishop: {
		0, -2, -15, -16, -16, -10, -9, -3,
		-1, 4, 9, -4, -2, 5, 10, 1,
		-10, 4, 2, 3, 0, 2, -3, -3,
		-13, -1, 4, 15, 18, -7, -1, -4,
		-16, 11, 6, 28, 17, 21, 8, -9,
		-8, 4, 15, 7, 23, 27, 15, -2,
		-31, -19, -12, -13, -16, -2, -20, -7,
		-20, -8, -14, -15, -9, -20, 0, -10,
	},
	gm.PieceTypeRook: {
		0, 4, 8, 15, 10, 13, 11, -1,
		-31, -9, -12, -6, -11, 1, 11, -24,
		-19, -12, -19, -8, -14, -14, 3, -12,
		-16, -17, -16, -6, -15, -15, 3, -11,
		-6, 3, 8, 21, 5, 8, 7, 3,
		0, 32, 18, 33, 29, 24, 32, 14,
		6, 0, 15, 20, 3, 11, -3, 18,
		23, 20, 6, 8, -7, 3, 12, 22,
	},
	gm.PieceTypeQueen: {
		15, 13, 19, 26, 27, 4, -2, 1,
		10, 17, 25, 22, 24, 35, 33, 9,
		7, 19, 16, 11, 9, 10, 18, 3,
		10, 13, 7, 2, -4, -14, 0, -9,
		0, 4, -14, -28, -22, -23, -7, -11,
		-4, 2, 1, -21, -35, -26, -31, -34,
		-1, -37, -3, -16, -55, -28, -23, 15,
		1, 9, 4, -2, -10, 3, 11, 14,
	},
	gm.PieceTypeKing: {
		-4, 36, 4, -53, -22, -51, 14, 22,
		0, -12, -20, -52, -35, -37, -6, 13,
		-4, -5, 6, 7, 15, 8, 1, -12,
		-1, 8, 21, 19, 23, 18, 20, -7,
		0, 8, 16, 11, 14, 16, 11, -8,
		0, 8, 13, 10, 8, 14, 9, 0,
		-2, 4, 5, 3, 3, 5, 3, -2,
		-2, 0, 1, 1, 1, 0, 0, -1,
	},
}
var PSQT_EG = [7][64]int{
	gm.PieceTypePawn: {
		0, 0, 0, 0, 0, 0, 0, 0,
		20, 16, 20, 19, 26, 22, 7, -5,
		16, 14, 14, 15, 17, 16, 4, 3,
		22, 21, 9, 7, 4, 9, 7, 9,
		33, 27, 22, 0, 8, 12, 18, 17,
		56, 62, 56, 51, 48, 41, 54, 50,
		121, 110, 94, 79, 73, 72, 84, 100,
		0, 0, 0, 0, 0, 0, 0, 0,
	},
	gm.PieceTypeKnight: {
		-20, -40, -15, -5, -7, -15, -30, -22,
		-16, -1, -3, 6, 7, -6, -5, -16,
		-26, 7, 12, 28, 24, 7, 5, -24,
		-3, 22, 39, 45, 41, 41, 22, 1,
		2, 23, 36, 49, 54, 42, 35, 10,
		-8, 16, 30, 32, 27, 40, 20, -1,
		-13, 0, 10, 31, 29, 8, 1, -6,
		-29, -3, 11, 8, 8, 11, 1, -13,
	},
	gm.PieceTypeBishop: {
		-17, -3, -16, 1, -5, -8, -10, -10,
		-1, -14, -3, 4, 4, -8, -6, -26,
		-1, 8, 14, 19, 16, 10, -1, 1,
		5, 13, 23, 20, 17, 20, 13, -2,
		11, 20, 19, 21, 25, 20, 24, 15,
		10, 21, 21, 18, 20, 26, 24, 15,
		6, 22, 20, 21, 20, 21, 21, 7,
		8, 12, 14, 18, 17, 7, 11, 8,
	},
	gm.PieceTypeRook: {
		-3, 1, 0, -6, -11, 0, -3, -16,
		-1, -5, 1, -5, -8, -17, -9, -6,
		6, 13, 11, 5, 1, 0, 4, -2,
		20, 28, 26, 17, 14, 15, 16, 11,
		26, 26, 25, 17, 15, 12, 16, 19,
		30, 18, 26, 15, 9, 18, 11, 17,
		9, 14, 10, 11, 9, -3, 7, -1,
		25, 27, 28, 22, 24, 35, 38, 37,
	},
	gm.PieceTypeQueen: {
		-9, -10, -14, 3, -14, -8, -6, -1,
		0, -5, -14, 6, -2, -31, -15, -1,
		3, 15, 20, -7, -8, 25, 14, 3,
		13, 25, 3, 18, 16, 14, 34, 27,
		22, 38, 2, 21, 12, 11, 42, 29,
		24, 26, 23, 7, 0, 12, 13, 17,
		29, 44, 24, 21, 25, -4, 16, 19,
		11, 21, 16, 9, 2, 11, 19, 14,
	},
	gm.PieceTypeKing: {
		-39, -39, -19, -21, -42, -15, -42, -85,
		-17, -9, 2, 8, 4, 4, -17, -38,
		-14, 1, 12, 24, 19, 9, -7, -18,
		-14, 13, 27, 36, 31, 22, 6, -18,
		0, 25, 33, 36, 34, 32, 23, -5,
		3, 29, 31, 24, 21, 39, 35, 0,
		-11, 15, 14, 6, 6, 14, 23, -8,
		-15, -9, -3, 0, -3, -2, -4, -11,
	},
}

var pieceValueMG = [7]int{gm.PieceTypeKing: 0, gm.PieceTypePawn: 77, gm.PieceTypeKnight: 333, gm.PieceTypeBishop: 358, gm.PieceTypeRook: 473, gm.PieceTypeQueen: 990}
var pieceValueEG = [7]int{gm.PieceTypeKing: 0, gm.PieceTypePawn: 95, gm.PieceTypeKnight: 298, gm.PieceTypeBishop: 305, gm.PieceTypeRook: 521, gm.PieceTypeQueen: 905}
var mobilityValueMG = [7]int{gm.PieceTypeKing: 0, gm.PieceTypePawn: 0, gm.PieceTypeKnight: 3, gm.PieceTypeBishop: 2, gm.PieceTypeRook: 2, gm.PieceTypeQueen: 1}
var mobilityValueEG = [7]int{gm.PieceTypeKing: 0, gm.PieceTypePawn: 0, gm.PieceTypeKnight: 2, gm.PieceTypeBishop: 3, gm.PieceTypeRook: 6, gm.PieceTypeQueen: 7}

var attackerInner = [7]int{gm.PieceTypePawn: 1, gm.PieceTypeKnight: 2, gm.PieceTypeBishop: 2, gm.PieceTypeRook: 4, gm.PieceTypeQueen: 6, gm.PieceTypeKing: 0}
var attackerOuter = [7]int{gm.PieceTypePawn: 0, gm.PieceTypeKnight: 1, gm.PieceTypeBishop: 1, gm.PieceTypeRook: 2, gm.PieceTypeQueen: 2, gm.PieceTypeKing: 0}

/* Pawn variables */
var PassedPawnPSQT_MG = [64]int{
	0, 0, 0, 0, 0, 0, 0, 0,
	-13, -10, -9, -10, -7, -24, -6, 11,
	-3, -4, -16, -14, -10, -14, -12, 8,
	18, 8, -9, -5, -11, -19, -4, 6,
	38, 35, 26, 20, 10, 6, 15, 22,
	77, 64, 52, 43, 29, 26, 21, 24,
	65, 59, 65, 62, 49, 32, 13, 20,
	0, 0, 0, 0, 0, 0, 0, 0,
}
var PassedPawnPSQT_EG = [64]int{
	0, 0, 0, 0, 0, 0, 0, 0,
	14, 12, 8, 7, 3, -2, 8, 15,
	11, 16, 9, 9, 6, 5, 20, 11,
	32, 35, 31, 27, 27, 29, 42, 32,
	62, 58, 44, 46, 39, 38, 52, 47,
	104, 82, 64, 43, 37, 54, 60, 80,
	65, 67, 61, 52, 52, 53, 62, 65,
	0, 0, 0, 0, 0, 0, 0, 0,
}

var DoubledPawnPenaltyMG = 11
var DoubledPawnPenaltyEG = 20
var IsolatedPawnMG = 5
var IsolatedPawnEG = 12
var ConnectedPawnsBonusMG = 18
var ConnectedPawnsBonusEG = -2
var PhalanxPawnsBonusMG = 9
var PhalanxPawnsBonusEG = 5
var BlockedPawnBonusMG = 25
var BlockedPawnBonusEG = 15
var PawnLeverMG = -4
var PawnLeverEG = -6
var BackwardPawnMG = 4
var BackwardPawnEG = -4
var PawnStormMG = -3
var PawnProximityPenaltyMG = -14
var PawnLeverStormPenaltyMG = -9
var KnightOutpostMG = 20
var KnightOutpostEG = 15
var KnightCanAttackPieceMG = -2
var KnightCanAttackPieceEG = 1
var BishopOutpostMG = 15
var BishopPairBonusMG = 0
var BishopPairBonusEG = 62
var BishopXrayKingMG = -3
var BishopXrayRookMG = 26
var BishopXrayQueenMG = 18
var StackedRooksMG = 13
var RookXrayQueenMG = 20
var ConnectedRooksBonusMG = 15
var RookSemiOpenFileBonusMG = 13
var RookOpenFileBonusMG = 24
var SeventhRankBonusEG = 19
var CentralizedQueenBonusEG = 30
var QueenInfiltrationBonusMG = -16
var QueenInfiltrationBonusEG = 45
var KingSemiOpenFilePenalty = 5
var KingOpenFilePenalty = 1
var KingMinorPieceDefenseBonus = 2
var KingPawnDefenseMG = 3

// Tempo bonus for side to move (MG and EG applied equally)
var TempoBonus = 9

var KingSafetyTable = [100]int{
	0, 1, 1, 3, 3, 5, 7, 9, 12, 15,
	18, 22, 26, 30, 35, 39, 43, 50, 55, 62,
	67, 75, 78, 85, 88, 97, 104, 113, 120, 130,
	135, 148, 164, 174, 185, 196, 207, 219, 231, 242,
	254, 266, 277, 289, 301, 313, 324, 336, 348, 360,
	371, 383, 395, 406, 418, 430, 442, 453, 465, 477,
	488, 494, 494, 500, 500, 500, 500, 500, 500, 500,
	500, 500, 500, 500, 500, 500, 500, 500, 500, 500,
	500, 500, 500, 500, 500, 500, 500, 500, 500, 500,
	500, 500, 500, 500, 500, 500, 500, 500, 500, 500,
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

func GetPiecePhase(b *gm.Board) (phase int) {
	phase += bits.OnesCount64(b.White.Knights|b.Black.Knights) * KnightPhase
	phase += bits.OnesCount64(b.White.Bishops|b.Black.Bishops) * BishopPhase
	phase += bits.OnesCount64(b.White.Rooks|b.Black.Rooks) * RookPhase
	phase += bits.OnesCount64(b.White.Queens|b.Black.Queens) * QueenPhase
	return phase
}

func countMaterial(bb *gm.Bitboards) (materialMG, materialEG int) {
	materialMG += bits.OnesCount64(bb.Pawns) * pieceValueMG[gm.PieceTypePawn]
	materialEG += bits.OnesCount64(bb.Pawns) * pieceValueEG[gm.PieceTypePawn]

	materialMG += bits.OnesCount64(bb.Knights) * pieceValueMG[gm.PieceTypeKnight]
	materialEG += bits.OnesCount64(bb.Knights) * pieceValueEG[gm.PieceTypeKnight]

	materialMG += bits.OnesCount64(bb.Bishops) * pieceValueMG[gm.PieceTypeBishop]
	materialEG += bits.OnesCount64(bb.Bishops) * pieceValueEG[gm.PieceTypeBishop]

	materialMG += bits.OnesCount64(bb.Rooks) * pieceValueMG[gm.PieceTypeRook]
	materialEG += bits.OnesCount64(bb.Rooks) * pieceValueEG[gm.PieceTypeRook]

	materialMG += bits.OnesCount64(bb.Queens) * pieceValueMG[gm.PieceTypeQueen]
	materialEG += bits.OnesCount64(bb.Queens) * pieceValueEG[gm.PieceTypeQueen]

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

func getWeakSquares(movementBB [2][5]uint64, kingInnerRing [2]uint64, wPawnAttackBB uint64, bPawnAttackBB uint64) (weakSquares [2]uint64, weakSquaresKing [2]uint64) {

	// Priority space masks (tunable): emphasize central and forward squares per side
	var wSide uint64 = 0x3c3c3c7e00
	var bSide uint64 = 0x7e3c3c3c000000

	// Iterative accumulation via for-loop per piece type (R -> B -> N).
	// Same-piece defenders cancel same-piece attackers. Pawns defend all. Q/K excluded entirely.
	// White zone vs black attackers
	var wRemainder uint64 = 0
	for i := 2; i >= 0; i-- { // indices: 2=R, 1=B, 0=N
		attackers := movementBB[1][i] // black raw attacks for piece i
		defenders := movementBB[0][i] // white raw defenses for same piece i
		attackers &^= wPawnAttackBB   // white pawns defend all
		wRemainder |= (attackers &^ defenders)
	}

	// Black zone vs white attackers
	var bRemainder uint64 = 0
	for i := 2; i >= 0; i-- { // indices: 2=R, 1=B, 0=N
		attackers := movementBB[0][i] // white raw attacks for piece i
		defenders := movementBB[1][i] // black raw defenses for same piece i
		attackers &^= bPawnAttackBB   // black pawns defend all
		bRemainder |= (attackers &^ defenders)
	}

	var wWeakSquares uint64 = wSide & wRemainder
	var bWeakSquares uint64 = bSide & bRemainder

	weakSquares[0] = wWeakSquares
	weakSquares[1] = bWeakSquares
	weakSquaresKing[0] = wWeakSquares & kingInnerRing[0]
	weakSquaresKing[1] = bWeakSquares & kingInnerRing[1]

	return weakSquares, weakSquaresKing
}

// evaluateWeakSquares computes the midgame weak-square contribution using raw attack maps.
// It treats P/N/B as strong defenders, rooks as weak defenders (half penalty), and excludes Q/K as defenders.
func weakSquaresPenalty(movementBB [2][5]uint64, kingInnerRing [2]uint64, wPawnAttackBB uint64, bPawnAttackBB uint64) (score int, weakSquares [2]uint64, weakKingSquares [2]uint64) {
	weakSquares, weakKingSquares = getWeakSquares(movementBB, kingInnerRing, wPawnAttackBB, bPawnAttackBB)

	// Uniform penalties: count weak squares and weak king-ring squares
	wWeak := weakSquares[0] &^ weakKingSquares[0]
	bWeak := weakSquares[1] &^ weakKingSquares[1]

	wWeakSquarePenalty := -bits.OnesCount64(wWeak) * weakSquaresPenaltyMG
	bWeakSquarePenalty := +bits.OnesCount64(bWeak) * weakSquaresPenaltyMG

	wWeakKingSquarePenalty := -bits.OnesCount64(weakKingSquares[0]) * weakKingSquaresPenaltyMG
	bWeakKingSquarePenalty := +bits.OnesCount64(weakKingSquares[1]) * weakKingSquaresPenaltyMG

	score = wWeakSquarePenalty + wWeakKingSquarePenalty + bWeakSquarePenalty + bWeakKingSquarePenalty
	return score, weakSquares, weakKingSquares
}

/*
	PAWN FUNCTIONS
*/

// isolatedPawnPenaltyFromBB scores isolated pawns from precomputed bitboards.
func isolatedPawnPenalty(wIsolated uint64, bIsolated uint64) (isolatedMG int, isolatedEG int) {
	wCount := bits.OnesCount64(wIsolated)
	bCount := bits.OnesCount64(bIsolated)
	isolatedMG = (bCount * IsolatedPawnMG) - (wCount * IsolatedPawnMG)
	isolatedEG = (bCount * IsolatedPawnEG) - (wCount * IsolatedPawnEG)
	return isolatedMG, isolatedEG
}

// passedPawnBonusFromBB scores passed pawns using PSQT tables from bitboards.
func passedPawnBonus(wPassed uint64, bPassed uint64) (passedMG int, passedEG int) {
	for x := wPassed; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		passedMG += PassedPawnPSQT_MG[sq]
		passedEG += PassedPawnPSQT_EG[sq]
	}
	for x := bPassed; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		revSQ := FlipView[sq]
		passedMG -= PassedPawnPSQT_MG[revSQ]
		passedEG -= PassedPawnPSQT_EG[revSQ]
	}
	return passedMG, passedEG
}

// blockedPawnBonusFromBB scores blocked advanced pawns from bitboards.
func blockedPawnBonus(wBlocked uint64, bBlocked uint64) (blockedBonusMG int, blockedBonusEG int) {
	wCount := bits.OnesCount64(wBlocked)
	bCount := bits.OnesCount64(bBlocked)
	blockedBonusMG = (wCount * BlockedPawnBonusMG) - (bCount * BlockedPawnBonusMG)
	blockedBonusEG = (wCount * BlockedPawnBonusEG) - (bCount * BlockedPawnBonusEG)
	return blockedBonusMG, blockedBonusEG
}

// backwardPawnPenalty scores backward pawns bitboards into MG/EG contributions.
func backwardPawnPenalty(wBackward uint64, bBackward uint64) (backMG int, backEG int) {
	wCount := bits.OnesCount64(wBackward)
	bCount := bits.OnesCount64(bBackward)
	backMG = (bCount * BackwardPawnMG) - (wCount * BackwardPawnMG)
	backEG = (bCount * BackwardPawnEG) - (wCount * BackwardPawnEG)
	return backMG, backEG
}

// pawnLeverReadiness scores immediate lever opportunities into MG/EG.
func pawnLeverBonus(wLever uint64, bLever uint64) (leverMG int, leverEG int) {
	wCount := bits.OnesCount64(wLever)
	bCount := bits.OnesCount64(bLever)
	leverMG = (wCount * PawnLeverMG) - (bCount * PawnLeverMG)
	leverEG = (wCount * PawnLeverEG) - (bCount * PawnLeverEG)
	return leverMG, leverEG
}

// returns MG-only contribution for storm (bonus) and proximity (penalty),
// with steeper proximity penalty if kings are on opposite wings, and extra penalty if a lever exists
// on the defender's wing among advanced ranks.
func pawnStormProximityMG(wStorm uint64, bStorm uint64, wProx uint64, bProx uint64, wLever uint64, bLever uint64, wWing uint64, bWing uint64, oppositeSides bool) (mg int) {
	wStormCnt := bits.OnesCount64(wStorm)
	bStormCnt := bits.OnesCount64(bStorm)
	wProxCnt := bits.OnesCount64(wProx)
	bProxCnt := bits.OnesCount64(bProx)

	// Storm bonus (symmetric)
	mg += (wStormCnt - bStormCnt) * PawnStormMG

	// Proximity penalty (steeper if opposite-side kings)
	if oppositeSides {
		wProxCnt = (wProxCnt*3 + 1) / 2
		bProxCnt = (bProxCnt*3 + 1) / 2
	}
	mg += (bProxCnt - wProxCnt) * PawnProximityPenaltyMG

	// Extra penalty if storm area also has immediate lever in defender wing
	wLeverStorm := bits.OnesCount64((wLever & bWing) & ranksAbove[3])
	bLeverStorm := bits.OnesCount64((bLever & wWing) & ranksBelow[4])
	mg += (bLeverStorm - wLeverStorm) * PawnLeverStormPenaltyMG
	return mg
}

func connectedOrPhalanxPawnBonus(b *gm.Board, wPawnAttackBB uint64, bPawnAttackBB uint64) (connectedMG, connectedEG, phalanxMG, phalanxEG int) {

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
		sq := bits.TrailingZeros64(x)
		bPhalanxBB = bPhalanxBB | (((PositionBB[sq-1]) & b.Black.Pawns &^ bitboardFileH) | ((PositionBB[sq+1]) & b.Black.Pawns &^ bitboardFileA))
	}

	phalanxMG += (bits.OnesCount64(wPhalanxBB&^secondRankMask) * PhalanxPawnsBonusMG) - (bits.OnesCount64(bPhalanxBB&^seventhRankMask) * PhalanxPawnsBonusMG)
	phalanxEG += (bits.OnesCount64(wPhalanxBB&^secondRankMask) * PhalanxPawnsBonusEG) - (bits.OnesCount64(bPhalanxBB&^seventhRankMask) * PhalanxPawnsBonusEG)

	return connectedMG, connectedEG, phalanxMG, phalanxEG
}

func pawnDoublingPenalties(b *gm.Board) (doubledMG, doubledEG int) {
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

/*
	KNIGHT FUNCTIONS
*/

func knightThreats(b *gm.Board) (knightThreatsMG int, knightThreatsEG int) {
	wPieces := (b.White.Bishops | b.White.Rooks | b.White.Queens)
	bPieces := (b.Black.Bishops | b.Black.Rooks | b.Black.Queens)
	for x := b.White.Knights; x != 0; x &= x - 1 {
		from := bits.TrailingZeros64(x)
		knightMoves := KnightMasks[from] &^ b.White.All
		for y := knightMoves; y != 0; y &= y - 1 {
			to := bits.TrailingZeros64(y)
			knightThreatBB := KnightMasks[to]
			if knightThreatBB&bPieces != 0 {
				bPieces &^= knightThreatBB
				knightThreatsMG += KnightCanAttackPieceMG
				knightThreatsEG += KnightCanAttackPieceEG
			}
		}
	}

	for x := b.Black.Knights; x != 0; x &= x - 1 {
		from := bits.TrailingZeros64(x)
		knightMoves := KnightMasks[from] &^ b.Black.All
		for y := knightMoves; y != 0; y &= y - 1 {
			to := bits.TrailingZeros64(y)
			knightThreatBB := KnightMasks[to]
			if knightThreatBB&wPieces != 0 {
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

func bishopPairBonuses(b *gm.Board) (bishopPairMG, bishopPairEG int) {

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

func bishopXrayAttacks(b *gm.Board) (bishopXrayMG int) {

	allPieces := b.White.All | b.Black.All
	whiteMask := allPieces &^ b.White.Knights
	blackMask := allPieces &^ b.Black.Knights
	for x := b.White.Bishops; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		occupied := (whiteMask &^ PositionBB[sq])
		bishopMovementBoard := gm.CalculateBishopMoveBitboard(uint8(sq), occupied)
		switch {
		case bishopMovementBoard&b.Black.Kings != 0:
			bishopXrayMG += BishopXrayKingMG
			continue
		case bishopMovementBoard&b.Black.Rooks != 0:
			bishopXrayMG += BishopXrayRookMG
			continue
		case bishopMovementBoard&b.Black.Queens != 0:
			bishopXrayMG += BishopXrayQueenMG
		}
	}

	for x := b.Black.Bishops; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		occupied := (blackMask &^ PositionBB[sq])
		bishopMovementBoard := gm.CalculateBishopMoveBitboard(uint8(sq), occupied)
		switch {
		case bishopMovementBoard&b.White.Kings != 0:
			bishopXrayMG -= BishopXrayKingMG
			continue
		case bishopMovementBoard&b.White.Rooks != 0:
			bishopXrayMG -= BishopXrayRookMG
			continue
		case bishopMovementBoard&b.White.Queens != 0:
			bishopXrayMG -= BishopXrayQueenMG
		}
	}

	return bishopXrayMG
}

/*
	ROOK FUNCTIONS
*/

func rookFilesBonus(b *gm.Board, openFiles uint64, wSemiOpenFiles uint64, bSemiOpenFiles uint64) (semiOpen, open int) {
	whiteRooks := b.White.Rooks
	blackRooks := b.Black.Rooks

	semiOpen = RookSemiOpenFileBonusMG * bits.OnesCount64(wSemiOpenFiles&whiteRooks)
	semiOpen -= RookSemiOpenFileBonusMG * bits.OnesCount64(bSemiOpenFiles&blackRooks)

	open = RookOpenFileBonusMG * bits.OnesCount64(openFiles&whiteRooks)
	open -= RookOpenFileBonusMG * bits.OnesCount64(openFiles&blackRooks)

	return semiOpen, open
}

func rookAttacks(b *gm.Board) (xrayMG int) {
	allPieces := b.White.All | b.Black.All
	whiteMask := allPieces &^ b.White.Knights
	blackMask := allPieces &^ b.Black.Knights
	for x := b.White.Rooks; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		occupied := whiteMask &^ PositionBB[sq]
		rookMovementBoard := gm.CalculateRookMoveBitboard(uint8(sq), occupied)
		if rookMovementBoard&b.Black.Queens != 0 {
			xrayMG += RookXrayQueenMG
		}
	}
	for x := b.Black.Rooks; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		occupied := blackMask &^ PositionBB[sq]
		rookMovementBoard := gm.CalculateRookMoveBitboard(uint8(sq), occupied)
		if rookMovementBoard&b.White.Queens != 0 {
			xrayMG -= RookXrayQueenMG
		}
	}
	return xrayMG
}

/*
	QUEEN FUNCTIONS
*/

func centralizedQueen(b *gm.Board) (centralizedBonus int) {
	if b.White.Queens&centralizedQueenSquares != 0 {
		centralizedBonus += CentralizedQueenBonusEG
	}
	if b.Black.Queens&centralizedQueenSquares != 0 {
		centralizedBonus -= CentralizedQueenBonusEG
	}
	return centralizedBonus
}

func queenInfiltrationBonus(b *gm.Board, weakSquares [2]uint64, wPawnAttackSpan uint64, bPawnAttackSpan uint64) (queenInfiltrationBonusMG int, queenInfiltrationBonusEG int) {
	// Reward infiltration only when the queen occupies enemy weak squares in the enemy half
	// and is outside the enemy pawn attack span.
	// White queen occupancy on black weak squares (enemy half, outside black pawn span)
	wOcc := b.White.Queens & weakSquares[1] & ranksAbove[4] &^ bPawnAttackSpan
	if wOcc != 0 {
		queenInfiltrationBonusMG += QueenInfiltrationBonusMG
		queenInfiltrationBonusEG += QueenInfiltrationBonusEG
	}

	// Black queen occupancy on white weak squares (enemy half, outside white pawn span)
	bOcc := b.Black.Queens & weakSquares[0] & ranksBelow[4] &^ wPawnAttackSpan
	if bOcc != 0 {
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

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func absInt16(v int16) int16 {
	if v < 0 {
		return -v
	}
	return v
}

func kingDist(a, b int) int {
	dx := absInt((a & 7) - (b & 7))
	dy := absInt((a >> 3) - (b >> 3))
	if dx > dy {
		return dx
	}
	return dy
}

func edgeDist(sq int) int {
	file := sq & 7
	rank := sq >> 3
	return min(min(file, 7-file), min(rank, 7-rank))
}

func getKingMopUpBonus(b *gm.Board, whiteWithAdvantage, hasQueen, hasRook bool) int {
	wKing := bits.TrailingZeros64(b.White.Kings)
	bKing := bits.TrailingZeros64(b.Black.Kings)

	strongKing, weakKing := wKing, bKing
	if !whiteWithAdvantage {
		strongKing, weakKing = bKing, wKing
	}

	kingDistance := kingDist(strongKing, weakKing)
	defenderEdgeDistance := edgeDist(weakKing)

	closeWeight, edgeWeight := 12, 12
	if hasQueen && !hasRook {
		closeWeight, edgeWeight = 10, 12
	} else if hasRook && !hasQueen {
		closeWeight, edgeWeight = 18, 20
	}

	bonus := (7-kingDistance)*closeWeight + (3-defenderEdgeDistance)*edgeWeight
	if bonus > 120 {
		bonus = 120
	}
	if bonus < 0 {
		bonus = 0
	}
	return bonus
}

func kingPawnDefense(b *gm.Board, kingZoneBBInner [2]uint64) int {
	wPawnsCloseToKing := min(3, bits.OnesCount64(b.White.Pawns&kingZoneBBInner[0]))
	bPawnsCloseToKing := min(3, bits.OnesCount64(b.Black.Pawns&kingZoneBBInner[1]))
	return (wPawnsCloseToKing * KingPawnDefenseMG) - (bPawnsCloseToKing * KingPawnDefenseMG)
}

func kingFilesPenalty(b *gm.Board, openFiles uint64, wSemiOpenFiles uint64, bSemiOpenFiles uint64) (score int) {
	// Get the king's files
	wKingFile := onlyFile[bits.TrailingZeros64(b.White.Kings)%8]
	bKingFile := onlyFile[bits.TrailingZeros64(b.Black.Kings)%8]

	// Left & right files of the king
	wKingFiles := ((wKingFile & ^bitboardFileA) >> 1) | ((wKingFile & ^bitboardFileH) << 1)
	bKingFiles := ((bKingFile & ^bitboardFileA) >> 1) | ((bKingFile & ^bitboardFileH) << 1)

	wSemiOpenMask := wKingFiles & bSemiOpenFiles
	wOpenMask := wKingFiles & openFiles
	bSemiOpenMask := bKingFiles & wSemiOpenFiles
	bOpenMask := bKingFiles & openFiles

	wSemiOpenFilesCount := bits.OnesCount64(wSemiOpenMask)
	wOpenFilesCount := bits.OnesCount64(wOpenMask)
	bSemiOpenFilesCount := bits.OnesCount64(bSemiOpenMask)
	bOpenFilesCount := bits.OnesCount64(bOpenMask)

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

	wCount := min(attackUnitCount[0], 99)
	bCount := min(attackUnitCount[1], 99)

	wSafety := KingSafetyTable[wCount]
	bSafety := KingSafetyTable[bCount]

	return wSafety - bSafety, (wSafety / 4) - (bSafety / 4)
}

func kingEndGameCentralizationPenalty(b *gm.Board) (kingCmdEG int) {
	return (centerManhattanDistance[bits.TrailingZeros64(b.Black.Kings)] * 10) - (centerManhattanDistance[bits.TrailingZeros64(b.White.Kings)] * 10)
}

func Evaluation(b *gm.Board, debug bool, isQuiescence bool) (score int) {
	// UPDATE & INIT VARIABLES FOR EVAL
	// Get pawn attack bitboards
	var wPawnAttackBBEast, wPawnAttackBBWest = PawnCaptureBitboards(b.White.Pawns, true)
	var bPawnAttackBBEast, bPawnAttackBBWest = PawnCaptureBitboards(b.Black.Pawns, false)

	var wPawnAttackSpan = calculatePawnFileFill((wPawnAttackBBEast|wPawnAttackBBWest), true) & ranksBelow[4]
	var bPawnAttackSpan = calculatePawnFileFill((bPawnAttackBBEast|bPawnAttackBBWest), false) & ranksAbove[4]

	// Build file-level masks for open/semi-open files (per entire file, not per square)
	var whiteFiles uint64 = 0
	for x := b.White.Pawns; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		whiteFiles |= getFileOfSquare(sq)
	}
	var blackFiles uint64 = 0
	for x := b.Black.Pawns; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		blackFiles |= getFileOfSquare(sq)
	}

	var wSemiOpenFiles = ^whiteFiles & blackFiles
	var bSemiOpenFiles = ^blackFiles & whiteFiles

	var openFiles = ^whiteFiles & ^blackFiles

	var wPawnAttackBB = wPawnAttackBBEast | wPawnAttackBBWest
	var bPawnAttackBB = bPawnAttackBBEast | bPawnAttackBBWest

	// Precompute reusable pawn structure bitboards
	var wIsolatedBB, bIsolatedBB = getIsolatedPawnsBitboards(b)
	var wPassedBB, bPassedBB = getPassedPawnsBitboards(b, wPawnAttackBB, bPawnAttackBB)
	var wBlockedBB, bBlockedBB = getBlockedPawnsBitboards(b)
	var wBackwardBB, bBackwardBB = getBackwardPawnsBitboards(b, wPawnAttackBB, bPawnAttackBB, wIsolatedBB, bIsolatedBB, wPassedBB, bPassedBB)
	var wLeverBB, bLeverBB = getPawnLeverBitboards(b, wPawnAttackBB, bPawnAttackBB)
	var wRookStackFiles, bRookStackFiles = getRookConnectedFiles(b)
	var wWingMask, bWingMask = getKingWingMasks(b)
	var wStormBB, bStormBB = getPawnStormBitboards(b, wWingMask, bWingMask)
	var wProxBB, bProxBB = getEnemyPawnProximityBitboards(b, wWingMask, bWingMask)

	// Center state and openness index for mobility scaling
	lockedCenter, openIdx := getCenterState(b, openFiles, wSemiOpenFiles, bSemiOpenFiles, wLeverBB, bLeverBB)

	// Prepare raw attack maps for weak-squares and king-safety
	var knightMovementBB = [2]uint64{}
	var bishopMovementBB = [2]uint64{}
	var rookMovementBB = [2]uint64{}
	var queenMovementBB = [2]uint64{}
	var kingMovementBB = [2]uint64{}
	var kingAttackMobilityBB = [2]uint64{} // Aggregated raw attacks (non-pawn) used to restrict king moves

	// Get outpost bitboards
	var outposts = getOutpostsBB(b, wPawnAttackBB, bPawnAttackBB)
	whiteOutposts = outposts[0]
	blackOutposts = outposts[1]

	// Get game phase
	var piecePhase = GetPiecePhase(b)
	var currPhase = TotalPhase - piecePhase

	// Simple center-based scale factors (percent) moved to helper for clarity
	knightMobilityScale, bishopMobbilityScale, bishopPairScaleMG := getCenterMobilityScales(lockedCenter, openIdx)

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
		println("Pawn attack spans: ", wPawnAttackSpan, " <||> ", bPawnAttackSpan)
		println("Pawn attacks: ", wPawnAttackBB, " <||> ", bPawnAttackBB)
		println("Pawn isolated: ", wIsolatedBB, " <||> ", bIsolatedBB)
		println("Pawn passed: ", wPassedBB, " <||> ", bPassedBB)
		println("Pawn blocked: ", wBlockedBB, " <||> ", bBlockedBB)
		println("Pawn backward: ", wBackwardBB, " <||> ", bBackwardBB)
		println("Pawn levers: ", wLeverBB, " <||> ", bLeverBB)
		println("Rook stacks (files): ", wRookStackFiles, " <||> ", bRookStackFiles)
		println("King wings: ", wWingMask, " <||> ", bWingMask)
		println("Pawn storm: ", wStormBB, " <||> ", bStormBB)
		println("Pawn proximity: ", wProxBB, " <||> ", bProxBB)
		fmt.Printf("Center locked: %v, openIdx: %.2f\n", lockedCenter, openIdx)
		println("Open files: ", openFiles)
		println("Semi-Open files: ", wSemiOpenFiles, " <||> ", bSemiOpenFiles)
		println("Outposts: ", outposts[0], " <||> ", outposts[1])
		println("King safety tables inner: ", innerKingSafetyZones[0], " <||> ", innerKingSafetyZones[1])
		println("King safety tables outer: ", outerKingSafetyZones[0], " <||> ", outerKingSafetyZones[1])
		println("################### TACTICAL PIECE VALUES ###################")
	}

	wPieceCount := bits.OnesCount64(b.White.Bishops | b.White.Knights | b.White.Rooks | b.White.Queens)
	bPieceCount := bits.OnesCount64(b.Black.Bishops | b.Black.Knights | b.Black.Rooks | b.Black.Queens)

	for _, piece := range pieceList {
		switch piece {
		case gm.PieceTypePawn:
			pawnPsqtMG, pawnPsqtEG := countPieceTables(&b.White.Pawns, &b.Black.Pawns, &PSQT_MG[gm.PieceTypePawn], &PSQT_EG[gm.PieceTypePawn])
			isolatedMG, isolatedEG := isolatedPawnPenalty(wIsolatedBB, bIsolatedBB)
			doubledMG, doubledEG := pawnDoublingPenalties(b)
			connectedMG, connectedEG, phalanxMG, phalanxEG := connectedOrPhalanxPawnBonus(b, wPawnAttackBB, bPawnAttackBB)
			passedMG, passedEG := passedPawnBonus(wPassedBB, bPassedBB)
			blockedPawnBonusMG, blockedPawnBonusEG := blockedPawnBonus(wBlockedBB, bBlockedBB)
			backwardMG, backwardEG := backwardPawnPenalty(wBackwardBB, bBackwardBB)
			leverMG, leverEG := pawnLeverBonus(wLeverBB, bLeverBB)

			// Transition from more complex pawn structures to just prioritizing passers as endgame nears...
			// Not sure if it's good, but it's something?
			// Pawn storm and enemy pawn proximity (MG only)
			oppositeSides := (wWingMask != bWingMask)
			stormMG := pawnStormProximityMG(wStormBB, bStormBB, wProxBB, bProxBB, wLeverBB, bLeverBB, wWingMask, bWingMask, oppositeSides)
			pawnMG += pawnPsqtMG + passedMG + doubledMG + isolatedMG + connectedMG + phalanxMG + blockedPawnBonusMG + backwardMG + leverMG + stormMG
			pawnEG += pawnPsqtEG + passedEG + doubledEG + isolatedEG + connectedEG + phalanxEG + blockedPawnBonusEG + backwardEG + leverEG
			if debug {
				println("Pawn MG:\t", "PSQT: ", pawnPsqtMG, "\tIsolated: ", isolatedMG, "\tDoubled:", doubledMG, "\tPassed: ", passedMG, "\tConnected: ", connectedMG, "\tPhalanx: ", phalanxMG, "\tBlocked: ", blockedPawnBonusMG, "\tBackward:", backwardMG, "\tLever:", leverMG, "\tStorm: ", stormMG)
				println("Pawn EG:\t", "PSQT: ", pawnPsqtEG, "\tIsolated: ", isolatedEG, "\tDoubled:", doubledEG, "\tPassed: ", passedEG, "\tConnected: ", connectedEG, "\tPhalanx: ", phalanxEG, "\tBlocked: ", blockedPawnBonusEG, "\tBackward:", backwardEG, "\tLever:", leverEG)
			}
		case gm.PieceTypeKnight:
			knightPsqtMG, knightPsqtEG := countPieceTables(&b.White.Knights, &b.Black.Knights, &PSQT_MG[gm.PieceTypeKnight], &PSQT_EG[gm.PieceTypeKnight])
			var knightMobilityMG, knightMobilityEG int
			for x := b.White.Knights; x != 0; x &= x - 1 {
				square := bits.TrailingZeros64(x)
				attackedSquares := KnightMasks[square]
				kingAttackMobilityBB[0] |= attackedSquares &^ b.White.All
				knightMovementBB[0] |= attackedSquares
				mobilitySquares := attackedSquares &^ bPawnAttackBB &^ b.White.All
				popCnt := bits.OnesCount64(mobilitySquares)
				knightMobilityMG += popCnt * mobilityValueMG[gm.PieceTypeKnight]
				knightMobilityEG += popCnt * mobilityValueEG[gm.PieceTypeKnight]
				attackUnitCounts[0] += (bits.OnesCount64(attackedSquares&innerKingSafetyZones[1]) * attackerInner[gm.PieceTypeKnight])
				attackUnitCounts[0] += (bits.OnesCount64(attackedSquares&outerKingSafetyZones[1]) * attackerOuter[gm.PieceTypeKnight])
			}
			for x := b.Black.Knights; x != 0; x &= x - 1 {
				square := bits.TrailingZeros64(x)
				attackedSquares := KnightMasks[square]
				kingAttackMobilityBB[1] |= attackedSquares &^ b.Black.All
				knightMovementBB[1] |= attackedSquares
				mobilitySquares := attackedSquares &^ wPawnAttackBB &^ b.Black.All
				popCnt := bits.OnesCount64(mobilitySquares)
				knightMobilityMG -= popCnt * mobilityValueMG[gm.PieceTypeKnight]
				knightMobilityEG -= popCnt * mobilityValueEG[gm.PieceTypeKnight]
				attackUnitCounts[1] += (bits.OnesCount64(attackedSquares&innerKingSafetyZones[0]) * attackerInner[gm.PieceTypeKnight])
				attackUnitCounts[1] += (bits.OnesCount64(attackedSquares&outerKingSafetyZones[0]) * attackerOuter[gm.PieceTypeKnight])
			}
			var knightOutpostMG = (KnightOutpostMG * bits.OnesCount64(b.White.Knights&whiteOutposts)) - (KnightOutpostMG * bits.OnesCount64(b.Black.Knights&blackOutposts))
			var knightOutpostEG = (KnightOutpostEG * bits.OnesCount64(b.White.Knights&whiteOutposts)) - (KnightOutpostEG * bits.OnesCount64(b.Black.Knights&blackOutposts))
			var knightThreatsBonusMG, knightThreatsBonusEG = knightThreats(b)
			// Scale knight mobility by center state (simple integer scale)
			knightMobilityMG = (knightMobilityMG * knightMobilityScale) / 100
			knightMG += knightPsqtMG + knightOutpostMG + knightMobilityMG + knightThreatsBonusMG
			knightEG += knightPsqtEG + knightOutpostEG + knightMobilityEG + knightThreatsBonusEG
			if debug {
				println("Knight MG:\t", "PSQT: ", knightPsqtMG, "\tMobility: ", knightMobilityMG, "\tOutpost:", knightOutpostMG, "\tKnight threats: ", knightThreatsBonusMG)
				println("Knight EG:\t", "PSQT: ", knightPsqtEG, "\tMobility: ", knightMobilityEG, "\tOutpost:", knightOutpostEG, "\tKnight threats: ", knightThreatsBonusEG)
			}
		case gm.PieceTypeBishop:
			bishopPsqtMG, bishopPsqtEG := countPieceTables(&b.White.Bishops, &b.Black.Bishops, &PSQT_MG[gm.PieceTypeBishop], &PSQT_EG[gm.PieceTypeBishop])

			var bishopMobilityMG, bishopMobilityEG int
			allPieces := b.White.All | b.Black.All
			for x := b.White.Bishops; x != 0; x &= x - 1 {
				square := bits.TrailingZeros64(x)
				occupied := allPieces &^ PositionBB[square]
				attackedSquares := gm.CalculateBishopMoveBitboard(uint8(square), occupied)
				kingAttackMobilityBB[0] |= attackedSquares &^ b.White.All
				bishopMovementBB[0] |= attackedSquares
				mobilitySquares := attackedSquares &^ bPawnAttackBB &^ b.White.All
				popCnt := bits.OnesCount64(mobilitySquares)
				bishopMobilityMG += popCnt * mobilityValueMG[gm.PieceTypeBishop]
				bishopMobilityEG += popCnt * mobilityValueEG[gm.PieceTypeBishop]
				innerAttacks := attackedSquares & innerKingSafetyZones[1]
				outerAttacks := attackedSquares & outerKingSafetyZones[1]
				attackUnitCounts[0] += bits.OnesCount64(innerAttacks) * attackerInner[gm.PieceTypeBishop]
				attackUnitCounts[0] += bits.OnesCount64(outerAttacks) * attackerOuter[gm.PieceTypeBishop]
			}
			for x := b.Black.Bishops; x != 0; x &= x - 1 {
				square := bits.TrailingZeros64(x)
				occupied := allPieces &^ PositionBB[square]
				attackedSquares := gm.CalculateBishopMoveBitboard(uint8(square), occupied)
				kingAttackMobilityBB[1] |= attackedSquares &^ b.Black.All
				bishopMovementBB[1] |= attackedSquares
				mobilitySquares := attackedSquares &^ wPawnAttackBB &^ b.Black.All
				popCnt := bits.OnesCount64(mobilitySquares)
				bishopMobilityMG -= popCnt * mobilityValueMG[gm.PieceTypeBishop]
				bishopMobilityEG -= popCnt * mobilityValueEG[gm.PieceTypeBishop]
				innerAttacks := attackedSquares & innerKingSafetyZones[0]
				outerAttacks := attackedSquares & outerKingSafetyZones[0]
				attackUnitCounts[1] += bits.OnesCount64(innerAttacks) * attackerInner[gm.PieceTypeBishop]
				attackUnitCounts[1] += bits.OnesCount64(outerAttacks) * attackerOuter[gm.PieceTypeBishop]
			}
			var bishopOutpostMG = (BishopOutpostMG * bits.OnesCount64(b.White.Bishops&whiteOutposts)) - (BishopOutpostMG * bits.OnesCount64(b.Black.Bishops&blackOutposts))
			var bishopPairMG, bishopPairEG = bishopPairBonuses(b)
			var bishopXrayAttackMG = bishopXrayAttacks(b)

			// Scale bishop mobility and bishop-pair by center state (simple integer scale)
			bishopMobilityMG = (bishopMobilityMG * bishopMobbilityScale) / 100
			bishopPairMG = (bishopPairMG * bishopPairScaleMG) / 100
			bishopMG += bishopPsqtMG + bishopMobilityMG + bishopPairMG + bishopOutpostMG + bishopXrayAttackMG
			bishopEG += bishopPsqtEG + bishopMobilityEG + bishopPairEG
			if debug {
				println("Bishop MG:\t", "PSQT: ", bishopPsqtMG, "\tMobility: ", bishopMobilityMG, "\tOutpost:", bishopOutpostMG, "\tPair: ", bishopPairMG, "\tBishop attacks: ", bishopXrayAttackMG)
				println("Bishop EG:\t", "PSQT: ", bishopPsqtEG, "\tMobility: ", bishopMobilityEG, "\t\t\tPair: ", bishopPairEG)
			}
		case gm.PieceTypeRook:
			var rookPsqtMG, rookPsqtEG = countPieceTables(&b.White.Rooks, &b.Black.Rooks, &PSQT_MG[gm.PieceTypeRook], &PSQT_EG[gm.PieceTypeRook])
			var rookSemiOpenMG, rookOpenMG = rookFilesBonus(b, openFiles, wSemiOpenFiles, bSemiOpenFiles)
			var rookSeventhRankBonus = (bits.OnesCount64(b.White.Rooks&seventhRankMask) * SeventhRankBonusEG) - (bits.OnesCount64(b.Black.Rooks&secondRankMask) * SeventhRankBonusEG)
			var rookMobilityMG, rookMobilityEG int
			allPieces := b.White.All | b.Black.All
			for x := b.White.Rooks; x != 0; x &= x - 1 {
				square := bits.TrailingZeros64(x)
				occupied := allPieces &^ PositionBB[square]
				attackedSquares := gm.CalculateRookMoveBitboard(uint8(square), occupied)
				kingAttackMobilityBB[0] |= attackedSquares &^ b.White.All
				rookMovementBB[0] |= attackedSquares
				mobilitySquares := attackedSquares &^ bPawnAttackBB &^ b.White.All
				popCnt := bits.OnesCount64(mobilitySquares)
				rookMobilityMG += popCnt * mobilityValueMG[gm.PieceTypeRook]
				rookMobilityEG += popCnt * mobilityValueEG[gm.PieceTypeRook]
				innerAttacks := attackedSquares & innerKingSafetyZones[1]
				outerAttacks := attackedSquares & outerKingSafetyZones[1]
				attackUnitCounts[0] += bits.OnesCount64(innerAttacks) * attackerInner[gm.PieceTypeRook]
				attackUnitCounts[0] += bits.OnesCount64(outerAttacks) * attackerOuter[gm.PieceTypeRook]
			}
			for x := b.Black.Rooks; x != 0; x &= x - 1 {
				square := bits.TrailingZeros64(x)
				occupied := allPieces &^ PositionBB[square]
				attackedSquares := gm.CalculateRookMoveBitboard(uint8(square), occupied)
				kingAttackMobilityBB[1] |= attackedSquares &^ b.Black.All
				rookMovementBB[1] |= attackedSquares
				mobilitySquares := attackedSquares &^ wPawnAttackBB &^ b.Black.All
				popCnt := bits.OnesCount64(mobilitySquares)
				rookMobilityMG -= popCnt * mobilityValueMG[gm.PieceTypeRook]
				rookMobilityEG -= popCnt * mobilityValueEG[gm.PieceTypeRook]
				innerAttacks := attackedSquares & innerKingSafetyZones[0]
				outerAttacks := attackedSquares & outerKingSafetyZones[0]
				attackUnitCounts[1] += bits.OnesCount64(innerAttacks) * attackerInner[gm.PieceTypeRook]
				attackUnitCounts[1] += bits.OnesCount64(outerAttacks) * attackerOuter[gm.PieceTypeRook]
			}

			rookXrayAttack := rookAttacks(b)
			// Stacked rooks (MG only)
			rookStackedMG := scoreRookStacksMG(wRookStackFiles, bRookStackFiles)
			rookMG += rookPsqtMG + rookMobilityMG + rookOpenMG + rookSemiOpenMG + rookXrayAttack + rookStackedMG
			rookEG += rookPsqtEG + rookMobilityEG + rookSeventhRankBonus
			if debug {
				println("Rook MG:\t", "PSQT: ", rookPsqtMG, "\tMobility: ", rookMobilityMG, "\tOpen: ", rookOpenMG, "\tSemiOpen: ", rookSemiOpenMG, "\tRook Xray: ", rookXrayAttack, "\tRook stacks: ", rookStackedMG)
				println("Rook EG:\t", "PSQT: ", rookPsqtEG, "\tMobility: ", rookMobilityEG, "\tSeventh: ", rookSeventhRankBonus)
			}
		case gm.PieceTypeQueen:
			queenPsqtMG, queenPsqtEG := countPieceTables(&b.White.Queens, &b.Black.Queens, &PSQT_MG[gm.PieceTypeQueen], &PSQT_EG[gm.PieceTypeQueen])
			var queenMobilityMG, queenMobilityEG int
			allPieces := b.White.All | b.Black.All
			for x := b.White.Queens; x != 0; x &= x - 1 {
				square := bits.TrailingZeros64(x)
				occupied := allPieces &^ PositionBB[square]
				bishopAttacks := gm.CalculateBishopMoveBitboard(uint8(square), occupied)
				rookAttacks := gm.CalculateRookMoveBitboard(uint8(square), occupied)
				attackedSquares := (bishopAttacks | rookAttacks)
				kingAttackMobilityBB[0] |= attackedSquares &^ b.White.All
				queenMovementBB[0] |= attackedSquares
				mobilitySquares := attackedSquares &^ bPawnAttackBB &^ b.White.All
				popCnt := bits.OnesCount64(mobilitySquares)
				queenMobilityMG += popCnt * mobilityValueMG[gm.PieceTypeQueen]
				queenMobilityEG += popCnt * mobilityValueEG[gm.PieceTypeQueen]
				innerAttacks := attackedSquares & innerKingSafetyZones[1]
				outerAttacks := attackedSquares & outerKingSafetyZones[1]
				attackUnitCounts[0] += bits.OnesCount64(innerAttacks) * attackerInner[gm.PieceTypeQueen]
				attackUnitCounts[0] += bits.OnesCount64(outerAttacks) * attackerOuter[gm.PieceTypeQueen]
			}
			for x := b.Black.Queens; x != 0; x &= x - 1 {
				square := bits.TrailingZeros64(x)
				occupied := allPieces &^ PositionBB[square]
				bishopAttacks := gm.CalculateBishopMoveBitboard(uint8(square), occupied)
				rookAttacks := gm.CalculateRookMoveBitboard(uint8(square), occupied)
				attackedSquares := (bishopAttacks | rookAttacks)
				kingAttackMobilityBB[1] |= attackedSquares &^ b.Black.All
				queenMovementBB[1] |= attackedSquares
				mobilitySquares := attackedSquares &^ wPawnAttackBB &^ b.Black.All
				popCnt := bits.OnesCount64(mobilitySquares)
				queenMobilityMG -= popCnt * mobilityValueMG[gm.PieceTypeQueen]
				queenMobilityEG -= popCnt * mobilityValueEG[gm.PieceTypeQueen]
				innerAttacks := attackedSquares & innerKingSafetyZones[0]
				outerAttacks := attackedSquares & outerKingSafetyZones[0]
				attackUnitCounts[1] += bits.OnesCount64(innerAttacks) * attackerInner[gm.PieceTypeQueen]
				attackUnitCounts[1] += bits.OnesCount64(outerAttacks) * attackerOuter[gm.PieceTypeQueen]
			}

			var centralizedQueenBonus = centralizedQueen(b)

			queenMG += queenPsqtMG + queenMobilityMG
			queenEG += queenPsqtEG + queenMobilityEG + centralizedQueenBonus

			if debug {
				println("Queen MG:\t", "PSQT: ", queenPsqtMG, "\tMobility: ", queenMobilityMG)
				println("Queen EG:\t", "PSQT: ", queenPsqtEG, "\tMobility: ", queenMobilityEG, "\tCentralized Queen bonus", centralizedQueenBonus)
			}
		case gm.PieceTypeKing:
			kingPsqtMG, kingPsqtEG := countPieceTables(&b.White.Kings, &b.Black.Kings, &PSQT_MG[gm.PieceTypeKing], &PSQT_EG[gm.PieceTypeKing])
			kingAttackPenaltyMG, kingAttackPenaltyEG := kingAttackCountPenalty(&attackUnitCounts)
			kingPawnShieldPenaltyMG := kingFilesPenalty(b, openFiles, wSemiOpenFiles, bSemiOpenFiles)
			kingCentralManhattanPenalty := 0
			kingMopUpBonus := 0
			KingMinorPieceDefenseBonusMG := kingMinorPieceDefences(innerKingSafetyZones, knightMovementBB, bishopMovementBB)
			kingPawnDefenseMG := kingPawnDefense(b, innerKingSafetyZones)

			kingMovementBB[0] = (innerKingSafetyZones[0] &^ b.White.All) &^ kingAttackMobilityBB[1]
			kingMovementBB[1] = (innerKingSafetyZones[1] &^ b.Black.All) &^ kingAttackMobilityBB[0]
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
				if wPieceCount > 0 && bPieceCount == 0 {
					kingMopUpBonus = getKingMopUpBonus(b, true, b.White.Queens > 0, b.White.Rooks > 0)
				} else if wPieceCount == 0 && bPieceCount > 0 {
					kingMopUpBonus = getKingMopUpBonus(b, false, b.Black.Queens > 0, b.Black.Rooks > 0)
					kingMopUpBonus = -kingMopUpBonus
				} else {
					kingCentralManhattanPenalty = kingEndGameCentralizationPenalty(b)
				}

			}

			kingMG += kingPsqtMG + kingAttackPenaltyMG + kingPawnShieldPenaltyMG + KingMinorPieceDefenseBonusMG + kingPawnDefenseMG
			kingEG += kingPsqtEG + kingAttackPenaltyEG + kingCentralManhattanPenalty + kingMopUpBonus
			if debug {
				println("King MG:\t", "PSQT: ", kingPsqtMG, "\tAttack: ", kingAttackPenaltyMG, "\tFile: ", kingPawnShieldPenaltyMG, "\tKing pawn defense: ", kingPawnDefenseMG, "\tMinor defense: ", KingMinorPieceDefenseBonusMG)
				println("King EG:\t", "PSQT: ", kingPsqtEG, "\tAttack: ", kingAttackPenaltyEG, "\tCentralization: ", kingCentralManhattanPenalty, "\tMop up bonus: ", kingMopUpBonus)
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
	weakSquareMG, weakSquares, weakKingSquares := weakSquaresPenalty(movementBB, innerKingSafetyZones, wPawnAttackBB, bPawnAttackBB)

	// Queen infiltration based on occupancy of enemy weak squares (outside pawn spans)
	queenInfiltrationMG, queenInfiltrationEG := queenInfiltrationBonus(b, weakSquares, wPawnAttackSpan, bPawnAttackSpan)

	/* Calculate score from all variables */
	var materialScoreMG = (wMaterialMG - bMaterialMG)
	var materialScoreEG = (wMaterialEG - bMaterialEG)

	// Tempo bonus for side to move
	var toMoveBonus = TempoBonus
	if !b.Wtomove {
		toMoveBonus = -TempoBonus
	}

	var variableScoreMG = pawnMG + knightMG + bishopMG + rookMG + queenMG + kingMG + weakSquareMG + queenInfiltrationMG + toMoveBonus
	var variableScoreEG = pawnEG + knightEG + bishopEG + rookEG + queenEG + kingEG + queenInfiltrationEG + toMoveBonus

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

	if isQuiescence && b.HalfmoveClock() > 8 {
		println("Quiescence eval: ", score, " ---- FEN: ", b.ToFen())
	}

	if !b.Wtomove {
		score = -score
	}

	return score
}
