package tuner

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
)

// NoTrainIndex marks values that are present in the forward parameter vector
// but are not ordinary Adam coordinates.
const NoTrainIndex = -1

// ParameterElement is one generated scalar cell from a ParameterSpec.
// Index addresses the complete forward vector. TrainIndex addresses the dense
// continuous-Adam vector, or is NoTrainIndex for frozen/discrete values.
type ParameterElement struct {
	Index             int
	TrainIndex        int
	SpecIndex         int
	ID                ParameterID
	Coordinate        []int
	CoordinateLabels  []string
	Initial           float64
	Anchor            float64
	DeviationScale    float64
	AnchorStrength    float64
	LearningRateScale float64
	ExportIndex       int
	Bounds            Bounds
	Mode              TrainingMode
}

// ParameterHandle is resolved once from an ID and then used for direct vector
// access. Offset and indexes returned by Index address the complete forward
// parameter vector, not the Adam-state vector.
type ParameterHandle struct {
	ID         ParameterID
	SpecIndex  int
	Offset     int
	Length     int
	Dimensions []int
	Strides    []int
}

// Index converts semantic table coordinates to a row-major forward-vector
// index. Scalars accept no coordinates.
func (h ParameterHandle) Index(coordinates ...int) (int, error) {
	if len(coordinates) != len(h.Dimensions) {
		return 0, fmt.Errorf("parameter %q: got %d coordinates, want %d", h.ID, len(coordinates), len(h.Dimensions))
	}

	index := h.Offset
	for axis, coordinate := range coordinates {
		if coordinate < 0 || coordinate >= h.Dimensions[axis] {
			return 0, fmt.Errorf(
				"parameter %q: coordinate %d on axis %d is outside [0,%d)",
				h.ID,
				coordinate,
				axis,
				h.Dimensions[axis],
			)
		}
		index += coordinate * h.Strides[axis]
	}
	return index, nil
}

// MustIndex is the initialization-time convenience form of Index.
func (h ParameterHandle) MustIndex(coordinates ...int) int {
	index, err := h.Index(coordinates...)
	if err != nil {
		panic(err)
	}
	return index
}

// Registry is the validated, deterministic expansion of logical parameter
// definitions. Specs are sorted by stable ID; table cells use row-major order.
type Registry struct {
	Version     string
	Groups      []GroupSpec
	Specs       []ParameterSpec
	Elements    []ParameterElement
	Fingerprint string

	handles        map[ParameterID]ParameterHandle
	trainableCount int
}

// NewRegistry validates and expands logical definitions. Caller-owned slices
// are copied before canonical sorting.
func NewRegistry(version string, groups []GroupSpec, specs []ParameterSpec) (*Registry, error) {
	if strings.TrimSpace(version) == "" {
		return nil, errors.New("registry version is required")
	}

	registry := &Registry{
		Version: version,
		Groups:  cloneGroups(groups),
		Specs:   cloneSpecs(specs),
		handles: make(map[ParameterID]ParameterHandle, len(specs)),
	}

	sort.Slice(registry.Groups, func(i, j int) bool {
		return registry.Groups[i].ID < registry.Groups[j].ID
	})
	sort.Slice(registry.Specs, func(i, j int) bool {
		return registry.Specs[i].ID < registry.Specs[j].ID
	})

	groupByID, err := validateGroups(registry.Groups)
	if err != nil {
		return nil, err
	}
	if err := registry.expand(groupByID); err != nil {
		return nil, err
	}

	fingerprint, err := registry.layoutFingerprint()
	if err != nil {
		return nil, fmt.Errorf("fingerprint registry layout: %w", err)
	}
	registry.Fingerprint = fingerprint
	return registry, nil
}

// Resolve performs the one-time semantic lookup used to cache a hot-loop
// handle.
func (r *Registry) Resolve(id ParameterID) (ParameterHandle, bool) {
	if r == nil {
		return ParameterHandle{}, false
	}
	handle, ok := r.handles[id]
	return handle, ok
}

