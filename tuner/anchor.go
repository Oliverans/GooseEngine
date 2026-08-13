package tuner

import (
	"errors"
	"fmt"
	"math"
)

// ParameterAnchorConfig optionally replaces registry group strengths without
// changing the registry layout or dataset fingerprint. Omitted groups retain
// their registry strength.
type ParameterAnchorConfig struct {
	GroupStrengths map[GroupID]float64
}

type anchorCoordinate struct {
	parameterIndex int
	trainIndex     int
	groupIndex     int
	anchor         float64
	deviationScale float64
	strengthScale  float64
}

type anchorGroup struct {
	id       GroupID
	strength float64
	active   int
}

// ParameterAnchorModel is a precompiled L2-to-anchor penalty. Ordinary
// zero-directed L2/weight decay is intentionally not part of this model.
type ParameterAnchorModel struct {
	parameterCount   int
	trainableCount   int
	coordinates      []anchorCoordinate
	groups           []anchorGroup
	frozenParameters int
	uncovered        int
}

func NewParameterAnchorModel(registry *Registry, coverage []uint64, config ParameterAnchorConfig) (*ParameterAnchorModel, error) {
	if registry == nil {
		return nil, errors.New("parameter anchoring requires a registry")
	}
	if coverage != nil && len(coverage) != len(registry.Elements) {
		return nil, fmt.Errorf("coverage vector has length %d, want %d", len(coverage), len(registry.Elements))
	}
	groupIndexes := make(map[GroupID]int, len(registry.Groups))
	model := &ParameterAnchorModel{
		parameterCount: len(registry.Elements),
		trainableCount: registry.TrainableCount(),
		groups:         make([]anchorGroup, len(registry.Groups)),
	}
	for index, group := range registry.Groups {
		strength := group.AnchorStrength
		if override, exists := config.GroupStrengths[group.ID]; exists {
			strength = override
		}
		if !finite(strength) || strength < 0 {
			return nil, fmt.Errorf("anchor strength for group %q must be finite and non-negative, got %v", group.ID, strength)
		}
		groupIndexes[group.ID] = index
		model.groups[index] = anchorGroup{id: group.ID, strength: strength}
	}
	for group := range config.GroupStrengths {
		if _, exists := groupIndexes[group]; !exists {
			return nil, fmt.Errorf("anchor strength override refers to unknown group %q", group)
		}
	}

	model.coordinates = make([]anchorCoordinate, 0, registry.TrainableCount())
	for _, element := range registry.Elements {
		if element.Mode != TrainingContinuous || element.TrainIndex == NoTrainIndex {
			model.frozenParameters++
			continue
		}
		if coverage != nil && coverage[element.Index] == 0 {
			model.uncovered++
			continue
		}
		spec := registry.Specs[element.SpecIndex]
		groupIndex, exists := groupIndexes[spec.Group]
		if !exists {
			return nil, fmt.Errorf("parameter %q refers to unknown anchor group %q", element.ID, spec.Group)
		}
		model.groups[groupIndex].active++
		model.coordinates = append(model.coordinates, anchorCoordinate{
			parameterIndex: element.Index,
			trainIndex:     element.TrainIndex,
			groupIndex:     groupIndex,
			anchor:         element.Anchor,
			deviationScale: element.DeviationScale,
			strengthScale:  spec.Prior.StrengthScale,
		})
	}
	return model, nil
}

type AnchorGroupMetrics struct {
	Group                             GroupID
	Strength                          float64
	ActiveParameters                  int
	Loss                              float64
	MeanSquaredNormalizedDisplacement float64
	MaxAbsoluteNormalizedDisplacement float64
}

type ParameterAnchorMetrics struct {
	Loss                              float64
	ActiveParameters                  int
	FrozenParameters                  int
	UncoveredParameters               int
	MaxAbsoluteNormalizedDisplacement float64
	Groups                            []AnchorGroupMetrics
}

