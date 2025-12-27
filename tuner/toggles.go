package tuner

// PhaseToggles control which tiers run for evaluation and training.
// Turn Eval=true to include a tier in scoring; turn Train=true to update its weights.
// Eval=true + Train=false = freeze tier; both=false = exclude tier entirely.
//
// Tier structure:
//
//	Tier 1 — Core: PST, Material, Mobility, Outposts, RookFiles, StackedRooks, MobCenter scaling
//	Tier 2 — Pawns: Passers, PawnStruct (doubled/isolated/connected/phalanx/blocked/weak lever/backward)
//	Tier 3 — King Safety: KingTable, KingCorr, KingEndgame, Tropism, PawnStorm, WeakKingSquares
//	Tier 4 — Misc: BishopPair, Imbalance, Space, Tempo
//
// Suggested tuning sequence: Tier1 → Tier2 → Tier3 → Tier4
type PhaseToggles struct {
	// Tier 1: Core (PST, Material, Mobility, Piece Activity)
	Tier1Eval, Tier1Train bool

	// Tier 2: Pawn Structure (Passers + PawnStruct)
	Tier2Eval, Tier2Train bool

	// Tier 3: King Safety (Table, Corr, Endgame, Tropism, PawnStorm, WeakKingSquares)
	Tier3Eval, Tier3Train bool

	// Tier 4: Misc (BishopPair, Imbalance, Space, Tempo)
	Tier4Eval, Tier4Train bool

	// Fine-grained training toggles per-parameter family (used only to filter gradients)
	ParamTrain ParamTrainToggles
}

// ParamTrainToggles exposes fine-grained switches for every parameter block.
// These only affect training (via LR scale filtering); Eval always uses all terms.
// Stage coverage follows README_eval_tuning_stages-plan.md for clarity.
type ParamTrainToggles struct {
	// Stage 1-2 blocks
	PSTMG, PSTEG           bool
	MaterialMG, MaterialEG bool
	PassersMG, PassersEG   bool

	PawnStruct [16]bool
	// Named toggles
	DoubledMG         bool
	DoubledEG         bool
	IsolatedMG        bool
	IsolatedEG        bool
	ConnectedMG       bool
	ConnectedEG       bool
	PhalanxMG         bool
	PhalanxEG         bool
	BlockedMG         bool
	BlockedEG         bool
	WeakLeverMG       bool
	WeakLeverEG       bool
	BackwardMG        bool
	BackwardEG        bool
	CandidatePassedMG bool
	CandidatePassedEG bool

	// Stage 4: Mobility per piece (index by gm piece: 0..6), covers table entries per piece.
	MobilityMG [7]bool
	MobilityEG [7]bool
	// Named toggles by piece (P,N,B,R,Q,K indices from consts.go)
	MobilityMG_P bool
	MobilityMG_N bool
	MobilityMG_B bool
	MobilityMG_R bool
	MobilityMG_Q bool
	MobilityMG_K bool
	MobilityEG_P bool
	MobilityEG_N bool
	MobilityEG_B bool
	MobilityEG_R bool
	MobilityEG_Q bool
	MobilityEG_K bool

	P1 [6]bool
	// Named toggles for convenience
	BishopPairMG       bool
	BishopPairEG       bool
	RookSemiOpenFileMG bool
	RookOpenFileMG     bool
	SeventhRankEG      bool
	QueenCentralizedEG bool

	Extras [11]bool
	// Named extras
	ExtraKnightOutpostMG   bool
	ExtraKnightOutpostEG   bool
	ExtraStackedRooksMG    bool
	ExtraBishopOutpostMG   bool
	ExtraBishopOutpostEG   bool
	ExtraPawnStormMG       bool
	ExtraPawnProximityMG   bool
	ExtraKnightMobCenterMG bool
	ExtraBishopMobCenterMG bool

	// Stage 5: King safety table (100) - single switch; correlates (4) individually.
	KingTable bool
	KingCorr  [4]bool
	// Named correlates order: semi-open, open, minor, pawn
	KingSemiOpen bool
	KingOpen     bool
	KingMinor    bool
	KingPawnMG   bool

	// Stage 8: Imbalance scalars (4), order matches writeImbalanceToTheta.
	Imbalance          [4]bool
	ImbKnightPerPawnMG bool
	ImbKnightPerPawnEG bool
	ImbBishopPerPawnMG bool
	ImbBishopPerPawnEG bool

	// Stage 5 optional / Stage 9-10 standard: Space/weak-king + Tempo (4): SpaceMG, SpaceEG, WeakKingSquaresMG, Tempo.
	WeakTempo   [4]bool
	SpaceMG     bool
	SpaceEG     bool
	WeakKingsMG bool
	Tempo       bool
}

