package tuner

import (
	"fmt"
	"strings"
)

// AxisSelection identifies one labelled coordinate value in a parameter
// table. Omitted axes act as wildcards when a policy is compiled.
type AxisSelection struct {
	Axis  string
	Label string
}

// At selects one semantic table-axis value.
func At(axis, label string) AxisSelection {
	return AxisSelection{Axis: axis, Label: label}
}

// ParameterTrainingOverride changes optimizer ownership for all cells matching
// its axis selections. With no selections it applies to the entire parameter.
type ParameterTrainingOverride struct {
	Parameter  ParameterID
	Selections []AxisSelection
	Trainable  bool
}

// Freeze removes the selected parameter coordinates from optimizer ownership.
func Freeze(parameter ParameterID, selections ...AxisSelection) ParameterTrainingOverride {
	return ParameterTrainingOverride{
		Parameter: parameter, Selections: append([]AxisSelection(nil), selections...), Trainable: false,
	}
}

// Train gives optimizer ownership to the selected parameter coordinates.
func Train(parameter ParameterID, selections ...AxisSelection) ParameterTrainingOverride {
	return ParameterTrainingOverride{
		Parameter: parameter, Selections: append([]AxisSelection(nil), selections...), Trainable: true,
	}
}

// TrainingDefault controls the starting optimizer ownership used by a human
// training policy. Keeping registry defaults preserves the structural
// continuous/frozen choices. Freezing eligible parameters removes every
// structurally trainable coordinate until a group or parameter rule restores
// it.
type TrainingDefault string

const (
	KeepRegistryTrainingDefaults TrainingDefault = ""
	FreezeEligibleParameters     TrainingDefault = "freeze_eligible"
)

// GroupTrainingOverride changes optimizer ownership for a semantic parameter
// family. A TrainGroup rule restores only coordinates that the structural
// registry originally marked trainable; deliberately frozen controls still
// require an explicit parameter-level Train rule.
type GroupTrainingOverride struct {
	Group     GroupID
	Trainable bool
}

// FreezeGroup removes a parameter group from optimizer ownership.
func FreezeGroup(group GroupID) GroupTrainingOverride {
	return GroupTrainingOverride{Group: group, Trainable: false}
}

// TrainGroup gives the optimizer ownership of the structurally eligible
// coordinates in a parameter group.
func TrainGroup(group GroupID) GroupTrainingOverride {
	return GroupTrainingOverride{Group: group, Trainable: true}
}

// TrainingPolicy is the human-facing optimizer-ownership configuration. The
// structural engine registry remains independent of these experiment choices.
// Rules are applied from broadest to most specific: Default, Groups in slice
// order, then Overrides in slice order.
type TrainingPolicy struct {
	Default   TrainingDefault
	Groups    []GroupTrainingOverride
	Overrides []ParameterTrainingOverride
}