// TrainableCount returns the number of continuous Adam coordinates.
func (r *Registry) TrainableCount() int {
	if r == nil {
		return 0
	}
	return r.trainableCount
}

// InitialValues returns a fresh complete forward parameter vector.
func (r *Registry) InitialValues() []float64 {
	if r == nil {
		return nil
	}
	values := make([]float64, len(r.Elements))
	for i := range r.Elements {
		values[i] = r.Elements[i].Initial
	}
	return values
}

func (r *Registry) expand(groups map[GroupID]GroupSpec) error {
	seenIDs := make(map[ParameterID]struct{}, len(r.Specs))
	seenEngineNames := make(map[string]ParameterID, len(r.Specs))
	trainIndex := 0

	for specIndex := range r.Specs {
		spec := &r.Specs[specIndex]
		if _, exists := seenIDs[spec.ID]; exists {
			return fmt.Errorf("duplicate parameter ID %q", spec.ID)
		}
		seenIDs[spec.ID] = struct{}{}

		if err := validateSpec(*spec, groups); err != nil {
			return fmt.Errorf("parameter %q: %w", spec.ID, err)
		}
		trainableCells, err := resolveCoordinateTrainability(*spec)
		if err != nil {
			return fmt.Errorf("parameter %q: %w", spec.ID, err)
		}
		group := groups[spec.Group]
		anchorStrength := group.AnchorStrength * spec.Prior.StrengthScale
		learningRateScale := group.LearningRateScale * spec.Training.LearningRateScale
		if !finite(anchorStrength) {
			return fmt.Errorf("parameter %q: effective anchor strength is not finite", spec.ID)
		}
		if !finite(learningRateScale) {
			return fmt.Errorf("parameter %q: effective learning-rate scale is not finite", spec.ID)
		}
		if previous, exists := seenEngineNames[spec.EngineName]; exists {
			return fmt.Errorf("parameter %q: engine name %q is already used by %q", spec.ID, spec.EngineName, previous)
		}
		seenEngineNames[spec.EngineName] = spec.ID

		size := spec.Shape.ElementCount()
		initial, err := expandValues(spec.Initial, size, "initial")
		if err != nil {
			return fmt.Errorf("parameter %q: %w", spec.ID, err)
		}
		anchors, err := expandValues(spec.Prior.Anchor, size, "anchor")
		if err != nil {
			return fmt.Errorf("parameter %q: %w", spec.ID, err)
		}
		scales, err := expandValues(spec.Prior.DeviationScale, size, "deviation scale")
		if err != nil {
			return fmt.Errorf("parameter %q: %w", spec.ID, err)
		}

		offset := len(r.Elements)
		strides := rowMajorStrides(spec.Shape.Dimensions)
		r.handles[spec.ID] = ParameterHandle{
			ID:         spec.ID,
			SpecIndex:  specIndex,
			Offset:     offset,
			Length:     size,
			Dimensions: append([]int(nil), spec.Shape.Dimensions...),
			Strides:    strides,
		}

		for cell := 0; cell < size; cell++ {
			if scales[cell] <= 0 {
				return fmt.Errorf("parameter %q: deviation scale at cell %d must be positive", spec.ID, cell)
			}
			if err := validateBoundedValue(spec.Training.Bounds, initial[cell], "initial", cell); err != nil {
				return fmt.Errorf("parameter %q: %w", spec.ID, err)
			}
			if err := validateBoundedValue(spec.Training.Bounds, anchors[cell], "anchor", cell); err != nil {
				return fmt.Errorf("parameter %q: %w", spec.ID, err)
			}
			if spec.Training.Mode == TrainingDiscrete && math.Trunc(initial[cell]) != initial[cell] {
				return fmt.Errorf("parameter %q: discrete initial value at cell %d is not an integer", spec.ID, cell)
			}

			coordinate := coordinateFor(cell, spec.Shape.Dimensions, strides)
			exportIndex := exportIndexFor(spec.Export, spec.Shape, coordinate)
			elementMode := spec.Training.Mode
			elementTrainIndex := NoTrainIndex
			if trainableCells[cell] {
				elementMode = TrainingContinuous
				elementTrainIndex = trainIndex
				trainIndex++
			} else if spec.Training.Mode != TrainingDiscrete {
				elementMode = TrainingFrozen
			}
			r.Elements = append(r.Elements, ParameterElement{
				Index:             offset + cell,
				TrainIndex:        elementTrainIndex,
				SpecIndex:         specIndex,
				ID:                spec.ID,
				Coordinate:        coordinate,
				CoordinateLabels:  coordinateLabels(spec.Shape, coordinate),
				Initial:           initial[cell],
				Anchor:            anchors[cell],
				DeviationScale:    scales[cell],
				AnchorStrength:    anchorStrength,
				LearningRateScale: learningRateScale,
				ExportIndex:       exportIndex,
				Bounds:            spec.Training.Bounds,
				Mode:              elementMode,
			})
		}
	}

	r.trainableCount = trainIndex
	return nil
}

