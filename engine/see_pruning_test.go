package engine

import (
	"testing"

	gm "chess-engine/goosemg"
)

func TestSEEPruningEligibility(t *testing.T) {
	oldMaxDepth := SEEPruneMaxDepth
	SEEPruneMaxDepth = 9
	t.Cleanup(func() { SEEPruneMaxDepth = oldMaxDepth })

	move := gm.NewMove(1, 18, gm.WhiteKnight, gm.NoPiece, gm.NoPiece, gm.FlagNone)
	tests := []struct {
		name       string
		pv         bool
		root       bool
		check      bool
		depth      int8
		legalMoves int
		bestScore  int32
		alpha      int32
		ttMove     gm.Move
		want       bool
	}{
		{"eligible", false, false, false, 3, 1, 0, 0, 0, true},
		{"maximum depth", false, false, false, 9, 1, 0, 0, 0, true},
		{"pv", true, false, false, 3, 1, 0, 0, 0, false},
		{"root", false, true, false, 3, 1, 0, 0, 0, false},
		{"in check", false, false, true, 3, 1, 0, 0, 0, false},
		{"horizon", false, false, false, 0, 1, 0, 0, 0, false},
		{"too deep", false, false, false, 10, 1, 0, 0, 0, false},
		{"first move", false, false, false, 3, 0, 0, 0, 0, false},
		{"mated score", false, false, false, 3, 1, -Checkmate, 0, 0, false},
		{"positive mate window", false, false, false, 3, 1, 0, Checkmate, 0, false},
		{"negative mate window", false, false, false, 3, 1, 0, -Checkmate, 0, false},
		{"tt move", false, false, false, 3, 1, 0, 0, move, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := seePruningEligible(test.pv, test.root, test.check, test.depth, test.legalMoves, test.bestScore, test.alpha, move, test.ttMove)
			if got != test.want {
				t.Fatalf("got %v, want %v", got, test.want)
			}
		})
	}

	SEEPruneMaxDepth = 0
	if seePruningEligible(false, false, false, 1, 1, 0, 0, move, 0) {
		t.Fatal("max depth zero did not disable SEE pruning")
	}
}

func TestSEEPruningMoveKind(t *testing.T) {
	quiet := gm.NewMove(1, 18, gm.WhiteKnight, gm.NoPiece, gm.NoPiece, gm.FlagNone)
	capture := gm.NewMove(1, 18, gm.WhiteKnight, gm.BlackPawn, gm.NoPiece, gm.FlagNone)
	promotion := gm.NewMove(48, 56, gm.WhitePawn, gm.NoPiece, gm.WhiteQueen, gm.FlagNone)
	capturePromotion := gm.NewMove(48, 57, gm.WhitePawn, gm.BlackRook, gm.WhiteQueen, gm.FlagNone)
	castle := gm.NewMove(4, 6, gm.WhiteKing, gm.NoPiece, gm.NoPiece, gm.FlagCastle)
	king := gm.NewMove(4, 12, gm.WhiteKing, gm.NoPiece, gm.NoPiece, gm.FlagNone)
	kingCapture := gm.NewMove(4, 12, gm.WhiteKing, gm.BlackPawn, gm.NoPiece, gm.FlagNone)

	tests := []struct {
		name    string
		move    gm.Move
		capture bool
		quiet   bool
		want    uint8
	}{
		{"quiet", quiet, false, true, seePruningQuiet},
		{"capture", capture, true, false, seePruningNoisy},
		{"promotion", promotion, false, false, seePruningNone},
		{"capture promotion", capturePromotion, true, false, seePruningNone},
		{"castling", castle, false, true, seePruningNone},
		{"quiet king", king, false, true, seePruningNone},
		{"checking quiet", quiet, false, false, seePruningNone},
		{"king capture", kingCapture, true, false, seePruningNoisy},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := seePruningMoveKind(test.move, test.capture, test.quiet); got != test.want {
				t.Fatalf("got %d, want %d", got, test.want)
			}
		})
	}
}

