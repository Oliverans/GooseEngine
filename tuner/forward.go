package tuner

import (
	"fmt"
	"math"

	eng "chess-engine/engine"
)

// ForwardModel evaluates bound traces against registry-ordered parameter
// vectors. It contains no engine board state and never reruns evaluation.
type ForwardModel struct {
	binding               *TraceBinding
	parameterCount        int
	trainableCount        int
	trainIndexByParameter []int
}

// ExactForwardResult mirrors the three reference values carried by a tuning
// trace after applying engine-order integer arithmetic.
type ExactForwardResult struct {
	Buckets          eng.EvalPair
	WhitePerspective int32
	SideToMove       int32
}

// ContinuousForwardResult is the differentiable floating-point counterpart of
// ExactForwardResult. Gradients are added in the training-system phase.
type ContinuousForwardResult struct {
	MG               float64
	EG               float64
	WhitePerspective float64
	SideToMove       float64
}

// NewForwardModel verifies that the binding belongs to the supplied registry.
func NewForwardModel(registry *Registry, binding *TraceBinding) (*ForwardModel, error) {
	if registry == nil {
		return nil, fmt.Errorf("forward model requires a registry")
	}
	if binding == nil {
		return nil, fmt.Errorf("forward model requires a trace binding")
	}
	if binding.RegistryFingerprint != registry.Fingerprint {
		return nil, fmt.Errorf(
			"trace binding fingerprint %q does not match registry %q",
			binding.RegistryFingerprint,
			registry.Fingerprint,
		)
	}
	trainIndexes := make([]int, len(registry.Elements))
	for index, element := range registry.Elements {
		trainIndexes[index] = element.TrainIndex
	}
	return &ForwardModel{
		binding: binding, parameterCount: len(registry.Elements),
		trainableCount: registry.TrainableCount(), trainIndexByParameter: trainIndexes,
	}, nil
}

// InitialExactParameters converts the engine-seeded registry vector to the
// integer representation required by EngineExact.
func InitialExactParameters(registry *Registry) ([]int, error) {
	if registry == nil {
		return nil, fmt.Errorf("initial exact parameters require a registry")
	}
	values := make([]int, len(registry.Elements))
	for i, element := range registry.Elements {
		if !finite(element.Initial) || math.Trunc(element.Initial) != element.Initial {
			return nil, fmt.Errorf("parameter %q cell %v initial value %v is not an integer", element.ID, element.Coordinate, element.Initial)
		}
		values[i] = int(element.Initial)
	}
	return values, nil
}

// EngineExact evaluates one bound trace with the same integer operation order
// as engine.ScoreTuningTraceCurrent.
func (m *ForwardModel) EngineExact(trace BoundTrace, parameters []int) (ExactForwardResult, error) {
	if err := m.validateExactInputs(trace, parameters); err != nil {
		return ExactForwardResult{}, err
	}

	mg, eg := trace.Fixed.MG, trace.Fixed.EG
	for _, term := range trace.LinearMG {
		mg += term.Units * parameters[term.ParameterIndex]
	}
	for _, term := range trace.LinearEG {
		eg += term.Units * parameters[term.ParameterIndex]
	}

	nonlinearMG, nonlinearEG := m.engineExactNonlinear(trace.Nonlinear, parameters)
	mg += nonlinearMG
	eg += nonlinearEG

	score := int32((mg*trace.PiecePhase + eg*(trace.TotalPhase-trace.PiecePhase)) / trace.TotalPhase)
	if trace.TheoreticalDraw {
		score /= int32(parameters[m.binding.DrawDivider.Offset])
	}
	return ExactForwardResult{
		Buckets:          eng.EvalPair{MG: mg, EG: eg},
		WhitePerspective: score,
		SideToMove:       score * int32(trace.SideToMove),
	}, nil
}

