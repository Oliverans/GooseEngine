// tuner/loss.go
package tuner

import (
	eng "chess-engine/engine"
	"math"
)

// logistic probability p = 1/(1+exp(-k*E))
func prob(k, eval float64) float64 {
	z := k * eval
	if z > 40 {
		return 1
	}
	if z < -40 {
		return 0
	}
	return 1.0 / (1.0 + math.Exp(-z))
}

// one-dimensional refit for k on a subset
func refitK(fe Featurizer, pst *PST, data []Sample, stmMode bool) {
	k0 := pst.K
	bestK, bestLoss := k0, math.MaxFloat64
	cands := []float64{k0 * 0.5, k0 * 0.67, k0 * 0.8, k0 * 0.9, k0, k0 * 1.1, k0 * 1.25, k0 * 1.5}
	for _, k := range cands {
		sum := 0.0
		for i := range data {
			b := NewBoardFromSample(data[i])
			E := fe.Eval(b)
			if stmMode && data[i].STM == 0 {
				E = -E
			}
			p := prob(k, E)
			d := p - data[i].Label
			sum += d * d
		}
		if sum < bestLoss {
			bestLoss, bestK = sum, k
		}
	}
	pst.K = bestK
}

// BatchGradFlat carries gradients for a flattened θ vector produced by a Featurizer.
type BatchGradFlat struct {
	Grads []float64
	Dk    float64
	Loss  float64
	N     int
}

// batchGradFe uses a Featurizer to compute forward probabilities and accumulate gradients
// w.r.t. the parameter vector θ, while continuing to use pst.K for the logistic link scale.
func batchGradFe(fe Featurizer, pst *PST, batch []Sample, stmMode bool, l2 float64) BatchGradFlat {
	var out BatchGradFlat
	if fe == nil || pst == nil {
		return out
	}
	params := fe.Params() // snapshot and ensure length
	out.Grads = make([]float64, len(params))
	k := pst.K
	for i := range batch {
		s := &batch[i]
		b := NewBoardFromSample(*s)
		E := fe.Eval(b) // white-positive
		if stmMode && s.STM == 0 {
			E = -E
		}
		p := prob(k, E)
		y := s.Label
		diff := p - y
		out.Loss += diff * diff
		dLdE := 2.0 * diff * k * p * (1.0 - p)
		// Accumulate dE/dθ scaled into gradient
		fe.Grad(b, dLdE, out.Grads)
		out.Dk += 2.0 * diff * (E * p * (1.0 - p))
		out.N++
	}
	if l2 > 0 && len(params) == len(out.Grads) {
		applyStandard := true
		applyMatAnchor := true
		applyMobAnchor := true
		applyKsAnchor := true
		if le, ok := fe.(*LinearEval); ok {
			applyStandard = le.Toggles.PSTTrain || le.Toggles.MaterialTrain || le.Toggles.PassersTrain || le.Toggles.PawnStructTrain || le.Toggles.MobilityTrain || le.Toggles.Extras4Train || le.Toggles.Extras6Train || le.Toggles.Extras7Train || le.Toggles.P1Train || le.Toggles.KingTableTrain || le.Toggles.KingCorrTrain || le.Toggles.KingEndgameTrain || le.Toggles.ImbalanceTrain || le.Toggles.WeakTempoTrain
			applyMatAnchor = le.Toggles.MaterialTrain
			applyMobAnchor = le.Toggles.MobilityTrain
			applyKsAnchor = le.Toggles.KingTableTrain
		}
		// Apply standard L2 only to parameters that were touched in this batch.
		// This avoids shrinking sparse terms toward zero when absent.
		if applyStandard {
			for i := range out.Grads {
				if out.Grads[i] != 0 {
					out.Grads[i] += l2 * params[i]
				}
			}
		}
		// Anchored L2 for material values: offsets 768..773 (MG), 774..779 (EG)
		if applyMatAnchor {
			const matMGOff = 768
			const matEGOff = 774
			const matAnchorMul = 4.0
			matMG0 := eng.DefaultPieceValueMG()
			matEG0 := eng.DefaultPieceValueEG()
			for i := 0; i < 6 && matMGOff+i < len(out.Grads); i++ {
				anchor := float64(matMG0[i])
				out.Grads[matMGOff+i] += (l2 * matAnchorMul) * (params[matMGOff+i] - anchor)
			}
			for i := 0; i < 6 && matEGOff+i < len(out.Grads); i++ {
				anchor := float64(matEG0[i])
				out.Grads[matEGOff+i] += (l2 * matAnchorMul) * (params[matEGOff+i] - anchor)
			}
		}
		// Anchored L2 for mobility only: penalize deviation from engine defaults
		// θ_mob_MG: offsets 930..936, θ_mob_EG: 937..943
		// Use the same l2 scalar for anchoring to avoid new flags.
		if applyMobAnchor {
			const mobMGOff = 930
			const mobEGOff = 937
			mobMG0 := eng.DefaultMobilityValueMG()
			mobEG0 := eng.DefaultMobilityValueEG()
			// Always apply (mobility is dense); no need to gate on out.Grads[i]!=0
			for i := 0; i < 7 && mobMGOff+i < len(out.Grads); i++ {
				anchor := float64(mobMG0[i])
				out.Grads[mobMGOff+i] += l2 * (params[mobMGOff+i] - anchor)
			}
			for i := 0; i < 7 && mobEGOff+i < len(out.Grads); i++ {
				anchor := float64(mobEG0[i])
				out.Grads[mobEGOff+i] += l2 * (params[mobEGOff+i] - anchor)
			}
		}
		// Anchored L2 for KingSafetyTable (100 entries): offsets 944..1043
		// Apply a stronger anchor than global l2 to stabilize bins.
		if applyKsAnchor {
			const ksOff = 944
			const ksAnchorMul = 5.0
			ks0 := eng.DefaultKingSafetyTable()
			for i := 0; i < 100 && ksOff+i < len(out.Grads); i++ {
				anchor := float64(ks0[i])
				out.Grads[ksOff+i] += (l2 * ksAnchorMul) * (params[ksOff+i] - anchor)
			}
		}
	}
	return out
}

