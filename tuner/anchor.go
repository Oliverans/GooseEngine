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
	weights[ps+6] = cfg.Tier1Lambda  // PhalanxMG
	weights[ps+7] = cfg.Tier1Lambda  // PhalanxEG
	weights[ps+10] = cfg.Tier1Lambda // WeakLeverMG
	weights[ps+11] = cfg.Tier1Lambda // WeakLeverEG

	ksc := layout.KingCorrStart
	weights[ksc+0] = cfg.Tier1Lambda // KingSemiOpenFile
	weights[ksc+1] = cfg.Tier1Lambda // KingOpenFile

	ex := layout.ExtrasStart
	weights[ex+0] = cfg.Tier1Lambda // KnightOutpostMG
	weights[ex+1] = cfg.Tier1Lambda // KnightOutpostEG
	weights[ex+5] = cfg.Tier1Lambda // BishopOutpostMG (note: offset 5 in current layout)
	weights[ex+6] = cfg.Tier1Lambda // BishopOutpostEG

	// --- Tier 2 parameters ---
	p1 := layout.P1Start
	weights[p1+0] = cfg.Tier2Lambda // BishopPairMG
	weights[p1+1] = cfg.Tier2Lambda // BishopPairEG
	weights[p1+2] = cfg.Tier2Lambda // RookSemiOpenFileMG
	weights[p1+3] = cfg.Tier2Lambda // RookOpenFileMG
	weights[p1+4] = cfg.Tier2Lambda // SeventhRankEG

	weights[ps+0] = cfg.Tier2Lambda  // DoubledMG
	weights[ps+1] = cfg.Tier2Lambda  // DoubledEG
	weights[ps+2] = cfg.Tier2Lambda  // IsolatedMG
	weights[ps+3] = cfg.Tier2Lambda  // IsolatedEG
	weights[ps+12] = cfg.Tier2Lambda // BackwardMG
	weights[ps+13] = cfg.Tier2Lambda // BackwardEG

	weights[ksc+2] = cfg.Tier2Lambda // KingMinorDefense
	weights[ksc+3] = cfg.Tier2Lambda // KingPawnDefense

	wt := layout.WeakTempoStart
	weights[wt+3] = cfg.Tier2Lambda // Tempo

	// --- Tier 3 parameters ---
	weights[ps+4] = cfg.Tier3Lambda // ConnectedMG
	weights[ps+5] = cfg.Tier3Lambda // ConnectedEG
	weights[ps+8] = cfg.Tier3Lambda // BlockedMG
	weights[ps+9] = cfg.Tier3Lambda // BlockedEG

	weights[ex+2] = cfg.Tier3Lambda // KnightTropismMG
	weights[ex+3] = cfg.Tier3Lambda // KnightTropismEG
	weights[ex+4] = cfg.Tier3Lambda // StackedRooksMG

	// Material gets light anchor (traditional values are good starting points)
	for i := layout.MaterialMGStart; i < layout.MaterialMGStart+6 && i < len(weights); i++ {
		weights[i] = cfg.Tier3Lambda
	}
	for i := layout.MaterialEGStart; i < layout.MaterialEGStart+6 && i < len(weights); i++ {
		weights[i] = cfg.Tier3Lambda
	}

	return weights
}
