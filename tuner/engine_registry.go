package tuner

import (
	"fmt"
	"math"
	"strconv"

	eng "chess-engine/engine"
	gm "chess-engine/goosemg"
)

// EngineRegistryVersion changes when the semantic parameter layout changes.
const EngineRegistryVersion = "goose-eval-v2"

const (
	groupMaterial      GroupID = "material"
	groupPSQT          GroupID = "psqt"
	groupMobility      GroupID = "mobility"
	groupPawnStructure GroupID = "pawn_structure"
	groupCenter        GroupID = "center"
	groupPieceActivity GroupID = "piece_activity"
	groupRook          GroupID = "rook"
	groupKingSafety    GroupID = "king_safety"
	groupKingPasser    GroupID = "king_passer"
	groupSpace         GroupID = "space"
	groupImbalance     GroupID = "imbalance"
	groupTempo         GroupID = "tempo"
	groupFinalScale    GroupID = "final_scale"
)

// NewEngineRegistry returns the complete active evaluation-parameter registry
// for the current engine. L2-to-anchor strengths are deliberately zero until
// their loss scale is calibrated; anchors and provisional deviation scales
// are still populated now so that calibration does not require a layout
// change or dataset rebuild.
func NewEngineRegistry() (*Registry, error) {
	return newEngineRegistryWithPolicy(engineTrainingPolicy())
}

func newEngineRegistryWithPolicy(policy TrainingPolicy) (*Registry, error) {
	builder := engineRegistryBuilder{}
	builder.addMaterial()
	builder.addPieceSquareTables()
	builder.addPassedPawnTables()
	builder.addMobility()
	builder.addPawnStructure()
	builder.addCenterScaling()
	builder.addPieceActivity()
	builder.addRooksAndQueen()
	builder.addShelterAndStorm()
	builder.addKingDanger()
	builder.addKingPasser()
	builder.addSpace()
	builder.addImbalance()
	builder.scalar("tempo", "TempoBonus", groupTempo, PhaseShared, FormulaLinear, RoleCoefficient, eng.TempoBonus, TrainingContinuous, scalarExport("TempoBonus", EngineInt))
	builder.scalar("final.draw_divider", "DrawDivider", groupFinalScale, PhaseFinal, FormulaFinalScale, RoleFinalDivider, int(eng.DrawDivider), TrainingFrozen, scalarExport("DrawDivider", EngineInt32))

	specs, err := applyTrainingPolicy(builder.specs, policy)
	if err != nil {
		return nil, fmt.Errorf("apply engine training policy: %w", err)
	}
	return NewRegistry(EngineRegistryVersion, engineParameterGroups(), specs)
}

func engineParameterGroups() []GroupSpec {
	// Strength zero means "anchoring policy not calibrated yet", not "these
	// values have no anchor". Learning-rate scales are similarly neutral.
	return []GroupSpec{
		{ID: groupMaterial, Description: "Piece material values", AnchorStrength: 0, LearningRateScale: 1},
		{ID: groupPSQT, Description: "Piece-square and passed-pawn tables", AnchorStrength: 0, LearningRateScale: 1},
		{ID: groupMobility, Description: "Mobility lookup tables", AnchorStrength: 0, LearningRateScale: 1},
		{ID: groupPawnStructure, Description: "Pawn-structure terms", AnchorStrength: 0, LearningRateScale: 1},
		{ID: groupCenter, Description: "Centre-openness scaling", AnchorStrength: 0, LearningRateScale: 1},
		{ID: groupPieceActivity, Description: "Minor-piece and general activity", AnchorStrength: 0, LearningRateScale: 1},
		{ID: groupRook, Description: "Rook and queen placement terms", AnchorStrength: 0, LearningRateScale: 1},
		{ID: groupKingSafety, Description: "Shelter, storm, and king danger", AnchorStrength: 0, LearningRateScale: 1},
		{ID: groupKingPasser, Description: "King proximity to passed pawns", AnchorStrength: 0, LearningRateScale: 1},
		{ID: groupSpace, Description: "Space evaluation", AnchorStrength: 0, LearningRateScale: 1},
		{ID: groupImbalance, Description: "Material imbalance", AnchorStrength: 0, LearningRateScale: 1},
		{ID: groupTempo, Description: "Side-to-move tempo", AnchorStrength: 0, LearningRateScale: 1},
		{ID: groupFinalScale, Description: "Final-score scaling", AnchorStrength: 0, LearningRateScale: 1},
	}
}

