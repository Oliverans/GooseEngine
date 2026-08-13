package tuner

import (
	"math"
	"math/rand"
	"testing"

	gm "chess-engine/goosemg"
)

func TestContinuousPackedGradientMatchesForwardAndFiniteDifferences(t *testing.T) {
	registry, binding, model := newForwardTestSystem(t)
	shard := nonlinearGradientTestShard(binding)
	parameters := registry.InitialValues()
	gradient := make([]float64, registry.TrainableCount())
	got, err := model.ContinuousPackedGradient(&shard, 0, parameters, 1, gradient)
	if err != nil {
		t.Fatal(err)
	}
	want, err := model.ContinuousPacked(&shard, 0, parameters)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("gradient forward result = %+v, want %+v", got, want)
	}

	const epsilon = 1e-4
	nonzero := 0
	for _, element := range registry.Elements {
		if element.TrainIndex == NoTrainIndex {
			continue
		}
		plus := append([]float64(nil), parameters...)
		minus := append([]float64(nil), parameters...)
		plus[element.Index] += epsilon
		minus[element.Index] -= epsilon
		plusResult, err := model.ContinuousPacked(&shard, 0, plus)
		if err != nil {
			t.Fatalf("plus %s%v: %v", element.ID, element.Coordinate, err)
		}
		minusResult, err := model.ContinuousPacked(&shard, 0, minus)
		if err != nil {
			t.Fatalf("minus %s%v: %v", element.ID, element.Coordinate, err)
		}
		numeric := (plusResult.WhitePerspective - minusResult.WhitePerspective) / (2 * epsilon)
		analytic := gradient[element.TrainIndex]
		tolerance := 2e-7 * math.Max(1, math.Abs(numeric))
		if math.Abs(analytic-numeric) > tolerance {
			t.Errorf("%s%v gradient = %.12g, numeric %.12g, difference %.3g > %.3g",
				element.ID, element.Coordinate, analytic, numeric, math.Abs(analytic-numeric), tolerance)
		}
		if analytic != 0 {
			nonzero++
		}
	}
	if nonzero < 40 {
		t.Fatalf("only %d train coordinates received gradients", nonzero)
	}
	for name, parameterIndex := range map[string]int{
		"linear":               binding.Tempo.Offset,
		"connected pawn":       binding.Pawn.Connected.MustIndex(2),
		"candidate percentage": binding.Pawn.CandidatePct.MG.Offset,
		"center scaling":       binding.Center.KnightMobilityPct.MG.Offset,
		"mobility table":       binding.Mobility.Knight.MG.Offset + 6,
		"bishop pair":          binding.Piece.BishopPair.MG.Offset,
		"king danger":          binding.Danger.AttackValue.MG.Offset,
		"king passer":          binding.KingPasser.EnemyWeight.Offset,
		"space":                binding.Space.SafeMG.Offset,
		"imbalance":            binding.Imbalance.KnightPerPawn.MG.Offset,
	} {
		trainIndex := registry.Elements[parameterIndex].TrainIndex
		if trainIndex == NoTrainIndex || gradient[trainIndex] == 0 {
			t.Errorf("%s representative parameter %d received no gradient", name, parameterIndex)
		}
	}
}

func TestContinuousPackedGradientDirectionalDerivative(t *testing.T) {
	registry, binding, model := newForwardTestSystem(t)
	shard := nonlinearGradientTestShard(binding)
	parameters := registry.InitialValues()
	gradient := make([]float64, registry.TrainableCount())
	if _, err := model.ContinuousPackedGradient(&shard, 0, parameters, 1, gradient); err != nil {
		t.Fatal(err)
	}
	rng := rand.New(rand.NewSource(0x6772616469656e74))
	plus := append([]float64(nil), parameters...)
	minus := append([]float64(nil), parameters...)
	const epsilon = 1e-5
	analytic := 0.0
	for _, element := range registry.Elements {
		if element.TrainIndex == NoTrainIndex {
			continue
		}
		value := rng.Float64()*2 - 1
		plus[element.Index] += epsilon * value
		minus[element.Index] -= epsilon * value
		analytic += gradient[element.TrainIndex] * value
	}
	plusResult, err := model.ContinuousPacked(&shard, 0, plus)
	if err != nil {
		t.Fatal(err)
	}
	minusResult, err := model.ContinuousPacked(&shard, 0, minus)
	if err != nil {
		t.Fatal(err)
	}
	numeric := (plusResult.WhitePerspective - minusResult.WhitePerspective) / (2 * epsilon)
	assertNear(t, analytic, numeric, 2e-6*math.Max(1, math.Abs(numeric)))
}

