package tuner

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestCompileBookFilesShardedIsDeterministicAndTracksConflicts(t *testing.T) {
	registry, binding, model := newDatasetTestSystem(t)
	root := t.TempDir()
	firstBook := filepath.Join(root, "alpha.book")
	secondBook := filepath.Join(root, "beta.book")
	kingDraw := "8/8/8/8/8/8/8/K6k w - - 0 1 [0.5]\n"
	complexWhite := "r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1 [1.0]\n"
	complexBlack := "r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 17 42 [0.0]\n"
	pawnWhite := "8/8/4P3/8/8/8/8/K5k1 w - - 0 1 [1.0]\n"
	if err := os.WriteFile(firstBook, []byte(kingDraw+complexWhite), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secondBook, []byte(kingDraw+complexBlack+pawnWhite), 0o644); err != nil {
		t.Fatal(err)
	}
	config := DefaultShardedCompileConfig()
	config.Compile.Split = SplitConfig{Seed: 17}
	config.Compile.ProgressEvery = 0
	config.MaxRecordsPerShard = 1
	firstOutput := filepath.Join(root, "compiled-first")
	first, err := CompileBookFilesSharded(context.Background(), []string{secondBook, firstBook}, firstOutput, registry, binding, model, config)
	if err != nil {
		t.Fatal(err)
	}
	if first.Stats.SourceLines != 5 || first.Stats.UniquePositions != 3 || first.Stats.Records != 4 {
		t.Fatalf("unexpected aggregate stats: %+v", first.Stats)
	}
	if first.Stats.Duplicates != 1 || first.Stats.ConflictingOutcomeRecords != 1 || first.Stats.ConflictingOutcomePositions != 1 {
		t.Fatalf("unexpected duplicate/conflict stats: %+v", first.Stats)
	}
	if !first.Manifest.Complete || len(first.Manifest.Shards) != 4 || first.Manifest.Shuffle.Seed != config.ShuffleSeed {
		t.Fatalf("unexpected manifest: %+v", first.Manifest)
	}
	if first.Manifest.RegistryFingerprint != registry.Fingerprint || first.Manifest.RegistryVersion != registry.Version {
		t.Fatal("manifest does not identify the active registry")
	}
	for _, shard := range first.Manifest.Shards {
		if shard.Records != 1 || shard.Split != "train" {
			t.Fatalf("unexpected shard metadata: %+v", shard)
		}
		path := filepath.Join(firstOutput, filepath.FromSlash(shard.Path))
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(data[datasetHeaderSize:])
		if got := hex.EncodeToString(digest[:]); got != shard.RecordsSHA256 {
			t.Fatalf("shard checksum = %s, want %s", got, shard.RecordsSHA256)
		}
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		decoder, err := NewDatasetDecoder(file, registry)
		if err != nil {
			_ = file.Close()
			t.Fatal(err)
		}
		record, err := decoder.Next()
		_ = file.Close()
		if err != nil || record.Split != SplitTraining {
			t.Fatalf("decode shard %s: record=%+v error=%v", shard.Path, record, err)
		}
	}
	manifestData, err := os.ReadFile(filepath.Join(firstOutput, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	manifestDigest := sha256.Sum256(manifestData)
	if got := hex.EncodeToString(manifestDigest[:]); got != first.ManifestSHA256 {
		t.Fatalf("manifest checksum = %s, want %s", got, first.ManifestSHA256)
	}
	if _, err := os.Stat(filepath.Join(firstOutput, "statistics.json")); err != nil {
		t.Fatal(err)
	}

	secondOutput := filepath.Join(root, "compiled-second")
	second, err := CompileBookFilesSharded(context.Background(), []string{firstBook, secondBook}, secondOutput, registry, binding, model, config)
	if err != nil {
		t.Fatal(err)
	}
	secondManifest, err := os.ReadFile(filepath.Join(secondOutput, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if first.ManifestSHA256 != second.ManifestSHA256 || !bytes.Equal(manifestData, secondManifest) {
		t.Fatal("identical sharded conversions produced different manifests")
	}
	loadedManifest, err := LoadDatasetManifest(firstOutput, registry)
	if err != nil {
		t.Fatal(err)
	}
	if loadedManifest.SHA256 != first.ManifestSHA256 {
		t.Fatalf("loaded manifest checksum = %s, want %s", loadedManifest.SHA256, first.ManifestSHA256)
	}
	loadedStatistics, err := LoadDatasetStatistics(loadedManifest, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(loadedStatistics.Coverage) != len(registry.Elements) || loadedStatistics.Data.Records != first.Manifest.Statistics.Records {
		t.Fatalf("unexpected loaded statistics: coverage=%d records=%d", len(loadedStatistics.Coverage), loadedStatistics.Data.Records)
	}
	shardOrderA, err := DeterministicEpochShards(loadedManifest, SplitTraining, 2)
	if err != nil {
		t.Fatal(err)
	}
	shardOrderB, err := DeterministicEpochShards(loadedManifest, SplitTraining, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.EqualFunc(shardOrderA, shardOrderB, func(a, b ManifestShard) bool { return a.Path == b.Path }) {
		t.Fatal("deterministic epoch shard orders differ")
	}
	loadedShard, err := LoadDatasetShard(loadedManifest, first.Manifest.Shards[0], registry, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(loadedShard.Records) != int(first.Manifest.Shards[0].Records) {
		t.Fatalf("loaded %d records, want %d", len(loadedShard.Records), first.Manifest.Shards[0].Records)
	}
	exactParameters, err := InitialExactParameters(registry)
	if err != nil {
		t.Fatal(err)
	}
	continuousParameters := registry.InitialValues()
	for _, shardMetadata := range first.Manifest.Shards {
		objectShard, err := LoadDatasetShard(loadedManifest, shardMetadata, registry, true)
		if err != nil {
			t.Fatal(err)
		}
		packedShard, err := LoadPackedDatasetShard(loadedManifest, shardMetadata, registry)
		if err != nil {
			t.Fatal(err)
		}
		if len(packedShard.Records) != len(objectShard.Records) {
			t.Fatalf("packed shard %s has %d records, want %d", shardMetadata.Path, len(packedShard.Records), len(objectShard.Records))
		}
		for index, objectRecord := range objectShard.Records {
			packedRecord := packedShard.Records[index]
			if packedRecord.PositionKey != objectRecord.PositionKey || packedRecord.Outcome != objectRecord.Outcome || packedRecord.Split != objectRecord.Split {
				t.Fatalf("packed shard %s record %d identity differs", shardMetadata.Path, index)
			}
			wantExact, err := model.EngineExact(objectRecord.Trace, exactParameters)
			if err != nil {
				t.Fatal(err)
			}
			gotExact, err := model.EngineExactPacked(&packedShard, index, exactParameters)
			if err != nil {
				t.Fatal(err)
			}
			if gotExact != wantExact {
				t.Fatalf("packed exact shard %s record %d = %+v, want %+v", shardMetadata.Path, index, gotExact, wantExact)
			}
			wantContinuous, err := model.Continuous(objectRecord.Trace, continuousParameters)
			if err != nil {
				t.Fatal(err)
			}
			gotContinuous, err := model.ContinuousPacked(&packedShard, index, continuousParameters)
			if err != nil {
				t.Fatal(err)
			}
			if gotContinuous != wantContinuous {
				t.Fatalf("packed continuous shard %s record %d = %+v, want %+v", shardMetadata.Path, index, gotContinuous, wantContinuous)
			}
		}
	}
	permutationA, err := DeterministicRecordPermutation(first.Manifest.Shards[0].Records, first.Manifest.Shuffle.Seed, first.ManifestSHA256, 3, first.Manifest.Shards[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	permutationB, err := DeterministicRecordPermutation(first.Manifest.Shards[0].Records, first.Manifest.Shuffle.Seed, first.ManifestSHA256, 3, first.Manifest.Shards[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(permutationA, permutationB) {
		t.Fatal("deterministic record permutations differ")
	}
}

func TestConflictingOutcomeErrorPolicyStopsCompilation(t *testing.T) {
	registry, binding, model := newDatasetTestSystem(t)
	root := t.TempDir()
	book := filepath.Join(root, "conflict.book")
	data := "8/8/8/8/8/8/8/K6k w - - 0 1 [0.0]\n8/8/8/8/8/8/8/K6k w - - 1 2 [1.0]\n"
	if err := os.WriteFile(book, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	config := DefaultShardedCompileConfig()
	config.Compile.Conflicts = ConflictError
	config.Compile.ProgressEvery = 0
	output := filepath.Join(root, "compiled")
	if _, err := CompileBookFilesSharded(context.Background(), []string{book}, output, registry, binding, model, config); err == nil {
		t.Fatal("conflicting outcomes unexpectedly compiled under error policy")
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("failed conversion published output directory: %v", err)
	}
}
