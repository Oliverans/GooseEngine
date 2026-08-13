package tuner

import (
	"errors"
	"fmt"
	"math"
	"sort"
)

// TexelLink converts a White-perspective engine evaluation into an expected
// White result. K uses the same natural-logistic convention as the old tuner:
// p = 1 / (1 + exp(-K * evaluation)).
type TexelLink struct {
	K float64
}

func NewTexelLink(k float64) (TexelLink, error) {
	if !finite(k) || k <= 0 {
		return TexelLink{}, fmt.Errorf("Texel K must be finite and positive, got %v", k)
	}
	return TexelLink{K: k}, nil
}

// TexelSampleLoss contains unweighted per-position losses and their
// derivatives with respect to the White-perspective evaluation. A batch
// multiplies these values by its sample weight and divides by total weight.
type TexelSampleLoss struct {
	ExpectedWhiteScore  float64
	TargetWhiteScore    float64
	Brier               float64
	LogLoss             float64
	BrierDerivativeEval float64
	LogDerivativeEval   float64
	BrierDerivativeK    float64
}

func (l TexelLink) Evaluate(evaluation float64, outcome Outcome) (TexelSampleLoss, error) {
	if !finite(l.K) || l.K <= 0 {
		return TexelSampleLoss{}, fmt.Errorf("Texel K must be finite and positive, got %v", l.K)
	}
	if !finite(evaluation) {
		return TexelSampleLoss{}, fmt.Errorf("evaluation must be finite, got %v", evaluation)
	}
	if !outcome.valid() {
		return TexelSampleLoss{}, fmt.Errorf("invalid outcome %d", outcome)
	}
	z := l.K * evaluation
	if !finite(z) {
		return TexelSampleLoss{}, fmt.Errorf("scaled evaluation is not finite: K=%v evaluation=%v", l.K, evaluation)
	}
	p := stableSigmoid(z)
	y := outcome.WhiteScore()
	difference := p - y
	pSlope := p * (1 - p)
	return TexelSampleLoss{
		ExpectedWhiteScore:  p,
		TargetWhiteScore:    y,
		Brier:               difference * difference,
		LogLoss:             stableSoftplus(z) - y*z,
		BrierDerivativeEval: 2 * difference * l.K * pSlope,
		LogDerivativeEval:   l.K * difference,
		BrierDerivativeK:    2 * difference * evaluation * pSlope,
	}, nil
}

func stableSigmoid(value float64) float64 {
	if value >= 0 {
		exponential := math.Exp(-value)
		return 1 / (1 + exponential)
	}
	exponential := math.Exp(value)
	return exponential / (1 + exponential)
}

func stableSoftplus(value float64) float64 {
	if value > 0 {
		return value + math.Log1p(math.Exp(-value))
	}
	return math.Log1p(math.Exp(value))
}

type OutcomeLossMetrics struct {
	Samples                uint64
	Weight                 float64
	Brier                  float64
	LogLoss                float64
	MeanExpectedWhiteScore float64
	MeanTargetWhiteScore   float64
}

type DataLossMetrics struct {
	OutcomeLossMetrics
	ByOutcome [3]OutcomeLossMetrics
}

type weightedLossSums struct {
	samples    uint64
	weight     float64
	brier      float64
	logLoss    float64
	prediction float64
	target     float64
}

// OutcomeLossAccumulator reports data fit only. Parameter-anchor penalties are
// deliberately accumulated separately so validation quality cannot be hidden
// by a regularization term.
type OutcomeLossAccumulator struct {
	total     weightedLossSums
	byOutcome [3]weightedLossSums
}

// Add evaluates and accumulates one sample. A zero weight explicitly excludes
// the sample. The returned derivatives remain unweighted.
func (a *OutcomeLossAccumulator) Add(link TexelLink, evaluation float64, outcome Outcome, weight float64) (TexelSampleLoss, error) {
	if a == nil {
		return TexelSampleLoss{}, errors.New("cannot add loss to a nil accumulator")
	}
	if !finite(weight) || weight < 0 {
		return TexelSampleLoss{}, fmt.Errorf("sample weight must be finite and non-negative, got %v", weight)
	}
	sample, err := link.Evaluate(evaluation, outcome)
	if err != nil {
		return TexelSampleLoss{}, err
	}
	if weight == 0 {
		return sample, nil
	}
	a.total.add(sample, weight)
	a.byOutcome[outcome].add(sample, weight)
	return sample, nil
}

func (s *weightedLossSums) add(sample TexelSampleLoss, weight float64) {
	s.samples++
	s.weight += weight
	s.brier += weight * sample.Brier
	s.logLoss += weight * sample.LogLoss
	s.prediction += weight * sample.ExpectedWhiteScore
	s.target += weight * sample.TargetWhiteScore
}

func (a *OutcomeLossAccumulator) Metrics() (DataLossMetrics, error) {
	if a == nil || a.total.weight <= 0 {
		return DataLossMetrics{}, errors.New("loss metrics require positive accumulated weight")
	}
	metrics := DataLossMetrics{OutcomeLossMetrics: a.total.metrics()}
	for outcome := range metrics.ByOutcome {
		metrics.ByOutcome[outcome] = a.byOutcome[outcome].metrics()
	}
	return metrics, nil
}

func (s weightedLossSums) metrics() OutcomeLossMetrics {
	metrics := OutcomeLossMetrics{Samples: s.samples, Weight: s.weight}
	if s.weight == 0 {
		return metrics
	}
	metrics.Brier = s.brier / s.weight
	metrics.LogLoss = s.logLoss / s.weight
	metrics.MeanExpectedWhiteScore = s.prediction / s.weight
	metrics.MeanTargetWhiteScore = s.target / s.weight
	return metrics
}

