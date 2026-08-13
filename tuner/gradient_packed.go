package tuner

import (
	"fmt"
	"math"
)

// ContinuousPackedGradient evaluates one packed record and adds
// scoreAdjoint*d(WhitePerspective)/d(parameter) to the dense train-coordinate
// gradient. It does not clear an existing gradient.
func (m *ForwardModel) ContinuousPackedGradient(shard *PackedDatasetShard, recordIndex int, parameters []float64, scoreAdjoint float64, trainGradient []float64) (ContinuousForwardResult, error) {
	record, err := m.validatePackedContinuousInputs(shard, recordIndex, parameters)
	if err != nil {
		return ContinuousForwardResult{}, err
	}
	if !finite(scoreAdjoint) {
		return ContinuousForwardResult{}, fmt.Errorf("score adjoint is not finite: %v", scoreAdjoint)
	}
	if len(trainGradient) != m.trainableCount {
		return ContinuousForwardResult{}, fmt.Errorf("train gradient has length %d, want %d", len(trainGradient), m.trainableCount)
	}
	accumulator := trainGradientAccumulator{values: trainGradient}
	return m.continuousPackedGradientRecord(shard, record, parameters, scoreAdjoint, &accumulator)
}

type trainGradientAccumulator struct {
	values     []float64
	touched    []int
	marks      []uint32
	generation uint32
	tracked    bool
}

func newTrackedGradientAccumulator(size int) trainGradientAccumulator {
	return trainGradientAccumulator{
		values: make([]float64, size), touched: make([]int, 0, 128),
		marks: make([]uint32, size), tracked: true,
	}
}

func (a *trainGradientAccumulator) beginRecord() {
	if a == nil || !a.tracked {
		return
	}
	a.generation++
	if a.generation == 0 {
		clear(a.marks)
		a.generation = 1
	}
	a.touched = a.touched[:0]
}

func (a *trainGradientAccumulator) add(index int, value float64) {
	if value == 0 {
		return
	}
	if a.tracked && a.marks[index] != a.generation {
		a.marks[index] = a.generation
		a.touched = append(a.touched, index)
	}
	a.values[index] += value
}

func (a *trainGradientAccumulator) reduceInto(destination []float64, scale float64) {
	for _, index := range a.touched {
		destination[index] += a.values[index] * scale
		a.values[index] = 0
	}
}

