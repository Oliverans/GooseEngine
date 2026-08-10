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
// the two adjacent files, on every rank ahead. "Can this knight be kicked?"
// depends only on the square, never on the position, so getOutpostsBB used to
// rebuild a constant per candidate per node. Filled by InitPassedPawnMasks;
// 1 KB for both colours, which stays resident in L1.
var outpostBlockersWhite [64]uint64
var outpostBlockersBlack [64]uint64

// Outpost and rank masks
var wPhalanxOrConnectedEndgameInvalidSquares uint64 = 0x000000000000ffff // ranks 1-2
var bPhalanxOrConnectedEndgameInvalidSquares uint64 = 0xffff000000000000 // ranks 7-8
// Squares where knights/bishops can be outposts: ranks 4-7 for white, ranks 2-5
// for black (exact mirrors), files b-g. Rank 8 is excluded because a supported
// minor on the back rank is not an outpost; rank 3 and below are excluded because
// a square in our own half is not one either. The previous masks covered ranks
// 1-5 / 4-8, which paid the outpost bonus to ordinary developing squares like a
// knight on c3 while ignoring genuine outposts on the 6th and 7th.
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
	// Backward pawns split by whether an enemy pawn stands ahead on the same
	// file. Stockfish charges Backward S(9,24) plus WeakUnopposed S(13,27) when
	// unopposed, making an unopposed backward pawn roughly 2.4x worse in the
	// middlegame and 2.1x worse in the endgame. The reason is the file rather
	// than the pawn: with nothing ahead of it, it can never be traded off by
	// advancing past an opposing pawn, and the half-open file in front of it is
	// the lane enemy rooks come down.
	//
	// Seeded to reproduce the old flat 6/11 when averaged over the measured
	// 66.7% opposed / 33.3% unopposed split in Goose's own games, so this
	// redistributes weight rather than adding any.
	BackwardOpposedMG   = 4
	BackwardOpposedEG   = 8
	BackwardUnopposedMG = 10
	BackwardUnopposedEG = 17

	// Isolated pawns split by the same `opposed` test as backward and doubled.
	// Every engine keeps isolated flat across ranks -- Stockfish S(5,15),
	// Ethereal and Weiss likewise -- so rank is not the dimension to add here;
	// opposed is. Stockfish charges Isolated plus WeakUnopposed S(13,27) when
	// nothing faces it, making an unopposed isolated pawn ~3.6x worse in the
	// middlegame and ~2.8x in the endgame. The reason is the file rather than
	// the pawn: with no enemy pawn ahead it can never be traded off by
	// advancing, and the half-open file in front of it is where their rooks go.
	//
	// This is the steepest of the three opposed splits, and isolated pawns are
	// the most evenly divided population: 54.0% opposed to 46.0% unopposed.
	// Seeded to hold the old flat 9/14 across that split, then damped once
	// against the corpus: a split stops the two sides' isolated pawns cancelling
	// when they fall in different classes, so the ratio-implied seeds came in 8%
	// heavy on realised mass. These land within 2% of the retired term with its
	// p90 and p99 unchanged.
	IsolatedOpposedMG   = 4
	IsolatedOpposedEG   = 8
	IsolatedUnopposedMG = 14
	IsolatedUnopposedEG = 20

	// Doubled pawns split by the same test. An unopposed stack sits on a
	// half-open file, so its front pawn can still advance and a rook behind it
	// has a clear line -- compensation an opposed stack does not get.
	//
	// Unlike the backward split this has no external reference. Stockfish does
	// not split doubled by opposed at all, and Ethereal does but its values are
	// not something to reproduce from memory. The 1.3x ratio is a guess and the
	// direction is what a match tests; this is the least-founded number in the
	// pawn group. Seeded mass-neutral against the measured 63.1% / 36.9% split.
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

	// Centre-openness scaling, as a percentage swing applied at a fully open or
	// fully closed centre and interpolated linearly between. Knights are scaled
	// the other way from bishops and the pair.
	//
	// These replace four hard buckets that left 45.7% of positions with no centre
	// knowledge at all -- the old thresholds at openIdx 0.25 and 0.75 straddled a
	// dead band holding the three most common index values. Coverage rises to
	// 82.9%; only an exactly balanced centre now scales nothing.
	//
	// The MG values are fitted to hold the old mechanism's mass (0.802 total).
	// The EG values are deliberately about a third of MG in PERCENTAGE terms
	// because endgame mobility is roughly three times the middlegame figure
	// (knight 16.8 vs 5.5, bishop 28.1 vs 9.3 mean absolute): matching MG's
	// percentages would have made the endgame half dominate a mechanism that was
	// middlegame-only before. At a third they contribute about as much as MG does,
	// which is what stops the centre read evaporating the moment queens come off.
	CenterKnightMobilityMG = 12
	CenterKnightMobilityEG = 4
	CenterBishopMobilityMG = 10
	CenterBishopMobilityEG = 3
	CenterBishopPairMG     = 7
	CenterBishopPairEG     = 2

	RookStackedMG = 20

	// The file bonuses used to be middlegame-only and the seventh-rank bonus
	// endgame-only. An open file keeps its value into the endgame (Stockfish
	// S(49,26), Weiss S(28,31)) and rooks on the seventh already matter before
	// the endgame proper. The added halves are deliberate placeholders at
	// slightly under half the existing phase, to be tuned rather than trusted.
	RookSeventhRankMG = 7
	RookSeventhRankEG = 15
	RookSemiOpenMG    = 15
	RookSemiOpenEG    = 7
	RookOpenMG        = 25
	RookOpenEG        = 12

	// Rook file-COUNT bonus, paid per rook per usable file whether or not that
	// rook stands on one. RookOpen/RookSemiOpen above answer "is this rook well
	// placed now", which the search resolves for itself within a ply or two.
	// These answer the different question the search cannot reach: given that we
	// own rooks, is this a structure worth having? They are what makes the engine
	// want to trade and push toward files its rooks can eventually use.
	//
	// Berserk's HCE carries the same idea as a global open-file count. Measured
	// against the corpus the two questions really are distinct -- correlation
	// with the per-rook terms above is only r = +0.32 (semi) and +0.36 (open).
	//
	// Open files are shared by both sides, so that half cancels unless the rook
	// COUNTS differ, and it is silent in 89.2% of positions. The semi-open half
	// is per-side and fires in 43.3%; it is the half that supplies the nudge.
	// Ratio 5:3 mirrors the 25:15 above. Deliberately small: together these carry
	// 39% of the mass of the per-rook terms.
	RookFileCountOpenMG = 5
	RookFileCountOpenEG = 3
	RookFileCountSemiMG = 3
	RookFileCountSemiEG = 2

	QueenCentralizationEG = 9

	// King danger, accumulated per attacking side and then made non-linear once.
	// A piece contributes its weight only if it bears on the enemy king ring at
	// all, so the summed weight grows with the NUMBER of attacking pieces and
	// squaring it below makes danger quadratic in that count. That is the
	// property the old flat attack-unit count lacked: one piece sweeping many
	// ring squares used to read the same as a genuine multi-piece attack.
	//
	// Values follow Ethereal's Safety* constants, whose middlegame piece values
	// are close to Goose's, with four deliberate departures measured against
	// Goose's own games: the divisor, the queenless floor, the dead-band offset
	// and the middlegame check weights. Each is explained where it is set. They
	// are otherwise untuned here and want a tuning pass.
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

	// Ethereal's value is -237/-259, large enough to zero the accumulator on its
	// own. Measured against Goose's own games that floored 86.2% of queenless
	// sides at exactly zero, against 6.5% under the tuned attack-unit table it
	// replaced, so the engine simply stopped seeing queenless attacks. This is
	// the level that keeps "trade queens and the attack mostly dies" without
	// making it an absolute.
	SafetyNoEnemyQueensMG = -60
	SafetyNoEnemyQueensEG = -65

	// A check is "safe" when the checking square is not covered by an enemy
	// pawn, knight, bishop or rook. King and queen cover does not make a square
	// unsafe: neither can recapture there without inheriting the same problem,
	// which is exactly how Stockfish and Ethereal define it.
	//
	// The middlegame weights are Ethereal's scaled to 70%. At full weight checks
	// supplied 62% of the raw accumulator across the top 1% of king-danger
	// positions, against 328 points of actual ring pressure — so the term was
	// mostly counting checks, and the engine read "attack" wherever a few safe
	// checks existed. That is the worst thing for static evaluation to bet on:
	// checks are generated first, extended, and never pruned, so search already
	// prices them accurately, and the majority of the attacks it was valuing at
	// +3 fell flat. At 70% checks are 48% of raw in that same top 1%, roughly at
	// parity with positional pressure rather than dominating it.
	//
	// The endgame weights are deliberately left at full Ethereal. That output is
	// linear, so three safe checks there add three times one check rather than
	// nine times; the compounding that produced the middlegame spikes cannot
	// happen, and the queenless floor above already covers the phase where
	// checks stop meaning much.
	SafetySafeKnightCheckMG = 78
	SafetySafeKnightCheckEG = 117
	SafetySafeBishopCheckMG = 41
	SafetySafeBishopCheckEG = 59
	SafetySafeRookCheckMG   = 63
	SafetySafeRookCheckEG   = 98
	SafetySafeQueenCheckMG  = 65
	SafetySafeQueenCheckEG  = 83

	// Checks that exist but land on a defended square. Ethereal does not score
	// these at all and Weiss has no separate category; Stockfish weights them at
	// roughly a fifth of a safe check, which is what these approximate. Unsafe
	// squares are counted once across all piece types, not once per type.
	SafetyUnsafeCheckMG = 11
	SafetyUnsafeCheckEG = 15

	// Ethereal runs a standing -74/-26 offset which, with the clamp at zero in
	// kingDangerRaw, acts as a dead band. On Goose's nine-square ring that band
	// swallowed the term: 48.0% of queen-present sides scored nothing, because
	// clearing the offset took 101 accumulator points before the first centipawn
	// appeared. Removed rather than reduced -- the ring is small enough that the
	// squaring alone already suppresses token attacks.
	SafetyAdjustmentMG = 0
	SafetyAdjustmentEG = 0

	// The middlegame half is squared and the endgame half stays linear, so king
	// danger ramps sharply while queens are on and decays gently once they are
	// not. Dividing rather than multiplying keeps the accumulator in integers.
	//
	// 2200 rather than Ethereal's 720 because Goose feeds a nine-square ring
	// where Ethereal feeds a larger king area, so the same divisor produced a
	// distribution four times too fat in the tail. Fitted so the percentiles up
	// to p99 land on those of the tuned attack-unit table this replaced.
	SafetyMGDivisor = 2200
	SafetyEGDivisor = 20

	KingMinorDefenseBonusMG = 3
	KingPasserProximityEG   = 1
	KingPasserProximityDiv  = 10
	KingPasserEnemyWeight   = 5
	KingPasserOwnWeight     = 2

	// Space is manoeuvring room in our OWN half: safe squares on the centre
	// files behind our own front line. Each safe square pays SpaceSafe, and
	// three adjustments say how durable it is.
	//
	// SpaceBehindPawn is the dynamic half. Squares are "behind" a pawn of ours
	// after filling downward from it, so advancing a pawn pays for the ground it
	// leaves behind -- and a doubled pawn standing in that ground takes the
	// payment straight back. On one file: a pawn on e4 alone scores two safe
	// squares both behind it; e2 alone scores two safe squares behind nothing;
	// e2 and e4 together score one of each. Doubling forfeits exactly the gain
	// from having advanced.
	//
	// SpaceSemiOpen and SpaceOpen discount squares we cannot hold structurally.
	// Stockfish instead requires the square to be attacked by no enemy piece,
	// but that is a fact search re-derives every node, and measured here it only
	// separates 16.7% of the behind-pawn population. File state is durable and
	// separates 56.0% of all safe squares, so it is both the stronger signal and
	// the one static evaluation is better placed to carry.
	SpaceSafeMG       = 3
	SpaceBehindPawnMG = 3
	SpaceSemiOpenMG   = -1
	SpaceOpenMG       = -2

	// Space is worth nothing without pieces to put in it, so the summed bonus is
	// scaled by a weight that grows with material and with how blocked the
	// position is, then squared -- Stockfish's shape. The divisor is fitted
	// against Goose's own games rather than copied.
	SpaceWeightOffset = 3
	SpaceBlockedCap   = 9
	SpaceWeightDiv    = 268

	// WeakKingSquarePenaltyMG is retired. It charged for king-ring squares no
	// friendly pawn covered, with no requirement that anything was attacking
	// them -- 30.5% of the squares it counted had no enemy piece on the ring at
	// all. What survived that condition was already carried twice over: the
	// danger accumulator counts attacked ring squares WITH multiplicity, which
	// is the stronger signal, and KingShelterMG now prices missing pawns per
	// file around the king. Measured, the only information left was the
	// pawn-defence filter, and it was inert in 63.6% of attacked king-sides.
	//
	// Stockfish's equivalent is worth 183 per square, but its predicate is
	// stronger -- attacked by them, not doubly defended by us, and defended by
	// nothing better than our king or queen. That needs a double-attack map
	// Goose does not build, and building one is a king-safety project rather
	// than a repair to this term.

	// Retired from the evaluation: the storm is now scored per file alongside
	// the shelter, by KingStormUnblockedMG and KingStormBlockedMG/EG. These
	// survive only because the frozen tuner seeds and exports them. Delete with
	// their tuner call sites.
	PawnStormBaseMG             = [8]int{0, 0, 0, 5, 12, 8, 3, 0}
	PawnStormFreePct            = [8]int{0, 0, 0, 100, 100, 100, 100, 0}
	PawnStormLeverPct           = [8]int{0, 0, 0, 80, 85, 85, 90, 0}
	PawnStormWeakLeverPct       = [8]int{0, 0, 0, 50, 55, 60, 65, 0}
	PawnStormBlockedPct         = [8]int{0, 0, 0, 30, 33, 40, 45, 0}
	PawnStormOppositeMultiplier = 151

	TempoBonus        = 11
	DrawDivider int32 = 8
)

