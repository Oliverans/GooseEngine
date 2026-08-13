package tuner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	eng "chess-engine/engine"
)

const (
	DatasetManifestFormat  = "goose-tuner-sharded"
	DatasetManifestVersion = 1
)

type ShardedCompileConfig struct {
	Compile            CompileConfig
	MaxRecordsPerShard uint64
	ShuffleSeed        uint64
}

func DefaultShardedCompileConfig() ShardedCompileConfig {
	return ShardedCompileConfig{
		Compile:            DefaultCompileConfig(),
		MaxRecordsPerShard: 500000,
		ShuffleSeed:        0x73687566666c6576,
	}
}

type DatasetManifest struct {
	Format               string                `json:"format"`
	ManifestVersion      int                   `json:"manifestVersion"`
	Complete             bool                  `json:"complete"`
	DatasetFormatVersion uint16                `json:"datasetFormatVersion"`
	TraceSchemaVersion   int                   `json:"traceSchemaVersion"`
	RegistryVersion      string                `json:"registryVersion"`
	RegistryFingerprint  string                `json:"registryFingerprint"`
	Split                SplitConfig           `json:"split"`
	Deduplication        ManifestDeduplication `json:"deduplication"`
	Sharding             ManifestSharding      `json:"sharding"`
	Shuffle              ManifestShuffle       `json:"shuffle"`
	StatisticsFile       string                `json:"statisticsFile"`
	Statistics           ManifestStatistics    `json:"statistics"`
	Sources              []ManifestSource      `json:"sources"`
	Shards               []ManifestShard       `json:"shards"`
}

type ManifestDeduplication struct {
	Enabled             bool           `json:"enabled"`
	Identity            string         `json:"identity"`
	ConflictingOutcomes ConflictPolicy `json:"conflictingOutcomes"`
}

type ManifestSharding struct {
	Layout             string `json:"layout"`
	MaxRecordsPerShard uint64 `json:"maxRecordsPerShard"`
}

// ManifestShuffle defines the future trainer's reproducibility contract. The
// trainer will derive each epoch seed from this seed, the manifest SHA-256 and
// the epoch number, then checkpoint the stated cursors and optimizer state.
type ManifestShuffle struct {
	Algorithm        string   `json:"algorithm"`
	Seed             uint64   `json:"seed"`
	CheckpointFields []string `json:"checkpointFields"`
}

type ManifestStatistics struct {
	SourceLines                 uint64               `json:"sourceLines"`
	BlankLines                  uint64               `json:"blankLines"`
	InvalidLines                uint64               `json:"invalidLines"`
	SourceScoreAnnotations      uint64               `json:"sourceScoreAnnotations"`
	UniquePositions             uint64               `json:"uniquePositions"`
	DuplicatePositionOutcomes   uint64               `json:"duplicatePositionOutcomes"`
	ConflictingOutcomeRecords   uint64               `json:"conflictingOutcomeRecords"`
	ConflictingOutcomePositions uint64               `json:"conflictingOutcomePositions"`
	Records                     uint64               `json:"records"`
	Splits                      DatasetSplitCounts   `json:"splits"`
	Outcomes                    DatasetOutcomeCounts `json:"outcomes"`
}

type ManifestSource struct {
	ID                     string               `json:"id"`
	Name                   string               `json:"name"`
	InputBytes             int64                `json:"inputBytes"`
	InputSHA256            string               `json:"inputSha256,omitempty"`
	Complete               bool                 `json:"complete"`
	SourceLines            uint64               `json:"sourceLines"`
	SourceScoreAnnotations uint64               `json:"sourceScoreAnnotations"`
	Records                uint64               `json:"records"`
	Splits                 DatasetSplitCounts   `json:"splits"`
	Outcomes               DatasetOutcomeCounts `json:"outcomes"`
}

