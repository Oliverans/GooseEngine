package engine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	gm "chess-engine/goosemg"
)

func TestPositionTraceLevelsAreAdditive(t *testing.T) {
	for _, test := range []struct {
		level      PositionTraceLevel
		wantExtend bool
		wantMoves  bool
	}{
		{PositionTraceBasic, false, false},
		{PositionTraceExtended, true, false},
		{PositionTraceMoves, true, true},
	} {
		board := gm.ParseFen(gm.Startpos)
		trace := PositionTraceForBoard(&board, test.level)
		if (trace.Extended != nil) != test.wantExtend {
			t.Fatalf("level %s: extended present = %v, want %v", test.level, trace.Extended != nil, test.wantExtend)
		}
		if (len(trace.Moves) > 0) != test.wantMoves {
			t.Fatalf("level %s: move deltas present = %v, want %v", test.level, len(trace.Moves) > 0, test.wantMoves)
		}
		if trace.Choices.LegalCount != 20 || len(trace.Choices.Moves) != 20 {
			t.Fatalf("level %s: start position legal count = %d/%d, want 20", test.level, trace.Choices.LegalCount, len(trace.Choices.Moves))
		}
	}
}

func TestPositionTraceDoesNotMutateBoardOrEvaluation(t *testing.T) {
	const fen = "r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1"
	for _, level := range []PositionTraceLevel{PositionTraceBasic, PositionTraceExtended, PositionTraceMoves} {
		board := gm.ParseFen(fen)
		beforeFEN := board.ToFen()
		beforeEval := Evaluation(&board, false)

		_ = ExplainTraceForBoard(&board, level)

		if got := board.ToFen(); got != beforeFEN {
			t.Fatalf("level %s mutated board:\n before %s\n after  %s", level, beforeFEN, got)
		}
		if got := Evaluation(&board, false); got != beforeEval {
			t.Fatalf("level %s changed evaluation: before %d, after %d", level, beforeEval, got)
		}
	}
}

func TestPositionTraceControlAndMoveConsistency(t *testing.T) {
	board := gm.ParseFen("r1bq1rk1/ppp2ppp/2nb1n2/3pp3/2B1P3/2NP1N2/PPP2PPP/R1BQ1RK1 w - - 0 1")
	trace := PositionTraceForBoard(&board, PositionTraceMoves)

	var white, black uint64
	for _, piece := range trace.Pieces {
		if piece.Side == "white" {
			white |= parseTraceBitboard(t, piece.GeometricAttacks.Hex)
		} else {
			black |= parseTraceBitboard(t, piece.GeometricAttacks.Hex)
		}
	}
	if got := parseTraceBitboard(t, trace.Control.White.Union.Hex); got != white {
		t.Fatalf("white control union = %#x, piece union = %#x", got, white)
	}
	if got := parseTraceBitboard(t, trace.Control.Black.Union.Hex); got != black {
		t.Fatalf("black control union = %#x, piece union = %#x", got, black)
	}
	if len(trace.Moves) != trace.Choices.LegalCount {
		t.Fatalf("move deltas = %d, legal moves = %d", len(trace.Moves), trace.Choices.LegalCount)
	}
}

func TestPositionTraceAbsolutePin(t *testing.T) {
	board := gm.ParseFen("4r1k1/8/8/8/8/8/4R3/4K3 w - - 0 1")
	trace := PositionTraceForBoard(&board, PositionTraceBasic)
	for _, piece := range trace.Pieces {
		if piece.Square != "e2" {
			continue
		}
		if piece.AbsolutePin == nil || piece.AbsolutePin.Pinner.Square != "e8" {
			t.Fatalf("e2 rook pin = %#v, want pinner e8", piece.AbsolutePin)
		}
		return
	}
	t.Fatal("e2 rook missing from piece trace")
}

func TestPositionTraceKingAttackAccountingMatchesEvaluation(t *testing.T) {
	board := gm.ParseFen("r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1")
	trace := ExplainTraceForBoard(&board, PositionTraceBasic)
	if got, want := trace.Position.Kings.White.AttackUnits, trace.Eval.King.AttackUnitsOnWhiteKing; got != want {
		t.Fatalf("attack units on white king = %d, evaluation trace = %d", got, want)
	}
	if got, want := trace.Position.Kings.Black.AttackUnits, trace.Eval.King.AttackUnitsOnBlackKing; got != want {
		t.Fatalf("attack units on black king = %d, evaluation trace = %d", got, want)
	}
	if got, want := trace.Position.Kings.White.Danger, trace.Eval.King.DangerToWhiteKing; got != want {
		t.Fatalf("danger to white king = %d, evaluation trace = %d", got, want)
	}
	if got, want := trace.Position.Kings.Black.Danger, trace.Eval.King.DangerToBlackKing; got != want {
		t.Fatalf("danger to black king = %d, evaluation trace = %d", got, want)
	}
}

func TestPositionTraceEnPassantCapturedSquare(t *testing.T) {
	board := gm.ParseFen("4k3/8/8/3pP3/8/8/8/4K3 w - d6 0 1")
	trace := PositionTraceForBoard(&board, PositionTraceBasic)
	for _, move := range trace.Choices.Moves {
		if move.Move != "e5d6" {
			continue
		}
		if !move.Capture || move.Captured != "pawn" || move.CapturedSquare != "d5" {
			t.Fatalf("en passant trace = %#v", move)
		}
		return
	}
	t.Fatal("en passant move e5d6 missing")
}

func TestExplainTraceJSON(t *testing.T) {
	board := gm.ParseFen(gm.Startpos)
	trace := ExplainTraceForBoard(&board, PositionTraceBasic)
	var buf bytes.Buffer
	if err := RenderExplainTraceJSON(&buf, trace); err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["level"] != "basic" || decoded["eval"] == nil || decoded["position"] == nil {
		t.Fatalf("unexpected top-level trace: %#v", decoded)
	}
	if got := int(decoded["schemaVersion"].(float64)); got != PositionTraceSchemaVersion {
		t.Fatalf("schema version = %d, want %d", got, PositionTraceSchemaVersion)
	}
}

func parseTraceBitboard(t *testing.T, value string) uint64 {
	t.Helper()
	var parsed uint64
	if _, err := fmt.Sscanf(value, "%x", &parsed); err != nil {
		t.Fatalf("parse bitboard %q: %v", value, err)
	}
	return parsed
}