// KingSafetyTable is retired from the evaluation: king danger is now the
// squared kingDanger accumulator. The data survives only because the frozen
// tuner seeds and scales it. Delete with its tuner call sites.
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

// KingShelterMG scores the pawn cover in front of a king, one file at a time.
//
//	first index  = the file's distance from the board edge: 0 = a/h, 1 = b/g,
//	               2 = c/f, 3 = d/e
//	second index = the sheltering pawn's rank relative to its own side, so 1 is
//	               that side's second rank. Index 0 means "no sheltering pawn on
//	               this file at all"; index 7 is unreachable (a pawn there would
//	               have promoted) and exists only so an illegal position cannot
//	               index out of range.
//
// This replaces two terms that between them could not tell a pawn on g2 from
// one on g3: kingPawnDefense paid a flat 2 cp per pawn inside the king ring
// regardless of rank, and kingFilesPenalty charged a flat 12/20 cp for a
// semi-open/open file beside the king. Index 0 of each row now carries that
// missing-pawn penalty, and the rest of the row carries the rank information
// that did not exist before.
//
// The shape is Stockfish's ShelterStrength scaled by roughly 0.4 for Goose's
// smaller piece values. Treat the absolute level of each row as a placeholder:
// it overlaps with the king PSQT, which already prices king placement, so the
// rows want tuning rather than trust. The profile across ranks is the part the
// measurements actually motivated.
var KingShelterMG = [4][8]int{
	{-1, 34, 38, 21, 16, 9, 10},
	{-22, 26, 13, -22, -12, -4, -24},
	{-4, 30, 8, -2, 10, 4, -19},
	{-16, -4, -11, -23, -17, -26, -65},
}