type ManifestShard struct {
	Path          string               `json:"path"`
	SourceID      string               `json:"sourceId"`
	Split         string               `json:"split"`
	Part          int                  `json:"part"`
	Records       uint64               `json:"records"`
	Outcomes      DatasetOutcomeCounts `json:"outcomes"`
	Bytes         int64                `json:"bytes"`
	RecordsSHA256 string               `json:"recordsSha256"`
}

type ShardedCompileResult struct {
	Manifest       DatasetManifest
	ManifestSHA256 string
	Stats          DatasetStats
}

type shardKey struct {
	sourceID string
	split    DatasetSplit
}

type shardState struct {
	part    int
	current *openShard
}

type openShard struct {
	key      shardKey
	part     int
	relative string
	file     *os.File
	encoder  *DatasetEncoder
}

type shardManager struct {
	root       string
	registry   *Registry
	metadata   DatasetMetadata
	maxRecords uint64
	states     map[shardKey]*shardState
	shards     []ManifestShard
}

func newShardManager(root string, registry *Registry, metadata DatasetMetadata, maxRecords uint64) (*shardManager, error) {
	if maxRecords == 0 {
		return nil, errors.New("max records per shard must be positive")
	}
	return &shardManager{
		root: root, registry: registry, metadata: metadata, maxRecords: maxRecords,
		states: make(map[shardKey]*shardState),
	}, nil
}

func (m *shardManager) Write(source CompileSource, record CompiledTrainingRecord) error {
	key := shardKey{sourceID: source.ID, split: record.Split}
	state := m.states[key]
	if state == nil {
		state = &shardState{}
		m.states[key] = state
	}
	if state.current == nil || state.current.encoder.Header().Records >= m.maxRecords {
		if state.current != nil {
			if err := m.closeShard(state.current); err != nil {
				return err
			}
		}
		opened, err := m.openShard(key, state.part)
		if err != nil {
			return err
		}
		state.current = opened
		state.part++
	}
	return state.current.encoder.Encode(record)
}

func (m *shardManager) openShard(key shardKey, part int) (*openShard, error) {
	relative := filepath.Join(splitName(key.split), key.sourceID, fmt.Sprintf("part-%05d.tune", part))
	absolute := filepath.Join(m.root, relative)
	if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(absolute, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return nil, err
	}
	encoder, err := NewDatasetEncoderWithMetadata(file, m.registry, m.metadata)
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	return &openShard{key: key, part: part, relative: relative, file: file, encoder: encoder}, nil
}

func (m *shardManager) closeShard(shard *openShard) error {
	if err := shard.encoder.Close(); err != nil {
		return err
	}
	if err := shard.file.Sync(); err != nil {
		return err
	}
	if err := shard.file.Close(); err != nil {
		return err
	}
	info, err := os.Stat(filepath.Join(m.root, shard.relative))
	if err != nil {
		return err
	}
	header := shard.encoder.Header()
	m.shards = append(m.shards, ManifestShard{
		Path: filepath.ToSlash(shard.relative), SourceID: shard.key.sourceID,
		Split: splitName(shard.key.split), Part: shard.part, Records: header.Records,
		Outcomes: DatasetOutcomeCounts{
			BlackWins: header.Outcomes[OutcomeBlackWin], Draws: header.Outcomes[OutcomeDraw], WhiteWins: header.Outcomes[OutcomeWhiteWin],
		},
		Bytes: info.Size(), RecordsSHA256: shard.encoder.RecordsSHA256(),
	})
	return nil
}

func (m *shardManager) Close() error {
	keys := make([]shardKey, 0, len(m.states))
	for key := range m.states {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].sourceID != keys[j].sourceID {
			return keys[i].sourceID < keys[j].sourceID
		}
		return keys[i].split < keys[j].split
	})
	for _, key := range keys {
		state := m.states[key]
		if state.current != nil {
			if err := m.closeShard(state.current); err != nil {
				return err
			}
			state.current = nil
		}
	}
	sort.Slice(m.shards, func(i, j int) bool { return m.shards[i].Path < m.shards[j].Path })
	return nil
}

