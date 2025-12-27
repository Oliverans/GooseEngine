// tuner/opt_adagrad.go
package tuner

import "math"

func NewAdaGrad(numParams int, lr float64) *AdaGrad {
	return &AdaGrad{
		G:       make([]float64, numParams),
		LR:      lr,
		Eps:     1e-8,
		LRScale: nil,
	}
}

// SetLR updates the base learning rate.
func (opt *AdaGrad) SetLR(lr float64) {
	opt.LR = lr
}

// GetLR returns the current base learning rate.
func (opt *AdaGrad) GetLR() float64 {
	return opt.LR
}

// SetLRScale sets an optional per-parameter learning-rate multiplier vector.
// If nil or wrong length, it is ignored and a scale of 1.0 is used.
func (opt *AdaGrad) SetLRScale(scale []float64) {
	opt.LRScale = scale
}

func (opt *AdaGrad) Step(params []float64, grads []float64) {
	useScale := opt.LRScale != nil && len(opt.LRScale) == len(params)
	for i := range params {
		g := grads[i]
		if g == 0 {
			continue
		}
		opt.G[i] += g * g
		s := 1.0
		if useScale {
			s = opt.LRScale[i]
		}
		adj := (opt.LR * s) / (math.Sqrt(opt.G[i]) + opt.Eps)
		params[i] -= adj * g
	}
}
