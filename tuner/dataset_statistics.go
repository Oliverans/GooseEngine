package tuner

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
)

type LoadedDatasetStatistics struct {
	Data     DatasetStats
	Coverage []uint64
}

// LoadDatasetStatistics validates statistics.json against both the manifest
// and registry and returns registry-ordered coverage counts for optimizer
// masking.
func LoadDatasetStatistics(dataset LoadedDatasetManifest, registry *Registry) (LoadedDatasetStatistics, error) {
	if registry == nil {
		return LoadedDatasetStatistics{}, errors.New("dataset statistics loading requires a registry")
	}
	path, err := safeShardPath(dataset.Root, dataset.Data.StatisticsFile)
	if err != nil {
		return LoadedDatasetStatistics{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return LoadedDatasetStatistics{}, err
	}
	var statistics DatasetStats
	if err := json.Unmarshal(data, &statistics); err != nil {
		return LoadedDatasetStatistics{}, err
	}
	if statistics.RegistryFingerprint != registry.Fingerprint {
		return LoadedDatasetStatistics{}, fmt.Errorf("statistics registry fingerprint %q, want %q", statistics.RegistryFingerprint, registry.Fingerprint)
	}
	if statistics.Records != dataset.Data.Statistics.Records || statistics.Splits != dataset.Data.Statistics.Splits || statistics.Outcomes != dataset.Data.Statistics.Outcomes {
		return LoadedDatasetStatistics{}, errors.New("statistics totals do not match the manifest")
	}
	if len(statistics.FeatureFrequency) != len(registry.Elements) {
		return LoadedDatasetStatistics{}, fmt.Errorf("statistics contain %d feature frequencies, want %d", len(statistics.FeatureFrequency), len(registry.Elements))
	}
	coverage := make([]uint64, len(registry.Elements))
	seen := make([]bool, len(registry.Elements))
	for _, frequency := range statistics.FeatureFrequency {
		if frequency.Index < 0 || frequency.Index >= len(registry.Elements) {
			return LoadedDatasetStatistics{}, fmt.Errorf("feature frequency index %d outside [0,%d)", frequency.Index, len(registry.Elements))
		}
		if seen[frequency.Index] {
			return LoadedDatasetStatistics{}, fmt.Errorf("feature frequency index %d is duplicated", frequency.Index)
		}
		element := registry.Elements[frequency.Index]
		if frequency.ID != element.ID || !slices.Equal(frequency.Coordinate, element.Coordinate) {
			return LoadedDatasetStatistics{}, fmt.Errorf("feature frequency index %d identifies %q%v, want %q%v",
				frequency.Index, frequency.ID, frequency.Coordinate, element.ID, element.Coordinate)
		}
		if frequency.Records > statistics.Records {
			return LoadedDatasetStatistics{}, fmt.Errorf("feature frequency index %d has %d records, dataset has %d",
				frequency.Index, frequency.Records, statistics.Records)
		}
		seen[frequency.Index] = true
		coverage[frequency.Index] = frequency.Records
	}
	return LoadedDatasetStatistics{Data: statistics, Coverage: coverage}, nil
}