func validateGroups(groups []GroupSpec) (map[GroupID]GroupSpec, error) {
	byID := make(map[GroupID]GroupSpec, len(groups))
	for _, group := range groups {
		if strings.TrimSpace(string(group.ID)) == "" {
			return nil, errors.New("parameter group ID is required")
		}
		if _, exists := byID[group.ID]; exists {
			return nil, fmt.Errorf("duplicate parameter group ID %q", group.ID)
		}
		if !finite(group.AnchorStrength) || group.AnchorStrength < 0 {
			return nil, fmt.Errorf("parameter group %q: anchor strength must be finite and non-negative", group.ID)
		}
		if !finite(group.LearningRateScale) || group.LearningRateScale <= 0 {
			return nil, fmt.Errorf("parameter group %q: learning-rate scale must be finite and positive", group.ID)
		}
		byID[group.ID] = group
	}
	return byID, nil
}

func validateSpec(spec ParameterSpec, groups map[GroupID]GroupSpec) error {
	if strings.TrimSpace(string(spec.ID)) == "" {
		return errors.New("stable ID is required")
	}
	if strings.TrimSpace(spec.EngineName) == "" {
		return errors.New("engine name is required")
	}
	if _, exists := groups[spec.Group]; !exists {
		return fmt.Errorf("unknown parameter group %q", spec.Group)
	}
	if !validPhase(spec.Phase) {
		return fmt.Errorf("unknown phase %q", spec.Phase)
	}
	if !validFormula(spec.Formula) {
		return fmt.Errorf("unknown formula %q", spec.Formula)
	}
	if !validRole(spec.Role) {
		return fmt.Errorf("unknown role %q", spec.Role)
	}
	if err := validateShape(spec.Shape); err != nil {
		return err
	}
	if !validTrainingMode(spec.Training.Mode) {
		return fmt.Errorf("unknown training mode %q", spec.Training.Mode)
	}
	if !finite(spec.Training.LearningRateScale) || spec.Training.LearningRateScale <= 0 {
		return errors.New("learning-rate scale must be finite and positive")
	}
	if len(spec.Training.Overrides) != 0 && spec.Training.Mode == TrainingDiscrete {
		return errors.New("coordinate training overrides are not valid for discrete parameters")
	}
	if !finite(spec.Prior.StrengthScale) || spec.Prior.StrengthScale < 0 {
		return errors.New("prior strength scale must be finite and non-negative")
	}
	if err := validateBounds(spec.Training.Bounds); err != nil {
		return err
	}
	if strings.TrimSpace(spec.Export.GoSymbol) == "" {
		return errors.New("export Go symbol is required")
	}
	if !validEngineType(spec.Export.GoType) {
		return fmt.Errorf("unknown export Go type %q", spec.Export.GoType)
	}
	if !validRounding(spec.Export.Rounding) {
		return fmt.Errorf("unknown rounding policy %q", spec.Export.Rounding)
	}
	if err := validateExportLayout(spec.Shape, spec.Export); err != nil {
		return err
	}
	return nil
}