func (m *ForwardModel) engineExactNonlinear(units BoundNonlinearUnits, p []int) (mg, eg int) {
	b := m.binding

	for rank := 1; rank < 7; rank++ {
		value := p[b.Pawn.Connected.MustIndex(rank-1)]
		white := value * units.Connected.White[rank]
		black := value * units.Connected.Black[rank]
		mg += white - black
		eg += white*(rank-2)/4 - black*(rank-2)/4
	}

	candidateMG := p[b.Pawn.CandidatePct.MG.Offset]
	candidateEG := p[b.Pawn.CandidatePct.EG.Offset]
	for _, candidate := range units.CandidatePassers {
		bestMG, bestEG := 0, 0
		for _, target := range candidate.Targets {
			bestMG = max(bestMG, p[target.MGParameterIndex]*candidateMG/100)
			bestEG = max(bestEG, p[target.EGParameterIndex]*candidateEG/100)
		}
		mg += candidate.Side * bestMG
		eg += candidate.Side * bestEG
	}

	center := units.CenterOpenness
	knightScaleMG := 100 - center*p[b.Center.KnightMobilityPct.MG.Offset]/4
	knightScaleEG := 100 - center*p[b.Center.KnightMobilityPct.EG.Offset]/4
	bishopScaleMG := 100 + center*p[b.Center.BishopMobilityPct.MG.Offset]/4
	bishopScaleEG := 100 + center*p[b.Center.BishopMobilityPct.EG.Offset]/4
	pairScaleMG := 100 + center*p[b.Center.BishopPairPct.MG.Offset]/4
	pairScaleEG := 100 + center*p[b.Center.BishopPairPct.EG.Offset]/4

	knightMG := exactTableDot(units.KnightMobility[:], b.Mobility.Knight.MG, p)
	knightEG := exactTableDot(units.KnightMobility[:], b.Mobility.Knight.EG, p)
	bishopMG := exactTableDot(units.BishopMobility[:], b.Mobility.Bishop.MG, p)
	bishopEG := exactTableDot(units.BishopMobility[:], b.Mobility.Bishop.EG, p)
	mg += knightMG * knightScaleMG / 100
	eg += knightEG * knightScaleEG / 100
	mg += bishopMG * bishopScaleMG / 100
	eg += bishopEG * bishopScaleEG / 100
	mg += units.BishopPair * p[b.Piece.BishopPair.MG.Offset] * pairScaleMG / 100
	eg += units.BishopPair * p[b.Piece.BishopPair.EG.Offset] * pairScaleEG / 100

	whiteMG, whiteEG := m.engineExactDanger(units.Danger.White, p)
	blackMG, blackEG := m.engineExactDanger(units.Danger.Black, p)
	mg += whiteMG - blackMG
	eg += whiteEG - blackEG

	for _, passer := range units.KingPassers {
		delta := passer.EnemyDistance*p[b.KingPasser.EnemyWeight.Offset] -
			passer.OwnDistance*p[b.KingPasser.OwnWeight.Offset]
		eg += passer.Side *
			(delta * passer.RelativeRank * passer.RelativeRank * p[b.KingPasser.ProximityEG.Offset]) /
			p[b.KingPasser.Divisor.Offset]
	}

	blocked := units.Space.BlockedPawns
	if capValue := p[b.Space.BlockedCap.Offset]; blocked > capValue {
		blocked = capValue
	}
	whiteRaw := exactSpaceRaw(units.Space.White, b.Space, p)
	blackRaw := exactSpaceRaw(units.Space.Black, b.Space, p)
	whiteWeight := max(0, units.Space.White.PieceCount-p[b.Space.WeightOffset.Offset]+blocked)
	blackWeight := max(0, units.Space.Black.PieceCount-p[b.Space.WeightOffset.Offset]+blocked)
	mg += (whiteRaw*whiteWeight*whiteWeight - blackRaw*blackWeight*blackWeight) /
		p[b.Space.WeightDivisor.Offset]

	imbalanceUnits := (units.Imbalance.TotalPawns - p[b.Imbalance.ReferencePawnCount.Offset]) *
		units.Imbalance.KnightDiff
	mg += imbalanceUnits * p[b.Imbalance.KnightPerPawn.MG.Offset]
	eg += imbalanceUnits * p[b.Imbalance.KnightPerPawn.EG.Offset]
	return mg, eg
}