type engineRegistryBuilder struct {
	specs []ParameterSpec
}

func (b *engineRegistryBuilder) addMaterial() {
	mg, eg := eng.CurrentTuningPieceValues()
	for _, piece := range activePieces()[:5] {
		b.scalar(
			"material."+piece.id+".mg",
			"pieceValueMG["+piece.engineLabel+"]",
			groupMaterial,
			PhaseMG,
			FormulaLinear,
			RoleCoefficient,
			mg[piece.kind],
			TrainingContinuous,
			arrayElementExport("pieceValueMG", EngineInt, []int{7}, int(piece.kind)),
		)
		b.scalar(
			"material."+piece.id+".eg",
			"pieceValueEG["+piece.engineLabel+"]",
			groupMaterial,
			PhaseEG,
			FormulaLinear,
			RoleCoefficient,
			eg[piece.kind],
			TrainingContinuous,
			arrayElementExport("pieceValueEG", EngineInt, []int{7}, int(piece.kind)),
		)
	}
}

func (b *engineRegistryBuilder) addPieceSquareTables() {
	for _, phase := range []struct {
		id     string
		kind   PhaseKind
		symbol string
		values [7][64]int
	}{
		{id: "mg", kind: PhaseMG, symbol: "PSQT_MG", values: eng.PSQT_MG},
		{id: "eg", kind: PhaseEG, symbol: "PSQT_EG", values: eng.PSQT_EG},
	} {
		for _, piece := range activePieces() {
			id := "psqt." + piece.id + "." + phase.id
			engineName := phase.symbol + "[" + piece.engineLabel + "]"
			if piece.kind == gm.PieceTypePawn {
				values := rectangularValues(phase.values[piece.kind][:], 8, 56)
				b.table(
					id,
					engineName,
					groupPSQT,
					phase.kind,
					FormulaLinear,
					RoleCoefficient,
					values,
					TrainingContinuous,
					TableShape(
						AxisSpec{Name: "rank", Labels: numberLabels(2, 7)},
						AxisSpec{Name: "file", Labels: fileLabels()},
					),
					ExportSpec{
						GoSymbol:          phase.symbol,
						GoType:            EngineInt,
						Rounding:          RoundNearest,
						StorageDimensions: []int{7, 64},
						StorageOffset:     int(piece.kind)*64 + 8,
						StorageStrides:    []int{8, 1},
					},
				)
				continue
			}
			b.table(
				id,
				engineName,
				groupPSQT,
				phase.kind,
				FormulaLinear,
				RoleCoefficient,
				intSlice(phase.values[piece.kind][:]),
				TrainingContinuous,
				TableShape(AxisSpec{Name: "square", Labels: squareLabels()}),
				ExportSpec{
					GoSymbol:          phase.symbol,
					GoType:            EngineInt,
					Rounding:          RoundNearest,
					StorageDimensions: []int{7, 64},
					StorageOffset:     int(piece.kind) * 64,
					StorageStrides:    []int{1},
				},
			)
		}
	}
}

func (b *engineRegistryBuilder) addPassedPawnTables() {
	b.table(
		"pawn.passed.psqt.mg",
		"PassedPawnPSQT_MG",
		groupPSQT,
		PhaseMG,
		FormulaLinear,
		RoleCoefficient,
		rectangularValues(eng.PassedPawnPSQT_MG[:], 8, 56),
		TrainingContinuous,
		TableShape(
			AxisSpec{Name: "relativeRank", Labels: numberLabels(1, 6)},
			AxisSpec{Name: "file", Labels: fileLabels()},
		),
		subshapeExport("PassedPawnPSQT_MG", EngineInt, []int{64}, 8, []int{8, 1}),
	)
	b.table(
		"pawn.passed.psqt.eg",
		"PassedPawnPSQT_EG",
		groupPSQT,
		PhaseEG,
		FormulaLinear,
		RoleCoefficient,
		rectangularValues(eng.PassedPawnPSQT_EG[:], 8, 56),
		TrainingContinuous,
		TableShape(
			AxisSpec{Name: "relativeRank", Labels: numberLabels(1, 6)},
			AxisSpec{Name: "file", Labels: fileLabels()},
		),
		subshapeExport("PassedPawnPSQT_EG", EngineInt, []int{64}, 8, []int{8, 1}),
	)
}

