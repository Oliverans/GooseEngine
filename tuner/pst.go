// tuner/pst.go
package tuner

// tapered eval: E = mgf*MG + egf*EG
func taperedPhases(s Sample) (mgf, egf float64) {
	curr := TotalPhase - s.PiecePhase
	mgf = 1.0 - float64(curr)/24.0
	if mgf < 0 { mgf = 0 }
	if mgf > 1 { mgf = 1 }
	egf = float64(curr) / 24.0
	if egf < 0 { egf = 0 }
	if egf > 1 { egf = 1 }
	return
}

// white-positive
func evalPST(pst *PST, s Sample) float64 {
	mgf, egf := taperedPhases(s)
	mg, eg := 0.0, 0.0
	for pt := 0; pt < 6; pt++ {
		for _, sq := range s.Pieces[pt] {
			mg += pst.MG[pt][sq]
			eg += pst.EG[pt][sq]
		}
	}
	for pt := 0; pt < 6; pt++ {
		for _, sq := range s.BP[pt] {
			rev := flipView[sq]
			mg -= pst.MG[pt][rev]
			eg -= pst.EG[pt][rev]
		}
	}
	return mgf*mg + egf*eg
}

// accumulate gradient dE/dθ for PST slots
func addEvalGrad(pst *PST, s Sample, gMG *[6][64]float64, gEG *[6][64]float64, scale float64) {
	mgf, egf := taperedPhases(s)
	for pt := 0; pt < 6; pt++ {
		for _, sq := range s.Pieces[pt] {
			gMG[pt][sq] += scale * mgf
			gEG[pt][sq] += scale * egf
		}
		for _, sq := range s.BP[pt] {
			rev := flipView[sq]
			gMG[pt][rev] -= scale * mgf
			gEG[pt][rev] -= scale * egf
		}
	}
}
