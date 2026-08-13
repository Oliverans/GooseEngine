package tuner

import (
	"errors"
	"fmt"
)

type PackedBatchMetrics struct {
	Data       DataLossMetrics
	AnchorLoss float64
	TotalLoss  float64
	Records    uint64
	Weight     float64
}

// PackedBatchAccumulator is the single-threaded, reusable batch workspace. It
// owns a touched-index scratch gradient so each position is differentiated
// once and only touched coordinates are reduced into the batch gradient.
type PackedBatchAccumulator struct {
	model   *ForwardModel
	link    TexelLink
	anchor  *ParameterAnchorModel
	scratch trainGradientAccumulator
}

func NewPackedBatchAccumulator(model *ForwardModel, link TexelLink, anchor *ParameterAnchorModel) (*PackedBatchAccumulator, error) {
	if model == nil || model.binding == nil {
		return nil, errors.New("packed batch accumulation requires a forward model")
	}
	if _, err := NewTexelLink(link.K); err != nil {
		return nil, err
	}
	if anchor != nil && (anchor.parameterCount != model.parameterCount || anchor.trainableCount != model.trainableCount) {
		return nil, errors.New("parameter-anchor model dimensions do not match the forward model")
	}
	return &PackedBatchAccumulator{
		model: model, link: link, anchor: anchor,
		scratch: newTrackedGradientAccumulator(model.trainableCount),
	}, nil
}

// Compute clears and fills trainGradient. A nil recordIndexes slice selects
// every shard record in storage order; a non-nil slice supplies an explicit
// batch order and may contain repeated records. Weights may be nil for uniform
// weight one, or must contain one non-negative weight per selected record.
func (a *PackedBatchAccumulator) Compute(shard *PackedDatasetShard, recordIndexes []uint32, weights []float64, parameters []float64, trainGradient []float64) (PackedBatchMetrics, error) {
	if a == nil || a.model == nil {
		return PackedBatchMetrics{}, errors.New("cannot compute with a nil packed batch accumulator")
	}
	if shard == nil {
		return PackedBatchMetrics{}, errors.New("packed batch requires a shard")
	}
	if len(trainGradient) != a.model.trainableCount {
		return PackedBatchMetrics{}, fmt.Errorf("train gradient has length %d, want %d", len(trainGradient), a.model.trainableCount)
	}
	if err := a.model.ValidateContinuousParameters(parameters); err != nil {
		return PackedBatchMetrics{}, err
	}
	sampleCount := len(recordIndexes)
	if recordIndexes == nil {
		sampleCount = len(shard.Records)
	}
	if sampleCount == 0 {
		return PackedBatchMetrics{}, errors.New("packed batch contains no records")
	}
	if weights != nil && len(weights) != sampleCount {
		return PackedBatchMetrics{}, fmt.Errorf("batch weights have length %d, want %d", len(weights), sampleCount)
	}
	for sample := 0; sample < sampleCount; sample++ {
		recordIndex := sample
		if recordIndexes != nil {
			recordIndex = int(recordIndexes[sample])
		}
		if recordIndex < 0 || recordIndex >= len(shard.Records) {
			return PackedBatchMetrics{}, fmt.Errorf("batch record index %d outside [0,%d)", recordIndex, len(shard.Records))
		}
		weight := 1.0
		if weights != nil {
			weight = weights[sample]
		}
		if !finite(weight) || weight < 0 {
			return PackedBatchMetrics{}, fmt.Errorf("batch weight %d must be finite and non-negative, got %v", sample, weight)
		}
	}

	clear(trainGradient)
	var losses OutcomeLossAccumulator
	for sample := 0; sample < sampleCount; sample++ {
		recordIndex := sample
		if recordIndexes != nil {
			recordIndex = int(recordIndexes[sample])
		}
		weight := 1.0
		if weights != nil {
			weight = weights[sample]
		}
		if weight == 0 {
			continue
		}
		record := &shard.Records[recordIndex]
		a.scratch.beginRecord()
		forward, err := a.model.continuousPackedGradientRecord(shard, record, parameters, 1, &a.scratch)
		if err != nil {
			a.discardFailedBatch(trainGradient)
			return PackedBatchMetrics{}, fmt.Errorf("batch record %d: %w", recordIndex, err)
		}
		sampleLoss, err := a.link.Evaluate(forward.WhitePerspective, record.Outcome)
		if err != nil {
			a.discardFailedBatch(trainGradient)
			return PackedBatchMetrics{}, fmt.Errorf("batch record %d loss: %w", recordIndex, err)
		}
		losses.total.add(sampleLoss, weight)
		losses.byOutcome[record.Outcome].add(sampleLoss, weight)
		a.scratch.reduceInto(trainGradient, weight*sampleLoss.BrierDerivativeEval)
	}
	data, err := losses.Metrics()
	if err != nil {
		a.discardFailedBatch(trainGradient)
		return PackedBatchMetrics{}, err
	}
	for index := range trainGradient {
		trainGradient[index] /= data.Weight
	}
	anchorLoss := 0.0
	if a.anchor != nil {
		anchorLoss, err = a.anchor.Penalty(parameters, trainGradient)
		if err != nil {
			clear(trainGradient)
			return PackedBatchMetrics{}, err
		}
	}
	for index, value := range trainGradient {
		if !finite(value) {
			clear(trainGradient)
			return PackedBatchMetrics{}, fmt.Errorf("batch gradient %d is not finite", index)
		}
	}
	return PackedBatchMetrics{
		Data: data, AnchorLoss: anchorLoss, TotalLoss: data.Brier + anchorLoss,
		Records: data.Samples, Weight: data.Weight,
	}, nil
}

func (a *PackedBatchAccumulator) discardFailedBatch(trainGradient []float64) {
	clear(trainGradient)
	for _, index := range a.scratch.touched {
		a.scratch.values[index] = 0
	}
	a.scratch.touched = a.scratch.touched[:0]
}