func (m *ForwardModel) engineExactDanger(side eng.TuningDangerSide, p []int) (mg, eg int) {
	b := m.binding.Danger
	mg = p[b.Adjustment.MG.Offset] + side.RingHits*p[b.AttackValue.MG.Offset]
	eg = p[b.Adjustment.EG.Offset] + side.RingHits*p[b.AttackValue.EG.Offset]
	for kind := 0; kind < 4; kind++ {
		mg += side.Attackers[kind]*p[b.AttackerWeight[kind].MG.Offset] +
			side.SafeChecks[kind]*p[b.SafeCheck[kind].MG.Offset]
		eg += side.Attackers[kind]*p[b.AttackerWeight[kind].EG.Offset] +
			side.SafeChecks[kind]*p[b.SafeCheck[kind].EG.Offset]
	}
	mg += side.UnsafeChecks * p[b.UnsafeCheck.MG.Offset]
	eg += side.UnsafeChecks * p[b.UnsafeCheck.EG.Offset]
	if !side.HasQueen {
		mg += p[b.NoEnemyQueen.MG.Offset]
		eg += p[b.NoEnemyQueen.EG.Offset]
	}
	mg = max(0, mg)
	eg = max(0, eg)
	mgDivisor := p[b.Divisor.MG.Offset]
	return mg * mg / (mgDivisor * mgDivisor), eg / p[b.Divisor.EG.Offset]
}

func exactTableDot(units []int, handle ParameterHandle, parameters []int) int {
	total := 0
	for i, unit := range units {
		total += unit * parameters[handle.Offset+i]
	}
	return total
}

func exactSpaceRaw(side eng.TuningSpaceSide, binding SpaceBindings, p []int) int {
	return side.Safe*p[binding.SafeMG.Offset] +
		side.BehindPawn*p[binding.BehindPawnMG.Offset] +
		side.SemiOpen*p[binding.SemiOpenMG.Offset] +
		side.Open*p[binding.OpenMG.Offset]
}

func (m *ForwardModel) validateExactInputs(trace BoundTrace, parameters []int) error {
	if m == nil || m.binding == nil {
		return fmt.Errorf("cannot evaluate with a nil forward model")
	}
	if len(parameters) != m.parameterCount {
		return fmt.Errorf("exact parameter vector has length %d, want %d", len(parameters), m.parameterCount)
	}
	if trace.SchemaVersion != eng.TuningTraceSchemaVersion {
		return fmt.Errorf("bound trace schema %d, want %d", trace.SchemaVersion, eng.TuningTraceSchemaVersion)
	}
	if trace.SideToMove != 1 && trace.SideToMove != -1 {
		return fmt.Errorf("bound trace sideToMove %d is not +1 or -1", trace.SideToMove)
	}
	if trace.TotalPhase <= 0 || trace.PiecePhase < 0 {
		return fmt.Errorf("invalid bound phase %d/%d", trace.PiecePhase, trace.TotalPhase)
	}
	if err := validateExactDivisors(m.binding, parameters); err != nil {
		return err
	}
	if parameters[m.binding.Space.BlockedCap.Offset] < 0 {
		return fmt.Errorf("space blocked cap must be non-negative")
	}
	return validateLinearIndexes(trace, m.parameterCount)
}

func validateExactDivisors(binding *TraceBinding, p []int) error {
	divisors := []struct {
		name  string
		index int
	}{
		{name: "king danger MG", index: binding.Danger.Divisor.MG.Offset},
		{name: "king danger EG", index: binding.Danger.Divisor.EG.Offset},
		{name: "king passer", index: binding.KingPasser.Divisor.Offset},
		{name: "space", index: binding.Space.WeightDivisor.Offset},
		{name: "draw", index: binding.DrawDivider.Offset},
	}
	for _, divisor := range divisors {
		if p[divisor.index] <= 0 {
			return fmt.Errorf("%s divisor must be positive", divisor.name)
		}
	}
	return nil
}