// TexelCalibrationSet collapses repeated fixed-engine evaluations into
// weighted outcome bins. This permits deterministic one-dimensional K fitting
// without retaining one float per position.
type TexelCalibrationSet struct {
	bins           map[float64][3]float64
	samples        uint64
	outcomeSamples [3]uint64
	totalWeight    float64
}

func (s *TexelCalibrationSet) Add(evaluation float64, outcome Outcome, weight float64) error {
	if s == nil {
		return errors.New("cannot add calibration data to a nil set")
	}
	if !finite(evaluation) {
		return fmt.Errorf("calibration evaluation must be finite, got %v", evaluation)
	}
	if !outcome.valid() {
		return fmt.Errorf("invalid calibration outcome %d", outcome)
	}
	if !finite(weight) || weight < 0 {
		return fmt.Errorf("calibration weight must be finite and non-negative, got %v", weight)
	}
	if weight == 0 {
		return nil
	}
	if s.bins == nil {
		s.bins = make(map[float64][3]float64)
	}
	weights := s.bins[evaluation]
	weights[outcome] += weight
	s.bins[evaluation] = weights
	s.samples++
	s.outcomeSamples[outcome]++
	s.totalWeight += weight
	return nil
}

// Metrics evaluates all collapsed calibration bins without expanding them
// back to per-position records.
func (s *TexelCalibrationSet) Metrics(link TexelLink) (DataLossMetrics, error) {
	if s == nil || s.totalWeight <= 0 {
		return DataLossMetrics{}, errors.New("calibration metrics require positive accumulated weight")
	}
	var accumulator OutcomeLossAccumulator
	for evaluation, weights := range s.bins {
		for outcome, weight := range weights {
			if weight == 0 {
				continue
			}
			sample, err := link.Evaluate(evaluation, Outcome(outcome))
			if err != nil {
				return DataLossMetrics{}, err
			}
			accumulator.total.addAggregate(sample, weight)
			accumulator.byOutcome[outcome].addAggregate(sample, weight)
		}
	}
	accumulator.total.samples = s.samples
	for outcome := range accumulator.byOutcome {
		accumulator.byOutcome[outcome].samples = s.outcomeSamples[outcome]
	}
	return accumulator.Metrics()
}

func (s *weightedLossSums) addAggregate(sample TexelSampleLoss, weight float64) {
	s.weight += weight
	s.brier += weight * sample.Brier
	s.logLoss += weight * sample.LogLoss
	s.prediction += weight * sample.ExpectedWhiteScore
	s.target += weight * sample.TargetWhiteScore
}

type TexelKFit struct {
	K           float64
	Brier       float64
	Iterations  int
	Samples     uint64
	TotalWeight float64
	UniqueEvals int
}

// FitBrier finds a fixed K with deterministic golden-section search. Bounds
// must be chosen in the natural-logistic K convention used by TexelLink.
func (s *TexelCalibrationSet) FitBrier(lower, upper, tolerance float64, maxIterations int) (TexelKFit, error) {
	if s == nil || s.totalWeight <= 0 || len(s.bins) == 0 {
		return TexelKFit{}, errors.New("Texel K fitting requires positive calibration weight")
	}
	if !finite(lower) || !finite(upper) || lower <= 0 || upper <= lower {
		return TexelKFit{}, fmt.Errorf("invalid Texel K bounds [%v,%v]", lower, upper)
	}
	if !finite(tolerance) || tolerance <= 0 {
		return TexelKFit{}, fmt.Errorf("Texel K tolerance must be finite and positive, got %v", tolerance)
	}
	if maxIterations <= 0 {
		return TexelKFit{}, fmt.Errorf("Texel K iterations must be positive, got %d", maxIterations)
	}

	type bin struct {
		evaluation float64
		weights    [3]float64
	}
	bins := make([]bin, 0, len(s.bins))
	for evaluation, weights := range s.bins {
		bins = append(bins, bin{evaluation: evaluation, weights: weights})
	}
	sort.Slice(bins, func(i, j int) bool { return bins[i].evaluation < bins[j].evaluation })
	lossAt := func(k float64) float64 {
		total := 0.0
		for _, item := range bins {
			prediction := stableSigmoid(k * item.evaluation)
			for outcome, weight := range item.weights {
				difference := prediction - Outcome(outcome).WhiteScore()
				total += weight * difference * difference
			}
		}
		return total / s.totalWeight
	}

	const golden = 0.6180339887498948482
	left, right := lower, upper
	innerLeft := right - golden*(right-left)
	innerRight := left + golden*(right-left)
	leftLoss, rightLoss := lossAt(innerLeft), lossAt(innerRight)
	iterations := 0
	for iterations < maxIterations && right-left > tolerance {
		if leftLoss <= rightLoss {
			right, innerRight, rightLoss = innerRight, innerLeft, leftLoss
			innerLeft = right - golden*(right-left)
			leftLoss = lossAt(innerLeft)
		} else {
			left, innerLeft, leftLoss = innerLeft, innerRight, rightLoss
			innerRight = left + golden*(right-left)
			rightLoss = lossAt(innerRight)
		}
		iterations++
	}

	candidates := []float64{lower, upper, left, right, innerLeft, innerRight, (left + right) / 2}
	bestK, bestLoss := candidates[0], lossAt(candidates[0])
	for _, candidate := range candidates[1:] {
		if loss := lossAt(candidate); loss < bestLoss {
			bestK, bestLoss = candidate, loss
		}
	}
	return TexelKFit{
		K: bestK, Brier: bestLoss, Iterations: iterations,
		Samples: s.samples, TotalWeight: s.totalWeight, UniqueEvals: len(bins),
	}, nil
}
