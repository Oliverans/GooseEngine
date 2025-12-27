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

// Init initialized masks
var PositionBB [65]uint64
var PassedMaskWhite [64]uint64
var PassedMaskBlack [64]uint64

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

// Dark and light square bitmasks
const lightSquares uint64 = 0x55AA55AA55AA55AA
const darkSquares uint64 = 0xAA55AA55AA55AA55

// King safety attacker unit weights (inner ring and outer ring)
var attackerInner = [7]int{
	gm.PieceTypePawn: 1, gm.PieceTypeKnight: 2, gm.PieceTypeBishop: 2,
	gm.PieceTypeRook: 4, gm.PieceTypeQueen: 6, gm.PieceTypeKing: 0,
}
var attackerOuter = [7]int{
	gm.PieceTypePawn: 0, gm.PieceTypeKnight: 1, gm.PieceTypeBishop: 1,
	gm.PieceTypeRook: 2, gm.PieceTypeQueen: 2, gm.PieceTypeKing: 0,
}

var PSQT_MG = [7][64]int{
	gm.PieceTypePawn: {
		0, 0, 0, 0, 0, 0, 0, 0,
		-12, -6, -8, -3, -6, 9, 8, -13,
		-18, -18, -14, -13, -4, -14, -7, -20,
		-11, -10, -4, -3, 5, -1, -7, -18,
		4, 7, 9, 25, 32, 26, 10, -3,
		1, 30, 47, 55, 46, 55, 24, 1,
		2, 37, 122, 71, 95, 60, 119, 85,
		0, 0, 0, 0, 0, 0, 0, 0,
	},
	gm.PieceTypeKnight: {
		-24, -9, -9, -6, -2, -6, -5, -74,
		-14, -11, 0, 9, 7, -3, -13, -4,
		-16, 1, 5, 11, 13, 5, -2, -12,
		3, 11, 12, 14, 21, 13, 21, 7,
		12, 12, 31, 30, 22, 38, 16, 14,
		31, 48, 83, 62, 67, 51, 53, -28,
		-7, 5, 53, 33, 40, 68, -35, -57,
		-109, -15, -95, 58, -46, -35, -89, -167,
	},
	gm.PieceTypeBishop: {
		11, -1, -6, -6, -9, -2, -3, 5,
		9, 15, 13, 6, 7, 10, 14, 6,
		2, 14, 12, 13, 9, 11, 9, 5,
		0, 8, 10, 22, 23, 3, 8, 1,
		-3, 16, 19, 35, 33, 21, 19, 3,
		1, 30, 43, 31, 41, 44, 37, 3,
		-40, 8, 42, 23, -8, -13, 11, -21,
		-11, 6, -43, -26, -38, -82, 4, -29,
	},
	gm.PieceTypeRook: {
		-11, -4, 3, 9, 4, 2, 0, -12,
		-62, -12, -10, -8, -15, -12, -10, -49,
		-28, -9, -12, -9, -18, -18, -10, -33,
		-22, -4, -11, -2, -17, -16, -15, -25,
		-13, 1, 17, 23, 10, 3, -1, -12,
		7, 49, 38, 31, 35, 30, 28, 1,
		21, 14, 47, 65, 40, 47, 22, 22,
		43, 34, 12, 54, 41, 30, 43, 34,
	},
	gm.PieceTypeQueen: {
		9, 11, 18, 23, 26, 9, 10, 4,
		8, 19, 24, 23, 24, 27, 20, -15,
		1, 16, 13, 10, 10, 7, 19, -1,
		2, 4, -3, -1, -11, -11, -4, -4,
		-9, -9, -13, -28, -27, -24, -13, -14,
		15, 16, 16, -6, -22, -10, -25, -25,
		11, -22, 24, -29, -35, -17, -38, -4,
		29, 37, 37, 44, 0, 25, 1, -19,
	},
	gm.PieceTypeKing: {
		2, 14, -13, -24, -23, -31, 10, 12,
		4, -7, -20, -53, -47, -31, -4, 9,
		-27, -17, -23, -36, -30, -12, -6, -16,
		-51, -30, -38, -41, -33, -21, 4, -47,
		-36, -13, -24, -29, -26, -9, -17, -18,
		-22, 22, 6, -19, -15, 5, 26, -9,
		-29, -38, -4, -8, -7, -19, 0, 29,
		13, 2, -34, -56, -15, 16, 23, -65,
	},
}
var PSQT_EG = [7][64]int{
	gm.PieceTypePawn: {
		0, 0, 0, 0, 0, 0, 0, 0,
		14, 11, 18, 20, 25, 24, 8, 0,
		5, 1, 5, 4, 7, 9, -2, 1,
		10, 8, -1, -3, -4, 2, 3, 8,
		29, 23, 16, -2, 3, 10, 21, 24,
		62, 70, 49, 51, 62, 62, 81, 65,
		159, 147, 118, 130, 119, 142, 149, 148,
		0, 0, 0, 0, 0, 0, 0, 0,
	},
	gm.PieceTypeKnight: {
		-61, -36, -13, -5, 0, -5, -37, -23,
		-36, -14, -17, -1, 1, -11, -3, -26,
		-26, -16, -6, 10, 8, -9, -11, -25,
		-9, 1, 14, 16, 17, 15, 0, -7,
		-12, 0, 10, 21, 20, 12, 2, -8,
		-38, -16, -8, -1, 1, 15, -18, -15,
		-40, -20, -26, 1, -1, -29, -7, -18,
		-98, -60, -17, -32, -22, -11, -35, -57,
	},
	gm.PieceTypeBishop: {
		-15, -1, -7, -5, -2, -7, -7, -17,
		-23, -19, -9, -1, -4, -11, -11, -20,
		-11, -4, 3, 6, 5, 0, -7, -7,
		-10, -3, 9, 4, 5, 7, -2, -8,
		2, 3, 0, 4, 3, 5, 7, 2,
		4, 4, 2, 0, 1, 6, 2, 8,
		-4, 1, -12, -3, -2, 11, -1, -3,
		-19, -12, -4, 0, 0, -5, -16, -11,
	},
	gm.PieceTypeRook: {
		-11, -5, -5, -12, -14, -5, -6, -16,
		-1, -16, -14, -15, -14, -15, -10, -9,
		-11, -4, -8, -10, -7, -8, 0, -6,
		3, 6, 8, 0, 1, 6, 11, 5,
		14, 12, 5, 3, -1, 4, 8, 11,
		13, -3, 5, 5, -3, 6, 2, 11,
		0, 5, -5, -9, -6, -11, -3, -4,
		10, 15, 21, 3, 5, 18, 14, 18,
	},
	gm.PieceTypeQueen: {
		-27, -13, -24, 1, -33, -22, -24, -33,
		-27, -27, -25, -4, -9, -32, -17, -16,
		4, 11, 20, 14, 11, 24, -12, -12,
		23, 39, 33, 40, 50, 25, 37, -6,
		32, 56, 30, 51, 44, 27, 35, 15,
		1, 12, 22, 34, 37, 5, 8, -11,
		-1, 30, 14, 53, 32, 26, 26, -3,
		10, 7, 14, 12, 18, 19, 25, -1,
	},
	gm.PieceTypeKing: {
		-48, -27, -7, -32, -35, -12, -27, -67,
		-10, -3, 6, 6, 6, 7, -6, -21,
		-6, 5, 16, 21, 18, 11, -1, -12,
		-11, 12, 26, 26, 23, 19, 5, -14,
		4, 25, 30, 23, 22, 25, 25, -5,
		13, 39, 37, 18, 14, 31, 31, 9,
		8, 27, 35, 14, 14, 17, 25, -14,
		-19, 3, 14, -11, -18, -18, -34, -74,
	},
}
var pieceValueMG = [7]int{gm.PieceTypeKing: 0, gm.PieceTypePawn: 75, gm.PieceTypeKnight: 326, gm.PieceTypeBishop: 324, gm.PieceTypeRook: 474, gm.PieceTypeQueen: 1032}
var pieceValueEG = [7]int{gm.PieceTypeKing: 0, gm.PieceTypePawn: 95, gm.PieceTypeKnight: 328, gm.PieceTypeBishop: 332, gm.PieceTypeRook: 550, gm.PieceTypeQueen: 1014}
var mobilityValueMG = [7]int{gm.PieceTypeKing: 0, gm.PieceTypePawn: 0, gm.PieceTypeKnight: 2, gm.PieceTypeBishop: 4, gm.PieceTypeRook: 0, gm.PieceTypeQueen: 0}
var mobilityValueEG = [7]int{gm.PieceTypeKing: 0, gm.PieceTypePawn: 0, gm.PieceTypeKnight: 5, gm.PieceTypeBishop: 5, gm.PieceTypeRook: 5, gm.PieceTypeQueen: 4}
var KnightMobilityMG = [9]int{-25, -4, 0, 2, 5, 8, 12, 18, 26}
var KnightMobilityEG = [9]int{-72, -27, 5, 21, 29, 36, 37, 31, 20}
var BishopMobilityMG = [14]int{14, 21, 27, 30, 34, 36, 36, 35, 37, 41, 55, 66, 87, 91}
var BishopMobilityEG = [14]int{-52, -9, 16, 33, 48, 59, 66, 69, 72, 70, 65, 62, 81, 77}
var RookMobilityMG = [15]int{7, 6, 6, 6, 4, 5, 5, 7, 9, 10, 9, 12, 18, 26, 44}
var RookMobilityEG = [15]int{-36, 29, 59, 78, 94, 103, 111, 113, 116, 121, 124, 127, 126, 117, 104}
var QueenMobilityMG = [22]int{-14, 17, 35, 42, 45, 46, 48, 49, 51, 50, 51, 51, 48, 45, 44, 42, 45, 47, 53, 61, 77, 83}
var QueenMobilityEG = [22]int{-48, -30, -5, 24, 46, 63, 76, 91, 98, 108, 112, 116, 127, 132, 134, 134, 133, 135, 136, 138, 137, 140}
var PassedPawnPSQT_MG = [64]int{
	0, 0, 0, 0, 0, 0, 0, 0,
	-10, -4, -9, -10, -5, -11, 6, 9,
	-1, -5, -15, -17, -7, -1, 2, 12,
	10, 5, -15, -9, -14, -14, 9, 11,
	22, 20, 11, 10, 8, 6, 11, 11,
	52, 30, 35, 19, 24, 32, 23, 11,
	55, 55, 40, 46, 31, 36, 11, -3,
	0, 0, 0, 0, 0, 0, 0, 0,
}
var PassedPawnPSQT_EG = [64]int{
	0, 0, 0, 0, 0, 0, 0, 0,
	19, 14, 5, 2, -1, -3, 7, 15,
	22, 27, 17, 16, 12, 7, 24, 12,
	36, 41, 36, 30, 32, 33, 42, 31,
	52, 54, 46, 43, 39, 39, 50, 43,
	88, 73, 69, 45, 34, 51, 55, 76,
	50, 57, 52, 37, 46, 47, 46, 47,
	0, 0, 0, 0, 0, 0, 0, 0,
}

