package engine

import (
	"testing"

	gm "chess-engine/goosemg"
)

func TestCaptureFutilityEligibility(t *testing.T) {
	oldMaxDepth := CaptureFutilityMaxDepth
	CaptureFutilityMaxDepth = 4
	t.Cleanup(func() { CaptureFutilityMaxDepth = oldMaxDepth })

	tests := []struct {
		name       string
		pv         bool
		root       bool
		inCheck    bool
		depth      int8
		legalMoves int
		bestScore  int32
		alpha      int32
		capture    bool
		promotion  bool
		want       bool
	}{
		{"eligible", false, false, false, 2, 1, 0, 0, true, false, true},
		{"maximum depth", false, false, false, 4, 1, 0, 0, true, false, true},
		{"pv", true, false, false, 2, 1, 0, 0, true, false, false},
		{"root", false, true, false, 2, 1, 0, 0, true, false, false},
		{"in check", false, false, true, 2, 1, 0, 0, true, false, false},
		{"horizon", false, false, false, 0, 1, 0, 0, true, false, false},
		{"too deep", false, false, false, 5, 1, 0, 0, true, false, false},
		{"first move", false, false, false, 2, 0, 0, 0, true, false, false},
		{"mated score", false, false, false, 2, 1, -Checkmate, 0, true, false, false},
		{"positive mate window", false, false, false, 2, 1, 0, Checkmate, true, false, false},
		{"negative mate window", false, false, false, 2, 1, 0, -Checkmate, true, false, false},
		{"quiet", false, false, false, 2, 1, 0, 0, false, false, false},
		{"promotion", false, false, false, 2, 1, 0, 0, true, true, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := captureFutilityEligible(test.pv, test.root, test.inCheck, test.depth, test.legalMoves, test.bestScore, test.alpha, test.capture, test.promotion)
			if got != test.want {
				t.Fatalf("got %v, want %v", got, test.want)
			}
		})
	}

	CaptureFutilityMaxDepth = 0
	if captureFutilityEligible(false, false, false, 1, 1, 0, 0, true, false) {
		t.Fatal("max depth zero did not disable capture futility")
	}
}

func TestCaptureFutilityVictimValue(t *testing.T) {
	pawn := gm.NewMove(0, 8, gm.WhiteRook, gm.BlackPawn, gm.NoPiece, gm.FlagNone)
	queen := gm.NewMove(0, 8, gm.WhiteRook, gm.BlackQueen, gm.NoPiece, gm.FlagNone)
	enPassant := gm.NewMove(36, 43, gm.WhitePawn, gm.NoPiece, gm.NoPiece, gm.FlagEnPassant)
	quiet := gm.NewMove(0, 8, gm.WhiteRook, gm.NoPiece, gm.NoPiece, gm.FlagNone)

	tests := []struct {
		name string
		move gm.Move
		want int32
	}{
		{"pawn", pawn, int32(SeePieceValue[gm.PieceTypePawn])},
		{"queen", queen, int32(SeePieceValue[gm.PieceTypeQueen])},
		{"en passant", enPassant, int32(SeePieceValue[gm.PieceTypePawn])},
		{"quiet", quiet, 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := captureFutilityVictimValue(test.move); got != test.want {
				t.Fatalf("got %d, want %d", got, test.want)
			}
		})
	}
}

func TestCaptureFutilityDecision(t *testing.T) {
	oldBase := CaptureFutilityBase
	oldScale := CaptureFutilityScale
	oldDivisor := CaptureFutilityHistoryDivisor
	oldQuietBase := FutilityBase
	oldQuietScale := FutilityScale
	CaptureFutilityBase = 100
	CaptureFutilityScale = 100
	CaptureFutilityHistoryDivisor = 100
	FutilityBase = 400
	FutilityScale = 400
	t.Cleanup(func() {
		CaptureFutilityBase = oldBase
		CaptureFutilityScale = oldScale
		CaptureFutilityHistoryDivisor = oldDivisor
		FutilityBase = oldQuietBase
		FutilityScale = oldQuietScale
	})

	tests := []struct {
		name        string
		rawEval     int32
		alpha       int32
		depth       int8
		victim      int32
		history     int
		wantPruned  bool
		wantBase    bool
		wantRefined bool
	}{
		{"base boundary", 0, 300, 1, 100, 0, true, true, false},
		{"base survives", 0, 299, 1, 100, 0, false, false, false},
		{"depth margin", -100, 300, 2, 100, 0, true, true, false},
		{"positive suppresses", 0, 300, 1, 100, 100, false, true, true},
		{"positive unchanged", -1, 300, 1, 100, 100, true, true, true},
		{"negative enables", 0, 299, 1, 100, -100, true, false, true},
		{"negative unchanged", 1, 299, 1, 100, -100, false, false, true},
		{"small positive rounds to zero", 0, 300, 1, 100, 99, true, true, false},
		{"small negative rounds to zero", 0, 299, 1, 100, -99, false, false, false},
		{"positive saturation", -50, 300, 1, 100, captureHistoryMax * 2, false, true, true},
		{"negative saturation", 50, 299, 1, 100, -captureHistoryMax * 2, true, false, true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pruned, basePruned, refined := captureFutilityDecision(test.rawEval, test.alpha, test.depth, test.victim, test.history)
			if pruned != test.wantPruned || basePruned != test.wantBase || refined != test.wantRefined {
				t.Fatalf("got (%v, %v, %v), want (%v, %v, %v)", pruned, basePruned, refined, test.wantPruned, test.wantBase, test.wantRefined)
			}
		})
	}

	CaptureFutilityHistoryDivisor = 0
	pruned, basePruned, refined := captureFutilityDecision(0, 300, 1, 100, captureHistoryMax)
	if !pruned || !basePruned || refined {
		t.Fatalf("disabled refinement = (%v, %v, %v)", pruned, basePruned, refined)
	}
}

func TestCaptureFutilityCheckingCaptureExemption(t *testing.T) {
	board := gm.ParseFen("4k3/8/8/8/8/8/4r3/4R1K1 w - - 0 1")
	var capture gm.Move
	for _, move := range board.GenerateLegalMoves() {
		if move.String() == "e1e2" {
			capture = move
			break
		}
	}
	if capture == 0 {
		t.Fatal("checking capture not generated")
	}
	if !board.GivesCheck(capture) {
		t.Fatal("capture should give check")
	}
	if captureFutilityEligible(false, false, false, 1, 1, 0, 0, true, false) && !board.GivesCheck(capture) {
		t.Fatal("checking capture passed the complete gate")
	}
}
