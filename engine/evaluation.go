package engine

import (
	"cmp"
	"fmt"
	"math/bits"

	gm "chess-engine/goosemg"
)

// Board indexing and bit masks for evaluation
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

// Outpost and rank masks
var wPhalanxOrConnectedEndgameInvalidSquares uint64 = 0x000000000000ffff // ranks 1-2 (little-endian board)
var bPhalanxOrConnectedEndgameInvalidSquares uint64 = 0xffff000000000000 // ranks 7-8 for black
var wAllowedOutpostMask uint64 = 0x0000007e7e7e7e7e                      // squares where white knights/bishops can be outposts
var bAllowedOutpostMask uint64 = 0x7e7e7e7e7e000000                      // outpost mask for black
var secondRankMask uint64 = 0x000000000000ff00
var seventhRankMask uint64 = 0x00ff000000000000
var centralizedQueenSquares uint64 = 0x0000183c3c180000 // central diamond for queen bonus

// Game phase weights for interpolation
const (
	PawnPhase   = 0
	KnightPhase = 1
	BishopPhase = 1
	RookPhase   = 2
	QueenPhase  = 4
	TotalPhase  = PawnPhase*16 + KnightPhase*4 + BishopPhase*4 + RookPhase*4 + QueenPhase*2
)

// King safety attacker unit weights (inner ring and outer ring)
var attackerInner = [7]int{
	gm.PieceTypePawn: 1, gm.PieceTypeKnight: 2, gm.PieceTypeBishop: 2,
	gm.PieceTypeRook: 4, gm.PieceTypeQueen: 6, gm.PieceTypeKing: 0,
}
var attackerOuter = [7]int{
	gm.PieceTypePawn: 0, gm.PieceTypeKnight: 1, gm.PieceTypeBishop: 1,
	gm.PieceTypeRook: 2, gm.PieceTypeQueen: 2, gm.PieceTypeKing: 0,
}

// Piece-Square Tables (midgame and endgame) for all piece types
var PSQT_MG = [7][64]int{
	gm.PieceTypePawn: {
		0, 0, 0, 0, 0, 0, 0, 0,
		-46, -41, -42, -39, -40, -12, 1, -21,
		-51, -52, -45, -45, -37, -37, -20, -30,
		-46, -40, -33, -33, -23, -26, -15, -30,
		-36, -27, -27, -11, 1, 2, -4, -21,
		-33, -6, 7, 13, 27, 57, 19, -11,
		57, 54, 55, 54, 46, 32, 4, 9,
		0, 0, 0, 0, 0, 0, 0, 0,
	},
	gm.PieceTypeKnight: {
		-24, -28, -46, -30, -25, -21, -27, -40,
		-35, -32, -18, -10, -14, -12, -20, -18,
		-25, -8, -4, 6, 7, -1, -1, -17,
		-14, -1, 8, 5, 13, 10, 26, -1,
		-5, 8, 30, 35, 24, 43, 19, 22,
		-21, 12, 40, 49, 67, 64, 37, 14,
		-17, -12, 20, 33, 33, 37, -8, 3,
		-61, -6, -12, -2, 1, -6, -1, -16,
	},
	gm.PieceTypeBishop: {
		4, -2, -15, -21, -18, -8, -8, 2,
		4, 8, 11, -2, 1, 5, 20, 11,
		-2, 11, 8, 13, 10, 8, 10, 13,
		-7, 10, 15, 21, 26, 11, 10, 7,
		-4, 22, 24, 49, 34, 37, 20, 6,
		4, 18, 36, 36, 47, 55, 37, 24,
		-22, 6, 3, -7, 4, 14, -3, 8,
		-27, -8, -13, -12, -8, -21, 1, -10,
	},
	gm.PieceTypeRook: {
		-46, -41, -37, -34, -36, -40, -19, -42,
		-71, -45, -44, -43, -47, -37, -25, -51,
		-60, -46, -50, -44, -47, -48, -21, -38,
		-49, -45, -43, -35, -37, -34, -13, -29,
		-33, -21, -11, 6, 0, 7, 8, 2,
		-22, 10, 4, 25, 41, 38, 44, 20,
		-3, -5, 16, 28, 31, 37, 9, 30,
		23, 22, 19, 24, 23, 20, 21, 34,
	},
	gm.PieceTypeQueen: {
		-6, -17, -12, -3, -6, -28, -27, -12,
		-11, -4, 2, -2, -1, 7, 8, -7,
		-8, -1, -2, -4, -4, -1, 8, 7,
		-5, -3, -2, -6, -6, 10, 7, 16,
		-11, -6, -2, -1, 12, 22, 26, 26,
		-13, -6, -1, 14, 36, 58, 71, 42,
		-11, -40, 5, 5, 20, 44, -2, 27,
		0, 16, 21, 29, 36, 38, 25, 36,
	},
	gm.PieceTypeKing: {
		-4, 36, -1, -69, -23, -74, 19, 26,
		12, 0, -18, -53, -33, -39, 7, 25,
		-6, -4, -3, -11, -6, -8, 4, -15,
		-1, 8, 16, 10, 15, 12, 23, -9,
		0, 9, 16, 10, 13, 15, 15, -8,
		1, 11, 12, 9, 8, 14, 12, 0,
		-2, 6, 6, 2, 3, 4, 3, -2,
		-1, 0, 0, 2, 0, 0, 0, -2,
	},
}
var PSQT_EG = [7][64]int{
	gm.PieceTypePawn: {
		0, 0, 0, 0, 0, 0, 0, 0,
		-9, -8, -4, -2, 7, 2, -14, -29,
		-16, -17, -13, -12, -9, -12, -26, -29,
		-8, -10, -19, -18, -19, -17, -22, -21,
		3, -2, -5, -23, -16, -14, -10, -12,
		21, 22, 21, 22, 22, 11, 25, 17,
		75, 69, 58, 48, 43, 43, 55, 63,
		0, 0, 0, 0, 0, 0, 0, 0,
	},
	gm.PieceTypeKnight: {
		-29, -60, -26, -18, -20, -28, -48, -30,
		-28, -13, -13, -6, -4, -16, -18, -31,
		-38, -3, 6, 19, 18, 5, -2, -33,
		-15, 11, 32, 36, 34, 35, 16, -9,
		-11, 14, 28, 43, 48, 36, 28, -1,
		-20, 6, 24, 26, 20, 31, 12, -11,
		-25, -12, 1, 21, 19, -3, -9, -16,
		-41, -11, 2, 0, 1, 4, -4, -17,
	},
	gm.PieceTypeBishop: {
		-28, -16, -38, -14, -19, -24, -21, -20,
		-10, -20, -12, -4, -5, -18, -18, -33,
		-12, -1, 7, 10, 8, 3, -11, -11,
		-5, 6, 17, 18, 15, 14, 4, -10,
		0, 11, 12, 17, 24, 15, 19, 3,
		-5, 8, 11, 11, 13, 19, 12, 3,
		-7, 7, 10, 11, 12, 10, 12, -6,
		1, 5, 5, 8, 4, 0, 2, 2,
	},
	gm.PieceTypeRook: {
		-10, 0, 5, 5, 3, 3, -1, -18,
		-8, -10, -3, -6, -5, -11, -14, -10,
		-2, 7, 8, 5, 4, 3, -1, -8,
		13, 25, 26, 22, 20, 18, 12, 6,
		25, 27, 30, 26, 23, 20, 16, 16,
		34, 24, 32, 25, 17, 24, 14, 18,
		36, 42, 40, 41, 40, 23, 28, 22,
		32, 37, 40, 37, 38, 42, 39, 37,
	},
	gm.PieceTypeQueen: {
		-25, -35, -41, -48, -50, -39, -27, -9,
		-26, -24, -44, -27, -36, -62, -57, -17,
		-22, -17, 5, -10, -11, 1, -19, -14,
		-19, 5, 6, 38, 32, 30, 17, 20,
		-11, 14, 13, 42, 52, 57, 49, 33,
		-1, 3, 20, 29, 45, 56, 40, 38,
		7, 31, 25, 36, 57, 44, 28, 25,
		14, 26, 29, 38, 44, 43, 31, 33,
	},
	gm.PieceTypeKing: {
		-37, -29, -20, -26, -54, -14, -35, -78,
		-15, -9, -3, 4, -2, 1, -15, -35,
		-16, -3, 7, 16, 13, 6, -8, -18,
		-16, 8, 21, 28, 25, 19, 5, -18,
		-2, 22, 29, 30, 29, 26, 20, -5,
		1, 26, 25, 19, 16, 32, 31, -1,
		-12, 14, 11, 3, 5, 10, 20, -9,
		-17, -12, -6, -1, -6, -6, -6, -14,
	},
}

