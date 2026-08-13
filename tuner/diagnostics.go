package tuner

import (
	"errors"
	"math"
	"sort"
)

type TrainingGroupDiagnostics struct {
	Group                        GroupID
	ActiveParameters             int
	FrozenParameters             int
	UncoveredParameters          int
	GradientL2                   float64
	UpdateL2                     float64
	RMSNormalizedDisplacement    float64
	MaxAbsNormalizedDisplacement float64
	SignChangesFromAnchor        int
	AtBounds                     int
}

type ParameterDiagnostics struct {
	Index                  int
	ID                     ParameterID
	Coordinate             []int
	CoordinateLabels       []string
	Group                  GroupID
	Value                  float64
	Anchor                 float64
	DeviationScale         float64
	NormalizedDisplacement float64
	Gradient               float64
	ParameterDelta         float64
	AtLowerBound           bool
	AtUpperBound           bool
}

type TrainingDiagnostics struct {
	AdamStep         uint64
	Cursor           TrainingCursor
	Groups           []TrainingGroupDiagnostics
	TopDisplacements []ParameterDiagnostics
}

// Diagnostics summarizes the most recently computed batch gradient/update and
// current displacement from the parameter anchors. topN <= 0 omits the detail
// list while retaining complete group summaries.
func (t *Trainer) Diagnostics(topN int) (TrainingDiagnostics, error) {
	if t == nil || t.registry == nil || t.optimizer == nil {
		return TrainingDiagnostics{}, errors.New("cannot diagnose a nil trainer")
	}
	groupIndex := make(map[GroupID]int, len(t.registry.Groups))
	result := TrainingDiagnostics{
		AdamStep: t.optimizer.step, Cursor: t.cursor,
		Groups: make([]TrainingGroupDiagnostics, len(t.registry.Groups)),
	}
	for index, group := range t.registry.Groups {
		groupIndex[group.ID] = index
		result.Groups[index].Group = group.ID
	}
	activeByParameter := make([]bool, len(t.registry.Elements))
	trainByParameter := make([]int, len(t.registry.Elements))
	for index := range trainByParameter {
		trainByParameter[index] = NoTrainIndex
	}
	for trainIndex, coordinate := range t.optimizer.coordinates {
		activeByParameter[coordinate.parameterIndex] = coordinate.active
		trainByParameter[coordinate.parameterIndex] = trainIndex
	}
	candidates := make([]ParameterDiagnostics, 0, t.optimizer.activeCount)
	groupSquaredDisplacement := make([]float64, len(result.Groups))
	for _, element := range t.registry.Elements {
		group := t.registry.Specs[element.SpecIndex].Group
		index := groupIndex[group]
		metrics := &result.Groups[index]
		if element.Mode != TrainingContinuous || element.TrainIndex == NoTrainIndex {
			metrics.FrozenParameters++
			continue
		}
		if !activeByParameter[element.Index] {
			metrics.UncoveredParameters++
			continue
		}
		metrics.ActiveParameters++
		value := t.parameters[element.Index]
		normalized := (value - element.Anchor) / element.DeviationScale
		absolute := math.Abs(normalized)
		groupSquaredDisplacement[index] += normalized * normalized
		metrics.MaxAbsNormalizedDisplacement = max(metrics.MaxAbsNormalizedDisplacement, absolute)
		if value*element.Anchor < 0 {
			metrics.SignChangesFromAnchor++
		}
		trainIndex := trainByParameter[element.Index]
		gradient := t.gradient[trainIndex]
		parameterDelta := -t.optimizer.lastUpdate[trainIndex]
		metrics.GradientL2 += gradient * gradient
		metrics.UpdateL2 += parameterDelta * parameterDelta
		atLower := element.Bounds.Lower.Set && value == element.Bounds.Lower.Value
		atUpper := element.Bounds.Upper.Set && value == element.Bounds.Upper.Value
		if atLower || atUpper {
			metrics.AtBounds++
		}
		if topN > 0 {
			candidates = append(candidates, ParameterDiagnostics{
				Index: element.Index, ID: element.ID,
				Coordinate:       append([]int(nil), element.Coordinate...),
				CoordinateLabels: append([]string(nil), element.CoordinateLabels...),
				Group:            group, Value: value, Anchor: element.Anchor,
				DeviationScale: element.DeviationScale, NormalizedDisplacement: normalized,
				Gradient: gradient, ParameterDelta: parameterDelta,
				AtLowerBound: atLower, AtUpperBound: atUpper,
			})
		}
	}
	for index := range result.Groups {
		metrics := &result.Groups[index]
		metrics.GradientL2 = math.Sqrt(metrics.GradientL2)
		metrics.UpdateL2 = math.Sqrt(metrics.UpdateL2)
		if metrics.ActiveParameters > 0 {
			metrics.RMSNormalizedDisplacement = math.Sqrt(groupSquaredDisplacement[index] / float64(metrics.ActiveParameters))
		}
	}
	if topN > 0 {
		sort.Slice(candidates, func(i, j int) bool {
			left := math.Abs(candidates[i].NormalizedDisplacement)
			right := math.Abs(candidates[j].NormalizedDisplacement)
			if left != right {
				return left > right
			}
			return candidates[i].Index < candidates[j].Index
		})
		if topN > len(candidates) {
			topN = len(candidates)
		}
		result.TopDisplacements = candidates[:topN]
	}
	return result, nil
}
