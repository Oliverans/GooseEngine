package tuner

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"

	eng "chess-engine/engine"
)

const (
	DatasetFormatVersion = 1
	datasetHeaderSize    = 120
	maxRecordBytes       = 1 << 20
	maxTermsPerPhase     = 4096
	maxCandidatePassers  = 64
	maxCandidateTargets  = 64
	maxKingPassers       = 64
)

var datasetMagic = [8]byte{'G', 'O', 'O', 'S', 'E', 'T', 'N', '2'}

// DatasetHeader is intentionally timestamp-free, making a conversion with the
// same inputs and settings byte-for-byte reproducible.
type DatasetHeader struct {
	FormatVersion         uint16
	TraceSchemaVersion    uint32
	RegistryFingerprint   [32]byte
	Records               uint64
	Splits                [3]uint64
	Outcomes              [3]uint64
	SplitSeed             uint64
	ValidationBasisPoints uint16
	TestBasisPoints       uint16
	Flags                 uint32
}

const (
	DatasetFlagDeduplicated uint32 = 1 << iota
	DatasetFlagExactVerified
)

type DatasetMetadata struct {
	Split         SplitConfig
	Deduplicated  bool
	ExactVerified bool
}

func NewDatasetHeader(registry *Registry) (DatasetHeader, error) {
	if registry == nil {
		return DatasetHeader{}, errors.New("dataset header requires a registry")
	}
	fingerprint, err := hex.DecodeString(registry.Fingerprint)
	if err != nil || len(fingerprint) != 32 {
		return DatasetHeader{}, fmt.Errorf("registry fingerprint %q is not a SHA-256 digest", registry.Fingerprint)
	}
	header := DatasetHeader{
		FormatVersion:      DatasetFormatVersion,
		TraceSchemaVersion: eng.TuningTraceSchemaVersion,
	}
	copy(header.RegistryFingerprint[:], fingerprint)
	return header, nil
}

func (h DatasetHeader) Validate(registry *Registry) error {
	if h.FormatVersion != DatasetFormatVersion {
		return fmt.Errorf("dataset format %d, want %d", h.FormatVersion, DatasetFormatVersion)
	}
	if h.TraceSchemaVersion != eng.TuningTraceSchemaVersion {
		return fmt.Errorf("tuning trace schema %d, want %d", h.TraceSchemaVersion, eng.TuningTraceSchemaVersion)
	}
	if registry != nil {
		want, err := hex.DecodeString(registry.Fingerprint)
		if err != nil || len(want) != 32 {
			return fmt.Errorf("registry fingerprint %q is not a SHA-256 digest", registry.Fingerprint)
		}
		if !bytes.Equal(h.RegistryFingerprint[:], want) {
			return fmt.Errorf("dataset registry fingerprint %x does not match %s", h.RegistryFingerprint, registry.Fingerprint)
		}
	}
	if h.Records != h.Splits[0]+h.Splits[1]+h.Splits[2] {
		return fmt.Errorf("dataset split counts do not total %d records", h.Records)
	}
	if h.Records != h.Outcomes[0]+h.Outcomes[1]+h.Outcomes[2] {
		return fmt.Errorf("dataset outcome counts do not total %d records", h.Records)
	}
	if err := (SplitConfig{ValidationBasisPoints: h.ValidationBasisPoints, TestBasisPoints: h.TestBasisPoints}).Validate(); err != nil {
		return fmt.Errorf("dataset split metadata: %w", err)
	}
	const knownFlags = DatasetFlagDeduplicated | DatasetFlagExactVerified
	if h.Flags & ^knownFlags != 0 {
		return fmt.Errorf("dataset has unknown flags 0x%x", h.Flags & ^knownFlags)
	}
	return nil
}

