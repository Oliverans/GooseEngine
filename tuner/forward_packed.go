package tuner

import (
	"fmt"
	"math"

	eng "chess-engine/engine"
)

// EngineExactPacked evaluates one record without expanding its arena-backed
// variable-length features into a BoundTrace.
func (m *ForwardModel) EngineExactPacked(shard *PackedDatasetShard, recordIndex int, parameters []int) (ExactForwardResult, error) {
	record, err := m.validatePackedExactInputs(shard, recordIndex, parameters)
	if err != nil {
		return ExactForwardResult{}, err
	}

	mg, eg := int(record.FixedMG), int(record.FixedEG)
	for _, term := range shard.linearTerms(record.LinearMG) {
		mg += int(term.Units) * parameters[int(term.ParameterIndex)]
	}
	for _, term := range shard.linearTerms(record.LinearEG) {
		eg += int(term.Units) * parameters[int(term.ParameterIndex)]
	}

	nonlinearMG, nonlinearEG := m.engineExactPackedNonlinear(shard, record, parameters)
	mg += nonlinearMG
	eg += nonlinearEG

	phase, totalPhase := int(record.PiecePhase), int(record.TotalPhase)
	score := int32((mg*phase + eg*(totalPhase-phase)) / totalPhase)
	if record.TheoreticalDraw() {
		score /= int32(parameters[m.binding.DrawDivider.Offset])
	}
	return ExactForwardResult{
		Buckets:          eng.EvalPair{MG: mg, EG: eg},
		WhitePerspective: score,
		SideToMove:       score * int32(record.SideToMove),
	}, nil
}

func (m *ForwardModel) engineExactPackedNonlinear(shard *PackedDatasetShard, record *PackedRecord, p []int) (mg, eg int) {
	b := m.binding
	u := &record.Nonlinear

	for rank := 1; rank < 7; rank++ {
		value := p[b.Pawn.Connected.MustIndex(rank-1)]
		white := value * int(u.ConnectedWhite[rank])
		black := value * int(u.ConnectedBlack[rank])
		mg += white - black
		eg += white*(rank-2)/4 - black*(rank-2)/4
	}

	candidateMG := p[b.Pawn.CandidatePct.MG.Offset]
	candidateEG := p[b.Pawn.CandidatePct.EG.Offset]
	for _, candidate := range shard.candidatePassers(record.Candidates) {
		bestMG, bestEG := 0, 0
		for _, target := range shard.candidateTargets(candidate) {
			bestMG = max(bestMG, p[int(target.MGParameterIndex)]*candidateMG/100)
			bestEG = max(bestEG, p[int(target.EGParameterIndex)]*candidateEG/100)
		}
		mg += int(candidate.Side) * bestMG
		eg += int(candidate.Side) * bestEG
	}

	center := int(u.CenterOpenness)
	knightScaleMG := 100 - center*p[b.Center.KnightMobilityPct.MG.Offset]/4
	knightScaleEG := 100 - center*p[b.Center.KnightMobilityPct.EG.Offset]/4
	bishopScaleMG := 100 + center*p[b.Center.BishopMobilityPct.MG.Offset]/4
	bishopScaleEG := 100 + center*p[b.Center.BishopMobilityPct.EG.Offset]/4
	pairScaleMG := 100 + center*p[b.Center.BishopPairPct.MG.Offset]/4
	pairScaleEG := 100 + center*p[b.Center.BishopPairPct.EG.Offset]/4

	knightMG := exactPackedTableDot(u.KnightMobility[:], b.Mobility.Knight.MG, p)
	knightEG := exactPackedTableDot(u.KnightMobility[:], b.Mobility.Knight.EG, p)
	bishopMG := exactPackedTableDot(u.BishopMobility[:], b.Mobility.Bishop.MG, p)
	bishopEG := exactPackedTableDot(u.BishopMobility[:], b.Mobility.Bishop.EG, p)
	mg += knightMG * knightScaleMG / 100
	eg += knightEG * knightScaleEG / 100
	mg += bishopMG * bishopScaleMG / 100
	eg += bishopEG * bishopScaleEG / 100
	mg += int(u.BishopPair) * p[b.Piece.BishopPair.MG.Offset] * pairScaleMG / 100
	eg += int(u.BishopPair) * p[b.Piece.BishopPair.EG.Offset] * pairScaleEG / 100

	whiteMG, whiteEG := m.engineExactPackedDanger(u.DangerWhite, p)
	blackMG, blackEG := m.engineExactPackedDanger(u.DangerBlack, p)
	mg += whiteMG - blackMG
	eg += whiteEG - blackEG

	for _, passer := range shard.kingPassers(record.KingPassers) {
		rank := int(passer.RelativeRank)
		delta := int(passer.EnemyDistance)*p[b.KingPasser.EnemyWeight.Offset] -
			int(passer.OwnDistance)*p[b.KingPasser.OwnWeight.Offset]
		eg += int(passer.Side) *
			(delta * rank * rank * p[b.KingPasser.ProximityEG.Offset]) /
			p[b.KingPasser.Divisor.Offset]
	}

	blocked := int(u.SpaceBlocked)
	if capValue := p[b.Space.BlockedCap.Offset]; blocked > capValue {
		blocked = capValue
	}
	whiteRaw := exactPackedSpaceRaw(u.SpaceWhite, b.Space, p)
	blackRaw := exactPackedSpaceRaw(u.SpaceBlack, b.Space, p)
	whiteWeight := max(0, int(u.SpaceWhite.PieceCount)-p[b.Space.WeightOffset.Offset]+blocked)
	blackWeight := max(0, int(u.SpaceBlack.PieceCount)-p[b.Space.WeightOffset.Offset]+blocked)
	mg += (whiteRaw*whiteWeight*whiteWeight - blackRaw*blackWeight*blackWeight) /
		p[b.Space.WeightDivisor.Offset]

	imbalanceUnits := (int(u.TotalPawns) - p[b.Imbalance.ReferencePawnCount.Offset]) * int(u.KnightDiff)
	mg += imbalanceUnits * p[b.Imbalance.KnightPerPawn.MG.Offset]
	eg += imbalanceUnits * p[b.Imbalance.KnightPerPawn.EG.Offset]
	return mg, eg
}