// KingStormUnblockedMG penalises an enemy pawn bearing down on the king with no
// pawn of ours directly in its path. Indexing matches KingShelterMG: the first
// index is the file's distance from the edge, the second is the enemy pawn's
// rank measured from OUR back rank, so 2 means it has reached our third rank.
// Index 0 means the file holds no enemy pawn at all.
//
// The value is subtracted from the side being stormed, so a negative entry is a
// bonus: a distant enemy pawn on a file we have covered is one fewer pawn
// defending its own king.
//
// The shape is Stockfish's UnblockedStorm at ~0.32: the 0.4 used for
// KingShelterMG, then 0.8 again after the first version made the engine
// trigger-happy about pushing pawns at a king. There is one further deliberate
// departure from the shape. Stockfish's ranks 1 and 2 are
// negative on most rows and hugely negative on the a/h row (-289 and -166
// unscaled), which pays the defender a bonus for an enemy pawn sitting on its
// own second or third rank. The reading is that such a pawn is usually
// unsupported and simply lost — but that is a tactical fact search resolves in a
// ply or two, and Goose does not model whether the storm pawn is defended. Left
// as-is those two columns invert the sign of the whole term on the commonest
// attacking motif the engine sees: a black pawn on h3 beside a white king on g1
// scored +100 for white, the side under attack.
//
// So ranks 1 and 2 are both set to that row's rank-2 magnitude. This claims only
// that a storm pawn that deep is at least as bad as one on our third rank and is
// never a bonus; it deliberately does not claim which of the two is worse,
// because the positions are rare enough that Goose has no evidence either way.
// Ranks 3 to 6 are Stockfish's, scaled, and are well behaved.
var KingStormUnblockedMG = [4][8]int{
	{27, 53, 53, 31, 16, 14, 16},
	{14, 39, 39, 14, 12, -3, 6},
	{-2, 54, 54, 11, -1, -7, -5},
	{-5, 32, 32, 2, 3, -5, -10},
}

