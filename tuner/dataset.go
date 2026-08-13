package tuner

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"strings"

	eng "chess-engine/engine"
	gm "chess-engine/goosemg"
)

// Outcome is the game result from White's point of view.
type Outcome uint8

const (
	OutcomeBlackWin Outcome = iota
	OutcomeDraw
	OutcomeWhiteWin
)

func (o Outcome) WhiteScore() float64 {
	switch o {
	case OutcomeBlackWin:
		return 0
	case OutcomeDraw:
		return 0.5
	case OutcomeWhiteWin:
		return 1
	default:
		return 0
	}
}

func (o Outcome) valid() bool { return o <= OutcomeWhiteWin }

// DatasetSplit is persisted with every compiled record. Split assignment is a
// pure function of the position identity and SplitConfig, so input order and
// parallel conversion cannot change it.
type DatasetSplit uint8

const (
	SplitTraining DatasetSplit = iota
	SplitValidation
	SplitTest
)

func (s DatasetSplit) valid() bool { return s <= SplitTest }

// SplitConfig uses basis points to avoid platform-dependent floating-point
// threshold behavior. Validation and test ranges are disjoint; the remainder
// is training data.
type SplitConfig struct {
	Seed                  uint64 `json:"seed"`
	ValidationBasisPoints uint16 `json:"validationBasisPoints"`
	TestBasisPoints       uint16 `json:"testBasisPoints"`
}

func DefaultSplitConfig() SplitConfig {
	return SplitConfig{Seed: 0x676f6f736574756e, ValidationBasisPoints: 1000}
}

func (c SplitConfig) Validate() error {
	if uint32(c.ValidationBasisPoints)+uint32(c.TestBasisPoints) > 10000 {
		return fmt.Errorf("validation and test splits total more than 100%%")
	}
	return nil
}

// BookPosition is one validated source row. FEN is the engine's canonical
// six-field form; IdentityFEN excludes move clocks because they are not engine
// evaluation features.
type BookPosition struct {
	FEN         string
	IdentityFEN string
	Board       *gm.Board
	Outcome     Outcome
	// SourceScore is an optional signed integer annotation found in some book
	// exports. It is retained for provenance statistics but is not a training
	// label or part of the compiled record.
	SourceScore *int
}

// ParseBookLine parses "six-field FEN [white result]" and the E12.52 variant
// with one trailing signed integer source score.
// It is deliberately strict: malformed training data is reported rather than
// silently changing the effective dataset.
func ParseBookLine(line string) (BookPosition, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return BookPosition{}, errors.New("empty book line")
	}
	open := strings.LastIndexByte(line, '[')
	if open < 0 {
		return BookPosition{}, errors.New("book line must contain [0.0], [0.5], or [1.0]")
	}
	closeOffset := strings.IndexByte(line[open+1:], ']')
	if closeOffset < 0 {
		return BookPosition{}, errors.New("book result is missing its closing bracket")
	}
	close := open + 1 + closeOffset
	fen := strings.TrimSpace(line[:open])
	fields := strings.Fields(fen)
	if len(fields) != 6 {
		return BookPosition{}, fmt.Errorf("FEN has %d fields, want 6", len(fields))
	}
	halfmove, err := strconv.Atoi(fields[4])
	if err != nil || halfmove < 0 {
		return BookPosition{}, fmt.Errorf("FEN halfmove clock %q is not a non-negative integer", fields[4])
	}
	fullmove, err := strconv.Atoi(fields[5])
	if err != nil || fullmove < 1 {
		return BookPosition{}, fmt.Errorf("FEN fullmove number %q is not a positive integer", fields[5])
	}
	labelText := strings.TrimSpace(line[open+1 : close])
	label, err := strconv.ParseFloat(labelText, 64)
	if err != nil {
		return BookPosition{}, fmt.Errorf("invalid result %q: %w", labelText, err)
	}
	var outcome Outcome
	switch label {
	case 0:
		outcome = OutcomeBlackWin
	case 0.5:
		outcome = OutcomeDraw
	case 1:
		outcome = OutcomeWhiteWin
	default:
		return BookPosition{}, fmt.Errorf("result %q is not 0.0, 0.5, or 1.0", labelText)
	}
	var sourceScore *int
	trailing := strings.Fields(strings.TrimSpace(line[close+1:]))
	if len(trailing) > 1 {
		return BookPosition{}, fmt.Errorf("book line has %d trailing fields after its result, want at most one source score", len(trailing))
	}
	if len(trailing) == 1 {
		value, parseErr := strconv.Atoi(trailing[0])
		if parseErr != nil {
			return BookPosition{}, fmt.Errorf("invalid trailing source score %q: %w", trailing[0], parseErr)
		}
		sourceScore = &value
	}
	board, err := gm.ParseFEN(strings.Join(fields, " "))
	if err != nil {
		return BookPosition{}, err
	}
	canonical := board.ToFEN()
	canonicalFields := strings.Fields(canonical)
	return BookPosition{
		FEN:         canonical,
		IdentityFEN: strings.Join(canonicalFields[:4], " "),
		Board:       board,
		Outcome:     outcome,
		SourceScore: sourceScore,
	}, nil
}

// PositionKey is a stable 128-bit identity used for deduplication and persisted
// diagnostics. It is the first half of SHA-256, not a Go runtime hash.
type PositionKey [16]byte

func KeyForPosition(identityFEN string) PositionKey {
	sum := sha256.Sum256([]byte(identityFEN))
	var key PositionKey
	copy(key[:], sum[:len(key)])
	return key
}

func AssignSplit(identityFEN string, config SplitConfig) (DatasetSplit, error) {
	if err := config.Validate(); err != nil {
		return 0, err
	}
	h := sha256.New()
	var seed [8]byte
	binary.LittleEndian.PutUint64(seed[:], config.Seed)
	_, _ = h.Write(seed[:])
	_, _ = h.Write([]byte(identityFEN))
	sum := h.Sum(nil)
	bucket := binary.LittleEndian.Uint64(sum[:8]) % 10000
	if bucket < uint64(config.ValidationBasisPoints) {
		return SplitValidation, nil
	}
	if bucket < uint64(config.ValidationBasisPoints)+uint64(config.TestBasisPoints) {
		return SplitTest, nil
	}
	return SplitTraining, nil
}

// CompiledTrainingRecord contains everything required by the forward pass and
// loss, with no board state and no need to rerun engine evaluation.
type CompiledTrainingRecord struct {
	PositionKey PositionKey
	Outcome     Outcome
	Split       DatasetSplit
	Trace       BoundTrace
}

func CompilePosition(position BookPosition, binding *TraceBinding) (CompiledTrainingRecord, error) {
	if position.Board == nil {
		return CompiledTrainingRecord{}, errors.New("cannot compile a nil board")
	}
	if binding == nil {
		return CompiledTrainingRecord{}, errors.New("cannot compile without a trace binding")
	}
	trace := eng.TuningTraceForBoard(position.Board)
	bound, err := binding.BindTrace(trace)
	if err != nil {
		return CompiledTrainingRecord{}, err
	}
	return CompiledTrainingRecord{
		PositionKey: KeyForPosition(position.IdentityFEN),
		Outcome:     position.Outcome,
		Trace:       bound,
	}, nil
}
