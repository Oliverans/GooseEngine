package engine

import (
	"testing"

	gm "chess-engine/goosemg"
)

func TestQuiescenceSearchesQuietQueenPromotion(t *testing.T) {
	b := gm.ParseFen("8/P6k/8/8/8/8/8/7K w - - 0 1")
	initVariables(&b)
	SearchState.ResetForSearch(&b)
	SearchState.ClearStop()
	SearchState.searchShouldStop = false
	SearchState.timeHandler.stopSearch = false

	standpat := Evaluation(&b, false)
	var pv PVLine
	got := quiescence(&b, -MaxScore, MaxScore, &pv, 30, 0, -1)
	if got <= standpat {
		t.Fatalf("qsearch score %d did not improve on stand-pat %d", got, standpat)
	}
	if len(pv.Moves) == 0 || pv.Moves[0].PromotionPieceType() != gm.PieceTypeQueen {
		t.Fatalf("qsearch PV does not start with a queen promotion: %v", pv.Moves)
	}
}

func TestQuiescenceMaxPlyGuard(t *testing.T) {
	b := gm.ParseFen(gm.FENStartPos)
	initVariables(&b)
	SearchState.ResetForSearch(&b)
	SearchState.ClearStop()
	SearchState.searchShouldStop = false
	SearchState.timeHandler.stopSearch = false

	var pv PVLine
	got := quiescence(&b, -MaxScore, MaxScore, &pv, 30, MaxDepth, -1)
	want := Evaluation(&b, false)
	if got != want {
		t.Fatalf("qsearch max-ply score %d, want %d", got, want)
	}
}

func TestQuiescenceDoesNotPruneMateRangeCapture(t *testing.T) {
	b := gm.ParseFen("7k/6p1/5K2/8/8/8/6Q1/8 w - - 0 1")
	initVariables(&b)
	SearchState.ResetForSearch(&b)
	SearchState.ClearStop()
	SearchState.searchShouldStop = false
	SearchState.timeHandler.stopSearch = false

	var pv PVLine
	got := quiescence(&b, Checkmate, MaxScore, &pv, 30, 0, -1)
	if want := MaxScore - 1; got != want {
		t.Fatalf("qsearch mate-range score %d, want %d", got, want)
	}
}