func resolveCoordinateTrainability(spec ParameterSpec) ([]bool, error) {
	size := spec.Shape.ElementCount()
	trainable := make([]bool, size)
	defaultTrainable := spec.Training.Mode == TrainingContinuous
	for cell := range trainable {
		trainable[cell] = defaultTrainable
	}
	if len(spec.Training.Overrides) == 0 {
		return trainable, nil
	}
	strides := rowMajorStrides(spec.Shape.Dimensions)
	seen := make(map[int]int, len(spec.Training.Overrides))
	for overrideIndex, override := range spec.Training.Overrides {
		if len(override.AxisValues) != len(spec.Shape.Axes) {
			return nil, fmt.Errorf(
				"training override %d names %d axes, want %d",
				overrideIndex, len(override.AxisValues), len(spec.Shape.Axes),
			)
		}
		cell := 0
		for axisIndex, axis := range spec.Shape.Axes {
			label, exists := override.AxisValues[axis.Name]
			if !exists {
				return nil, fmt.Errorf("training override %d does not name axis %q", overrideIndex, axis.Name)
			}
			coordinate := -1
			for candidate, candidateLabel := range axis.Labels {
				if candidateLabel == label {
					coordinate = candidate
					break
				}
			}
			if coordinate == -1 {
				return nil, fmt.Errorf(
					"training override %d has unknown label %q for axis %q",
					overrideIndex, label, axis.Name,
				)
			}
			cell += coordinate * strides[axisIndex]
		}
		if previous, exists := seen[cell]; exists {
			return nil, fmt.Errorf("training override %d duplicates override %d", overrideIndex, previous)
		}
		seen[cell] = overrideIndex
		trainable[cell] = override.Trainable
	}
	return trainable, nil
}

func validateExportLayout(shape Shape, export ExportSpec) error {
	dimensions := export.StorageDimensions
	if len(dimensions) == 0 {
		dimensions = shape.Dimensions
	}
	storageSize := 1
	for i, dimension := range dimensions {
		if dimension <= 0 {
			return fmt.Errorf("export storage dimension %d must be positive", i)
		}
		storageSize *= dimension
	}
	if export.StorageOffset < 0 || export.StorageOffset >= storageSize {
		return fmt.Errorf("export storage offset %d is outside [0,%d)", export.StorageOffset, storageSize)
	}

	strides := export.StorageStrides
	if len(strides) == 0 {
		strides = rowMajorStrides(shape.Dimensions)
	}
	if len(strides) != len(shape.Dimensions) {
		return fmt.Errorf("export has %d strides for %d parameter dimensions", len(strides), len(shape.Dimensions))
	}
	maxIndex := export.StorageOffset
	for i, stride := range strides {
		if stride <= 0 {
			return fmt.Errorf("export stride %d must be positive", i)
		}
		maxIndex += (shape.Dimensions[i] - 1) * stride
	}
	if maxIndex >= storageSize {
		return fmt.Errorf("export layout reaches storage index %d, outside [0,%d)", maxIndex, storageSize)
	}
	return nil
}

