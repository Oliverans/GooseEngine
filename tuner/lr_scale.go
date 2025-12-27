package tuner

// DefaultLRScales returns the recommended per-parameter learning rate multipliers.
func DefaultLRScales() LRScaleConfig {
	return LRScaleConfig{

		// Freeze
		KingSafetyTable: 0.0,

		// Tier 1: Strong constraint
		KnightOutpost:         0.3,
		BishopOutpost:         0.3,
		KingSemiOpenFile:      0.3,
		KingOpenFile:          0.3,
		PawnStormBaseMG:       0.3,
		PawnStormLeverPct:     0.3,
		PawnStormWeakLeverPct: 0.3,
		PawnStormBlockedPct:   0.3,
		Imbalance:             0.3,

		// Tier 2: Moderate constraint
		BishopPair:            0.5,
		RookFiles:             0.5,
		SeventhRank:           0.5,
		BackwardPawn:          0.5,
		IsolatedPawn:          0.5,
		DoubledPawn:           0.5,
		CandidatePassed:       0.5,
		PawnWeakLever:         0.5,
		KingDefense:           0.5,
		Tempo:                 0.5,
		PawnStormOppositeMult: 0.5,
		Space:                 0.5,
		Material:              0.5,

		// Tier 3: Light constraint
		ConnectedPawn: 0.7,
		BlockedPawn:   0.7,
		BadBishop:     0.7,
		PawnPhalanx:   0.7,
		KnightTropism: 0.7,
		StackedRooks:  0.7,

		// Free: large groups, abundant signal
		PST:        1.0,
		PasserPSQT: 1.0,
		Mobility:   1.0,
	}
}