func TestContinuousPackedGradientIncludesCurrentlyFrozenFormulaParameters(t *testing.T) {
	base, err := NewEngineRegistry()
	if err != nil {
		t.Fatal(err)
	}
	specs := cloneSpecs(base.Specs)
	for index := range specs {
		specs[index].Training.Mode = TrainingContinuous
	}
	registry, err := NewRegistry(base.Version+"-gradient-all-continuous", base.Groups, specs)
	if err != nil {
		t.Fatal(err)
	}
	binding, err := NewTraceBinding(registry)
	if err != nil {
		t.Fatal(err)
	}
	model, err := NewForwardModel(registry, binding)
	if err != nil {
		t.Fatal(err)
	}
	shard := nonlinearGradientTestShard(binding)
	shard.Records[0].Flags |= packedFlagTheoreticalDraw
	parameters := registry.InitialValues()
	gradient := make([]float64, registry.TrainableCount())
	if _, err := model.ContinuousPackedGradient(&shard, 0, parameters, 1, gradient); err != nil {
		t.Fatal(err)
	}
	indexes := map[string]int{
		"danger MG divisor":   binding.Danger.Divisor.MG.Offset,
		"danger EG divisor":   binding.Danger.Divisor.EG.Offset,
		"king passer divisor": binding.KingPasser.Divisor.Offset,
		"space cap":           binding.Space.BlockedCap.Offset,
		"space divisor":       binding.Space.WeightDivisor.Offset,
		"imbalance reference": binding.Imbalance.ReferencePawnCount.Offset,
		"draw divider":        binding.DrawDivider.Offset,
	}
	const epsilon = 1e-5
	for name, parameterIndex := range indexes {
		plus := append([]float64(nil), parameters...)
		minus := append([]float64(nil), parameters...)
		plus[parameterIndex] += epsilon
		minus[parameterIndex] -= epsilon
		plusResult, err := model.ContinuousPacked(&shard, 0, plus)
		if err != nil {
			t.Fatal(err)
		}
		minusResult, err := model.ContinuousPacked(&shard, 0, minus)
		if err != nil {
			t.Fatal(err)
		}
		numeric := (plusResult.WhitePerspective - minusResult.WhitePerspective) / (2 * epsilon)
		analytic := gradient[registry.Elements[parameterIndex].TrainIndex]
		assertNear(t, analytic, numeric, 3e-7*math.Max(1, math.Abs(numeric)))
		if analytic == 0 {
			t.Errorf("%s unexpectedly received zero gradient", name)
		}
	}
}

func nonlinearGradientTestShard(binding *TraceBinding) PackedDatasetShard {
	shard := PackedDatasetShard{}
	record := PackedRecord{
		FixedMG: 17, FixedEG: -11, PiecePhase: 13, TotalPhase: 24,
		SideToMove: 1, Outcome: OutcomeWhiteWin, Split: SplitTraining,
	}
	record.LinearMG = PackedSpan{Offset: uint32(len(shard.LinearTerms)), Count: 2}
	shard.LinearTerms = append(shard.LinearTerms,
		PackedLinearTerm{ParameterIndex: uint16(binding.Material[gm.PieceTypePawn].MG.Offset), Units: 2},
		PackedLinearTerm{ParameterIndex: uint16(binding.Tempo.Offset), Units: 1},
	)
	record.LinearEG = PackedSpan{Offset: uint32(len(shard.LinearTerms)), Count: 2}
	shard.LinearTerms = append(shard.LinearTerms,
		PackedLinearTerm{ParameterIndex: uint16(binding.Material[gm.PieceTypeKnight].EG.Offset), Units: -1},
		PackedLinearTerm{ParameterIndex: uint16(binding.Pawn.Passed.EG.MustIndex(4, 3)), Units: 2},
	)
	record.Nonlinear.ConnectedWhite[3] = 2
	record.Nonlinear.ConnectedBlack[5] = 1

	record.Candidates = PackedSpan{Offset: 0, Count: 1}
	shard.Candidates = append(shard.Candidates, PackedCandidate{TargetOffset: 0, TargetCount: 2, Side: 1, Source: 24})
	shard.Targets = append(shard.Targets,
		PackedCandidateTarget{
			MGParameterIndex: uint16(binding.Material[gm.PieceTypeKnight].MG.Offset),
			EGParameterIndex: uint16(binding.Material[gm.PieceTypeKnight].EG.Offset),
		},
		PackedCandidateTarget{
			MGParameterIndex: uint16(binding.Material[gm.PieceTypePawn].MG.Offset),
			EGParameterIndex: uint16(binding.Material[gm.PieceTypePawn].EG.Offset),
		},
	)

	record.Nonlinear.CenterOpenness = 2
	record.Nonlinear.KnightMobility[2] = -1
	record.Nonlinear.KnightMobility[6] = 2
	record.Nonlinear.BishopMobility[4] = 1
	record.Nonlinear.BishopMobility[10] = -2
	record.Nonlinear.BishopPair = 1
	record.Nonlinear.DangerWhite = PackedDangerSide{
		Attackers: [4]int16{4, 3, 2, 1}, RingHits: 12,
		SafeChecks: [4]int16{2, 1, 1, 1}, UnsafeChecks: 3, HasQueen: false,
	}
	record.Nonlinear.DangerBlack = PackedDangerSide{
		Attackers: [4]int16{2, 1, 1, 1}, RingHits: 8,
		SafeChecks: [4]int16{1, 1, 0, 1}, UnsafeChecks: 1, HasQueen: true,
	}
	record.KingPassers = PackedSpan{Offset: 0, Count: 2}
	shard.KingPassers = append(shard.KingPassers,
		PackedKingPasser{Side: 1, RelativeRank: 5, EnemyDistance: 3, OwnDistance: 2},
		PackedKingPasser{Side: -1, RelativeRank: 4, EnemyDistance: 5, OwnDistance: 1},
	)
	record.Nonlinear.SpaceWhite = PackedSpaceSide{Safe: 8, BehindPawn: 3, SemiOpen: 2, Open: 1, PieceCount: 20}
	record.Nonlinear.SpaceBlack = PackedSpaceSide{Safe: 5, BehindPawn: 2, SemiOpen: 1, Open: 2, PieceCount: 18}
	record.Nonlinear.SpaceBlocked = 12
	record.Nonlinear.TotalPawns = 11
	record.Nonlinear.KnightDiff = 2
	shard.Records = append(shard.Records, record)
	return shard
}
