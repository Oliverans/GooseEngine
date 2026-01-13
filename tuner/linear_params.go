package tuner

// Params returns the flattened parameter vector θ using the consolidated layout.
// Layout order (see phase_offsets.go for offsets):
//   - Tier 1: PST MG/EG, Material MG/EG, Mobility MG/EG, Core scalars, Tier1 extras
//   - Tier 2: Passers MG/EG, PawnStruct
//   - Tier 3: King table, correlates, endgame, Tier3 extras, WeakKingSquares
//   - Tier 4: BishopPair, Imbalance, Space/Tempo
func (le *LinearEval) Params() []float64 {
	if le == nil {
		return nil
	}
	le.ensureLayout()
	if le.theta == nil || len(le.theta) != le.layout.Total {
		le.theta = make([]float64, le.layout.Total)
	}
	if le.PST != nil {
		off := 0
		off = le.writePSTToTheta(off)
		off = le.writeMaterialToTheta(off)
		off = le.writeMobilityToTheta(off)
		off = le.writeCoreScalarsToTheta(off)
		off = le.writeTier1ExtrasToTheta(off)
		off = le.writePassersToTheta(off)
		off = le.writePawnStructToTheta(off)
		off = le.writeKingTableToTheta(off)
		off = le.writeKingCorrToTheta(off)
		off = le.writeKingEndgameToTheta(off)
		off = le.writeTier3ExtrasToTheta(off)
		off = le.writeWeakKingToTheta(off)
		off = le.writeBishopPairToTheta(off)
		off = le.writeImbalanceToTheta(off)
		off = le.writeSpaceTempoToTheta(off)
		_ = off
	}
	return le.theta
}

// SetParams replaces θ, and updates backing PST plus derived feature weights.
func (le *LinearEval) SetParams(p []float64) {
	if le == nil {
		return
	}
	le.ensureLayout()
	want := le.layout.Total
	if le.theta == nil || len(le.theta) != want {
		le.theta = make([]float64, want)
	}
	n := len(p)
	if n > want {
		n = want
	}
	copy(le.theta[:n], p[:n])
	for i := n; i < want; i++ {
		le.theta[i] = 0
	}
	if le.PST != nil {
		off := 0
		off = le.readPSTFromTheta(off)
		off = le.readMaterialFromTheta(off)
		off = le.readMobilityFromTheta(off)
		off = le.readCoreScalarsFromTheta(off)
		off = le.readTier1ExtrasFromTheta(off)
		off = le.readPassersFromTheta(off)
		off = le.readPawnStructFromTheta(off)
		off = le.readKingTableFromTheta(off)
		off = le.readKingCorrFromTheta(off)
		off = le.readKingEndgameFromTheta(off)
		off = le.readTier3ExtrasFromTheta(off)
		off = le.readWeakKingFromTheta(off)
		off = le.readBishopPairFromTheta(off)
		off = le.readImbalanceFromTheta(off)
		off = le.readSpaceTempoFromTheta(off)
		_ = off
	}
}
