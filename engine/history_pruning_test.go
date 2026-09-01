package engine

import "testing"

func TestHistoryPruningEligibility(t *testing.T) {
	oldMaxDepth := HistPruneMaxDepth
	HistPruneMaxDepth = 4
	t.Cleanup(func() { HistPruneMaxDepth = oldMaxDepth })

	tests := []struct {
		name          string
		pv            bool
		root          bool
		check         bool
		depth         int8
		legalMoves    int
		bestScore     int32
		quiet         bool
		orderingScore int32
		want          bool
	}{
		{"eligible", false, false, false, 2, 1, 0, true, scoreQuietBase, true},
		{"maximum depth", false, false, false, 4, 1, 0, true, scoreQuietBase, true},
		{"pv", true, false, false, 2, 1, 0, true, scoreQuietBase, false},
		{"root", false, true, false, 2, 1, 0, true, scoreQuietBase, false},
		{"in check", false, false, true, 2, 1, 0, true, scoreQuietBase, false},
		{"horizon", false, false, false, 0, 1, 0, true, scoreQuietBase, false},
		{"too deep", false, false, false, 5, 1, 0, true, scoreQuietBase, false},
		{"first move", false, false, false, 2, 0, 0, true, scoreQuietBase, false},
		{"mated score", false, false, false, 2, 1, -Checkmate, true, scoreQuietBase, false},
		{"tactical", false, false, false, 2, 1, 0, false, scoreQuietBase, false},
		{"priority move", false, false, false, 2, 1, 0, true, scoreQuietPriorityCutoff, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := historyPruningEligible(test.pv, test.root, test.check, test.depth, test.legalMoves, test.bestScore, test.quiet, test.orderingScore)
			if got != test.want {
				t.Fatalf("got %v, want %v", got, test.want)
			}
		})
	}

	HistPruneMaxDepth = 0
	if historyPruningEligible(false, false, false, 1, 1, 0, true, scoreQuietBase) {
		t.Fatal("max depth zero did not disable history pruning")
	}
}

func TestHistoryPruningDecision(t *testing.T) {
	oldMargin := HistPruneMargin
	HistPruneMargin = -1500
	t.Cleanup(func() { HistPruneMargin = oldMargin })

	tests := []struct {
		name   string
		score  int32
		depth  int8
		pruned bool
	}{
		{"depth one prune", -1, 1, true},
		{"depth one boundary", 0, 1, false},
		{"depth two prune", -1501, 2, true},
		{"depth two boundary", -1500, 2, false},
		{"depth four prune", -4501, 4, true},
		{"depth four boundary", -4500, 4, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := historyPruningDecision(test.score, test.depth); got != test.pruned {
				t.Fatalf("got %v, want %v", got, test.pruned)
			}
		})
	}
}
