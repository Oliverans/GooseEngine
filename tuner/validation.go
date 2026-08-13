package tuner

import (
	"errors"
	"fmt"
	"sort"
)

type SourceValidationMetrics struct {
	SourceID string
	Data     DataLossMetrics
}

type ValidationMetrics struct {
	Split     DatasetSplit
	Shards    int
	Data      DataLossMetrics
	BySource  []SourceValidationMetrics
	Anchor    ParameterAnchorMetrics
	TotalLoss float64
}

// Validate performs a read-only, canonical-order pass. Data metrics remain
// separate from anchoring so model selection always sees unregularized fit.
func (t *Trainer) Validate(dataset LoadedDatasetManifest, split DatasetSplit) (ValidationMetrics, error) {
	if t == nil || t.registry == nil || t.model == nil {
		return ValidationMetrics{}, errors.New("cannot validate with a nil trainer")
	}
	if !split.valid() {
		return ValidationMetrics{}, fmt.Errorf("invalid validation split %d", split)
	}
	if dataset.Data.RegistryFingerprint != t.registry.Fingerprint {
		return ValidationMetrics{}, fmt.Errorf("dataset registry fingerprint %q, want %q", dataset.Data.RegistryFingerprint, t.registry.Fingerprint)
	}
	return t.ValidateContinuousParameters(dataset, split, t.parameters)
}

func (t *Trainer) ValidateContinuousParameters(dataset LoadedDatasetManifest, split DatasetSplit, parameters []float64) (ValidationMetrics, error) {
	if t == nil || t.registry == nil || t.model == nil {
		return ValidationMetrics{}, errors.New("cannot validate with a nil trainer")
	}
	if len(parameters) != len(t.registry.Elements) {
		return ValidationMetrics{}, fmt.Errorf("continuous parameter vector has length %d, want %d", len(parameters), len(t.registry.Elements))
	}
	return t.validateParameters(dataset, split, parameters, func(shard *PackedDatasetShard, index int) (float64, error) {
		result, err := t.model.ContinuousPacked(shard, index, parameters)
		return result.WhitePerspective, err
	})
}

// ValidateExact reports the same metrics after integer quantization using the
// engine-order exact forward pass.
func (t *Trainer) ValidateExact(dataset LoadedDatasetManifest, split DatasetSplit, parameters []int) (ValidationMetrics, error) {
	if t == nil || t.registry == nil || t.model == nil {
		return ValidationMetrics{}, errors.New("cannot validate with a nil trainer")
	}
	if !split.valid() {
		return ValidationMetrics{}, fmt.Errorf("invalid validation split %d", split)
	}
	if len(parameters) != len(t.registry.Elements) {
		return ValidationMetrics{}, fmt.Errorf("exact parameter vector has length %d, want %d", len(parameters), len(t.registry.Elements))
	}
	continuous := make([]float64, len(parameters))
	for index, value := range parameters {
		continuous[index] = float64(value)
	}
	return t.validateParameters(dataset, split, continuous, func(shard *PackedDatasetShard, index int) (float64, error) {
		result, err := t.model.EngineExactPacked(shard, index, parameters)
		return float64(result.WhitePerspective), err
	})
}

func (t *Trainer) validateParameters(dataset LoadedDatasetManifest, split DatasetSplit, anchorParameters []float64, evaluate func(*PackedDatasetShard, int) (float64, error)) (ValidationMetrics, error) {
	if dataset.Data.RegistryFingerprint != t.registry.Fingerprint {
		return ValidationMetrics{}, fmt.Errorf("dataset registry fingerprint %q, want %q", dataset.Data.RegistryFingerprint, t.registry.Fingerprint)
	}
	name := splitName(split)
	shards := make([]ManifestShard, 0)
	for _, shard := range dataset.Data.Shards {
		if shard.Split == name {
			shards = append(shards, shard)
		}
	}
	if len(shards) == 0 {
		return ValidationMetrics{}, fmt.Errorf("manifest has no %s shards", name)
	}
	sort.Slice(shards, func(i, j int) bool { return shards[i].Path < shards[j].Path })

	var overall OutcomeLossAccumulator
	bySource := make(map[string]*OutcomeLossAccumulator)
	for _, metadata := range shards {
		shard, err := LoadPackedDatasetShard(dataset, metadata, t.registry)
		if err != nil {
			return ValidationMetrics{}, fmt.Errorf("load %s shard %q: %w", name, metadata.Path, err)
		}
		source := bySource[metadata.SourceID]
		if source == nil {
			source = &OutcomeLossAccumulator{}
			bySource[metadata.SourceID] = source
		}
		for index := range shard.Records {
			evaluation, err := evaluate(&shard, index)
			if err != nil {
				return ValidationMetrics{}, fmt.Errorf("%s shard %q record %d: %w", name, metadata.Path, index, err)
			}
			outcome := shard.Records[index].Outcome
			if _, err := overall.Add(t.link, evaluation, outcome, 1); err != nil {
				return ValidationMetrics{}, err
			}
			if _, err := source.Add(t.link, evaluation, outcome, 1); err != nil {
				return ValidationMetrics{}, err
			}
		}
	}
	data, err := overall.Metrics()
	if err != nil {
		return ValidationMetrics{}, err
	}
	metrics := ValidationMetrics{Split: split, Shards: len(shards), Data: data}
	sourceIDs := make([]string, 0, len(bySource))
	for sourceID := range bySource {
		sourceIDs = append(sourceIDs, sourceID)
	}
	sort.Strings(sourceIDs)
	metrics.BySource = make([]SourceValidationMetrics, 0, len(sourceIDs))
	for _, sourceID := range sourceIDs {
		sourceData, err := bySource[sourceID].Metrics()
		if err != nil {
			return ValidationMetrics{}, err
		}
		metrics.BySource = append(metrics.BySource, SourceValidationMetrics{SourceID: sourceID, Data: sourceData})
	}
	if t.anchor != nil {
		metrics.Anchor, err = t.anchor.Evaluate(anchorParameters, nil)
		if err != nil {
			return ValidationMetrics{}, err
		}
	}
	metrics.TotalLoss = metrics.Data.Brier + metrics.Anchor.Loss
	return metrics, nil
}
