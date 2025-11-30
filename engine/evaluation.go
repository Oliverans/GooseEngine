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
		-35, -1, -20, -23, -15, 24, 38, -22,
		-26, -4, -4, -10, 3, 3, 33, -12,
		-27, -2, -5, 12, 17, 6, 10, -25,
		-14, 13, 6, 21, 23, 12, 17, -23,
		-6, 7, 26, 31, 65, 56, 25, -20,
		98, 134, 61, 95, 68, 126, 34, -11,
		0, 0, 0, 0, 0, 0, 0, 0,
	},
	gm.PieceTypeKnight: {
		-21, -21, -41, -24, -19, -16, -20, -35,
		-31, -28, -12, -4, -8, -4, -17, -14,
		-20, -5, 0, 10, 11, 3, 1, -12,
		-8, 3, 13, 9, 17, 14, 29, 5,
		-3, 13, 32, 38, 27, 48, 23, 26,
		-19, 12, 40, 49, 66, 64, 36, 15,
		-17, -12, 19, 31, 30, 37, -4, 7,
		-54, -5, -11, 0, 1, -5, -1, -15,
	},
	gm.PieceTypeBishop: {
		5, 0, -11, -18, -17, -6, -6, 2,
		5, 9, 13, 0, 2, 10, 22, 11,
		2, 11, 10, 14, 11, 9, 10, 15,
		-7, 10, 16, 23, 28, 11, 12, 8,
		-8, 21, 22, 50, 35, 39, 19, 7,
		2, 15, 34, 32, 42, 50, 35, 23,
		-27, 3, -2, -8, -1, 10, -3, 5,
		-24, -7, -13, -12, -7, -19, 1, -9,
	},
	gm.PieceTypeRook: {
		-27, -22, -18, -14, -18, -21, -6, -23,
		-58, -29, -28, -24, -29, -19, -14, -42,
		-43, -31, -34, -26, -30, -33, -12, -26,
		-33, -33, -28, -18, -23, -21, -6, -16,
		-18, -8, 2, 19, 10, 15, 10, 9,
		-8, 24, 17, 36, 44, 36, 41, 21,
		10, 5, 27, 36, 27, 33, 11, 34,
		26, 24, 18, 22, 12, 15, 20, 31,
	},
	gm.PieceTypeQueen: {
		12, 1, 6, 12, 11, -10, -15, -2,
		3, 11, 16, 12, 12, 22, 19, 5,
		5, 13, 12, 10, 9, 12, 21, 17,
		8, 10, 12, 9, 9, 13, 17, 18,
		-5, 5, 3, -2, 10, 12, 20, 20,
		-8, -1, 5, 7, 2, 8, 3, 0,
		-6, -33, 8, 4, -23, 4, -3, 32,
		5, 19, 20, 21, 17, 27, 24, 32,
	},
	gm.PieceTypeKing: {
		-12, 32, -3, -72, -24, -75, 15, 22,
		3, -8, -23, -60, -38, -42, 5, 19,
		-7, -8, -5, -9, 0, -3, 5, -13,
		-1, 7, 12, 7, 14, 12, 21, -7,
		2, 9, 12, 6, 9, 12, 13, -7,
		2, 12, 12, 9, 7, 13, 11, 2,
		-1, 7, 6, 3, 4, 5, 5, 0,
		-2, 1, 1, 1, 1, 0, 0, -1,
	},
}
var PSQT_EG = [7][64]int{
	gm.PieceTypePawn: {
		0, 0, 0, 0, 0, 0, 0, 0,
		14, 16, 11, 9, 16, 14, 10, -10,
		7, 11, 0, 4, 4, 4, 6, -7,
		15, 15, -3, -4, -6, -3, 5, -2,
		35, 29, 19, 2, -1, 2, 17, 13,
		87, 84, 70, 57, 50, 43, 64, 66,
		139, 129, 109, 90, 79, 73, 87, 102,
		0, 0, 0, 0, 0, 0, 0, 0,
	},
	gm.PieceTypeKnight: {
		-27, -57, -27, -18, -19, -27, -44, -28,
		-30, -11, -12, -4, -1, -14, -14, -29,
		-36, -1, 7, 22, 19, 6, 2, -30,
		-12, 14, 34, 41, 37, 38, 17, -6,
		-6, 18, 32, 45, 50, 39, 31, 3,
		-13, 12, 29, 31, 26, 37, 16, -6,
		-19, -6, 8, 28, 23, 4, -5, -12,
		-36, -7, 6, 4, 5, 6, -2, -15,
	},
	gm.PieceTypeBishop: {
		-24, -14, -35, -11, -15, -22, -17, -16,
		-11, -18, -10, -2, -1, -14, -15, -29,
		-9, 1, 10, 11, 12, 7, -6, -5,
		-3, 9, 19, 19, 16, 17, 9, -5,
		4, 16, 16, 18, 26, 17, 23, 8,
		2, 15, 16, 15, 16, 24, 17, 9,
		-2, 15, 16, 15, 16, 12, 15, -3,
		4, 8, 8, 13, 9, 2, 4, 4,
	},
	gm.PieceTypeRook: {
		-21, -10, -4, -5, -7, -6, -8, -29,
		-17, -19, -11, -14, -13, -20, -18, -15,
		-12, -1, -1, -4, -3, -4, -4, -12,
		5, 18, 20, 15, 13, 13, 9, 0,
		18, 23, 25, 21, 21, 18, 14, 12,
		27, 20, 31, 21, 18, 27, 15, 16,
		33, 39, 38, 40, 43, 26, 28, 21,
		33, 37, 43, 41, 42, 43, 39, 38,
	},
	gm.PieceTypeQueen: {
		-23, -32, -40, -37, -44, -32, -21, -6,
		-21, -22, -39, -21, -29, -55, -40, -12,
		-15, -8, 5, -6, -5, 5, -6, -3,
		-10, 11, 11, 38, 37, 44, 32, 40,
		1, 23, 21, 55, 54, 54, 61, 50,
		8, 14, 25, 42, 46, 46, 43, 51,
		16, 40, 32, 38, 55, 28, 28, 30,
		17, 30, 31, 35, 33, 35, 30, 32,
	},
	gm.PieceTypeKing: {
		-39, -32, -23, -29, -57, -18, -35, -79,
		-16, -11, -3, 1, -3, -1, -17, -36,
		-19, -4, 3, 13, 9, 2, -10, -22,
		-15, 11, 20, 27, 23, 16, 5, -19,
		2, 27, 31, 31, 30, 29, 23, -3,
		7, 32, 30, 21, 19, 37, 35, 3,
		-7, 19, 14, 5, 6, 12, 23, -4,
		-15, -8, -3, 0, -3, -2, -4, -11,
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

// Most other non-material evaluation parameters
const (
	QueenCentralizationEG = 8
	QueenInfiltrationMG   = -5
	QueenInfiltrationEG   = 20

	RookXrayQueenMG = 20
	RookXrayKingMG  = 20

	RookStackedMG     = 20
	RookConnectedMG   = 20
	RookSeventhRankEG = 20
	RookSemiOpenMG    = 10
	RookOpenMG        = 20

	KnightOutpostMG = 17
	KnightOutpostEG = 9
	KnightThreatMG  = 10
	KnightThreatEG  = 5

	BishopXrayRookMG  = 10
	BishopXrayQueenMG = 15
	BishopXrayKingMG  = 20

	BishopOutpostMG = 12
	BishopOutpostEG = 4

	BishopPairBonusMG = 28
	BishopPairBonusEG = 34

	PassingPawnFileMG      = 9
	PassingPawnRankMG      = 10
	PassingPawnCandidateMG = 10
	PassingPawnFileEG      = 5
	PassingPawnRankEG      = 12
	PassedPawnBlocked      = -6
	PassedPawnFilePenalty  = -2

	BackwardPawnMG = 6
	BackwardPawnEG = 6

	IsolatedPawnMG = 7
	IsolatedPawnEG = 12

	PawnDoubledMG = 10
	PawnDoubledEG = 7

	PawnStormMG          = 12
	PawnFrontProximityMG = 10

	PawnConnectedMG = 8
	PawnConnectedEG = 5
	PawnPhalanxMG   = 8
	PawnPhalanxEG   = 5

	PawnWeakLeverMG = 10
	PawnWeakLeverEG = 5

	PawnBlockedMG = -6
	PawnBlockedEG = -7

	KingOpenFileMG          = -5
	KingSemiOpenFileMG      = -3
	KingMinorDefenseBonusMG = 7
	KingPawnDefenseBonusMG  = 6

	KnightTropismMG = 3
	KnightTropismEG = 4

	WeakSquarePenaltyMG    = -3
	WeakSquarePenaltyEG    = -2
	ProtectedSquareBonusMG = 2
	ProtectedSquareBonusEG = 1

	TempoBonus = 10

	DrawDivider = 8
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
// Precomputed isolated pawn mask per file (from dragontooth engine)
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

/* ============= EVALUATION SUBROUTINES ============= */

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

// materialImbalance based on a Kaufman-style non-linear material term
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

func getWeakSquares(movementBB [2][5]uint64, kingInnerRing [2]uint64, wPawnAttackBB, bPawnAttackBB uint64) (weakSquares [2]uint64, weakSquaresKing [2]uint64) {

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

	var wWeakBoard uint64 = wRemainder & wSide
	var bWeakBoard uint64 = bRemainder & bSide

	weakSquares[0] = wWeakBoard
	weakSquares[1] = bWeakBoard

	weakSquaresKing[0] = wWeakBoard & kingInnerRing[0]
	weakSquaresKing[1] = bWeakBoard & kingInnerRing[1]

	return weakSquares, weakSquaresKing
}

// evaluateWeakSquares computes the midgame weak-square contribution using raw attack maps.
// It treats P/N/B as strong defenders, rooks as weak defenders (half penalty), and excludes Q/K as defenders.
func weakSquaresPenalty(movementBB [2][5]uint64, kingInnerRing [2]uint64, wPawnAttackBB uint64, bPawnAttackBB uint64) (weakSquareScore int, protectedSquareScore int, weakSquares [2]uint64, weakKingSquares [2]uint64, protectedSquares [2]uint64) {

	// 1) Candidate weak squares from your existing logic
	weakSquares, weakKingSquares = getWeakSquares(movementBB, kingInnerRing, wPawnAttackBB, bPawnAttackBB)

	// Same priority masks as in getWeakSquares (white "zone" and black "zone").
	const wZoneMask uint64 = 0x3c3c3c7e00     // b2–g2, c3–f3, c4–f4, c5–f5
	const bZoneMask uint64 = 0x7e3c3c3c000000 // b7–g7, c6–f6, c5–f5, c4–f4

	// 2) "Solid defenders" for each side: pawn + N + B + R
	// movementBB indices: 0 = N, 1 = B, 2 = R, 3 = Q, 4 = K
	wSolidDef := wPawnAttackBB | movementBB[0][0] | movementBB[0][1] | movementBB[0][2]
	bSolidDef := bPawnAttackBB | movementBB[1][0] | movementBB[1][1] | movementBB[1][2]

	// 3) Important squares that are NOT already weak (these can become "protected")
	wImportant := wZoneMask
	bImportant := bZoneMask

	wCandidates := wImportant &^ weakSquares[0]
	bCandidates := bImportant &^ weakSquares[1]

	// 4) Protected squares: important + solidly defended
	wProtected := wCandidates & wSolidDef
	bProtected := bCandidates & bSolidDef

	// King-ring subsets of protected squares
	wProtectedKing := wProtected & kingInnerRing[0]
	bProtectedKing := bProtected & kingInnerRing[1]

	// 5) Separate general weak vs king-ring weak
	wWeakGeneral := weakSquares[0] &^ weakKingSquares[0]
	bWeakGeneral := weakSquares[1] &^ weakKingSquares[1]

	// 6) Popcounts
	wProtCnt := bits.OnesCount64(wProtected)
	bProtCnt := bits.OnesCount64(bProtected)
	wProtKingCnt := bits.OnesCount64(wProtectedKing)
	bProtKingCnt := bits.OnesCount64(bProtectedKing)

	wWeakCnt := bits.OnesCount64(wWeakGeneral)
	bWeakCnt := bits.OnesCount64(bWeakGeneral)
	wWeakKingCnt := bits.OnesCount64(weakKingSquares[0])
	bWeakKingCnt := bits.OnesCount64(weakKingSquares[1])

	// 7) Scoring from White's POV

	// Protected squares: we like having them in our zone
	protectedSquareScore += (wProtCnt - bProtCnt) * ProtectedSquareBonusMG
	protectedSquareScore += (wProtKingCnt - bProtKingCnt) * ProtectedSquareBonusEG

	// Weak squares: we dislike our own, like opponent's
	weakSquareScore += (bWeakCnt - wWeakCnt) * WeakSquarePenaltyMG
	weakSquareScore += (bWeakKingCnt - wWeakKingCnt) * WeakSquarePenaltyEG

	protectedSquares = [2]uint64{wProtected, bProtected}

	return weakSquareScore, protectedSquareScore, weakSquares, weakKingSquares, protectedSquares
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
	blockedBonusMG = (wCount * PawnBlockedMG) - (bCount * PawnBlockedMG)
	blockedBonusEG = (wCount * PawnBlockedEG) - (bCount * PawnBlockedEG)
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

// pawnWeakLeverPenalty scores unsupported pawns whose advance squares
// can be hit by multiple enemy pawns.
func pawnWeakLeverPenalty(wWeak uint64, bWeak uint64) (mg int, eg int) {
	wCount := bits.OnesCount64(wWeak)
	bCount := bits.OnesCount64(bWeak)
	diffMG := (bCount - wCount) * PawnWeakLeverMG
	diffEG := (bCount - wCount) * PawnWeakLeverEG
	return diffMG, diffEG
}

// returns MG-only contribution for storm (bonus) and proximity (penalty),
// with steeper proximity penalty if kings are on opposite wings, and extra penalty if a lever exists
// on the defender's wing among advanced ranks.
func pawnStormProximity(wStorm uint64, bStorm uint64, wProx uint64, bProx uint64, wLever uint64, bLever uint64, wWing uint64, bWing uint64, oppositeSides bool) (mg int) {
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
	mg += (bProxCnt - wProxCnt) * PawnFrontProximityMG

	// Extra penalty if storm area also has immediate lever in defender wing
	wLeverStorm := bits.OnesCount64((wLever & bWing) & ranksAbove[3])
	bLeverStorm := bits.OnesCount64((bLever & wWing) & ranksBelow[4])
	mg += (bLeverStorm - wLeverStorm) * PawnFrontProximityMG
	return mg
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

/*
	KNIGHT FUNCTIONS
*/

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
			tropismMG += (7 - dist) * KnightTropismEG
		}
	}

	for x := b.Black.Knights; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		dist := chebyshevDistance(sq, wKingSq)
		if dist <= 6 {
			tropismMG -= (7 - dist) * KnightTropismMG
			tropismMG -= (7 - dist) * KnightTropismEG
		}
	}

	return tropismMG, tropismEG
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

	// White bishops
	for x := b.White.Bishops; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		bishopBB := PositionBB[sq]
		occupied := allPieces &^ bishopBB

		// Normal bishop attacks
		normalAttacks := gm.CalculateBishopMoveBitboard(uint8(sq), occupied)

		// Find all pieces (own and enemy) that the bishop directly hits
		directHits := normalAttacks & allPieces

		for y := directHits; y != 0; y &= y - 1 {
			blockerSq := bits.TrailingZeros64(y)
			blockerBB := PositionBB[blockerSq]

			// Calculate attacks with this piece removed (x-ray through it)
			xrayOccupied := occupied &^ blockerBB
			xrayAttacks := gm.CalculateBishopMoveBitboard(uint8(sq), xrayOccupied)

			// Newly revealed squares
			revealed := xrayAttacks &^ normalAttacks

			// Check for enemy targets behind
			revealedEnemies := revealed & b.Black.All
			if revealedEnemies == 0 {
				continue
			}

			// Determine if this is a useful x-ray based on blocker type
			blockerIsOwn := (blockerBB & b.White.All) != 0

			if blockerIsOwn {
				// X-ray through own piece = discovered attack potential
				// Any enemy target is valuable
				switch {
				case revealedEnemies&b.Black.Kings != 0:
					bishopXrayMG += BishopXrayRookMG
				case revealedEnemies&b.Black.Queens != 0:
					bishopXrayMG += BishopXrayQueenMG
				case revealedEnemies&b.Black.Rooks != 0:
					bishopXrayMG += BishopXrayRookMG
				}
			} else {
				// X-ray through enemy piece = pin/skewer potential
				// Only valuable if target behind is MORE valuable than blocker
				blockerValue := getPieceValue(blockerBB, &b.Black)

				switch {
				case revealedEnemies&b.Black.Kings != 0:
					bishopXrayMG += BishopXrayKingMG
				case revealedEnemies&b.Black.Queens != 0 && blockerValue < 9:
					bishopXrayMG += BishopXrayQueenMG
				case revealedEnemies&b.Black.Rooks != 0 && blockerValue < 5:
					bishopXrayMG += BishopXrayRookMG
				}
			}
		}
	}

	// Black bishops (mirror)
	for x := b.Black.Bishops; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		bishopBB := PositionBB[sq]
		occupied := allPieces &^ bishopBB

		normalAttacks := gm.CalculateBishopMoveBitboard(uint8(sq), occupied)
		directHits := normalAttacks & allPieces

		for y := directHits; y != 0; y &= y - 1 {
			blockerSq := bits.TrailingZeros64(y)
			blockerBB := PositionBB[blockerSq]

			xrayOccupied := occupied &^ blockerBB
			xrayAttacks := gm.CalculateBishopMoveBitboard(uint8(sq), xrayOccupied)
			revealed := xrayAttacks &^ normalAttacks

			revealedEnemies := revealed & b.White.All
			if revealedEnemies == 0 {
				continue
			}

			blockerIsOwn := (blockerBB & b.Black.All) != 0

			if blockerIsOwn {
				switch {
				case revealedEnemies&b.White.Kings != 0:
					bishopXrayMG -= BishopXrayKingMG
				case revealedEnemies&b.White.Queens != 0:
					bishopXrayMG -= BishopXrayQueenMG
				case revealedEnemies&b.White.Rooks != 0:
					bishopXrayMG -= BishopXrayRookMG
				}
			} else {
				blockerValue := getPieceValue(blockerBB, &b.White)

				switch {
				case revealedEnemies&b.White.Kings != 0:
					bishopXrayMG -= BishopXrayKingMG
				case revealedEnemies&b.White.Queens != 0 && blockerValue < 9:
					bishopXrayMG -= BishopXrayQueenMG
				case revealedEnemies&b.White.Rooks != 0 && blockerValue < 5:
					bishopXrayMG -= BishopXrayRookMG
				}
			}
		}
	}

	return bishopXrayMG
}

