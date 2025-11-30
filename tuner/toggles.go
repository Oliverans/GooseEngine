package tuner

// PhaseToggles controls inclusion of each stage for Eval (model use)
// and Train (gradient updates). Order mirrors README_eval_tuning_stages-plan.md:
// Stages: 1) PST + Material, 2) Passers, 3) Core PawnStruct, 4) Activity (Mobility + stage-4 Extras/P1), 5) King safety, 6) Aggressive Extras, 7) Rook/Queen Extras/P1, 8) Imbalance, 9-10) Weak squares + Tempo.
//
// Leaving Eval=true and Train=false "freezes" a stage; setting both=false removes its effect.
type PhaseToggles struct {
	// Stage 1: Baseline PST + Material
	PSTEval, PSTTrain           bool
	MaterialEval, MaterialTrain bool

	// Stage 2: Passed pawns
	PassersEval, PassersTrain bool

	// Stage 3: Core pawn structure
	PawnStructEval, PawnStructTrain bool

	// Stage 4: Piece activity (mobility + simple extras/P1)
	MobilityEval, MobilityTrain bool
	Extras4Eval, Extras4Train   bool
	P1Eval, P1Train             bool

	// Stage 6: Aggressive extras (pawn storm/proximity/lever storm)
	Extras6Eval, Extras6Train bool

	// Stage 7: Rook/queen structure extras + P1 rook-file/queen infil/centralization
	Extras7Eval, Extras7Train bool

	// Stage 5: King safety table + correlates
	KingTableEval, KingTableTrain bool // Table (100)
	KingCorrEval, KingCorrTrain   bool // Correlates (4)

	// Stage 8: Material imbalance
	ImbalanceEval, ImbalanceTrain bool

	// Stage 9-10: Weak squares + tempo
	WeakTempoEval, WeakTempoTrain bool

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

	// Stage 3: Pawn structure (16) — order must match writePawnStructToTheta.
	// Core: Doubled/Isolated/Connected/Phalanx/Blocked/Backward.
	// Stage 6 aggression: PawnLever/WeakLever.
	// 0: DoubledMG,1: DoubledEG,2:IsolatedMG,3:IsolatedEG,4:ConnectedMG,5:ConnectedEG,
	// 6: PhalanxMG,7: PhalanxEG,8:BlockedMG,9:BlockedEG,10:PawnLeverMG,11:PawnLeverEG,
	// 12: WeakLeverMG,13: WeakLeverEG,14: BackwardMG,15: BackwardEG
	PawnStruct [16]bool
	// Named toggles
	DoubledMG   bool
	DoubledEG   bool
	IsolatedMG  bool
	IsolatedEG  bool
	ConnectedMG bool
	ConnectedEG bool
	PhalanxMG   bool
	PhalanxEG   bool
	BlockedMG   bool
	BlockedEG   bool
	PawnLeverMG bool
	PawnLeverEG bool
	WeakLeverMG bool
	WeakLeverEG bool
	BackwardMG  bool
	BackwardEG  bool

	// Stage 4: Mobility per piece (7) for MG and EG (index by gm piece: 0..6).
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

	// Stage 4/7/9/11: P1 scalars (8) — order must match writeP1ScalarsToTheta/readP1ScalarsFromTheta.
	// Stage 4 core P1, Stage 7 rook-file/queen infil/centralization, Stage 9/11 optional extras.
	// 0: BishopPairMG, 1: BishopPairEG, 2: RookSemiOpenFileMG, 3: RookOpenFileMG,
	// 4: SeventhRankEG, 5: QueenCentralizedEG, 6: QueenInfiltrationMG, 7: QueenInfiltrationEG
	P1 [8]bool
	// Named toggles for convenience
	BishopPairMG        bool
	BishopPairEG        bool
	RookSemiOpenFileMG  bool
	RookOpenFileMG      bool
	SeventhRankEG       bool
	QueenCentralizedEG  bool
	QueenInfiltrationMG bool
	QueenInfiltrationEG bool

	// Stage 4/6/7: Extras (16), order must match writeExtrasToTheta/readExtrasFromTheta.
	// Stage 4: knight/bishop outposts + knight threats/mobility scaling.
	// Stage 6: pawn storm/proximity/lever storm.
	// Stage 7: rook/queen structure x-rays/stack/connect.
	Extras [16]bool
	// Named extras
	ExtraKnightOutpostMG   bool
	ExtraKnightOutpostEG   bool
	ExtraKnightThreatsMG   bool
	ExtraKnightThreatsEG   bool
	ExtraStackedRooksMG    bool
	ExtraRookXrayQueenMG   bool
	ExtraConnectedRooksMG  bool
	ExtraBishopOutpostMG   bool
	ExtraBishopXrayKingMG  bool
	ExtraBishopXrayRookMG  bool
	ExtraBishopXrayQueenMG bool
	ExtraPawnStormMG       bool
	ExtraPawnProximityMG   bool
	ExtraPawnLeverStormMG  bool
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

	// Stage 8: Imbalance scalars (12), order matches writeImbalanceToTheta.
	Imbalance             [12]bool
	ImbKnightPerPawnMG    bool
	ImbKnightPerPawnEG    bool
	ImbBishopPerPawnMG    bool
	ImbBishopPerPawnEG    bool
	ImbMinorsForMajorMG   bool
	ImbMinorsForMajorEG   bool
	ImbRedundantRookMG    bool
	ImbRedundantRookEG    bool
	ImbRookQueenOverlapMG bool
	ImbRookQueenOverlapEG bool
	ImbQueenManyMinorsMG  bool
	ImbQueenManyMinorsEG  bool

	// Stage 5 optional / Stage 9-10 standard: Weak squares + Tempo (3): WeakSquaresMG, WeakKingSquaresMG, Tempo.
	WeakTempo     [3]bool
	WeakSquaresMG bool
	WeakKingsMG   bool
	Tempo         bool
}

