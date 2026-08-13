package tuner

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type FeatureFrequency struct {
	Index            int         `json:"index"`
	ID               ParameterID `json:"id"`
	Coordinate       []int       `json:"coordinate,omitempty"`
	CoordinateLabels []string    `json:"coordinateLabels,omitempty"`
	Records          uint64      `json:"records"`
}

type DatasetSplitCounts struct {
	Training   uint64 `json:"training"`
	Validation uint64 `json:"validation"`
	Test       uint64 `json:"test"`
}

func (c *DatasetSplitCounts) add(split DatasetSplit) {
	switch split {
	case SplitTraining:
		c.Training++
	case SplitValidation:
		c.Validation++
	case SplitTest:
		c.Test++
	}
}

type DatasetOutcomeCounts struct {
	BlackWins uint64 `json:"blackWins"`
	Draws     uint64 `json:"draws"`
	WhiteWins uint64 `json:"whiteWins"`
}

func (c *DatasetOutcomeCounts) add(outcome Outcome) {
	switch outcome {
	case OutcomeBlackWin:
		c.BlackWins++
	case OutcomeDraw:
		c.Draws++
	case OutcomeWhiteWin:
		c.WhiteWins++
	}
}

type DatasetStats struct {
	RegistryFingerprint         string               `json:"registryFingerprint"`
	SplitConfig                 SplitConfig          `json:"splitConfig"`
	Deduplicated                bool                 `json:"deduplicated"`
	ExactVerified               bool                 `json:"exactVerified"`
	SourceFiles                 int                  `json:"sourceFiles"`
	SourceLines                 uint64               `json:"sourceLines"`
	BlankLines                  uint64               `json:"blankLines"`
	InvalidLines                uint64               `json:"invalidLines"`
	SourceScoreAnnotations      uint64               `json:"sourceScoreAnnotations"`
	Duplicates                  uint64               `json:"duplicatePositionOutcomes"`
	ConflictingOutcomeRecords   uint64               `json:"conflictingOutcomeRecords"`
	ConflictingOutcomePositions uint64               `json:"conflictingOutcomePositions"`
	UniquePositions             uint64               `json:"uniquePositions"`
	Records                     uint64               `json:"records"`
	Splits                      DatasetSplitCounts   `json:"splits"`
	Outcomes                    DatasetOutcomeCounts `json:"outcomes"`
	FeatureFrequency            []FeatureFrequency   `json:"featureFrequency"`
	Sources                     []DatasetSourceStats `json:"sources"`
}

type DatasetSourceStats struct {
	ID                     string               `json:"id"`
	Name                   string               `json:"name"`
	InputBytes             int64                `json:"inputBytes"`
	InputSHA256            string               `json:"inputSha256,omitempty"`
	Complete               bool                 `json:"complete"`
	SourceLines            uint64               `json:"sourceLines"`
	BlankLines             uint64               `json:"blankLines"`
	InvalidLines           uint64               `json:"invalidLines"`
	SourceScoreAnnotations uint64               `json:"sourceScoreAnnotations"`
	Duplicates             uint64               `json:"duplicatePositionOutcomes"`
	ConflictingOutcomes    uint64               `json:"conflictingOutcomeRecords"`
	Records                uint64               `json:"records"`
	Splits                 DatasetSplitCounts   `json:"splits"`
	Outcomes               DatasetOutcomeCounts `json:"outcomes"`
}

type CompileSource struct {
	ID   string
	Name string
	Path string
}

type ConflictPolicy string

const (
	ConflictKeep  ConflictPolicy = "keep"
	ConflictDrop  ConflictPolicy = "drop"
	ConflictError ConflictPolicy = "error"
)

func (p ConflictPolicy) Validate() error {
	switch p {
	case ConflictKeep, ConflictDrop, ConflictError:
		return nil
	default:
		return fmt.Errorf("unknown conflicting-outcome policy %q", p)
	}
}

type CompileProgress struct {
	File        string
	FileLine    uint64
	SourceLines uint64
	Records     uint64
}

type CompileConfig struct {
	Split             SplitConfig
	Deduplicate       bool
	SkipInvalid       bool
	VerifyExact       bool
	Conflicts         ConflictPolicy
	Overwrite         bool
	MaxRecords        uint64
	ExpectedPositions uint64
	ProgressEvery     uint64
	ReportProgress    func(CompileProgress)
}

func DefaultCompileConfig() CompileConfig {
	return CompileConfig{
		Split:         DefaultSplitConfig(),
		Deduplicate:   true,
		VerifyExact:   true,
		Conflicts:     ConflictKeep,
		ProgressEvery: 100000,
	}
}

