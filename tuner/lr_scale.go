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
		Mobility:      0.7,

		// Free: large groups, abundant signal
		PST:        1.0,
		PasserPSQT: 1.0,
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

	// Core scalars
	core := layout.CoreScalarStart
	scales[core+0] = cfg.RookFiles   // RookSemiOpenFileMG
	scales[core+1] = cfg.RookFiles   // RookOpenFileMG
	scales[core+2] = cfg.SeventhRank // SeventhRankEG
	// core+3 QueenCentralizedEG remains free

	// Bishop pair (Tier4)
	bp := layout.BishopPairStart
	scales[bp+0] = cfg.BishopPair // BishopPairMG
	scales[bp+1] = cfg.BishopPair // BishopPairEG

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

	// Tier1 extras
	ex1 := layout.Tier1ExtrasStart
	scales[ex1+0] = cfg.KnightOutpost // KnightOutpostMG
	scales[ex1+1] = cfg.KnightOutpost // KnightOutpostEG
	scales[ex1+2] = cfg.BishopOutpost // BishopOutpostMG
	scales[ex1+3] = cfg.BishopOutpost // BishopOutpostEG
	scales[ex1+4] = cfg.StackedRooks  // StackedRooksMG
	// ex1+5 KnightMobCenterMG remains free
	// ex1+6 BishopMobCenterMG remains free
	scales[ex1+7] = cfg.BadBishop // BadBishopMG
	scales[ex1+8] = cfg.BadBishop // BadBishopEG

	// Tier3 extras
	ex3 := layout.Tier3ExtrasStart
	scales[ex3+0] = cfg.KnightTropism // KnightTropismMG
	scales[ex3+1] = cfg.KnightTropism // KnightTropismEG
	// Pawn storm percentage arrays (per-rank)
	for i := 0; i < 8; i++ {
		scales[ex3+2+i] = cfg.PawnStormBaseMG        // FreePct
		scales[ex3+10+i] = cfg.PawnStormLeverPct     // LeverPct
		scales[ex3+18+i] = cfg.PawnStormWeakLeverPct // WeakLeverPct
		scales[ex3+26+i] = cfg.PawnStormBlockedPct   // BlockedPct
	}
	// Opposite-side multiplier
	scales[ex3+34] = cfg.PawnStormOppositeMult
	// Pawn storm base (per-rank)
	for i := 0; i < 8; i++ {
		scales[ex3+36+i] = cfg.PawnStormBaseMG
	}

	// Phase 6: Imbalance
	for i := layout.ImbalanceStart; i < layout.ImbalanceStart+4; i++ {
		scales[i] = cfg.Imbalance
	}

	// Phase 7: Space + Tempo (WeakKingSquares is its own Tier3 block)
	st := layout.SpaceTempoStart
	scales[st+0] = cfg.Space // SpaceMG
	scales[st+1] = cfg.Space // SpaceEG
	scales[st+2] = cfg.Tempo // Tempo

	return scales
}
