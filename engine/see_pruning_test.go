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

func TestNoisySEEPruningThreshold(t *testing.T) {
	oldScale := SEENoisyScale
	SEENoisyScale = 100
	t.Cleanup(func() { SEENoisyScale = oldScale })

	tests := []struct {
		score int32
		depth int8
		want  bool
	}{
		{-101, 1, true},
		{-100, 1, false},
		{-99, 1, false},
		{-401, 2, true},
		{-400, 2, false},
	}

	for _, test := range tests {
		if got := noisySEEPruned(test.score, test.depth); got != test.want {
			t.Errorf("score %d depth %d: got %v, want %v", test.score, test.depth, got, test.want)
		}
	}
}