// CompileSourcesFromPaths assigns stable source IDs from filenames and sorts
// sources deterministically. Source-ID collisions are rejected rather than
// silently merging provenance.
func CompileSourcesFromPaths(inputs []string) ([]CompileSource, error) {
	if len(inputs) == 0 {
		return nil, errors.New("no input book files")
	}
	paths := append([]string(nil), inputs...)
	for i := range paths {
		paths[i] = filepath.Clean(paths[i])
	}
	sort.Strings(paths)
	sources := make([]CompileSource, len(paths))
	ids := make(map[string]string, len(paths))
	for i, path := range paths {
		name := filepath.Base(path)
		id := sourceID(name)
		if id == "" {
			return nil, fmt.Errorf("cannot derive a source ID from %q", name)
		}
		if previous, exists := ids[id]; exists {
			return nil, fmt.Errorf("source ID %q is shared by %q and %q", id, previous, path)
		}
		ids[id] = path
		sources[i] = CompileSource{ID: id, Name: name, Path: path}
	}
	return sources, nil
}

func sourceID(name string) string {
	name = strings.TrimSuffix(name, filepath.Ext(name))
	var out strings.Builder
	dash := false
	for _, character := range strings.ToLower(name) {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' {
			out.WriteRune(character)
			dash = false
		} else if !dash && out.Len() != 0 {
			out.WriteByte('-')
			dash = true
		}
	}
	return strings.Trim(out.String(), "-")
}

type compiledRecordSink func(CompileSource, CompiledTrainingRecord) error

// compileBookRecords owns parsing, global duplicate handling, feature
// extraction and exact verification. Storage backends only receive accepted,
// registry-bound records.
func compileBookRecords(
	ctx context.Context,
	sources []CompileSource,
	registry *Registry,
	binding *TraceBinding,
	model *ForwardModel,
	config CompileConfig,
	sink compiledRecordSink,
) (stats DatasetStats, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if len(sources) == 0 {
		return stats, errors.New("no input book files")
	}
	if registry == nil || binding == nil || sink == nil {
		return stats, errors.New("dataset compilation requires a registry, trace binding, and record sink")
	}
	if err := config.Split.Validate(); err != nil {
		return stats, err
	}
	if err := config.Conflicts.Validate(); err != nil {
		return stats, err
	}
	if config.VerifyExact && model == nil {
		return stats, errors.New("exact verification requires a forward model")
	}
	stats.RegistryFingerprint = registry.Fingerprint
	stats.SplitConfig = config.Split
	stats.Deduplicated = config.Deduplicate
	stats.ExactVerified = config.VerifyExact
	stats.SourceFiles = len(sources)
	stats.Sources = make([]DatasetSourceStats, len(sources))
	for i, source := range sources {
		info, statErr := os.Stat(source.Path)
		if statErr != nil {
			return stats, statErr
		}
		stats.Sources[i] = DatasetSourceStats{ID: source.ID, Name: source.Name, InputBytes: info.Size()}
	}
	coverage := NewCoverageAccumulator(binding, len(registry.Elements))

	var exactParameters []int
	if config.VerifyExact {
		exactParameters, err = InitialExactParameters(registry)
		if err != nil {
			return stats, err
		}
	}
	var seen map[PositionKey]uint8
	if config.Deduplicate {
		if config.ExpectedPositions > uint64(^uint(0)>>1) {
			return stats, fmt.Errorf("expected position count %d overflows this platform", config.ExpectedPositions)
		}
		seen = make(map[PositionKey]uint8, int(config.ExpectedPositions))
	}
	stop := false
	for sourceIndex, source := range sources {
		if stop {
			break
		}
		sourceStats := &stats.Sources[sourceIndex]
		file, openErr := os.Open(source.Path)
		if openErr != nil {
			return stats, openErr
		}
		hasher := sha256.New()
		scanner := bufio.NewScanner(io.TeeReader(file, hasher))
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		var fileLine uint64
		for scanner.Scan() {
			fileLine++
			stats.SourceLines++
			sourceStats.SourceLines++
			if contextErr := ctx.Err(); contextErr != nil {
				_ = file.Close()
				return stats, contextErr
			}
			line := scanner.Text()
			if strings.TrimSpace(line) == "" {
				stats.BlankLines++
				sourceStats.BlankLines++
				continue
			}
			position, parseErr := ParseBookLine(line)
			if parseErr != nil {
				stats.InvalidLines++
				sourceStats.InvalidLines++
				if config.SkipInvalid {
					continue
				}
				_ = file.Close()
				return stats, fmt.Errorf("%s:%d: %w", source.Path, fileLine, parseErr)
			}
			if position.SourceScore != nil {
				stats.SourceScoreAnnotations++
				sourceStats.SourceScoreAnnotations++
			}
			key := KeyForPosition(position.IdentityFEN)
			if config.Deduplicate {
				bit := uint8(1) << uint8(position.Outcome)
				mask := seen[key]
				if mask == 0 {
					stats.UniquePositions++
					seen[key] = bit
				} else if mask&bit != 0 {
					stats.Duplicates++
					sourceStats.Duplicates++
					continue
				} else {
					stats.ConflictingOutcomeRecords++
					sourceStats.ConflictingOutcomes++
					if mask&(mask-1) == 0 {
						stats.ConflictingOutcomePositions++
					}
					seen[key] = mask | bit
					switch config.Conflicts {
					case ConflictDrop:
						continue
					case ConflictError:
						_ = file.Close()
						return stats, fmt.Errorf("%s:%d: position has more than one outcome label", source.Path, fileLine)
					}
				}
			}
			record, compileErr := CompilePosition(position, binding)
			if compileErr != nil {
				_ = file.Close()
				return stats, fmt.Errorf("%s:%d: %w", source.Path, fileLine, compileErr)
			}
			record.PositionKey = key
			record.Split, err = AssignSplit(position.IdentityFEN, config.Split)
			if err != nil {
				_ = file.Close()
				return stats, err
			}
			if config.VerifyExact {
				exact, forwardErr := model.EngineExact(record.Trace, exactParameters)
				if forwardErr != nil {
					_ = file.Close()
					return stats, fmt.Errorf("%s:%d: exact forward: %w", source.Path, fileLine, forwardErr)
				}
				want := record.Trace.Reference
				if exact.Buckets != want.Buckets || exact.WhitePerspective != want.WhitePerspective || exact.SideToMove != want.SideToMove {
					_ = file.Close()
					return stats, fmt.Errorf("%s:%d: exact forward mismatch: got %+v, want %+v", source.Path, fileLine, exact, want)
				}
			}
			coverage.Add(record.Trace)
			if sinkErr := sink(source, record); sinkErr != nil {
				_ = file.Close()
				return stats, fmt.Errorf("%s:%d: %w", source.Path, fileLine, sinkErr)
			}
			stats.Records++
			stats.Splits.add(record.Split)
			stats.Outcomes.add(record.Outcome)
			sourceStats.Records++
			sourceStats.Splits.add(record.Split)
			sourceStats.Outcomes.add(record.Outcome)
			if config.ReportProgress != nil && config.ProgressEvery != 0 && stats.SourceLines%config.ProgressEvery == 0 {
				config.ReportProgress(CompileProgress{File: source.Path, FileLine: fileLine, SourceLines: stats.SourceLines, Records: stats.Records})
			}
			if config.MaxRecords != 0 && stats.Records >= config.MaxRecords {
				stop = true
				break
			}
		}
		scanErr := scanner.Err()
		closeErr := file.Close()
		if scanErr != nil {
			return stats, fmt.Errorf("scan %s: %w", source.Path, scanErr)
		}
		if closeErr != nil {
			return stats, closeErr
		}
		if !stop {
			sourceStats.Complete = true
			sourceStats.InputSHA256 = hex.EncodeToString(hasher.Sum(nil))
		}
	}
	stats.FeatureFrequency = make([]FeatureFrequency, len(registry.Elements))
	for i, element := range registry.Elements {
		stats.FeatureFrequency[i] = FeatureFrequency{
			Index: i, ID: element.ID,
			Coordinate:       append([]int(nil), element.Coordinate...),
			CoordinateLabels: append([]string(nil), element.CoordinateLabels...),
			Records:          coverage.Counts()[i],
		}
	}
	return stats, nil
}

