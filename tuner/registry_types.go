package tuner

// ParameterID is the stable semantic identity of a parameter specification.
// IDs are independent of generated numeric vector positions.
type ParameterID string

// GroupID identifies a collection of parameters that share tuning defaults.
type GroupID string

// PhaseKind describes where a parameter participates in tapered evaluation.
type PhaseKind string

const (
	PhaseMG      PhaseKind = "mg"
	PhaseEG      PhaseKind = "eg"
	PhaseShared  PhaseKind = "shared"
	PhaseDerived PhaseKind = "derived"
	PhaseFinal   PhaseKind = "final"
)

// FormulaKind identifies the forward-model calculation that consumes a
// parameter. It is not merely debug metadata: it selects the reconstruction
// and derivative rules used by the tuner.
type FormulaKind string

const (
	FormulaLinear          FormulaKind = "linear"
	FormulaConnectedPawn   FormulaKind = "connected_pawn"
	FormulaCandidatePasser FormulaKind = "candidate_passer"
	FormulaCenterScale     FormulaKind = "center_scale"
	FormulaSpace           FormulaKind = "space"
	FormulaKingDanger      FormulaKind = "king_danger"
	FormulaKingPasser      FormulaKind = "king_passer"
	FormulaImbalance       FormulaKind = "imbalance"
	FormulaFinalScale      FormulaKind = "final_scale"
)

// ParameterRole describes how a value participates inside its formula. Roles
// allow validation and sensible training defaults without encoding the whole
// calculation in the parameter definition.
type ParameterRole string

const (
	RoleCoefficient  ParameterRole = "coefficient"
	RolePercentage   ParameterRole = "percentage"
	RoleDivisor      ParameterRole = "divisor"
	RoleOffset       ParameterRole = "offset"
	RoleCap          ParameterRole = "cap"
	RoleReference    ParameterRole = "reference"
	RoleFinalDivider ParameterRole = "final_divider"
)

// TrainingMode determines which optimization mechanism, if any, owns a value.
type TrainingMode string

const (
	TrainingContinuous TrainingMode = "continuous"
	TrainingFrozen     TrainingMode = "frozen"
	TrainingDiscrete   TrainingMode = "discrete"
)

// EngineValueType is the integer type used by the engine export target.
type EngineValueType string

const (
	EngineInt   EngineValueType = "int"
	EngineInt16 EngineValueType = "int16"
	EngineInt32 EngineValueType = "int32"
	EngineInt64 EngineValueType = "int64"
)

// RoundingPolicy defines how a continuous value is quantized for engine use.
type RoundingPolicy string

const (
	RoundNearest    RoundingPolicy = "nearest"
	RoundTowardZero RoundingPolicy = "toward_zero"
	RoundFloor      RoundingPolicy = "floor"
	RoundCeil       RoundingPolicy = "ceil"
)

// AxisSpec assigns semantic meaning to one table dimension. Labels are also
// used in diagnostics and generated element names.
type AxisSpec struct {
	Name   string
	Labels []string
}

// Shape describes a scalar (no dimensions) or a row-major parameter table.
// TableShape should normally be used so Dimensions and Axes cannot drift.
type Shape struct {
	Dimensions []int
	Axes       []AxisSpec
}

// ScalarShape returns the shape of a scalar parameter.
func ScalarShape() Shape {
	return Shape{}
}

// TableShape builds a table shape from named axes.
func TableShape(axes ...AxisSpec) Shape {
	shape := Shape{
		Dimensions: make([]int, len(axes)),
		Axes:       cloneAxes(axes),
	}
	for i := range axes {
		shape.Dimensions[i] = len(axes[i].Labels)
	}
	return shape
}

// ElementCount returns the number of scalar values represented by the shape.
func (s Shape) ElementCount() int {
	if len(s.Dimensions) == 0 {
		return 1
	}

	total := 1
	for _, dimension := range s.Dimensions {
		if dimension <= 0 {
			return 0
		}
		total *= dimension
	}
	return total
}

// ValueSpec contains either one broadcast value or one value per shape cell.
type ValueSpec struct {
	Values []float64
}

// BroadcastValue constructs a value that applies to every cell in a spec.
func BroadcastValue(value float64) ValueSpec {
	return ValueSpec{Values: []float64{value}}
}

// ElementValues constructs explicit row-major values for a spec.
func ElementValues(values ...float64) ValueSpec {
	return ValueSpec{Values: append([]float64(nil), values...)}
}

// Limit is an optional numeric boundary. Set distinguishes an absent boundary
// from a boundary whose value is zero.
type Limit struct {
	Value float64
	Set   bool
}

// Bounds contains optional inclusive lower and upper limits.
type Bounds struct {
	Lower Limit
	Upper Limit
}

// ClosedBounds constructs inclusive lower and upper limits.
func ClosedBounds(lower, upper float64) Bounds {
	return Bounds{
		Lower: Limit{Value: lower, Set: true},
		Upper: Limit{Value: upper, Set: true},
	}
}

// PriorSpec describes normalized L2-to-anchor behavior. This penalizes
// deviation from Anchor; it is not zero-directed L2 weight decay.
// StrengthScale multiplies the owning group's anchor strength, and
// DeviationScale supplies the normalization.
type PriorSpec struct {
	Anchor         ValueSpec
	DeviationScale ValueSpec
	StrengthScale  float64
}

// CoordinateTrainingOverride changes optimizer ownership for one labelled
// table cell. AxisValues must name every axis in the owning ParameterSpec.
// Overrides are resolved once while the registry is expanded.
type CoordinateTrainingOverride struct {
	AxisValues map[string]string
	Trainable  bool
}

// TrainingSpec describes default optimizer ownership and update constraints.
// Mode applies to the whole specification unless a coordinate override changes
// a continuous/frozen table cell's trainability.
type TrainingSpec struct {
	Mode              TrainingMode
	LearningRateScale float64
	Bounds            Bounds
	Overrides         []CoordinateTrainingOverride
}

// ExportSpec binds a spec to its integer representation in the engine.
type ExportSpec struct {
	GoSymbol string
	GoType   EngineValueType
	Rounding RoundingPolicy

	// StorageDimensions describes the complete engine array addressed by
	// GoSymbol. StorageOffset and StorageStrides map semantic registry
	// coordinates into its row-major flattened storage. Empty metadata means
	// the registry shape and engine storage are identical.
	StorageDimensions []int
	StorageOffset     int
	StorageStrides    []int
}

// GroupSpec supplies defaults shared by a semantic parameter family.
type GroupSpec struct {
	ID                GroupID
	Description       string
	AnchorStrength    float64
	LearningRateScale float64
}

// ParameterSpec is the logical, human-maintained definition of one scalar or
// table parameter. Numeric vector positions are generated by NewRegistry.
type ParameterSpec struct {
	ID         ParameterID
	EngineName string
	Group      GroupID
	Shape      Shape
	Phase      PhaseKind
	Formula    FormulaKind
	Role       ParameterRole
	Initial    ValueSpec
	Prior      PriorSpec
	Training   TrainingSpec
	Export     ExportSpec
}

func cloneAxes(axes []AxisSpec) []AxisSpec {
	cloned := make([]AxisSpec, len(axes))
	for i := range axes {
		cloned[i] = AxisSpec{
			Name:   axes[i].Name,
			Labels: append([]string(nil), axes[i].Labels...),
		}
	}
	return cloned
}
