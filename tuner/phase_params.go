package tuner

// The following helpers centralize θ reads/writes per parameter block.
// They use the consolidated Layout offsets from phase_offsets.go.
//
// Tier mapping for each block (see toggles.go for tier definitions):
//   - PST, Material, Mobility: Tier 1 (Core)
//   - Passers, PawnStruct: Tier 2 (Pawns)
//   - P1 Scalars: Tier 1 (RookFiles, SeventhRank, QueenCentralized) + Tier 4 (BishopPair)
//   - KingTable, KingCorr, KingEndgame: Tier 3 (King Safety)
//   - Extras: Tier 1 (Outposts, StackedRooks, MobCenter) + Tier 3 (Tropism, PawnStorm)
//   - Imbalance: Tier 4 (Misc)
//   - WeakTempo: Tier 3 (WeakKingSquares) + Tier 4 (Space, Tempo)

func (le *LinearEval) ensureLayout() {
	if le.layout.Total == 0 {
		le.layout = computeLayout()
	}
}

// ---- Writes (struct -> theta) ----

func (le *LinearEval) writePSTToTheta(off int) int {
	for pt := 0; pt < 6; pt++ {
		copy(le.theta[off:off+64], le.PST.MG[pt][:])
		off += 64
	}
	for pt := 0; pt < 6; pt++ {
		copy(le.theta[off:off+64], le.PST.EG[pt][:])
		off += 64
	}
	return off
}

func (le *LinearEval) writeMaterialToTheta(off int) int {
	for i := 0; i < 6; i++ {
		le.theta[off+i] = le.MatMG[i]
	}
	off += 6
	for i := 0; i < 6; i++ {
		le.theta[off+i] = le.MatEG[i]
	}
	off += 6
	return off
}

func (le *LinearEval) writePassersToTheta(off int) int {
	for i := 0; i < 64; i++ {
		le.theta[off+i] = le.PasserMG[i]
	}
	off += 64
	for i := 0; i < 64; i++ {
		le.theta[off+i] = le.PasserEG[i]
	}
	off += 64
	return off
}

func (le *LinearEval) writeP1ScalarsToTheta(off int) int {
	le.theta[off+0] = le.BishopPairMG
	le.theta[off+1] = le.BishopPairEG
	le.theta[off+2] = le.RookSemiOpenFileMG
	le.theta[off+3] = le.RookOpenFileMG
	le.theta[off+4] = le.SeventhRankEG
	le.theta[off+5] = le.QueenCentralizedEG
	return off + 6
}

func (le *LinearEval) writePawnStructToTheta(off int) int {
	le.theta[off+0] = le.DoubledMG
	le.theta[off+1] = le.DoubledEG
	le.theta[off+2] = le.IsolatedMG
	le.theta[off+3] = le.IsolatedEG
	le.theta[off+4] = le.ConnectedMG
	le.theta[off+5] = le.ConnectedEG
	le.theta[off+6] = le.PhalanxMG
	le.theta[off+7] = le.PhalanxEG
	le.theta[off+8] = le.BlockedMG
	le.theta[off+9] = le.BlockedEG
	le.theta[off+10] = le.WeakLeverMG
	le.theta[off+11] = le.WeakLeverEG
	le.theta[off+12] = le.BackwardMG
	le.theta[off+13] = le.BackwardEG
	le.theta[off+14] = le.CandidatePassedPctMG
	le.theta[off+15] = le.CandidatePassedPctEG
	return off + 16
}

func (le *LinearEval) writeMobilityToTheta(off int) int {
	for i := 0; i < len(le.KnightMobilityMG); i++ {
		le.theta[off+i] = le.KnightMobilityMG[i]
	}
	off += len(le.KnightMobilityMG)
	for i := 0; i < len(le.BishopMobilityMG); i++ {
		le.theta[off+i] = le.BishopMobilityMG[i]
	}
	off += len(le.BishopMobilityMG)
	for i := 0; i < len(le.RookMobilityMG); i++ {
		le.theta[off+i] = le.RookMobilityMG[i]
	}
	off += len(le.RookMobilityMG)
	for i := 0; i < len(le.QueenMobilityMG); i++ {
		le.theta[off+i] = le.QueenMobilityMG[i]
	}
	off += len(le.QueenMobilityMG)
	for i := 0; i < len(le.KnightMobilityEG); i++ {
		le.theta[off+i] = le.KnightMobilityEG[i]
	}
	off += len(le.KnightMobilityEG)
	for i := 0; i < len(le.BishopMobilityEG); i++ {
		le.theta[off+i] = le.BishopMobilityEG[i]
	}
	off += len(le.BishopMobilityEG)
	for i := 0; i < len(le.RookMobilityEG); i++ {
		le.theta[off+i] = le.RookMobilityEG[i]
	}
	off += len(le.RookMobilityEG)
	for i := 0; i < len(le.QueenMobilityEG); i++ {
		le.theta[off+i] = le.QueenMobilityEG[i]
	}
	off += len(le.QueenMobilityEG)
	return off
}

