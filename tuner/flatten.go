// tuner/flatten.go
package tuner

func flatten(pst *PST) ([]float64, func([]float64)) {
	total := 6*64*2
	buf := make([]float64, 0, total)
	var slots []*float64
	for pt := 0; pt < 6; pt++ {
		for sq := 0; sq < 64; sq++ { slots = append(slots, &pst.MG[pt][sq]) }
	}
	for pt := 0; pt < 6; pt++ {
		for sq := 0; sq < 64; sq++ { slots = append(slots, &pst.EG[pt][sq]) }
	}
	for _, p := range slots { buf = append(buf, *p) }
	restore := func(vals []float64) {
		i := 0
		for pt := 0; pt < 6; pt++ {
			for sq := 0; sq < 64; sq++ { pst.MG[pt][sq] = vals[i]; i++ }
		}
		for pt := 0; pt < 6; pt++ {
			for sq := 0; sq < 64; sq++ { pst.EG[pt][sq] = vals[i]; i++ }
		}
	}
	return buf, restore
}

func flattenGrads(bg *BatchGrad) []float64 {
	out := make([]float64, 6*64*2)
	i := 0
	for pt := 0; pt < 6; pt++ {
		for sq := 0; sq < 64; sq++ { out[i] = bg.MG[pt][sq]; i++ }
	}
	for pt := 0; pt < 6; pt++ {
		for sq := 0; sq < 64; sq++ { out[i] = bg.EG[pt][sq]; i++ }
	}
	return out
}
