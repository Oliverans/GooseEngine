package tuner

import (
	"errors"
	"fmt"
	"math"
)

type AdamConfig struct {
	Beta1   float64
	Beta2   float64
	Epsilon float64
}

func DefaultAdamConfig() AdamConfig {
	return AdamConfig{Beta1: 0.9, Beta2: 0.999, Epsilon: 1e-8}
}

func (c AdamConfig) Validate() error {
	if !finite(c.Beta1) || c.Beta1 < 0 || c.Beta1 >= 1 {
		return fmt.Errorf("Adam beta1 must be in [0,1), got %v", c.Beta1)
	}
	if !finite(c.Beta2) || c.Beta2 < 0 || c.Beta2 >= 1 {
		return fmt.Errorf("Adam beta2 must be in [0,1), got %v", c.Beta2)
	}
	if !finite(c.Epsilon) || c.Epsilon <= 0 {
		return fmt.Errorf("Adam epsilon must be finite and positive, got %v", c.Epsilon)
	}
	return nil
}

type adamCoordinate struct {
	parameterIndex    int
	learningRateScale float64
	bounds            Bounds
	active            bool
}

type AdamStepMetrics struct {
	Step              uint64
	LearningRate      float64
	ActiveCoordinates int
	BoundHits         int
	GradientL2        float64
	UpdateL2          float64
	MaxAbsoluteUpdate float64
}

// AdamOptimizer owns dense first/second moments while updating the complete
// forward parameter vector through precompiled registry coordinate mappings.
type AdamOptimizer struct {
	config         AdamConfig
	parameterCount int
	coordinates    []adamCoordinate
	firstMoment    []float64
	secondMoment   []float64
	nextFirst      []float64
	nextSecond     []float64
	nextValue      []float64
	lastUpdate     []float64
	nextUpdate     []float64
	step           uint64
	activeCount    int
}