// Evaluate returns the normalized anchor penalty and optionally adds its
// derivative to a dense train-coordinate gradient. Passing nil reports loss
// without modifying a gradient.
func (m *ParameterAnchorModel) Evaluate(parameters []float64, trainGradient []float64) (ParameterAnchorMetrics, error) {
	if err := m.validateInputs(parameters, trainGradient); err != nil {
		return ParameterAnchorMetrics{}, err
	}
	metrics := ParameterAnchorMetrics{
		ActiveParameters: m.activeCount(), FrozenParameters: m.frozenParameters,
		UncoveredParameters: m.uncovered,
		Groups:              make([]AnchorGroupMetrics, len(m.groups)),
	}
	for index, group := range m.groups {
		metrics.Groups[index] = AnchorGroupMetrics{
			Group: group.id, Strength: group.strength, ActiveParameters: group.active,
		}
	}
	for _, coordinate := range m.coordinates {
		value := parameters[coordinate.parameterIndex]
		group := m.groups[coordinate.groupIndex]
		groupMetrics := &metrics.Groups[coordinate.groupIndex]
		normalized := (value - coordinate.anchor) / coordinate.deviationScale
		absolute := math.Abs(normalized)
		groupMetrics.MeanSquaredNormalizedDisplacement += normalized * normalized
		groupMetrics.MaxAbsoluteNormalizedDisplacement = max(groupMetrics.MaxAbsoluteNormalizedDisplacement, absolute)
		metrics.MaxAbsoluteNormalizedDisplacement = max(metrics.MaxAbsoluteNormalizedDisplacement, absolute)
		if group.active == 0 || group.strength == 0 || coordinate.strengthScale == 0 {
			continue
		}
		coefficient := group.strength * coordinate.strengthScale / float64(group.active)
		penalty := coefficient * normalized * normalized
		groupMetrics.Loss += penalty
		metrics.Loss += penalty
		if trainGradient != nil {
			trainGradient[coordinate.trainIndex] += 2 * coefficient *
				(value - coordinate.anchor) / (coordinate.deviationScale * coordinate.deviationScale)
		}
	}
	for index, group := range m.groups {
		if group.active != 0 {
			metrics.Groups[index].MeanSquaredNormalizedDisplacement /= float64(group.active)
		}
	}
	if !finite(metrics.Loss) {
		return ParameterAnchorMetrics{}, errors.New("parameter-anchor loss is not finite")
	}
	return metrics, nil
}

// Penalty is the allocation-free hot-loop form. It returns the same total loss
// as Evaluate and optionally adds the anchor derivative to a dense train
// gradient, but omits diagnostic group reporting.
func (m *ParameterAnchorModel) Penalty(parameters []float64, trainGradient []float64) (float64, error) {
	if err := m.validateInputs(parameters, trainGradient); err != nil {
		return 0, err
	}
	loss := 0.0
	for _, coordinate := range m.coordinates {
		group := m.groups[coordinate.groupIndex]
		if group.active == 0 || group.strength == 0 || coordinate.strengthScale == 0 {
			continue
		}
		value := parameters[coordinate.parameterIndex]
		difference := value - coordinate.anchor
		coefficient := group.strength * coordinate.strengthScale / float64(group.active)
		loss += coefficient * difference * difference / (coordinate.deviationScale * coordinate.deviationScale)
		if trainGradient != nil {
			trainGradient[coordinate.trainIndex] += 2 * coefficient * difference /
				(coordinate.deviationScale * coordinate.deviationScale)
		}
	}
	if !finite(loss) {
		return 0, errors.New("parameter-anchor loss is not finite")
	}
	return loss, nil
}

func (m *ParameterAnchorModel) validateInputs(parameters []float64, trainGradient []float64) error {
	if m == nil {
		return errors.New("cannot evaluate a nil parameter-anchor model")
	}
	if len(parameters) != m.parameterCount {
		return fmt.Errorf("parameter vector has length %d, want %d", len(parameters), m.parameterCount)
	}
	if trainGradient != nil && len(trainGradient) != m.trainableCount {
		return fmt.Errorf("train gradient has length %d, want %d", len(trainGradient), m.trainableCount)
	}
	for _, coordinate := range m.coordinates {
		if !finite(parameters[coordinate.parameterIndex]) {
			return fmt.Errorf("parameter %d is not finite", coordinate.parameterIndex)
		}
	}
	return nil
}

func (m *ParameterAnchorModel) activeCount() int {
	if m == nil {
		return 0
	}
	return len(m.coordinates)
}

// ActiveTrainIndexes returns the continuous optimizer coordinates not removed
// by coverage freezing.
func (m *ParameterAnchorModel) ActiveTrainIndexes() []int {
	if m == nil {
		return nil
	}
	indexes := make([]int, len(m.coordinates))
	for index, coordinate := range m.coordinates {
		indexes[index] = coordinate.trainIndex
	}
	return indexes
}