// KingStormBlockedMG and KingStormBlockedEG replace the unblocked table when one
// of our own pawns stands directly in front of the enemy storm pawn. There is no
// file index here: once the pair is locked, how near the edge the file sits stops
// mattering and only the depth of the wedge does.
//
// Only rank 2 is a real penalty. An enemy pawn on our third rank with our pawn
// on our second is the wedge that takes squares away from the king permanently.
// Deeper on the board a blocked storm pawn is mildly good for us: it is frozen,
// it cannot open a file, and it fixes the enemy structure.
//
// This is the only storm term with an endgame component, and at rank 2 it is
// almost as large as the middlegame one. That is deliberate — see the note on
// kingShelterStormFor. Both rows carry the same 0.8 rescale as the unblocked
// table, so the two stay in proportion.
var KingStormBlockedMG = [8]int{0, 0, 24, -3, -2, -2, 0, 0}
var KingStormBlockedEG = [8]int{0, 0, 25, 5, 3, 2, 1, 0}

// PawnConnected scores a pawn that is defended by one of our own pawns, stands
// beside one, or both. It is indexed by the pawn's own rank relative to its back
// rank, so index 1 is that side's second rank and index 6 its seventh.
//
// This merges what were two independent flat terms, connected (11/5 per defended
// pawn) and phalanx (7/10 per side-by-side pawn), following Stockfish:
//
//	v = PawnConnected[r] * (2 + phalanx - opposed)
//	middlegame += v,  endgame += v * (r-2) / 4
//
// Two things the old pair could not express. Phalanx is a MULTIPLIER on the
// connected value rather than a separate addend, so being both is worth more
// than the sum of being either. And an enemy pawn ahead on the same file
// subtracts from that multiplier, because most of a connected duo's value is
// that it can advance as a unit and manufacture a passer -- once the file is
// contested, that advance ends in a lock or a trade and only the structure is
// left. Measured on Goose's own games, `opposed` covers 84.4% of the qualifying
// population and the multiplier lands roughly 33% at 1x, 57% at 2x, 10% at 3x.
//
// Stockfish also adds 21*popcount(support), distinguishing a pawn defended twice
// from one defended once. That is dropped here: only 4.0% of qualifying pawns
// are defended twice, so it buys almost nothing for an extra popcount.
//
// Index 1 is deliberately zero rather than Stockfish's 7. 47.5% of the
// qualifying population sits on its own second rank, all of it phalanx without
// support -- the d2+e2 starting shape. The retired term excluded those with a
// hard rank mask on the reasoning that rewarding them discourages the pawns from
// advancing; keeping the exclusion as a zeroed table entry preserves that
// judgement while leaving it a number the tuner can lift later.
//
// The rest is Stockfish's seed {7, 8, 12, 29, 48, 86} scaled to hold the old
// pair's middlegame mass on the corpus.
var PawnConnectedMG = [7]int{0, 0, 4, 5, 13, 22, 40}

