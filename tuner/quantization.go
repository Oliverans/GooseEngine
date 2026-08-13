package tuner

import (
	"errors"
	"fmt"
	"math"
	"strconv"
)

type ParameterQuantization struct {
	Index             int             `json:"index"`
	ID                ParameterID     `json:"id"`
	Coordinate        []int           `json:"coordinate,omitempty"`
	CoordinateLabels  []string        `json:"coordinateLabels,omitempty"`
	GoSymbol          string          `json:"goSymbol"`
	ExportIndex       int             `json:"exportIndex"`
	EngineType        EngineValueType `json:"engineType"`
	Rounding          RoundingPolicy  `json:"rounding"`
	Initial           float64         `json:"initial"`
	Continuous        float64         `json:"continuous"`
	Bounded           float64         `json:"bounded"`
	Quantized         int             `json:"quantized"`
	QuantizationError float64         `json:"quantizationError"`
	BoundClamped      bool            `json:"boundClamped"`
	PolicyReset       bool            `json:"policyReset"`
}

type QuantizationMetrics struct {
	Parameters            int     `json:"parameters"`
	ChangedFromInitial    int     `json:"changedFromInitial"`
	NonzeroRoundingErrors int     `json:"nonzeroRoundingErrors"`
	BoundClamps           int     `json:"boundClamps"`
	PolicyResets          int     `json:"policyResets"`
	MeanAbsoluteError     float64 `json:"meanAbsoluteError"`
	RootMeanSquaredError  float64 `json:"rootMeanSquaredError"`
	MaxAbsoluteError      float64 `json:"maxAbsoluteError"`
}

type QuantizationResult struct {
	Values  []int                   `json:"-"`
	Metrics QuantizationMetrics     `json:"metrics"`
	Entries []ParameterQuantization `json:"entries"`
}

// QuantizeParameters applies registry bounds, rounding policy and engine-type
// range validation in that order.
func QuantizeParameters(registry *Registry, parameters []float64) (QuantizationResult, error) {
	if registry == nil {
		return QuantizationResult{}, errors.New("quantization requires a registry")
	}
	if len(parameters) != len(registry.Elements) {
		return QuantizationResult{}, fmt.Errorf("parameter vector has length %d, want %d", len(parameters), len(registry.Elements))
	}
	result := QuantizationResult{
		Values: make([]int, len(parameters)), Entries: make([]ParameterQuantization, len(parameters)),
		Metrics: QuantizationMetrics{Parameters: len(parameters)},
	}
	errorSquared := 0.0
	for index, element := range registry.Elements {
		value := parameters[index]
		if !finite(value) {
			return QuantizationResult{}, fmt.Errorf("parameter %d is not finite", index)
		}
		bounded := value
		policyReset := false
		if element.Mode != TrainingContinuous && bounded != element.Initial {
			bounded = element.Initial
			policyReset = true
		}
		beforeBounds := bounded
		if element.Bounds.Lower.Set && bounded < element.Bounds.Lower.Value {
			bounded = element.Bounds.Lower.Value
		}
		if element.Bounds.Upper.Set && bounded > element.Bounds.Upper.Value {
			bounded = element.Bounds.Upper.Value
		}
		clamped := bounded != beforeBounds
		rounded, err := roundParameter(bounded, registry.Specs[element.SpecIndex].Export.Rounding)
		if err != nil {
			return QuantizationResult{}, fmt.Errorf("parameter %q%v: %w", element.ID, element.Coordinate, err)
		}
		if element.Bounds.Lower.Set && float64(rounded) < element.Bounds.Lower.Value ||
			element.Bounds.Upper.Set && float64(rounded) > element.Bounds.Upper.Value {
			return QuantizationResult{}, fmt.Errorf("parameter %q%v quantizes outside its bounds", element.ID, element.Coordinate)
		}
		export := registry.Specs[element.SpecIndex].Export
		if err := validateEngineInteger(export.GoType, rounded); err != nil {
			return QuantizationResult{}, fmt.Errorf("parameter %q%v: %w", element.ID, element.Coordinate, err)
		}
		quantized := int(rounded)
		result.Values[index] = quantized
		errorValue := float64(quantized) - bounded
		absolute := math.Abs(errorValue)
		if quantized != int(element.Initial) {
			result.Metrics.ChangedFromInitial++
		}
		if errorValue != 0 {
			result.Metrics.NonzeroRoundingErrors++
		}
		if clamped {
			result.Metrics.BoundClamps++
		}
		if policyReset {
			result.Metrics.PolicyResets++
		}
		result.Metrics.MeanAbsoluteError += absolute
		errorSquared += errorValue * errorValue
		result.Metrics.MaxAbsoluteError = max(result.Metrics.MaxAbsoluteError, absolute)
		result.Entries[index] = ParameterQuantization{
			Index: index, ID: element.ID, Coordinate: append([]int(nil), element.Coordinate...),
			CoordinateLabels: append([]string(nil), element.CoordinateLabels...),
			GoSymbol:         export.GoSymbol, ExportIndex: element.ExportIndex, EngineType: export.GoType,
			Rounding: export.Rounding, Initial: element.Initial, Continuous: value, Bounded: bounded,
			Quantized: quantized, QuantizationError: errorValue, BoundClamped: clamped, PolicyReset: policyReset,
		}
	}
	if len(parameters) != 0 {
		result.Metrics.MeanAbsoluteError /= float64(len(parameters))
		result.Metrics.RootMeanSquaredError = math.Sqrt(errorSquared / float64(len(parameters)))
	}
	return result, nil
}

func roundParameter(value float64, policy RoundingPolicy) (int64, error) {
	var rounded float64
	switch policy {
	case RoundNearest:
		rounded = math.Round(value)
	case RoundTowardZero:
		rounded = math.Trunc(value)
	case RoundFloor:
		rounded = math.Floor(value)
	case RoundCeil:
		rounded = math.Ceil(value)
	default:
		return 0, fmt.Errorf("unknown rounding policy %q", policy)
	}
	if !finite(rounded) || rounded < math.MinInt64 || rounded > math.MaxInt64 {
		return 0, fmt.Errorf("rounded value %v is outside int64", rounded)
	}
	return int64(rounded), nil
}

func validateEngineInteger(kind EngineValueType, value int64) error {
	var lower, upper int64
	switch kind {
	case EngineInt:
		if strconv.IntSize == 32 {
			lower, upper = math.MinInt32, math.MaxInt32
		} else {
			lower, upper = math.MinInt64, math.MaxInt64
		}
	case EngineInt16:
		lower, upper = math.MinInt16, math.MaxInt16
	case EngineInt32:
		lower, upper = math.MinInt32, math.MaxInt32
	case EngineInt64:
		lower, upper = math.MinInt64, math.MaxInt64
	default:
		return fmt.Errorf("unknown engine integer type %q", kind)
	}
	if value < lower || value > upper {
		return fmt.Errorf("value %d is outside %s range", value, kind)
	}
	return nil
}