// Piece base values (midgame/endgame) and mobility values
var pieceValueMG = [7]int{
	gm.PieceTypeKing: 0, gm.PieceTypePawn: 88, gm.PieceTypeKnight: 316, gm.PieceTypeBishop: 331, gm.PieceTypeRook: 494, gm.PieceTypeQueen: 993,
}
var pieceValueEG = [7]int{
	gm.PieceTypeKing: 0, gm.PieceTypePawn: 111, gm.PieceTypeKnight: 305, gm.PieceTypeBishop: 333, gm.PieceTypeRook: 535, gm.PieceTypeQueen: 963,
}
var mobilityValueMG = [7]int{
	gm.PieceTypeKing: 0, gm.PieceTypePawn: 0, gm.PieceTypeKnight: 2, gm.PieceTypeBishop: 3, gm.PieceTypeRook: 2, gm.PieceTypeQueen: 1,
}
var mobilityValueEG = [7]int{
	gm.PieceTypeKing: 0, gm.PieceTypePawn: 0, gm.PieceTypeKnight: 3, gm.PieceTypeBishop: 2, gm.PieceTypeRook: 4, gm.PieceTypeQueen: 4,
}

// Passed pawn bonuses (PSQT offsets)
var PassedPawnPSQT_MG = [64]int{
	0, 0, 0, 0, 0, 0, 0, 0,
	-11, -10, -11, -11, -1, -6, 16, 14,
	-2, -4, -17, -17, -7, -6, -5, 15,
	15, 6, -8, -5, -8, -8, -2, 6,
	34, 33, 25, 17, 11, 8, 15, 17,
	68, 52, 41, 33, 24, 24, 19, 17,
	56, 53, 55, 54, 46, 31, 4, 9,
	0, 0, 0, 0, 0, 0, 0, 0,
}
var PassedPawnPSQT_EG = [64]int{
	0, 0, 0, 0, 0, 0, 0, 0,
	18, 16, 10, 9, 4, 0, 8, 15,
	13, 22, 12, 10, 9, 8, 25, 13,
	32, 36, 29, 24, 23, 30, 44, 33,
	60, 54, 40, 41, 35, 37, 48, 45,
	102, 86, 64, 41, 33, 50, 57, 78,
	68, 66, 56, 46, 43, 42, 55, 62,
	0, 0, 0, 0, 0, 0, 0, 0,
}

// Most other non-material evaluation parameters
var (
	QueenCentralizationEG = 15

	RookStackedMG     = 20
	RookConnectedMG   = 20
	RookSeventhRankEG = 10
	RookSemiOpenMG    = 13
	RookOpenMG        = 30

	KnightOutpostMG = 17
	KnightOutpostEG = 9
	KnightThreatMG  = 10
	KnightThreatEG  = 5

	BishopOutpostMG = 12
	BishopOutpostEG = 4

	BishopPairBonusMG = 10
	BishopPairBonusEG = 50

	KnightTropismMG = 1
	KnightTropismEG = 4

	BackwardPawnMG       = 1
	BackwardPawnEG       = 4
	IsolatedPawnMG       = 6
	IsolatedPawnEG       = 7
	PawnDoubledMG        = 4
	PawnDoubledEG        = 17
	PawnStormMG          = 12
	PawnFrontProximityMG = 10
	PawnConnectedMG      = 14
	PawnConnectedEG      = 8
	PawnPhalanxMG        = 6
	PawnPhalanxEG        = 10
	PawnWeakLeverMG      = 2
	PawnWeakLeverEG      = 6
	PawnBlockedMG        = -6
	PawnBlockedEG        = -7

	KingOpenFileMG          = -5
	KingSemiOpenFileMG      = -3
	KingMinorDefenseBonusMG = 7
	KingPawnDefenseBonusMG  = 6

	SpaceBonusMG            = 3 // Per safe square in our space zone
	SpaceBonusEG            = 1
	WeakKingSquarePenaltyMG = 8 // Per weak square adjacent to king
	WeakKingSquarePenaltyEG = 2

	PawnStormBaseMG             = [8]int{0, 0, 0, 5, 10, 20, 30, 0}
	PawnStormBlockedMG          = 2
	PawnStormOppositeMultiplier = 150

	WeakSquarePenaltyMG    = -3
	WeakSquarePenaltyEG    = -2
	ProtectedSquareBonusMG = 2
	ProtectedSquareBonusEG = 1

	TempoBonus = 10

	DrawDivider int32 = 8
)

// King safety table (index = "attack unit count") – higher values = worse safety
var KingSafetyTable = [100]int{
	7, 12, 10, 13, 11, 13, 13, 14, 18, 19,
	21, 23, 24, 29, 33, 36, 40, 45, 45, 54,
	57, 63, 66, 74, 76, 89, 90, 101, 105, 118,
	124, 139, 147, 160, 168, 180, 188, 201, 210, 222,
	232, 245, 256, 268, 279, 292, 302, 315, 326, 338,
	349, 361, 373, 384, 396, 408, 420, 431, 443, 456,
	466, 474, 480, 486, 483, 486, 486, 489, 489, 491,
	492, 495, 495, 497, 497, 499, 499, 499, 500, 500,
	500, 500, 500, 500, 500, 500, 500, 500, 500, 500,
	500, 500, 500, 500, 500, 500, 500, 500, 500, 500,
}

// Material imbalance constants (Kaufman-style adjustments)
var ImbalanceRefPawnCount = 5
var ImbalanceKnightPerPawnMG = 3
var ImbalanceKnightPerPawnEG = 5
var ImbalanceBishopPerPawnMG = 2
var ImbalanceBishopPerPawnEG = 5
var ImbalanceMinorsForMajorMG = -8
var ImbalanceMinorsForMajorEG = -2
var ImbalanceRedundantRookMG = 5
var ImbalanceRedundantRookEG = -10
var ImbalanceRookQueenOverlapMG = 5
var ImbalanceRookQueenOverlapEG = -8
var ImbalanceQueenManyMinorsMG = 13
var ImbalanceQueenManyMinorsEG = -17

/* ============= HELPER VARIABLES ============= */
var isolatedPawnTable = [8]uint64{
	0x0303030303030303, 0x0707070707070707, 0x0e0e0e0e0e0e0e0e, 0x1c1c1c1c1c1c1c1c,
	0x3838383838383838, 0x7070707070707070, 0xe0e0e0e0e0e0e0e0, 0xc0c0c0c0c0c0c0c0,
}

var centerManhattanDistance = [64]int{
	6, 5, 4, 3, 3, 4, 5, 6,
	5, 4, 3, 2, 2, 3, 4, 5,
	4, 3, 2, 1, 1, 2, 3, 4,
	3, 2, 1, 0, 0, 1, 2, 3,
	3, 2, 1, 0, 0, 1, 2, 3,
	4, 3, 2, 1, 1, 2, 3, 4,
	5, 4, 3, 2, 2, 3, 4, 5,
	6, 5, 4, 3, 3, 4, 5, 6,
}

var (
	wSpaceZoneMask uint64 = 0x00003c3c3c000000 // c2-f2, c3-f3, c4-f4
	bSpaceZoneMask uint64 = 0x0000003c3c3c0000 // c5-f5, c6-f6, c7-f7
)

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

/* ============= MATERIAL + PHASE ============= */

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

