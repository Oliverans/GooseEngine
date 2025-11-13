package engine

import gm "chess-engine/goosemg"

func init() {
	weakSquaresPenaltyMG = 3
	weakKingSquaresPenaltyMG = 6
	PSQT_MG = [7][64]int{
		gm.PieceTypePawn: {
			0, 0, 0, 0, 0, 0, 0, 0,
			-12, -12, -13, -6, -8, 27, 28, -1,
			-19, -23, -16, -15, -4, -1, 7, -12,
			-12, -10, -3, -5, 9, 13, 10, -9,
			-2, 6, 5, 21, 35, 41, 25, 0,
			-2, 16, 32, 39, 55, 81, 39, 0,
			77, 79, 67, 67, 51, 56, 13, 16,
			0, 0, 0, 0, 0, 0, 0, 0,
		},
		gm.PieceTypeKnight: {
			-33, -7, -23, -3, 4, 6, -7, -27,
			-17, -16, 4, 13, 10, 12, 5, 5,
			-9, 10, 12, 21, 26, 12, 12, -2,
			7, 17, 27, 28, 35, 27, 40, 17,
			8, 25, 44, 51, 34, 54, 28, 36,
			-15, 26, 48, 51, 66, 75, 42, 17,
			-12, 0, 39, 38, 36, 39, -1, 11,
			-92, -16, -8, 0, 7, -24, 0, -33,
		},
		gm.PieceTypeBishop: {
			-5, -4, -18, -18, -19, -12, -14, -8,
			-3, 3, 9, -5, -3, 4, 10, -2,
			-11, 4, 1, 3, -1, 2, -4, -3,
			-15, 0, 3, 14, 18, -8, 0, -6,
			-17, 12, 7, 31, 17, 22, 8, -10,
			-8, 7, 20, 10, 25, 30, 18, -2,
			-30, -12, -11, -9, -9, 6, -16, -9,
			-17, -6, -20, -13, -7, -19, 1, -9,
		},
		gm.PieceTypeRook: {
			1, 5, 9, 17, 12, 14, 9, -4,
			-31, -8, -12, -6, -10, 3, 12, -28,
			-20, -11, -18, -7, -12, -12, 5, -13,
			-17, -15, -15, -6, -14, -13, 7, -13,
			-5, 5, 10, 24, 9, 12, 9, 1,
			3, 34, 23, 37, 29, 29, 36, 17,
			11, 7, 22, 26, 19, 18, 2, 21,
			29, 26, 11, 14, 6, 6, 16, 27,
		},
		gm.PieceTypeQueen: {
			7, 5, 12, 22, 18, -6, -7, -6,
			2, 11, 21, 18, 20, 30, 28, 5,
			0, 14, 10, 6, 5, 5, 15, -1,
			5, 6, 1, -5, -8, -17, -2, -12,
			-6, -2, -18, -30, -24, -23, -9, -13,
			-7, -2, -2, -22, -28, -10, -16, -12,
			-4, -43, -5, -16, -50, -11, -18, 19,
			-2, 7, 4, -3, -2, 5, 11, 15,
		},
		gm.PieceTypeKing: {
			-4, 42, 9, -51, -19, -48, 15, 23,
			2, -8, -15, -51, -34, -35, -5, 14,
			-4, -4, 5, -1, 7, 4, 2, -14,
			-2, 7, 16, 12, 10, 7, 13, -10,
			0, 6, 14, 9, 12, 12, 9, -10,
			0, 8, 12, 10, 8, 13, 8, -1,
			-2, 3, 5, 3, 3, 5, 3, -2,
			-2, 0, 1, 1, 0, 0, 0, -1,
		},
	}
	PSQT_EG = [7][64]int{
		gm.PieceTypePawn: {
			0, 0, 0, 0, 0, 0, 0, 0,
			24, 18, 22, 20, 27, 24, 7, -2,
			17, 13, 13, 14, 15, 15, 2, 4,
			23, 19, 7, 5, 2, 8, 5, 10,
			35, 26, 21, 0, 6, 12, 17, 19,
			70, 78, 67, 59, 53, 48, 63, 61,
			140, 126, 111, 94, 88, 80, 100, 119,
			0, 0, 0, 0, 0, 0, 0, 0,
		},
		gm.PieceTypeKnight: {
			-16, -47, -17, -8, -13, -18, -37, -22,
			-18, -2, -7, 2, 2, -12, -9, -22,
			-31, 4, 10, 28, 23, 4, 0, -30,
			-5, 21, 40, 46, 42, 42, 20, -1,
			0, 22, 39, 50, 56, 42, 34, 8,
			-9, 13, 32, 33, 29, 38, 18, -3,
			-11, 0, 7, 33, 30, 8, 1, -8,
			-22, -1, 10, 8, 5, 13, -3, -27,
		},
		gm.PieceTypeBishop: {
			-12, -3, -16, 0, -5, -8, -9, -8,
			0, -14, -3, 5, 5, -8, -7, -25,
			-1, 8, 14, 19, 17, 9, -1, -1,
			6, 12, 25, 22, 17, 21, 13, -1,
			11, 20, 19, 20, 26, 22, 25, 15,
			10, 19, 19, 17, 20, 27, 23, 16,
			3, 18, 19, 20, 18, 18, 20, 7,
			7, 11, 15, 17, 16, 6, 10, 7,
		},
		gm.PieceTypeRook: {
			-2, 1, 0, -7, -11, 0, 2, -11,
			-2, -6, 0, -6, -9, -18, -9, -4,
			6, 12, 9, 4, -2, -3, 3, -2,
			20, 28, 25, 17, 13, 14, 15, 12,
			27, 26, 25, 17, 14, 12, 16, 21,
			29, 18, 25, 15, 10, 18, 10, 17,
			8, 12, 8, 10, 3, -5, 7, 0,
			25, 26, 27, 21, 20, 35, 38, 36,
		},
		gm.PieceTypeQueen: {
			-8, -9, -13, -3, -9, -8, -7, -2,
			1, -5, -17, 2, -7, -31, -14, -2,
			2, 10, 20, -5, -5, 23, 10, 1,
			12, 28, 7, 26, 19, 15, 30, 23,
			19, 37, 5, 22, 16, 9, 37, 22,
			16, 22, 19, 9, -4, -1, -4, -15,
			23, 41, 21, 18, 18, -15, 12, 14,
			9, 18, 13, 8, -5, 9, 16, 12,
		},
		gm.PieceTypeKing: {
			-38, -42, -21, -22, -43, -15, -41, -85,
			-18, -10, 2, 9, 5, 5, -16, -36,
			-15, 1, 14, 28, 24, 12, -5, -16,
			-14, 13, 30, 40, 38, 28, 11, -16,
			-1, 26, 36, 39, 38, 37, 26, -3,
			2, 30, 33, 25, 23, 43, 38, 1,
			-11, 15, 15, 6, 7, 15, 23, -8,
			-17, -9, -2, 1, -2, -1, -4, -11,
		},
	}
	pieceValueMG = [7]int{gm.PieceTypeKing: 0, gm.PieceTypePawn: 79, gm.PieceTypeKnight: 337, gm.PieceTypeBishop: 364, gm.PieceTypeRook: 481, gm.PieceTypeQueen: 1004}
	pieceValueEG = [7]int{gm.PieceTypeKing: 0, gm.PieceTypePawn: 95, gm.PieceTypeKnight: 293, gm.PieceTypeBishop: 301, gm.PieceTypeRook: 520, gm.PieceTypeQueen: 916}
	mobilityValueMG = [7]int{gm.PieceTypeKing: 0, gm.PieceTypePawn: 0, gm.PieceTypeKnight: 3, gm.PieceTypeBishop: 2, gm.PieceTypeRook: 2, gm.PieceTypeQueen: 1}
	mobilityValueEG = [7]int{gm.PieceTypeKing: 0, gm.PieceTypePawn: 0, gm.PieceTypeKnight: 2, gm.PieceTypeBishop: 3, gm.PieceTypeRook: 6, gm.PieceTypeQueen: 7}
	PassedPawnPSQT_MG = [64]int{
		0, 0, 0, 0, 0, 0, 0, 0,
		-11, -10, -11, -11, -7, -21, -4, 10,
		-2, -5, -17, -17, -12, -11, -11, 8,
		19, 7, -11, -7, -12, -18, -1, 7,
		41, 34, 26, 21, 11, 9, 13, 21,
		75, 62, 52, 43, 29, 28, 20, 23,
		60, 52, 62, 57, 44, 24, 11, 17,
		0, 0, 0, 0, 0, 0, 0, 0,
	}
	PassedPawnPSQT_EG = [64]int{
		0, 0, 0, 0, 0, 0, 0, 0,
		10, 9, 5, 5, 2, -4, 8, 15,
		10, 17, 10, 9, 7, 6, 21, 11,
		33, 38, 33, 29, 29, 32, 45, 34,
		64, 61, 47, 46, 40, 39, 55, 50,
		100, 77, 60, 41, 35, 54, 57, 77,
		61, 61, 57, 48, 47, 46, 53, 56,
		0, 0, 0, 0, 0, 0, 0, 0,
	}
	DoubledPawnPenaltyMG = 13
	DoubledPawnPenaltyEG = 20
	IsolatedPawnMG = 5
	IsolatedPawnEG = 12
	ConnectedPawnsBonusMG = 17
	ConnectedPawnsBonusEG = 0
	PhalanxPawnsBonusMG = 8
	PhalanxPawnsBonusEG = 7
	BlockedPawnBonusMG = 25
	BlockedPawnBonusEG = 15
	PawnLeverMG = -3
	PawnLeverEG = -6
	BackwardPawnMG = 5
	BackwardPawnEG = -4
	PawnStormMG = -1
	PawnProximityPenaltyMG = -8
	PawnLeverStormPenaltyMG = -9
	KnightOutpostMG = 20
	KnightOutpostEG = 15
	KnightCanAttackPieceMG = -2
	KnightCanAttackPieceEG = 1
	BishopOutpostMG = 15
	BishopPairBonusMG = 0
	BishopPairBonusEG = 50
	BishopXrayKingMG = 1
	BishopXrayRookMG = 23
	BishopXrayQueenMG = 18
	StackedRooksMG = 14
	RookXrayQueenMG = 22
	ConnectedRooksBonusMG = 16
	RookSemiOpenFileBonusMG = 15
	RookOpenFileBonusMG = 25
	SeventhRankBonusEG = 19
	CentralizedQueenBonusEG = 25
	QueenInfiltrationBonusMG = -5
	QueenInfiltrationBonusEG = 25
	KingSemiOpenFilePenalty = 7
	KingOpenFilePenalty = 2
	KingMinorPieceDefenseBonus = 2
	KingPawnDefenseMG = 2
	TempoBonus = 10
	KingSafetyTable = [100]int{
		0, 1, 1, 3, 3, 5, 7, 9, 12, 15,
		18, 22, 26, 30, 35, 39, 43, 50, 55, 62,
		67, 75, 80, 85, 88, 97, 104, 113, 120, 130,
		138, 148, 167, 177, 188, 199, 210, 222, 234, 245,
		257, 269, 280, 292, 304, 316, 327, 339, 351, 363,
		374, 386, 398, 409, 421, 433, 445, 456, 468, 480,
		491, 497, 497, 500, 500, 500, 500, 500, 500, 500,
		500, 500, 500, 500, 500, 500, 500, 500, 500, 500,
		500, 500, 500, 500, 500, 500, 500, 500, 500, 500,
		500, 500, 500, 500, 500, 500, 500, 500, 500, 500,
	}
}
