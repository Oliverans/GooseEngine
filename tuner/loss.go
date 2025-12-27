package tuner

import "math"

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
func refitK(fe Featurizer, pst *PST, data []BinarySample, stmMode bool) {
	k0 := pst.K
	bestK, bestLoss := k0, math.MaxFloat64
	cands := []float64{k0 * 0.5, k0 * 0.67, k0 * 0.8, k0 * 0.9, k0, k0 * 1.1, k0 * 1.25, k0 * 1.5}
	for _, k := range cands {
		sum := 0.0
		for i := range data {
			b := NewBoardFromBinarySample(data[i])
			E := fe.Eval(b)
			if stmMode && data[i].STM == 0 {
				E = -E
			}
			p := prob(k, E)
			d := p - float64(data[i].Label)
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
func batchGradFe(fe Featurizer, pst *PST, batch []Sample, stmMode bool) BatchGradFlat {
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
	return out
}

// batchGradFeIdx is a zero-allocation variant that accumulates into the provided grads slice
// for a window [off,end) of indices drawn from order over data. It returns (loss, dk, n).
func batchGradFeIdx(fe Featurizer, pst *PST, data []BinarySample, order []int, off, end int, stmMode bool, grads []float64) (float64, float64, int) {
	if fe == nil || pst == nil || grads == nil {
		return 0, 0, 0
	}
	// zero grads
	for i := range grads {
		grads[i] = 0
	}
	k := pst.K
	var loss, dk float64
	n := 0
	for i := off; i < end; i++ {
		s := &data[order[i]]
		b := NewBoardFromBinarySample(*s)
		E := fe.Eval(b)
		if stmMode && s.STM == 0 {
			E = -E
		}
		p := prob(k, E)
		diff := p - float64(s.Label)
		loss += diff * diff
		dLdE := 2.0 * diff * k * p * (1.0 - p)
		fe.Grad(b, dLdE, grads)
		dk += 2.0 * diff * (E * p * (1.0 - p))
		n++
	}
	// [DEBUG_TMP] quick grad norm/logits on first batch
	if off == 0 {
		var gradNorm float64
		for _, v := range grads {
			gradNorm += math.Abs(v)
		}
		_ = gradNorm
	}
	return loss, dk, n
}

// batchLossFeIdx computes loss over a window [off,end) of indices drawn from order.
// It mirrors batchGradFeIdx but skips gradient accumulation.
func batchLossFeIdx(fe Featurizer, pst *PST, data []BinarySample, order []int, off, end int, stmMode bool) (float64, int) {
	if fe == nil || pst == nil || order == nil {
		return 0, 0
	}
	k := pst.K
	var loss float64
	n := 0
	for i := off; i < end; i++ {
		s := &data[order[i]]
		b := NewBoardFromBinarySample(*s)
		E := fe.Eval(b)
		if stmMode && s.STM == 0 {
			E = -E
		}
		p := prob(k, E)
		diff := p - float64(s.Label)
		loss += diff * diff
		n++
	}
	return loss, n
}

// AnchoredL2Loss computes base loss + weighted L2 penalty from anchor values.
func AnchoredL2Loss(theta, anchor, anchorWeights []float64, baseLoss float64) float64 {
	penalty := 0.0
	for i := range theta {
		diff := theta[i] - anchor[i]
		penalty += anchorWeights[i] * diff * diff
	}
	return baseLoss + penalty
}

// AnchoredL2Grad adds the anchored L2 gradient to the existing gradient buffer.
func AnchoredL2Grad(grad, theta, anchor, anchorWeights []float64) {
	for i := range grad {
		grad[i] += 2.0 * anchorWeights[i] * (theta[i] - anchor[i])
	}
}
