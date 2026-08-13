package tuner

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/bits"
	"path/filepath"
	"sort"
)

type trainingShardSelection struct {
	Metadata ManifestShard
	Records  uint64
}

type sourceSelectionQuota struct {
	id        string
	records   uint64
	quota     uint64
	remainder uint64
}

type TrainingSelectionShard struct {
	Path             string `json:"path"`
	SourceID         string `json:"sourceId"`
	AvailableRecords uint64 `json:"availableRecords"`
	SelectedRecords  uint64 `json:"selectedRecords"`
}

type TrainingSelectionSource struct {
	SourceID        string `json:"sourceId"`
	SelectedRecords uint64 `json:"selectedRecords"`
}

type TrainingSelectionSummary struct {
	DatasetTrainingRecords uint64                    `json:"datasetTrainingRecords"`
	SelectedRecords        uint64                    `json:"selectedRecords"`
	Sources                []TrainingSelectionSource `json:"sources"`
	Shards                 []TrainingSelectionShard  `json:"shards"`
}

func BuildTrainingSelectionSummary(dataset LoadedDatasetManifest, limit uint64) (TrainingSelectionSummary, error) {
	selection, err := deterministicTrainingSelection(dataset, limit)
	if err != nil {
		return TrainingSelectionSummary{}, err
	}
	summary := TrainingSelectionSummary{DatasetTrainingRecords: dataset.Data.Statistics.Splits.Training}
	bySource := make(map[string]uint64)
	for _, selected := range selection {
		summary.SelectedRecords += selected.Records
		bySource[selected.Metadata.SourceID] += selected.Records
		summary.Shards = append(summary.Shards, TrainingSelectionShard{
			Path: selected.Metadata.Path, SourceID: selected.Metadata.SourceID,
			AvailableRecords: selected.Metadata.Records, SelectedRecords: selected.Records,
		})
	}
	sourceIDs := make([]string, 0, len(bySource))
	for sourceID := range bySource {
		sourceIDs = append(sourceIDs, sourceID)
	}
	sort.Strings(sourceIDs)
	for _, sourceID := range sourceIDs {
		summary.Sources = append(summary.Sources, TrainingSelectionSource{SourceID: sourceID, SelectedRecords: bySource[sourceID]})
	}
	return summary, nil
}

// deterministicTrainingSelection fixes a source-proportional subset before
// epoch shuffling. Within each source, manifest shards are ranked by a stable
// hash, avoiding dependence on manifest serialization or source file order.
func deterministicTrainingSelection(dataset LoadedDatasetManifest, limit uint64) ([]trainingShardSelection, error) {
	training := make([]ManifestShard, 0)
	var total uint64
	bySource := make(map[string][]ManifestShard)
	for _, shard := range dataset.Data.Shards {
		if shard.Split != splitName(SplitTraining) {
			continue
		}
		if ^uint64(0)-total < shard.Records {
			return nil, errors.New("training record total overflows uint64")
		}
		total += shard.Records
		training = append(training, shard)
		bySource[shard.SourceID] = append(bySource[shard.SourceID], shard)
	}
	if total == 0 {
		return nil, errors.New("manifest has no training records")
	}
	if limit > total {
		return nil, fmt.Errorf("requested %d training records per epoch, dataset has %d", limit, total)
	}
	if limit == 0 || limit == total {
		sort.Slice(training, func(i, j int) bool { return training[i].Path < training[j].Path })
		selected := make([]trainingShardSelection, len(training))
		for index, shard := range training {
			selected[index] = trainingShardSelection{Metadata: shard, Records: shard.Records}
		}
		return selected, nil
	}

	quotas := make([]sourceSelectionQuota, 0, len(bySource))
	for sourceID, shards := range bySource {
		var records uint64
		for _, shard := range shards {
			records += shard.Records
		}
		quotient, remainder := multipliedDivision(limit, records, total)
		quotas = append(quotas, sourceSelectionQuota{id: sourceID, records: records, quota: quotient, remainder: remainder})
	}
	sort.Slice(quotas, func(i, j int) bool { return quotas[i].id < quotas[j].id })
	var assigned uint64
	for _, quota := range quotas {
		assigned += quota.quota
	}
	remaining := limit - assigned
	sort.SliceStable(quotas, func(i, j int) bool {
		if quotas[i].remainder != quotas[j].remainder {
			return quotas[i].remainder > quotas[j].remainder
		}
		return quotas[i].id < quotas[j].id
	})
	for index := uint64(0); index < remaining; index++ {
		quotas[index].quota++
	}

	selected := make([]trainingShardSelection, 0, len(quotas))
	for _, source := range quotas {
		if source.quota == 0 {
			continue
		}
		shards := append([]ManifestShard(nil), bySource[source.id]...)
		sort.Slice(shards, func(i, j int) bool {
			left := selectionShardRank(dataset, source.id, shards[i].Path)
			right := selectionShardRank(dataset, source.id, shards[j].Path)
			if comparison := bytes.Compare(left[:], right[:]); comparison != 0 {
				return comparison < 0
			}
			return shards[i].Path < shards[j].Path
		})
		needed := source.quota
		for _, shard := range shards {
			count := min(needed, shard.Records)
			selected = append(selected, trainingShardSelection{Metadata: shard, Records: count})
			needed -= count
			if needed == 0 {
				break
			}
		}
		if needed != 0 {
			return nil, fmt.Errorf("source %q selection is short by %d records", source.id, needed)
		}
	}
	sort.Slice(selected, func(i, j int) bool { return selected[i].Metadata.Path < selected[j].Metadata.Path })
	return selected, nil
}