func (le *LinearEval) writeKingTableToTheta(off int) int {
	for i := 0; i < 100; i++ {
		le.theta[off+i] = le.KingSafety[i]
	}
	return off + 100
}

func (le *LinearEval) writeKingCorrToTheta(off int) int {
	le.theta[off+0] = le.KingSemiOpenFilePenalty
	le.theta[off+1] = le.KingOpenFilePenalty
	le.theta[off+2] = le.KingMinorPieceDefense
	le.theta[off+3] = le.KingPawnDefenseMG
	return off + 4
}

func (le *LinearEval) writeKingEndgameToTheta(off int) int {
	le.theta[off+0] = le.KingEndgameCenterEG
	le.theta[off+1] = le.KingMopUpEG
	return off + 2
}

func (le *LinearEval) readKingEndgameFromTheta(off int) int {
	le.KingEndgameCenterEG = le.theta[off+0]
	le.KingMopUpEG = le.theta[off+1]
	return off + 2
}

func (le *LinearEval) writeExtrasToTheta(off int) int {
	le.theta[off+0] = le.KnightOutpostMG
	le.theta[off+1] = le.KnightOutpostEG
	le.theta[off+2] = le.KnightTropismMG
	le.theta[off+3] = le.KnightTropismEG
	le.theta[off+4] = le.StackedRooksMG
	le.theta[off+5] = le.BishopOutpostMG
	le.theta[off+6] = le.BishopOutpostEG
	for i := 0; i < 8; i++ {
		le.theta[off+7+i] = le.PawnStormFreePct[i]
		le.theta[off+15+i] = le.PawnStormLeverPct[i]
		le.theta[off+23+i] = le.PawnStormWeakLeverPct[i]
		le.theta[off+31+i] = le.PawnStormBlockedPct[i]
	}
	le.theta[off+39] = le.PawnStormOppositeMult
	le.theta[off+40] = le.PawnProximityMG
	le.theta[off+41] = le.KnightMobCenterMG
	le.theta[off+42] = le.BishopMobCenterMG
	for i := 0; i < 8; i++ {
		le.theta[off+43+i] = le.PawnStormBaseMG[i]
	}
	le.theta[off+51] = le.BadBishopMG
	le.theta[off+52] = le.BadBishopEG
	return off + 53
}

func (le *LinearEval) writeImbalanceToTheta(off int) int {
	le.theta[off+0] = le.ImbalanceKnightPerPawnMG
	le.theta[off+1] = le.ImbalanceKnightPerPawnEG
	le.theta[off+2] = le.ImbalanceBishopPerPawnMG
	le.theta[off+3] = le.ImbalanceBishopPerPawnEG
	return off + 4
}

func (le *LinearEval) writeWeakTempoToTheta(off int) int {
	le.theta[off+0] = le.SpaceMG
	le.theta[off+1] = le.SpaceEG
	le.theta[off+2] = le.WeakKingSquaresMG
	le.theta[off+3] = le.Tempo
	return off + 4
}

// ---- Reads (theta -> struct) ----

func (le *LinearEval) readPSTFromTheta(off int) int {
	for pt := 0; pt < 6; pt++ {
		copy(le.PST.MG[pt][:], le.theta[off:off+64])
		off += 64
	}
	for pt := 0; pt < 6; pt++ {
		copy(le.PST.EG[pt][:], le.theta[off:off+64])
		off += 64
	}
	return off
}

func (le *LinearEval) readMaterialFromTheta(off int) int {
	for i := 0; i < 6; i++ {
		le.MatMG[i] = le.theta[off+i]
	}
	off += 6
	for i := 0; i < 6; i++ {
		le.MatEG[i] = le.theta[off+i]
	}
	off += 6
	return off
}

func (le *LinearEval) readPassersFromTheta(off int) int {
	for i := 0; i < 64; i++ {
		le.PasserMG[i] = le.theta[off+i]
	}
	off += 64
	for i := 0; i < 64; i++ {
		le.PasserEG[i] = le.theta[off+i]
	}
	off += 64
	return off
}

func (le *LinearEval) readP1ScalarsFromTheta(off int) int {
	le.BishopPairMG = le.theta[off+0]
	le.BishopPairEG = le.theta[off+1]
	le.RookSemiOpenFileMG = le.theta[off+2]
	le.RookOpenFileMG = le.theta[off+3]
	le.SeventhRankEG = le.theta[off+4]
	le.QueenCentralizedEG = le.theta[off+5]
	return off + 6
}