func (b *engineRegistryBuilder) addMobility() {
	b.phaseTablePair("mobility.knight", "KnightMobility", groupMobility, FormulaCenterScale, RoleCoefficient, eng.KnightMobilityMG[:], eng.KnightMobilityEG[:], "mobility")
	b.phaseTablePair("mobility.bishop", "BishopMobility", groupMobility, FormulaCenterScale, RoleCoefficient, eng.BishopMobilityMG[:], eng.BishopMobilityEG[:], "mobility")
	b.phaseTablePair("mobility.rook", "RookMobility", groupMobility, FormulaLinear, RoleCoefficient, eng.RookMobilityMG[:], eng.RookMobilityEG[:], "mobility")
	b.phaseTablePair("mobility.queen", "QueenMobility", groupMobility, FormulaLinear, RoleCoefficient, eng.QueenMobilityMG[:], eng.QueenMobilityEG[:], "mobility")
}

func (b *engineRegistryBuilder) addPawnStructure() {
	b.phasePair("pawn.backward.opposed", "BackwardOpposed", groupPawnStructure, FormulaLinear, RoleCoefficient, eng.BackwardOpposedMG, eng.BackwardOpposedEG)
	b.phasePair("pawn.backward.unopposed", "BackwardUnopposed", groupPawnStructure, FormulaLinear, RoleCoefficient, eng.BackwardUnopposedMG, eng.BackwardUnopposedEG)
	b.phasePair("pawn.isolated.opposed", "IsolatedOpposed", groupPawnStructure, FormulaLinear, RoleCoefficient, eng.IsolatedOpposedMG, eng.IsolatedOpposedEG)
	b.phasePair("pawn.isolated.unopposed", "IsolatedUnopposed", groupPawnStructure, FormulaLinear, RoleCoefficient, eng.IsolatedUnopposedMG, eng.IsolatedUnopposedEG)
	b.phasePair("pawn.doubled.opposed", "PawnDoubledOpposed", groupPawnStructure, FormulaLinear, RoleCoefficient, eng.PawnDoubledOpposedMG, eng.PawnDoubledOpposedEG)
	b.phasePair("pawn.doubled.unopposed", "PawnDoubledUnopposed", groupPawnStructure, FormulaLinear, RoleCoefficient, eng.PawnDoubledUnopposedMG, eng.PawnDoubledUnopposedEG)
	b.phasePair("pawn.weak_lever", "PawnWeakLever", groupPawnStructure, FormulaLinear, RoleCoefficient, eng.PawnWeakLeverMG, eng.PawnWeakLeverEG)
	b.phaseArrayPair("pawn.blocked", "PawnBlocked", groupPawnStructure, FormulaLinear, RoleCoefficient, eng.PawnBlockedMG[:], eng.PawnBlockedEG[:])

	b.table(
		"pawn.connected.mg",
		"PawnConnectedMG[1:7]",
		groupPawnStructure,
		PhaseMG,
		FormulaConnectedPawn,
		RoleCoefficient,
		intSlice(eng.PawnConnectedMG[1:]),
		TrainingContinuous,
		TableShape(AxisSpec{Name: "relativeRank", Labels: numberLabels(1, 6)}),
		subshapeExport("PawnConnectedMG", EngineInt, []int{7}, 1, []int{1}),
	)
	b.scalar("pawn.candidate_passed_pct.mg", "CandidatePassedPctMG", groupPawnStructure, PhaseMG, FormulaCandidatePasser, RolePercentage, eng.CandidatePassedPctMG, TrainingContinuous, scalarExport("CandidatePassedPctMG", EngineInt))
	b.scalar("pawn.candidate_passed_pct.eg", "CandidatePassedPctEG", groupPawnStructure, PhaseEG, FormulaCandidatePasser, RolePercentage, eng.CandidatePassedPctEG, TrainingContinuous, scalarExport("CandidatePassedPctEG", EngineInt))
}