func multipliedDivision(left, right, divisor uint64) (uint64, uint64) {
	high, low := bits.Mul64(left, right)
	return bits.Div64(high, low, divisor)
}

func selectionShardRank(dataset LoadedDatasetManifest, sourceID, shardPath string) [sha256.Size]byte {
	return sha256.Sum256([]byte("goose-tuner-training-subset-shard-v1\x00" + dataset.SHA256 + "\x00" + sourceID + "\x00" + filepath.ToSlash(shardPath)))
}

func deterministicEpochTrainingSelection(dataset LoadedDatasetManifest, limit, epoch uint64) ([]trainingShardSelection, error) {
	if limit > dataset.Data.Statistics.Splits.Training {
		return nil, fmt.Errorf("requested %d training records per epoch, dataset has %d", limit, dataset.Data.Statistics.Splits.Training)
	}
	if limit == 0 || limit == dataset.Data.Statistics.Splits.Training {
		shards, err := DeterministicEpochShards(dataset, SplitTraining, epoch)
		if err != nil {
			return nil, err
		}
		selected := make([]trainingShardSelection, len(shards))
		for index, shard := range shards {
			selected[index] = trainingShardSelection{Metadata: shard, Records: shard.Records}
		}
		return selected, nil
	}
	selected, err := deterministicTrainingSelection(dataset, limit)
	if err != nil {
		return nil, err
	}
	rng := splitMix64{state: selectionSeed(dataset, "goose-tuner-training-subset-shards-epoch-v1", epoch, "")}
	for index := len(selected) - 1; index > 0; index-- {
		other := int(rng.next() % uint64(index+1))
		selected[index], selected[other] = selected[other], selected[index]
	}
	return selected, nil
}

func deterministicSelectedRecordPermutation(dataset LoadedDatasetManifest, selection trainingShardSelection, epoch uint64) ([]uint32, error) {
	if selection.Records == 0 || selection.Records > selection.Metadata.Records {
		return nil, fmt.Errorf("shard %q selects %d of %d records", selection.Metadata.Path, selection.Records, selection.Metadata.Records)
	}
	if selection.Records == selection.Metadata.Records {
		return DeterministicRecordPermutation(selection.Metadata.Records, dataset.Data.Shuffle.Seed, dataset.SHA256, epoch, selection.Metadata.Path)
	}
	if selection.Metadata.Records > math.MaxUint32 {
		return nil, fmt.Errorf("record count %d exceeds uint32 permutation indexes", selection.Metadata.Records)
	}
	indexes := make([]uint32, int(selection.Metadata.Records))
	for index := range indexes {
		indexes[index] = uint32(index)
	}
	selectionRNG := splitMix64{state: selectionSeed(dataset, "goose-tuner-training-subset-records-v1", 0, selection.Metadata.Path)}
	for index := len(indexes) - 1; index > 0; index-- {
		other := int(selectionRNG.next() % uint64(index+1))
		indexes[index], indexes[other] = indexes[other], indexes[index]
	}
	indexes = indexes[:int(selection.Records)]
	epochRNG := splitMix64{state: selectionSeed(dataset, "goose-tuner-training-subset-records-epoch-v1", epoch, selection.Metadata.Path)}
	for index := len(indexes) - 1; index > 0; index-- {
		other := int(epochRNG.next() % uint64(index+1))
		indexes[index], indexes[other] = indexes[other], indexes[index]
	}
	return indexes, nil
}

func selectionSeed(dataset LoadedDatasetManifest, domain string, epoch uint64, shardPath string) uint64 {
	hasher := sha256.New()
	_, _ = hasher.Write([]byte(domain))
	_, _ = hasher.Write([]byte{0})
	var number [8]byte
	binary.LittleEndian.PutUint64(number[:], dataset.Data.Shuffle.Seed)
	_, _ = hasher.Write(number[:])
	_, _ = hasher.Write([]byte(dataset.SHA256))
	binary.LittleEndian.PutUint64(number[:], epoch)
	_, _ = hasher.Write(number[:])
	_, _ = hasher.Write([]byte(filepath.ToSlash(shardPath)))
	digest := hasher.Sum(nil)
	return binary.LittleEndian.Uint64(digest[:8])
}