// batchGradFeIdx is a zero-allocation variant that accumulates into the provided grads slice
// for a window [off,end) of indices drawn from order over data. It returns (loss, dk, n).
func batchGradFeIdx(fe Featurizer, pst *PST, data []Sample, order []int, off, end int, stmMode bool, l2 float64, grads []float64) (float64, float64, int) {
	if fe == nil || pst == nil || grads == nil {
		return 0, 0, 0
	}
	// zero grads
	for i := range grads {
		grads[i] = 0
	}
	params := fe.Params()
	k := pst.K
	var loss, dk float64
	n := 0
	for i := off; i < end; i++ {
		s := &data[order[i]]
		b := NewBoardFromSample(*s)
		E := fe.Eval(b)
		if stmMode && s.STM == 0 {
			E = -E
		}
		p := prob(k, E)
		diff := p - s.Label
		loss += diff * diff
		dLdE := 2.0 * diff * k * p * (1.0 - p)
		fe.Grad(b, dLdE, grads)
		dk += 2.0 * diff * (E * p * (1.0 - p))
		n++
	}
	if l2 > 0 && len(params) == len(grads) {
		applyStandard := true
		applyMatAnchor := true
		applyMobAnchor := true
		applyKsAnchor := true
		if le, ok := fe.(*LinearEval); ok {
			applyStandard = le.Toggles.PSTTrain || le.Toggles.MaterialTrain || le.Toggles.PassersTrain || le.Toggles.PawnStructTrain || le.Toggles.MobilityTrain || le.Toggles.Extras4Train || le.Toggles.Extras6Train || le.Toggles.Extras7Train || le.Toggles.P1Train || le.Toggles.KingTableTrain || le.Toggles.KingCorrTrain || le.Toggles.KingEndgameTrain || le.Toggles.ImbalanceTrain || le.Toggles.WeakTempoTrain
			applyMatAnchor = le.Toggles.MaterialTrain
			applyMobAnchor = le.Toggles.MobilityTrain
			applyKsAnchor = le.Toggles.KingTableTrain
		}
		// Standard L2 on touched params
		if applyStandard {
			for i := range grads {
				if grads[i] != 0 {
					grads[i] += l2 * params[i]
				}
			}
		}
		// Anchored L2 for material values
		if applyMatAnchor {
			const matMGOff = 768
			const matEGOff = 774
			const matAnchorMul = 2.0
			matMG0 := eng.DefaultPieceValueMG()
			matEG0 := eng.DefaultPieceValueEG()
			for i := 0; i < 6 && matMGOff+i < len(grads); i++ {
				grads[matMGOff+i] += (l2 * matAnchorMul) * (params[matMGOff+i] - float64(matMG0[i]))
			}
			for i := 0; i < 6 && matEGOff+i < len(grads); i++ {
				grads[matEGOff+i] += (l2 * matAnchorMul) * (params[matEGOff+i] - float64(matEG0[i]))
			}
		}
		// Anchored L2 for mobility only
		if applyMobAnchor {
			const mobMGOff = 930
			const mobEGOff = 937
			mobMG0 := eng.DefaultMobilityValueMG()
			mobEG0 := eng.DefaultMobilityValueEG()
			for i := 0; i < 7 && mobMGOff+i < len(grads); i++ {
				grads[mobMGOff+i] += l2 * (params[mobMGOff+i] - float64(mobMG0[i]))
			}
			for i := 0; i < 7 && mobEGOff+i < len(grads); i++ {
				grads[mobEGOff+i] += l2 * (params[mobEGOff+i] - float64(mobEG0[i]))
			}
		}
		// Anchored L2 for KingSafetyTable as well, with stronger anchor
		if applyKsAnchor {
			const ksOff = 944
			const ksAnchorMul = 5.0
			ks0 := eng.DefaultKingSafetyTable()
			for i := 0; i < 100 && ksOff+i < len(grads); i++ {
				grads[ksOff+i] += (l2 * ksAnchorMul) * (params[ksOff+i] - float64(ks0[i]))
			}
		}
	}
	return loss, dk, n
}
