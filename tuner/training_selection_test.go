package tuner

import (
	"slices"
	"strings"
	"testing"
)

func TestTrainingSelectionIsSourceProportionalAndStableAcrossEpochs(t *testing.T) {
	dataset := LoadedDatasetManifest{
		SHA256: strings.Repeat("ab", 32),
		Data: DatasetManifest{
			Shuffle:    ManifestShuffle{Seed: 91},
			Statistics: ManifestStatistics{Splits: DatasetSplitCounts{Training: 1000}},
			Shards: []ManifestShard{
				{Path: "train/a-0.bin", SourceID: "a", Split: "train", Records: 400},
				{Path: "train/a-1.bin", SourceID: "a", Split: "train", Records: 400},
				{Path: "train/b-0.bin", SourceID: "b", Split: "train", Records: 100},
				{Path: "train/b-1.bin", SourceID: "b", Split: "train", Records: 100},
			},
		},
	}
	selection, err := deterministicTrainingSelection(dataset, 100)
	if err != nil {
		t.Fatal(err)
	}
	bySource := map[string]uint64{}
	var total uint64
	for _, shard := range selection {
		bySource[shard.Metadata.SourceID] += shard.Records
		total += shard.Records
	}
	if total != 100 || bySource["a"] != 80 || bySource["b"] != 20 {
		t.Fatalf("unexpected source quotas: total=%d sources=%v", total, bySource)
	}

	firstEpoch, err := deterministicEpochTrainingSelection(dataset, 100, 0)
	if err != nil {
		t.Fatal(err)
	}
	secondEpoch, err := deterministicEpochTrainingSelection(dataset, 100, 1)
	if err != nil {
		t.Fatal(err)
	}
	firstByPath := make(map[string][]uint32)
	for _, shard := range firstEpoch {
		permutation, err := deterministicSelectedRecordPermutation(dataset, shard, 0)
		if err != nil {
			t.Fatal(err)
		}
		firstByPath[shard.Metadata.Path] = permutation
	}
	orderChanged := false
	for _, shard := range secondEpoch {
		permutation, err := deterministicSelectedRecordPermutation(dataset, shard, 1)
		if err != nil {
			t.Fatal(err)
		}
		first := append([]uint32(nil), firstByPath[shard.Metadata.Path]...)
		second := append([]uint32(nil), permutation...)
		if !slices.Equal(first, second) {
			orderChanged = true
		}
		slices.Sort(first)
		slices.Sort(second)
		if !slices.Equal(first, second) {
			t.Fatalf("epoch changed selected membership for %s", shard.Metadata.Path)
		}
	}
	if !orderChanged {
		t.Fatal("epoch shuffle changed no selected record order")
	}
}

func TestTrainingSelectionRejectsOversizedLimit(t *testing.T) {
	dataset := LoadedDatasetManifest{
		SHA256: strings.Repeat("cd", 32),
		Data: DatasetManifest{
			Statistics: ManifestStatistics{Splits: DatasetSplitCounts{Training: 10}},
			Shards:     []ManifestShard{{Path: "train/a.bin", SourceID: "a", Split: "train", Records: 10}},
		},
	}
	if _, err := deterministicEpochTrainingSelection(dataset, 11, 0); err == nil {
		t.Fatal("oversized record limit unexpectedly accepted")
	}
}
