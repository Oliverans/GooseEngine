package engine

import (
	"cmp"
	"math/bits"
	"os"

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

// Squares from which an enemy pawn could ever advance to attack this square:
var outpostBlockersWhite [64]uint64
var outpostBlockersBlack [64]uint64

// Outpost and rank masks
var wAllowedOutpostMask uint64 = 0x007e7e7e7e000000
var bAllowedOutpostMask uint64 = 0x0000007e7e7e7e00
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

var PSQT_MG = [7][64]int{
	gm.PieceTypePawn: {
		0, 0, 0, 0, 0, 0, 0, 0,
		-5, -6, -4, 0, -2, 12, 9, -5,
		-8, -14, -7, -5, 3, -6, -2, -9,
		-3, -8, 0, 2, 8, 5, -6, -8,
		3, 2, 4, 18, 22, 16, 0, -3,
		7, 15, 21, 28, 30, 30, 17, 8,
		31, 32, 33, 34, 33, 32, 29, 28,
		0, 0, 0, 0, 0, 0, 0, 0,
	},
	gm.PieceTypeKnight: {
		-48, -20, -16, -14, -10, -11, -21, -48,
		-27, -12, -6, 3, 1, -7, -9, -19,
		-22, -8, -3, 5, 7, -3, -9, -20,
		-5, 3, 5, 7, 13, 5, 9, -4,
		-4, 4, 16, 20, 12, 22, 5, 0,
		-19, 5, 18, 21, 22, 17, 4, -18,
		-27, -11, 3, 9, 8, 2, -11, -28,
		-52, -30, -20, -20, -20, -21, -30, -51,
	},
	gm.PieceTypeBishop: {
		4, -5, -10, -10, -11, -7, -8, -2,
		-1, 10, 7, 3, 4, 4, 7, -1,
		-3, 7, 8, 11, 7, 7, 4, -2,
		-2, 2, 8, 17, 18, 2, 2, -1,
		-1, 12, 10, 23, 21, 13, 14, 1,
		1, 8, 11, 9, 10, 10, 8, 3,
		-16, -1, 0, -1, -1, 0, -1, -12,
		-21, -10, -11, -11, -11, -12, -10, -20,
	},
	gm.PieceTypeRook: {
		-10, -3, 5, 10, 5, 4, 0, -11,
		-18, -8, -8, -6, -9, -6, -2, -12,
		-15, -4, -6, -4, -8, -7, 0, -11,
		-11, -3, -3, -1, -7, -5, 0, -8,
		-6, 2, 4, 7, 1, 1, 2, -4,
		-3, 9, 6, 9, 5, 4, 5, -3,
		8, 11, 15, 17, 12, 14, 12, 9,
		5, 4, 2, 6, 5, 2, 2, 4,
	},
	gm.PieceTypeQueen: {
		-4, 0, 7, 17, 11, 0, -3, -10,
		-5, 6, 15, 15, 16, 14, 8, -10,
		-3, 11, 10, 6, 6, 7, 9, -5,
		2, 7, 2, 2, 0, -4, 3, -5,
		-2, 4, -4, -7, -4, -6, 1, -8,
		-6, -2, 2, -2, -4, -2, -7, -16,
		-10, -14, -1, -4, -11, -4, -4, -8,
		-18, -8, -4, 0, -2, -5, -9, -18,
	},
	gm.PieceTypeKing: {
		19, 33, 8, -6, -6, -3, 28, 26,
		22, 16, 0, -19, -16, -8, 14, 24,
		-10, -19, -17, -28, -25, -15, -13, -9,
		-30, -29, -38, -49, -48, -37, -26, -30,
		-39, -39, -49, -60, -60, -48, -38, -39,
		-40, -39, -49, -60, -60, -48, -38, -39,
		-40, -39, -50, -60, -60, -50, -39, -40,
		-40, -40, -50, -60, -60, -50, -40, -40,
	},
}
var PSQT_EG = [7][64]int{
	gm.PieceTypePawn: {
		0, 0, 0, 0, 0, 0, 0, 0,
		13, 12, 19, 16, 20, 23, 10, 2,
		7, 4, 9, 9, 11, 12, 2, 4,
		13, 11, 5, 3, 2, 6, 7, 11,
		23, 21, 17, 5, 6, 13, 19, 19,
		47, 52, 49, 45, 47, 51, 52, 47,
		70, 77, 79, 79, 80, 78, 78, 74,
		0, 0, 0, 0, 0, 0, 0, 0,
	},
	gm.PieceTypeKnight: {
		-49, -37, -28, -23, -22, -24, -36, -49,
		-38, -19, -13, -6, -5, -14, -18, -34,
		-31, -13, -4, 7, 5, -8, -14, -30,
		-24, -4, 9, 13, 13, 9, -4, -24,
		-24, -5, 10, 17, 17, 10, -3, -24,
		-29, -8, 10, 11, 10, 9, -9, -29,
		-38, -20, -10, -2, -4, -11, -21, -39,
		-51, -40, -29, -29, -29, -29, -40, -51,
	},
	gm.PieceTypeBishop: {
		-17, -8, -9, -9, -8, -9, -9, -19,
		-10, -9, -3, -1, -3, -4, -6, -12,
		-9, -1, 4, 5, 4, 1, -3, -8,
		-10, 0, 6, 8, 8, 5, 0, -10,
		-8, 1, 4, 10, 10, 5, 3, -7,
		-8, 4, 7, 6, 7, 9, 4, -7,
		-11, 1, 1, 0, 0, 1, 0, -10,
		-19, -9, -9, -8, -8, -10, -10, -19,
	},
	gm.PieceTypeRook: {
		-8, -3, -3, -7, -10, -3, -2, -9,
		-7, -6, -6, -7, -9, -9, -4, -5,
		-6, -1, -3, -4, -6, -6, -1, -5,
		1, 4, 4, 1, -1, 0, 2, -1,
		9, 9, 10, 9, 5, 5, 6, 7,
		15, 13, 15, 13, 9, 12, 9, 11,
		6, 6, 8, 9, 6, 3, 4, 4,
		14, 14, 13, 10, 8, 11, 12, 13,
	},
	gm.PieceTypeQueen: {
		-17, -9, -10, -2, -6, -11, -9, -19,
		-9, 0, -5, 2, -1, -5, 0, -10,
		-9, 4, 8, 6, 5, 8, 2, -10,
		-3, 4, 7, 14, 13, 5, 3, -3,
		-4, 4, 4, 10, 10, 4, 3, -4,
		-8, 1, 6, 4, 3, 3, -1, -10,
		-6, 2, 1, 1, -2, -2, 0, -7,
		-17, -7, -8, -5, -6, -9, -8, -18,
	},
	gm.PieceTypeKing: {
		-51, -32, -17, -27, -33, -20, -30, -61,
		-24, -12, -3, -5, -6, -3, -12, -26,
		-18, 0, 8, 10, 10, 8, 0, -16,
		-20, 6, 16, 17, 17, 15, 8, -19,
		-16, 10, 18, 18, 18, 18, 12, -15,
		-18, 7, 14, 14, 14, 16, 10, -17,
		-30, -7, 1, 5, 5, 1, -5, -30,
		-50, -30, -20, -20, -20, -20, -29, -50,
	},
}
var pieceValueMG = [7]int{gm.PieceTypeKing: 0, gm.PieceTypePawn: 84, gm.PieceTypeKnight: 325, gm.PieceTypeBishop: 338, gm.PieceTypeRook: 496, gm.PieceTypeQueen: 951}
var pieceValueEG = [7]int{gm.PieceTypeKing: 0, gm.PieceTypePawn: 95, gm.PieceTypeKnight: 321, gm.PieceTypeBishop: 340, gm.PieceTypeRook: 548, gm.PieceTypeQueen: 1002}
var KnightMobilityMG = [9]int{-26, -9, -4, -1, 1, 5, 10, 17, 20}
var KnightMobilityEG = [9]int{-50, -20, 5, 20, 27, 32, 33, 28, 22}
var BishopMobilityMG = [14]int{-17, -7, 2, 7, 12, 15, 15, 15, 17, 19, 23, 26, 28, 29}
var BishopMobilityEG = [14]int{-43, -18, 3, 18, 32, 43, 50, 54, 56, 56, 56, 55, 63, 60}
var RookMobilityMG = [15]int{-6, -4, -3, -3, -5, -3, -3, 0, 2, 4, 5, 7, 10, 12, 18}
var RookMobilityEG = [15]int{-25, 7, 27, 46, 62, 73, 82, 86, 91, 96, 100, 103, 105, 100, 93}
var QueenMobilityMG = [22]int{-17, -3, 12, 19, 23, 26, 28, 30, 32, 33, 33, 33, 32, 31, 30, 30, 32, 35, 39, 44, 48, 48}
var QueenMobilityEG = [22]int{-40, -21, 0, 16, 32, 43, 56, 67, 78, 87, 95, 102, 106, 111, 115, 117, 119, 119, 120, 122, 123, 123}
var PassedPawnPSQT_MG = [64]int{
	0, 0, 0, 0, 0, 0, 0, 0,
	-2, 2, -1, 2, 2, -4, 2, 3,
	1, 2, -3, -4, -1, 0, 2, 3,
	6, 8, -3, -2, 4, 2, 11, 7,
	16, 20, 15, 17, 17, 15, 21, 18,
	37, 36, 38, 34, 34, 40, 41, 35,
	51, 53, 54, 54, 54, 53, 51, 48,
	0, 0, 0, 0, 0, 0, 0, 0,
}
var PassedPawnPSQT_EG = [64]int{
	0, 0, 0, 0, 0, 0, 0, 0,
	16, 14, 9, 9, 8, 4, 12, 16,
	19, 21, 14, 12, 11, 12, 20, 17,
	20, 24, 18, 19, 19, 21, 28, 22,
	34, 37, 34, 34, 37, 36, 41, 36,
	60, 62, 60, 56, 57, 60, 61, 60,
	71, 78, 79, 79, 80, 79, 78, 74,
	0, 0, 0, 0, 0, 0, 0, 0,
}

var (
	BackwardOpposedMG      = 4
	BackwardOpposedEG      = 8
	BackwardUnopposedMG    = 10
	BackwardUnopposedEG    = 17
	IsolatedOpposedMG      = 4
	IsolatedOpposedEG      = 8
	IsolatedUnopposedMG    = 14
	IsolatedUnopposedEG    = 20
	PawnDoubledOpposedMG   = 10
	PawnDoubledOpposedEG   = 22
	PawnDoubledUnopposedMG = 8
	PawnDoubledUnopposedEG = 17

	PawnWeakLeverMG      = 5
	PawnWeakLeverEG      = 8
	CandidatePassedPctMG = 17
	CandidatePassedPctEG = 12

	KnightOutpostMG = 25
	KnightOutpostEG = 15
	KnightTropismMG = 1
	KnightTropismEG = 3

	BishopOutpostMG = 15
	BishopOutpostEG = 10
	BadBishopMG     = -4
	BadBishopEG     = -16

	BishopPairBonusMG = 25
	BishopPairBonusEG = 50

	CenterKnightMobilityMG = 12
	CenterKnightMobilityEG = 4
	CenterBishopMobilityMG = 10
	CenterBishopMobilityEG = 3
	CenterBishopPairMG     = 7
	CenterBishopPairEG     = 2

	RookStackedMG = 20

	RookSeventhRankMG = 7
	RookSeventhRankEG = 15
	RookSemiOpenMG    = 15
	RookSemiOpenEG    = 7
	RookOpenMG        = 25
	RookOpenEG        = 12

	RookFileCountOpenMG = 5
	RookFileCountOpenEG = 3
	RookFileCountSemiMG = 3
	RookFileCountSemiEG = 2

	QueenCentralizationEG = 9

	SafetyKnightWeightMG = 48
	SafetyKnightWeightEG = 41
	SafetyBishopWeightMG = 24
	SafetyBishopWeightEG = 35
	SafetyRookWeightMG   = 36
	SafetyRookWeightEG   = 8
	SafetyQueenWeightMG  = 30
	SafetyQueenWeightEG  = 6

	SafetyAttackValueMG = 45
	SafetyAttackValueEG = 34

	// Grabbed from Ethereal; it values were -237/-259 as a reference
	// Lower value means larger bridge to gap to get a non-zero bonus attack without a queen involved
	SafetyNoEnemyQueensMG = -60
	SafetyNoEnemyQueensEG = -65

	SafetySafeKnightCheckMG = 78
	SafetySafeKnightCheckEG = 117
	SafetySafeBishopCheckMG = 41
	SafetySafeBishopCheckEG = 59
	SafetySafeRookCheckMG   = 63
	SafetySafeRookCheckEG   = 98
	SafetySafeQueenCheckMG  = 65
	SafetySafeQueenCheckEG  = 83

	SafetyUnsafeCheckMG = 11
	SafetyUnsafeCheckEG = 15

	// Safety limiter; without this you don't get an attack bonus at all
	// Also taken from Ethereal, which uses a -74/-26 offset
	SafetyAdjustmentMG = 0
	SafetyAdjustmentEG = 0

	// Scale raw king danger to centipawns
	SafetyMGDivisor = 47
	SafetyEGDivisor = 20

	KingMinorDefenseBonusMG = 3
	KingPasserProximityEG   = 1
	KingPasserProximityDiv  = 10
	KingPasserEnemyWeight   = 5
	KingPasserOwnWeight     = 2

	SpaceSafeMG       = 3
	SpaceBehindPawnMG = 3
	SpaceSemiOpenMG   = -1
	SpaceOpenMG       = -2

	SpaceWeightOffset = 3
	SpaceBlockedCap   = 9
	SpaceWeightDiv    = 268

	TempoBonus        = 11
	DrawDivider int32 = 8
)

var KingShelterMG = [4][8]int{
	{-1, 34, 38, 21, 16, 9, 10},
	{-22, 26, 13, -22, -12, -4, -24},
	{-4, 30, 8, -2, 10, 4, -19},
	{-16, -4, -11, -23, -17, -26, -65},
}

var KingStormUnblockedMG = [4][8]int{
	{27, 53, 53, 31, 16, 14, 16},
	{14, 39, 39, 14, 12, -3, 6},
	{-2, 54, 54, 11, -1, -7, -5},
	{-5, 32, 32, 2, 3, -5, -10},
}

var KingStormBlockedMG = [8]int{0, 0, 24, -3, -2, -2, 0, 0}
var KingStormBlockedEG = [8]int{0, 0, 25, 5, 3, 2, 1, 0}

var PawnConnectedMG = [7]int{0, 0, 4, 5, 13, 22, 40}

var PawnBlockedMG = [2]int{-5, -2}
var PawnBlockedEG = [2]int{-2, -5}

var ImbalanceRefPawnCount = 10
var ImbalanceKnightPerPawnMG = 3
var ImbalanceKnightPerPawnEG = 2

/* ============= HELPER VARIABLES ============= */
// Adjacent files only (the pawn's OWN file is excluded): a pawn is isolated when
// it has no friendly pawns on the files immediately left/right. Including the own
// file here was a bug that made the isolated test never fire.
var isolatedPawnTable = [8]uint64{
	0x0202020202020202, 0x0505050505050505, 0x0a0a0a0a0a0a0a0a, 0x1414141414141414,
	0x2828282828282828, 0x5050505050505050, 0xa0a0a0a0a0a0a0a0, 0x4040404040404040,
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
	wSpaceZoneMask uint64 = 0x000000003c3c3c00 // c2-f2, c3-f3, c4-f4
	bSpaceZoneMask uint64 = 0x003c3c3c00000000 // c7-f7, c6-f6, c5-f5
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
	pawnDelta := bits.OnesCount64(b.White.Pawns|b.Black.Pawns) - ImbalanceRefPawnCount
	knightDiff := bits.OnesCount64(b.White.Knights) - bits.OnesCount64(b.Black.Knights)

	units := pawnDelta * knightDiff
	return units * ImbalanceKnightPerPawnMG, units * ImbalanceKnightPerPawnEG
}

func spaceBonusFor(ownPawns, enemyPawnAttacks, zone, ownSemiOpenFiles, openFiles uint64, white bool) int {
	safe := zone &^ ownPawns &^ enemyPawnAttacks
	if safe == 0 {
		return 0
	}

	// Fill toward our own back rank: each pawn's square plus the three ranks
	// behind it. Our own pawns are already out of `safe`, so this picks up only
	// the ground they cover.
	behind := ownPawns
	if white {
		behind |= behind >> 8
		behind |= behind >> 16
	} else {
		behind |= behind << 8
		behind |= behind << 16
	}

	return bits.OnesCount64(safe)*SpaceSafeMG +
		bits.OnesCount64(safe&behind)*SpaceBehindPawnMG +
		bits.OnesCount64(safe&ownSemiOpenFiles)*SpaceSemiOpenMG +
		bits.OnesCount64(safe&openFiles)*SpaceOpenMG
}

func spaceEvaluation(b *gm.Board, entry *PawnHashEntry) (spaceMG int) {
	blocked := bits.OnesCount64(entry.WBlockedBB) + bits.OnesCount64(entry.BBlockedBB)
	if blocked > SpaceBlockedCap {
		blocked = SpaceBlockedCap
	}

	wWeight := bits.OnesCount64(b.White.All) - SpaceWeightOffset + blocked
	bWeight := bits.OnesCount64(b.Black.All) - SpaceWeightOffset + blocked
	if wWeight < 0 {
		wWeight = 0
	}
	if bWeight < 0 {
		bWeight = 0
	}

	return (entry.WSpaceBonus*wWeight*wWeight - entry.BSpaceBonus*bWeight*bWeight) / SpaceWeightDiv
}

/* ============= PAWN FUNCTIONS ============= */

func isolatedPawnPenalty(entry *PawnHashEntry) (isolatedMG int, isolatedEG int) {
	wOpp := bits.OnesCount64(entry.WIsolatedBB & entry.WOpposedBB)
	wUnopp := bits.OnesCount64(entry.WIsolatedBB &^ entry.WOpposedBB)
	bOpp := bits.OnesCount64(entry.BIsolatedBB & entry.BOpposedBB)
	bUnopp := bits.OnesCount64(entry.BIsolatedBB &^ entry.BOpposedBB)

	isolatedMG = (bOpp-wOpp)*IsolatedOpposedMG + (bUnopp-wUnopp)*IsolatedUnopposedMG
	isolatedEG = (bOpp-wOpp)*IsolatedOpposedEG + (bUnopp-wUnopp)*IsolatedUnopposedEG
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

func blockedPawnPenalty(wBlocked uint64, bBlocked uint64) (blockedMG int, blockedEG int) {
	for i := range 2 {
		// Relative rank 5 then 6: absolute ranks 5 and 6 for white, 4 and 3 for black.
		diff := bits.OnesCount64(wBlocked&onlyRank[4+i]) - bits.OnesCount64(bBlocked&onlyRank[3-i])
		blockedMG += diff * PawnBlockedMG[i]
		blockedEG += diff * PawnBlockedEG[i]
	}
	return blockedMG, blockedEG
}

func backwardPawnPenalty(entry *PawnHashEntry) (backMG int, backEG int) {
	wOpp := bits.OnesCount64(entry.WBackwardBB & entry.WOpposedBB)
	wUnopp := bits.OnesCount64(entry.WBackwardBB &^ entry.WOpposedBB)
	bOpp := bits.OnesCount64(entry.BBackwardBB & entry.BOpposedBB)
	bUnopp := bits.OnesCount64(entry.BBackwardBB &^ entry.BOpposedBB)

	backMG = (bOpp-wOpp)*BackwardOpposedMG + (bUnopp-wUnopp)*BackwardUnopposedMG
	backEG = (bOpp-wOpp)*BackwardOpposedEG + (bUnopp-wUnopp)*BackwardUnopposedEG
	return backMG, backEG
}

func pawnWeakLeverPenalty(wWeak uint64, bWeak uint64) (mg int, eg int) {
	wCount := bits.OnesCount64(wWeak)
	bCount := bits.OnesCount64(bWeak)
	diffMG := (bCount - wCount) * PawnWeakLeverMG
	diffEG := (bCount - wCount) * PawnWeakLeverEG
	return diffMG, diffEG
}

func phalanxPawns(own uint64) uint64 {
	return own & (((own << 1) &^ bitboardFileA) | ((own >> 1) &^ bitboardFileH))
}

func connectedPawnBonus(b *gm.Board, entry *PawnHashEntry) (mg, eg int) {
	wMG, wEG := connectedPawnsFor(b.White.Pawns, entry.WPawnAttackBB, entry.WOpposedBB, true)
	bMG, bEG := connectedPawnsFor(b.Black.Pawns, entry.BPawnAttackBB, entry.BOpposedBB, false)
	return wMG - bMG, wEG - bEG
}

func connectedPawnsFor(own, ownAttacks, opposed uint64, white bool) (mg, eg int) {
	phalanx := phalanxPawns(own)
	qualifying := phalanx | (own & ownAttacks)

	for r := 1; r < 7; r++ {
		mask := onlyRank[r]
		if !white {
			mask = onlyRank[7-r]
		}
		n := bits.OnesCount64(qualifying & mask)
		if n == 0 {
			continue
		}
		units := 2*n +
			bits.OnesCount64(phalanx&mask) -
			bits.OnesCount64(qualifying&opposed&mask)

		v := PawnConnectedMG[r] * units
		mg += v
		// Connected pawns low on the board are worth little once the pieces come
		// off, so the endgame share ramps with rank and is negative on the second.
		eg += v * (r - 2) / 4
	}
	return mg, eg
}

func doubledPawnBitboards(b *gm.Board) (wDoubled, bDoubled uint64) {
	wDoubled = b.White.Pawns & calculatePawnNorthFill(b.White.Pawns)
	bDoubled = b.Black.Pawns & calculatePawnSouthFill(b.Black.Pawns)
	return wDoubled, bDoubled
}

func pawnDoublingPenalties(b *gm.Board, entry *PawnHashEntry) (doubledMG, doubledEG int) {
	wDoubled, bDoubled := doubledPawnBitboards(b)

	wOpp := bits.OnesCount64(wDoubled & entry.WOpposedBB)
	wUnopp := bits.OnesCount64(wDoubled &^ entry.WOpposedBB)
	bOpp := bits.OnesCount64(bDoubled & entry.BOpposedBB)
	bUnopp := bits.OnesCount64(bDoubled &^ entry.BOpposedBB)

	doubledMG = (bOpp-wOpp)*PawnDoubledOpposedMG + (bUnopp-wUnopp)*PawnDoubledUnopposedMG
	doubledEG = (bOpp-wOpp)*PawnDoubledOpposedEG + (bUnopp-wUnopp)*PawnDoubledUnopposedEG
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

func rookSeventhRankBonus(b *gm.Board) (bonusMG, bonusEG int) {
	wRooksOnSeventh := bits.OnesCount64(b.White.Rooks & seventhRankMask)
	bRooksOnSecond := bits.OnesCount64(b.Black.Rooks & secondRankMask)

	// Base bonus per rook
	diff := wRooksOnSeventh - bRooksOnSecond
	bonusMG = diff * RookSeventhRankMG
	bonusEG = diff * RookSeventhRankEG

	// Extra bonus for doubled rooks on 7th (the "pigs")
	if wRooksOnSeventh >= 2 {
		bonusMG += RookSeventhRankMG * 2
		bonusEG += RookSeventhRankEG * 2
	}
	if bRooksOnSecond >= 2 {
		bonusMG -= RookSeventhRankMG * 2
		bonusEG -= RookSeventhRankEG * 2
	}

	return bonusMG, bonusEG
}

func rookFilesBonus(b *gm.Board, openFiles uint64, wSemiOpenFiles uint64, bSemiOpenFiles uint64) (semiOpenMG, semiOpenEG, openMG, openEG int) {
	whiteRooks := b.White.Rooks
	blackRooks := b.Black.Rooks

	semiDiff := bits.OnesCount64(wSemiOpenFiles&whiteRooks) - bits.OnesCount64(bSemiOpenFiles&blackRooks)
	semiOpenMG = RookSemiOpenMG * semiDiff
	semiOpenEG = RookSemiOpenEG * semiDiff

	openDiff := bits.OnesCount64(openFiles&whiteRooks) - bits.OnesCount64(openFiles&blackRooks)
	openMG = RookOpenMG * openDiff
	openEG = RookOpenEG * openDiff

	return semiOpenMG, semiOpenEG, openMG, openEG
}

func rookFileCountBonus(b *gm.Board, openFiles, wSemiOpenFiles, bSemiOpenFiles uint64) (mg, eg int) {
	openCount := bits.OnesCount64(openFiles) / 8
	wSemiCount := bits.OnesCount64(wSemiOpenFiles) / 8
	bSemiCount := bits.OnesCount64(bSemiOpenFiles) / 8

	wRooks := bits.OnesCount64(b.White.Rooks)
	bRooks := bits.OnesCount64(b.Black.Rooks)

	openDiff := openCount * (wRooks - bRooks)
	semiDiff := wRooks*wSemiCount - bRooks*bSemiCount

	mg = openDiff*RookFileCountOpenMG + semiDiff*RookFileCountSemiMG
	eg = openDiff*RookFileCountOpenEG + semiDiff*RookFileCountSemiEG
	return mg, eg
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

func kingShelterStorm(b *gm.Board, wPawnAttackBB, bPawnAttackBB uint64) (shelterMG, stormMG, stormEG int) {
	wShelter, wStormMG, wStormEG := kingShelterStormFor(b, true, bPawnAttackBB)
	bShelter, bStormMG, bStormEG := kingShelterStormFor(b, false, wPawnAttackBB)
	return wShelter - bShelter, wStormMG - bStormMG, wStormEG - bStormEG
}

func kingShelterStormFor(b *gm.Board, white bool, enemyPawnAttackBB uint64) (shelterMG, stormMG, stormEG int) {
	var kings, ourPawns, theirPawns uint64
	if white {
		kings, ourPawns, theirPawns = b.White.Kings, b.White.Pawns, b.Black.Pawns
	} else {
		kings, ourPawns, theirPawns = b.Black.Kings, b.Black.Pawns, b.White.Pawns
	}

	kingSq := bits.TrailingZeros64(kings)
	kingRank, kingFile := kingSq/8, kingSq%8

	shelterPawns := ourPawns &^ enemyPawnAttackBB
	if white {
		shelterPawns &= ranksAbove[kingRank]
		theirPawns &= ranksAbove[kingRank]
	} else {
		shelterPawns &= ranksBelow[kingRank]
		theirPawns &= ranksBelow[kingRank]
	}

	center := min(max(kingFile, 1), 6)
	for f := center - 1; f <= center+1; f++ {
		edgeDist := min(f, 7-f)
		ourRank := frontmostRelRank(shelterPawns&onlyFile[f], white)
		theirRank := frontmostRelRank(theirPawns&onlyFile[f], white)

		shelterMG += KingShelterMG[edgeDist][ourRank]

		if ourRank != 0 && ourRank == theirRank-1 {
			stormMG -= KingStormBlockedMG[theirRank]
			stormEG -= KingStormBlockedEG[theirRank]
		} else {
			stormMG -= KingStormUnblockedMG[edgeDist][theirRank]
		}
	}
	return shelterMG, stormMG, stormEG
}

func frontmostRelRank(bb uint64, white bool) int {
	if bb == 0 {
		return 0
	}
	if white {
		return bits.TrailingZeros64(bb) / 8
	}
	return 7 - (63-bits.LeadingZeros64(bb))/8
}

func safetyWeight(pt gm.PieceType) (mg, eg int) {
	switch pt {
	case gm.PieceTypeKnight:
		return SafetyKnightWeightMG, SafetyKnightWeightEG
	case gm.PieceTypeBishop:
		return SafetyBishopWeightMG, SafetyBishopWeightEG
	case gm.PieceTypeRook:
		return SafetyRookWeightMG, SafetyRookWeightEG
	case gm.PieceTypeQueen:
		return SafetyQueenWeightMG, SafetyQueenWeightEG
	}
	return 0, 0
}

type kingDanger struct {
	attackers [2]int // distinct attacking pieces; diagnostic only
	weightMG  [2]int // summed weights of those pieces
	weightEG  [2]int
	squares   [2]int // king-ring squares attacked, summed over attackers
	checkMG   [2]int // safe/unsafe check contribution
	checkEG   [2]int
	safeChk   [2]int // diagnostic counts only
	unsafeChk [2]int
}

func (k *kingDanger) addAttacker(side int, ringHits, weightMG, weightEG int) {
	if ringHits == 0 {
		return
	}
	k.attackers[side]++
	k.weightMG[side] += weightMG
	k.weightEG[side] += weightEG
	k.squares[side] += ringHits
}

func kingDangerRaw(k *kingDanger, side int, attackerHasQueen bool) (mg, eg int) {
	mg = k.weightMG[side] + SafetyAttackValueMG*k.squares[side] + k.checkMG[side] + SafetyAdjustmentMG
	eg = k.weightEG[side] + SafetyAttackValueEG*k.squares[side] + k.checkEG[side] + SafetyAdjustmentEG
	if !attackerHasQueen {
		mg += SafetyNoEnemyQueensMG
		eg += SafetyNoEnemyQueensEG
	}

	// Safety Adjustment is used here to have a minimum requirement to get any bonus; currently set at 0 pre-tuning
	if mg < 0 {
		mg = 0
	}
	if eg < 0 {
		eg = 0
	}
	return mg, eg
}

func kingDangerFor(k *kingDanger, side int, attackerHasQueen bool) (mg, eg int) {
	rawMG, rawEG := kingDangerRaw(k, side, attackerHasQueen)
	return (rawMG * rawMG) / (SafetyMGDivisor * SafetyMGDivisor), rawEG / SafetyEGDivisor
}

func rookAttackOccupancy(b *gm.Board, white bool) uint64 {
	ownRooks := b.White.Rooks
	if !white {
		ownRooks = b.Black.Rooks
	}
	return (b.White.All | b.Black.All) &^ (b.White.Queens | b.Black.Queens) &^ ownRooks
}

func kingCheckThreats(b *gm.Board, danger *kingDanger, allPieces uint64,
	wPawnAttackBB, bPawnAttackBB uint64,
	knightAttacks, bishopAttacks, queenAttacks [2]uint64) {

	var rookTrue [2]uint64
	for side := 0; side < 2; side++ {
		rooks := b.White.Rooks
		if side == 1 {
			rooks = b.Black.Rooks
		}
		for x := rooks; x != 0; x &= x - 1 {
			sq := bits.TrailingZeros64(x)
			rookTrue[side] |= gm.CalculateRookMoveBitboard(uint8(sq), allPieces&^PositionBB[sq])
		}
	}

	pawnAttacks := [2]uint64{wPawnAttackBB, bPawnAttackBB}
	allBySide := [2]uint64{b.White.All, b.Black.All}
	kingBySide := [2]uint64{b.White.Kings, b.Black.Kings}

	for def := 0; def < 2; def++ {
		atk := 1 - def
		if kingBySide[def] == 0 {
			continue
		}
		kingSq := bits.TrailingZeros64(kingBySide[def])

		knightCk := KnightMasks[kingSq]
		bishopCk := gm.CalculateBishopMoveBitboard(uint8(kingSq), allPieces)
		rookCk := gm.CalculateRookMoveBitboard(uint8(kingSq), allPieces)

		defended := pawnAttacks[def] | knightAttacks[def] | bishopAttacks[def] | rookTrue[def]

		var unsafe uint64
		fold := func(reach, checkMask uint64, safeMG, safeEG int) {
			landing := reach & checkMask &^ allBySide[atk]
			safe := landing &^ defended
			unsafe |= landing & defended
			if n := bits.OnesCount64(safe); n > 0 {
				danger.checkMG[atk] += safeMG * n
				danger.checkEG[atk] += safeEG * n
				danger.safeChk[atk] += n
			}
		}
		fold(knightAttacks[atk], knightCk, SafetySafeKnightCheckMG, SafetySafeKnightCheckEG)
		fold(bishopAttacks[atk], bishopCk, SafetySafeBishopCheckMG, SafetySafeBishopCheckEG)
		fold(rookTrue[atk], rookCk, SafetySafeRookCheckMG, SafetySafeRookCheckEG)
		fold(queenAttacks[atk], bishopCk|rookCk, SafetySafeQueenCheckMG, SafetySafeQueenCheckEG)

		if n := bits.OnesCount64(unsafe); n > 0 {
			danger.checkMG[atk] += SafetyUnsafeCheckMG * n
			danger.checkEG[atk] += SafetyUnsafeCheckEG * n
			danger.unsafeChk[atk] = n
		}
	}
}

func kingDangerScore(k *kingDanger, b *gm.Board) (mg, eg int) {
	toBlackMG, toBlackEG := kingDangerFor(k, 0, b.White.Queens != 0)
	toWhiteMG, toWhiteEG := kingDangerFor(k, 1, b.Black.Queens != 0)
	return toBlackMG - toWhiteMG, toBlackEG - toWhiteEG
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
	kingRing [2]uint64,
	scales centerScales,
	whiteOutposts, blackOutposts uint64,
	knightMovementBB *[2]uint64,
	kingAttackMobilityBB *[2]uint64,
	danger *kingDanger,
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
		danger.addAttacker(0, bits.OnesCount64(attackedSquares&kingRing[1]), SafetyKnightWeightMG, SafetyKnightWeightEG)
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
		danger.addAttacker(1, bits.OnesCount64(attackedSquares&kingRing[0]), SafetyKnightWeightMG, SafetyKnightWeightEG)
	}

	knightOutpostMG := KnightOutpostMG*bits.OnesCount64(b.White.Knights&whiteOutposts) -
		KnightOutpostMG*bits.OnesCount64(b.Black.Knights&blackOutposts)
	knightOutpostEG := KnightOutpostEG*bits.OnesCount64(b.White.Knights&whiteOutposts) -
		KnightOutpostEG*bits.OnesCount64(b.Black.Knights&blackOutposts)

	knightTropismBonusMG, knightTropismBonusEG := knightKingTropism(b)
	knightMobilityMG = (knightMobilityMG * scales.knightMobilityMG) / 100
	knightMobilityEG = (knightMobilityEG * scales.knightMobilityEG) / 100

	knightMG = knightPsqtMG + knightOutpostMG + knightMobilityMG + knightTropismBonusMG
	knightEG = knightPsqtEG + knightOutpostEG + knightMobilityEG + knightTropismBonusEG

	return knightMG, knightEG
}

func evaluateBishops(
	b *gm.Board,
	allPieces uint64,
	wPawnAttackBB, bPawnAttackBB uint64,
	kingRing [2]uint64,
	scales centerScales,
	whiteOutposts, blackOutposts uint64,
	wBlockedPawns, bBlockedPawns uint64,
	bishopMovementBB *[2]uint64,
	kingAttackMobilityBB *[2]uint64,
	danger *kingDanger,
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
		danger.addAttacker(0, bits.OnesCount64(bishopAttacks&kingRing[1]), SafetyBishopWeightMG, SafetyBishopWeightEG)
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
		danger.addAttacker(1, bits.OnesCount64(bishopAttacks&kingRing[0]), SafetyBishopWeightMG, SafetyBishopWeightEG)
	}

	bishopOutpostMG := BishopOutpostMG*bits.OnesCount64(b.White.Bishops&whiteOutposts) -
		BishopOutpostMG*bits.OnesCount64(b.Black.Bishops&blackOutposts)
	bishopOutpostEG := BishopOutpostEG*bits.OnesCount64(b.White.Bishops&whiteOutposts) -
		BishopOutpostEG*bits.OnesCount64(b.Black.Bishops&blackOutposts)

	bishopPairMG, bishopPairEG := bishopPairBonuses(b)
	bishopPairMG = (bishopPairMG * scales.bishopPairMG) / 100
	bishopPairEG = (bishopPairEG * scales.bishopPairEG) / 100

	bishopMobilityMG = (bishopMobilityMG * scales.bishopMobilityMG) / 100
	bishopMobilityEG = (bishopMobilityEG * scales.bishopMobilityEG) / 100

	bishopMG = bishopPsqtMG + bishopOutpostMG + bishopPairMG + bishopMobilityMG + bishopBadMG
	bishopEG = bishopPsqtEG + bishopOutpostEG + bishopPairEG + bishopMobilityEG + bishopBadEG

	return bishopMG, bishopEG
}

func evaluateRooks(
	b *gm.Board,
	allPieces uint64,
	wPawnAttackBB, bPawnAttackBB uint64,
	kingRing [2]uint64,
	openFiles, wSemiOpenFiles, bSemiOpenFiles uint64,
	wRookStackFiles, bRookStackFiles uint64,
	rookMovementBB *[2]uint64,
	kingAttackMobilityBB *[2]uint64,
	danger *kingDanger,
	debug bool,
) (rookMG, rookEG int) {

	rookPsqtMG, rookPsqtEG := countPieceTables(&b.White.Rooks, &b.Black.Rooks,
		&PSQT_MG[gm.PieceTypeRook], &PSQT_EG[gm.PieceTypeRook])

	var rookMobilityMG, rookMobilityEG int

	for x := b.White.Rooks; x != 0; x &= x - 1 {
		square := bits.TrailingZeros64(x)
		occupied := rookAttackOccupancy(b, true)
		rookAttacks := gm.CalculateRookMoveBitboard(uint8(square), occupied)
		(*kingAttackMobilityBB)[0] |= rookAttacks &^ b.White.All
		(*rookMovementBB)[0] |= rookAttacks
		mobilitySquares := rookAttacks &^ bPawnAttackBB &^ b.White.All
		popCnt := bits.OnesCount64(mobilitySquares)
		idx := mobilityIndex(popCnt, len(RookMobilityMG)-1)
		rookMobilityMG += RookMobilityMG[idx]
		rookMobilityEG += RookMobilityEG[idx]
		danger.addAttacker(0, bits.OnesCount64(rookAttacks&kingRing[1]), SafetyRookWeightMG, SafetyRookWeightEG)
	}
	for x := b.Black.Rooks; x != 0; x &= x - 1 {
		square := bits.TrailingZeros64(x)
		occupied := rookAttackOccupancy(b, false)
		rookAttacks := gm.CalculateRookMoveBitboard(uint8(square), occupied)
		(*kingAttackMobilityBB)[1] |= rookAttacks &^ b.Black.All
		(*rookMovementBB)[1] |= rookAttacks
		mobilitySquares := rookAttacks &^ wPawnAttackBB &^ b.Black.All
		popCnt := bits.OnesCount64(mobilitySquares)
		idx := mobilityIndex(popCnt, len(RookMobilityMG)-1)
		rookMobilityMG -= RookMobilityMG[idx]
		rookMobilityEG -= RookMobilityEG[idx]
		danger.addAttacker(1, bits.OnesCount64(rookAttacks&kingRing[0]), SafetyRookWeightMG, SafetyRookWeightEG)
	}

	rookSemiOpenMG, rookSemiOpenEG, rookOpenMG, rookOpenEG := rookFilesBonus(b, openFiles, wSemiOpenFiles, bSemiOpenFiles)
	rookFileCountMG, rookFileCountEG := rookFileCountBonus(b, openFiles, wSemiOpenFiles, bSemiOpenFiles)
	rookStackedMG := rookStackBonusMG(wRookStackFiles, bRookStackFiles)

	rookSeventhBonusMG, rookSeventhBonusEG := rookSeventhRankBonus(b)

	rookMG = rookPsqtMG + rookMobilityMG + rookOpenMG + rookSemiOpenMG + rookFileCountMG + rookStackedMG + rookSeventhBonusMG
	rookEG = rookPsqtEG + rookMobilityEG + rookOpenEG + rookSemiOpenEG + rookFileCountEG + rookSeventhBonusEG

	return rookMG, rookEG
}

func evaluateQueens(
	b *gm.Board,
	allPieces uint64,
	wPawnAttackBB, bPawnAttackBB uint64,
	kingRing [2]uint64,
	queenMovementBB *[2]uint64,
	kingAttackMobilityBB *[2]uint64,
	danger *kingDanger,
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
		danger.addAttacker(0, bits.OnesCount64(attackedSquares&kingRing[1]), SafetyQueenWeightMG, SafetyQueenWeightEG)
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
		danger.addAttacker(1, bits.OnesCount64(attackedSquares&kingRing[0]), SafetyQueenWeightMG, SafetyQueenWeightEG)
	}

	centralizedQueenBonus := centralizedQueen(b)

	queenMG = queenPsqtMG + queenMobilityMG
	queenEG = queenPsqtEG + queenMobilityEG + centralizedQueenBonus

	return queenMG, queenEG
}

/* ============= MAIN EVALUATION ============= */
func Evaluation(b *gm.Board, debug bool) (score int32) {
	if debug {
		score, trace := EvaluateWithTrace(b)
		_ = RenderEvalTraceText(os.Stdout, trace)
		return score
	}

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

	// Occupancy-dependent, so computed per eval rather than pawn-hash cached.
	candidateMG, candidateEG, _, _ := CandidatePassedTerm(b, pawnEntry)
	pawnMG += candidateMG
	pawnEG += candidateEG

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
	scales := getCenterMobilityScales(lockedCenter, openIdx)

	// Movement bitboards for king-safety and weak-squares
	var knightMovementBB [2]uint64
	var bishopMovementBB [2]uint64
	var rookMovementBB [2]uint64
	var queenMovementBB [2]uint64
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
	var danger kingDanger
	kingRing := getKingSafetyTable(b, true, 0, 0)

	wPieceCount := bits.OnesCount64(b.White.Bishops | b.White.Knights | b.White.Rooks | b.White.Queens)
	bPieceCount := bits.OnesCount64(b.Black.Bishops | b.Black.Knights | b.Black.Rooks | b.Black.Queens)
	wPawnCount := bits.OnesCount64(b.White.Pawns)
	bPawnCount := bits.OnesCount64(b.Black.Pawns)

	var kingPsqtMG, kingPsqtEG int

	allPieces := b.White.All | b.Black.All

	// KNIGHTS
	knightMG, knightEG = evaluateKnights(
		b,
		wPawnAttackBB, bPawnAttackBB,
		kingRing,
		scales,
		whiteOutposts, blackOutposts,
		&knightMovementBB,
		&kingAttackMobilityBB,
		&danger,
		debug,
	)

	// BISHOPS
	bishopMG, bishopEG = evaluateBishops(
		b,
		allPieces,
		wPawnAttackBB, bPawnAttackBB,
		kingRing,
		scales,
		whiteOutposts, blackOutposts,
		pawnEntry.WBlockedBB, pawnEntry.BBlockedBB,
		&bishopMovementBB,
		&kingAttackMobilityBB,
		&danger,
		debug,
	)

	// ROOKS
	rookMG, rookEG = evaluateRooks(
		b,
		allPieces,
		wPawnAttackBB, bPawnAttackBB,
		kingRing,
		openFiles, wSemiOpenFiles, bSemiOpenFiles,
		wRookStackFiles, bRookStackFiles,
		&rookMovementBB,
		&kingAttackMobilityBB,
		&danger,
		debug,
	)

	// QUEENS
	queenMG, queenEG = evaluateQueens(
		b,
		allPieces,
		wPawnAttackBB, bPawnAttackBB,
		kingRing,
		&queenMovementBB,
		&kingAttackMobilityBB,
		&danger,
		debug,
	)

	// Checking squares need every side's attack sets, so this runs after all
	// four piece loops rather than inside them.
	kingCheckThreats(b, &danger, allPieces, wPawnAttackBB, bPawnAttackBB,
		knightMovementBB, bishopMovementBB, queenMovementBB)

	kingPsqtMG, kingPsqtEG = countPieceTables(&b.White.Kings, &b.Black.Kings, &PSQT_MG[gm.PieceTypeKing], &PSQT_EG[gm.PieceTypeKing])

	kingAttackPenaltyMG, kingAttackPenaltyEG := kingDangerScore(&danger, b)
	kingShelterMG, kingStormMG, kingStormEG := kingShelterStorm(b, wPawnAttackBB, bPawnAttackBB)
	KingMinorPieceDefenseBonusMG := kingMinorPieceDefences(kingRing, knightMovementBB, bishopMovementBB)
	kingPasserProximityEG := kingPasserProximity(b, pawnEntry)

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

	kingMG = kingPsqtMG + kingAttackPenaltyMG + kingShelterMG + kingStormMG + KingMinorPieceDefenseBonusMG
	kingEG = kingPsqtEG + kingAttackPenaltyEG + kingStormEG + kingCentralManhattanPenalty + kingMopUpBonus + kingPasserProximityEG

	spaceMG := spaceEvaluation(b, pawnEntry)

	// FINAL SCORE CALCULATION (unchanged)
	materialScoreMG := wMaterialMG - bMaterialMG
	materialScoreEG := wMaterialEG - bMaterialEG

	toMoveBonus := TempoBonus
	if !b.Wtomove {
		toMoveBonus = -TempoBonus
	}

	imbalanceMG, imbalanceEG := materialImbalance(b)

	// PHASES
	mgWeight := piecePhase
	egWeight := TotalPhase - piecePhase

	variableScoreMG := pawnMG + knightMG + bishopMG + rookMG + queenMG + kingMG + toMoveBonus + imbalanceMG + spaceMG
	variableScoreEG := pawnEG + knightEG + bishopEG + rookEG + queenEG + kingEG + toMoveBonus + imbalanceEG

	mgScore := materialScoreMG + variableScoreMG
	egScore := materialScoreEG + variableScoreEG

	score = int32((mgScore*mgWeight + egScore*egWeight) / TotalPhase)

	if isTheoreticalDraw(b, debug) {
		score = score / DrawDivider
	}

	if !b.Wtomove {
		score = -score
	}

	return score
}
