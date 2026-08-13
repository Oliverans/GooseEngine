package tuner

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"

	eng "chess-engine/engine"
)

type LoadedDatasetManifest struct {
	Root   string
	Data   DatasetManifest
	SHA256 string
}

type LoadedDatasetShard struct {
	Metadata ManifestShard
	Header   DatasetHeader
	Records  []CompiledTrainingRecord
}

// DeterministicEpochShards returns one split's shards in a stable epoch-specific
// order. The canonical input order is path-sorted before Fisher-Yates shuffling,
// so manifest serialization order is not an implicit training input.
func DeterministicEpochShards(dataset LoadedDatasetManifest, split DatasetSplit, epoch uint64) ([]ManifestShard, error) {
	if !split.valid() {
		return nil, fmt.Errorf("invalid dataset split %d", split)
	}
	manifestDigest, err := hex.DecodeString(dataset.SHA256)
	if err != nil || len(manifestDigest) != sha256.Size {
		return nil, fmt.Errorf("manifest SHA-256 %q is invalid", dataset.SHA256)
	}
	splitName := splitName(split)
	shards := make([]ManifestShard, 0, len(dataset.Data.Shards))
	for _, shard := range dataset.Data.Shards {
		if shard.Split == splitName {
			shards = append(shards, shard)
		}
	}
	if len(shards) == 0 {
		return nil, fmt.Errorf("manifest has no %s shards", splitName)
	}
	sort.Slice(shards, func(i, j int) bool { return shards[i].Path < shards[j].Path })
	hasher := sha256.New()
	_, _ = hasher.Write([]byte("goose-tuner-shard-permutation-v1\x00"))
	var number [8]byte
	binary.LittleEndian.PutUint64(number[:], dataset.Data.Shuffle.Seed)
	_, _ = hasher.Write(number[:])
	_, _ = hasher.Write(manifestDigest)
	binary.LittleEndian.PutUint64(number[:], epoch)
	_, _ = hasher.Write(number[:])
	_, _ = hasher.Write([]byte(splitName))
	seedDigest := hasher.Sum(nil)
	rng := splitMix64{state: binary.LittleEndian.Uint64(seedDigest[:8])}
	for index := len(shards) - 1; index > 0; index-- {
		other := int(rng.next() % uint64(index+1))
		shards[index], shards[other] = shards[other], shards[index]
	}
	return shards, nil
}

// LoadDatasetManifest verifies the exact manifest bytes used as the training
// resume identity and validates their registry/schema compatibility.
func LoadDatasetManifest(root string, registry *Registry) (LoadedDatasetManifest, error) {
	if registry == nil {
		return LoadedDatasetManifest{}, errors.New("dataset manifest loading requires a registry")
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return LoadedDatasetManifest{}, err
	}
	data, err := os.ReadFile(filepath.Join(absRoot, "manifest.json"))
	if err != nil {
		return LoadedDatasetManifest{}, err
	}
	var manifest DatasetManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return LoadedDatasetManifest{}, err
	}
	if err := validateDatasetManifest(manifest, registry); err != nil {
		return LoadedDatasetManifest{}, err
	}
	digest := sha256.Sum256(data)
	return LoadedDatasetManifest{Root: absRoot, Data: manifest, SHA256: hex.EncodeToString(digest[:])}, nil
}

