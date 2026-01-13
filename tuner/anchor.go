package tuner

// BuildAnchorWeights constructs per-parameter L2 anchor weights.
// These penalize deviation from the seeded defaults.
func BuildAnchorWeights(layout Layout, cfg AnchorConfig) []float64 {
	weights := make([]float64, layout.Total)

	// Default all to free (minimal regularization)
	for i := range weights {
		weights[i] = cfg.FreeLambda
	}

	// --- Tier 1 parameters ---
	ps := layout.PawnStructStart
	weights[ps+0] = cfg.Tier1Lambda  // DoubledMG
	weights[ps+1] = cfg.Tier1Lambda  // DoubledEG
	weights[ps+2] = cfg.Tier1Lambda  // IsolatedMG
	weights[ps+3] = cfg.Tier1Lambda  // IsolatedEG
	weights[ps+6] = cfg.Tier1Lambda  // PhalanxMG
	weights[ps+7] = cfg.Tier1Lambda  // PhalanxEG
	weights[ps+10] = cfg.Tier1Lambda // WeakLeverMG
	weights[ps+11] = cfg.Tier1Lambda // WeakLeverEG
	weights[ps+12] = cfg.Tier1Lambda // BackwardMG
	weights[ps+13] = cfg.Tier1Lambda // BackwardEG

	ksc := layout.KingCorrStart
	weights[ksc+0] = cfg.Tier1Lambda // KingSemiOpenFile
	weights[ksc+1] = cfg.Tier1Lambda // KingOpenFile

	// --- Tier 2 parameters ---
	bp := layout.BishopPairStart
	weights[bp+0] = cfg.Tier2Lambda // BishopPairMG
	weights[bp+1] = cfg.Tier2Lambda // BishopPairEG

	core := layout.CoreScalarStart
	weights[core+0] = cfg.Tier2Lambda // RookSemiOpenFileMG
	weights[core+1] = cfg.Tier2Lambda // RookOpenFileMG
	weights[core+2] = cfg.Tier2Lambda // SeventhRankEG

	weights[ksc+2] = cfg.Tier2Lambda // KingMinorDefense
	weights[ksc+3] = cfg.Tier2Lambda // KingPawnDefense

	ex1 := layout.Tier1ExtrasStart
	weights[ex1+0] = cfg.Tier2Lambda // KnightOutpostMG
	weights[ex1+1] = cfg.Tier2Lambda // KnightOutpostEG
	weights[ex1+2] = cfg.Tier2Lambda // BishopOutpostMG
	weights[ex1+3] = cfg.Tier2Lambda // BishopOutpostEG

	st := layout.SpaceTempoStart
	weights[st+2] = cfg.Tier2Lambda // Tempo

	// --- Tier 3 parameters ---
	weights[ps+4] = cfg.Tier3Lambda // ConnectedMG
	weights[ps+5] = cfg.Tier3Lambda // ConnectedEG
	weights[ps+8] = cfg.Tier3Lambda // BlockedMG
	weights[ps+9] = cfg.Tier3Lambda // BlockedEG

	ex3 := layout.Tier3ExtrasStart
	weights[ex3+0] = cfg.Tier3Lambda // KnightTropismMG
	weights[ex3+1] = cfg.Tier3Lambda // KnightTropismEG
	weights[ex1+4] = cfg.Tier3Lambda // StackedRooksMG

	// Material gets light anchor (traditional values are good starting points)
	for i := layout.MaterialMGStart; i < layout.MaterialMGStart+6 && i < len(weights); i++ {
		weights[i] = cfg.Tier3Lambda
	}
	for i := layout.MaterialEGStart; i < layout.MaterialEGStart+6 && i < len(weights); i++ {
		weights[i] = cfg.Tier3Lambda
	}

	return weights
}