/*
	ROOK FUNCTIONS
*/

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

// rookXrayAttacks detects rooks that x-ray through pieces to valuable targets.
// Scenarios:
// - X-ray through own piece to enemy target (discovered attack potential)
// - X-ray through enemy piece to more valuable enemy piece (pin/skewer)
func rookXrayAttacks(b *gm.Board) (xrayMG int) {
	allPieces := b.White.All | b.Black.All

	// White rooks
	for x := b.White.Rooks; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		rookBB := PositionBB[sq]
		occupied := allPieces &^ rookBB

		// Normal rook attacks
		normalAttacks := gm.CalculateRookMoveBitboard(uint8(sq), occupied)

		// Find all pieces the rook directly hits
		directHits := normalAttacks & allPieces

		for y := directHits; y != 0; y &= y - 1 {
			blockerSq := bits.TrailingZeros64(y)
			blockerBB := PositionBB[blockerSq]

			// Calculate attacks with this piece removed (x-ray through it)
			xrayOccupied := occupied &^ blockerBB
			xrayAttacks := gm.CalculateRookMoveBitboard(uint8(sq), xrayOccupied)

			// Newly revealed squares
			revealed := xrayAttacks &^ normalAttacks

			// Check for enemy targets behind
			revealedEnemies := revealed & b.Black.All
			if revealedEnemies == 0 {
				continue
			}

			blockerIsOwn := (blockerBB & b.White.All) != 0

			if blockerIsOwn {
				// X-ray through own piece = discovered attack potential
				switch {
				case revealedEnemies&b.Black.Kings != 0:
					xrayMG += RookXrayKingMG
				case revealedEnemies&b.Black.Queens != 0:
					xrayMG += RookXrayQueenMG
				}
			} else {
				// X-ray through enemy piece = pin/skewer potential
				blockerValue := getPieceValue(blockerBB, &b.Black)

				switch {
				case revealedEnemies&b.Black.Kings != 0:
					// Pin to king is always valuable
					xrayMG += RookXrayKingMG
				case revealedEnemies&b.Black.Queens != 0 && blockerValue < 9:
					// Pin/skewer on queen (blocker worth less than queen)
					xrayMG += RookXrayQueenMG
				}
			}
		}
	}

	// Black rooks
	for x := b.Black.Rooks; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		rookBB := PositionBB[sq]
		occupied := allPieces &^ rookBB

		normalAttacks := gm.CalculateRookMoveBitboard(uint8(sq), occupied)
		directHits := normalAttacks & allPieces

		for y := directHits; y != 0; y &= y - 1 {
			blockerSq := bits.TrailingZeros64(y)
			blockerBB := PositionBB[blockerSq]

			xrayOccupied := occupied &^ blockerBB
			xrayAttacks := gm.CalculateRookMoveBitboard(uint8(sq), xrayOccupied)
			revealed := xrayAttacks &^ normalAttacks

			revealedEnemies := revealed & b.White.All
			if revealedEnemies == 0 {
				continue
			}

			blockerIsOwn := (blockerBB & b.Black.All) != 0

			if blockerIsOwn {
				switch {
				case revealedEnemies&b.White.Kings != 0:
					xrayMG -= RookXrayKingMG
				case revealedEnemies&b.White.Queens != 0:
					xrayMG -= RookXrayQueenMG
				}
			} else {
				blockerValue := getPieceValue(blockerBB, &b.White)

				switch {
				case revealedEnemies&b.White.Kings != 0:
					xrayMG -= RookXrayKingMG
				case revealedEnemies&b.White.Queens != 0 && blockerValue < 9:
					xrayMG -= RookXrayQueenMG
				}
			}
		}
	}

	return xrayMG
}