var (
	BackwardPawnMG       = -1
	BackwardPawnEG       = 5
	IsolatedPawnMG       = 5
	IsolatedPawnEG       = 9
	PawnDoubledMG        = 6
	PawnDoubledEG        = 17
	PawnConnectedMG      = 15
	PawnConnectedEG      = 8
	PawnPhalanxMG        = 7
	PawnPhalanxEG        = 10
	PawnWeakLeverMG      = 2
	PawnWeakLeverEG      = 5
	PawnBlockedMG        = -8
	PawnBlockedEG        = -8
	CandidatePassedPctMG = 13
	CandidatePassedPctEG = 11

	KnightOutpostMG = 25
	KnightOutpostEG = 15
	KnightTropismMG = 3
	KnightTropismEG = 3

	BishopOutpostMG = 20
	BishopOutpostEG = 10
	BadBishopMG     = -6
	BadBishopEG     = -14

	BishopPairBonusMG = 23
	BishopPairBonusEG = 57

	RookStackedMG     = 25
	RookSeventhRankEG = 11
	RookSemiOpenMG    = 13
	RookOpenMG        = 27

	QueenCentralizationEG = 5

	KingOpenFileMG          = 15
	KingSemiOpenFileMG      = 15
	KingMinorDefenseBonusMG = 3
	KingPawnDefenseBonusMG  = 0
	KingPasserProximityEG   = 1
	KingPasserProximityDiv  = 10
	KingPasserEnemyWeight   = 5
	KingPasserOwnWeight     = 2

	SpaceBonusMG            = 3
	SpaceBonusEG            = 3
	WeakKingSquarePenaltyMG = 5

	PawnStormBaseMG             = [8]int{0, 0, 0, 5, 11, 4, 8, 0}
	PawnStormFreePct            = [8]int{0, 0, 0, 100, 100, 100, 100, 0}
	PawnStormLeverPct           = [8]int{0, 0, 0, 76, 80, 85, 90, 0}
	PawnStormWeakLeverPct       = [8]int{0, 0, 0, 55, 60, 65, 70, 0}
	PawnStormBlockedPct         = [8]int{0, 0, 0, 36, 36, 45, 50, 0}
	PawnStormOppositeMultiplier = 149

	TempoBonus        = 10
	DrawDivider int32 = 8
)

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