func validateShape(shape Shape) error {
	if len(shape.Dimensions) == 0 {
		if len(shape.Axes) != 0 {
			return errors.New("scalar shape cannot define axes")
		}
		return nil
	}
	if len(shape.Axes) != len(shape.Dimensions) {
		return fmt.Errorf("shape has %d dimensions but %d axes", len(shape.Dimensions), len(shape.Axes))
	}

	axisNames := make(map[string]struct{}, len(shape.Axes))
	for i, dimension := range shape.Dimensions {
		if dimension <= 0 {
			return fmt.Errorf("shape dimension %d must be positive", i)
		}
		axis := shape.Axes[i]
		if strings.TrimSpace(axis.Name) == "" {
			return fmt.Errorf("shape axis %d has no name", i)
		}
		if _, exists := axisNames[axis.Name]; exists {
			return fmt.Errorf("duplicate shape axis name %q", axis.Name)
		}
		axisNames[axis.Name] = struct{}{}
		if len(axis.Labels) != dimension {
			return fmt.Errorf("shape axis %q has %d labels, want %d", axis.Name, len(axis.Labels), dimension)
		}
		labels := make(map[string]struct{}, len(axis.Labels))
		for _, label := range axis.Labels {
			if strings.TrimSpace(label) == "" {
				return fmt.Errorf("shape axis %q contains an empty label", axis.Name)
			}
			if _, exists := labels[label]; exists {
				return fmt.Errorf("shape axis %q contains duplicate label %q", axis.Name, label)
			}
			labels[label] = struct{}{}
		}
	}
	return nil
}

func expandValues(spec ValueSpec, size int, field string) ([]float64, error) {
	if len(spec.Values) != 1 && len(spec.Values) != size {
		return nil, fmt.Errorf("%s has %d values, want 1 or %d", field, len(spec.Values), size)
	}
	values := make([]float64, size)
	if len(spec.Values) == 1 {
		for i := range values {
			values[i] = spec.Values[0]
		}
	} else {
		copy(values, spec.Values)
	}
	for i, value := range values {
		if !finite(value) {
			return nil, fmt.Errorf("%s at cell %d must be finite", field, i)
		}
	}
	return values, nil
}

func validateBounds(bounds Bounds) error {
	if bounds.Lower.Set && !finite(bounds.Lower.Value) {
		return errors.New("lower bound must be finite")
	}
	if bounds.Upper.Set && !finite(bounds.Upper.Value) {
		return errors.New("upper bound must be finite")
	}
	if bounds.Lower.Set && bounds.Upper.Set && bounds.Lower.Value > bounds.Upper.Value {
		return errors.New("lower bound exceeds upper bound")
	}
	return nil
}

func validateBoundedValue(bounds Bounds, value float64, field string, cell int) error {
	if bounds.Lower.Set && value < bounds.Lower.Value {
		return fmt.Errorf("%s at cell %d is below lower bound", field, cell)
	}
	if bounds.Upper.Set && value > bounds.Upper.Value {
		return fmt.Errorf("%s at cell %d is above upper bound", field, cell)
	}
	return nil
}

func rowMajorStrides(dimensions []int) []int {
	strides := make([]int, len(dimensions))
	stride := 1
	for i := len(dimensions) - 1; i >= 0; i-- {
		strides[i] = stride
		stride *= dimensions[i]
	}
	return strides
}

func coordinateFor(cell int, dimensions, strides []int) []int {
	coordinate := make([]int, len(dimensions))
	for i := range dimensions {
		coordinate[i] = (cell / strides[i]) % dimensions[i]
	}
	return coordinate
}

func coordinateLabels(shape Shape, coordinate []int) []string {
	labels := make([]string, len(coordinate))
	for i := range coordinate {
		labels[i] = shape.Axes[i].Labels[coordinate[i]]
	}
	return labels
}