func writeDatasetHeader(w io.Writer, h DatasetHeader) error {
	var data [datasetHeaderSize]byte
	copy(data[:8], datasetMagic[:])
	binary.LittleEndian.PutUint16(data[8:10], h.FormatVersion)
	binary.LittleEndian.PutUint16(data[10:12], datasetHeaderSize)
	binary.LittleEndian.PutUint32(data[12:16], h.TraceSchemaVersion)
	copy(data[16:48], h.RegistryFingerprint[:])
	offset := 48
	put := func(value uint64) {
		binary.LittleEndian.PutUint64(data[offset:offset+8], value)
		offset += 8
	}
	put(h.Records)
	for _, count := range h.Splits {
		put(count)
	}
	for _, count := range h.Outcomes {
		put(count)
	}
	put(h.SplitSeed)
	binary.LittleEndian.PutUint16(data[offset:offset+2], h.ValidationBasisPoints)
	offset += 2
	binary.LittleEndian.PutUint16(data[offset:offset+2], h.TestBasisPoints)
	offset += 2
	binary.LittleEndian.PutUint32(data[offset:offset+4], h.Flags)
	_, err := w.Write(data[:])
	return err
}

func readDatasetHeader(r io.Reader) (DatasetHeader, error) {
	var data [datasetHeaderSize]byte
	if _, err := io.ReadFull(r, data[:]); err != nil {
		return DatasetHeader{}, err
	}
	if !bytes.Equal(data[:8], datasetMagic[:]) {
		return DatasetHeader{}, fmt.Errorf("not a tuner-v2 dataset (magic %q)", data[:8])
	}
	if size := binary.LittleEndian.Uint16(data[10:12]); size != datasetHeaderSize {
		return DatasetHeader{}, fmt.Errorf("dataset header size %d, want %d", size, datasetHeaderSize)
	}
	h := DatasetHeader{
		FormatVersion:      binary.LittleEndian.Uint16(data[8:10]),
		TraceSchemaVersion: binary.LittleEndian.Uint32(data[12:16]),
	}
	copy(h.RegistryFingerprint[:], data[16:48])
	offset := 48
	get := func() uint64 {
		value := binary.LittleEndian.Uint64(data[offset : offset+8])
		offset += 8
		return value
	}
	h.Records = get()
	for i := range h.Splits {
		h.Splits[i] = get()
	}
	for i := range h.Outcomes {
		h.Outcomes[i] = get()
	}
	h.SplitSeed = get()
	h.ValidationBasisPoints = binary.LittleEndian.Uint16(data[offset : offset+2])
	offset += 2
	h.TestBasisPoints = binary.LittleEndian.Uint16(data[offset : offset+2])
	offset += 2
	h.Flags = binary.LittleEndian.Uint32(data[offset : offset+4])
	return h, nil
}

// DatasetEncoder writes records sequentially and patches aggregate counts into
// the fixed header on Close. It does not close the caller-owned stream.
type DatasetEncoder struct {
	w          io.WriteSeeker
	header     DatasetHeader
	buf        bytes.Buffer
	recordHash hash.Hash
	closed     bool
}

func NewDatasetEncoder(w io.WriteSeeker, registry *Registry) (*DatasetEncoder, error) {
	return NewDatasetEncoderWithMetadata(w, registry, DatasetMetadata{})
}

func NewDatasetEncoderWithMetadata(w io.WriteSeeker, registry *Registry, metadata DatasetMetadata) (*DatasetEncoder, error) {
	if w == nil {
		return nil, errors.New("dataset encoder requires a stream")
	}
	header, err := NewDatasetHeader(registry)
	if err != nil {
		return nil, err
	}
	if err := metadata.Split.Validate(); err != nil {
		return nil, err
	}
	header.SplitSeed = metadata.Split.Seed
	header.ValidationBasisPoints = metadata.Split.ValidationBasisPoints
	header.TestBasisPoints = metadata.Split.TestBasisPoints
	if metadata.Deduplicated {
		header.Flags |= DatasetFlagDeduplicated
	}
	if metadata.ExactVerified {
		header.Flags |= DatasetFlagExactVerified
	}
	if _, err := w.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	if err := writeDatasetHeader(w, header); err != nil {
		return nil, err
	}
	return &DatasetEncoder{w: w, header: header, recordHash: sha256.New()}, nil
}