func (m *ForwardModel) engineExactPackedDanger(side PackedDangerSide, p []int) (mg, eg int) {
	b := m.binding.Danger
	mg = p[b.Adjustment.MG.Offset] + int(side.RingHits)*p[b.AttackValue.MG.Offset]
	eg = p[b.Adjustment.EG.Offset] + int(side.RingHits)*p[b.AttackValue.EG.Offset]
	for kind := 0; kind < 4; kind++ {
		mg += int(side.Attackers[kind])*p[b.AttackerWeight[kind].MG.Offset] +
			int(side.SafeChecks[kind])*p[b.SafeCheck[kind].MG.Offset]
		eg += int(side.Attackers[kind])*p[b.AttackerWeight[kind].EG.Offset] +
			int(side.SafeChecks[kind])*p[b.SafeCheck[kind].EG.Offset]
	}
	mg += int(side.UnsafeChecks) * p[b.UnsafeCheck.MG.Offset]
	eg += int(side.UnsafeChecks) * p[b.UnsafeCheck.EG.Offset]
	if !side.HasQueen {
		mg += p[b.NoEnemyQueen.MG.Offset]
		eg += p[b.NoEnemyQueen.EG.Offset]
	}
	mg = max(0, mg)
	eg = max(0, eg)
	mgDivisor := p[b.Divisor.MG.Offset]
	return mg * mg / (mgDivisor * mgDivisor), eg / p[b.Divisor.EG.Offset]
}

func exactPackedTableDot(units []int16, handle ParameterHandle, p []int) int {
	total := 0
	for index, unit := range units {
		total += int(unit) * p[handle.Offset+index]
	}
	return total
}

func exactPackedSpaceRaw(side PackedSpaceSide, binding SpaceBindings, p []int) int {
	return int(side.Safe)*p[binding.SafeMG.Offset] +
		int(side.BehindPawn)*p[binding.BehindPawnMG.Offset] +
		int(side.SemiOpen)*p[binding.SemiOpenMG.Offset] +
		int(side.Open)*p[binding.OpenMG.Offset]
}

