// tuner/loss.go
package tuner

import (
    "math"
    eng "chess-engine/engine"
)

// logistic probability p = 1/(1+exp(-k*E))
func prob(k, eval float64) float64 {
	z := k * eval
	if z > 40 { return 1 }
	if z < -40 { return 0 }
	return 1.0 / (1.0 + math.Exp(-z))
}

// Legacy PST-only batch gradient (kept for reference). New code should use batchGradFe.
func batchGrad(pst *PST, batch []Sample, stmMode bool, l2 float64) BatchGrad {
	var out BatchGrad
	k := pst.K
	for i := range batch {
		s := &batch[i]
		E := evalPST(pst, *s) // white-positive
		if stmMode && s.STM == 0 { E = -E }
		p := prob(k, E)
		y := s.Label
		diff := p - y
		out.Loss += diff * diff
		// dL/dE = 2*(p-y) * (k*p*(1-p))
		dLdE := 2.0 * diff * k * p * (1.0 - p)
		addEvalGrad(pst, *s, &out.MG, &out.EG, dLdE)
		// dL/dk (optional)
		out.Dk += 2.0 * diff * (E * p * (1.0 - p))
		out.N++
	}
	if l2 > 0 {
		for pt := 0; pt < 6; pt++ {
			for sq := 0; sq < 64; sq++ {
				out.MG[pt][sq] += l2 * pst.MG[pt][sq]
				out.EG[pt][sq] += l2 * pst.EG[pt][sq]
			}
		}
	}
	return out
}

// one-dimensional refit for k on a subset
func refitK(fe Featurizer, pst *PST, data []Sample, stmMode bool) {
    k0 := pst.K
    bestK, bestLoss := k0, math.MaxFloat64
    cands := []float64{k0*0.5, k0*0.67, k0*0.8, k0*0.9, k0, k0*1.1, k0*1.25, k0*1.5}
    for _, k := range cands {
        sum := 0.0
        for i := range data {
            b := NewBoardFromSample(data[i])
            E := fe.Eval(b)
            if stmMode && data[i].STM == 0 { E = -E }
            p := prob(k, E)
            d := p - data[i].Label
            sum += d*d
        }
        if sum < bestLoss { bestLoss, bestK = sum, k }
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
        if stmMode && s.STM == 0 { E = -E }
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
        // Apply standard L2 only to parameters that were touched in this batch.
        // This avoids shrinking sparse terms toward zero when absent.
        for i := range out.Grads {
            if out.Grads[i] != 0 {
                out.Grads[i] += l2 * params[i]
            }
        }
        // Anchored L2 for mobility only: penalize deviation from engine defaults
        // θ_mob_MG: offsets 930..936, θ_mob_EG: 937..943
        // Use the same l2 scalar for anchoring to avoid new flags.
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
        // Anchored L2 for KingSafetyTable (100 entries): offsets 944..1043
        // Apply a stronger anchor than global l2 to stabilize bins.
        const ksOff = 944
        const ksAnchorMul = 5.0
        ks0 := eng.DefaultKingSafetyTable()
        for i := 0; i < 100 && ksOff+i < len(out.Grads); i++ {
            anchor := float64(ks0[i])
            out.Grads[ksOff+i] += (l2 * ksAnchorMul) * (params[ksOff+i] - anchor)
        }
    }
    return out
}

// batchGradFeIdx is a zero-allocation variant that accumulates into the provided grads slice
// for a window [off,end) of indices drawn from order over data. It returns (loss, dk, n).
func batchGradFeIdx(fe Featurizer, pst *PST, data []Sample, order []int, off, end int, stmMode bool, l2 float64, grads []float64) (float64, float64, int) {
    if fe == nil || pst == nil || grads == nil { return 0, 0, 0 }
    // zero grads
    for i := range grads { grads[i] = 0 }
    params := fe.Params()
    k := pst.K
    var loss, dk float64
    n := 0
    for i := off; i < end; i++ {
        s := &data[order[i]]
        b := NewBoardFromSample(*s)
        E := fe.Eval(b)
        if stmMode && s.STM == 0 { E = -E }
        p := prob(k, E)
        diff := p - s.Label
        loss += diff * diff
        dLdE := 2.0 * diff * k * p * (1.0 - p)
        fe.Grad(b, dLdE, grads)
        dk += 2.0 * diff * (E * p * (1.0 - p))
        n++
    }
    if l2 > 0 && len(params) == len(grads) {
        // Standard L2 on touched params
        for i := range grads {
            if grads[i] != 0 { grads[i] += l2 * params[i] }
        }
        // Anchored L2 for mobility only
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
        // Anchored L2 for KingSafetyTable as well, with stronger anchor
        const ksOff = 944
        const ksAnchorMul = 5.0
        ks0 := eng.DefaultKingSafetyTable()
        for i := 0; i < 100 && ksOff+i < len(grads); i++ {
            grads[ksOff+i] += (l2 * ksAnchorMul) * (params[ksOff+i] - float64(ks0[i]))
        }
    }
    return loss, dk, n
}