func (e *DatasetEncoder) Encode(record CompiledTrainingRecord) error {
	if e == nil || e.closed {
		return errors.New("dataset encoder is closed")
	}
	if !record.Outcome.valid() {
		return fmt.Errorf("invalid outcome %d", record.Outcome)
	}
	if !record.Split.valid() {
		return fmt.Errorf("invalid dataset split %d", record.Split)
	}
	if record.Trace.SchemaVersion != int(e.header.TraceSchemaVersion) {
		return fmt.Errorf("record trace schema %d, want %d", record.Trace.SchemaVersion, e.header.TraceSchemaVersion)
	}
	if record.Trace.SideToMove != 1 && record.Trace.SideToMove != -1 {
		return fmt.Errorf("record side to move %d is not +1 or -1", record.Trace.SideToMove)
	}
	if record.Trace.TotalPhase <= 0 || record.Trace.PiecePhase < 0 {
		return fmt.Errorf("invalid record phase %d/%d", record.Trace.PiecePhase, record.Trace.TotalPhase)
	}
	if len(record.Trace.LinearMG) > maxTermsPerPhase || len(record.Trace.LinearEG) > maxTermsPerPhase {
		return errors.New("record has too many linear terms")
	}
	if len(record.Trace.Nonlinear.CandidatePassers) > maxCandidatePassers || len(record.Trace.Nonlinear.KingPassers) > maxKingPassers {
		return errors.New("record has too many nonlinear observations")
	}
	e.buf.Reset()
	writer := compactWriter{w: &e.buf}
	writer.raw(record.PositionKey[:])
	writer.u(uint64(record.Outcome))
	writer.u(uint64(record.Split))
	writer.i(int64(record.Trace.SideToMove))
	writer.u(uint64(record.Trace.PiecePhase))
	writer.u(uint64(record.Trace.TotalPhase))
	writer.boolean(record.Trace.TheoreticalDraw)
	writer.i(int64(record.Trace.Fixed.MG))
	writer.i(int64(record.Trace.Fixed.EG))
	writer.terms(record.Trace.LinearMG)
	writer.terms(record.Trace.LinearEG)
	writer.nonlinear(record.Trace.Nonlinear)
	if writer.err != nil {
		return writer.err
	}
	if e.buf.Len() > maxRecordBytes {
		return fmt.Errorf("compiled record is %d bytes, limit is %d", e.buf.Len(), maxRecordBytes)
	}
	var size [4]byte
	binary.LittleEndian.PutUint32(size[:], uint32(e.buf.Len()))
	if _, err := e.w.Write(size[:]); err != nil {
		return err
	}
	if _, err := e.w.Write(e.buf.Bytes()); err != nil {
		return err
	}
	_, _ = e.recordHash.Write(size[:])
	_, _ = e.recordHash.Write(e.buf.Bytes())
	e.header.Records++
	e.header.Splits[record.Split]++
	e.header.Outcomes[record.Outcome]++
	return nil
}

func (e *DatasetEncoder) Close() error {
	if e == nil || e.closed {
		return nil
	}
	e.closed = true
	if _, err := e.w.Seek(0, io.SeekStart); err != nil {
		return err
	}
	return writeDatasetHeader(e.w, e.header)
}

func (e *DatasetEncoder) Header() DatasetHeader { return e.header }

// RecordsSHA256 covers every length prefix and record payload in write order.
// Header fields are represented separately in the manifest and validated by
// DatasetDecoder.
func (e *DatasetEncoder) RecordsSHA256() string {
	if e == nil || e.recordHash == nil {
		return ""
	}
	return hex.EncodeToString(e.recordHash.Sum(nil))
}

// DatasetDecoder validates compatibility before exposing any training rows.
type DatasetDecoder struct {
	r              *bufio.Reader
	Header         DatasetHeader
	remaining      uint64
	buf            []byte
	sizeData       [4]byte
	parameterCount int
}

func NewDatasetDecoder(r io.Reader, registry *Registry) (*DatasetDecoder, error) {
	if r == nil {
		return nil, errors.New("dataset decoder requires a stream")
	}
	buffered := bufio.NewReaderSize(r, 128*1024)
	header, err := readDatasetHeader(buffered)
	if err != nil {
		return nil, err
	}
	if err := header.Validate(registry); err != nil {
		return nil, err
	}
	parameterCount := 0
	if registry != nil {
		parameterCount = len(registry.Elements)
	}
	return &DatasetDecoder{r: buffered, Header: header, remaining: header.Records, parameterCount: parameterCount}, nil
}