func (b *engineRegistryBuilder) addCenterScaling() {
	b.phasePair("center.knight_mobility_pct", "CenterKnightMobility", groupCenter, FormulaCenterScale, RolePercentage, eng.CenterKnightMobilityMG, eng.CenterKnightMobilityEG)
	b.phasePair("center.bishop_mobility_pct", "CenterBishopMobility", groupCenter, FormulaCenterScale, RolePercentage, eng.CenterBishopMobilityMG, eng.CenterBishopMobilityEG)
	b.phasePair("center.bishop_pair_pct", "CenterBishopPair", groupCenter, FormulaCenterScale, RolePercentage, eng.CenterBishopPairMG, eng.CenterBishopPairEG)
}

func (b *engineRegistryBuilder) addPieceActivity() {
	b.phasePair("knight.outpost", "KnightOutpost", groupPieceActivity, FormulaLinear, RoleCoefficient, eng.KnightOutpostMG, eng.KnightOutpostEG)
	b.phasePair("knight.tropism", "KnightTropism", groupPieceActivity, FormulaLinear, RoleCoefficient, eng.KnightTropismMG, eng.KnightTropismEG)
	b.phasePair("bishop.outpost", "BishopOutpost", groupPieceActivity, FormulaLinear, RoleCoefficient, eng.BishopOutpostMG, eng.BishopOutpostEG)
	b.phasePair("bishop.bad", "BadBishop", groupPieceActivity, FormulaLinear, RoleCoefficient, eng.BadBishopMG, eng.BadBishopEG)
	b.phasePair("bishop.pair", "BishopPairBonus", groupPieceActivity, FormulaCenterScale, RoleCoefficient, eng.BishopPairBonusMG, eng.BishopPairBonusEG)
}

func (b *engineRegistryBuilder) addRooksAndQueen() {
	b.phasePair("rook.semi_open", "RookSemiOpen", groupRook, FormulaLinear, RoleCoefficient, eng.RookSemiOpenMG, eng.RookSemiOpenEG)
	b.phasePair("rook.open", "RookOpen", groupRook, FormulaLinear, RoleCoefficient, eng.RookOpenMG, eng.RookOpenEG)
	b.phasePair("rook.file_count.open", "RookFileCountOpen", groupRook, FormulaLinear, RoleCoefficient, eng.RookFileCountOpenMG, eng.RookFileCountOpenEG)
	b.phasePair("rook.file_count.semi_open", "RookFileCountSemi", groupRook, FormulaLinear, RoleCoefficient, eng.RookFileCountSemiMG, eng.RookFileCountSemiEG)
	b.phasePair("rook.seventh_rank", "RookSeventhRank", groupRook, FormulaLinear, RoleCoefficient, eng.RookSeventhRankMG, eng.RookSeventhRankEG)
	b.scalar("rook.stacked.mg", "RookStackedMG", groupRook, PhaseMG, FormulaLinear, RoleCoefficient, eng.RookStackedMG, TrainingContinuous, scalarExport("RookStackedMG", EngineInt))
	b.scalar("queen.centralization.eg", "QueenCentralizationEG", groupRook, PhaseEG, FormulaLinear, RoleCoefficient, eng.QueenCentralizationEG, TrainingContinuous, scalarExport("QueenCentralizationEG", EngineInt))
}

