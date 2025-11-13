package tuner

// The following helpers centralize θ reads/writes per phase. They use the
// consolidated Layout offsets from phase_offsets.go.

func (le *LinearEval) ensureLayout() {
    if le.layout.Total == 0 {
        le.layout = computeLayout()
    }
}

// ---- Writes (struct -> theta) ----

func (le *LinearEval) writePSTToTheta(off int) int {
    for pt := 0; pt < 6; pt++ {
        copy(le.theta[off:off+64], le.PST.MG[pt][:]); off += 64
    }
    for pt := 0; pt < 6; pt++ {
        copy(le.theta[off:off+64], le.PST.EG[pt][:]); off += 64
    }
    return off
}

func (le *LinearEval) writeMaterialToTheta(off int) int {
    for i := 0; i < 6; i++ { le.theta[off+i] = le.MatMG[i] }
    off += 6
    for i := 0; i < 6; i++ { le.theta[off+i] = le.MatEG[i] }
    off += 6
    return off
}

func (le *LinearEval) writePassersToTheta(off int) int {
    for i := 0; i < 64; i++ { le.theta[off+i] = le.PasserMG[i] }
    off += 64
    for i := 0; i < 64; i++ { le.theta[off+i] = le.PasserEG[i] }
    off += 64
    return off
}

func (le *LinearEval) writeP1ScalarsToTheta(off int) int {
    le.theta[off+0] = le.BishopPairMG
    le.theta[off+1] = le.BishopPairEG
    le.theta[off+2] = le.RookSemiOpenFileMG
    le.theta[off+3] = le.RookOpenFileMG
    le.theta[off+4] = le.SeventhRankEG
    le.theta[off+5] = le.QueenCentralizedEG
    le.theta[off+6] = le.QueenInfiltrationMG
    le.theta[off+7] = le.QueenInfiltrationEG
    return off + 8
}

func (le *LinearEval) writePawnStructToTheta(off int) int {
    le.theta[off+0] = le.DoubledMG
    le.theta[off+1] = le.DoubledEG
    le.theta[off+2] = le.IsolatedMG
    le.theta[off+3] = le.IsolatedEG
    le.theta[off+4] = le.ConnectedMG
    le.theta[off+5] = le.ConnectedEG
    le.theta[off+6] = le.PhalanxMG
    le.theta[off+7] = le.PhalanxEG
    le.theta[off+8] = le.BlockedMG
    le.theta[off+9] = le.BlockedEG
    le.theta[off+10] = le.PawnLeverMG
    le.theta[off+11] = le.PawnLeverEG
    le.theta[off+12] = le.BackwardMG
    le.theta[off+13] = le.BackwardEG
    return off + 14
}

func (le *LinearEval) writeMobilityToTheta(off int) int {
    for i := 0; i < 7; i++ { le.theta[off+i] = le.MobilityMG[i] }
    off += 7
    for i := 0; i < 7; i++ { le.theta[off+i] = le.MobilityEG[i] }
    off += 7
    return off
}

func (le *LinearEval) writeKingTableToTheta(off int) int {
    for i := 0; i < 100; i++ { le.theta[off+i] = le.KingSafety[i] }
    return off + 100
}

func (le *LinearEval) writeKingCorrToTheta(off int) int {
    le.theta[off+0] = le.KingSemiOpenFilePenalty
    le.theta[off+1] = le.KingOpenFilePenalty
    le.theta[off+2] = le.KingMinorPieceDefense
    le.theta[off+3] = le.KingPawnDefenseMG
    return off + 4
}

func (le *LinearEval) writeExtrasToTheta(off int) int {
    le.theta[off+0] = le.KnightOutpostMG
    le.theta[off+1] = le.KnightOutpostEG
    le.theta[off+2] = le.KnightThreatsMG
    le.theta[off+3] = le.KnightThreatsEG
    le.theta[off+4] = le.StackedRooksMG
    le.theta[off+5] = le.RookXrayQueenMG
    le.theta[off+6] = le.ConnectedRooksMG
    le.theta[off+7] = le.BishopOutpostMG
    // New extras
    le.theta[off+8]  = le.BishopXrayKingMG
    le.theta[off+9]  = le.BishopXrayRookMG
    le.theta[off+10] = le.BishopXrayQueenMG
    le.theta[off+11] = le.PawnStormMG
    le.theta[off+12] = le.PawnProximityMG
    le.theta[off+13] = le.PawnLeverStormMG
    le.theta[off+14] = le.KnightMobCenterMG
    le.theta[off+15] = le.BishopMobCenterMG
    return off + 16
}

func (le *LinearEval) writeWeakTempoToTheta(off int) int {
    le.theta[off+0] = le.WeakSquaresMG
    le.theta[off+1] = le.WeakKingSquaresMG
    le.theta[off+2] = le.Tempo
    return off + 3
}