func TestSEEPruningOrderingPriority(t *testing.T) {
	historyRange := int32(historyMaxVal + contHistMax)
	tests := []struct {
		name  string
		score int32
		want  bool
	}{
		{"losing capture", scoreLosingCapture + 505, true},
		{"ordinary quiet ceiling", scoreQuietBase + historyRange, true},
		{"below cutoff", scoreSEEPruningCutoff - 1, true},
		{"at cutoff", scoreSEEPruningCutoff, false},
		{"countermove floor", scoreCounterMove - historyRange, false},
		{"second killer", scoreKiller2, false},
		{"first killer", scoreKiller1, false},
		{"equal capture", scoreEqualCapture, false},
		{"winning capture", scoreWinningCapture, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := seePruningLowPriority(test.score); got != test.want {
				t.Fatalf("got %v, want %v", got, test.want)
			}
		})
	}
}

func TestNoisySEEPruningDecision(t *testing.T) {
	oldScale := SEENoisyScale
	oldDivisor := SEENoisyHistoryDivisor
	SEENoisyScale = 100
	SEENoisyHistoryDivisor = 128
	t.Cleanup(func() {
		SEENoisyScale = oldScale
		SEENoisyHistoryDivisor = oldDivisor
	})

	tests := []struct {
		name        string
		score       int32
		depth       int8
		history     int
		wantPruned  bool
		wantBase    bool
		wantRefined bool
	}{
		{"base prune", -101, 1, 0, true, true, false},
		{"base boundary", -100, 1, 0, false, false, false},
		{"depth scaled", -401, 2, 0, true, true, false},
		{"positive suppresses", -101, 1, 128, false, true, true},
		{"positive unchanged", -102, 1, 128, true, true, true},
		{"negative enables", -100, 1, -128, true, false, true},
		{"negative unchanged", -99, 1, -128, false, false, true},
		{"small positive rounds to zero", -101, 1, 127, true, true, false},
		{"small negative rounds to zero", -100, 1, -127, false, false, false},
		{"positive saturation", -150, 1, captureHistoryMax, false, true, true},
		{"negative saturation", -50, 1, -captureHistoryMax, true, false, true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pruned, basePruned, refined := noisySEEPruningDecision(test.score, test.depth, test.history)
			if pruned != test.wantPruned || basePruned != test.wantBase || refined != test.wantRefined {
				t.Fatalf("got (%v, %v, %v), want (%v, %v, %v)", pruned, basePruned, refined, test.wantPruned, test.wantBase, test.wantRefined)
			}
		})
	}

	SEENoisyHistoryDivisor = 0
	pruned, basePruned, refined := noisySEEPruningDecision(-101, 1, captureHistoryMax)
	if !pruned || !basePruned || refined {
		t.Fatalf("disabled refinement = (%v, %v, %v)", pruned, basePruned, refined)
	}
}

func TestNoisySEEPruningUsesCarriedScoreAndCaptureHistory(t *testing.T) {
	oldHistory := SearchState.captureHistory
	oldScale := SEENoisyScale
	oldDivisor := SEENoisyHistoryDivisor
	t.Cleanup(func() {
		SearchState.captureHistory = oldHistory
		SEENoisyScale = oldScale
		SEENoisyHistoryDivisor = oldDivisor
	})

	captureHistoryClear()
	SEENoisyScale = 100
	SEENoisyHistoryDivisor = 128
	move := gm.NewMove(1, 18, gm.WhiteKnight, gm.BlackPawn, gm.NoPiece, gm.FlagNone)
	captureHistoryUpdate(move, 128)

	pruned, basePruned, refined := noisySEEPruningDecision(-101, 1, captureHistoryScore(move))
	if pruned || !basePruned || !refined {
		t.Fatalf("decision = (%v, %v, %v)", pruned, basePruned, refined)
	}
}