// scoreRookStacksMG returns a midgame-only bonus for connected rook stacks per side.
func scoreRookStacksMG(wFiles uint64, bFiles uint64) (mg int) {
	wCount := bits.OnesCount64(wFiles) / 8
	bCount := bits.OnesCount64(bFiles) / 8
	mg = (wCount * RookStackedMG) - (bCount * RookStackedMG)
	return mg
}

/*
	QUEEN FUNCTIONS
*/

func centralizedQueen(b *gm.Board) (centralizedBonus int) {
	if b.White.Queens&centralizedQueenSquares != 0 {
		centralizedBonus += QueenCentralizationEG
	}
	if b.Black.Queens&centralizedQueenSquares != 0 {
		centralizedBonus -= QueenCentralizationEG
	}
	return centralizedBonus
}

func queenInfiltrationBonus(b *gm.Board, weakSquares [2]uint64, wPawnAttackSpan uint64, bPawnAttackSpan uint64) (queenInfiltrationBonusMG int, queenInfiltrationBonusEG int) {
	// Reward infiltration only when the queen occupies enemy weak squares in the enemy half
	// and is outside the enemy pawn attack span.
	// White queen occupancy on black weak squares (enemy half, outside black pawn span)
	wOcc := b.White.Queens & weakSquares[1] & ranksAbove[4] &^ bPawnAttackSpan
	if wOcc != 0 {
		queenInfiltrationBonusMG += QueenInfiltrationMG
		queenInfiltrationBonusEG += QueenInfiltrationEG
	}

	// Black queen occupancy on white weak squares (enemy half, outside white pawn span)
	bOcc := b.Black.Queens & weakSquares[0] & ranksBelow[3] &^ wPawnAttackSpan
	if bOcc != 0 {
		queenInfiltrationBonusMG -= QueenInfiltrationMG
		queenInfiltrationBonusEG -= QueenInfiltrationEG
	}

	return queenInfiltrationBonusMG, queenInfiltrationBonusEG
}

