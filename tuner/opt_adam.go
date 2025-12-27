package tuner

import "math"

type Adam struct {
	M, V    []float64 // First and second moment estimates
	LR      float64
	Beta1   float64 // Typically 0.9
	Beta2   float64 // Typically 0.999
	Eps     float64
	T       int // Timestep (for bias correction)
	LRScale []float64
}

func NewAdam(numParams int, lr float64) *Adam {
	return &Adam{
		M:       make([]float64, numParams),
		V:       make([]float64, numParams),
		LR:      lr,
		Beta1:   0.9,
		Beta2:   0.999,
		Eps:     1e-8,
		T:       0,
		LRScale: nil,
	}
}

// SetLR updates the base learning rate.
func (opt *Adam) SetLR(lr float64) {
	opt.LR = lr
}

// GetLR returns the current base learning rate.
func (opt *Adam) GetLR() float64 {
	return opt.LR
}

// SetLRScale sets an optional per-parameter learning-rate multiplier vector.
// If the slice is nil or the length does not match params, scaling is ignored.
func (opt *Adam) SetLRScale(scale []float64) {
	opt.LRScale = scale
}

func (opt *Adam) Step(params []float64, grads []float64) {
	opt.T++
	useScale := opt.LRScale != nil && len(opt.LRScale) == len(params)

	// Bias correction factors
	bc1 := 1.0 - math.Pow(opt.Beta1, float64(opt.T))
	bc2 := 1.0 - math.Pow(opt.Beta2, float64(opt.T))

	for i := range params {
		g := grads[i]
		if g == 0 {
			continue
		}

		// Update biased moments
		opt.M[i] = opt.Beta1*opt.M[i] + (1-opt.Beta1)*g
		opt.V[i] = opt.Beta2*opt.V[i] + (1-opt.Beta2)*g*g

		// Bias-corrected estimates
		mHat := opt.M[i] / bc1
		vHat := opt.V[i] / bc2

		s := 1.0
		if useScale {
			s = opt.LRScale[i]
		}

		params[i] -= (opt.LR * s) * mHat / (math.Sqrt(vHat) + opt.Eps)
	}
}