func NewAdamOptimizer(registry *Registry, activeTrainIndexes []int, config AdamConfig) (*AdamOptimizer, error) {
	if registry == nil {
		return nil, errors.New("Adam requires a registry")
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	active := make([]bool, registry.TrainableCount())
	if activeTrainIndexes == nil {
		for index := range active {
			active[index] = true
		}
	} else {
		for _, index := range activeTrainIndexes {
			if index < 0 || index >= len(active) {
				return nil, fmt.Errorf("active train index %d outside [0,%d)", index, len(active))
			}
			if active[index] {
				return nil, fmt.Errorf("active train index %d is duplicated", index)
			}
			active[index] = true
		}
	}
	coordinates := make([]adamCoordinate, registry.TrainableCount())
	seen := make([]bool, registry.TrainableCount())
	activeCount := 0
	for _, element := range registry.Elements {
		if element.TrainIndex == NoTrainIndex {
			continue
		}
		coordinates[element.TrainIndex] = adamCoordinate{
			parameterIndex: element.Index, learningRateScale: element.LearningRateScale,
			bounds: element.Bounds, active: active[element.TrainIndex],
		}
		seen[element.TrainIndex] = true
		if active[element.TrainIndex] {
			activeCount++
		}
	}
	for index, exists := range seen {
		if !exists {
			return nil, fmt.Errorf("registry has no parameter for train index %d", index)
		}
	}
	count := registry.TrainableCount()
	return &AdamOptimizer{
		config: config, parameterCount: len(registry.Elements), coordinates: coordinates,
		firstMoment: make([]float64, count), secondMoment: make([]float64, count),
		nextFirst: make([]float64, count), nextSecond: make([]float64, count), nextValue: make([]float64, count),
		lastUpdate: make([]float64, count), nextUpdate: make([]float64, count),
		activeCount: activeCount,
	}, nil
}

func (o *AdamOptimizer) Step() uint64 {
	if o == nil {
		return 0
	}
	return o.step
}

func (o *AdamOptimizer) ActiveCount() int {
	if o == nil {
		return 0
	}
	return o.activeCount
}

// Update performs one atomic bounded Adam step. Inactive coordinates retain
// both their parameter value and zero optimizer state.
func (o *AdamOptimizer) Update(parameters []float64, gradient []float64, learningRate float64) (AdamStepMetrics, error) {
	if o == nil {
		return AdamStepMetrics{}, errors.New("cannot update with a nil Adam optimizer")
	}
	if len(gradient) != len(o.coordinates) {
		return AdamStepMetrics{}, fmt.Errorf("Adam gradient has length %d, want %d", len(gradient), len(o.coordinates))
	}
	if len(parameters) != o.parameterCount {
		return AdamStepMetrics{}, fmt.Errorf("Adam parameter vector has length %d, want %d", len(parameters), o.parameterCount)
	}
	if !finite(learningRate) || learningRate <= 0 {
		return AdamStepMetrics{}, fmt.Errorf("learning rate must be finite and positive, got %v", learningRate)
	}
	if o.step == ^uint64(0) {
		return AdamStepMetrics{}, errors.New("Adam step counter overflow")
	}
	for index, coordinate := range o.coordinates {
		if coordinate.parameterIndex < 0 || coordinate.parameterIndex >= len(parameters) {
			return AdamStepMetrics{}, fmt.Errorf("Adam parameter index %d outside vector length %d", coordinate.parameterIndex, len(parameters))
		}
		if !finite(parameters[coordinate.parameterIndex]) {
			return AdamStepMetrics{}, fmt.Errorf("Adam parameter %d is not finite", coordinate.parameterIndex)
		}
		if !finite(gradient[index]) {
			return AdamStepMetrics{}, fmt.Errorf("Adam gradient %d is not finite", index)
		}
	}

	nextStep := o.step + 1
	bias1 := 1 - math.Pow(o.config.Beta1, float64(nextStep))
	bias2 := 1 - math.Pow(o.config.Beta2, float64(nextStep))
	metrics := AdamStepMetrics{Step: nextStep, LearningRate: learningRate, ActiveCoordinates: o.activeCount}
	gradientSquared, updateSquared := 0.0, 0.0
	for index, coordinate := range o.coordinates {
		if !coordinate.active {
			o.nextUpdate[index] = 0
			continue
		}
		value := gradient[index]
		first := o.config.Beta1*o.firstMoment[index] + (1-o.config.Beta1)*value
		second := o.config.Beta2*o.secondMoment[index] + (1-o.config.Beta2)*value*value
		correctedFirst := first / bias1
		correctedSecond := second / bias2
		update := learningRate * coordinate.learningRateScale * correctedFirst /
			(math.Sqrt(correctedSecond) + o.config.Epsilon)
		candidate := parameters[coordinate.parameterIndex] - update
		bounded := false
		if coordinate.bounds.Lower.Set && candidate < coordinate.bounds.Lower.Value {
			candidate = coordinate.bounds.Lower.Value
			bounded = true
		}
		if coordinate.bounds.Upper.Set && candidate > coordinate.bounds.Upper.Value {
			candidate = coordinate.bounds.Upper.Value
			bounded = true
		}
		if !finite(first) || !finite(second) || !finite(candidate) {
			return AdamStepMetrics{}, fmt.Errorf("Adam coordinate %d produced a non-finite state", index)
		}
		o.nextFirst[index], o.nextSecond[index], o.nextValue[index] = first, second, candidate
		actualUpdate := parameters[coordinate.parameterIndex] - candidate
		o.nextUpdate[index] = actualUpdate
		gradientSquared += value * value
		updateSquared += actualUpdate * actualUpdate
		metrics.MaxAbsoluteUpdate = max(metrics.MaxAbsoluteUpdate, math.Abs(actualUpdate))
		if bounded {
			metrics.BoundHits++
		}
	}
	if !finite(gradientSquared) || !finite(updateSquared) || !finite(metrics.MaxAbsoluteUpdate) {
		return AdamStepMetrics{}, errors.New("Adam step metrics are not finite")
	}

	for index, coordinate := range o.coordinates {
		if !coordinate.active {
			continue
		}
		o.firstMoment[index] = o.nextFirst[index]
		o.secondMoment[index] = o.nextSecond[index]
		o.lastUpdate[index] = o.nextUpdate[index]
		parameters[coordinate.parameterIndex] = o.nextValue[index]
	}
	o.step = nextStep
	metrics.GradientL2 = math.Sqrt(gradientSquared)
	metrics.UpdateL2 = math.Sqrt(updateSquared)
	return metrics, nil
}