/*
	KING FUNCTIONS
*/

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
		println("Knight MG:\t", "PSQT: ", knightPsqtMG, "\tOutpost: ", knightOutpostMG,
			"\tMobility: ", knightMobilityMG, "\tThreats: ", knightThreatsBonusMG, "\tTropism: ", knightTropismBonusMG)
		println("Knight EG:\t", "PSQT: ", knightPsqtEG, "\tOutpost: ", knightOutpostEG,
			"\tMobility: ", knightMobilityEG, "\tThreats: ", knightThreatsBonusEG, "\tTropism: ", knightTropismBonusEG)
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
	bishopXrayMG := bishopXrayAttacks(b)

	bishopMobilityMG = (bishopMobilityMG * bishopMobilityScale) / 100

	bishopMG = bishopPsqtMG + bishopOutpostMG + bishopPairMG + bishopMobilityMG + bishopXrayMG
	bishopEG = bishopPsqtEG + bishopOutpostEG + bishopPairEG + bishopMobilityEG

	if debug {
		println("Bishop MG:\t", "PSQT: ", bishopPsqtMG, "\tOutpost: ", bishopOutpostMG,
			"\tPair: ", bishopPairMG, "\tMobility: ", bishopMobilityMG, "\tXray: ", bishopXrayMG)
		println("Bishop EG:\t", "PSQT: ", bishopPsqtEG, "\tOutpost: ", bishopOutpostEG,
			"\tPair: ", bishopPairEG, "\tMobility: ", bishopMobilityEG)
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
	rookXrayAttack := rookXrayAttacks(b)
	rookStackedMG := scoreRookStacksMG(wRookStackFiles, bRookStackFiles)

	rookSeventhBonusEG := rookSeventhRankBonus(b)

	rookMG = rookPsqtMG + rookMobilityMG + rookOpenMG + rookSemiOpenMG + rookXrayAttack + rookStackedMG
	rookEG = rookPsqtEG + rookMobilityEG + rookSeventhBonusEG

	if debug {
		println("Rook MG:\t", "PSQT: ", rookPsqtMG, "\tMobility: ", rookMobilityMG,
			"\tOpen: ", rookOpenMG, "\tSemi-open: ", rookSemiOpenMG, "\tXray: ", rookXrayAttack,
			"\tStacked: ", rookStackedMG)
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

func Evaluation(b *gm.Board, debug bool, isQuiescence bool) (score int16) {
	// ===========================================
	// PAWN_HASH: Get cached pawn structure
	// ===========================================
	pawnEntry := GetPawnEntry(b, debug)

	// PAWN_HASH: Use cached pawn attacks instead of recomputing
	wPawnAttackBB := pawnEntry.WPawnAttackBB
	bPawnAttackBB := pawnEntry.BPawnAttackBB

	// Pawn attack spans (computed from cached attacks - cheap operation)
	wPawnAttackSpan := calculatePawnFileFill(wPawnAttackBB, true) & ranksBelow[4]
	bPawnAttackSpan := calculatePawnFileFill(bPawnAttackBB, false) & ranksAbove[4]

	// PAWN_HASH: Use cached file structure
	openFiles := pawnEntry.OpenFiles
	wSemiOpenFiles := pawnEntry.WSemiOpenFiles
	bSemiOpenFiles := pawnEntry.BSemiOpenFiles

	// Pawn structure score from hash
	pawnMG := pawnEntry.PawnScoreMG
	pawnEG := pawnEntry.PawnScoreEG

	wWingMask, bWingMask := getKingWingMasks(b)

	oppositeSides := (wWingMask != bWingMask)
	wPawnStormBB, bPawnStormBB := getPawnStormBitboards(b, wWingMask, bWingMask)
	wPawnFrontProximity, bPawnFrontProximity := getEnemyPawnProximityBitboards(b, wWingMask, bWingMask)
	stormMG := pawnStormProximity(wPawnStormBB, bPawnStormBB, wPawnFrontProximity, bPawnFrontProximity, pawnEntry.WLeverBB, pawnEntry.BLeverBB, wWingMask, bWingMask, oppositeSides)

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

	if debug {
		println("################### FINAL PIECE EVALUATION ###################")
		println("Pawn: ", pawnMG, ":", pawnEG)
		println("Knight: ", knightMG, ":", knightEG)
		println("Bishop: ", bishopMG, ":", bishopEG)
		println("Rook: ", rookMG, ":", rookEG)
		println("Queen: ", queenMG, ":", queenEG)
	}

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
		if wPieceCount > 0 && bPieceCount == 0 {
			kingMopUpBonus = getKingMopUpBonus(b, true, b.White.Queens > 0, b.White.Rooks > 0)
		} else if wPieceCount == 0 && bPieceCount > 0 {
			kingMopUpBonus = getKingMopUpBonus(b, false, b.Black.Queens > 0, b.Black.Rooks > 0)
		} else {
			kingCentralManhattanPenalty = kingEndGameCentralizationPenalty(b)
		}
	}

	kingMG = kingPsqtMG + kingAttackPenaltyMG + kingPawnShieldPenaltyMG +
		KingMinorPieceDefenseBonusMG + kingPawnDefenseMG
	kingEG = kingPsqtEG + kingAttackPenaltyEG + kingCentralManhattanPenalty +
		kingMopUpBonus + kingPawnDefenseMG

	if debug {
		println("King MG:\t", "PSQT: ", kingPsqtMG, "\tAttack: ", kingAttackPenaltyMG,
			"\tFile: ", kingPawnShieldPenaltyMG, "\tMinorDefense: ", KingMinorPieceDefenseBonusMG,
			"\tPawnDefense: ", kingPawnDefenseMG)
		println("King EG:\t", "PSQT: ", kingPsqtEG, "\tAttack: ", kingAttackPenaltyEG,
			"\tCmd: ", kingCentralManhattanPenalty, "\tMopUp: ", kingMopUpBonus,
			"\tPawnDefense: ", kingPawnDefenseMG)
	}

	var movementBB = [2][5]uint64{
		{knightMovementBB[0], bishopMovementBB[0], rookMovementBB[0], queenMovementBB[0]},
		{knightMovementBB[1], bishopMovementBB[1], rookMovementBB[1], queenMovementBB[1]},
	}

	// Weak squares & protected squares (unchanged call)
	weakSquareMG, protectedSquaresMG, weakSquares, weakKingSquares, protectedSquares := weakSquaresPenalty(movementBB, innerKingSafetyZones, wPawnAttackBB, bPawnAttackBB)

	_ = weakKingSquares
	_ = protectedSquares

	// Queen infiltration (still computed but unused in score, as in your original)
	queenInfiltrationMG, queenInfiltrationEG := queenInfiltrationBonus(b, weakSquares, wPawnAttackSpan, bPawnAttackSpan)

	// FINAL SCORE CALCULATION (unchanged)
	materialScoreMG := wMaterialMG - bMaterialMG
	materialScoreEG := wMaterialEG - bMaterialEG

	toMoveBonus := TempoBonus
	if !b.Wtomove {
		toMoveBonus = -TempoBonus
	}

	imbalanceMG, imbalanceEG := materialImbalance(b)

	variableScoreMG := pawnMG + knightMG + bishopMG + rookMG + queenMG + kingMG + toMoveBonus + imbalanceMG + weakSquareMG + protectedSquaresMG + queenInfiltrationMG
	variableScoreEG := pawnEG + knightEG + bishopEG + rookEG + queenEG + kingEG + toMoveBonus + imbalanceEG + queenInfiltrationEG

	mgScore := materialScoreMG + variableScoreMG
	egScore := materialScoreEG + variableScoreEG

	mgWeight := piecePhase
	egWeight := TotalPhase - piecePhase
	score = int16((mgScore*mgWeight + egScore*egWeight) / TotalPhase)

	if isTheoreticalDraw(b, debug) {
		score = score / DrawDivider
	}

	if isQuiescence && b.HalfmoveClock() > 8 {
		println("Quiescence eval: ", score, " ---- FEN: ", b.ToFen())
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