// DefaultEvalToggles: turn on all evaluation paths (training toggles ignored in Eval).
func DefaultEvalToggles() PhaseToggles {
	// Recommended tuning order: Tier1 → Tier2 → Tier3 → Tier4
	// After each tier, keep eval true, set train to false
	return PhaseToggles{
		Tier1Eval: true, Tier1Train: true, // Core: PST, Material, Mobility, Outposts, RookFiles, etc.
		Tier2Eval: true, Tier2Train: true, // Pawns: Passers, PawnStruct
		Tier3Eval: true, Tier3Train: true, // KingSafety: Table, Corr, Endgame, Tropism, Storm, WeakKing
		Tier4Eval: true, Tier4Train: true, // Misc: BishopPair, Imbalance, Space, Tempo
		ParamTrain: DefaultTrainToggles(),
	}
}

// DefaultTrainToggles: enable all parameter groups by default; can be edited by the user.
func DefaultTrainToggles() ParamTrainToggles {
	var t ParamTrainToggles
	t.PSTMG, t.PSTEG = true, true
	t.MaterialMG, t.MaterialEG = true, true
	t.PassersMG, t.PassersEG = true, true
	for i := 0; i < len(t.P1); i++ {
		t.P1[i] = true
	}
	for i := 0; i < len(t.PawnStruct); i++ {
		t.PawnStruct[i] = true
	}
	for i := 0; i < len(t.MobilityMG); i++ {
		t.MobilityMG[i] = true
	}
	for i := 0; i < len(t.MobilityEG); i++ {
		t.MobilityEG[i] = true
	}
	t.KingTable = true
	for i := 0; i < len(t.KingCorr); i++ {
		t.KingCorr[i] = true
	}
	for i := 0; i < len(t.Extras); i++ {
		t.Extras[i] = true
	}
	for i := 0; i < len(t.Imbalance); i++ {
		t.Imbalance[i] = true
	}
	for i := 0; i < len(t.WeakTempo); i++ {
		t.WeakTempo[i] = true
	}
	// Named mirrors default to true as well
	t.BishopPairMG, t.BishopPairEG = true, true
	t.RookSemiOpenFileMG, t.RookOpenFileMG = true, true
	t.SeventhRankEG, t.QueenCentralizedEG = true, true
	t.DoubledMG, t.DoubledEG = true, true
	t.IsolatedMG, t.IsolatedEG = true, true
	t.ConnectedMG, t.ConnectedEG = true, true
	t.PhalanxMG, t.PhalanxEG = true, true
	t.BlockedMG, t.BlockedEG = true, true
	t.WeakLeverMG, t.WeakLeverEG = true, true
	t.BackwardMG, t.BackwardEG = true, true
	t.CandidatePassedMG, t.CandidatePassedEG = true, true
	t.MobilityMG_P, t.MobilityMG_N, t.MobilityMG_B, t.MobilityMG_R, t.MobilityMG_Q, t.MobilityMG_K = true, true, true, true, true, true
	t.MobilityEG_P, t.MobilityEG_N, t.MobilityEG_B, t.MobilityEG_R, t.MobilityEG_Q, t.MobilityEG_K = true, true, true, true, true, true
	t.KingSemiOpen, t.KingOpen, t.KingMinor, t.KingPawnMG = true, true, true, true
	t.ExtraKnightOutpostMG, t.ExtraKnightOutpostEG = true, true
	t.ExtraStackedRooksMG = true
	t.ExtraBishopOutpostMG, t.ExtraBishopOutpostEG = true, true
	t.ExtraPawnStormMG, t.ExtraPawnProximityMG = true, true
	t.ExtraKnightMobCenterMG, t.ExtraBishopMobCenterMG = true, true
	t.ImbKnightPerPawnMG, t.ImbKnightPerPawnEG = true, true
	t.ImbBishopPerPawnMG, t.ImbBishopPerPawnEG = true, true
	t.SpaceMG, t.SpaceEG, t.WeakKingsMG, t.Tempo = true, true, true, true
	return t
}

// ensureToggles initializes default toggles if none are set.
func (le *LinearEval) ensureToggles() {
	t := le.Toggles
	if !(t.Tier1Eval || t.Tier2Eval || t.Tier3Eval || t.Tier4Eval) {
		le.Toggles = DefaultEvalToggles()
		return
	}
	// Initialize ParamTrain toggles if zero-value (all false)
	zero := true
	for _, v := range t.ParamTrain.P1 {
		if v {
			zero = false
			break
		}
	}
	if zero {
		le.Toggles.ParamTrain = DefaultTrainToggles()
	}
}
