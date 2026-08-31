package engine

import (
	"testing"

	gm "chess-engine/goosemg"
)

func prepareAlphaBetaTest(t *testing.T, b *gm.Board, tt TransTable) {
	t.Helper()
	previousTT := SearchState.tt
	t.Cleanup(func() { SearchState.tt = previousTT })
	SearchState.tt = tt
	initVariables(b)
	SearchState.ResetForSearch(b)
	SearchState.ClearStop()
	SearchState.searchShouldStop = false
	SearchState.timeHandler.stopSearch = false
	SearchState.nodesChecked = 0
	ResetCutStats()
}

func TestTTCorrectedPruningEval(t *testing.T) {
	tests := []struct {
		name    string
		raw     int32
		ttScore int32
		flag    int8
		want    int32
	}{
		{"unusable", 100, UnusableScore, ExactFlag, 100},
		{"exact higher", 100, 150, ExactFlag, 150},
		{"exact lower", 100, 50, ExactFlag, 50},
		{"lower raises", 100, 150, BetaFlag, 150},
		{"lower does not lower", 100, 50, BetaFlag, 100},
		{"upper lowers", 100, 50, AlphaFlag, 50},
		{"upper does not raise", 100, 150, AlphaFlag, 100},
		{"PV bit", 100, 150, makeTTFlag(BetaFlag, true), 150},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ttCorrectedPruningEval(test.raw, test.ttScore, test.flag); got != test.want {
				t.Fatalf("ttCorrectedPruningEval() = %d, want %d", got, test.want)
			}
		})
	}
}

func TestTTCorrectionSuppressesRFPAndKeepsRawEval(t *testing.T) {
	previousScale, previousMaxDepth := RFPScale, RFPMaxDepth
	t.Cleanup(func() {
		RFPScale = previousScale
		RFPMaxDepth = previousMaxDepth
	})
	RFPScale = 83
	RFPMaxDepth = 7

	b := gm.ParseFen(gm.FENStartPos)
	tt := newTestTT()
	prepareAlphaBetaTest(t, &b, tt)

	const rawEval int32 = 200
	SearchState.tt.storeEntry(b.Hash(), 1, 0, 0, 10, rawEval, AlphaFlag, false)

	var pv PVLine
	alphabeta(&b, 0, 1, 1, 1, &pv, 0, false, false, 0, false, 0)

	if SearchState.cutStats.RFPEligible != 1 || SearchState.cutStats.RFPRefinements != 1 || SearchState.cutStats.RFPSuppressedByTT != 1 || SearchState.cutStats.RFPEnabledByTT != 0 {
		t.Fatalf("RFP eligible/refinement/suppression/enable = %d/%d/%d/%d, want 1/1/1/0",
			SearchState.cutStats.RFPEligible,
			SearchState.cutStats.RFPRefinements,
			SearchState.cutStats.RFPSuppressedByTT,
			SearchState.cutStats.RFPEnabledByTT)
	}
	if SearchState.evalStack[1] != rawEval {
		t.Fatalf("evalStack[1] = %d, want raw eval %d", SearchState.evalStack[1], rawEval)
	}
	entry, found := SearchState.tt.ProbeEntry(b.Hash())
	if !found || int32(entry.StaticEval) != rawEval {
		t.Fatalf("stored static eval = %d, found %v; want %d", entry.StaticEval, found, rawEval)
	}
}

func TestTTCorrectionEnablesNullMove(t *testing.T) {
	previousMinDepth, previousMarginBase, previousMarginDepth := NullMoveMinDepth, NMMarginBase, NMMarginDepth
	t.Cleanup(func() {
		NullMoveMinDepth = previousMinDepth
		NMMarginBase = previousMarginBase
		NMMarginDepth = previousMarginDepth
	})
	NullMoveMinDepth = 4
	NMMarginBase = 210
	NMMarginDepth = 16

	b := gm.ParseFen(gm.FENStartPos)
	tt := newTestTT()
	prepareAlphaBetaTest(t, &b, tt)

	ttMove := b.GenerateLegalMoves()[0]
	SearchState.tt.storeEntry(b.Hash(), 4, 0, ttMove, 0, -100, BetaFlag, false)

	var pv PVLine
	alphabeta(&b, 99, 100, 4, 1, &pv, 0, false, false, 0, false, 0)

	if SearchState.cutStats.NullMoveGateChecks != 1 || SearchState.cutStats.NullMoveRefinements != 1 {
		t.Fatalf("null gate checks/refinements = %d/%d, want 1/1",
			SearchState.cutStats.NullMoveGateChecks,
			SearchState.cutStats.NullMoveRefinements)
	}
	if SearchState.cutStats.NullMoveEnabledByTT != 1 || SearchState.cutStats.NullMoveSuppressedByTT != 0 || SearchState.cutStats.NullMoveAttempts != 1 {
		t.Fatalf("null enabled/suppressed/attempts = %d/%d/%d, want 1/0/1",
			SearchState.cutStats.NullMoveEnabledByTT,
			SearchState.cutStats.NullMoveSuppressedByTT,
			SearchState.cutStats.NullMoveAttempts)
	}
}