func countPieceTypes(b *gm.Board) (pieceCount [2][7]int) {
	// White
	pieceCount[0][gm.PieceTypePawn] = bits.OnesCount64(b.White.Pawns)
	pieceCount[0][gm.PieceTypeKnight] = bits.OnesCount64(b.White.Knights)
	pieceCount[0][gm.PieceTypeBishop] = bits.OnesCount64(b.White.Bishops)
	pieceCount[0][gm.PieceTypeRook] = bits.OnesCount64(b.White.Rooks)
	pieceCount[0][gm.PieceTypeQueen] = bits.OnesCount64(b.White.Queens)

	// Black
	pieceCount[1][gm.PieceTypePawn] = bits.OnesCount64(b.Black.Pawns)
	pieceCount[1][gm.PieceTypeKnight] = bits.OnesCount64(b.Black.Knights)
	pieceCount[1][gm.PieceTypeBishop] = bits.OnesCount64(b.Black.Bishops)
	pieceCount[1][gm.PieceTypeRook] = bits.OnesCount64(b.Black.Rooks)
	pieceCount[1][gm.PieceTypeQueen] = bits.OnesCount64(b.Black.Queens)

	return pieceCount
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

/* ============= IMBALANCE & SPACE ============= */

func materialImbalance(b *gm.Board) (imbMG int, imbEG int) {
	pieceCount := countPieceTypes(b)

	// For convenience
	const White = 0
	const Black = 1

	wp := pieceCount[White][gm.PieceTypePawn]
	wn := pieceCount[White][gm.PieceTypeKnight]
	wb := pieceCount[White][gm.PieceTypeBishop]
	wr := pieceCount[White][gm.PieceTypeRook]
	wq := pieceCount[White][gm.PieceTypeQueen]

	bp := pieceCount[Black][gm.PieceTypePawn]
	bn := pieceCount[Black][gm.PieceTypeKnight]
	bb := pieceCount[Black][gm.PieceTypeBishop]
	br := pieceCount[Black][gm.PieceTypeRook]
	bq := pieceCount[Black][gm.PieceTypeQueen]

	// Knight vs bishop vs pawn-count (Kaufman-ish)
	wPawnDelta := wp - ImbalanceRefPawnCount
	bPawnDelta := bp - ImbalanceRefPawnCount

	// More pawns -> knights slightly better, bishops slightly worse (MG/EG)
	wKnightAdjMG := wPawnDelta * wn * ImbalanceKnightPerPawnMG
	wKnightAdjEG := wPawnDelta * wn * ImbalanceKnightPerPawnEG
	wBishopAdjMG := wPawnDelta * wb * ImbalanceBishopPerPawnMG
	wBishopAdjEG := wPawnDelta * wb * ImbalanceBishopPerPawnEG

	bKnightAdjMG := bPawnDelta * bn * ImbalanceKnightPerPawnMG
	bKnightAdjEG := bPawnDelta * bn * ImbalanceKnightPerPawnEG
	bBishopAdjMG := bPawnDelta * bb * ImbalanceBishopPerPawnMG
	bBishopAdjEG := bPawnDelta * bb * ImbalanceBishopPerPawnEG

	imbMG += (wKnightAdjMG + wBishopAdjMG) - (bKnightAdjMG + bBishopAdjMG)
	imbEG += (wKnightAdjEG + wBishopAdjEG) - (bKnightAdjEG + bBishopAdjEG)

	// Groupings
	totalPawns := wp + bp
	wMinors := wn + wb
	bMinors := bn + bb

	// E2: “Bad” R+P vs B+N-style lineups
	// Idea: In crowded, queen-on positions, the side with
	// more rooks but clearly fewer minors tends to be worse.
	// -----------------------------
	if totalPawns >= 11 && (wq+bq) > 0 {
		//  White has a favorable trade
		if wr > br && wMinors < bMinors {
			outnumbered := bMinors - wMinors // how many more minors Black has
			imbMG += outnumbered * ImbalanceMinorsForMajorMG
			imbEG += outnumbered * ImbalanceMinorsForMajorEG
		}

		// Black has a favorable trade
		if br > wr && bMinors < wMinors {
			outnumbered := wMinors - bMinors // how many more minors White has
			imbMG -= outnumbered * ImbalanceMinorsForMajorMG
			imbEG -= outnumbered * ImbalanceMinorsForMajorEG
		}
	}
	// Redundant rooks
	if wr > 1 {
		// Extra rooks are a bit less valuable for the side that owns them
		extra := wr - 1
		imbMG -= extra * ImbalanceRedundantRookMG
		imbEG -= extra * ImbalanceRedundantRookEG
	}
	if br > 1 {
		extra := br - 1
		imbMG += extra * ImbalanceRedundantRookMG
		imbEG += extra * ImbalanceRedundantRookEG
	}

	// Rook–queen overlap
	if wq >= 1 && wr >= 2 {
		// Each white rook slightly overlaps with the queen's role
		imbMG -= wr * ImbalanceRookQueenOverlapMG
		imbEG -= wr * ImbalanceRookQueenOverlapEG
	}
	if bq >= 1 && br >= 2 {
		imbMG += br * ImbalanceRookQueenOverlapMG
		imbEG += br * ImbalanceRookQueenOverlapEG
	}

	if wq > 0 && wMinors >= 3 {
		extraMinors := wMinors - 2 // 0 when 2 minors, >0 when 3+
		imbMG -= extraMinors * ImbalanceQueenManyMinorsMG
		imbEG -= extraMinors * ImbalanceQueenManyMinorsEG
	}
	if bq > 0 && bMinors >= 3 {
		extraMinors := bMinors - 2
		imbMG += extraMinors * ImbalanceQueenManyMinorsMG
		imbEG += extraMinors * ImbalanceQueenManyMinorsEG
	}

	return imbMG, imbEG
}

func spaceEvaluation(
	b *gm.Board,
	wPawnAttackBB, bPawnAttackBB uint64,
	knightMovementBB, bishopMovementBB [2]uint64,
	piecePhase int,
) (spaceMG, spaceEG int) {

	if piecePhase < 6 {
		return 0, 0
	}

	wSpaceZone := wSpaceZoneMask &^ b.Black.Pawns
	bSpaceZone := bSpaceZoneMask &^ b.White.Pawns

	wControl := wPawnAttackBB | knightMovementBB[0] | bishopMovementBB[0] // Combine pawn attacks with minor piece control
	bControl := bPawnAttackBB | knightMovementBB[1] | bishopMovementBB[1]

	wSafe := wSpaceZone & wControl &^ bPawnAttackBB // Safe squares in our own zone
	bSafe := bSpaceZone & bControl &^ wPawnAttackBB

	wSafe |= wSpaceZone & b.White.All // Space we occupy
	bSafe |= bSpaceZone & b.Black.All

	wCount := bits.OnesCount64(wSafe)
	bCount := bits.OnesCount64(bSafe)

	spaceMG = (wCount - bCount) * SpaceBonusMG
	spaceEG = (wCount - bCount) * SpaceBonusEG

	return spaceMG, spaceEG
}

func weakKingSquaresPenalty(
	b *gm.Board,
	wPawnAttackBB, bPawnAttackBB uint64,
	kingInnerRing [2]uint64,
) (penaltyMG, penaltyEG int) {
	wWeakKingSquares := kingInnerRing[0] &^ wPawnAttackBB &^ b.White.All
	bWeakKingSquares := kingInnerRing[1] &^ bPawnAttackBB &^ b.Black.All

	wCount := bits.OnesCount64(wWeakKingSquares)
	bCount := bits.OnesCount64(bWeakKingSquares)

	penaltyMG = (bCount - wCount) * WeakKingSquarePenaltyMG
	penaltyEG = (bCount - wCount) * WeakKingSquarePenaltyEG

	return penaltyMG, penaltyEG
}

/* ============= PAWN FUNCTIONS ============= */

func isolatedPawnPenalty(wIsolated uint64, bIsolated uint64) (isolatedMG int, isolatedEG int) {
	wCount := bits.OnesCount64(wIsolated)
	bCount := bits.OnesCount64(bIsolated)
	isolatedMG = (bCount * IsolatedPawnMG) - (wCount * IsolatedPawnMG)
	isolatedEG = (bCount * IsolatedPawnEG) - (wCount * IsolatedPawnEG)
	return isolatedMG, isolatedEG
}

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

func blockedPawnBonus(wBlocked uint64, bBlocked uint64) (blockedBonusMG int, blockedBonusEG int) {
	wCount := bits.OnesCount64(wBlocked)
	bCount := bits.OnesCount64(bBlocked)
	blockedBonusMG = (wCount * PawnBlockedMG) - (bCount * PawnBlockedMG)
	blockedBonusEG = (wCount * PawnBlockedEG) - (bCount * PawnBlockedEG)
	return blockedBonusMG, blockedBonusEG
}

func backwardPawnPenalty(wBackward uint64, bBackward uint64) (backMG int, backEG int) {
	wCount := bits.OnesCount64(wBackward)
	bCount := bits.OnesCount64(bBackward)
	backMG = (bCount * BackwardPawnMG) - (wCount * BackwardPawnMG)
	backEG = (bCount * BackwardPawnEG) - (wCount * BackwardPawnEG)
	return backMG, backEG
}

func pawnWeakLeverPenalty(wWeak uint64, bWeak uint64) (mg int, eg int) {
	wCount := bits.OnesCount64(wWeak)
	bCount := bits.OnesCount64(bWeak)
	diffMG := (bCount - wCount) * PawnWeakLeverMG
	diffEG := (bCount - wCount) * PawnWeakLeverEG
	return diffMG, diffEG
}

func evaluatePawnStorm(b *gm.Board) (stormMG int) {
	// Get king squares and files
	wKingSq := bits.TrailingZeros64(b.White.Kings)
	bKingSq := bits.TrailingZeros64(b.Black.Kings)
	wKingFile := wKingSq % 8
	bKingFile := bKingSq % 8

	// Determine castling "side" roughly by file
	wQueenside := wKingFile <= 2 // a–c files
	wKingside := wKingFile >= 5  // f–h files
	bQueenside := bKingFile <= 2
	bKingside := bKingFile >= 5

	// 1) If both kings are in the center (d/e) there is no real storm dynamic.
	if !wQueenside && !wKingside && !bQueenside && !bKingside {
		return 0
	}

	// 2) If both kings are castled to the SAME wing, don't treat pawn pushes as a storm:
	//    advancing pawns in front of your own king should mostly be handled
	//    by king-safety / pawn-shield terms, not rewarded here.
	sameSide := (wQueenside && bQueenside) || (wKingside && bKingside)
	if sameSide {
		return 0
	}

	// 3) Opposite-side castling?
	oppositeSide := (wQueenside && bKingside) || (wKingside && bQueenside)

	// 4) Build "king file zones" (three files around each king).
	wKingZone := getKingFileZone(wKingFile)
	bKingZone := getKingFileZone(bKingFile)

	// 5) White's storm: white pawns in the zone around Black's king.
	var wStormScore int
	for x := b.White.Pawns & bKingZone; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		rank := sq / 8 // 0..7 from White's POV
		bonus := PawnStormBaseMG[rank]

		if bonus == 0 {
			continue // ignore very early or irrelevant ranks
		}

		// Reduce storm value if this pawn is blocked by a black pawn directly ahead.
		// (White pawns move "up" = +8).
		if PositionBB[sq+8]&b.Black.Pawns != 0 {
			bonus -= PawnStormBlockedMG
		}

		if bonus > 0 {
			wStormScore += bonus
		}
	}

	// 6) Black's storm: black pawns in the zone around White's king.
	var bStormScore int
	for x := b.Black.Pawns & wKingZone; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		rank := sq / 8       // 0..7 from White's POV
		sideRank := 7 - rank // flip for Black's perspective
		bonus := PawnStormBaseMG[sideRank]

		if bonus == 0 {
			continue
		}

		// Black pawns move "down" = -8.
		if PositionBB[sq-8]&b.White.Pawns != 0 {
			bonus -= PawnStormBlockedMG
		}

		if bonus > 0 {
			bStormScore += bonus
		}
	}

	stormMG = wStormScore - bStormScore

	// 7) Amplify in opposite-side castling, where storms are truly lethal.
	if oppositeSide {
		stormMG = (stormMG * PawnStormOppositeMultiplier) / 100
	}

	return stormMG
}