func (d *DatasetDecoder) Next() (CompiledTrainingRecord, error) {
	if d.remaining == 0 {
		return CompiledTrainingRecord{}, io.EOF
	}
	if _, err := io.ReadFull(d.r, d.sizeData[:]); err != nil {
		return CompiledTrainingRecord{}, err
	}
	size := binary.LittleEndian.Uint32(d.sizeData[:])
	if size == 0 || size > maxRecordBytes {
		return CompiledTrainingRecord{}, fmt.Errorf("invalid compiled record size %d", size)
	}
	if cap(d.buf) < int(size) {
		d.buf = make([]byte, size)
	} else {
		d.buf = d.buf[:size]
	}
	if _, err := io.ReadFull(d.r, d.buf); err != nil {
		return CompiledTrainingRecord{}, err
	}
	reader := compactReader{data: d.buf}
	var record CompiledTrainingRecord
	reader.raw(record.PositionKey[:])
	record.Outcome = Outcome(reader.u())
	record.Split = DatasetSplit(reader.u())
	record.Trace.SchemaVersion = int(d.Header.TraceSchemaVersion)
	record.Trace.SideToMove = reader.int()
	record.Trace.PiecePhase = reader.uint()
	record.Trace.TotalPhase = reader.uint()
	record.Trace.TheoreticalDraw = reader.boolean()
	record.Trace.Fixed.MG = reader.int()
	record.Trace.Fixed.EG = reader.int()
	record.Trace.LinearMG = reader.terms(maxTermsPerPhase)
	record.Trace.LinearEG = reader.terms(maxTermsPerPhase)
	record.Trace.Nonlinear = reader.nonlinear()
	if reader.err != nil {
		return CompiledTrainingRecord{}, reader.err
	}
	if reader.offset != len(reader.data) {
		return CompiledTrainingRecord{}, fmt.Errorf("compiled record has %d trailing bytes", len(reader.data)-reader.offset)
	}
	if !record.Outcome.valid() || !record.Split.valid() {
		return CompiledTrainingRecord{}, errors.New("compiled record has an invalid outcome or split")
	}
	if err := validateDecodedRecord(record, d.parameterCount); err != nil {
		return CompiledTrainingRecord{}, err
	}
	d.remaining--
	return record, nil
}

func validateDecodedRecord(record CompiledTrainingRecord, parameterCount int) error {
	trace := record.Trace
	if trace.SideToMove != 1 && trace.SideToMove != -1 {
		return fmt.Errorf("compiled record side to move %d is not +1 or -1", trace.SideToMove)
	}
	if trace.TotalPhase <= 0 || trace.PiecePhase < 0 {
		return fmt.Errorf("invalid compiled phase %d/%d", trace.PiecePhase, trace.TotalPhase)
	}
	if trace.Nonlinear.CenterOpenness < -4 || trace.Nonlinear.CenterOpenness > 4 {
		return fmt.Errorf("compiled center openness %d is outside [-4,4]", trace.Nonlinear.CenterOpenness)
	}
	checkIndex := func(index int) error {
		if index < 0 || parameterCount != 0 && index >= parameterCount {
			return fmt.Errorf("compiled parameter index %d is outside registry vector length %d", index, parameterCount)
		}
		return nil
	}
	for _, terms := range [][]BoundLinearTerm{trace.LinearMG, trace.LinearEG} {
		for _, term := range terms {
			if err := checkIndex(term.ParameterIndex); err != nil {
				return err
			}
		}
	}
	for _, candidate := range trace.Nonlinear.CandidatePassers {
		if candidate.Side != 1 && candidate.Side != -1 {
			return fmt.Errorf("compiled candidate side %d is not +1 or -1", candidate.Side)
		}
		for _, target := range candidate.Targets {
			if err := checkIndex(target.MGParameterIndex); err != nil {
				return err
			}
			if err := checkIndex(target.EGParameterIndex); err != nil {
				return err
			}
		}
	}
	for _, passer := range trace.Nonlinear.KingPassers {
		if passer.Side != 1 && passer.Side != -1 {
			return fmt.Errorf("compiled king-passer side %d is not +1 or -1", passer.Side)
		}
	}
	return nil
}