// applyTrainingPolicy validates semantic selectors and expands them into the
// exact per-cell overrides consumed by NewRegistry. It does not mutate specs.
func applyTrainingPolicy(specs []ParameterSpec, policy TrainingPolicy) ([]ParameterSpec, error) {
	result := cloneSpecs(specs)
	byID := make(map[ParameterID]int, len(result))
	byGroup := make(map[GroupID][]int)
	structural := make([][]bool, len(result))
	trainable := make([][]bool, len(result))
	for index, spec := range result {
		if _, exists := byID[spec.ID]; exists {
			return nil, fmt.Errorf("duplicate parameter ID %q", spec.ID)
		}
		byID[spec.ID] = index
		byGroup[spec.Group] = append(byGroup[spec.Group], index)
		resolved, err := resolveCoordinateTrainability(spec)
		if err != nil {
			return nil, fmt.Errorf("parameter %q: resolve structural training ownership: %w", spec.ID, err)
		}
		structural[index] = resolved
		trainable[index] = append([]bool(nil), resolved...)
	}

	switch policy.Default {
	case KeepRegistryTrainingDefaults:
	case FreezeEligibleParameters:
		for specIndex, spec := range result {
			if spec.Training.Mode == TrainingDiscrete {
				continue
			}
			clear(trainable[specIndex])
		}
	default:
		return nil, fmt.Errorf("unknown training policy default %q", policy.Default)
	}

	for overrideIndex, override := range policy.Groups {
		if strings.TrimSpace(string(override.Group)) == "" {
			return nil, fmt.Errorf("training group override %d has no group ID", overrideIndex)
		}
		specIndexes, exists := byGroup[override.Group]
		if !exists {
			return nil, fmt.Errorf("training group override %d refers to unknown group %q", overrideIndex, override.Group)
		}
		for _, specIndex := range specIndexes {
			if result[specIndex].Training.Mode == TrainingDiscrete {
				continue
			}
			for cell := range trainable[specIndex] {
				trainable[specIndex][cell] = override.Trainable && structural[specIndex][cell]
			}
		}
	}

	for overrideIndex, override := range policy.Overrides {
		if strings.TrimSpace(string(override.Parameter)) == "" {
			return nil, fmt.Errorf("training policy override %d has no parameter ID", overrideIndex)
		}
		specIndex, exists := byID[override.Parameter]
		if !exists {
			return nil, fmt.Errorf("training policy override %d refers to unknown parameter %q", overrideIndex, override.Parameter)
		}
		spec := result[specIndex]
		if spec.Training.Mode == TrainingDiscrete {
			return nil, fmt.Errorf("training policy override %d cannot change discrete parameter %q", overrideIndex, override.Parameter)
		}

		selectedAxes, err := validatePolicySelections(spec, overrideIndex, override.Selections)
		if err != nil {
			return nil, err
		}
		matched := 0
		strides := rowMajorStrides(spec.Shape.Dimensions)
		for cell := 0; cell < spec.Shape.ElementCount(); cell++ {
			coordinate := coordinateFor(cell, spec.Shape.Dimensions, strides)
			if !policyCoordinateMatches(spec.Shape, coordinate, selectedAxes) {
				continue
			}
			trainable[specIndex][cell] = override.Trainable
			matched++
		}
		if matched == 0 {
			return nil, fmt.Errorf("training policy override %d for %q matches no coordinates", overrideIndex, override.Parameter)
		}
	}

	for specIndex := range result {
		spec := &result[specIndex]
		if spec.Training.Mode == TrainingDiscrete {
			continue
		}
		spec.Training.Overrides = nil
		defaultTrainable := spec.Training.Mode == TrainingContinuous
		strides := rowMajorStrides(spec.Shape.Dimensions)
		for cell, selected := range trainable[specIndex] {
			if selected == defaultTrainable {
				continue
			}
			coordinate := coordinateFor(cell, spec.Shape.Dimensions, strides)
			axisValues := make(map[string]string, len(spec.Shape.Axes))
			for axisIndex, axis := range spec.Shape.Axes {
				axisValues[axis.Name] = axis.Labels[coordinate[axisIndex]]
			}
			spec.Training.Overrides = append(spec.Training.Overrides, CoordinateTrainingOverride{
				AxisValues: axisValues,
				Trainable:  selected,
			})
		}
	}
	return result, nil
}

func validatePolicySelections(spec ParameterSpec, overrideIndex int, selections []AxisSelection) (map[int]string, error) {
	if len(spec.Shape.Axes) == 0 && len(selections) != 0 {
		return nil, fmt.Errorf("training policy override %d for scalar %q cannot select axes", overrideIndex, spec.ID)
	}
	axisIndexes := make(map[string]int, len(spec.Shape.Axes))
	for index, axis := range spec.Shape.Axes {
		axisIndexes[axis.Name] = index
	}
	selected := make(map[int]string, len(selections))
	for selectionIndex, selection := range selections {
		if strings.TrimSpace(selection.Axis) == "" {
			return nil, fmt.Errorf("training policy override %d selection %d has no axis", overrideIndex, selectionIndex)
		}
		axisIndex, exists := axisIndexes[selection.Axis]
		if !exists {
			return nil, fmt.Errorf(
				"training policy override %d for %q has unknown axis %q",
				overrideIndex, spec.ID, selection.Axis,
			)
		}
		if _, exists := selected[axisIndex]; exists {
			return nil, fmt.Errorf(
				"training policy override %d for %q selects axis %q more than once",
				overrideIndex, spec.ID, selection.Axis,
			)
		}
		if strings.TrimSpace(selection.Label) == "" {
			return nil, fmt.Errorf(
				"training policy override %d for %q has an empty label for axis %q",
				overrideIndex, spec.ID, selection.Axis,
			)
		}
		if !containsLabel(spec.Shape.Axes[axisIndex].Labels, selection.Label) {
			return nil, fmt.Errorf(
				"training policy override %d for %q has unknown label %q for axis %q",
				overrideIndex, spec.ID, selection.Label, selection.Axis,
			)
		}
		selected[axisIndex] = selection.Label
	}
	return selected, nil
}

func policyCoordinateMatches(shape Shape, coordinate []int, selected map[int]string) bool {
	for axisIndex, label := range selected {
		if shape.Axes[axisIndex].Labels[coordinate[axisIndex]] != label {
			return false
		}
	}
	return true
}

func containsLabel(labels []string, want string) bool {
	for _, label := range labels {
		if label == want {
			return true
		}
	}
	return false
}