func exportIndexFor(export ExportSpec, shape Shape, coordinate []int) int {
	strides := export.StorageStrides
	if len(strides) == 0 {
		strides = rowMajorStrides(shape.Dimensions)
	}
	index := export.StorageOffset
	for i := range coordinate {
		index += coordinate[i] * strides[i]
	}
	return index
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func validPhase(value PhaseKind) bool {
	switch value {
	case PhaseMG, PhaseEG, PhaseShared, PhaseDerived, PhaseFinal:
		return true
	default:
		return false
	}
}

func validFormula(value FormulaKind) bool {
	switch value {
	case FormulaLinear, FormulaConnectedPawn, FormulaCandidatePasser, FormulaCenterScale,
		FormulaSpace, FormulaKingDanger, FormulaKingPasser, FormulaFinalScale:
		return true
	case FormulaImbalance:
		return true
	default:
		return false
	}
}

func validRole(value ParameterRole) bool {
	switch value {
	case RoleCoefficient, RolePercentage, RoleDivisor, RoleOffset, RoleCap, RoleReference, RoleFinalDivider:
		return true
	default:
		return false
	}
}

func validTrainingMode(value TrainingMode) bool {
	switch value {
	case TrainingContinuous, TrainingFrozen, TrainingDiscrete:
		return true
	default:
		return false
	}
}

func validEngineType(value EngineValueType) bool {
	switch value {
	case EngineInt, EngineInt16, EngineInt32, EngineInt64:
		return true
	default:
		return false
	}
}

func validRounding(value RoundingPolicy) bool {
	switch value {
	case RoundNearest, RoundTowardZero, RoundFloor, RoundCeil:
		return true
	default:
		return false
	}
}

type fingerprintSpec struct {
	ID         ParameterID
	EngineName string
	Group      GroupID
	Shape      Shape
	Phase      PhaseKind
	Formula    FormulaKind
	Role       ParameterRole
	Mode       TrainingMode
	Export     ExportSpec
}

func (r *Registry) layoutFingerprint() (string, error) {
	definition := struct {
		Version string
		Specs   []fingerprintSpec
	}{
		Version: r.Version,
		Specs:   make([]fingerprintSpec, len(r.Specs)),
	}
	for i, spec := range r.Specs {
		definition.Specs[i] = fingerprintSpec{
			ID:         spec.ID,
			EngineName: spec.EngineName,
			Group:      spec.Group,
			Shape:      spec.Shape,
			Phase:      spec.Phase,
			Formula:    spec.Formula,
			Role:       spec.Role,
			Mode:       spec.Training.Mode,
			Export:     spec.Export,
		}
	}

	encoded, err := json.Marshal(definition)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(encoded)
	return hex.EncodeToString(hash[:]), nil
}

func cloneGroups(groups []GroupSpec) []GroupSpec {
	return append([]GroupSpec(nil), groups...)
}

func cloneSpecs(specs []ParameterSpec) []ParameterSpec {
	cloned := make([]ParameterSpec, len(specs))
	for i := range specs {
		cloned[i] = specs[i]
		cloned[i].Shape.Dimensions = append([]int(nil), specs[i].Shape.Dimensions...)
		cloned[i].Shape.Axes = cloneAxes(specs[i].Shape.Axes)
		cloned[i].Initial.Values = append([]float64(nil), specs[i].Initial.Values...)
		cloned[i].Prior.Anchor.Values = append([]float64(nil), specs[i].Prior.Anchor.Values...)
		cloned[i].Prior.DeviationScale.Values = append([]float64(nil), specs[i].Prior.DeviationScale.Values...)
		cloned[i].Training.Overrides = make([]CoordinateTrainingOverride, len(specs[i].Training.Overrides))
		for j, override := range specs[i].Training.Overrides {
			cloned[i].Training.Overrides[j] = CoordinateTrainingOverride{
				AxisValues: make(map[string]string, len(override.AxisValues)),
				Trainable:  override.Trainable,
			}
			for axis, value := range override.AxisValues {
				cloned[i].Training.Overrides[j].AxisValues[axis] = value
			}
		}
		cloned[i].Export.StorageDimensions = append([]int(nil), specs[i].Export.StorageDimensions...)
		cloned[i].Export.StorageStrides = append([]int(nil), specs[i].Export.StorageStrides...)
	}
	return cloned
}