// PawnBlockedMG and PawnBlockedEG penalise one of our own pawns that an enemy
// pawn stands directly in front of. Index 0 is our fifth rank, index 1 our
// sixth; those are the only two ranks scored, matching Stockfish's r >= RANK_5.
//
// This replaces a bonus. The old term paid for the same pawns over the same two
// ranks on the reasoning that an advanced pawn takes space and cramps the
// opponent. Stockfish scores the identical feature over the identical window as
// a penalty, and its argument is narrower than the space one: "blocked" here
// means head-to-head against an enemy PAWN, and pawns facing each other can
// neither capture nor pass one another. Such a pawn has stopped taking new
// space and become fixed on a square the opponent has permanently mapped, where
// it can be neither advanced nor traded. Taking space is real, but it is what
// the pawn PSQT and the space term already pay for.
//
// The crossover is Stockfish's and worth keeping: the middlegame cost shrinks as
// the pawn advances while the endgame cost grows. On the fifth rank it is a
// committed pawn giving their pieces a hook; on the sixth it is a fixed weakness
// far from our king and near theirs.
//
// Values are Stockfish's S(-13,-4) and S(-5,-13) at Goose's ~0.4 scale. They are
// among the smallest pawn constants in either engine.
var PawnBlockedMG = [2]int{-5, -2}
var PawnBlockedEG = [2]int{-2, -5}

