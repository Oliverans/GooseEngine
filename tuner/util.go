// tuner/util.go
package tuner

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// BuildLRScale returns a per-parameter LR multiplier aligned with the current
// LinearEval θ layout. Defaults to 1.0 for unknown featurizers.
// Groups (length 944):
//   - PST MG (384) -> 1.0
//   - PST EG (384) -> 1.0
//   - Material MG (6) -> 0.25
//   - Material EG (6) -> 0.25
//   - Passers MG (64) -> 0.5
//   - Passers EG (64) -> 0.5
//   - Phase 1 scalars (8) -> 0.5
//   - Pawn structure (14) -> 0.5
//   - Mobility MG (7) -> 0.25
//   - Mobility EG (7) -> 0.25
func BuildLRScale(fe Featurizer) []float64 {
    if fe == nil {
        return nil
    }
    theta := fe.Params()
    scale := make([]float64, len(theta))
    for i := range scale {
        scale[i] = 1.0
    }
    le, ok := fe.(*LinearEval)
    if !ok {
        return scale
    }
    // Use computed layout and fine-grained toggles
    le.ensureLayout()
    if len(theta) != le.layout.Total {
        return scale
    }
    t := le.Toggles.ParamTrain
    apply := func(start, n int, mult float64, enabled bool) {
        for i := 0; i < n; i++ {
            if enabled { scale[start+i] = mult } else { scale[start+i] = 0.0 }
        }
    }
    // PST
    apply(le.layout.PSTMGStart, 384, 1.0, t.PSTMG)
    apply(le.layout.PSTEGStart, 384, 1.0, t.PSTEG)
    // Material
    apply(le.layout.MaterialMGStart, 6, 0.25, t.MaterialMG)
    apply(le.layout.MaterialEGStart, 6, 0.25, t.MaterialEG)
    // Passers
    apply(le.layout.PasserMGStart, 64, 0.5, t.PassersMG)
    apply(le.layout.PasserEGStart, 64, 0.5, t.PassersEG)
    // P1 scalars (8). Prefer named toggles when set, else fall back to array.
    p1Enabled := [8]bool{
        pickBool(t.BishopPairMG, t.P1[0]),
        pickBool(t.BishopPairEG, t.P1[1]),
        pickBool(t.RookSemiOpenFileMG, t.P1[2]),
        pickBool(t.RookOpenFileMG, t.P1[3]),
        pickBool(t.SeventhRankEG, t.P1[4]),
        pickBool(t.QueenCentralizedEG, t.P1[5]),
        pickBool(t.QueenInfiltrationMG, t.P1[6]),
        pickBool(t.QueenInfiltrationEG, t.P1[7]),
    }
    for i := 0; i < 8; i++ { if p1Enabled[i] { scale[le.layout.P1Start+i] = 0.5 } else { scale[le.layout.P1Start+i] = 0.0 } }
    // Pawn structure (14)
    pawnEnabled := [14]bool{
        pickBool(t.DoubledMG, t.PawnStruct[0]),
        pickBool(t.DoubledEG, t.PawnStruct[1]),
        pickBool(t.IsolatedMG, t.PawnStruct[2]),
        pickBool(t.IsolatedEG, t.PawnStruct[3]),
        pickBool(t.ConnectedMG, t.PawnStruct[4]),
        pickBool(t.ConnectedEG, t.PawnStruct[5]),
        pickBool(t.PhalanxMG, t.PawnStruct[6]),
        pickBool(t.PhalanxEG, t.PawnStruct[7]),
        pickBool(t.BlockedMG, t.PawnStruct[8]),
        pickBool(t.BlockedEG, t.PawnStruct[9]),
        pickBool(t.PawnLeverMG, t.PawnStruct[10]),
        pickBool(t.PawnLeverEG, t.PawnStruct[11]),
        pickBool(t.BackwardMG, t.PawnStruct[12]),
        pickBool(t.BackwardEG, t.PawnStruct[13]),
    }
    for i := 0; i < 14; i++ { if pawnEnabled[i] { scale[le.layout.PawnStructStart+i] = 0.5 } else { scale[le.layout.PawnStructStart+i] = 0.0 } }
    // Mobility MG/EG (7 each). Map named toggles to indices P..K
    mgEnabled := [7]bool{}
    egEnabled := [7]bool{}
    // MG named mapping (skip King index 5 default to named)
    mgEnabled[0] = pickBool(t.MobilityMG_P, t.MobilityMG[0])
    mgEnabled[1] = pickBool(t.MobilityMG_N, t.MobilityMG[1])
    mgEnabled[2] = pickBool(t.MobilityMG_B, t.MobilityMG[2])
    mgEnabled[3] = pickBool(t.MobilityMG_R, t.MobilityMG[3])
    mgEnabled[4] = pickBool(t.MobilityMG_Q, t.MobilityMG[4])
    mgEnabled[5] = pickBool(t.MobilityMG_K, t.MobilityMG[5])
    // EG named mapping
    egEnabled[0] = pickBool(t.MobilityEG_P, t.MobilityEG[0])
    egEnabled[1] = pickBool(t.MobilityEG_N, t.MobilityEG[1])
    egEnabled[2] = pickBool(t.MobilityEG_B, t.MobilityEG[2])
    egEnabled[3] = pickBool(t.MobilityEG_R, t.MobilityEG[3])
    egEnabled[4] = pickBool(t.MobilityEG_Q, t.MobilityEG[4])
    egEnabled[5] = pickBool(t.MobilityEG_K, t.MobilityEG[5])
    for i := 0; i < 7; i++ { if mgEnabled[i] { scale[le.layout.MobilityMGStart+i] = 0.02 } else { scale[le.layout.MobilityMGStart+i] = 0.0 } }
    for i := 0; i < 7; i++ { if egEnabled[i] { scale[le.layout.MobilityEGStart+i] = 0.02 } else { scale[le.layout.MobilityEGStart+i] = 0.0 } }
    // King safety table and correlates
    apply(le.layout.KingTableStart, 100, 0.02, t.KingTable)
    kc := [4]bool{
        pickBool(t.KingSemiOpen, t.KingCorr[0]),
        pickBool(t.KingOpen, t.KingCorr[1]),
        pickBool(t.KingMinor, t.KingCorr[2]),
        pickBool(t.KingPawnMG, t.KingCorr[3]),
    }
    for i := 0; i < 4; i++ { if kc[i] { scale[le.layout.KingCorrStart+i] = 0.5 } else { scale[le.layout.KingCorrStart+i] = 0.0 } }
    // Extras (16)
    ex := [16]bool{
        pickBool(t.ExtraKnightOutpostMG, t.Extras[0]),
        pickBool(t.ExtraKnightOutpostEG, t.Extras[1]),
        pickBool(t.ExtraKnightThreatsMG, t.Extras[2]),
        pickBool(t.ExtraKnightThreatsEG, t.Extras[3]),
        pickBool(t.ExtraStackedRooksMG, t.Extras[4]),
        pickBool(t.ExtraRookXrayQueenMG, t.Extras[5]),
        pickBool(t.ExtraConnectedRooksMG, t.Extras[6]),
        pickBool(t.ExtraBishopOutpostMG, t.Extras[7]),
        pickBool(t.ExtraBishopXrayKingMG, t.Extras[8]),
        pickBool(t.ExtraBishopXrayRookMG, t.Extras[9]),
        pickBool(t.ExtraBishopXrayQueenMG, t.Extras[10]),
        pickBool(t.ExtraPawnStormMG, t.Extras[11]),
        pickBool(t.ExtraPawnProximityMG, t.Extras[12]),
        pickBool(t.ExtraPawnLeverStormMG, t.Extras[13]),
        pickBool(t.ExtraKnightMobCenterMG, t.Extras[14]),
        pickBool(t.ExtraBishopMobCenterMG, t.Extras[15]),
    }
    for i := 0; i < 16; i++ { if ex[i] { scale[le.layout.ExtrasStart+i] = 0.5 } else { scale[le.layout.ExtrasStart+i] = 0.0 } }
    // Weak + tempo (3)
    wt := [3]bool{
        pickBool(t.WeakSquaresMG, t.WeakTempo[0]),
        pickBool(t.WeakKingsMG, t.WeakTempo[1]),
        pickBool(t.Tempo, t.WeakTempo[2]),
    }
    for i := 0; i < 3; i++ { if wt[i] { scale[le.layout.WeakTempoStart+i] = 0.5 } else { scale[le.layout.WeakTempoStart+i] = 0.0 } }
    return scale
}

// pickBool returns named if named is true; otherwise fallback.
func pickBool(named bool, fallback bool) bool {
    // Named toggles are authoritative. DefaultTrainToggles sets them true
    // by default, so explicitly setting false will disable the feature even
    // if array-based fallbacks are true. Fallback is ignored once named exists.
    return named
}