// BuildLRScaleVector constructs the per-parameter LR scale vector matching the layout order.
func BuildLRScaleVector(layout Layout, cfg LRScaleConfig) []float64 {
	scales := make([]float64, layout.Total)
	for i := range scales {
		scales[i] = 1.0
	}

	// PST blocks (MG/EG)
	for i := layout.PSTMGStart; i < layout.PSTMGStart+384; i++ {
		scales[i] = cfg.PST
	}
	for i := layout.PSTEGStart; i < layout.PSTEGStart+384; i++ {
		scales[i] = cfg.PST
	}

	// Material (MG/EG)
	for i := layout.MaterialMGStart; i < layout.MaterialMGStart+6; i++ {
		scales[i] = cfg.Material
	}
	for i := layout.MaterialEGStart; i < layout.MaterialEGStart+6; i++ {
		scales[i] = cfg.Material
	}

	// Passed pawn PSQT (MG/EG)
	for i := layout.PasserMGStart; i < layout.PasserMGStart+64; i++ {
		scales[i] = cfg.PasserPSQT
	}
	for i := layout.PasserEGStart; i < layout.PasserEGStart+64; i++ {
		scales[i] = cfg.PasserPSQT
	}

	// Phase 1 scalars
	p1 := layout.P1Start
	scales[p1+0] = cfg.BishopPair  // BishopPairMG
	scales[p1+1] = cfg.BishopPair  // BishopPairEG
	scales[p1+2] = cfg.RookFiles   // RookSemiOpenFileMG
	scales[p1+3] = cfg.RookFiles   // RookOpenFileMG
	scales[p1+4] = cfg.SeventhRank // SeventhRankEG
	// p1+5 QueenCentralizedEG remains free

	// Phase 2: Pawn structure scalars
	ps := layout.PawnStructStart
	scales[ps+0] = cfg.DoubledPawn      // DoubledMG
	scales[ps+1] = cfg.DoubledPawn      // DoubledEG
	scales[ps+2] = cfg.IsolatedPawn     // IsolatedMG
	scales[ps+3] = cfg.IsolatedPawn     // IsolatedEG
	scales[ps+4] = cfg.ConnectedPawn    // ConnectedMG
	scales[ps+5] = cfg.ConnectedPawn    // ConnectedEG
	scales[ps+6] = cfg.PawnPhalanx      // PhalanxMG (Tier 1)
	scales[ps+7] = cfg.PawnPhalanx      // PhalanxEG (Tier 1)
	scales[ps+8] = cfg.BlockedPawn      // BlockedMG
	scales[ps+9] = cfg.BlockedPawn      // BlockedEG
	scales[ps+10] = cfg.PawnWeakLever   // WeakLeverMG (Tier 1)
	scales[ps+11] = cfg.PawnWeakLever   // WeakLeverEG (Tier 1)
	scales[ps+12] = cfg.BackwardPawn    // BackwardMG
	scales[ps+13] = cfg.BackwardPawn    // BackwardEG
	scales[ps+14] = cfg.CandidatePassed // CandidatePassedPctMG
	scales[ps+15] = cfg.CandidatePassed // CandidatePassedPctEG

	// Phase 3: Mobility (MG/EG)
	mobilityCount := 9 + 14 + 15 + 22
	for i := layout.MobilityMGStart; i < layout.MobilityMGStart+mobilityCount; i++ {
		scales[i] = cfg.Mobility
	}
	for i := layout.MobilityEGStart; i < layout.MobilityEGStart+mobilityCount; i++ {
		scales[i] = cfg.Mobility
	}

	// Phase 4: King safety table
	for i := layout.KingTableStart; i < layout.KingTableStart+100; i++ {
		scales[i] = cfg.KingSafetyTable
	}

	// Phase 4: King safety correlates
	ksc := layout.KingCorrStart
	scales[ksc+0] = cfg.KingSemiOpenFile
	scales[ksc+1] = cfg.KingOpenFile
	scales[ksc+2] = cfg.KingDefense
	scales[ksc+3] = cfg.KingDefense

	// Phase 4: King endgame (free)

	// Phase 5: Extras
	ex := layout.ExtrasStart
	scales[ex+0] = cfg.KnightOutpost // KnightOutpostMG
	scales[ex+1] = cfg.KnightOutpost // KnightOutpostEG
	// KnightTropismMG/EG now at offsets 2/3
	scales[ex+2] = cfg.KnightTropism // KnightTropismMG
	scales[ex+3] = cfg.KnightTropism // KnightTropismEG
	// StackedRooks/BishopOutpost at offsets 4/5/6
	scales[ex+4] = cfg.StackedRooks  // StackedRooksMG
	scales[ex+5] = cfg.BishopOutpost // BishopOutpostMG
	scales[ex+6] = cfg.BishopOutpost // BishopOutpostEG
	// Pawn storm percentage arrays (per-rank)
	for i := 0; i < 8; i++ {
		scales[ex+7+i] = cfg.PawnStormBaseMG        // FreePct
		scales[ex+15+i] = cfg.PawnStormLeverPct     // LeverPct
		scales[ex+23+i] = cfg.PawnStormWeakLeverPct // WeakLeverPct
		scales[ex+31+i] = cfg.PawnStormBlockedPct   // BlockedPct
	}
	// Opposite-side multiplier
	scales[ex+39] = cfg.PawnStormOppositeMult
	// Pawn storm base (per-rank)
	for i := 0; i < 8; i++ {
		scales[ex+43+i] = cfg.PawnStormBaseMG
	}
	// Bad bishop (MG/EG)
	scales[ex+51] = cfg.BadBishop
	scales[ex+52] = cfg.BadBishop

	// Phase 6: Imbalance
	for i := layout.ImbalanceStart; i < layout.ImbalanceStart+4; i++ {
		scales[i] = cfg.Imbalance
	}

	// Phase 7: Space/weak-king + Tempo
	wt := layout.WeakTempoStart
	scales[wt+0] = cfg.Space // SpaceMG
	scales[wt+1] = cfg.Space // SpaceEG
	// wt+2 weak king MG remains free
	scales[wt+3] = cfg.Tempo // Tempo

	return scales
}
