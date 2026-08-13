package tuner

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	eng "chess-engine/engine"
)

func TestParseBookLineCanonicalizesAndValidates(t *testing.T) {
	position, err := ParseBookLine("  8/4k3/7p/8/4bP2/P1p1P3/5KP1/3B4 w - - 1 55 [1.0]  ")
	if err != nil {
		t.Fatalf("ParseBookLine() error = %v", err)
	}
	if position.Outcome != OutcomeWhiteWin || position.Outcome.WhiteScore() != 1 {
		t.Fatalf("outcome = %v", position.Outcome)
	}
	if position.FEN != "8/4k3/7p/8/4bP2/P1p1P3/5KP1/3B4 w - - 1 55" {
		t.Fatalf("canonical FEN = %q", position.FEN)
	}
	if position.IdentityFEN != "8/4k3/7p/8/4bP2/P1p1P3/5KP1/3B4 w - -" {
		t.Fatalf("identity FEN = %q", position.IdentityFEN)
	}
	annotated, err := ParseBookLine("2k2rnr/1ppqb1p1/p1n3b1/3pp1P1/7p/2NPBN1P/PPP1QPB1/1K1R3R w - - 0 18 [0.5] -34")
	if err != nil {
		t.Fatalf("ParseBookLine() annotated error = %v", err)
	}
	if annotated.SourceScore == nil || *annotated.SourceScore != -34 || annotated.Outcome != OutcomeDraw {
		t.Fatalf("annotated position score/outcome = %v/%v", annotated.SourceScore, annotated.Outcome)
	}
	invalid := []string{
		"",
		"8/8/8/8/8/8/8/K6k w - - 0 1",
		"8/8/8/8/8/8/8/K6k w - - 0 [0.5]",
		"8/8/8/8/8/8/8/K6k w - - 0 1 [0.25]",
		"8/8/8/8/8/8/8/K6k w - - 0 1 [0.5] not-a-score",
		"8/8/8/8/8/8/8/K6k w - - 0 1 [0.5] 1 extra",
		"not-a-fen [0.5]",
	}
	for _, line := range invalid {
		if _, err := ParseBookLine(line); err == nil {
			t.Errorf("ParseBookLine(%q) unexpectedly succeeded", line)
		}
	}
}

func TestPositionIdentityMakesClockVariantsShareSplit(t *testing.T) {
	first, err := ParseBookLine("8/8/8/8/8/8/8/K6k w - - 0 1 [0.5]")
	if err != nil {
		t.Fatal(err)
	}
	second, err := ParseBookLine("8/8/8/8/8/8/8/K6k w - - 99 73 [0.5]")
	if err != nil {
		t.Fatal(err)
	}
	if first.IdentityFEN != second.IdentityFEN || KeyForPosition(first.IdentityFEN) != KeyForPosition(second.IdentityFEN) {
		t.Fatal("move clocks changed position identity")
	}
	config := SplitConfig{Seed: 42, ValidationBasisPoints: 2500, TestBasisPoints: 2500}
	firstSplit, err := AssignSplit(first.IdentityFEN, config)
	if err != nil {
		t.Fatal(err)
	}
	secondSplit, err := AssignSplit(second.IdentityFEN, config)
	if err != nil {
		t.Fatal(err)
	}
	if firstSplit != secondSplit {
		t.Fatalf("clock variants split as %v and %v", firstSplit, secondSplit)
	}
}

func TestCompiledDatasetAcceptsPromotionPhaseAboveTotal(t *testing.T) {
	registry, binding, model := newDatasetTestSystem(t)
	position, err := ParseBookLine("Qnk4r/1pprq1np/8/1P2p3/1bNpp1p1/1Q1P2P1/1B2PP1P/R4RK1 b - - 0 23 [1.0]")
	if err != nil {
		t.Fatal(err)
	}
	record, err := CompilePosition(position, binding)
	if err != nil {
		t.Fatal(err)
	}
	if record.Trace.PiecePhase != 25 || record.Trace.TotalPhase != 24 {
		t.Fatalf("promotion phase = %d/%d, want 25/24", record.Trace.PiecePhase, record.Trace.TotalPhase)
	}
	parameters, err := InitialExactParameters(registry)
	if err != nil {
		t.Fatal(err)
	}
	exact, err := model.EngineExact(record.Trace, parameters)
	if err != nil {
		t.Fatal(err)
	}
	if exact.Buckets != record.Trace.Reference.Buckets || exact.SideToMove != record.Trace.Reference.SideToMove {
		t.Fatalf("promotion exact result = %+v, want %+v", exact, record.Trace.Reference)
	}
	path := filepath.Join(t.TempDir(), "promotion.tune")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	encoder, err := NewDatasetEncoder(file, registry)
	if err != nil {
		t.Fatal(err)
	}
	if err := encoder.Encode(record); err != nil {
		t.Fatal(err)
	}
	if err := encoder.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	file, err = os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	decoder, err := NewDatasetDecoder(file, registry)
	if err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	decoded, err := decoder.Next()
	_ = file.Close()
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Trace.PiecePhase != 25 || decoded.Trace.TotalPhase != 24 {
		t.Fatalf("decoded promotion phase = %d/%d", decoded.Trace.PiecePhase, decoded.Trace.TotalPhase)
	}
}