func (b *engineRegistryBuilder) addShelterAndStorm() {
	b.table(
		"king.shelter.mg",
		"KingShelterMG",
		groupKingSafety,
		PhaseMG,
		FormulaLinear,
		RoleCoefficient,
		activeColumns4x8(eng.KingShelterMG, 7),
		TrainingContinuous,
		TableShape(
			AxisSpec{Name: "edgeDistance", Labels: numberLabels(0, 3)},
			AxisSpec{Name: "relativeRank", Labels: append([]string{"none"}, numberLabels(1, 6)...)},
		),
		subshapeExport("KingShelterMG", EngineInt, []int{4, 8}, 0, []int{8, 1}),
	)
	b.table(
		"king.storm.unblocked.mg",
		"KingStormUnblockedMG",
		groupKingSafety,
		PhaseMG,
		FormulaLinear,
		RoleCoefficient,
		flatten4x8(eng.KingStormUnblockedMG),
		TrainingContinuous,
		TableShape(
			AxisSpec{Name: "edgeDistance", Labels: numberLabels(0, 3)},
			AxisSpec{Name: "relativeRank", Labels: append([]string{"none"}, numberLabels(1, 7)...)},
		),
		tableExport("KingStormUnblockedMG", EngineInt, []int{4, 8}),
	)
	b.table(
		"king.storm.blocked.mg",
		"KingStormBlockedMG[2:8]",
		groupKingSafety,
		PhaseMG,
		FormulaLinear,
		RoleCoefficient,
		intSlice(eng.KingStormBlockedMG[2:]),
		TrainingContinuous,
		TableShape(AxisSpec{Name: "relativeRank", Labels: numberLabels(2, 7)}),
		subshapeExport("KingStormBlockedMG", EngineInt, []int{8}, 2, []int{1}),
	)
	b.table(
		"king.storm.blocked.eg",
		"KingStormBlockedEG[2:8]",
		groupKingSafety,
		PhaseEG,
		FormulaLinear,
		RoleCoefficient,
		intSlice(eng.KingStormBlockedEG[2:]),
		TrainingContinuous,
		TableShape(AxisSpec{Name: "relativeRank", Labels: numberLabels(2, 7)}),
		subshapeExport("KingStormBlockedEG", EngineInt, []int{8}, 2, []int{1}),
	)
	b.scalar("king.minor_defense.mg", "KingMinorDefenseBonusMG", groupKingSafety, PhaseMG, FormulaLinear, RoleCoefficient, eng.KingMinorDefenseBonusMG, TrainingContinuous, scalarExport("KingMinorDefenseBonusMG", EngineInt))
}

func (b *engineRegistryBuilder) addKingDanger() {
	b.phasePair("king.danger.attacker.knight", "SafetyKnightWeight", groupKingSafety, FormulaKingDanger, RoleCoefficient, eng.SafetyKnightWeightMG, eng.SafetyKnightWeightEG)
	b.phasePair("king.danger.attacker.bishop", "SafetyBishopWeight", groupKingSafety, FormulaKingDanger, RoleCoefficient, eng.SafetyBishopWeightMG, eng.SafetyBishopWeightEG)
	b.phasePair("king.danger.attacker.rook", "SafetyRookWeight", groupKingSafety, FormulaKingDanger, RoleCoefficient, eng.SafetyRookWeightMG, eng.SafetyRookWeightEG)
	b.phasePair("king.danger.attacker.queen", "SafetyQueenWeight", groupKingSafety, FormulaKingDanger, RoleCoefficient, eng.SafetyQueenWeightMG, eng.SafetyQueenWeightEG)
	b.phasePair("king.danger.ring_attack", "SafetyAttackValue", groupKingSafety, FormulaKingDanger, RoleCoefficient, eng.SafetyAttackValueMG, eng.SafetyAttackValueEG)
	b.phasePair("king.danger.no_enemy_queen", "SafetyNoEnemyQueens", groupKingSafety, FormulaKingDanger, RoleOffset, eng.SafetyNoEnemyQueensMG, eng.SafetyNoEnemyQueensEG)
	b.phasePair("king.danger.safe_check.knight", "SafetySafeKnightCheck", groupKingSafety, FormulaKingDanger, RoleCoefficient, eng.SafetySafeKnightCheckMG, eng.SafetySafeKnightCheckEG)
	b.phasePair("king.danger.safe_check.bishop", "SafetySafeBishopCheck", groupKingSafety, FormulaKingDanger, RoleCoefficient, eng.SafetySafeBishopCheckMG, eng.SafetySafeBishopCheckEG)
	b.phasePair("king.danger.safe_check.rook", "SafetySafeRookCheck", groupKingSafety, FormulaKingDanger, RoleCoefficient, eng.SafetySafeRookCheckMG, eng.SafetySafeRookCheckEG)
	b.phasePair("king.danger.safe_check.queen", "SafetySafeQueenCheck", groupKingSafety, FormulaKingDanger, RoleCoefficient, eng.SafetySafeQueenCheckMG, eng.SafetySafeQueenCheckEG)
	b.phasePair("king.danger.unsafe_check", "SafetyUnsafeCheck", groupKingSafety, FormulaKingDanger, RoleCoefficient, eng.SafetyUnsafeCheckMG, eng.SafetyUnsafeCheckEG)
	b.phasePair("king.danger.adjustment", "SafetyAdjustment", groupKingSafety, FormulaKingDanger, RoleOffset, eng.SafetyAdjustmentMG, eng.SafetyAdjustmentEG)
	b.scalar("king.danger.divisor.mg", "SafetyMGDivisor", groupKingSafety, PhaseMG, FormulaKingDanger, RoleDivisor, eng.SafetyMGDivisor, TrainingFrozen, positiveScalarExport("SafetyMGDivisor", EngineInt))
	b.scalar("king.danger.divisor.eg", "SafetyEGDivisor", groupKingSafety, PhaseEG, FormulaKingDanger, RoleDivisor, eng.SafetyEGDivisor, TrainingFrozen, positiveScalarExport("SafetyEGDivisor", EngineInt))
}