// ContinuousPacked is the floating-point training forward pass over a packed
// record. Its arithmetic order matches Continuous.
func (m *ForwardModel) ContinuousPacked(shard *PackedDatasetShard, recordIndex int, parameters []float64) (ContinuousForwardResult, error) {
	record, err := m.validatePackedContinuousInputs(shard, recordIndex, parameters)
	if err != nil {
		return ContinuousForwardResult{}, err
	}

	mg, eg := float64(record.FixedMG), float64(record.FixedEG)
	for _, term := range shard.linearTerms(record.LinearMG) {
		mg += float64(term.Units) * parameters[int(term.ParameterIndex)]
	}
	for _, term := range shard.linearTerms(record.LinearEG) {
		eg += float64(term.Units) * parameters[int(term.ParameterIndex)]
	}
	nonlinearMG, nonlinearEG := m.continuousPackedNonlinear(shard, record, parameters)
	mg += nonlinearMG
	eg += nonlinearEG

	phase, totalPhase := float64(record.PiecePhase), float64(record.TotalPhase)
	score := (mg*phase + eg*(totalPhase-phase)) / totalPhase
	if record.TheoreticalDraw() {
		score /= parameters[m.binding.DrawDivider.Offset]
	}
	result := ContinuousForwardResult{
		MG: mg, EG: eg, WhitePerspective: score,
		SideToMove: score * float64(record.SideToMove),
	}
	if !finite(result.MG) || !finite(result.EG) || !finite(result.WhitePerspective) || !finite(result.SideToMove) {
		return ContinuousForwardResult{}, fmt.Errorf("continuous packed forward result is not finite")
	}
	return result, nil
}

func (m *ForwardModel) continuousPackedNonlinear(shard *PackedDatasetShard, record *PackedRecord, p []float64) (mg, eg float64) {
	b := m.binding
	u := &record.Nonlinear

	for rank := 1; rank < 7; rank++ {
		value := p[b.Pawn.Connected.MustIndex(rank-1)]
		white := value * float64(u.ConnectedWhite[rank])
		black := value * float64(u.ConnectedBlack[rank])
		mg += white - black
		eg += (white - black) * float64(rank-2) / 4
	}

	candidateMG := p[b.Pawn.CandidatePct.MG.Offset]
	candidateEG := p[b.Pawn.CandidatePct.EG.Offset]
	for _, candidate := range shard.candidatePassers(record.Candidates) {
		bestMG, bestEG := 0.0, 0.0
		for _, target := range shard.candidateTargets(candidate) {
			bestMG = math.Max(bestMG, p[int(target.MGParameterIndex)]*candidateMG/100)
			bestEG = math.Max(bestEG, p[int(target.EGParameterIndex)]*candidateEG/100)
		}
		mg += float64(candidate.Side) * bestMG
		eg += float64(candidate.Side) * bestEG
	}

	center := float64(u.CenterOpenness)
	knightScaleMG := 100 - center*p[b.Center.KnightMobilityPct.MG.Offset]/4
	knightScaleEG := 100 - center*p[b.Center.KnightMobilityPct.EG.Offset]/4
	bishopScaleMG := 100 + center*p[b.Center.BishopMobilityPct.MG.Offset]/4
	bishopScaleEG := 100 + center*p[b.Center.BishopMobilityPct.EG.Offset]/4
	pairScaleMG := 100 + center*p[b.Center.BishopPairPct.MG.Offset]/4
	pairScaleEG := 100 + center*p[b.Center.BishopPairPct.EG.Offset]/4

	knightMG := continuousPackedTableDot(u.KnightMobility[:], b.Mobility.Knight.MG, p)
	knightEG := continuousPackedTableDot(u.KnightMobility[:], b.Mobility.Knight.EG, p)
	bishopMG := continuousPackedTableDot(u.BishopMobility[:], b.Mobility.Bishop.MG, p)
	bishopEG := continuousPackedTableDot(u.BishopMobility[:], b.Mobility.Bishop.EG, p)
	mg += knightMG * knightScaleMG / 100
	eg += knightEG * knightScaleEG / 100
	mg += bishopMG * bishopScaleMG / 100
	eg += bishopEG * bishopScaleEG / 100
	mg += float64(u.BishopPair) * p[b.Piece.BishopPair.MG.Offset] * pairScaleMG / 100
	eg += float64(u.BishopPair) * p[b.Piece.BishopPair.EG.Offset] * pairScaleEG / 100

	whiteMG, whiteEG := m.continuousPackedDanger(u.DangerWhite, p)
	blackMG, blackEG := m.continuousPackedDanger(u.DangerBlack, p)
	mg += whiteMG - blackMG
	eg += whiteEG - blackEG

	for _, passer := range shard.kingPassers(record.KingPassers) {
		rank := float64(passer.RelativeRank)
		delta := float64(passer.EnemyDistance)*p[b.KingPasser.EnemyWeight.Offset] -
			float64(passer.OwnDistance)*p[b.KingPasser.OwnWeight.Offset]
		eg += float64(passer.Side) * delta * rank * rank * p[b.KingPasser.ProximityEG.Offset] /
			p[b.KingPasser.Divisor.Offset]
	}

	blocked := math.Min(float64(u.SpaceBlocked), p[b.Space.BlockedCap.Offset])
	whiteRaw := continuousPackedSpaceRaw(u.SpaceWhite, b.Space, p)
	blackRaw := continuousPackedSpaceRaw(u.SpaceBlack, b.Space, p)
	whiteWeight := math.Max(0, float64(u.SpaceWhite.PieceCount)-p[b.Space.WeightOffset.Offset]+blocked)
	blackWeight := math.Max(0, float64(u.SpaceBlack.PieceCount)-p[b.Space.WeightOffset.Offset]+blocked)
	mg += (whiteRaw*whiteWeight*whiteWeight - blackRaw*blackWeight*blackWeight) /
		p[b.Space.WeightDivisor.Offset]

	imbalanceUnits := (float64(u.TotalPawns) - p[b.Imbalance.ReferencePawnCount.Offset]) * float64(u.KnightDiff)
	mg += imbalanceUnits * p[b.Imbalance.KnightPerPawn.MG.Offset]
	eg += imbalanceUnits * p[b.Imbalance.KnightPerPawn.EG.Offset]
	return mg, eg
}