func validateLinearIndexes(trace BoundTrace, parameterCount int) error {
	for _, terms := range [][]BoundLinearTerm{trace.LinearMG, trace.LinearEG} {
		for _, term := range terms {
			if term.ParameterIndex < 0 || term.ParameterIndex >= parameterCount {
				return fmt.Errorf("linear parameter index %d outside [0,%d)", term.ParameterIndex, parameterCount)
			}
		}
	}
	return nil
}

// Continuous evaluates one bound trace with floating-point parameters and no
// integer truncation between operations.
func (m *ForwardModel) Continuous(trace BoundTrace, parameters []float64) (ContinuousForwardResult, error) {
	if err := m.validateContinuousInputs(trace, parameters); err != nil {
		return ContinuousForwardResult{}, err
	}

	mg, eg := float64(trace.Fixed.MG), float64(trace.Fixed.EG)
	for _, term := range trace.LinearMG {
		mg += float64(term.Units) * parameters[term.ParameterIndex]
	}
	for _, term := range trace.LinearEG {
		eg += float64(term.Units) * parameters[term.ParameterIndex]
	}
	nonlinearMG, nonlinearEG := m.continuousNonlinear(trace.Nonlinear, parameters)
	mg += nonlinearMG
	eg += nonlinearEG

	phase := float64(trace.PiecePhase)
	totalPhase := float64(trace.TotalPhase)
	score := (mg*phase + eg*(totalPhase-phase)) / totalPhase
	if trace.TheoreticalDraw {
		score /= parameters[m.binding.DrawDivider.Offset]
	}
	result := ContinuousForwardResult{
		MG:               mg,
		EG:               eg,
		WhitePerspective: score,
		SideToMove:       score * float64(trace.SideToMove),
	}
	if !finite(result.MG) || !finite(result.EG) ||
		!finite(result.WhitePerspective) || !finite(result.SideToMove) {
		return ContinuousForwardResult{}, fmt.Errorf("continuous forward result is not finite")
	}
	return result, nil
}