func connectedOrPhalanxPawnBonus(b *gm.Board, wPawnAttackBB uint64, bPawnAttackBB uint64) (connectedMG, connectedEG, phalanxMG, phalanxEG int) {

	var wConnectedMG = bits.OnesCount64(b.White.Pawns & wPawnAttackBB)
	var wConnectedEG = bits.OnesCount64((b.White.Pawns & wPawnAttackBB) &^ wPhalanxOrConnectedEndgameInvalidSquares)
	var bConnectedMG = bits.OnesCount64(b.Black.Pawns & bPawnAttackBB)
	var bConnectedEG = bits.OnesCount64((b.Black.Pawns & bPawnAttackBB) &^ bPhalanxOrConnectedEndgameInvalidSquares)
	connectedMG = (wConnectedMG * PawnConnectedMG) - (bConnectedMG * PawnConnectedMG)
	connectedEG = (wConnectedEG * PawnConnectedEG) - (bConnectedEG * PawnConnectedEG)
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

	phalanxMG += (bits.OnesCount64(wPhalanxBB&^secondRankMask) * PawnPhalanxMG) - (bits.OnesCount64(bPhalanxBB&^seventhRankMask) * PawnPhalanxMG)
	phalanxEG += (bits.OnesCount64(wPhalanxBB&^secondRankMask) * PawnPhalanxEG) - (bits.OnesCount64(bPhalanxBB&^seventhRankMask) * PawnPhalanxEG)

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

	doubledMG = (bDoubledPawnCount * PawnDoubledMG) - (wDoubledPawnCount * PawnDoubledMG)
	doubledEG = (bDoubledPawnCount * PawnDoubledEG) - (wDoubledPawnCount * PawnDoubledEG)
	return doubledMG, doubledEG
}

/* ============= KNIGHT FUNCTIONS ============= */

func knightThreats(b *gm.Board) (threatsMG int, threatsEG int) {
	// Targets: pieces worth more than a knight
	bTargets := b.Black.Rooks | b.Black.Queens
	wTargets := b.White.Rooks | b.White.Queens

	for x := b.White.Knights; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		attacks := KnightMasks[sq]

		// Count attacked high-value pieces (don't double count same piece from multiple knights)
		threatened := attacks & bTargets
		if threatened != 0 {
			bTargets &^= threatened // Remove to avoid double counting
			threatsMG += KnightThreatMG
			threatsEG += KnightThreatEG
		}
	}

	for x := b.Black.Knights; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		attacks := KnightMasks[sq]

		threatened := attacks & wTargets
		if threatened != 0 {
			wTargets &^= threatened
			threatsMG -= KnightThreatMG
			threatsEG -= KnightThreatEG
		}
	}

	return threatsMG, threatsEG
}

func knightKingTropism(b *gm.Board) (tropismMG int, tropismEG int) {
	wKingSq := bits.TrailingZeros64(b.White.Kings)
	bKingSq := bits.TrailingZeros64(b.Black.Kings)

	for x := b.White.Knights; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		dist := chebyshevDistance(sq, bKingSq)
		// Max bonus when distance is 1-2 (striking range), decreasing with distance
		if dist <= 6 {
			tropismMG += (7 - dist) * KnightTropismMG
			tropismEG += (7 - dist) * KnightTropismEG
		}
	}

	for x := b.Black.Knights; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		dist := chebyshevDistance(sq, wKingSq)
		if dist <= 6 {
			tropismMG -= (7 - dist) * KnightTropismMG
			tropismEG -= (7 - dist) * KnightTropismEG
		}
	}

	return tropismMG, tropismEG
}

/* ============= BISHOP FUNCTIONS ============= */

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

/* ============= ROOK FUNCTIONS ============= */

func rookSeventhRankBonus(b *gm.Board) (bonusEG int) {
	wRooksOnSeventh := bits.OnesCount64(b.White.Rooks & seventhRankMask)
	bRooksOnSecond := bits.OnesCount64(b.Black.Rooks & secondRankMask)

	// Base bonus per rook
	bonusEG = (wRooksOnSeventh - bRooksOnSecond) * RookSeventhRankEG

	// Extra bonus for doubled rooks on 7th (the "pigs")
	if wRooksOnSeventh >= 2 {
		bonusEG += RookSeventhRankEG * 2
	}
	if bRooksOnSecond >= 2 {
		bonusEG -= RookSeventhRankEG * 2
	}

	return bonusEG
}