// CompileBookFiles keeps the original single-file backend for small fixtures
// and compatibility. Large conversions should use CompileBookFilesSharded.
func CompileBookFiles(
	ctx context.Context,
	inputs []string,
	output string,
	registry *Registry,
	binding *TraceBinding,
	model *ForwardModel,
	config CompileConfig,
) (stats DatasetStats, err error) {
	if output == "" {
		return stats, errors.New("no output dataset path")
	}
	sources, err := CompileSourcesFromPaths(inputs)
	if err != nil {
		return stats, err
	}
	if !config.Overwrite {
		if _, statErr := os.Stat(output); statErr == nil {
			return stats, fmt.Errorf("output %q already exists", output)
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return stats, statErr
		}
	}
	temp, err := os.CreateTemp(filepath.Dir(output), "."+filepath.Base(output)+".tmp-*")
	if err != nil {
		return stats, err
	}
	tempName := temp.Name()
	committed := false
	defer func() {
		_ = temp.Close()
		if !committed {
			_ = os.Remove(tempName)
		}
	}()
	encoder, err := NewDatasetEncoderWithMetadata(temp, registry, DatasetMetadata{
		Split: config.Split, Deduplicated: config.Deduplicate, ExactVerified: config.VerifyExact,
	})
	if err != nil {
		return stats, err
	}
	stats, err = compileBookRecords(ctx, sources, registry, binding, model, config, func(_ CompileSource, record CompiledTrainingRecord) error {
		return encoder.Encode(record)
	})
	if err != nil {
		return stats, err
	}
	if err := encoder.Close(); err != nil {
		return stats, err
	}
	if err := temp.Sync(); err != nil {
		return stats, err
	}
	if err := temp.Close(); err != nil {
		return stats, err
	}
	if config.Overwrite {
		if removeErr := os.Remove(output); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return stats, removeErr
		}
	}
	if err := os.Rename(tempName, output); err != nil {
		return stats, err
	}
	committed = true
	return stats, nil
}