func (m *ForwardModel) continuousNonlinear(units BoundNonlinearUnits, p []float64) (mg, eg float64) {
	b := m.binding

	for rank := 1; rank < 7; rank++ {
		value := p[b.Pawn.Connected.MustIndex(rank-1)]
		white := value * float64(units.Connected.White[rank])
		black := value * float64(units.Connected.Black[rank])
		mg += white - black
		eg += (white - black) * float64(rank-2) / 4
	}

	candidateMG := p[b.Pawn.CandidatePct.MG.Offset]
	candidateEG := p[b.Pawn.CandidatePct.EG.Offset]
	for _, candidate := range units.CandidatePassers {
		bestMG, bestEG := 0.0, 0.0
		for _, target := range candidate.Targets {
			bestMG = math.Max(bestMG, p[target.MGParameterIndex]*candidateMG/100)
			bestEG = math.Max(bestEG, p[target.EGParameterIndex]*candidateEG/100)
		}
		mg += float64(candidate.Side) * bestMG
		eg += float64(candidate.Side) * bestEG
	}

	center := float64(units.CenterOpenness)
	knightScaleMG := 100 - center*p[b.Center.KnightMobilityPct.MG.Offset]/4
	knightScaleEG := 100 - center*p[b.Center.KnightMobilityPct.EG.Offset]/4
	bishopScaleMG := 100 + center*p[b.Center.BishopMobilityPct.MG.Offset]/4
	bishopScaleEG := 100 + center*p[b.Center.BishopMobilityPct.EG.Offset]/4
	pairScaleMG := 100 + center*p[b.Center.BishopPairPct.MG.Offset]/4
	pairScaleEG := 100 + center*p[b.Center.BishopPairPct.EG.Offset]/4

	knightMG := continuousTableDot(units.KnightMobility[:], b.Mobility.Knight.MG, p)
	knightEG := continuousTableDot(units.KnightMobility[:], b.Mobility.Knight.EG, p)
	bishopMG := continuousTableDot(units.BishopMobility[:], b.Mobility.Bishop.MG, p)
	bishopEG := continuousTableDot(units.BishopMobility[:], b.Mobility.Bishop.EG, p)
	mg += knightMG * knightScaleMG / 100
	eg += knightEG * knightScaleEG / 100
	mg += bishopMG * bishopScaleMG / 100
	eg += bishopEG * bishopScaleEG / 100
	mg += float64(units.BishopPair) * p[b.Piece.BishopPair.MG.Offset] * pairScaleMG / 100
	eg += float64(units.BishopPair) * p[b.Piece.BishopPair.EG.Offset] * pairScaleEG / 100

	whiteMG, whiteEG := m.continuousDanger(units.Danger.White, p)
	blackMG, blackEG := m.continuousDanger(units.Danger.Black, p)
	mg += whiteMG - blackMG
	eg += whiteEG - blackEG

	for _, passer := range units.KingPassers {
		delta := float64(passer.EnemyDistance)*p[b.KingPasser.EnemyWeight.Offset] -
			float64(passer.OwnDistance)*p[b.KingPasser.OwnWeight.Offset]
		eg += float64(passer.Side) *
			delta * float64(passer.RelativeRank*passer.RelativeRank) *
			p[b.KingPasser.ProximityEG.Offset] /
			p[b.KingPasser.Divisor.Offset]
	}

	blocked := math.Min(float64(units.Space.BlockedPawns), p[b.Space.BlockedCap.Offset])
	whiteRaw := continuousSpaceRaw(units.Space.White, b.Space, p)
	blackRaw := continuousSpaceRaw(units.Space.Black, b.Space, p)
	whiteWeight := math.Max(0, float64(units.Space.White.PieceCount)-p[b.Space.WeightOffset.Offset]+blocked)
	blackWeight := math.Max(0, float64(units.Space.Black.PieceCount)-p[b.Space.WeightOffset.Offset]+blocked)
	mg += (whiteRaw*whiteWeight*whiteWeight - blackRaw*blackWeight*blackWeight) /
		p[b.Space.WeightDivisor.Offset]

	imbalanceUnits := (float64(units.Imbalance.TotalPawns) - p[b.Imbalance.ReferencePawnCount.Offset]) *
		float64(units.Imbalance.KnightDiff)
	mg += imbalanceUnits * p[b.Imbalance.KnightPerPawn.MG.Offset]
	eg += imbalanceUnits * p[b.Imbalance.KnightPerPawn.EG.Offset]
	return mg, eg
}

func (m *ForwardModel) continuousDanger(side eng.TuningDangerSide, p []float64) (mg, eg float64) {
	b := m.binding.Danger
	mg = p[b.Adjustment.MG.Offset] + float64(side.RingHits)*p[b.AttackValue.MG.Offset]
	eg = p[b.Adjustment.EG.Offset] + float64(side.RingHits)*p[b.AttackValue.EG.Offset]
	for kind := 0; kind < 4; kind++ {
		mg += float64(side.Attackers[kind])*p[b.AttackerWeight[kind].MG.Offset] +
			float64(side.SafeChecks[kind])*p[b.SafeCheck[kind].MG.Offset]
		eg += float64(side.Attackers[kind])*p[b.AttackerWeight[kind].EG.Offset] +
			float64(side.SafeChecks[kind])*p[b.SafeCheck[kind].EG.Offset]
	}
	mg += float64(side.UnsafeChecks) * p[b.UnsafeCheck.MG.Offset]
	eg += float64(side.UnsafeChecks) * p[b.UnsafeCheck.EG.Offset]
	if !side.HasQueen {
		mg += p[b.NoEnemyQueen.MG.Offset]
		eg += p[b.NoEnemyQueen.EG.Offset]
	}
	mg = math.Max(0, mg)
	eg = math.Max(0, eg)
	mgDivisor := p[b.Divisor.MG.Offset]
	return mg * mg / (mgDivisor * mgDivisor), eg / p[b.Divisor.EG.Offset]
}