func (m *ForwardModel) continuousPackedGradientRecord(shard *PackedDatasetShard, record *PackedRecord, p []float64, scoreAdjoint float64, gradient *trainGradientAccumulator) (ContinuousForwardResult, error) {
	b := m.binding
	u := &record.Nonlinear
	phase, totalPhase := float64(record.PiecePhase), float64(record.TotalPhase)
	preScaleAdjoint := scoreAdjoint
	if record.TheoreticalDraw() {
		preScaleAdjoint /= p[b.DrawDivider.Offset]
	}
	mgAdjoint := preScaleAdjoint * phase / totalPhase
	egAdjoint := preScaleAdjoint * (totalPhase - phase) / totalPhase

	mg, eg := float64(record.FixedMG), float64(record.FixedEG)
	for _, term := range shard.linearTerms(record.LinearMG) {
		parameterIndex := int(term.ParameterIndex)
		units := float64(term.Units)
		mg += units * p[parameterIndex]
		m.addTrainGradient(gradient, parameterIndex, mgAdjoint*units)
	}
	for _, term := range shard.linearTerms(record.LinearEG) {
		parameterIndex := int(term.ParameterIndex)
		units := float64(term.Units)
		eg += units * p[parameterIndex]
		m.addTrainGradient(gradient, parameterIndex, egAdjoint*units)
	}
	baseMG, baseEG := mg, eg
	mg, eg = 0, 0

	for rank := 1; rank < 7; rank++ {
		parameterIndex := b.Pawn.Connected.MustIndex(rank - 1)
		value := p[parameterIndex]
		whiteUnits := float64(u.ConnectedWhite[rank])
		blackUnits := float64(u.ConnectedBlack[rank])
		white := value * whiteUnits
		black := value * blackUnits
		difference := white - black
		factor := float64(rank-2) / 4
		mg += difference
		eg += difference * factor
		m.addTrainGradient(gradient, parameterIndex,
			(whiteUnits-blackUnits)*(mgAdjoint+egAdjoint*factor))
	}

	candidateMGIndex := b.Pawn.CandidatePct.MG.Offset
	candidateEGIndex := b.Pawn.CandidatePct.EG.Offset
	candidateMG := p[candidateMGIndex]
	candidateEG := p[candidateEGIndex]
	for _, candidate := range shard.candidatePassers(record.Candidates) {
		bestMG, bestEG := 0.0, 0.0
		bestMGIndex, bestEGIndex := -1, -1
		for _, target := range shard.candidateTargets(candidate) {
			mgIndex := int(target.MGParameterIndex)
			egIndex := int(target.EGParameterIndex)
			mgValue := p[mgIndex] * candidateMG / 100
			egValue := p[egIndex] * candidateEG / 100
			if mgValue > bestMG {
				bestMG, bestMGIndex = mgValue, mgIndex
			}
			if egValue > bestEG {
				bestEG, bestEGIndex = egValue, egIndex
			}
		}
		side := float64(candidate.Side)
		mg += side * bestMG
		eg += side * bestEG
		if bestMGIndex != -1 {
			m.addTrainGradient(gradient, bestMGIndex, mgAdjoint*side*candidateMG/100)
			m.addTrainGradient(gradient, candidateMGIndex, mgAdjoint*side*p[bestMGIndex]/100)
		}
		if bestEGIndex != -1 {
			m.addTrainGradient(gradient, bestEGIndex, egAdjoint*side*candidateEG/100)
			m.addTrainGradient(gradient, candidateEGIndex, egAdjoint*side*p[bestEGIndex]/100)
		}
	}

	center := float64(u.CenterOpenness)
	knightPctMGIndex := b.Center.KnightMobilityPct.MG.Offset
	knightPctEGIndex := b.Center.KnightMobilityPct.EG.Offset
	bishopPctMGIndex := b.Center.BishopMobilityPct.MG.Offset
	bishopPctEGIndex := b.Center.BishopMobilityPct.EG.Offset
	pairPctMGIndex := b.Center.BishopPairPct.MG.Offset
	pairPctEGIndex := b.Center.BishopPairPct.EG.Offset
	knightScaleMG := 100 - center*p[knightPctMGIndex]/4
	knightScaleEG := 100 - center*p[knightPctEGIndex]/4
	bishopScaleMG := 100 + center*p[bishopPctMGIndex]/4
	bishopScaleEG := 100 + center*p[bishopPctEGIndex]/4
	pairScaleMG := 100 + center*p[pairPctMGIndex]/4
	pairScaleEG := 100 + center*p[pairPctEGIndex]/4

	knightMG := continuousPackedTableDot(u.KnightMobility[:], b.Mobility.Knight.MG, p)
	knightEG := continuousPackedTableDot(u.KnightMobility[:], b.Mobility.Knight.EG, p)
	bishopMG := continuousPackedTableDot(u.BishopMobility[:], b.Mobility.Bishop.MG, p)
	bishopEG := continuousPackedTableDot(u.BishopMobility[:], b.Mobility.Bishop.EG, p)
	mg += knightMG * knightScaleMG / 100
	eg += knightEG * knightScaleEG / 100
	mg += bishopMG * bishopScaleMG / 100
	eg += bishopEG * bishopScaleEG / 100
	for index, units := range u.KnightMobility {
		m.addTrainGradient(gradient, b.Mobility.Knight.MG.Offset+index,
			mgAdjoint*float64(units)*knightScaleMG/100)
		m.addTrainGradient(gradient, b.Mobility.Knight.EG.Offset+index,
			egAdjoint*float64(units)*knightScaleEG/100)
	}
	for index, units := range u.BishopMobility {
		m.addTrainGradient(gradient, b.Mobility.Bishop.MG.Offset+index,
			mgAdjoint*float64(units)*bishopScaleMG/100)
		m.addTrainGradient(gradient, b.Mobility.Bishop.EG.Offset+index,
			egAdjoint*float64(units)*bishopScaleEG/100)
	}
	m.addTrainGradient(gradient, knightPctMGIndex, -mgAdjoint*knightMG*center/400)
	m.addTrainGradient(gradient, knightPctEGIndex, -egAdjoint*knightEG*center/400)
	m.addTrainGradient(gradient, bishopPctMGIndex, mgAdjoint*bishopMG*center/400)
	m.addTrainGradient(gradient, bishopPctEGIndex, egAdjoint*bishopEG*center/400)

	bishopPair := float64(u.BishopPair)
	pairMGIndex := b.Piece.BishopPair.MG.Offset
	pairEGIndex := b.Piece.BishopPair.EG.Offset
	mg += bishopPair * p[pairMGIndex] * pairScaleMG / 100
	eg += bishopPair * p[pairEGIndex] * pairScaleEG / 100
	m.addTrainGradient(gradient, pairMGIndex, mgAdjoint*bishopPair*pairScaleMG/100)
	m.addTrainGradient(gradient, pairEGIndex, egAdjoint*bishopPair*pairScaleEG/100)
	m.addTrainGradient(gradient, pairPctMGIndex, mgAdjoint*bishopPair*p[pairMGIndex]*center/400)
	m.addTrainGradient(gradient, pairPctEGIndex, egAdjoint*bishopPair*p[pairEGIndex]*center/400)

	whiteMG, whiteEG := m.continuousPackedDangerGradient(u.DangerWhite, p, mgAdjoint, egAdjoint, gradient)
	blackMG, blackEG := m.continuousPackedDangerGradient(u.DangerBlack, p, -mgAdjoint, -egAdjoint, gradient)
	mg += whiteMG - blackMG
	eg += whiteEG - blackEG

	for _, passer := range shard.kingPassers(record.KingPassers) {
		rank := float64(passer.RelativeRank)
		rankSquared := rank * rank
		side := float64(passer.Side)
		enemyDistance := float64(passer.EnemyDistance)
		ownDistance := float64(passer.OwnDistance)
		enemyIndex := b.KingPasser.EnemyWeight.Offset
		ownIndex := b.KingPasser.OwnWeight.Offset
		proximityIndex := b.KingPasser.ProximityEG.Offset
		divisorIndex := b.KingPasser.Divisor.Offset
		delta := enemyDistance*p[enemyIndex] - ownDistance*p[ownIndex]
		proximity := p[proximityIndex]
		divisor := p[divisorIndex]
		contribution := side * delta * rankSquared * proximity / divisor
		eg += contribution
		common := egAdjoint * side * rankSquared
		m.addTrainGradient(gradient, enemyIndex, common*enemyDistance*proximity/divisor)
		m.addTrainGradient(gradient, ownIndex, -common*ownDistance*proximity/divisor)
		m.addTrainGradient(gradient, proximityIndex, common*delta/divisor)
		m.addTrainGradient(gradient, divisorIndex, -common*delta*proximity/(divisor*divisor))
	}

	blockedCapIndex := b.Space.BlockedCap.Offset
	blockedInput := float64(u.SpaceBlocked)
	blockedCap := p[blockedCapIndex]
	blocked := math.Min(blockedInput, blockedCap)
	blockedCapDerivative := 0.0
	if blockedCap <= blockedInput {
		blockedCapDerivative = 1
	}
	whiteRaw := continuousPackedSpaceRaw(u.SpaceWhite, b.Space, p)
	blackRaw := continuousPackedSpaceRaw(u.SpaceBlack, b.Space, p)
	offsetIndex := b.Space.WeightOffset.Offset
	whiteWeightInput := float64(u.SpaceWhite.PieceCount) - p[offsetIndex] + blocked
	blackWeightInput := float64(u.SpaceBlack.PieceCount) - p[offsetIndex] + blocked
	whiteWeight := math.Max(0, whiteWeightInput)
	blackWeight := math.Max(0, blackWeightInput)
	whiteWeightDerivative, blackWeightDerivative := 0.0, 0.0
	if whiteWeightInput > 0 {
		whiteWeightDerivative = 1
	}
	if blackWeightInput > 0 {
		blackWeightDerivative = 1
	}
	whiteSquared := whiteWeight * whiteWeight
	blackSquared := blackWeight * blackWeight
	spaceNumerator := whiteRaw*whiteSquared - blackRaw*blackSquared
	spaceDivisorIndex := b.Space.WeightDivisor.Offset
	spaceDivisor := p[spaceDivisorIndex]
	mg += spaceNumerator / spaceDivisor
	spaceRawGradient := func(index int, whiteUnits, blackUnits int16) {
		derivative := (float64(whiteUnits)*whiteSquared - float64(blackUnits)*blackSquared) / spaceDivisor
		m.addTrainGradient(gradient, index, mgAdjoint*derivative)
	}
	spaceRawGradient(b.Space.SafeMG.Offset, u.SpaceWhite.Safe, u.SpaceBlack.Safe)
	spaceRawGradient(b.Space.BehindPawnMG.Offset, u.SpaceWhite.BehindPawn, u.SpaceBlack.BehindPawn)
	spaceRawGradient(b.Space.SemiOpenMG.Offset, u.SpaceWhite.SemiOpen, u.SpaceBlack.SemiOpen)
	spaceRawGradient(b.Space.OpenMG.Offset, u.SpaceWhite.Open, u.SpaceBlack.Open)
	weightDerivative := 2*whiteRaw*whiteWeight*whiteWeightDerivative -
		2*blackRaw*blackWeight*blackWeightDerivative
	m.addTrainGradient(gradient, offsetIndex, -mgAdjoint*weightDerivative/spaceDivisor)
	m.addTrainGradient(gradient, blockedCapIndex, mgAdjoint*weightDerivative*blockedCapDerivative/spaceDivisor)
	m.addTrainGradient(gradient, spaceDivisorIndex, -mgAdjoint*spaceNumerator/(spaceDivisor*spaceDivisor))

	referenceIndex := b.Imbalance.ReferencePawnCount.Offset
	knightMGIndex := b.Imbalance.KnightPerPawn.MG.Offset
	knightEGIndex := b.Imbalance.KnightPerPawn.EG.Offset
	knightDifference := float64(u.KnightDiff)
	imbalanceUnits := (float64(u.TotalPawns) - p[referenceIndex]) * knightDifference
	mg += imbalanceUnits * p[knightMGIndex]
	eg += imbalanceUnits * p[knightEGIndex]
	m.addTrainGradient(gradient, knightMGIndex, mgAdjoint*imbalanceUnits)
	m.addTrainGradient(gradient, knightEGIndex, egAdjoint*imbalanceUnits)
	m.addTrainGradient(gradient, referenceIndex, -knightDifference*(mgAdjoint*p[knightMGIndex]+egAdjoint*p[knightEGIndex]))

	mg, eg = baseMG+mg, baseEG+eg
	preScaleScore := (mg*phase + eg*(totalPhase-phase)) / totalPhase
	score := preScaleScore
	if record.TheoreticalDraw() {
		dividerIndex := b.DrawDivider.Offset
		divider := p[dividerIndex]
		score /= divider
		m.addTrainGradient(gradient, dividerIndex, -scoreAdjoint*preScaleScore/(divider*divider))
	}
	result := ContinuousForwardResult{
		MG: mg, EG: eg, WhitePerspective: score,
		SideToMove: score * float64(record.SideToMove),
	}
	if !finite(result.MG) || !finite(result.EG) || !finite(result.WhitePerspective) || !finite(result.SideToMove) {
		return ContinuousForwardResult{}, fmt.Errorf("continuous packed gradient result is not finite")
	}
	return result, nil
}

