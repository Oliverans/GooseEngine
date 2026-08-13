package tuner

// engineTrainingPolicy is the editable optimizer-ownership policy for the
// current engine registry. Structural declarations, trace formulas and export
// layouts remain in engine_registry.go.
//
// Example: freeze only connected pawns on relative rank 1:
//
//	Freeze("pawn.connected.mg", At("relativeRank", "1")),
//
// With no At selections, Freeze applies to the entire parameter.
//
// Example: train only the structurally eligible material, PSQT and mobility
// coordinates while retaining every other engine value in forward evaluation:
//
//	Default: FreezeEligibleParameters,
//	Groups: []GroupTrainingOverride{
//		TrainGroup(groupMaterial),
//		TrainGroup(groupPSQT),
//		TrainGroup(groupMobility),
//	},
func engineTrainingPolicy() TrainingPolicy {
	return TrainingPolicy{
		Default: FreezeEligibleParameters,
		Groups: []GroupTrainingOverride{
			TrainGroup(groupMaterial),
			TrainGroup(groupPSQT),
			TrainGroup(groupMobility),
			TrainGroup(groupPawnStructure),
			TrainGroup(groupKingPasser),
			TrainGroup(groupCenter),
			TrainGroup(groupPieceActivity),
			TrainGroup(groupRook),
			TrainGroup(groupSpace),
			TrainGroup(groupImbalance),
			TrainGroup(groupTempo),
		},
		Overrides: []ParameterTrainingOverride{
			Freeze(
				"pawn.connected.mg",
				At("relativeRank", "1"),
			),
		},
	}
}