// DefaultEvalToggles: turn on all evaluation paths (training toggles ignored in Eval).
func DefaultEvalToggles() PhaseToggles {
	return PhaseToggles{
		PSTEval: true, PSTTrain: false,
		MaterialEval: true, MaterialTrain: false,
		PassersEval: true, PassersTrain: false,
		PawnStructEval: true, PawnStructTrain: false,
		MobilityEval: true, MobilityTrain: false,
		Extras4Eval: true, Extras4Train: false,
		P1Eval: true, P1Train: false,
		Extras6Eval: true, Extras6Train: false,
		Extras7Eval: true, Extras7Train: false,
		KingTableEval: true, KingTableTrain: false,
		KingCorrEval: true, KingCorrTrain: false,
		ImbalanceEval: true, ImbalanceTrain: false,
		WeakTempoEval: true, WeakTempoTrain: true,
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
	t.QueenInfiltrationMG, t.QueenInfiltrationEG = true, true
	t.DoubledMG, t.DoubledEG = true, true
	t.IsolatedMG, t.IsolatedEG = true, true
	t.ConnectedMG, t.ConnectedEG = true, true
	t.PhalanxMG, t.PhalanxEG = true, true
	t.BlockedMG, t.BlockedEG = true, true
	t.PawnLeverMG, t.PawnLeverEG = true, true
	t.WeakLeverMG, t.WeakLeverEG = true, true
	t.BackwardMG, t.BackwardEG = true, true
	t.MobilityMG_P, t.MobilityMG_N, t.MobilityMG_B, t.MobilityMG_R, t.MobilityMG_Q, t.MobilityMG_K = true, true, true, true, true, true
	t.MobilityEG_P, t.MobilityEG_N, t.MobilityEG_B, t.MobilityEG_R, t.MobilityEG_Q, t.MobilityEG_K = true, true, true, true, true, true
	t.KingSemiOpen, t.KingOpen, t.KingMinor, t.KingPawnMG = true, true, true, true
	t.ExtraKnightOutpostMG, t.ExtraKnightOutpostEG = true, true
	t.ExtraKnightThreatsMG, t.ExtraKnightThreatsEG = true, true
	t.ExtraStackedRooksMG, t.ExtraRookXrayQueenMG, t.ExtraConnectedRooksMG = true, true, true
	t.ExtraBishopOutpostMG = true
	t.ExtraBishopXrayKingMG, t.ExtraBishopXrayRookMG, t.ExtraBishopXrayQueenMG = true, true, true
	t.ExtraPawnStormMG, t.ExtraPawnProximityMG, t.ExtraPawnLeverStormMG = true, true, true
	t.ExtraKnightMobCenterMG, t.ExtraBishopMobCenterMG = true, true
	t.ImbKnightPerPawnMG, t.ImbKnightPerPawnEG = true, true
	t.ImbBishopPerPawnMG, t.ImbBishopPerPawnEG = true, true
	t.ImbMinorsForMajorMG, t.ImbMinorsForMajorEG = true, true
	t.ImbRedundantRookMG, t.ImbRedundantRookEG = true, true
	t.ImbRookQueenOverlapMG, t.ImbRookQueenOverlapEG = true, true
	t.ImbQueenManyMinorsMG, t.ImbQueenManyMinorsEG = true, true
	t.WeakSquaresMG, t.WeakKingsMG, t.Tempo = true, true, true
	return t
}

// ensureToggles initializes default toggles if none are set.
func (le *LinearEval) ensureToggles() {
	t := le.Toggles
	if !(t.PSTEval || t.MaterialEval || t.PassersEval || t.PawnStructEval || t.MobilityEval || t.Extras4Eval || t.Extras6Eval || t.Extras7Eval || t.P1Eval || t.KingTableEval || t.KingCorrEval || t.ImbalanceEval || t.WeakTempoEval) {
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