func rookFilesBonus(b *gm.Board, openFiles uint64, wSemiOpenFiles uint64, bSemiOpenFiles uint64) (semiOpen, open int) {
	whiteRooks := b.White.Rooks
	blackRooks := b.Black.Rooks

	semiOpen += RookSemiOpenMG * bits.OnesCount64(wSemiOpenFiles&whiteRooks)
	semiOpen -= RookSemiOpenMG * bits.OnesCount64(bSemiOpenFiles&blackRooks)

	open += RookOpenMG * bits.OnesCount64(openFiles&whiteRooks)
	open -= RookOpenMG * bits.OnesCount64(openFiles&blackRooks)

	return semiOpen, open
}

func rookStackBonusMG(wFiles uint64, bFiles uint64) (mg int) {
	wCount := bits.OnesCount64(wFiles) / 8
	bCount := bits.OnesCount64(bFiles) / 8
	mg = (wCount * RookStackedMG) - (bCount * RookStackedMG)
	return mg
}

/* ============= QUEEN FUNCTIONS ============= */

func centralizedQueen(b *gm.Board) (centralizedBonus int) {
	if b.White.Queens&centralizedQueenSquares != 0 {
		centralizedBonus += QueenCentralizationEG
	}
	if b.Black.Queens&centralizedQueenSquares != 0 {
		centralizedBonus -= QueenCentralizationEG
	}
	return centralizedBonus
}

/* ============= KING FUNCTIONS ============= */

func kingMinorPieceDefences(kingInnerRing [2]uint64, knightMovementBB [2]uint64, bishopMovementBB [2]uint64) int {
	wDefendingPiecesCount := bits.OnesCount64(kingInnerRing[0] & (knightMovementBB[0] | bishopMovementBB[0]))
	bDefendingPiecesCount := bits.OnesCount64(kingInnerRing[1] & (knightMovementBB[1] | bishopMovementBB[1]))

	return (wDefendingPiecesCount * KingMinorDefenseBonusMG) - (bDefendingPiecesCount * KingMinorDefenseBonusMG)
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
	if !whiteWithAdvantage {
		bonus = -bonus
	}
	return bonus
}

func kingPawnDefense(b *gm.Board, kingZoneBBInner [2]uint64) int {
	wPawnsCloseToKing := min(3, bits.OnesCount64(b.White.Pawns&kingZoneBBInner[0]))
	bPawnsCloseToKing := min(3, bits.OnesCount64(b.Black.Pawns&kingZoneBBInner[1]))
	return (wPawnsCloseToKing * KingPawnDefenseBonusMG) - (bPawnsCloseToKing * KingPawnDefenseBonusMG)
}