func (b *engineRegistryBuilder) addKingPasser() {
	b.scalar("king.passer.proximity.eg", "KingPasserProximityEG", groupKingPasser, PhaseEG, FormulaKingPasser, RoleCoefficient, eng.KingPasserProximityEG, TrainingContinuous, scalarExport("KingPasserProximityEG", EngineInt))
	b.scalar("king.passer.divisor", "KingPasserProximityDiv", groupKingPasser, PhaseShared, FormulaKingPasser, RoleDivisor, eng.KingPasserProximityDiv, TrainingFrozen, positiveScalarExport("KingPasserProximityDiv", EngineInt))
	b.scalar("king.passer.enemy_weight", "KingPasserEnemyWeight", groupKingPasser, PhaseShared, FormulaKingPasser, RoleCoefficient, eng.KingPasserEnemyWeight, TrainingContinuous, scalarExport("KingPasserEnemyWeight", EngineInt))
	b.scalar("king.passer.own_weight", "KingPasserOwnWeight", groupKingPasser, PhaseShared, FormulaKingPasser, RoleCoefficient, eng.KingPasserOwnWeight, TrainingContinuous, scalarExport("KingPasserOwnWeight", EngineInt))
}

func (b *engineRegistryBuilder) addSpace() {
	b.scalar("space.safe.mg", "SpaceSafeMG", groupSpace, PhaseMG, FormulaSpace, RoleCoefficient, eng.SpaceSafeMG, TrainingContinuous, scalarExport("SpaceSafeMG", EngineInt))
	b.scalar("space.behind_pawn.mg", "SpaceBehindPawnMG", groupSpace, PhaseMG, FormulaSpace, RoleCoefficient, eng.SpaceBehindPawnMG, TrainingContinuous, scalarExport("SpaceBehindPawnMG", EngineInt))
	b.scalar("space.semi_open.mg", "SpaceSemiOpenMG", groupSpace, PhaseMG, FormulaSpace, RoleCoefficient, eng.SpaceSemiOpenMG, TrainingContinuous, scalarExport("SpaceSemiOpenMG", EngineInt))
	b.scalar("space.open.mg", "SpaceOpenMG", groupSpace, PhaseMG, FormulaSpace, RoleCoefficient, eng.SpaceOpenMG, TrainingContinuous, scalarExport("SpaceOpenMG", EngineInt))
	b.scalar("space.weight_offset", "SpaceWeightOffset", groupSpace, PhaseShared, FormulaSpace, RoleOffset, eng.SpaceWeightOffset, TrainingContinuous, scalarExport("SpaceWeightOffset", EngineInt))
	b.scalar("space.blocked_cap", "SpaceBlockedCap", groupSpace, PhaseShared, FormulaSpace, RoleCap, eng.SpaceBlockedCap, TrainingFrozen, nonNegativeScalarExport("SpaceBlockedCap", EngineInt))
	b.scalar("space.weight_divisor", "SpaceWeightDiv", groupSpace, PhaseShared, FormulaSpace, RoleDivisor, eng.SpaceWeightDiv, TrainingFrozen, positiveScalarExport("SpaceWeightDiv", EngineInt))
}