func continuousTableDot(units []int, handle ParameterHandle, parameters []float64) float64 {
	total := 0.0
	for i, unit := range units {
		total += float64(unit) * parameters[handle.Offset+i]
	}
	return total
}

func continuousSpaceRaw(side eng.TuningSpaceSide, binding SpaceBindings, p []float64) float64 {
	return float64(side.Safe)*p[binding.SafeMG.Offset] +
		float64(side.BehindPawn)*p[binding.BehindPawnMG.Offset] +
		float64(side.SemiOpen)*p[binding.SemiOpenMG.Offset] +
		float64(side.Open)*p[binding.OpenMG.Offset]
}

func (m *ForwardModel) validateContinuousInputs(trace BoundTrace, parameters []float64) error {
	if m == nil || m.binding == nil {
		return fmt.Errorf("cannot evaluate with a nil forward model")
	}
	if len(parameters) != m.parameterCount {
		return fmt.Errorf("continuous parameter vector has length %d, want %d", len(parameters), m.parameterCount)
	}
	if trace.SchemaVersion != eng.TuningTraceSchemaVersion {
		return fmt.Errorf("bound trace schema %d, want %d", trace.SchemaVersion, eng.TuningTraceSchemaVersion)
	}
	if trace.SideToMove != 1 && trace.SideToMove != -1 {
		return fmt.Errorf("bound trace sideToMove %d is not +1 or -1", trace.SideToMove)
	}
	if trace.TotalPhase <= 0 || trace.PiecePhase < 0 {
		return fmt.Errorf("invalid bound phase %d/%d", trace.PiecePhase, trace.TotalPhase)
	}
	if err := validateContinuousDivisors(m.binding, parameters); err != nil {
		return err
	}
	if parameters[m.binding.Space.BlockedCap.Offset] < 0 {
		return fmt.Errorf("space blocked cap must be non-negative")
	}
	return validateLinearIndexes(trace, m.parameterCount)
}

// ValidateContinuousParameters performs the whole-vector checks intended once
// after an optimizer update, not once per position.
func (m *ForwardModel) ValidateContinuousParameters(parameters []float64) error {
	if m == nil || m.binding == nil {
		return fmt.Errorf("cannot validate parameters with a nil forward model")
	}
	if len(parameters) != m.parameterCount {
		return fmt.Errorf("continuous parameter vector has length %d, want %d", len(parameters), m.parameterCount)
	}
	for i, value := range parameters {
		if !finite(value) {
			return fmt.Errorf("continuous parameter %d is not finite", i)
		}
	}
	if err := validateContinuousDivisors(m.binding, parameters); err != nil {
		return err
	}
	if parameters[m.binding.Space.BlockedCap.Offset] < 0 {
		return fmt.Errorf("space blocked cap must be non-negative")
	}
	return nil
}

func validateContinuousDivisors(binding *TraceBinding, p []float64) error {
	divisors := []struct {
		name  string
		index int
	}{
		{name: "king danger MG", index: binding.Danger.Divisor.MG.Offset},
		{name: "king danger EG", index: binding.Danger.Divisor.EG.Offset},
		{name: "king passer", index: binding.KingPasser.Divisor.Offset},
		{name: "space", index: binding.Space.WeightDivisor.Offset},
		{name: "draw", index: binding.DrawDivider.Offset},
	}
	for _, divisor := range divisors {
		if p[divisor.index] <= 0 {
			return fmt.Errorf("%s divisor must be positive", divisor.name)
		}
	}
	return nil
}