func kingFilesPenalty(b *gm.Board, openFiles uint64, wSemiOpenFiles uint64, bSemiOpenFiles uint64) (score int) {
	// Get the king's files
	wKingFile := onlyFile[bits.TrailingZeros64(b.White.Kings)%8]
	bKingFile := onlyFile[bits.TrailingZeros64(b.Black.Kings)%8]

	// Left & right files of the king
	wKingFiles := wKingFile | ((wKingFile & ^bitboardFileA) >> 1) | ((wKingFile & ^bitboardFileH) << 1)
	bKingFiles := bKingFile | ((bKingFile & ^bitboardFileA) >> 1) | ((bKingFile & ^bitboardFileH) << 1)

	wSemiOpenMask := wKingFiles & bSemiOpenFiles
	wOpenMask := wKingFiles & openFiles
	bSemiOpenMask := bKingFiles & wSemiOpenFiles
	bOpenMask := bKingFiles & openFiles

	wSemiOpenFilesCount := bits.OnesCount64(wSemiOpenMask)
	wOpenFilesCount := bits.OnesCount64(wOpenMask)
	bSemiOpenFilesCount := bits.OnesCount64(bSemiOpenMask)
	bOpenFilesCount := bits.OnesCount64(bOpenMask)

	if wSemiOpenFilesCount > 0 {
		score += (wSemiOpenFilesCount / 8) * KingSemiOpenFileMG
	}
	if wOpenFilesCount > 0 {
		score += (wOpenFilesCount / 8) * KingOpenFileMG
	}
	if bSemiOpenFilesCount > 0 {
		score -= (bSemiOpenFilesCount / 8) * KingSemiOpenFileMG
	}
	if bOpenFilesCount > 0 {
		score -= (bOpenFilesCount / 8) * KingOpenFileMG
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

/* ============= EVALUATION SUBROUTINES ============= */

func evaluateKnights(
	b *gm.Board,
	wPawnAttackBB, bPawnAttackBB uint64,
	innerKingSafetyZones, outerKingSafetyZones [2]uint64,
	knightMobilityScale int,
	whiteOutposts, blackOutposts uint64,
	knightMovementBB *[2]uint64,
	kingAttackMobilityBB *[2]uint64,
	attackUnitCounts *[2]int,
	debug bool,
) (knightMG, knightEG int) {

	knightPsqtMG, knightPsqtEG := countPieceTables(&b.White.Knights, &b.Black.Knights,
		&PSQT_MG[gm.PieceTypeKnight], &PSQT_EG[gm.PieceTypeKnight])

	var knightMobilityMG, knightMobilityEG int

	for x := b.White.Knights; x != 0; x &= x - 1 {
		square := bits.TrailingZeros64(x)
		attackedSquares := KnightMasks[square]
		(*kingAttackMobilityBB)[0] |= attackedSquares &^ b.White.All
		(*knightMovementBB)[0] |= attackedSquares
		mobilitySquares := attackedSquares &^ bPawnAttackBB &^ b.White.All
		popCnt := bits.OnesCount64(mobilitySquares)
		knightMobilityMG += popCnt * mobilityValueMG[gm.PieceTypeKnight]
		knightMobilityEG += popCnt * mobilityValueEG[gm.PieceTypeKnight]
		(*attackUnitCounts)[0] += bits.OnesCount64(attackedSquares&innerKingSafetyZones[1]) * attackerInner[gm.PieceTypeKnight]
		(*attackUnitCounts)[0] += bits.OnesCount64(attackedSquares&outerKingSafetyZones[1]) * attackerOuter[gm.PieceTypeKnight]
	}
	for x := b.Black.Knights; x != 0; x &= x - 1 {
		square := bits.TrailingZeros64(x)
		attackedSquares := KnightMasks[square]
		(*kingAttackMobilityBB)[1] |= attackedSquares &^ b.Black.All
		(*knightMovementBB)[1] |= attackedSquares
		mobilitySquares := attackedSquares &^ wPawnAttackBB &^ b.Black.All
		popCnt := bits.OnesCount64(mobilitySquares)
		knightMobilityMG -= popCnt * mobilityValueMG[gm.PieceTypeKnight]
		knightMobilityEG -= popCnt * mobilityValueEG[gm.PieceTypeKnight]
		(*attackUnitCounts)[1] += bits.OnesCount64(attackedSquares&innerKingSafetyZones[0]) * attackerInner[gm.PieceTypeKnight]
		(*attackUnitCounts)[1] += bits.OnesCount64(attackedSquares&outerKingSafetyZones[0]) * attackerOuter[gm.PieceTypeKnight]
	}

	knightOutpostMG := KnightOutpostMG*bits.OnesCount64(b.White.Knights&whiteOutposts) -
		KnightOutpostMG*bits.OnesCount64(b.Black.Knights&blackOutposts)
	knightOutpostEG := KnightOutpostEG*bits.OnesCount64(b.White.Knights&whiteOutposts) -
		KnightOutpostEG*bits.OnesCount64(b.Black.Knights&blackOutposts)

	knightThreatsBonusMG, knightThreatsBonusEG := knightThreats(b)
	knightTropismBonusMG, knightTropismBonusEG := knightKingTropism(b)
	knightMobilityMG = (knightMobilityMG * knightMobilityScale) / 100

	knightMG = knightPsqtMG + knightOutpostMG + knightMobilityMG + knightThreatsBonusMG + knightTropismBonusMG
	knightEG = knightPsqtEG + knightOutpostEG + knightMobilityEG + knightThreatsBonusEG + knightTropismBonusEG

	if debug {
		println("Knight MG:\t", "PSQT: ", knightPsqtMG, "\tMobility: ", knightMobilityMG,
			"\tOutpost: ", knightOutpostMG, "\tThreats: ", knightThreatsBonusMG, "\tTropism: ", knightTropismBonusMG)
		println("Knight EG:\t", "PSQT: ", knightPsqtEG, "\tMobility: ", knightMobilityEG,
			"\tOutpost: ", knightOutpostEG, "\tThreats: ", knightThreatsBonusEG, "\tTropism: ", knightTropismBonusEG)
	}

	return knightMG, knightEG
}

func evaluateBishops(
	b *gm.Board,
	allPieces uint64,
	wPawnAttackBB, bPawnAttackBB uint64,
	innerKingSafetyZones, outerKingSafetyZones [2]uint64,
	bishopMobilityScale, bishopPairScaleMG int,
	whiteOutposts, blackOutposts uint64,
	bishopMovementBB *[2]uint64,
	kingAttackMobilityBB *[2]uint64,
	attackUnitCounts *[2]int,
	debug bool,
) (bishopMG, bishopEG int) {

	bishopPsqtMG, bishopPsqtEG := countPieceTables(&b.White.Bishops, &b.Black.Bishops,
		&PSQT_MG[gm.PieceTypeBishop], &PSQT_EG[gm.PieceTypeBishop])

	var bishopMobilityMG, bishopMobilityEG int

	for x := b.White.Bishops; x != 0; x &= x - 1 {
		square := bits.TrailingZeros64(x)
		occupied := allPieces &^ PositionBB[square]
		bishopAttacks := gm.CalculateBishopMoveBitboard(uint8(square), occupied)
		(*kingAttackMobilityBB)[0] |= bishopAttacks &^ b.White.All
		(*bishopMovementBB)[0] |= bishopAttacks
		mobilitySquares := bishopAttacks &^ bPawnAttackBB &^ b.White.All
		popCnt := bits.OnesCount64(mobilitySquares)
		bishopMobilityMG += popCnt * mobilityValueMG[gm.PieceTypeBishop]
		bishopMobilityEG += popCnt * mobilityValueEG[gm.PieceTypeBishop]
		(*attackUnitCounts)[0] += bits.OnesCount64(bishopAttacks&innerKingSafetyZones[1]) * attackerInner[gm.PieceTypeBishop]
		(*attackUnitCounts)[0] += bits.OnesCount64(bishopAttacks&outerKingSafetyZones[1]) * attackerOuter[gm.PieceTypeBishop]
	}
	for x := b.Black.Bishops; x != 0; x &= x - 1 {
		square := bits.TrailingZeros64(x)
		occupied := allPieces &^ PositionBB[square]
		bishopAttacks := gm.CalculateBishopMoveBitboard(uint8(square), occupied)
		(*kingAttackMobilityBB)[1] |= bishopAttacks &^ b.Black.All
		(*bishopMovementBB)[1] |= bishopAttacks
		mobilitySquares := bishopAttacks &^ wPawnAttackBB &^ b.Black.All
		popCnt := bits.OnesCount64(mobilitySquares)
		bishopMobilityMG -= popCnt * mobilityValueMG[gm.PieceTypeBishop]
		bishopMobilityEG -= popCnt * mobilityValueEG[gm.PieceTypeBishop]
		(*attackUnitCounts)[1] += bits.OnesCount64(bishopAttacks&innerKingSafetyZones[0]) * attackerInner[gm.PieceTypeBishop]
		(*attackUnitCounts)[1] += bits.OnesCount64(bishopAttacks&outerKingSafetyZones[0]) * attackerOuter[gm.PieceTypeBishop]
	}

	bishopOutpostMG := BishopOutpostMG*bits.OnesCount64(b.White.Bishops&whiteOutposts) -
		BishopOutpostMG*bits.OnesCount64(b.Black.Bishops&blackOutposts)
	bishopOutpostEG := BishopOutpostEG*bits.OnesCount64(b.White.Bishops&whiteOutposts) -
		BishopOutpostEG*bits.OnesCount64(b.Black.Bishops&blackOutposts)

	bishopPairMG, bishopPairEG := bishopPairBonuses(b)
	bishopPairMG = (bishopPairMG * bishopPairScaleMG) / 100

	bishopMobilityMG = (bishopMobilityMG * bishopMobilityScale) / 100

	bishopMG = bishopPsqtMG + bishopOutpostMG + bishopPairMG + bishopMobilityMG
	bishopEG = bishopPsqtEG + bishopOutpostEG + bishopPairEG + bishopMobilityEG

	if debug {
		println("Bishop MG:\t", "PSQT: ", bishopPsqtMG, "\tMobility: ", bishopMobilityMG,
			"\tOutpost: ", bishopOutpostMG, "\tPair: ", bishopPairMG)
		println("Bishop EG:\t", "PSQT: ", bishopPsqtEG, "\tMobility: ", bishopMobilityEG,
			"\tOutpost: ", bishopOutpostEG, "\tPair: ", bishopPairEG)
	}

	return bishopMG, bishopEG
}

func evaluateRooks(
	b *gm.Board,
	allPieces uint64,
	wPawnAttackBB, bPawnAttackBB uint64,
	innerKingSafetyZones, outerKingSafetyZones [2]uint64,
	openFiles, wSemiOpenFiles, bSemiOpenFiles uint64,
	wRookStackFiles, bRookStackFiles uint64,
	rookMovementBB *[2]uint64,
	kingAttackMobilityBB *[2]uint64,
	attackUnitCounts *[2]int,
	debug bool,
) (rookMG, rookEG int) {

	rookPsqtMG, rookPsqtEG := countPieceTables(&b.White.Rooks, &b.Black.Rooks,
		&PSQT_MG[gm.PieceTypeRook], &PSQT_EG[gm.PieceTypeRook])

	var rookMobilityMG, rookMobilityEG int

	for x := b.White.Rooks; x != 0; x &= x - 1 {
		square := bits.TrailingZeros64(x)
		occupied := allPieces &^ PositionBB[square]
		rookAttacks := gm.CalculateRookMoveBitboard(uint8(square), occupied)
		(*kingAttackMobilityBB)[0] |= rookAttacks &^ b.White.All
		(*rookMovementBB)[0] |= rookAttacks
		mobilitySquares := rookAttacks &^ bPawnAttackBB &^ b.White.All
		popCnt := bits.OnesCount64(mobilitySquares)
		rookMobilityMG += popCnt * mobilityValueMG[gm.PieceTypeRook]
		rookMobilityEG += popCnt * mobilityValueEG[gm.PieceTypeRook]
		(*attackUnitCounts)[0] += bits.OnesCount64(rookAttacks&innerKingSafetyZones[1]) * attackerInner[gm.PieceTypeRook]
		(*attackUnitCounts)[0] += bits.OnesCount64(rookAttacks&outerKingSafetyZones[1]) * attackerOuter[gm.PieceTypeRook]
	}
	for x := b.Black.Rooks; x != 0; x &= x - 1 {
		square := bits.TrailingZeros64(x)
		occupied := allPieces &^ PositionBB[square]
		rookAttacks := gm.CalculateRookMoveBitboard(uint8(square), occupied)
		(*kingAttackMobilityBB)[1] |= rookAttacks &^ b.Black.All
		(*rookMovementBB)[1] |= rookAttacks
		mobilitySquares := rookAttacks &^ wPawnAttackBB &^ b.Black.All
		popCnt := bits.OnesCount64(mobilitySquares)
		rookMobilityMG -= popCnt * mobilityValueMG[gm.PieceTypeRook]
		rookMobilityEG -= popCnt * mobilityValueEG[gm.PieceTypeRook]
		(*attackUnitCounts)[1] += bits.OnesCount64(rookAttacks&innerKingSafetyZones[0]) * attackerInner[gm.PieceTypeRook]
		(*attackUnitCounts)[1] += bits.OnesCount64(rookAttacks&outerKingSafetyZones[0]) * attackerOuter[gm.PieceTypeRook]
	}

	rookSemiOpenMG, rookOpenMG := rookFilesBonus(b, openFiles, wSemiOpenFiles, bSemiOpenFiles)
	rookStackedMG := rookStackBonusMG(wRookStackFiles, bRookStackFiles)

	rookSeventhBonusEG := rookSeventhRankBonus(b)

	rookMG = rookPsqtMG + rookMobilityMG + rookOpenMG + rookSemiOpenMG + rookStackedMG
	rookEG = rookPsqtEG + rookMobilityEG + rookSeventhBonusEG

	if debug {
		println("Rook MG:\t", "PSQT: ", rookPsqtMG, "\tMobility: ", rookMobilityMG,
			"\tOpen: ", rookOpenMG, "\tSemi-open: ", rookSemiOpenMG, "\tStacked: ", rookStackedMG)
		println("Rook EG:\t", "PSQT: ", rookPsqtEG, "\tMobility: ", rookMobilityEG,
			"\tSeventh: ", rookSeventhBonusEG)
	}

	return rookMG, rookEG
}

func evaluateQueens(
	b *gm.Board,
	allPieces uint64,
	wPawnAttackBB, bPawnAttackBB uint64,
	innerKingSafetyZones, outerKingSafetyZones [2]uint64,
	queenMovementBB *[2]uint64,
	kingAttackMobilityBB *[2]uint64,
	attackUnitCounts *[2]int,
	debug bool,
) (queenMG, queenEG int) {

	queenPsqtMG, queenPsqtEG := countPieceTables(&b.White.Queens, &b.Black.Queens,
		&PSQT_MG[gm.PieceTypeQueen], &PSQT_EG[gm.PieceTypeQueen])

	var queenMobilityMG, queenMobilityEG int

	for x := b.White.Queens; x != 0; x &= x - 1 {
		square := bits.TrailingZeros64(x)
		occupied := allPieces &^ PositionBB[square]
		bishopAttacks := gm.CalculateBishopMoveBitboard(uint8(square), occupied)
		rookAttacks := gm.CalculateRookMoveBitboard(uint8(square), occupied)
		attackedSquares := bishopAttacks | rookAttacks
		(*kingAttackMobilityBB)[0] |= attackedSquares &^ b.White.All
		(*queenMovementBB)[0] |= attackedSquares
		mobilitySquares := attackedSquares &^ bPawnAttackBB &^ b.White.All
		popCnt := bits.OnesCount64(mobilitySquares)
		queenMobilityMG += popCnt * mobilityValueMG[gm.PieceTypeQueen]
		queenMobilityEG += popCnt * mobilityValueEG[gm.PieceTypeQueen]
		(*attackUnitCounts)[0] += bits.OnesCount64(attackedSquares&innerKingSafetyZones[1]) * attackerInner[gm.PieceTypeQueen]
		(*attackUnitCounts)[0] += bits.OnesCount64(attackedSquares&outerKingSafetyZones[1]) * attackerOuter[gm.PieceTypeQueen]
	}
	for x := b.Black.Queens; x != 0; x &= x - 1 {
		square := bits.TrailingZeros64(x)
		occupied := allPieces &^ PositionBB[square]
		bishopAttacks := gm.CalculateBishopMoveBitboard(uint8(square), occupied)
		rookAttacks := gm.CalculateRookMoveBitboard(uint8(square), occupied)
		attackedSquares := bishopAttacks | rookAttacks
		(*kingAttackMobilityBB)[1] |= attackedSquares &^ b.Black.All
		(*queenMovementBB)[1] |= attackedSquares
		mobilitySquares := attackedSquares &^ wPawnAttackBB &^ b.Black.All
		popCnt := bits.OnesCount64(mobilitySquares)
		queenMobilityMG -= popCnt * mobilityValueMG[gm.PieceTypeQueen]
		queenMobilityEG -= popCnt * mobilityValueEG[gm.PieceTypeQueen]
		(*attackUnitCounts)[1] += bits.OnesCount64(attackedSquares&innerKingSafetyZones[0]) * attackerInner[gm.PieceTypeQueen]
		(*attackUnitCounts)[1] += bits.OnesCount64(attackedSquares&outerKingSafetyZones[0]) * attackerOuter[gm.PieceTypeQueen]
	}

	centralizedQueenBonus := centralizedQueen(b)

	queenMG = queenPsqtMG + queenMobilityMG
	queenEG = queenPsqtEG + queenMobilityEG + centralizedQueenBonus

	if debug {
		println("Queen MG:\t", "PSQT: ", queenPsqtMG, "\tMobility: ", queenMobilityMG)
		println("Queen EG:\t", "PSQT: ", queenPsqtEG, "\tMobility: ", queenMobilityEG,
			"\tCentralized: ", centralizedQueenBonus)
	}

	return queenMG, queenEG
}

/* ============= MAIN EVALUATION ============= */
func Evaluation(b *gm.Board, debug bool) (score int32) {
	// ===========================================
	// PAWN_HASH: Get cached pawn structure
	// ===========================================
	pawnEntry := GetPawnEntry(b, debug)

	wPawnAttackBB := pawnEntry.WPawnAttackBB
	bPawnAttackBB := pawnEntry.BPawnAttackBB

	openFiles := pawnEntry.OpenFiles
	wSemiOpenFiles := pawnEntry.WSemiOpenFiles
	bSemiOpenFiles := pawnEntry.BSemiOpenFiles

	pawnMG := pawnEntry.PawnScoreMG
	pawnEG := pawnEntry.PawnScoreEG

	stormMG := evaluatePawnStorm(b)
	pawnMG += stormMG

	// Outposts for knights/bishops
	outposts := getOutpostsBB(b, wPawnAttackBB, bPawnAttackBB)
	whiteOutposts := outposts[0]
	blackOutposts := outposts[1]

	// Rooks stacked files
	wRookStackFiles, bRookStackFiles := getRookConnectedFiles(b)

	// Get center state from pawn structure
	lockedCenter, openIdx := getCenterState(b, openFiles, wSemiOpenFiles, bSemiOpenFiles,
		pawnEntry.WLeverBB, pawnEntry.BLeverBB)

	// Get mobility scales based on center state
	knightMobilityScale, bishopMobilityScale, bishopPairScaleMG := getCenterMobilityScales(lockedCenter, openIdx)

	// Movement bitboards for king-safety and weak-squares
	var knightMovementBB [2]uint64
	var bishopMovementBB [2]uint64
	var rookMovementBB [2]uint64
	var queenMovementBB [2]uint64
	var kingMovementBB [2]uint64
	var kingAttackMobilityBB [2]uint64

	// ===========================================
	// PIECE EVALUATION (using per-piece helpers)
	// ===========================================

	var knightMG, knightEG int
	var bishopMG, bishopEG int
	var rookMG, rookEG int
	var queenMG, queenEG int
	var kingMG, kingEG int

	var wMaterialMG, wMaterialEG = countMaterial(&b.White)
	var bMaterialMG, bMaterialEG = countMaterial(&b.Black)

	// King safety setup
	var attackUnitCounts = [2]int{0, 0}
	innerKingSafetyZones := getKingSafetyTable(b, true, wPawnAttackBB, bPawnAttackBB)
	outerKingSafetyZones := getKingSafetyTable(b, false, 0, 0)

	wPieceCount := bits.OnesCount64(b.White.Bishops | b.White.Knights | b.White.Rooks | b.White.Queens)
	bPieceCount := bits.OnesCount64(b.Black.Bishops | b.Black.Knights | b.Black.Rooks | b.Black.Queens)
	wPawnCount := bits.OnesCount64(b.White.Pawns)
	bPawnCount := bits.OnesCount64(b.Black.Pawns)

	// Variables for debug output still needed outside helpers
	var kingPsqtMG, kingPsqtEG int

	if debug {
		println("################### FEN ###################")
		println("FEN: ", b.ToFen())
		println("################### PAWN HASH ###################")
		println("Pawn MG (cached): ", pawnEntry.PawnScoreMG)
		println("Pawn EG (cached): ", pawnEntry.PawnScoreEG)
		println("Pawn storm MG: ", stormMG)

		println("################### PAWN BITBOARDS ###################")
		fmt.Printf("BB Pawn attacks W/B: %016x / %016x\n", pawnEntry.WPawnAttackBB, pawnEntry.BPawnAttackBB)
		fmt.Printf("BB Files open | wSemi | bSemi: %016x | %016x | %016x\n", openFiles, wSemiOpenFiles, bSemiOpenFiles)
		fmt.Printf("BB Passed W/B: %016x / %016x\n", pawnEntry.WPassedBB, pawnEntry.BPassedBB)
		fmt.Printf("BB Isolated W/B: %016x / %016x\n", pawnEntry.WIsolatedBB, pawnEntry.BIsolatedBB)
		fmt.Printf("BB Backward W/B: %016x / %016x\n", pawnEntry.WBackwardBB, pawnEntry.BBackwardBB)
		fmt.Printf("BB Blocked W/B: %016x / %016x\n", pawnEntry.WBlockedBB, pawnEntry.BBlockedBB)
		fmt.Printf("BB Lever W/B: %016x / %016x\n", pawnEntry.WLeverBB, pawnEntry.BLeverBB)
		fmt.Printf("BB Weak lever W/B: %016x / %016x\n", pawnEntry.WWeakLeverBB, pawnEntry.BWeakLeverBB)

		println("################### PIECE PARAMETERS ###################")
	}

	allPieces := b.White.All | b.Black.All

	// KNIGHTS
	knightMG, knightEG = evaluateKnights(
		b,
		wPawnAttackBB, bPawnAttackBB,
		innerKingSafetyZones, outerKingSafetyZones,
		knightMobilityScale,
		whiteOutposts, blackOutposts,
		&knightMovementBB,
		&kingAttackMobilityBB,
		&attackUnitCounts,
		debug,
	)

	// BISHOPS
	bishopMG, bishopEG = evaluateBishops(
		b,
		allPieces,
		wPawnAttackBB, bPawnAttackBB,
		innerKingSafetyZones, outerKingSafetyZones,
		bishopMobilityScale, bishopPairScaleMG,
		whiteOutposts, blackOutposts,
		&bishopMovementBB,
		&kingAttackMobilityBB,
		&attackUnitCounts,
		debug,
	)

	// ROOKS
	rookMG, rookEG = evaluateRooks(
		b,
		allPieces,
		wPawnAttackBB, bPawnAttackBB,
		innerKingSafetyZones, outerKingSafetyZones,
		openFiles, wSemiOpenFiles, bSemiOpenFiles,
		wRookStackFiles, bRookStackFiles,
		&rookMovementBB,
		&kingAttackMobilityBB,
		&attackUnitCounts,
		debug,
	)

	// QUEENS
	queenMG, queenEG = evaluateQueens(
		b,
		allPieces,
		wPawnAttackBB, bPawnAttackBB,
		innerKingSafetyZones, outerKingSafetyZones,
		&queenMovementBB,
		&kingAttackMobilityBB,
		&attackUnitCounts,
		debug,
	)

	// KING (unchanged, but now uses attackUnitCounts and kingAttackMobilityBB filled by helpers)
	kingPsqtMG, kingPsqtEG = countPieceTables(&b.White.Kings, &b.Black.Kings, &PSQT_MG[gm.PieceTypeKing], &PSQT_EG[gm.PieceTypeKing])

	kingAttackPenaltyMG, kingAttackPenaltyEG := kingAttackCountPenalty(&attackUnitCounts)
	kingPawnShieldPenaltyMG := kingFilesPenalty(b, openFiles, wSemiOpenFiles, bSemiOpenFiles)
	KingMinorPieceDefenseBonusMG := kingMinorPieceDefences(innerKingSafetyZones, knightMovementBB, bishopMovementBB)
	kingPawnDefenseMG := kingPawnDefense(b, innerKingSafetyZones)

	kingMovementBB[0] = (innerKingSafetyZones[0] &^ b.White.All) &^ kingAttackMobilityBB[1]
	kingMovementBB[1] = (innerKingSafetyZones[1] &^ b.Black.All) &^ kingAttackMobilityBB[0]

	kingCentralManhattanPenalty := 0
	kingMopUpBonus := 0

	piecePhase := GetPiecePhase(b)

	if (piecePhase < 16 && bits.OnesCount64(b.White.Queens|b.Black.Queens) == 0) || piecePhase < 10 {
		noPawnsLeft := wPawnCount == 0 && bPawnCount == 0
		if wPieceCount > 0 && bPieceCount == 0 && noPawnsLeft {
			kingMopUpBonus = getKingMopUpBonus(b, true, b.White.Queens > 0, b.White.Rooks > 0)
		} else if wPieceCount == 0 && bPieceCount > 0 && noPawnsLeft {
			kingMopUpBonus = getKingMopUpBonus(b, false, b.Black.Queens > 0, b.Black.Rooks > 0)
		} else {
			kingCentralManhattanPenalty = kingEndGameCentralizationPenalty(b)
		}
	}

	kingMG = kingPsqtMG + kingAttackPenaltyMG + kingPawnShieldPenaltyMG +
		KingMinorPieceDefenseBonusMG + kingPawnDefenseMG
	kingEG = kingPsqtEG + kingAttackPenaltyEG + kingCentralManhattanPenalty +
		kingMopUpBonus

	if debug {
		println("King MG:\t", "PSQT: ", kingPsqtMG, "\tAttack: ", kingAttackPenaltyMG,
			"\tFile: ", kingPawnShieldPenaltyMG, "\tMinorDefense: ", KingMinorPieceDefenseBonusMG,
			"\tPawnDefense: ", kingPawnDefenseMG)
		println("King EG:\t", "PSQT: ", kingPsqtEG, "\tAttack: ", kingAttackPenaltyEG,
			"\tCmd: ", kingCentralManhattanPenalty, "\tMopUp: ", kingMopUpBonus,
			"\tPawnDefense: ", kingPawnDefenseMG)
	}

	if debug {
		println("################### FINAL PIECE EVALUATION ###################")
		println("Pawn: ", pawnMG, ":", pawnEG)
		println("Knight: ", knightMG, ":", knightEG)
		println("Bishop: ", bishopMG, ":", bishopEG)
		println("Rook: ", rookMG, ":", rookEG)
		println("Queen: ", queenMG, ":", queenEG)
		println("King: ", kingMG, ":", kingEG)
	}

	if debug {
		println("################### SPACE EVALUATION ###################")
	}

	// Weak squares & protected squares (unchanged call)
	spaceMG, spaceEG := spaceEvaluation(b, wPawnAttackBB, bPawnAttackBB, knightMovementBB, bishopMovementBB, piecePhase)
	weakKingMG, weakKingEG := weakKingSquaresPenalty(b, wPawnAttackBB, bPawnAttackBB, innerKingSafetyZones)

	// FINAL SCORE CALCULATION (unchanged)
	materialScoreMG := wMaterialMG - bMaterialMG
	materialScoreEG := wMaterialEG - bMaterialEG

	toMoveBonus := TempoBonus
	if !b.Wtomove {
		toMoveBonus = -TempoBonus
	}

	imbalanceMG, imbalanceEG := materialImbalance(b)

	if debug {
		println("################### SPACE EVALUATION ###################")
		println("Space: ", spaceMG, ":", spaceEG)
		println("Weak king: ", weakKingMG, ":", weakKingEG)
	}

	variableScoreMG := pawnMG + knightMG + bishopMG + rookMG + queenMG + kingMG + toMoveBonus + imbalanceMG + spaceMG + weakKingMG //+ queenInfiltrationMG
	variableScoreEG := pawnEG + knightEG + bishopEG + rookEG + queenEG + kingEG + toMoveBonus + imbalanceEG + spaceEG + weakKingEG //+ queenInfiltrationEG

	mgScore := materialScoreMG + variableScoreMG
	egScore := materialScoreEG + variableScoreEG

	mgWeight := piecePhase
	egWeight := TotalPhase - piecePhase
	score = int32((mgScore*mgWeight + egScore*egWeight) / TotalPhase)

	if isTheoreticalDraw(b, debug) {
		score = score / DrawDivider
	}

	if debug {
		println("################### MATERIAL ###################")
		println("Material: ", materialScoreMG, ":", materialScoreEG)
		println("Variable: ", variableScoreMG, ":", variableScoreEG)
		println("################### OTHERS ###################")
		println("Phase: ", piecePhase)
		println("!!!--- NOTE: Score is shown from white's perspective in the debug ---!!!")
		println("Final score:", score)
	}

	if !b.Wtomove {
		score = -score
	}

	return score
}