func (b *engineRegistryBuilder) addImbalance() {
	b.scalarWithBounds("imbalance.reference_pawn_count", "ImbalanceRefPawnCount", groupImbalance, PhaseShared, FormulaImbalance, RoleReference, eng.ImbalanceRefPawnCount, TrainingFrozen, ClosedBounds(0, 16), scalarExport("ImbalanceRefPawnCount", EngineInt))
	b.scalar("imbalance.knight_per_pawn.mg", "ImbalanceKnightPerPawnMG", groupImbalance, PhaseMG, FormulaImbalance, RoleCoefficient, eng.ImbalanceKnightPerPawnMG, TrainingContinuous, scalarExport("ImbalanceKnightPerPawnMG", EngineInt))
	b.scalar("imbalance.knight_per_pawn.eg", "ImbalanceKnightPerPawnEG", groupImbalance, PhaseEG, FormulaImbalance, RoleCoefficient, eng.ImbalanceKnightPerPawnEG, TrainingContinuous, scalarExport("ImbalanceKnightPerPawnEG", EngineInt))
}

func (b *engineRegistryBuilder) phasePair(id, engineBase string, group GroupID, formula FormulaKind, role ParameterRole, mg, eg int) {
	b.scalar(id+".mg", engineBase+"MG", group, PhaseMG, formula, role, mg, TrainingContinuous, scalarExport(engineBase+"MG", EngineInt))
	b.scalar(id+".eg", engineBase+"EG", group, PhaseEG, formula, role, eg, TrainingContinuous, scalarExport(engineBase+"EG", EngineInt))
}

func (b *engineRegistryBuilder) phaseTablePair(id, engineBase string, group GroupID, formula FormulaKind, role ParameterRole, mg, eg []int, axisName string) {
	if len(mg) != len(eg) {
		panic(fmt.Sprintf("%s phase table lengths differ: %d != %d", engineBase, len(mg), len(eg)))
	}
	shape := TableShape(AxisSpec{Name: axisName, Labels: numberLabels(0, len(mg)-1)})
	b.table(id+".mg", engineBase+"MG", group, PhaseMG, formula, role, intSlice(mg), TrainingContinuous, shape, tableExport(engineBase+"MG", EngineInt, []int{len(mg)}))
	b.table(id+".eg", engineBase+"EG", group, PhaseEG, formula, role, intSlice(eg), TrainingContinuous, shape, tableExport(engineBase+"EG", EngineInt, []int{len(eg)}))
}

func (b *engineRegistryBuilder) phaseArrayPair(id, engineBase string, group GroupID, formula FormulaKind, role ParameterRole, mg, eg []int) {
	b.phaseTablePair(id, engineBase, group, formula, role, mg, eg, "relativeRank")
}

func (b *engineRegistryBuilder) scalar(id string, engineName string, group GroupID, phase PhaseKind, formula FormulaKind, role ParameterRole, value int, mode TrainingMode, export ExportSpec) {
	b.scalarWithBounds(id, engineName, group, phase, formula, role, value, mode, defaultBounds(role), export)
}

func (b *engineRegistryBuilder) scalarWithBounds(id string, engineName string, group GroupID, phase PhaseKind, formula FormulaKind, role ParameterRole, value int, mode TrainingMode, bounds Bounds, export ExportSpec) {
	values := []int{value}
	b.specs = append(b.specs, ParameterSpec{
		ID:         ParameterID(id),
		EngineName: engineName,
		Group:      group,
		Shape:      ScalarShape(),
		Phase:      phase,
		Formula:    formula,
		Role:       role,
		Initial:    intValues(values),
		Prior:      provisionalPrior(role, values),
		Training: TrainingSpec{
			Mode:              mode,
			LearningRateScale: 1,
			Bounds:            bounds,
		},
		Export: export,
	})
}

func (b *engineRegistryBuilder) table(id string, engineName string, group GroupID, phase PhaseKind, formula FormulaKind, role ParameterRole, values []int, mode TrainingMode, shape Shape, export ExportSpec) {
	b.specs = append(b.specs, ParameterSpec{
		ID:         ParameterID(id),
		EngineName: engineName,
		Group:      group,
		Shape:      shape,
		Phase:      phase,
		Formula:    formula,
		Role:       role,
		Initial:    intValues(values),
		Prior:      provisionalPrior(role, values),
		Training: TrainingSpec{
			Mode:              mode,
			LearningRateScale: 1,
			Bounds:            defaultBounds(role),
		},
		Export: export,
	})
}

type activePiece struct {
	id          string
	engineLabel string
	kind        gm.PieceType
}