var ImbalanceRefPawnCount = 5
var ImbalanceKnightPerPawnMG = 3
var ImbalanceKnightPerPawnEG = 6
var ImbalanceBishopPerPawnMG = -3
var ImbalanceBishopPerPawnEG = -6

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

func isDarkSquare(sq int) bool {
	if PositionBB[sq]&darkSquares != 0 {
		return true
	} else {
		return false
	}
}

func mobilityIndex(cnt int, max int) int {
	if cnt < 0 {
		return 0
	}
	if cnt > max {
		return max
	}
	return cnt
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

	const White = 0
	const Black = 1

	wp := pieceCount[White][gm.PieceTypePawn]
	wn := pieceCount[White][gm.PieceTypeKnight]
	wb := pieceCount[White][gm.PieceTypeBishop]

	bp := pieceCount[Black][gm.PieceTypePawn]
	bn := pieceCount[Black][gm.PieceTypeKnight]
	bb := pieceCount[Black][gm.PieceTypeBishop]

	max := func(a, b int) int {
		if a > b {
			return a
		}
		return b
	}
	min := func(a, b int) int {
		if a < b {
			return a
		}
		return b
	}
	clamp := func(x, lo, hi int) int { return max(lo, min(hi, x)) }

	// Kaufman-ish pawn-count tilt (clamped deltas)
	wPawnDelta := clamp(wp-ImbalanceRefPawnCount, -4, 4)
	bPawnDelta := clamp(bp-ImbalanceRefPawnCount, -4, 4)

	imbMG += (wPawnDelta*wn*ImbalanceKnightPerPawnMG + wPawnDelta*wb*ImbalanceBishopPerPawnMG) -
		(bPawnDelta*bn*ImbalanceKnightPerPawnMG + bPawnDelta*bb*ImbalanceBishopPerPawnMG)

	imbEG += (wPawnDelta*wn*ImbalanceKnightPerPawnEG + wPawnDelta*wb*ImbalanceBishopPerPawnEG) -
		(bPawnDelta*bn*ImbalanceKnightPerPawnEG + bPawnDelta*bb*ImbalanceBishopPerPawnEG)

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
) (penaltyMG int) {
	wWeakKingSquares := kingInnerRing[0] &^ wPawnAttackBB &^ b.White.All
	bWeakKingSquares := kingInnerRing[1] &^ bPawnAttackBB &^ b.Black.All

	wCount := bits.OnesCount64(wWeakKingSquares)
	bCount := bits.OnesCount64(bWeakKingSquares)

	penaltyMG = (bCount - wCount) * WeakKingSquarePenaltyMG

	return penaltyMG
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

func candidatePassedBonus(
	b *gm.Board,
	wPassed, bPassed uint64,
	wLever, bLever uint64,
	wLeverPush, bLeverPush uint64,
) (bonusMG, bonusEG int, wCandidates, bCandidates uint64) {

	occ := b.White.All | b.Black.All

	for x := (wLever | wLeverPush) &^ wPassed; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		pawnBB := PositionBB[sq]
		bestMG, bestEG := 0, 0

		// Squares from which this pawn could make a capture
		captureOrigins := pawnBB & wLever
		if pawnBB&wLeverPush != 0 && sq < 56 {
			if front := PositionBB[sq+8]; front&occ == 0 {
				captureOrigins |= front
			}
		}

		// Evaluate each possible capture origin
		for originsBB := captureOrigins; originsBB != 0; originsBB &= originsBB - 1 {
			fromSq := bits.TrailingZeros64(originsBB)
			attacksE, attacksW := PawnCaptureBitboards(PositionBB[fromSq], true)

			// Check each enemy pawn we could capture; select the best passed pawn
			for targetsBB := (attacksE | attacksW) & b.Black.Pawns; targetsBB != 0; targetsBB &= targetsBB - 1 {
				capSq := bits.TrailingZeros64(targetsBB)
				if (b.Black.Pawns&^PositionBB[capSq])&PassedMaskWhite[capSq] == 0 {
					bestMG = max(bestMG, PassedPawnPSQT_MG[capSq]*CandidatePassedPctMG/100)
					bestEG = max(bestEG, PassedPawnPSQT_EG[capSq]*CandidatePassedPctEG/100)
				}
			}
		}

		if bestMG|bestEG != 0 {
			wCandidates |= pawnBB
			bonusMG += bestMG
			bonusEG += bestEG
		}
	}

	for x := (bLever | bLeverPush) &^ bPassed; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		pawnBB := PositionBB[sq]
		bestMG, bestEG := 0, 0

		captureOrigins := pawnBB & bLever
		if pawnBB&bLeverPush != 0 && sq >= 8 {
			if front := PositionBB[sq-8]; front&occ == 0 {
				captureOrigins |= front
			}
		}

		for originsBB := captureOrigins; originsBB != 0; originsBB &= originsBB - 1 {
			fromSq := bits.TrailingZeros64(originsBB)
			attacksE, attacksW := PawnCaptureBitboards(PositionBB[fromSq], false)

			for targetsBB := (attacksE | attacksW) & b.White.Pawns; targetsBB != 0; targetsBB &= targetsBB - 1 {
				capSq := bits.TrailingZeros64(targetsBB)
				if (b.White.Pawns&^PositionBB[capSq])&PassedMaskBlack[capSq] == 0 {
					revSq := FlipView[capSq]
					bestMG = max(bestMG, PassedPawnPSQT_MG[revSq]*CandidatePassedPctMG/100)
					bestEG = max(bestEG, PassedPawnPSQT_EG[revSq]*CandidatePassedPctEG/100)
				}
			}
		}

		if bestMG|bestEG != 0 {
			bCandidates |= pawnBB
			bonusMG -= bestMG
			bonusEG -= bestEG
		}
	}

	return
}