func (m *shardManager) Abort() {
	if m == nil {
		return
	}
	for _, state := range m.states {
		if state.current != nil && state.current.file != nil {
			_ = state.current.file.Close()
			state.current = nil
		}
	}
}

func splitName(split DatasetSplit) string {
	switch split {
	case SplitTraining:
		return "train"
	case SplitValidation:
		return "validation"
	case SplitTest:
		return "test"
	default:
		return "unknown"
	}
}

// CompileBookFilesSharded creates the complete dataset in a sibling temporary
// directory and publishes it only after every shard, checksum, statistic and
// manifest has been finalized.
func CompileBookFilesSharded(
	ctx context.Context,
	inputs []string,
	outputDirectory string,
	registry *Registry,
	binding *TraceBinding,
	model *ForwardModel,
	config ShardedCompileConfig,
) (result ShardedCompileResult, err error) {
	if outputDirectory == "" {
		return result, errors.New("no output dataset directory")
	}
	if registry == nil {
		return result, errors.New("sharded compilation requires a registry")
	}
	if config.MaxRecordsPerShard == 0 {
		return result, errors.New("max records per shard must be positive")
	}
	sources, err := CompileSourcesFromPaths(inputs)
	if err != nil {
		return result, err
	}
	if err := validateDatasetDirectoryTarget(outputDirectory, config.Compile.Overwrite); err != nil {
		return result, err
	}
	parent := filepath.Dir(filepath.Clean(outputDirectory))
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return result, err
	}
	tempRoot, err := os.MkdirTemp(parent, "."+filepath.Base(outputDirectory)+".tmp-*")
	if err != nil {
		return result, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(tempRoot)
		}
	}()
	metadata := DatasetMetadata{
		Split: config.Compile.Split, Deduplicated: config.Compile.Deduplicate, ExactVerified: config.Compile.VerifyExact,
	}
	manager, err := newShardManager(tempRoot, registry, metadata, config.MaxRecordsPerShard)
	if err != nil {
		return result, err
	}
	defer manager.Abort()
	stats, err := compileBookRecords(ctx, sources, registry, binding, model, config.Compile, manager.Write)
	if err != nil {
		return result, err
	}
	if err := manager.Close(); err != nil {
		return result, err
	}
	manifest := buildDatasetManifest(registry, config, stats, manager.shards)
	statisticsData, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return result, err
	}
	statisticsData = append(statisticsData, '\n')
	if err := os.WriteFile(filepath.Join(tempRoot, manifest.StatisticsFile), statisticsData, 0o644); err != nil {
		return result, err
	}
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return result, err
	}
	manifestData = append(manifestData, '\n')
	if err := os.WriteFile(filepath.Join(tempRoot, "manifest.json"), manifestData, 0o644); err != nil {
		return result, err
	}
	manifestHash := sha256.Sum256(manifestData)
	if err := commitDatasetDirectory(tempRoot, outputDirectory, config.Compile.Overwrite); err != nil {
		return result, err
	}
	committed = true
	return ShardedCompileResult{
		Manifest: manifest, ManifestSHA256: hex.EncodeToString(manifestHash[:]), Stats: stats,
	}, nil
}