func activePieces() []activePiece {
	return []activePiece{
		{id: "pawn", engineLabel: "Pawn", kind: gm.PieceTypePawn},
		{id: "knight", engineLabel: "Knight", kind: gm.PieceTypeKnight},
		{id: "bishop", engineLabel: "Bishop", kind: gm.PieceTypeBishop},
		{id: "rook", engineLabel: "Rook", kind: gm.PieceTypeRook},
		{id: "queen", engineLabel: "Queen", kind: gm.PieceTypeQueen},
		{id: "king", engineLabel: "King", kind: gm.PieceTypeKing},
	}
}

func provisionalPrior(role ParameterRole, values []int) PriorSpec {
	scales := make([]float64, len(values))
	for i, value := range values {
		absolute := math.Abs(float64(value))
		switch role {
		case RolePercentage:
			scales[i] = math.Max(5, absolute*0.25)
		case RoleCap, RoleReference:
			scales[i] = math.Max(1, absolute*0.1)
		default:
			scales[i] = math.Max(1, absolute*0.25)
		}
	}
	return PriorSpec{
		Anchor:         intValues(values),
		DeviationScale: ElementValues(scales...),
		StrengthScale:  1,
	}
}

func defaultBounds(role ParameterRole) Bounds {
	switch role {
	case RoleDivisor, RoleFinalDivider:
		return Bounds{Lower: Limit{Value: 1, Set: true}}
	case RoleCap:
		return Bounds{Lower: Limit{Value: 0, Set: true}}
	default:
		return Bounds{}
	}
}

func scalarExport(symbol string, valueType EngineValueType) ExportSpec {
	return ExportSpec{GoSymbol: symbol, GoType: valueType, Rounding: RoundNearest}
}

func positiveScalarExport(symbol string, valueType EngineValueType) ExportSpec {
	return scalarExport(symbol, valueType)
}

func nonNegativeScalarExport(symbol string, valueType EngineValueType) ExportSpec {
	return scalarExport(symbol, valueType)
}

func arrayElementExport(symbol string, valueType EngineValueType, dimensions []int, offset int) ExportSpec {
	return ExportSpec{
		GoSymbol:          symbol,
		GoType:            valueType,
		Rounding:          RoundNearest,
		StorageDimensions: append([]int(nil), dimensions...),
		StorageOffset:     offset,
	}
}

func tableExport(symbol string, valueType EngineValueType, dimensions []int) ExportSpec {
	return ExportSpec{
		GoSymbol:          symbol,
		GoType:            valueType,
		Rounding:          RoundNearest,
		StorageDimensions: append([]int(nil), dimensions...),
	}
}

func subshapeExport(symbol string, valueType EngineValueType, dimensions []int, offset int, strides []int) ExportSpec {
	return ExportSpec{
		GoSymbol:          symbol,
		GoType:            valueType,
		Rounding:          RoundNearest,
		StorageDimensions: append([]int(nil), dimensions...),
		StorageOffset:     offset,
		StorageStrides:    append([]int(nil), strides...),
	}
}

func intValues(values []int) ValueSpec {
	floats := make([]float64, len(values))
	for i, value := range values {
		floats[i] = float64(value)
	}
	return ElementValues(floats...)
}

func intSlice(values []int) []int {
	return append([]int(nil), values...)
}

func rectangularValues(values []int, start, end int) []int {
	return append([]int(nil), values[start:end]...)
}

func flatten4x8(values [4][8]int) []int {
	out := make([]int, 0, 32)
	for i := range values {
		out = append(out, values[i][:]...)
	}
	return out
}

func activeColumns4x8(values [4][8]int, columns int) []int {
	out := make([]int, 0, 4*columns)
	for i := range values {
		out = append(out, values[i][:columns]...)
	}
	return out
}

func numberLabels(first, last int) []string {
	labels := make([]string, 0, max(0, last-first+1))
	for value := first; value <= last; value++ {
		labels = append(labels, strconv.Itoa(value))
	}
	return labels
}

func fileLabels() []string {
	return []string{"a", "b", "c", "d", "e", "f", "g", "h"}
}

func squareLabels() []string {
	labels := make([]string, 0, 64)
	for rank := 1; rank <= 8; rank++ {
		for file := 'a'; file <= 'h'; file++ {
			labels = append(labels, string(file)+strconv.Itoa(rank))
		}
	}
	return labels
}