func TestCompiledDatasetBinaryRoundTrip(t *testing.T) {
	registry, binding, model := newDatasetTestSystem(t)
	position, err := ParseBookLine("r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1 [1.0]")
	if err != nil {
		t.Fatal(err)
	}
	record, err := CompilePosition(position, binding)
	if err != nil {
		t.Fatal(err)
	}
	record.Split = SplitValidation
	path := filepath.Join(t.TempDir(), "roundtrip.tune")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	encoder, err := NewDatasetEncoder(file, registry)
	if err != nil {
		t.Fatal(err)
	}
	if err := encoder.Encode(record); err != nil {
		t.Fatal(err)
	}
	if err := encoder.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	file, err = os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	decoder, err := NewDatasetDecoder(file, registry)
	if err != nil {
		t.Fatal(err)
	}
	if decoder.Header.Records != 1 || decoder.Header.Splits[SplitValidation] != 1 || decoder.Header.Outcomes[OutcomeWhiteWin] != 1 {
		t.Fatalf("header counts = %+v", decoder.Header)
	}
	got, err := decoder.Next()
	if err != nil {
		t.Fatal(err)
	}
	want := record
	want.Trace.FEN = ""
	want.Trace.Reference = eng.TuningReferenceTrace{}
	got.Trace.FEN = ""
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip mismatch\ngot:  %#v\nwant: %#v", got, want)
	}
	if _, err := decoder.Next(); err != io.EOF {
		t.Fatalf("second Next() error = %v, want EOF", err)
	}
	parameters, err := InitialExactParameters(registry)
	if err != nil {
		t.Fatal(err)
	}
	exact, err := model.EngineExact(got.Trace, parameters)
	if err != nil {
		t.Fatal(err)
	}
	if exact.Buckets != record.Trace.Reference.Buckets || exact.WhitePerspective != record.Trace.Reference.WhitePerspective || exact.SideToMove != record.Trace.Reference.SideToMove {
		t.Fatalf("decoded exact result = %+v, want %+v", exact, record.Trace.Reference)
	}
}

func TestCompileBookFilesIsDeterministicAndDeduplicates(t *testing.T) {
	registry, binding, model := newDatasetTestSystem(t)
	dir := t.TempDir()
	book := filepath.Join(dir, "fixture.book")
	data := []byte(
		"8/8/8/8/8/8/8/K6k w - - 0 1 [0.5]\n" +
			"8/8/8/8/8/8/8/K6k w - - 40 20 [0.5]\n" +
			"r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1 [1.0]\n")
	if err := os.WriteFile(book, data, 0o644); err != nil {
		t.Fatal(err)
	}
	config := DefaultCompileConfig()
	config.Split = SplitConfig{Seed: 7}
	config.ProgressEvery = 0
	firstPath := filepath.Join(dir, "first.tune")
	firstStats, err := CompileBookFiles(context.Background(), []string{book}, firstPath, registry, binding, model, config)
	if err != nil {
		t.Fatal(err)
	}
	if firstStats.SourceLines != 3 || firstStats.Records != 2 || firstStats.Duplicates != 1 || firstStats.Splits.Training != 2 {
		t.Fatalf("unexpected stats: %+v", firstStats)
	}
	if len(firstStats.FeatureFrequency) != len(registry.Elements) {
		t.Fatalf("feature frequency has %d cells, want %d", len(firstStats.FeatureFrequency), len(registry.Elements))
	}
	secondPath := filepath.Join(dir, "second.tune")
	if _, err := CompileBookFiles(context.Background(), []string{book}, secondPath, registry, binding, model, config); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(firstPath)
	if err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(secondPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("identical conversions produced different bytes")
	}
	file, err := os.Open(firstPath)
	if err != nil {
		t.Fatal(err)
	}
	decoder, err := NewDatasetDecoder(file, registry)
	if err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if decoder.Header.SplitSeed != config.Split.Seed || decoder.Header.Flags&(DatasetFlagDeduplicated|DatasetFlagExactVerified) != DatasetFlagDeduplicated|DatasetFlagExactVerified {
		t.Fatalf("conversion metadata was not persisted: %+v", decoder.Header)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func newDatasetTestSystem(t *testing.T) (*Registry, *TraceBinding, *ForwardModel) {
	t.Helper()
	registry, err := NewEngineRegistry()
	if err != nil {
		t.Fatal(err)
	}
	binding, err := NewTraceBinding(registry)
	if err != nil {
		t.Fatal(err)
	}
	model, err := NewForwardModel(registry, binding)
	if err != nil {
		t.Fatal(err)
	}
	return registry, binding, model
}