// ---- Reads (theta -> struct) ----

func (le *LinearEval) readPSTFromTheta(off int) int {
    for pt := 0; pt < 6; pt++ { copy(le.PST.MG[pt][:], le.theta[off:off+64]); off += 64 }
    for pt := 0; pt < 6; pt++ { copy(le.PST.EG[pt][:], le.theta[off:off+64]); off += 64 }
    return off
}

func (le *LinearEval) readMaterialFromTheta(off int) int {
    for i := 0; i < 6; i++ { le.MatMG[i] = le.theta[off+i] }
    off += 6
    for i := 0; i < 6; i++ { le.MatEG[i] = le.theta[off+i] }
    off += 6
    return off
}

func (le *LinearEval) readPassersFromTheta(off int) int {
    for i := 0; i < 64; i++ { le.PasserMG[i] = le.theta[off+i] }
    off += 64
    for i := 0; i < 64; i++ { le.PasserEG[i] = le.theta[off+i] }
    off += 64
    return off
}

func (le *LinearEval) readP1ScalarsFromTheta(off int) int {
    le.BishopPairMG = le.theta[off+0]
    le.BishopPairEG = le.theta[off+1]
    le.RookSemiOpenFileMG = le.theta[off+2]
    le.RookOpenFileMG = le.theta[off+3]
    le.SeventhRankEG = le.theta[off+4]
    le.QueenCentralizedEG = le.theta[off+5]
    le.QueenInfiltrationMG = le.theta[off+6]
    le.QueenInfiltrationEG = le.theta[off+7]
    return off + 8
}

func (le *LinearEval) readPawnStructFromTheta(off int) int {
    le.DoubledMG = le.theta[off+0]
    le.DoubledEG = le.theta[off+1]
    le.IsolatedMG = le.theta[off+2]
    le.IsolatedEG = le.theta[off+3]
    le.ConnectedMG = le.theta[off+4]
    le.ConnectedEG = le.theta[off+5]
    le.PhalanxMG = le.theta[off+6]
    le.PhalanxEG = le.theta[off+7]
    le.BlockedMG = le.theta[off+8]
    le.BlockedEG = le.theta[off+9]
    le.PawnLeverMG = le.theta[off+10]
    le.PawnLeverEG = le.theta[off+11]
    le.BackwardMG = le.theta[off+12]
    le.BackwardEG = le.theta[off+13]
    return off + 14
}

func (le *LinearEval) readMobilityFromTheta(off int) int {
    for i := 0; i < 7; i++ { le.MobilityMG[i] = le.theta[off+i] }
    off += 7
    for i := 0; i < 7; i++ { le.MobilityEG[i] = le.theta[off+i] }
    off += 7
    return off
}

func (le *LinearEval) readKingTableFromTheta(off int) int {
    for i := 0; i < 100; i++ { le.KingSafety[i] = le.theta[off+i] }
    return off + 100
}

func (le *LinearEval) readKingCorrFromTheta(off int) int {
    le.KingSemiOpenFilePenalty = le.theta[off+0]
    le.KingOpenFilePenalty = le.theta[off+1]
    le.KingMinorPieceDefense = le.theta[off+2]
    le.KingPawnDefenseMG = le.theta[off+3]
    return off + 4
}

func (le *LinearEval) readExtrasFromTheta(off int) int {
    le.KnightOutpostMG   = le.theta[off+0]
    le.KnightOutpostEG   = le.theta[off+1]
    le.KnightThreatsMG   = le.theta[off+2]
    le.KnightThreatsEG   = le.theta[off+3]
    le.StackedRooksMG    = le.theta[off+4]
    le.RookXrayQueenMG   = le.theta[off+5]
    le.ConnectedRooksMG  = le.theta[off+6]
    le.BishopOutpostMG   = le.theta[off+7]
    // New extras
    le.BishopXrayKingMG   = le.theta[off+8]
    le.BishopXrayRookMG   = le.theta[off+9]
    le.BishopXrayQueenMG  = le.theta[off+10]
    le.PawnStormMG        = le.theta[off+11]
    le.PawnProximityMG    = le.theta[off+12]
    le.PawnLeverStormMG   = le.theta[off+13]
    le.KnightMobCenterMG  = le.theta[off+14]
    le.BishopMobCenterMG  = le.theta[off+15]
    return off + 16
}

func (le *LinearEval) readWeakTempoFromTheta(off int) int {
    le.WeakSquaresMG     = le.theta[off+0]
    le.WeakKingSquaresMG = le.theta[off+1]
    le.Tempo             = le.theta[off+2]
    return off + 3
}