func blockedPawnBonus(wBlocked uint64, bBlocked uint64) (blockedBonusMG int, blockedBonusEG int) {
	thirdAndFourthRank := onlyRank[2] | onlyRank[3]
	fifthAndSixthRank := onlyRank[4] | onlyRank[5]

	// Center is "equal" - higher up is good for white / lower good for black, so we only check the uneven ones
	wCount := bits.OnesCount64(wBlocked & fifthAndSixthRank)
	bCount := bits.OnesCount64(bBlocked & thirdAndFourthRank)
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

func evaluatePawnStorm(b *gm.Board, entry *PawnHashEntry, debug bool) (stormMG int) {
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
		pawnBB := PositionBB[sq]
		rank := sq / 8

		bonus := PawnStormBaseMG[rank]
		if bonus == 0 {
			continue
		}

		pct := PawnStormFreePct[rank]

		if PositionBB[sq+8]&b.Black.Pawns != 0 {
			pct = PawnStormBlockedPct[rank]
		} else if pawnBB&entry.WLeverBB != 0 {
			pct = PawnStormLeverPct[rank]
		} else if pawnBB&entry.WWeakLeverBB != 0 {
			pct = PawnStormWeakLeverPct[rank]
		}

		wStormScore += (bonus * pct) / 100
	}

	// 6) Black's storm: black pawns in the zone around White's king.
	var bStormScore int
	for x := b.Black.Pawns & wKingZone; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		pawnBB := PositionBB[sq]
		rank := sq / 8
		sideRank := 7 - rank

		bonus := PawnStormBaseMG[sideRank]
		if bonus == 0 {
			continue
		}

		pct := PawnStormFreePct[sideRank]

		if PositionBB[sq-8]&b.White.Pawns != 0 {
			pct = PawnStormBlockedPct[sideRank]
		} else if pawnBB&entry.BLeverBB != 0 {
			pct = PawnStormLeverPct[sideRank]
		} else if pawnBB&entry.BWeakLeverBB != 0 {
			pct = PawnStormWeakLeverPct[sideRank]
		}

		bStormScore += (bonus * pct) / 100
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

func badBishopPenalty(sq, darkFixed int, lightFixed int) (bishopBadMG int, bishopBadEG int) {
	if isDarkSquare(sq) {
		bishopBadMG += darkFixed * BadBishopMG
		bishopBadEG += darkFixed * BadBishopEG
	} else {
		bishopBadMG += lightFixed * BadBishopMG
		bishopBadEG += lightFixed * BadBishopEG
	}
	return bishopBadMG, bishopBadEG
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

func kingFilesPenalty(b *gm.Board, openFiles, wSemiOpenFiles, bSemiOpenFiles uint64) int {
	wKingFile := onlyFile[bits.TrailingZeros64(b.White.Kings)%8]
	bKingFile := onlyFile[bits.TrailingZeros64(b.Black.Kings)%8]

	wKingFiles := wKingFile | ((wKingFile & ^bitboardFileA) >> 1) | ((wKingFile & ^bitboardFileH) << 1)
	bKingFiles := bKingFile | ((bKingFile & ^bitboardFileA) >> 1) | ((bKingFile & ^bitboardFileH) << 1)

	wSemiCnt := bits.OnesCount64(wKingFiles&wSemiOpenFiles) / 8
	wOpenCnt := bits.OnesCount64(wKingFiles&openFiles) / 8
	bSemiCnt := bits.OnesCount64(bKingFiles&bSemiOpenFiles) / 8
	bOpenCnt := bits.OnesCount64(bKingFiles&openFiles) / 8

	wPenalty := wSemiCnt*KingSemiOpenFileMG + wOpenCnt*KingOpenFileMG
	bPenalty := bSemiCnt*KingSemiOpenFileMG + bOpenCnt*KingOpenFileMG

	return bPenalty - wPenalty
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

func kingPasserProximity(b *gm.Board, entry *PawnHashEntry) int {
	wKingSq := bits.TrailingZeros64(b.White.Kings)
	bKingSq := bits.TrailingZeros64(b.Black.Kings)
	score := 0

	for x := entry.WPassedBB; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		rank := sq / 8
		if rank < 3 {
			continue
		}
		blockSq := sq + 8
		enemyDist := chebyshevDistance(blockSq, bKingSq)
		ownDist := chebyshevDistance(blockSq, wKingSq)
		delta := (enemyDist * KingPasserEnemyWeight) - (ownDist * KingPasserOwnWeight)
		rankSq := rank * rank
		score += (delta * rankSq * KingPasserProximityEG) / KingPasserProximityDiv
	}

	for x := entry.BPassedBB; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		rank := sq / 8
		sideRank := 7 - rank
		if sideRank < 3 {
			continue
		}
		blockSq := sq - 8
		enemyDist := chebyshevDistance(blockSq, wKingSq)
		ownDist := chebyshevDistance(blockSq, bKingSq)
		delta := (enemyDist * KingPasserEnemyWeight) - (ownDist * KingPasserOwnWeight)
		rankSq := sideRank * sideRank
		score -= (delta * rankSq * KingPasserProximityEG) / KingPasserProximityDiv
	}

	return score
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
		idx := mobilityIndex(popCnt, len(KnightMobilityMG)-1)
		knightMobilityMG += KnightMobilityMG[idx]
		knightMobilityEG += KnightMobilityEG[idx]
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
		idx := mobilityIndex(popCnt, len(KnightMobilityMG)-1)
		knightMobilityMG -= KnightMobilityMG[idx]
		knightMobilityEG -= KnightMobilityEG[idx]
		(*attackUnitCounts)[1] += bits.OnesCount64(attackedSquares&innerKingSafetyZones[0]) * attackerInner[gm.PieceTypeKnight]
		(*attackUnitCounts)[1] += bits.OnesCount64(attackedSquares&outerKingSafetyZones[0]) * attackerOuter[gm.PieceTypeKnight]
	}

	knightOutpostMG := KnightOutpostMG*bits.OnesCount64(b.White.Knights&whiteOutposts) -
		KnightOutpostMG*bits.OnesCount64(b.Black.Knights&blackOutposts)
	knightOutpostEG := KnightOutpostEG*bits.OnesCount64(b.White.Knights&whiteOutposts) -
		KnightOutpostEG*bits.OnesCount64(b.Black.Knights&blackOutposts)

	knightTropismBonusMG, knightTropismBonusEG := knightKingTropism(b)
	knightMobilityMG = (knightMobilityMG * knightMobilityScale) / 100

	knightMG = knightPsqtMG + knightOutpostMG + knightMobilityMG + knightTropismBonusMG
	knightEG = knightPsqtEG + knightOutpostEG + knightMobilityEG + knightTropismBonusEG

	if debug {
		println("Knight MG:\t", "PSQT: ", knightPsqtMG, "\tMobility: ", knightMobilityMG,
			"\tOutpost: ", knightOutpostMG, "\tTropism: ", knightTropismBonusMG)
		println("Knight EG:\t", "PSQT: ", knightPsqtEG, "\tMobility: ", knightMobilityEG,
			"\tOutpost: ", knightOutpostEG, "\tTropism: ", knightTropismBonusEG)
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
	wBlockedPawns, bBlockedPawns uint64,
	bishopMovementBB *[2]uint64,
	kingAttackMobilityBB *[2]uint64,
	attackUnitCounts *[2]int,
	debug bool,
) (bishopMG, bishopEG int) {

	bishopPsqtMG, bishopPsqtEG := countPieceTables(&b.White.Bishops, &b.Black.Bishops,
		&PSQT_MG[gm.PieceTypeBishop], &PSQT_EG[gm.PieceTypeBishop])

	var bishopMobilityMG, bishopMobilityEG int
	var bishopBadMG, bishopBadEG int

	// Prepare pawn color layout
	wLightFixed := bits.OnesCount64(wBlockedPawns & lightSquares)
	wDarkFixed := bits.OnesCount64(wBlockedPawns & darkSquares)
	bLightFixed := bits.OnesCount64(bBlockedPawns & lightSquares)
	bDarkFixed := bits.OnesCount64(bBlockedPawns & darkSquares)

	for x := b.White.Bishops; x != 0; x &= x - 1 {
		square := bits.TrailingZeros64(x)
		wBishopBadMG, wBishopBadEG := badBishopPenalty(square, wDarkFixed, wLightFixed)
		bishopBadMG += wBishopBadMG
		bishopBadEG += wBishopBadEG
		occupied := allPieces &^ PositionBB[square]
		bishopAttacks := gm.CalculateBishopMoveBitboard(uint8(square), occupied)
		(*kingAttackMobilityBB)[0] |= bishopAttacks &^ b.White.All
		(*bishopMovementBB)[0] |= bishopAttacks
		mobilitySquares := bishopAttacks &^ bPawnAttackBB &^ b.White.All
		popCnt := bits.OnesCount64(mobilitySquares)
		idx := mobilityIndex(popCnt, len(BishopMobilityMG)-1)
		bishopMobilityMG += BishopMobilityMG[idx]
		bishopMobilityEG += BishopMobilityEG[idx]
		(*attackUnitCounts)[0] += bits.OnesCount64(bishopAttacks&innerKingSafetyZones[1]) * attackerInner[gm.PieceTypeBishop]
		(*attackUnitCounts)[0] += bits.OnesCount64(bishopAttacks&outerKingSafetyZones[1]) * attackerOuter[gm.PieceTypeBishop]
	}
	for x := b.Black.Bishops; x != 0; x &= x - 1 {
		square := bits.TrailingZeros64(x)
		bBishopBadMG, bBishopBadEG := badBishopPenalty(square, bDarkFixed, bLightFixed)
		bishopBadMG -= bBishopBadMG
		bishopBadEG -= bBishopBadEG
		occupied := allPieces &^ PositionBB[square]
		bishopAttacks := gm.CalculateBishopMoveBitboard(uint8(square), occupied)
		(*kingAttackMobilityBB)[1] |= bishopAttacks &^ b.Black.All
		(*bishopMovementBB)[1] |= bishopAttacks
		mobilitySquares := bishopAttacks &^ wPawnAttackBB &^ b.Black.All
		popCnt := bits.OnesCount64(mobilitySquares)
		idx := mobilityIndex(popCnt, len(BishopMobilityMG)-1)
		bishopMobilityMG -= BishopMobilityMG[idx]
		bishopMobilityEG -= BishopMobilityEG[idx]
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

	bishopMG = bishopPsqtMG + bishopOutpostMG + bishopPairMG + bishopMobilityMG + bishopBadMG
	bishopEG = bishopPsqtEG + bishopOutpostEG + bishopPairEG + bishopMobilityEG + bishopBadEG

	if debug {
		println("Bishop MG:\t", "PSQT: ", bishopPsqtMG, "\tMobility: ", bishopMobilityMG,
			"\tOutpost: ", bishopOutpostMG, "\tPair: ", bishopPairMG, "\tBadBishop: ", bishopBadMG)
		println("Bishop EG:\t", "PSQT: ", bishopPsqtEG, "\tMobility: ", bishopMobilityEG,
			"\tOutpost: ", bishopOutpostEG, "\tPair: ", bishopPairEG, "\tBadBishop: ", bishopBadEG)
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
		idx := mobilityIndex(popCnt, len(RookMobilityMG)-1)
		rookMobilityMG += RookMobilityMG[idx]
		rookMobilityEG += RookMobilityEG[idx]
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
		idx := mobilityIndex(popCnt, len(RookMobilityMG)-1)
		rookMobilityMG -= RookMobilityMG[idx]
		rookMobilityEG -= RookMobilityEG[idx]
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
		idx := mobilityIndex(popCnt, len(QueenMobilityMG)-1)
		queenMobilityMG += QueenMobilityMG[idx]
		queenMobilityEG += QueenMobilityEG[idx]
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
		idx := mobilityIndex(popCnt, len(QueenMobilityMG)-1)
		queenMobilityMG -= QueenMobilityMG[idx]
		queenMobilityEG -= QueenMobilityEG[idx]
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

	stormMG := evaluatePawnStorm(b, pawnEntry, debug)
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
		fmt.Printf("BB Candidate W/B: %016x / %016x\n", pawnEntry.WCandidateBB, pawnEntry.BCandidateBB)

		println("################### EXTRA BITBOARDS ###################")
		fmt.Printf("BB Outpost W/B: %016x / %016x\n", whiteOutposts, blackOutposts)

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
		pawnEntry.WBlockedBB, pawnEntry.BBlockedBB,
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
	kingPasserProximityEG := kingPasserProximity(b, pawnEntry)

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

	kingMG = kingPsqtMG + kingAttackPenaltyMG + kingPawnShieldPenaltyMG + KingMinorPieceDefenseBonusMG + kingPawnDefenseMG
	kingEG = kingPsqtEG + kingAttackPenaltyEG + kingCentralManhattanPenalty + kingMopUpBonus + kingPasserProximityEG

	if debug {
		println("King MG:\t", "PSQT: ", kingPsqtMG, "\tAttack: ", kingAttackPenaltyMG,
			"\tFile: ", kingPawnShieldPenaltyMG, "\tMinorDefense: ", KingMinorPieceDefenseBonusMG,
			"\tPawnDefense: ", kingPawnDefenseMG)
		println("King EG:\t", "PSQT: ", kingPsqtEG, "\tAttack: ", kingAttackPenaltyEG,
			"\tCmd: ", kingCentralManhattanPenalty, "\tMopUp: ", kingMopUpBonus,
			"\tPawnDefense: ", kingPawnDefenseMG, "\tPasser proximity: ", kingPasserProximityEG)
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
	weakKingMG := weakKingSquaresPenalty(b, wPawnAttackBB, bPawnAttackBB, innerKingSafetyZones)

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
		println("Weak king MG: ", weakKingMG)
	}

	variableScoreMG := pawnMG + knightMG + bishopMG + rookMG + queenMG + kingMG + toMoveBonus + imbalanceMG + spaceMG + weakKingMG
	variableScoreEG := pawnEG + knightEG + bishopEG + rookEG + queenEG + kingEG + toMoveBonus + imbalanceEG + spaceEG

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
		println("!!!--- NOTE: All scores are shown from white's perspective in the debug ---!!!")
		println("Final score:", score)
	}

	if !b.Wtomove {
		score = -score
	}

	return score
}