// Kaufman's pawn-count tilt: a knight gains value while pawns stay on and loses
// it as they come off. The input is the TOTAL pawn count, not one side's own --
// how open the game is, is a property of the position, not of a colour. Reading
// each side's own count made the term re-price pawn material: with the knights
// equal and white a pawn up it still emitted +16 MG, which the pawn's own value
// already says.
//
// Because the delta is now shared by both colours the whole term collapses to
// delta * knightDiff * weight, and is exactly zero whenever the knights are
// equal. Reference 10 is Kaufman's five per side; the corpus mean total is 10.39
// and the term's mass is flat between 10 and 11, so which of the two is a tuner
// question rather than a design one.
//
// No clamp. The input is bounded by construction (0-16 pawns), the old one bound
// in under 2% of positions, and the extreme it guarded against now points the
// right way: a pawnless two-knight surplus scores -60 MG, and two knights cannot
// mate.
//
// The matching per-BISHOP tilt is gone. Kaufman adjusts the knight, the rook and
// the bishop PAIR -- never the single bishop. Mobility, BadBishop and the centre
// openness scales already answer that question from the actual structure instead
// of from a count.
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
	// Centre files over each side's own second, third and fourth ranks. The
	// previous constants sat two ranks higher, on ranks 4-6, which made the term
	// enemy-half control rather than own-half manoeuvring room; the comment
	// beside them described these squares. review/strategic/space_control.go has
	// carried the correct masks all along.
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

// spaceBonusFor sums one side's raw space bonus over its own zone. Everything it
// reads is pawn structure, which is why the result is cached in the pawn hash
// entry rather than recomputed per node.
//
// Only enemy PAWN attacks make a square unsafe. A piece bearing on it does not:
// a piece attack is contestable and search resolves it in a ply or two, whereas
// a pawn denies the square to any piece permanently, whatever else defends it.
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

// spaceEvaluation scales each side's cached bonus by how much material it has to
// use the room, and returns the difference. The weight replaces the old hard
// phase gate: it falls away on its own as pieces come off, without a step in the
// evaluation at a fixed phase.
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

