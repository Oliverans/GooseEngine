package tuner

// PhaseToggles controls inclusion of each phase for Eval (model use)
// and Train (gradient updates). Leaving Eval=true and Train=false "freezes"
// a phase; setting both=false effectively removes its effect.
type PhaseToggles struct {
	PSTEval, PSTTrain               bool
	MaterialEval, MaterialTrain     bool
	PassersEval, PassersTrain       bool
	P1Eval, P1Train                 bool // Phase 1 scalars
	PawnStructEval, PawnStructTrain bool // Phase 2
	MobilityEval, MobilityTrain     bool // Phase 3
	KingTableEval, KingTableTrain   bool // Phase 4 table (100)
	KingCorrEval, KingCorrTrain     bool // Phase 4 correlates (4)
	ExtrasEval, ExtrasTrain         bool // Phase 5
	WeakTempoEval, WeakTempoTrain   bool // Phase 6 (weak squares + tempo)

	// Fine-grained training toggles per-parameter family (used only to filter gradients)
	ParamTrain ParamTrainToggles
}

// ParamTrainToggles exposes fine-grained switches for every parameter block.
// These only affect training (via LR scale filtering); Eval always uses all terms.
type ParamTrainToggles struct {
	// PST and Material/Passers block-level toggles
	PSTMG, PSTEG           bool
	MaterialMG, MaterialEG bool
	PassersMG, PassersEG   bool

	// Phase 1 scalars (8), order must match writeP1ScalarsToTheta/readP1ScalarsFromTheta
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

	// Phase 2 pawn structure (14), order must match writePawnStructToTheta
	// 0: DoubledMG,1: DoubledEG,2:IsolatedMG,3:IsolatedEG,4:ConnectedMG,5:ConnectedEG,
	// 6: PhalanxMG,7: PhalanxEG,8:BlockedMG,9:BlockedEG,10:PawnLeverMG,11:PawnLeverEG,12:BackwardMG,13:BackwardEG
	PawnStruct [14]bool
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
	BackwardMG  bool
	BackwardEG  bool

	// Mobility per piece (7) for MG and EG (index by gm piece: 0..6)
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

	// King safety table (100) – single switch; correlates (4) individually
	KingTable bool
	KingCorr  [4]bool
	// Named correlates order: semi-open, open, minor, pawn
	KingSemiOpen bool
	KingOpen     bool
	KingMinor    bool
	KingPawnMG   bool

	// Phase 5 extras (16), order must match writeExtrasToTheta/readExtrasFromTheta
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

	// Phase 6 (3): WeakSquaresMG, WeakKingSquaresMG, Tempo
	WeakTempo     [3]bool
	WeakSquaresMG bool
	WeakKingsMG   bool
	Tempo         bool
}

// DefaultEvalToggles: turn on all evaluation paths (training toggles ignored in Eval).
func DefaultEvalToggles() PhaseToggles {
	return PhaseToggles{
		PSTEval: true, PSTTrain: true,
		MaterialEval: true, MaterialTrain: true,
		PassersEval: true, PassersTrain: true,
		P1Eval: true, P1Train: true,
		PawnStructEval: true, PawnStructTrain: true,
		MobilityEval: true, MobilityTrain: true,
		KingTableEval: true, KingTableTrain: true,
		KingCorrEval: true, KingCorrTrain: true,
		ExtrasEval: true, ExtrasTrain: true,
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
	for i := 0; i < len(t.WeakTempo); i++ {
		t.WeakTempo[i] = true
	}
	// Named mirrors default to true as well
	t.BishopPairMG, t.BishopPairEG = true, true
	t.RookSemiOpenFileMG, t.RookOpenFileMG = true, true
	t.SeventhRankEG, t.QueenCentralizedEG = true, true
	t.QueenInfiltrationMG, t.QueenInfiltrationEG = false, false
	t.DoubledMG, t.DoubledEG = true, true
	t.IsolatedMG, t.IsolatedEG = true, true
	t.ConnectedMG, t.ConnectedEG = true, true
	t.PhalanxMG, t.PhalanxEG = true, true
	t.BlockedMG, t.BlockedEG = true, true
	t.PawnLeverMG, t.PawnLeverEG = true, true
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
	t.WeakSquaresMG, t.WeakKingsMG, t.Tempo = true, true, true
	return t
}

// ensureToggles initializes default toggles if none are set.
func (le *LinearEval) ensureToggles() {
	t := le.Toggles
	if !(t.PSTEval || t.MaterialEval || t.PassersEval || t.P1Eval || t.PawnStructEval || t.MobilityEval || t.KingTableEval || t.KingCorrEval || t.ExtrasEval || t.WeakTempoEval) {
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
