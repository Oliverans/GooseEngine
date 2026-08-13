package tuner

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
)

type PackedSpan struct {
	Offset uint32
	Count  uint16
	_      uint16
}

type PackedLinearTerm struct {
	ParameterIndex uint16
	Units          int16
}

type PackedCandidateTarget struct {
	MGParameterIndex uint16
	EGParameterIndex uint16
}

type PackedCandidate struct {
	TargetOffset uint32
	TargetCount  uint16
	Source       uint8
	Side         int8
}

type PackedKingPasser struct {
	Side          int8
	RelativeRank  uint8
	EnemyDistance uint8
	OwnDistance   uint8
}

type PackedDangerSide struct {
	Attackers    [4]int16
	SafeChecks   [4]int16
	RingHits     int16
	UnsafeChecks int16
	HasQueen     bool
	_            [3]byte
}

type PackedSpaceSide struct {
	Safe       int16
	BehindPawn int16
	SemiOpen   int16
	Open       int16
	PieceCount int16
}

type PackedNonlinearUnits struct {
	ConnectedWhite [7]int16
	ConnectedBlack [7]int16
	KnightMobility [9]int16
	BishopMobility [14]int16
	DangerWhite    PackedDangerSide
	DangerBlack    PackedDangerSide
	SpaceWhite     PackedSpaceSide
	SpaceBlack     PackedSpaceSide
	CenterOpenness int16
	BishopPair     int16
	SpaceBlocked   int16
	TotalPawns     int16
	KnightDiff     int16
}

type PackedRecord struct {
	PositionKey PositionKey
	LinearMG    PackedSpan
	LinearEG    PackedSpan
	Candidates  PackedSpan
	KingPassers PackedSpan
	FixedMG     int32
	FixedEG     int32
	PiecePhase  uint16
	TotalPhase  uint16
	SideToMove  int8
	Outcome     Outcome
	Split       DatasetSplit
	Flags       uint8
	Nonlinear   PackedNonlinearUnits
}

const packedFlagTheoreticalDraw = 1 << 0

func (r PackedRecord) TheoreticalDraw() bool { return r.Flags&packedFlagTheoreticalDraw != 0 }

type PackedDatasetShard struct {
	Metadata    ManifestShard
	Header      DatasetHeader
	Records     []PackedRecord
	LinearTerms []PackedLinearTerm
	Candidates  []PackedCandidate
	Targets     []PackedCandidateTarget
	KingPassers []PackedKingPasser
}

func (s *PackedDatasetShard) linearTerms(span PackedSpan) []PackedLinearTerm {
	start := int(span.Offset)
	return s.LinearTerms[start : start+int(span.Count)]
}

func (s *PackedDatasetShard) candidatePassers(span PackedSpan) []PackedCandidate {
	start := int(span.Offset)
	return s.Candidates[start : start+int(span.Count)]
}

func (s *PackedDatasetShard) candidateTargets(candidate PackedCandidate) []PackedCandidateTarget {
	start := int(candidate.TargetOffset)
	return s.Targets[start : start+int(candidate.TargetCount)]
}

func (s *PackedDatasetShard) kingPassers(span PackedSpan) []PackedKingPasser {
	start := int(span.Offset)
	return s.KingPassers[start : start+int(span.Count)]
}

