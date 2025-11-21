package tuner

// Params returns the flattened parameter vector θ using the consolidated layout.
// Layout order (see phase_offsets.go for offsets):
//   - PST MG/EG blocks (6x64 each)
//   - Material MG/EG (6 each)
//   - Passed pawn MG/EG (64 each)
//   - Phase 1 scalars (8)
//   - Pawn structure scalars (16)
//   - Mobility MG/EG (7 each)
//   - King safety table (100) and correlates (4)
//   - Extras (16) and material imbalance scalars (12)
//   - Weak squares + tempo (3)
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
		off = le.writePassersToTheta(off)
		off = le.writeP1ScalarsToTheta(off)
		off = le.writePawnStructToTheta(off)
		off = le.writeMobilityToTheta(off)
		off = le.writeKingTableToTheta(off)
		off = le.writeKingCorrToTheta(off)
		off = le.writeExtrasToTheta(off)
		off = le.writeImbalanceToTheta(off)
		off = le.writeWeakTempoToTheta(off)
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
		off = le.readPassersFromTheta(off)
		off = le.readP1ScalarsFromTheta(off)
		off = le.readPawnStructFromTheta(off)
		off = le.readMobilityFromTheta(off)
		off = le.readKingTableFromTheta(off)
		off = le.readKingCorrFromTheta(off)
		off = le.readExtrasFromTheta(off)
		off = le.readImbalanceFromTheta(off)
		off = le.readWeakTempoFromTheta(off)
		_ = off
	}
}