func buildDatasetManifest(registry *Registry, config ShardedCompileConfig, stats DatasetStats, shards []ManifestShard) DatasetManifest {
	sources := make([]ManifestSource, len(stats.Sources))
	complete := config.Compile.MaxRecords == 0
	for i, source := range stats.Sources {
		sources[i] = ManifestSource{
			ID: source.ID, Name: source.Name, InputBytes: source.InputBytes, InputSHA256: source.InputSHA256,
			Complete: source.Complete, SourceLines: source.SourceLines,
			SourceScoreAnnotations: source.SourceScoreAnnotations, Records: source.Records,
			Splits: source.Splits, Outcomes: source.Outcomes,
		}
		complete = complete && source.Complete
	}
	return DatasetManifest{
		Format: DatasetManifestFormat, ManifestVersion: DatasetManifestVersion, Complete: complete,
		DatasetFormatVersion: DatasetFormatVersion, TraceSchemaVersion: eng.TuningTraceSchemaVersion,
		RegistryVersion: registry.Version, RegistryFingerprint: registry.Fingerprint,
		Split: config.Compile.Split,
		Deduplication: ManifestDeduplication{
			Enabled: config.Compile.Deduplicate, Identity: "canonical-fen-first-four-fields-sha256-128",
			ConflictingOutcomes: config.Compile.Conflicts,
		},
		Sharding: ManifestSharding{Layout: "split/source/part-sequential-v1", MaxRecordsPerShard: config.MaxRecordsPerShard},
		Shuffle: ManifestShuffle{
			Algorithm:        "sha256(global-seed,manifest-sha256,epoch)-shard-and-record-permutation-v1",
			Seed:             config.ShuffleSeed,
			CheckpointFields: []string{"manifestSha256", "epoch", "shardCursor", "batchCursor", "optimizerStep", "adamState", "learningRateState"},
		},
		StatisticsFile: "statistics.json",
		Statistics: ManifestStatistics{
			SourceLines: stats.SourceLines, BlankLines: stats.BlankLines, InvalidLines: stats.InvalidLines,
			SourceScoreAnnotations: stats.SourceScoreAnnotations,
			UniquePositions:        stats.UniquePositions, DuplicatePositionOutcomes: stats.Duplicates,
			ConflictingOutcomeRecords: stats.ConflictingOutcomeRecords, ConflictingOutcomePositions: stats.ConflictingOutcomePositions,
			Records: stats.Records, Splits: stats.Splits, Outcomes: stats.Outcomes,
		},
		Sources: sources, Shards: append([]ManifestShard(nil), shards...),
	}
}

func validateDatasetDirectoryTarget(path string, overwrite bool) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if filepath.Dir(abs) == abs {
		return fmt.Errorf("refusing to use filesystem root %q as a dataset directory", abs)
	}
	info, err := os.Stat(abs)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !overwrite {
		return fmt.Errorf("output dataset directory %q already exists", path)
	}
	if !info.IsDir() {
		return fmt.Errorf("refusing to replace non-directory output %q", path)
	}
	manifest, err := ReadDatasetManifest(filepath.Join(abs, "manifest.json"))
	if err != nil || manifest.Format != DatasetManifestFormat {
		return fmt.Errorf("refusing to replace %q because it is not a recognized tuner dataset", path)
	}
	return nil
}

func commitDatasetDirectory(tempRoot, outputDirectory string, overwrite bool) error {
	if _, err := os.Stat(outputDirectory); errors.Is(err, os.ErrNotExist) {
		return os.Rename(tempRoot, outputDirectory)
	} else if err != nil {
		return err
	}
	if !overwrite {
		return fmt.Errorf("output dataset directory %q already exists", outputDirectory)
	}
	parent := filepath.Dir(filepath.Clean(outputDirectory))
	backup, err := os.MkdirTemp(parent, "."+filepath.Base(outputDirectory)+".previous-*")
	if err != nil {
		return err
	}
	if err := os.Remove(backup); err != nil {
		return err
	}
	if err := os.Rename(outputDirectory, backup); err != nil {
		return err
	}
	if err := os.Rename(tempRoot, outputDirectory); err != nil {
		_ = os.Rename(backup, outputDirectory)
		return err
	}
	return os.RemoveAll(backup)
}

func ReadDatasetManifest(path string) (DatasetManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return DatasetManifest{}, err
	}
	var manifest DatasetManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return DatasetManifest{}, err
	}
	if manifest.Format != DatasetManifestFormat || manifest.ManifestVersion != DatasetManifestVersion {
		return DatasetManifest{}, fmt.Errorf("unsupported dataset manifest %q version %d", manifest.Format, manifest.ManifestVersion)
	}
	return manifest, nil
}