func (m *ForwardModel) continuousPackedDanger(side PackedDangerSide, p []float64) (mg, eg float64) {
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

func continuousPackedTableDot(units []int16, handle ParameterHandle, p []float64) float64 {
	total := 0.0
	for index, unit := range units {
		total += float64(unit) * p[handle.Offset+index]
	}
	return total
}

func continuousPackedSpaceRaw(side PackedSpaceSide, binding SpaceBindings, p []float64) float64 {
	return float64(side.Safe)*p[binding.SafeMG.Offset] +
		float64(side.BehindPawn)*p[binding.BehindPawnMG.Offset] +
		float64(side.SemiOpen)*p[binding.SemiOpenMG.Offset] +
		float64(side.Open)*p[binding.OpenMG.Offset]
}

func (m *ForwardModel) validatePackedExactInputs(shard *PackedDatasetShard, recordIndex int, parameters []int) (*PackedRecord, error) {
	if m == nil || m.binding == nil {
		return nil, fmt.Errorf("cannot evaluate with a nil forward model")
	}
	if shard == nil || recordIndex < 0 || recordIndex >= len(shard.Records) {
		return nil, fmt.Errorf("packed record index %d outside [0,%d)", recordIndex, packedRecordCount(shard))
	}
	if len(parameters) != m.parameterCount {
		return nil, fmt.Errorf("exact parameter vector has length %d, want %d", len(parameters), m.parameterCount)
	}
	if err := validateExactDivisors(m.binding, parameters); err != nil {
		return nil, err
	}
	if parameters[m.binding.Space.BlockedCap.Offset] < 0 {
		return nil, fmt.Errorf("space blocked cap must be non-negative")
	}
	return &shard.Records[recordIndex], nil
}

func (m *ForwardModel) validatePackedContinuousInputs(shard *PackedDatasetShard, recordIndex int, parameters []float64) (*PackedRecord, error) {
	if m == nil || m.binding == nil {
		return nil, fmt.Errorf("cannot evaluate with a nil forward model")
	}
	if shard == nil || recordIndex < 0 || recordIndex >= len(shard.Records) {
		return nil, fmt.Errorf("packed record index %d outside [0,%d)", recordIndex, packedRecordCount(shard))
	}
	if len(parameters) != m.parameterCount {
		return nil, fmt.Errorf("continuous parameter vector has length %d, want %d", len(parameters), m.parameterCount)
	}
	if err := validateContinuousDivisors(m.binding, parameters); err != nil {
		return nil, err
	}
	if parameters[m.binding.Space.BlockedCap.Offset] < 0 {
		return nil, fmt.Errorf("space blocked cap must be non-negative")
	}
	return &shard.Records[recordIndex], nil
}

func packedRecordCount(shard *PackedDatasetShard) int {
	if shard == nil {
		return 0
	}
	return len(shard.Records)
}