func (m *ForwardModel) continuousPackedDangerGradient(side PackedDangerSide, p []float64, mgAdjoint, egAdjoint float64, gradient *trainGradientAccumulator) (mg, eg float64) {
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
	rawMG, rawEG := mg, eg
	mg = math.Max(0, rawMG)
	eg = math.Max(0, rawEG)
	mgDivisorIndex := b.Divisor.MG.Offset
	egDivisorIndex := b.Divisor.EG.Offset
	mgDivisor := p[mgDivisorIndex]
	egDivisor := p[egDivisorIndex]
	mg = mg * mg / (mgDivisor * mgDivisor)
	eg = eg / egDivisor
	rawMGAdjoint, rawEGAdjoint := 0.0, 0.0
	if rawMG > 0 {
		rawMGAdjoint = mgAdjoint * 2 * rawMG / (mgDivisor * mgDivisor)
		m.addTrainGradient(gradient, mgDivisorIndex, -mgAdjoint*2*rawMG*rawMG/(mgDivisor*mgDivisor*mgDivisor))
	}
	if rawEG > 0 {
		rawEGAdjoint = egAdjoint / egDivisor
		m.addTrainGradient(gradient, egDivisorIndex, -egAdjoint*rawEG/(egDivisor*egDivisor))
	}
	m.addTrainGradient(gradient, b.Adjustment.MG.Offset, rawMGAdjoint)
	m.addTrainGradient(gradient, b.Adjustment.EG.Offset, rawEGAdjoint)
	m.addTrainGradient(gradient, b.AttackValue.MG.Offset, rawMGAdjoint*float64(side.RingHits))
	m.addTrainGradient(gradient, b.AttackValue.EG.Offset, rawEGAdjoint*float64(side.RingHits))
	for kind := 0; kind < 4; kind++ {
		m.addTrainGradient(gradient, b.AttackerWeight[kind].MG.Offset, rawMGAdjoint*float64(side.Attackers[kind]))
		m.addTrainGradient(gradient, b.AttackerWeight[kind].EG.Offset, rawEGAdjoint*float64(side.Attackers[kind]))
		m.addTrainGradient(gradient, b.SafeCheck[kind].MG.Offset, rawMGAdjoint*float64(side.SafeChecks[kind]))
		m.addTrainGradient(gradient, b.SafeCheck[kind].EG.Offset, rawEGAdjoint*float64(side.SafeChecks[kind]))
	}
	m.addTrainGradient(gradient, b.UnsafeCheck.MG.Offset, rawMGAdjoint*float64(side.UnsafeChecks))
	m.addTrainGradient(gradient, b.UnsafeCheck.EG.Offset, rawEGAdjoint*float64(side.UnsafeChecks))
	if !side.HasQueen {
		m.addTrainGradient(gradient, b.NoEnemyQueen.MG.Offset, rawMGAdjoint)
		m.addTrainGradient(gradient, b.NoEnemyQueen.EG.Offset, rawEGAdjoint)
	}
	return mg, eg
}

func (m *ForwardModel) addTrainGradient(gradient *trainGradientAccumulator, parameterIndex int, value float64) {
	trainIndex := m.trainIndexByParameter[parameterIndex]
	if trainIndex != NoTrainIndex {
		gradient.add(trainIndex, value)
	}
}