type compactWriter struct {
	w   io.Writer
	err error
	buf [binary.MaxVarintLen64]byte
}

func (w *compactWriter) raw(data []byte) {
	if w.err == nil {
		_, w.err = w.w.Write(data)
	}
}

func (w *compactWriter) u(value uint64) {
	if w.err != nil {
		return
	}
	n := binary.PutUvarint(w.buf[:], value)
	_, w.err = w.w.Write(w.buf[:n])
}

func (w *compactWriter) i(value int64) {
	if w.err != nil {
		return
	}
	n := binary.PutVarint(w.buf[:], value)
	_, w.err = w.w.Write(w.buf[:n])
}

func (w *compactWriter) boolean(value bool) {
	if value {
		w.u(1)
	} else {
		w.u(0)
	}
}

func (w *compactWriter) terms(terms []BoundLinearTerm) {
	w.u(uint64(len(terms)))
	for _, term := range terms {
		if term.ParameterIndex < 0 {
			w.err = fmt.Errorf("negative parameter index %d", term.ParameterIndex)
			return
		}
		w.u(uint64(term.ParameterIndex))
		w.i(int64(term.Units))
	}
}

func (w *compactWriter) intArray(values []int) {
	for _, value := range values {
		w.i(int64(value))
	}
}

func (w *compactWriter) dangerSide(side eng.TuningDangerSide) {
	w.intArray(side.Attackers[:])
	w.i(int64(side.RingHits))
	w.intArray(side.SafeChecks[:])
	w.i(int64(side.UnsafeChecks))
	w.boolean(side.HasQueen)
}

func (w *compactWriter) spaceSide(side eng.TuningSpaceSide) {
	w.i(int64(side.Safe))
	w.i(int64(side.BehindPawn))
	w.i(int64(side.SemiOpen))
	w.i(int64(side.Open))
	w.i(int64(side.PieceCount))
}

func (w *compactWriter) nonlinear(units BoundNonlinearUnits) {
	w.intArray(units.Connected.White[:])
	w.intArray(units.Connected.Black[:])
	w.u(uint64(len(units.CandidatePassers)))
	for _, candidate := range units.CandidatePassers {
		if len(candidate.Targets) > maxCandidateTargets {
			w.err = errors.New("candidate passer has too many targets")
			return
		}
		w.i(int64(candidate.Side))
		w.i(int64(candidate.Source))
		w.u(uint64(len(candidate.Targets)))
		for _, target := range candidate.Targets {
			w.index(target.MGParameterIndex)
			w.index(target.EGParameterIndex)
		}
	}
	w.i(int64(units.CenterOpenness))
	w.intArray(units.KnightMobility[:])
	w.intArray(units.BishopMobility[:])
	w.i(int64(units.BishopPair))
	w.dangerSide(units.Danger.White)
	w.dangerSide(units.Danger.Black)
	w.u(uint64(len(units.KingPassers)))
	for _, passer := range units.KingPassers {
		w.i(int64(passer.Side))
		w.i(int64(passer.RelativeRank))
		w.i(int64(passer.EnemyDistance))
		w.i(int64(passer.OwnDistance))
	}
	w.spaceSide(units.Space.White)
	w.spaceSide(units.Space.Black)
	w.i(int64(units.Space.BlockedPawns))
	w.i(int64(units.Imbalance.TotalPawns))
	w.i(int64(units.Imbalance.KnightDiff))
}

func (w *compactWriter) index(value int) {
	if value < 0 {
		w.err = fmt.Errorf("negative parameter index %d", value)
		return
	}
	w.u(uint64(value))
}

type compactReader struct {
	data   []byte
	offset int
	err    error
}

func (r *compactReader) raw(out []byte) {
	if r.err != nil {
		return
	}
	if len(r.data)-r.offset < len(out) {
		r.err = io.ErrUnexpectedEOF
		return
	}
	copy(out, r.data[r.offset:r.offset+len(out)])
	r.offset += len(out)
}

