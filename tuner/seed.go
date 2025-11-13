package tuner

import (
    gm "chess-engine/goosemg"
    eng "chess-engine/engine"
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
    le.QueenInfiltrationMG = float64(eng.DefaultQueenInfiltrationBonusMG())
    le.QueenInfiltrationEG = float64(eng.DefaultQueenInfiltrationBonusEG())

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
    le.BackwardMG = float64(eng.DefaultBackwardPawnMG())
    le.BackwardEG = float64(eng.DefaultBackwardPawnEG())
    le.PawnLeverMG = float64(eng.DefaultPawnLeverMG())
    le.PawnLeverEG = float64(eng.DefaultPawnLeverEG())

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

    // Phase 5: Extras
    le.KnightOutpostMG = float64(eng.DefaultKnightOutpostMG())
    le.KnightOutpostEG = float64(eng.DefaultKnightOutpostEG())
    le.BishopOutpostMG = float64(eng.DefaultBishopOutpostMG())
    le.KnightThreatsMG = float64(eng.DefaultKnightCanAttackPieceMG())
    le.KnightThreatsEG = float64(eng.DefaultKnightCanAttackPieceEG())
    le.StackedRooksMG  = float64(eng.DefaultStackedRooksMG())
    le.RookXrayQueenMG = float64(eng.DefaultRookXrayQueenMG())
    le.ConnectedRooksMG = float64(eng.DefaultConnectedRooksBonusMG())
    // SeventhRankMG has no engine default; seed 0
    // New extras
    le.BishopXrayKingMG = float64(eng.DefaultBishopXrayKingMG())
    le.BishopXrayRookMG = float64(eng.DefaultBishopXrayRookMG())
    le.BishopXrayQueenMG = float64(eng.DefaultBishopXrayQueenMG())
    le.PawnStormMG = float64(eng.DefaultPawnStormMG())
    le.PawnProximityMG = float64(eng.DefaultPawnProximityPenaltyMG())
    le.PawnLeverStormMG = float64(eng.DefaultPawnLeverStormPenaltyMG())
    // Center mobility tuning starts at 0 (no change from base engine behavior)

    // Phase 6: Weak squares + Tempo
    if ws := eng.DefaultWeakSquaresPenaltyMG(); ws != 0 { le.WeakSquaresMG = float64(ws) } else { le.WeakSquaresMG = 2 }
    if wks := eng.DefaultWeakKingSquaresPenaltyMG(); wks != 0 { le.WeakKingSquaresMG = float64(wks) } else { le.WeakKingSquaresMG = 5 }
    if tb := eng.DefaultTempoBonus(); tb != 0 { le.Tempo = float64(tb) } else { le.Tempo = 10 }
}