func (le *LinearEval) readPawnStructFromTheta(off int) int {
	le.DoubledMG = le.theta[off+0]
	le.DoubledEG = le.theta[off+1]
	le.IsolatedMG = le.theta[off+2]
	le.IsolatedEG = le.theta[off+3]
	le.ConnectedMG = le.theta[off+4]
	le.ConnectedEG = le.theta[off+5]
	le.PhalanxMG = le.theta[off+6]
	le.PhalanxEG = le.theta[off+7]
	le.BlockedMG = le.theta[off+8]
	le.BlockedEG = le.theta[off+9]
	le.WeakLeverMG = le.theta[off+10]
	le.WeakLeverEG = le.theta[off+11]
	le.BackwardMG = le.theta[off+12]
	le.BackwardEG = le.theta[off+13]
	le.CandidatePassedPctMG = le.theta[off+14]
	le.CandidatePassedPctEG = le.theta[off+15]
	return off + 16
}

func (le *LinearEval) readMobilityFromTheta(off int) int {
	for i := 0; i < len(le.KnightMobilityMG); i++ {
		le.KnightMobilityMG[i] = le.theta[off+i]
	}
	off += len(le.KnightMobilityMG)
	for i := 0; i < len(le.BishopMobilityMG); i++ {
		le.BishopMobilityMG[i] = le.theta[off+i]
	}
	off += len(le.BishopMobilityMG)
	for i := 0; i < len(le.RookMobilityMG); i++ {
		le.RookMobilityMG[i] = le.theta[off+i]
	}
	off += len(le.RookMobilityMG)
	for i := 0; i < len(le.QueenMobilityMG); i++ {
		le.QueenMobilityMG[i] = le.theta[off+i]
	}
	off += len(le.QueenMobilityMG)
	for i := 0; i < len(le.KnightMobilityEG); i++ {
		le.KnightMobilityEG[i] = le.theta[off+i]
	}
	off += len(le.KnightMobilityEG)
	for i := 0; i < len(le.BishopMobilityEG); i++ {
		le.BishopMobilityEG[i] = le.theta[off+i]
	}
	off += len(le.BishopMobilityEG)
	for i := 0; i < len(le.RookMobilityEG); i++ {
		le.RookMobilityEG[i] = le.theta[off+i]
	}
	off += len(le.RookMobilityEG)
	for i := 0; i < len(le.QueenMobilityEG); i++ {
		le.QueenMobilityEG[i] = le.theta[off+i]
	}
	off += len(le.QueenMobilityEG)
	return off
}

func (le *LinearEval) readKingTableFromTheta(off int) int {
	for i := 0; i < 100; i++ {
		le.KingSafety[i] = le.theta[off+i]
	}
	return off + 100
}

func (le *LinearEval) readKingCorrFromTheta(off int) int {
	le.KingSemiOpenFilePenalty = le.theta[off+0]
	le.KingOpenFilePenalty = le.theta[off+1]
	le.KingMinorPieceDefense = le.theta[off+2]
	le.KingPawnDefenseMG = le.theta[off+3]
	return off + 4
}

func (le *LinearEval) readExtrasFromTheta(off int) int {
	le.KnightOutpostMG = le.theta[off+0]
	le.KnightOutpostEG = le.theta[off+1]
	le.KnightTropismMG = le.theta[off+2]
	le.KnightTropismEG = le.theta[off+3]
	le.StackedRooksMG = le.theta[off+4]
	le.BishopOutpostMG = le.theta[off+5]
	le.BishopOutpostEG = le.theta[off+6]
	// Pawn storm percentage arrays (at positions 7-38 within Extras block)
	for i := 0; i < 8; i++ {
		le.PawnStormFreePct[i] = le.theta[off+7+i]
		le.PawnStormLeverPct[i] = le.theta[off+15+i]
		le.PawnStormWeakLeverPct[i] = le.theta[off+23+i]
		le.PawnStormBlockedPct[i] = le.theta[off+31+i]
	}
	le.PawnStormOppositeMult = le.theta[off+39]
	le.PawnProximityMG = le.theta[off+40]
	le.KnightMobCenterMG = le.theta[off+41]
	le.BishopMobCenterMG = le.theta[off+42]
	// Pawn storm base (at positions 43-50 within Extras block)
	for i := 0; i < 8; i++ {
		le.PawnStormBaseMG[i] = le.theta[off+43+i]
	}
	le.BadBishopMG = le.theta[off+51]
	le.BadBishopEG = le.theta[off+52]
	return off + 53
}

func (le *LinearEval) readWeakTempoFromTheta(off int) int {
	le.SpaceMG = le.theta[off+0]
	le.SpaceEG = le.theta[off+1]
	le.WeakKingSquaresMG = le.theta[off+2]
	le.Tempo = le.theta[off+3]
	return off + 4
}

func (le *LinearEval) readImbalanceFromTheta(off int) int {
	le.ImbalanceKnightPerPawnMG = le.theta[off+0]
	le.ImbalanceKnightPerPawnEG = le.theta[off+1]
	le.ImbalanceBishopPerPawnMG = le.theta[off+2]
	le.ImbalanceBishopPerPawnEG = le.theta[off+3]
	return off + 4
}