func (r *compactReader) u() uint64 {
	if r.err != nil {
		return 0
	}
	value, n := binary.Uvarint(r.data[r.offset:])
	if n <= 0 {
		r.err = errors.New("invalid unsigned varint in compiled record")
		return 0
	}
	r.offset += n
	return value
}

func (r *compactReader) i() int64 {
	if r.err != nil {
		return 0
	}
	value, n := binary.Varint(r.data[r.offset:])
	if n <= 0 {
		r.err = errors.New("invalid signed varint in compiled record")
		return 0
	}
	r.offset += n
	return value
}

func (r *compactReader) int() int { return int(r.i()) }

func (r *compactReader) uint() int {
	value := r.u()
	if value > uint64(^uint(0)>>1) {
		r.err = errors.New("compiled integer overflows this platform")
		return 0
	}
	return int(value)
}

func (r *compactReader) boolean() bool {
	value := r.u()
	if value > 1 {
		r.err = fmt.Errorf("invalid encoded boolean %d", value)
	}
	return value == 1
}

func (r *compactReader) count(limit int, name string) int {
	value := r.u()
	if value > uint64(limit) {
		r.err = fmt.Errorf("compiled %s count %d exceeds %d", name, value, limit)
		return 0
	}
	return int(value)
}

func (r *compactReader) terms(limit int) []BoundLinearTerm {
	count := r.count(limit, "linear term")
	out := make([]BoundLinearTerm, count)
	for i := range out {
		out[i] = BoundLinearTerm{ParameterIndex: r.uint(), Units: r.int()}
	}
	return out
}

func (r *compactReader) intArray(values []int) {
	for i := range values {
		values[i] = r.int()
	}
}

func (r *compactReader) dangerSide() eng.TuningDangerSide {
	var out eng.TuningDangerSide
	r.intArray(out.Attackers[:])
	out.RingHits = r.int()
	r.intArray(out.SafeChecks[:])
	out.UnsafeChecks = r.int()
	out.HasQueen = r.boolean()
	return out
}

func (r *compactReader) spaceSide() eng.TuningSpaceSide {
	return eng.TuningSpaceSide{
		Safe: r.int(), BehindPawn: r.int(), SemiOpen: r.int(), Open: r.int(), PieceCount: r.int(),
	}
}

func (r *compactReader) nonlinear() BoundNonlinearUnits {
	var out BoundNonlinearUnits
	r.intArray(out.Connected.White[:])
	r.intArray(out.Connected.Black[:])
	candidates := r.count(maxCandidatePassers, "candidate passer")
	if candidates != 0 {
		out.CandidatePassers = make([]BoundCandidatePasser, candidates)
	}
	for i := range out.CandidatePassers {
		candidate := &out.CandidatePassers[i]
		candidate.Side = r.int()
		candidate.Source = r.int()
		targets := r.count(maxCandidateTargets, "candidate target")
		candidate.Targets = make([]BoundCandidateTarget, targets)
		for j := range candidate.Targets {
			candidate.Targets[j] = BoundCandidateTarget{MGParameterIndex: r.uint(), EGParameterIndex: r.uint()}
		}
	}
	out.CenterOpenness = r.int()
	r.intArray(out.KnightMobility[:])
	r.intArray(out.BishopMobility[:])
	out.BishopPair = r.int()
	out.Danger.White = r.dangerSide()
	out.Danger.Black = r.dangerSide()
	passers := r.count(maxKingPassers, "king passer")
	if passers != 0 {
		out.KingPassers = make([]eng.TuningKingPasser, passers)
	}
	for i := range out.KingPassers {
		out.KingPassers[i] = eng.TuningKingPasser{
			Side: r.int(), RelativeRank: r.int(), EnemyDistance: r.int(), OwnDistance: r.int(),
		}
	}
	out.Space.White = r.spaceSide()
	out.Space.Black = r.spaceSide()
	out.Space.BlockedPawns = r.int()
	out.Imbalance.TotalPawns = r.int()
	out.Imbalance.KnightDiff = r.int()
	return out
}
