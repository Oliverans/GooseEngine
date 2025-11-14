package engine

import gm "chess-engine/goosemg"

func init() {
	weakSquaresPenaltyMG = 2
	weakKingSquaresPenaltyMG = 5
	PSQT_MG = [7][64]int{
		gm.PieceTypePawn: {
			0, 0, 0, 0, 0, 0, 0, 0,
			-14, -17, -17, -11, -12, 30, 30, 6,
			-22, -29, -22, -22, -11, 0, 7, -6,
			-14, -17, -8, -10, 2, 18, 14, 1,
			-5, -1, -2, 14, 27, 45, 28, 10,
			-2, 16, 30, 35, 47, 82, 40, 6,
			77, 71, 70, 67, 52, 38, 14, 20,
			0, 0, 0, 0, 0, 0, 0, 0,
		},
		gm.PieceTypeKnight: {
			-17, -14, -26, -8, -4, 1, -13, -30,
			-21, -19, -4, 6, 2, 2, -2, -3,
			-15, 3, 4, 14, 17, 4, 3, -9,
			1, 11, 20, 22, 26, 20, 32, 11,
			4, 18, 38, 43, 28, 44, 20, 30,
			-16, 17, 41, 43, 59, 61, 32, 11,
			-8, -1, 31, 35, 33, 35, -3, 8,
			-50, -3, -9, 1, 2, -4, 0, -13,
		},
		gm.PieceTypeBishop: {
			0, -2, -15, -16, -16, -10, -9, -3,
			-1, 4, 9, -4, -2, 5, 10, 1,
			-10, 4, 2, 3, 0, 1, -3, -3,
			-13, -1, 4, 15, 17, -7, -1, -4,
			-16, 11, 6, 28, 17, 21, 8, -9,
			-8, 4, 15, 7, 22, 27, 14, -2,
			-31, -19, -12, -13, -17, -3, -21, -8,
			-20, -8, -14, -15, -9, -20, 0, -10,
		},
		gm.PieceTypeRook: {
			0, 4, 7, 15, 10, 12, 11, -1,
			-31, -9, -13, -6, -11, 1, 10, -24,
			-20, -12, -19, -8, -14, -14, 2, -12,
			-16, -17, -16, -6, -15, -15, 2, -11,
			-6, 3, 8, 20, 4, 8, 7, 3,
			-1, 32, 18, 32, 29, 23, 31, 14,
			6, 0, 14, 19, 2, 10, -3, 18,
			23, 19, 5, 7, -8, 2, 12, 22,
		},
		gm.PieceTypeQueen: {
			16, 14, 20, 27, 28, 5, -1, 2,
			11, 18, 26, 23, 25, 36, 34, 10,
			8, 20, 17, 12, 10, 11, 19, 4,
			11, 14, 8, 3, -3, -13, 1, -8,
			1, 5, -13, -26, -21, -22, -7, -11,
			-3, 3, 3, -19, -35, -26, -31, -35,
			0, -35, -1, -15, -55, -28, -23, 15,
			2, 10, 5, -1, -10, 4, 11, 15,
		},
		gm.PieceTypeKing: {
			-5, 35, 3, -54, -23, -52, 14, 21,
			-1, -12, -20, -52, -36, -38, -6, 13,
			-4, -5, 6, 8, 15, 8, 0, -12,
			0, 8, 21, 20, 24, 19, 20, -7,
			0, 8, 16, 11, 15, 16, 11, -8,
			0, 8, 13, 10, 8, 14, 9, 0,
			-2, 4, 5, 3, 3, 5, 3, -2,
			-2, 0, 1, 1, 1, 0, 0, -1,
		},
	}
	PSQT_EG = [7][64]int{
		gm.PieceTypePawn: {
			0, 0, 0, 0, 0, 0, 0, 0,
			19, 16, 20, 19, 26, 22, 7, -5,
			16, 14, 15, 15, 17, 16, 4, 3,
			22, 21, 9, 7, 5, 10, 7, 9,
			33, 28, 23, 1, 8, 12, 18, 17,
			55, 60, 55, 50, 48, 41, 53, 49,
			118, 107, 92, 77, 70, 70, 81, 96,
			0, 0, 0, 0, 0, 0, 0, 0,
		},
		gm.PieceTypeKnight: {
			-20, -40, -15, -5, -7, -15, -30, -22,
			-16, -1, -3, 6, 7, -6, -5, -16,
			-26, 7, 12, 27, 24, 7, 5, -24,
			-3, 21, 39, 44, 41, 41, 22, 1,
			1, 23, 36, 49, 54, 42, 35, 10,
			-9, 16, 30, 32, 26, 39, 20, -1,
			-13, -1, 10, 31, 29, 8, 1, -6,
			-30, -3, 10, 7, 7, 10, 0, -13,
		},
		gm.PieceTypeBishop: {
			-18, -4, -17, 0, -5, -8, -10, -11,
			-2, -15, -3, 4, 4, -8, -7, -26,
			-2, 7, 13, 18, 16, 9, -1, 0,
			5, 12, 23, 19, 16, 19, 12, -3,
			11, 20, 19, 20, 25, 20, 23, 14,
			10, 20, 20, 17, 20, 25, 23, 14,
			5, 21, 19, 20, 20, 20, 21, 6,
			8, 12, 13, 18, 16, 7, 11, 8,
		},
		gm.PieceTypeRook: {
			-2, 1, 0, -6, -10, 0, -3, -16,
			-1, -5, 1, -5, -7, -17, -9, -7,
			6, 14, 11, 5, 1, 0, 4, -2,
			20, 28, 26, 17, 14, 15, 16, 11,
			26, 26, 25, 17, 15, 12, 16, 19,
			30, 18, 26, 15, 9, 19, 11, 17,
			9, 14, 10, 11, 10, -3, 7, 0,
			25, 27, 28, 22, 24, 36, 38, 37,
		},
		gm.PieceTypeQueen: {
			-9, -10, -14, 4, -14, -8, -5, -1,
			0, -5, -13, 7, -1, -30, -15, 0,
			3, 15, 20, -7, -8, 26, 15, 4,
			14, 26, 2, 17, 15, 14, 35, 28,
			23, 38, 2, 20, 12, 11, 43, 30,
			25, 27, 24, 7, 1, 14, 15, 22,
			30, 44, 25, 22, 26, -2, 17, 20,
			12, 22, 17, 10, 4, 12, 19, 15,
		},
		gm.PieceTypeKing: {
			-39, -39, -20, -21, -42, -16, -42, -85,
			-17, -9, 2, 8, 3, 3, -17, -38,
			-14, 1, 11, 23, 18, 8, -7, -18,
			-14, 12, 26, 35, 30, 22, 6, -19,
			0, 25, 33, 35, 34, 31, 23, -5,
			3, 29, 31, 23, 20, 39, 35, 0,
			-11, 15, 14, 5, 6, 13, 22, -8,
			-15, -9, -3, 0, -3, -2, -4, -11,
		},
	}
	pieceValueMG = [7]int{gm.PieceTypeKing: 0, gm.PieceTypePawn: 76, gm.PieceTypeKnight: 331, gm.PieceTypeBishop: 355, gm.PieceTypeRook: 469, gm.PieceTypeQueen: 983}
	pieceValueEG = [7]int{gm.PieceTypeKing: 0, gm.PieceTypePawn: 95, gm.PieceTypeKnight: 299, gm.PieceTypeBishop: 306, gm.PieceTypeRook: 522, gm.PieceTypeQueen: 900}
	mobilityValueMG = [7]int{gm.PieceTypeKing: 0, gm.PieceTypePawn: 0, gm.PieceTypeKnight: 3, gm.PieceTypeBishop: 2, gm.PieceTypeRook: 2, gm.PieceTypeQueen: 1}
	mobilityValueEG = [7]int{gm.PieceTypeKing: 0, gm.PieceTypePawn: 0, gm.PieceTypeKnight: 2, gm.PieceTypeBishop: 3, gm.PieceTypeRook: 6, gm.PieceTypeQueen: 7}
	PassedPawnPSQT_MG = [64]int{
		0, 0, 0, 0, 0, 0, 0, 0,
		-13, -10, -9, -10, -7, -25, -6, 11,
		-3, -3, -15, -13, -9, -14, -12, 8,
		18, 9, -8, -4, -10, -18, -4, 6,
		38, 36, 27, 20, 10, 5, 15, 22,
		77, 64, 52, 43, 29, 25, 21, 24,
		67, 61, 66, 63, 50, 34, 13, 20,
		0, 0, 0, 0, 0, 0, 0, 0,
	}
	PassedPawnPSQT_EG = [64]int{
		0, 0, 0, 0, 0, 0, 0, 0,
		15, 12, 8, 7, 3, -1, 8, 15,
		11, 16, 9, 8, 5, 5, 20, 11,
		32, 35, 30, 26, 27, 29, 42, 32,
		62, 57, 43, 46, 39, 38, 51, 47,
		105, 84, 66, 44, 37, 54, 61, 81,
		67, 69, 63, 54, 54, 55, 65, 69,
		0, 0, 0, 0, 0, 0, 0, 0,
	}
	DoubledPawnPenaltyMG = 10
	DoubledPawnPenaltyEG = 20
	IsolatedPawnMG = 5
	IsolatedPawnEG = 12
	ConnectedPawnsBonusMG = 18
	ConnectedPawnsBonusEG = -2
	PhalanxPawnsBonusMG = 9
	PhalanxPawnsBonusEG = 5
	BlockedPawnBonusMG = 25
	BlockedPawnBonusEG = 15
	PawnLeverMG = -4
	PawnLeverEG = -6
	BackwardPawnMG = 4
	BackwardPawnEG = -4
	PawnStormMG = -3
	PawnProximityPenaltyMG = -15
	PawnLeverStormPenaltyMG = -9
	KnightOutpostMG = 20
	KnightOutpostEG = 15
	KnightCanAttackPieceMG = -2
	KnightCanAttackPieceEG = 1
	BishopOutpostMG = 15
	BishopPairBonusMG = 0
	BishopPairBonusEG = 63
	BishopXrayKingMG = -4
	BishopXrayRookMG = 26
	BishopXrayQueenMG = 18
	StackedRooksMG = 13
	RookXrayQueenMG = 20
	ConnectedRooksBonusMG = 15
	RookSemiOpenFileBonusMG = 13
	RookOpenFileBonusMG = 24
	SeventhRankBonusEG = 19
	CentralizedQueenBonusEG = 31
	QueenInfiltrationBonusMG = -20
	QueenInfiltrationBonusEG = 55
	KingSemiOpenFilePenalty = 5
	KingOpenFilePenalty = 0
	KingMinorPieceDefenseBonus = 2
	KingPawnDefenseMG = 3
	TempoBonus = 9
	KingSafetyTable = [100]int{
		0, 1, 1, 3, 3, 5, 7, 9, 12, 15,
		18, 22, 26, 30, 35, 39, 43, 50, 55, 62,
		67, 75, 78, 85, 88, 97, 104, 113, 120, 130,
		135, 148, 163, 173, 184, 195, 206, 218, 230, 241,
		253, 265, 276, 288, 300, 312, 323, 335, 347, 359,
		370, 382, 394, 405, 417, 429, 441, 452, 464, 476,
		487, 493, 493, 500, 500, 500, 500, 500, 500, 500,
		500, 500, 500, 500, 500, 500, 500, 500, 500, 500,
		500, 500, 500, 500, 500, 500, 500, 500, 500, 500,
		500, 500, 500, 500, 500, 500, 500, 500, 500, 500,
	}
}