// LoadPackedDatasetShard decodes the existing binary format directly into
// fixed records and shared arenas. It never constructs BoundTrace slices.
func LoadPackedDatasetShard(dataset LoadedDatasetManifest, shard ManifestShard, registry *Registry) (PackedDatasetShard, error) {
	path, err := safeShardPath(dataset.Root, shard.Path)
	if err != nil {
		return PackedDatasetShard{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return PackedDatasetShard{}, err
	}
	if info.Size() != shard.Bytes {
		return PackedDatasetShard{}, fmt.Errorf("shard %q size %d, want %d", shard.Path, info.Size(), shard.Bytes)
	}
	file, err := os.Open(path)
	if err != nil {
		return PackedDatasetShard{}, err
	}
	defer file.Close()
	headerBytes := make([]byte, datasetHeaderSize)
	if _, err := io.ReadFull(file, headerBytes); err != nil {
		return PackedDatasetShard{}, err
	}
	recordHash := sha256.New()
	decoder, err := NewDatasetDecoder(io.MultiReader(bytes.NewReader(headerBytes), io.TeeReader(file, recordHash)), registry)
	if err != nil {
		return PackedDatasetShard{}, err
	}
	if err := validateShardHeader(shard, decoder.Header); err != nil {
		return PackedDatasetShard{}, err
	}
	if shard.Records > uint64(^uint(0)>>1) {
		return PackedDatasetShard{}, errors.New("shard record count overflows this platform")
	}
	recordCapacity := int(shard.Records)
	termCapacity := int(min(uint64(^uint(0)>>1), uint64(max(0, shard.Bytes))/4))
	out := PackedDatasetShard{
		Metadata:    shard,
		Header:      decoder.Header,
		Records:     make([]PackedRecord, 0, recordCapacity),
		LinearTerms: make([]PackedLinearTerm, 0, termCapacity),
		Candidates:  make([]PackedCandidate, 0, recordCapacity-recordCapacity/3),
		Targets:     make([]PackedCandidateTarget, 0, recordCapacity-recordCapacity/4),
		KingPassers: make([]PackedKingPasser, 0, recordCapacity/5*3),
	}
	for uint64(len(out.Records)) < shard.Records {
		if err := decoder.nextPacked(&out); err != nil {
			return PackedDatasetShard{}, fmt.Errorf("decode packed shard %q record %d: %w", shard.Path, len(out.Records), err)
		}
	}
	if decoder.remaining != 0 {
		return PackedDatasetShard{}, fmt.Errorf("packed shard %q has %d unread records", shard.Path, decoder.remaining)
	}
	trailing, err := io.Copy(io.Discard, decoder.r)
	if err != nil {
		return PackedDatasetShard{}, err
	}
	if trailing != 0 {
		return PackedDatasetShard{}, fmt.Errorf("packed shard %q has %d trailing bytes", shard.Path, trailing)
	}
	if got := hex.EncodeToString(recordHash.Sum(nil)); got != shard.RecordsSHA256 {
		return PackedDatasetShard{}, fmt.Errorf("shard %q record checksum %s, want %s", shard.Path, got, shard.RecordsSHA256)
	}
	return out, nil
}

func (d *DatasetDecoder) nextPacked(out *PackedDatasetShard) error {
	if d.remaining == 0 {
		return io.EOF
	}
	if _, err := io.ReadFull(d.r, d.sizeData[:]); err != nil {
		return err
	}
	size := binary.LittleEndian.Uint32(d.sizeData[:])
	if size == 0 || size > maxRecordBytes {
		return fmt.Errorf("invalid compiled record size %d", size)
	}
	if cap(d.buf) < int(size) {
		d.buf = make([]byte, size)
	} else {
		d.buf = d.buf[:size]
	}
	if _, err := io.ReadFull(d.r, d.buf); err != nil {
		return err
	}
	reader := compactReader{data: d.buf}
	out.Records = append(out.Records, PackedRecord{})
	record := &out.Records[len(out.Records)-1]
	reader.packedRecord(out, d.parameterCount, record)
	if reader.err != nil {
		out.Records = out.Records[:len(out.Records)-1]
		return reader.err
	}
	if reader.offset != len(reader.data) {
		return fmt.Errorf("compiled record has %d trailing bytes", len(reader.data)-reader.offset)
	}
	d.remaining--
	return nil
}

func (r *compactReader) packedRecord(out *PackedDatasetShard, parameterCount int, record *PackedRecord) {
	r.raw(record.PositionKey[:])
	record.Outcome = Outcome(r.u())
	record.Split = DatasetSplit(r.u())
	record.SideToMove = r.int8("side to move")
	record.PiecePhase = r.uint16("piece phase")
	record.TotalPhase = r.uint16("total phase")
	if r.boolean() {
		record.Flags |= packedFlagTheoreticalDraw
	}
	record.FixedMG = r.int32("fixed MG")
	record.FixedEG = r.int32("fixed EG")
	record.LinearMG = r.packedTerms(&out.LinearTerms, parameterCount)
	record.LinearEG = r.packedTerms(&out.LinearTerms, parameterCount)
	r.int16Array(record.Nonlinear.ConnectedWhite[:], "connected white")
	r.int16Array(record.Nonlinear.ConnectedBlack[:], "connected black")
	record.Candidates = r.packedCandidates(out, parameterCount)
	record.Nonlinear.CenterOpenness = r.int16("center openness")
	r.int16Array(record.Nonlinear.KnightMobility[:], "knight mobility")
	r.int16Array(record.Nonlinear.BishopMobility[:], "bishop mobility")
	record.Nonlinear.BishopPair = r.int16("bishop pair")
	record.Nonlinear.DangerWhite = r.packedDangerSide()
	record.Nonlinear.DangerBlack = r.packedDangerSide()
	record.KingPassers = r.packedKingPassers(&out.KingPassers)
	record.Nonlinear.SpaceWhite = r.packedSpaceSide()
	record.Nonlinear.SpaceBlack = r.packedSpaceSide()
	record.Nonlinear.SpaceBlocked = r.int16("blocked pawns")
	record.Nonlinear.TotalPawns = r.int16("total pawns")
	record.Nonlinear.KnightDiff = r.int16("knight difference")
	if !record.Outcome.valid() || !record.Split.valid() {
		r.fail("packed record has an invalid outcome or split")
	}
	if record.SideToMove != 1 && record.SideToMove != -1 {
		r.fail(fmt.Sprintf("packed side to move %d is not +1 or -1", record.SideToMove))
	}
	if record.TotalPhase == 0 {
		r.fail("packed total phase is zero")
	}
	if record.Nonlinear.CenterOpenness < -4 || record.Nonlinear.CenterOpenness > 4 {
		r.fail(fmt.Sprintf("packed center openness %d is outside [-4,4]", record.Nonlinear.CenterOpenness))
	}
}

func (r *compactReader) packedTerms(arena *[]PackedLinearTerm, parameterCount int) PackedSpan {
	count := r.count(maxTermsPerPhase, "linear term")
	span := packedSpan(len(*arena), count, r)
	for range count {
		index := r.uint()
		units := r.int16("linear units")
		if index < 0 || parameterCount != 0 && index >= parameterCount || index > math.MaxUint16 {
			r.fail(fmt.Sprintf("packed parameter index %d is outside registry vector length %d", index, parameterCount))
		}
		*arena = append(*arena, PackedLinearTerm{ParameterIndex: uint16(index), Units: units})
	}
	return span
}

func (r *compactReader) packedCandidates(out *PackedDatasetShard, parameterCount int) PackedSpan {
	count := r.count(maxCandidatePassers, "candidate passer")
	span := packedSpan(len(out.Candidates), count, r)
	for range count {
		side := r.int8("candidate side")
		source := r.signedUint8("candidate source")
		targetCount := r.count(maxCandidateTargets, "candidate target")
		candidate := PackedCandidate{
			TargetOffset: uint32ArenaOffset(len(out.Targets), r),
			TargetCount:  uint16Count(targetCount, r), Source: source, Side: side,
		}
		for range targetCount {
			mg := r.parameterIndex(parameterCount)
			eg := r.parameterIndex(parameterCount)
			out.Targets = append(out.Targets, PackedCandidateTarget{MGParameterIndex: mg, EGParameterIndex: eg})
		}
		out.Candidates = append(out.Candidates, candidate)
	}
	return span
}

func (r *compactReader) packedKingPassers(arena *[]PackedKingPasser) PackedSpan {
	count := r.count(maxKingPassers, "king passer")
	span := packedSpan(len(*arena), count, r)
	for range count {
		*arena = append(*arena, PackedKingPasser{
			Side: r.int8("king passer side"), RelativeRank: r.signedUint8("king passer rank"),
			EnemyDistance: r.signedUint8("king passer enemy distance"), OwnDistance: r.signedUint8("king passer own distance"),
		})
	}
	return span
}

func (r *compactReader) packedDangerSide() PackedDangerSide {
	var side PackedDangerSide
	r.int16Array(side.Attackers[:], "danger attackers")
	side.RingHits = r.int16("danger ring hits")
	r.int16Array(side.SafeChecks[:], "danger safe checks")
	side.UnsafeChecks = r.int16("danger unsafe checks")
	side.HasQueen = r.boolean()
	return side
}

func (r *compactReader) packedSpaceSide() PackedSpaceSide {
	return PackedSpaceSide{
		Safe: r.int16("space safe"), BehindPawn: r.int16("space behind pawn"),
		SemiOpen: r.int16("space semi-open"), Open: r.int16("space open"), PieceCount: r.int16("space piece count"),
	}
}

func packedSpan(offset, count int, r *compactReader) PackedSpan {
	return PackedSpan{Offset: uint32ArenaOffset(offset, r), Count: uint16Count(count, r)}
}

func uint32ArenaOffset(value int, r *compactReader) uint32 {
	if value < 0 || uint64(value) > math.MaxUint32 {
		r.fail(fmt.Sprintf("packed arena offset %d exceeds uint32", value))
		return 0
	}
	return uint32(value)
}

func uint16Count(value int, r *compactReader) uint16 {
	if value < 0 || value > math.MaxUint16 {
		r.fail(fmt.Sprintf("packed count %d exceeds uint16", value))
		return 0
	}
	return uint16(value)
}

func (r *compactReader) parameterIndex(parameterCount int) uint16 {
	value := r.uint()
	if value < 0 || value > math.MaxUint16 || parameterCount != 0 && value >= parameterCount {
		r.fail(fmt.Sprintf("packed parameter index %d is outside registry vector length %d", value, parameterCount))
		return 0
	}
	return uint16(value)
}

func (r *compactReader) int8(name string) int8 {
	value := r.i()
	if value < math.MinInt8 || value > math.MaxInt8 {
		r.fail(fmt.Sprintf("%s value %d exceeds int8", name, value))
	}
	return int8(value)
}

func (r *compactReader) signedUint8(name string) uint8 {
	value := r.i()
	if value < 0 || value > math.MaxUint8 {
		r.fail(fmt.Sprintf("%s value %d exceeds uint8", name, value))
	}
	return uint8(value)
}

func (r *compactReader) uint8(name string) uint8 {
	value := r.u()
	if value > math.MaxUint8 {
		r.fail(fmt.Sprintf("%s value %d exceeds uint8", name, value))
	}
	return uint8(value)
}

func (r *compactReader) int16(name string) int16 {
	value := r.i()
	if value < math.MinInt16 || value > math.MaxInt16 {
		r.fail(fmt.Sprintf("%s value %d exceeds int16", name, value))
	}
	return int16(value)
}

func (r *compactReader) uint16(name string) uint16 {
	value := r.u()
	if value > math.MaxUint16 {
		r.fail(fmt.Sprintf("%s value %d exceeds uint16", name, value))
	}
	return uint16(value)
}

func (r *compactReader) int32(name string) int32 {
	value := r.i()
	if value < math.MinInt32 || value > math.MaxInt32 {
		r.fail(fmt.Sprintf("%s value %d exceeds int32", name, value))
	}
	return int32(value)
}

func (r *compactReader) int16Array(values []int16, name string) {
	for index := range values {
		values[index] = r.int16(name)
	}
}

func (r *compactReader) fail(message string) {
	if r.err == nil {
		r.err = errors.New(message)
	}
}