// blockedPawnPenalty scores each side's pawns that an enemy pawn faces directly,
// on that side's own fifth and sixth ranks. Only the side that advanced further
// is ever counted: in a head-to-head lock its pawn sits on relative rank 5 or 6
// while the blocker sits on relative rank 2 or 3, outside the window.
func blockedPawnPenalty(wBlocked uint64, bBlocked uint64) (blockedMG int, blockedEG int) {
	for i := 0; i < 2; i++ {
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

// phalanxPawns returns the pawns of one side that have another of their own
// beside them on the same rank. The mask goes on the shifted result, so the
// wrap from the h-file to the a-file of the next rank is discarded.
func phalanxPawns(own uint64) uint64 {
	return own & (((own << 1) &^ bitboardFileA) | ((own >> 1) &^ bitboardFileH))
}

// connectedPawnBonus scores both sides' defended and side-by-side pawns and
// returns the difference, white minus black. See PawnConnectedMG for the shape.
func connectedPawnBonus(b *gm.Board, entry *PawnHashEntry) (mg, eg int) {
	wMG, wEG := connectedPawnsFor(b.White.Pawns, entry.WPawnAttackBB, entry.WOpposedBB, true)
	bMG, bEG := connectedPawnsFor(b.Black.Pawns, entry.BPawnAttackBB, entry.BOpposedBB, false)
	return wMG - bMG, wEG - bEG
}

// connectedPawnsFor walks the six ranks a pawn can occupy and can stay entirely
// in popcounts: the multiplier 2 + phalanx - opposed decomposes into a base
// count, plus the phalanx subset, minus the opposed subset. Phalanx is a subset
// of the qualifying set by construction, so the three counts nest cleanly.
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

// doubledPawnBitboards returns each side's pawns that have another pawn of the
// same colour strictly behind them on the same file -- the front pawn of every
// stack. The popcount of that set equals the old per-file "pawns on this file
// minus one" sum, so the population is identical and only its split is new.
//
// The fills exclude their origin square, so no shift is needed: a pawn is in its
// own side's fill exactly when another of its pawns lies behind it.
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

// rookFileCountBonus pays each side for owning rooks in a structure its rooks
// can use, independently of where those rooks currently stand. See the constants
// for why this is not a duplicate of rookFilesBonus.
//
// The file masks set every square on a file, so the counts divide by 8 -- the
// same idiom rookStackBonusMG uses below.
func rookFileCountBonus(b *gm.Board, openFiles, wSemiOpenFiles, bSemiOpenFiles uint64) (mg, eg int) {
	openCount := bits.OnesCount64(openFiles) / 8
	wSemiCount := bits.OnesCount64(wSemiOpenFiles) / 8
	bSemiCount := bits.OnesCount64(bSemiOpenFiles) / 8

	wRooks := bits.OnesCount64(b.White.Rooks)
	bRooks := bits.OnesCount64(b.Black.Rooks)

	// Open files are common to both sides, so this difference collapses to
	// openCount * (wRooks - bRooks) and only speaks when the rook counts differ.
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

// kingShelterStorm scores both kings' pawn cover and the enemy pawns storming
// them, and returns the differences, white minus black. Shelter and storm are
// kept apart in the return so the traces can show them separately; the
// evaluation just adds them.
func kingShelterStorm(b *gm.Board, wPawnAttackBB, bPawnAttackBB uint64) (shelterMG, stormMG, stormEG int) {
	wShelter, wStormMG, wStormEG := kingShelterStormFor(b, true, bPawnAttackBB)
	bShelter, bStormMG, bStormEG := kingShelterStormFor(b, false, wPawnAttackBB)
	return wShelter - bShelter, wStormMG - bStormMG, wStormEG - bStormEG
}

// kingShelterStormFor walks the king's own file and its two neighbours once,
// scoring our cover and their storm on each. The centre file is clamped to b..g,
// so a king on a1 is scored over a/b/c: it shelters behind the b-file pawn too,
// and without the clamp an edge king would only ever be scored over two files.
//
// Both terms are written from the defender's point of view: shelter is a bonus
// to this side, storm is subtracted from it. The offensive half of the storm
// falls out for free, because the caller returns white minus black — a black
// pawn wave against the white king is the same number as a white bonus.
//
// This replaces the old evaluatePawnStorm, which only ran when the kings had
// castled to opposite wings. That gate zeroed the term in 62.3% of real
// positions, including every same-side minority attack and every h-pawn push
// against an uncastled king. Sharing the file walk with the shelter also removes
// the base × percentage encoding, which could not express "advanced but harmless"
// separately from "close but locked".
//
// Two pawns of ours are excluded from counting as cover, both following
// Stockfish: pawns behind the king cannot shelter it, and a pawn that an enemy
// pawn attacks is already under challenge. A pawn excluded that way also stops
// counting as a blocker, which is the intent — it cannot hold the wedge either.
// Their pawns get only the behind-the-king filter.
//
// Storm is middlegame-only except when blocked. An unblocked storm pawn is a
// threat of tempo: it is coming, and what makes it dangerous is that pieces
// arrive behind it, so once the queens are gone it is just a pawn. A blocked one
// is a structural fact instead — the pair is frozen, the squares it takes away
// stay taken, and the endgame cares about that at nearly full strength.
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

// frontmostRelRank returns the rank, measured from the given side's own back
// rank, of the pawn in bb standing closest to that back rank. For our own pawns
// that picks the one actually doing the sheltering — a pawn further up the file
// has already left the king behind. For enemy pawns the same rule picks the most
// advanced one, which is the one storming us.
//
// Zero means bb is empty. No pawn can legally stand on its own side's back rank,
// nor on the far side's, so zero is never ambiguous for either colour.
func frontmostRelRank(bb uint64, white bool) int {
	if bb == 0 {
		return 0
	}
	if white {
		return bits.TrailingZeros64(bb) / 8
	}
	return 7 - (63-bits.LeadingZeros64(bb))/8
}

// safetyWeight returns the king-danger weight of one attacking piece type. Only
// the four piece types that can build an attack have one: pawns are not counted
// as king attackers, matching Stockfish, Ethereal and Weiss alike.
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

// kingDanger accumulates the raw danger each side generates against the enemy
// king. Index 0 is what WHITE generates against the black king, matching the
// convention the old attackUnitCounts array used.
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

// addAttacker folds one attacking piece into the accumulator. A piece that does
// not touch the ring contributes nothing, so the summed weight counts pieces
// rather than squares.
func (k *kingDanger) addAttacker(side int, ringHits, weightMG, weightEG int) {
	if ringHits == 0 {
		return
	}
	k.attackers[side]++
	k.weightMG[side] += weightMG
	k.weightEG[side] += weightEG
	k.squares[side] += ringHits
}

// kingDangerFor turns one side's accumulated danger into centipawns. Ethereal
// scales its square count by 9/|kingArea| because its area varies in size;
// Goose's ring is always exactly nine squares, so the scaling is the identity
// and is omitted.
func kingDangerRaw(k *kingDanger, side int, attackerHasQueen bool) (mg, eg int) {
	mg = k.weightMG[side] + SafetyAttackValueMG*k.squares[side] + k.checkMG[side] + SafetyAdjustmentMG
	eg = k.weightEG[side] + SafetyAttackValueEG*k.squares[side] + k.checkEG[side] + SafetyAdjustmentEG
	if !attackerHasQueen {
		mg += SafetyNoEnemyQueensMG
		eg += SafetyNoEnemyQueensEG
	}
	// Clamping here is what turns SafetyAdjustment into a dead band: anything
	// that does not clear the standing offset scores exactly nothing.
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
	return (rawMG * rawMG) / SafetyMGDivisor, rawEG / SafetyEGDivisor
}

// rookAttackOccupancy is the occupancy evaluateRooks uses for pressure: friendly
// rooks and both queens are transparent, so a rook behind them still bears on
// the same line. Anything reporting king-attack accounting must use this and
// not true occupancy, or the position trace and the evaluation disagree.
func rookAttackOccupancy(b *gm.Board, white bool) uint64 {
	ownRooks := b.White.Rooks
	if !white {
		ownRooks = b.Black.Rooks
	}
	return (b.White.All | b.Black.All) &^ (b.White.Queens | b.Black.Queens) &^ ownRooks
}

// kingCheckThreats folds checking squares into the danger accumulator.
//
// The squares from which a piece type checks a king are exactly that type's
// moves FROM the king square, because attack relations are symmetric. So four
// lookups per king serve every attacker, and checks are found at any distance
// rather than only inside the ring -- 57% of real checking squares lie outside
// it, so a ring-anchored test would miss most of them.
//
// knightAttacks/bishopAttacks/queenAttacks are per-side unions of true attack
// sets. Rooks are recomputed here against true occupancy on purpose: the x-ray
// set above is right for pressure but would invent checks from squares a rook
// cannot actually reach through a queen.
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

// kingDangerScore returns the danger difference from white's point of view:
// danger charged to the black king minus danger charged to the white king. The
// non-linearity is applied per king before subtracting, so two mirrored attacks
// still cancel but a lopsided one does not.
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

	// Friendly rooks and both queens are transparent to a rook: a rook behind a
	// friendly rook, or looking through a queen, still bears on the same file or
	// rank. Stockfish computes rook attacks over the same reduced occupancy.
	// This costs nothing — it replaces the occupancy the lookup already used.
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
	// The nine squares around each king, unfiltered. The zero arguments say "no
	// own-pawn filter": squares a friendly pawn covers are still part of the
	// ring. The second, outer ring is gone -- no other engine has one, and it
	// let a king that merely sat near enemy pieces read as under attack.
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

	// KING (uses the danger accumulator and kingAttackMobilityBB filled by helpers)
	kingPsqtMG, kingPsqtEG = countPieceTables(&b.White.Kings, &b.Black.Kings, &PSQT_MG[gm.PieceTypeKing], &PSQT_EG[gm.PieceTypeKing])

	kingAttackPenaltyMG, kingAttackPenaltyEG := kingDangerScore(&danger, b)
	kingShelterMG, kingStormMG, kingStormEG := kingShelterStorm(b, wPawnAttackBB, bPawnAttackBB)
	KingMinorPieceDefenseBonusMG := kingMinorPieceDefences(kingRing, knightMovementBB, bishopMovementBB)
	kingPasserProximityEG := kingPasserProximity(b, pawnEntry)

	// A king-mobility term once lived here. It was never read, and the
	// measurement says leave it that way: 3.65 safe king moves per side on
	// average, and only 0.3% of king-sides have none, so the feature would be
	// near-flat across real positions.

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
