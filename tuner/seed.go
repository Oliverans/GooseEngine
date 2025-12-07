package tuner

import (
	eng "chess-engine/engine"
	gm "chess-engine/goosemg"
)

// SeedFromEngineDefaults initializes PST, material and passed-pawn parameters
// from engine/evaluation.go defaults. It leaves pst.K unchanged.
func SeedFromEngineDefaults(le *LinearEval, pst *PST) {
	if le == nil || pst == nil {
		return
	}
	// PST MG/EG from engine PSQT tables (piece-major, 64 squares).
	psqtMG := eng.DefaultPSQT_MG()
	psqtEG := eng.DefaultPSQT_EG()

	// Map tuner indices P..K (0..5) to engine piece types.
	order := [6]gm.PieceType{gm.PieceTypePawn, gm.PieceTypeKnight, gm.PieceTypeBishop, gm.PieceTypeRook, gm.PieceTypeQueen, gm.PieceTypeKing}
	for i, pt := range order {
		for sq := 0; sq < 64; sq++ {
			pst.MG[i][sq] = float64(psqtMG[pt][sq])
			pst.EG[i][sq] = float64(psqtEG[pt][sq])
		}
	}

	// Material values
	mvMG := eng.DefaultPieceValueMG()
	mvEG := eng.DefaultPieceValueEG()
	le.MatMG[P] = float64(mvMG[gm.PieceTypePawn])
	le.MatMG[N] = float64(mvMG[gm.PieceTypeKnight])
	le.MatMG[B] = float64(mvMG[gm.PieceTypeBishop])
	le.MatMG[R] = float64(mvMG[gm.PieceTypeRook])
	le.MatMG[Q] = float64(mvMG[gm.PieceTypeQueen])
	le.MatMG[K] = float64(mvMG[gm.PieceTypeKing])

	le.MatEG[P] = float64(mvEG[gm.PieceTypePawn])
	le.MatEG[N] = float64(mvEG[gm.PieceTypeKnight])
	le.MatEG[B] = float64(mvEG[gm.PieceTypeBishop])
	le.MatEG[R] = float64(mvEG[gm.PieceTypeRook])
	le.MatEG[Q] = float64(mvEG[gm.PieceTypeQueen])
	le.MatEG[K] = float64(mvEG[gm.PieceTypeKing])

	// Passed pawn square weights: copy engine PSQT per square.
	passMG := eng.DefaultPassedPawnPSQT_MG()
	passEG := eng.DefaultPassedPawnPSQT_EG()
	for sq := 0; sq < 64; sq++ {
		le.PasserMG[sq] = float64(passMG[sq])
		le.PasserEG[sq] = float64(passEG[sq])
	}

	// Phase 1 scalars
	le.BishopPairMG = float64(eng.DefaultBishopPairBonusMG())
	le.BishopPairEG = float64(eng.DefaultBishopPairBonusEG())
	le.RookSemiOpenFileMG = float64(eng.DefaultRookSemiOpenFileBonusMG())
	le.RookOpenFileMG = float64(eng.DefaultRookOpenFileBonusMG())
	le.SeventhRankEG = float64(eng.DefaultSeventhRankBonusEG())
	le.QueenCentralizedEG = float64(eng.DefaultCentralizedQueenBonusEG())

	// Phase 2 pawn structure
	le.DoubledMG = float64(eng.DefaultDoubledPawnPenaltyMG())
	le.DoubledEG = float64(eng.DefaultDoubledPawnPenaltyEG())
	le.IsolatedMG = float64(eng.DefaultIsolatedPawnMG())
	le.IsolatedEG = float64(eng.DefaultIsolatedPawnEG())
	le.ConnectedMG = float64(eng.DefaultConnectedPawnsBonusMG())
	le.ConnectedEG = float64(eng.DefaultConnectedPawnsBonusEG())
	le.PhalanxMG = float64(eng.DefaultPhalanxPawnsBonusMG())
	le.PhalanxEG = float64(eng.DefaultPhalanxPawnsBonusEG())
	le.BlockedMG = float64(eng.DefaultBlockedPawnBonusMG())
	le.BlockedEG = float64(eng.DefaultBlockedPawnBonusEG())
	le.WeakLeverMG = float64(eng.DefaultWeakLeverPenaltyMG())
	le.WeakLeverEG = float64(eng.DefaultWeakLeverPenaltyEG())
	le.BackwardMG = float64(eng.DefaultBackwardPawnMG())
	le.BackwardEG = float64(eng.DefaultBackwardPawnEG())

	// Phase 3: mobility weights
	mobMGVals := eng.DefaultMobilityValueMG()
	mobEGVals := eng.DefaultMobilityValueEG()
	for i := 0; i < 7; i++ {
		le.MobilityMG[i] = float64(mobMGVals[i])
		le.MobilityEG[i] = float64(mobEGVals[i])
	}

	// Phase 4: King safety table
	ks := eng.DefaultKingSafetyTable()
	for i := 0; i < 100; i++ {
		le.KingSafety[i] = float64(ks[i])
	}
	// Phase 4 correlates
	le.KingSemiOpenFilePenalty = float64(eng.DefaultKingSemiOpenFilePenalty())
	le.KingOpenFilePenalty = float64(eng.DefaultKingOpenFilePenalty())
	le.KingMinorPieceDefense = float64(eng.DefaultKingMinorPieceDefenseBonus())
	le.KingPawnDefenseMG = float64(eng.DefaultKingPawnDefenseMG())
	le.KingEndgameCenterEG = 1.0
	le.KingMopUpEG = 1.0

	// Phase 5: Extras
	le.KnightOutpostMG = float64(eng.DefaultKnightOutpostMG())
	le.KnightOutpostEG = float64(eng.DefaultKnightOutpostEG())
	le.BishopOutpostMG = float64(eng.DefaultBishopOutpostMG())
	le.KnightThreatsMG = float64(eng.DefaultKnightCanAttackPieceMG())
	le.KnightThreatsEG = float64(eng.DefaultKnightCanAttackPieceEG())
	le.KnightTropismMG = float64(eng.KnightTropismMG)
	le.KnightTropismEG = float64(eng.KnightTropismEG)
	le.StackedRooksMG = float64(eng.DefaultStackedRooksMG())
	// SeventhRankMG has no engine default; seed 0
	// New extras
	le.PawnStormMG = float64(eng.DefaultPawnStormMG())
	le.PawnProximityMG = float64(eng.DefaultPawnProximityPenaltyMG())
	// Center mobility tuning starts at 0 (no change from base engine behavior)

	// Phase 6: Space/weak-king + Tempo
	le.SpaceMG = float64(eng.DefaultSpaceBonusMG())
	le.SpaceEG = float64(eng.DefaultSpaceBonusEG())
	le.WeakKingSquaresMG = float64(eng.DefaultWeakKingSquarePenaltyMG())
	le.WeakKingSquaresEG = float64(eng.DefaultWeakKingSquarePenaltyEG())
	if tb := eng.DefaultTempoBonus(); tb != 0 {
		le.Tempo = float64(tb)
	} else {
		le.Tempo = 10
	}

	// Material imbalance scalars
	le.ImbalanceKnightPerPawnMG = float64(eng.DefaultImbalanceKnightPerPawnMG())
	le.ImbalanceKnightPerPawnEG = float64(eng.DefaultImbalanceKnightPerPawnEG())
	le.ImbalanceBishopPerPawnMG = float64(eng.DefaultImbalanceBishopPerPawnMG())
	le.ImbalanceBishopPerPawnEG = float64(eng.DefaultImbalanceBishopPerPawnEG())
	le.ImbalanceMinorsForMajorMG = float64(eng.DefaultImbalanceMinorsForMajorMG())
	le.ImbalanceMinorsForMajorEG = float64(eng.DefaultImbalanceMinorsForMajorEG())
	le.ImbalanceRedundantRookMG = float64(eng.DefaultImbalanceRedundantRookMG())
	le.ImbalanceRedundantRookEG = float64(eng.DefaultImbalanceRedundantRookEG())
	le.ImbalanceRookQueenOverlapMG = float64(eng.DefaultImbalanceRookQueenOverlapMG())
	le.ImbalanceRookQueenOverlapEG = float64(eng.DefaultImbalanceRookQueenOverlapEG())
	le.ImbalanceQueenManyMinorsMG = float64(eng.DefaultImbalanceQueenManyMinorsMG())
	le.ImbalanceQueenManyMinorsEG = float64(eng.DefaultImbalanceQueenManyMinorsEG())
}