func validateDatasetManifest(manifest DatasetManifest, registry *Registry) error {
	if manifest.Format != DatasetManifestFormat || manifest.ManifestVersion != DatasetManifestVersion {
		return fmt.Errorf("unsupported dataset manifest %q version %d", manifest.Format, manifest.ManifestVersion)
	}
	if manifest.DatasetFormatVersion != DatasetFormatVersion {
		return fmt.Errorf("manifest dataset format %d, want %d", manifest.DatasetFormatVersion, DatasetFormatVersion)
	}
	if manifest.TraceSchemaVersion != eng.TuningTraceSchemaVersion {
		return fmt.Errorf("manifest trace schema %d, want %d", manifest.TraceSchemaVersion, eng.TuningTraceSchemaVersion)
	}
	if manifest.RegistryVersion != registry.Version || manifest.RegistryFingerprint != registry.Fingerprint {
		return fmt.Errorf("manifest registry %s/%s does not match %s/%s", manifest.RegistryVersion, manifest.RegistryFingerprint, registry.Version, registry.Fingerprint)
	}
	if err := manifest.Split.Validate(); err != nil {
		return err
	}
	sourceRecords := make(map[string]uint64, len(manifest.Sources))
	for _, source := range manifest.Sources {
		if source.ID == "" {
			return errors.New("manifest contains an empty source ID")
		}
		if _, exists := sourceRecords[source.ID]; exists {
			return fmt.Errorf("manifest repeats source ID %q", source.ID)
		}
		sourceRecords[source.ID] = 0
	}
	var records uint64
	var splits DatasetSplitCounts
	var outcomes DatasetOutcomeCounts
	paths := make(map[string]struct{}, len(manifest.Shards))
	for _, shard := range manifest.Shards {
		if _, exists := paths[shard.Path]; exists {
			return fmt.Errorf("manifest repeats shard path %q", shard.Path)
		}
		paths[shard.Path] = struct{}{}
		if _, exists := sourceRecords[shard.SourceID]; !exists {
			return fmt.Errorf("shard %q references unknown source %q", shard.Path, shard.SourceID)
		}
		split, err := parseSplitName(shard.Split)
		if err != nil {
			return fmt.Errorf("shard %q: %w", shard.Path, err)
		}
		records += shard.Records
		sourceRecords[shard.SourceID] += shard.Records
		addSplitCount(&splits, split, shard.Records)
		outcomes.BlackWins += shard.Outcomes.BlackWins
		outcomes.Draws += shard.Outcomes.Draws
		outcomes.WhiteWins += shard.Outcomes.WhiteWins
	}
	if records != manifest.Statistics.Records || splits != manifest.Statistics.Splits || outcomes != manifest.Statistics.Outcomes {
		return errors.New("manifest shard totals do not match aggregate statistics")
	}
	for _, source := range manifest.Sources {
		if sourceRecords[source.ID] != source.Records {
			return fmt.Errorf("source %q shard records total %d, want %d", source.ID, sourceRecords[source.ID], source.Records)
		}
	}
	return nil
}

func addSplitCount(counts *DatasetSplitCounts, split DatasetSplit, amount uint64) {
	switch split {
	case SplitTraining:
		counts.Training += amount
	case SplitValidation:
		counts.Validation += amount
	case SplitTest:
		counts.Test += amount
	}
}

func parseSplitName(name string) (DatasetSplit, error) {
	switch name {
	case "train":
		return SplitTraining, nil
	case "validation":
		return SplitValidation, nil
	case "test":
		return SplitTest, nil
	default:
		return 0, fmt.Errorf("unknown dataset split %q", name)
	}
}

func safeShardPath(root, relative string) (string, error) {
	if relative == "" || filepath.IsAbs(relative) {
		return "", fmt.Errorf("unsafe shard path %q", relative)
	}
	clean := filepath.Clean(filepath.FromSlash(relative))
	if clean == ".." || len(clean) >= 3 && clean[:3] == ".."+string(filepath.Separator) {
		return "", fmt.Errorf("unsafe shard path %q", relative)
	}
	return filepath.Join(root, clean), nil
}

// LoadDatasetShard validates the shard header and record checksum. When
// loadRecords is false it streams the encoded bytes without allocating record
// objects; true decodes the complete shard into the current training structure.
func LoadDatasetShard(dataset LoadedDatasetManifest, shard ManifestShard, registry *Registry, loadRecords bool) (LoadedDatasetShard, error) {
	path, err := safeShardPath(dataset.Root, shard.Path)
	if err != nil {
		return LoadedDatasetShard{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return LoadedDatasetShard{}, err
	}
	if info.Size() != shard.Bytes {
		return LoadedDatasetShard{}, fmt.Errorf("shard %q size %d, want %d", shard.Path, info.Size(), shard.Bytes)
	}
	file, err := os.Open(path)
	if err != nil {
		return LoadedDatasetShard{}, err
	}
	defer file.Close()
	headerBytes := make([]byte, datasetHeaderSize)
	if _, err := io.ReadFull(file, headerBytes); err != nil {
		return LoadedDatasetShard{}, err
	}
	recordHash := sha256.New()
	decoder, err := NewDatasetDecoder(io.MultiReader(bytes.NewReader(headerBytes), io.TeeReader(file, recordHash)), registry)
	if err != nil {
		return LoadedDatasetShard{}, err
	}
	if err := validateShardHeader(shard, decoder.Header); err != nil {
		return LoadedDatasetShard{}, err
	}
	out := LoadedDatasetShard{Metadata: shard, Header: decoder.Header}
	if loadRecords {
		if shard.Records > uint64(^uint(0)>>1) {
			return LoadedDatasetShard{}, errors.New("shard record count overflows this platform")
		}
		out.Records = make([]CompiledTrainingRecord, 0, int(shard.Records))
		split, _ := parseSplitName(shard.Split)
		for uint64(len(out.Records)) < shard.Records {
			record, err := decoder.Next()
			if err != nil {
				return LoadedDatasetShard{}, fmt.Errorf("decode shard %q record %d: %w", shard.Path, len(out.Records), err)
			}
			if record.Split != split {
				return LoadedDatasetShard{}, fmt.Errorf("shard %q contains split %d, want %d", shard.Path, record.Split, split)
			}
			out.Records = append(out.Records, record)
		}
		if _, err := decoder.Next(); !errors.Is(err, io.EOF) {
			return LoadedDatasetShard{}, fmt.Errorf("shard %q final decoder error: %w", shard.Path, err)
		}
		trailing, err := io.Copy(io.Discard, decoder.r)
		if err != nil {
			return LoadedDatasetShard{}, err
		}
		if trailing != 0 {
			return LoadedDatasetShard{}, fmt.Errorf("shard %q has %d trailing bytes", shard.Path, trailing)
		}
	} else {
		if _, err := io.Copy(io.Discard, decoder.r); err != nil {
			return LoadedDatasetShard{}, err
		}
	}
	if got := hex.EncodeToString(recordHash.Sum(nil)); got != shard.RecordsSHA256 {
		return LoadedDatasetShard{}, fmt.Errorf("shard %q record checksum %s, want %s", shard.Path, got, shard.RecordsSHA256)
	}
	return out, nil
}

func validateShardHeader(shard ManifestShard, header DatasetHeader) error {
	if header.Records != shard.Records {
		return fmt.Errorf("shard %q header records %d, want %d", shard.Path, header.Records, shard.Records)
	}
	wantOutcomes := [3]uint64{shard.Outcomes.BlackWins, shard.Outcomes.Draws, shard.Outcomes.WhiteWins}
	if header.Outcomes != wantOutcomes {
		return fmt.Errorf("shard %q header outcomes %v, want %v", shard.Path, header.Outcomes, wantOutcomes)
	}
	split, err := parseSplitName(shard.Split)
	if err != nil {
		return err
	}
	for index, count := range header.Splits {
		want := uint64(0)
		if DatasetSplit(index) == split {
			want = shard.Records
		}
		if count != want {
			return fmt.Errorf("shard %q header split counts %v do not match %s", shard.Path, header.Splits, shard.Split)
		}
	}
	return nil
}

// DeterministicRecordPermutation returns the stable within-shard permutation
// used by the future trainer. It intentionally uses a fixed SplitMix64
// implementation rather than math/rand, whose stream is not a file-format
// compatibility contract.
func DeterministicRecordPermutation(recordCount uint64, globalSeed uint64, manifestSHA256 string, epoch uint64, shardPath string) ([]uint32, error) {
	if recordCount > math.MaxUint32 {
		return nil, fmt.Errorf("record count %d exceeds uint32 permutation indexes", recordCount)
	}
	manifestDigest, err := hex.DecodeString(manifestSHA256)
	if err != nil || len(manifestDigest) != sha256.Size {
		return nil, fmt.Errorf("manifest SHA-256 %q is invalid", manifestSHA256)
	}
	hasher := sha256.New()
	_, _ = hasher.Write([]byte("goose-tuner-record-permutation-v1\x00"))
	var number [8]byte
	binary.LittleEndian.PutUint64(number[:], globalSeed)
	_, _ = hasher.Write(number[:])
	_, _ = hasher.Write(manifestDigest)
	binary.LittleEndian.PutUint64(number[:], epoch)
	_, _ = hasher.Write(number[:])
	_, _ = hasher.Write([]byte(filepath.ToSlash(shardPath)))
	seedDigest := hasher.Sum(nil)
	rng := splitMix64{state: binary.LittleEndian.Uint64(seedDigest[:8])}
	permutation := make([]uint32, int(recordCount))
	for index := range permutation {
		permutation[index] = uint32(index)
	}
	for index := len(permutation) - 1; index > 0; index-- {
		other := int(rng.next() % uint64(index+1))
		permutation[index], permutation[other] = permutation[other], permutation[index]
	}
	return permutation, nil
}

type splitMix64 struct{ state uint64 }

func (r *splitMix64) next() uint64 {
	r.state += 0x9e3779b97f4a7c15
	value := r.state
	value = (value ^ (value >> 30)) * 0xbf58476d1ce4e5b9
	value = (value ^ (value >> 27)) * 0x94d049bb133111eb
	return value ^ (value >> 31)
}
