package engine

import (
	"testing"

	gm "chess-engine/goosemg"
)

func prepareQuiescenceTest(t *testing.T, b *gm.Board, tt TransTable) {
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

func TestQuiescenceSearchesQuietQueenPromotion(t *testing.T) {
	b := gm.ParseFen("8/P6k/8/8/8/8/8/7K w - - 0 1")
	prepareQuiescenceTest(t, &b, TransTable{})

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
	prepareQuiescenceTest(t, &b, TransTable{})

	var pv PVLine
	got := quiescence(&b, -MaxScore, MaxScore, &pv, 30, MaxDepth, -1)
	want := Evaluation(&b, false)
	if got != want {
		t.Fatalf("qsearch max-ply score %d, want %d", got, want)
	}
}

func TestQuiescenceDoesNotPruneMateRangeCapture(t *testing.T) {
	b := gm.ParseFen("7k/6p1/5K2/8/8/8/6Q1/8 w - - 0 1")
	prepareQuiescenceTest(t, &b, TransTable{})

	var pv PVLine
	got := quiescence(&b, Checkmate, MaxScore, &pv, 30, 0, -1)
	if want := MaxScore - 1; got != want {
		t.Fatalf("qsearch mate-range score %d, want %d", got, want)
	}
}

func TestQuiescenceTTBoundCutoffs(t *testing.T) {
	tests := []struct {
		name  string
		bound int8
		score int32
		want  int32
	}{
		{"exact", ExactFlag, 50, 50},
		{"alpha", AlphaFlag, -50, 0},
		{"beta", BetaFlag, 50, 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			b := gm.ParseFen("7k/8/8/8/8/8/8/K7 w - - 0 1")
			tt := newTestTT()
			prepareQuiescenceTest(t, &b, tt)
			SearchState.tt.storeEntry(b.Hash(), 0, 0, 0, test.score, 0, test.bound, false)

			var pv PVLine
			got := quiescence(&b, 0, 1, &pv, 30, 0, -1)
			if got != test.want {
				t.Fatalf("qsearch score %d, want %d", got, test.want)
			}
			if SearchState.cutStats.QTTProbes != 1 || SearchState.cutStats.QTTHits != 1 || SearchState.cutStats.QTTCutoffs != 1 {
				t.Fatalf("QTT counters = %d/%d/%d, want 1/1/1", SearchState.cutStats.QTTProbes, SearchState.cutStats.QTTHits, SearchState.cutStats.QTTCutoffs)
			}
		})
	}
}

func TestQuiescenceTTCutoffRespectsPV(t *testing.T) {
	b := gm.ParseFen("8/P6k/8/8/8/8/8/7K w - - 0 1")
	tt := newTestTT()
	prepareQuiescenceTest(t, &b, tt)
	standpat := Evaluation(&b, false)
	SearchState.tt.storeEntry(b.Hash(), 0, 0, 0, standpat, standpat, ExactFlag, false)

	var pv PVLine
	got := quiescence(&b, -MaxScore, MaxScore, &pv, 30, 0, -1)
	if got <= standpat {
		t.Fatalf("PV qsearch score %d did not improve on TT score %d", got, standpat)
	}
	if SearchState.cutStats.QTTCutoffs != 0 {
		t.Fatalf("PV qsearch used %d direct TT cutoffs", SearchState.cutStats.QTTCutoffs)
	}
	if len(pv.Moves) == 0 || pv.Moves[0].PromotionPieceType() != gm.PieceTypeQueen {
		t.Fatalf("PV qsearch did not build the promotion line: %v", pv.Moves)
	}
}

func TestQuiescenceReusesTTStaticEval(t *testing.T) {
	b := gm.ParseFen("7k/8/8/8/8/8/8/K7 w - - 0 1")
	tt := newTestTT()
	prepareQuiescenceTest(t, &b, tt)
	SearchState.tt.storeEntry(b.Hash(), 0, 0, 0, 400, 321, AlphaFlag, false)

	var pv PVLine
	got := quiescence(&b, -1000, 1000, &pv, 30, 0, -1)
	if got != 321 {
		t.Fatalf("qsearch score %d, want stored static eval 321", got)
	}
}

func TestQuiescenceRefinesStandPatAndKeepsRawStaticEval(t *testing.T) {
	tests := []struct {
		name  string
		bound int8
		score int32
		want  int32
	}{
		{"exact", ExactFlag, 70, 70},
		{"better beta", BetaFlag, 150, 150},
		{"worse beta", BetaFlag, 50, 100},
		{"better alpha", AlphaFlag, 50, 50},
		{"worse alpha", AlphaFlag, 150, 100},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			b := gm.ParseFen("7k/8/8/8/8/8/8/K7 w - - 0 1")
			tt := newTestTT()
			prepareQuiescenceTest(t, &b, tt)
			SearchState.tt.storeEntry(b.Hash(), 0, 0, 0, test.score, 100, test.bound, false)

			var pv PVLine
			got := quiescence(&b, -1000, 1000, &pv, 30, 0, -1)
			if got != test.want {
				t.Fatalf("qsearch score %d, want %d", got, test.want)
			}
			entry, found := SearchState.tt.ProbeEntry(b.Hash())
			if !found || int32(entry.StaticEval) != 100 {
				t.Fatalf("stored raw static eval = %d, found %v; want 100, true", entry.StaticEval, found)
			}
			if int32(entry.Score) != test.want || ttBound(entry.Flag) != ExactFlag {
				t.Fatalf("stored qsearch result = score %d bound %d, want %d exact", entry.Score, ttBound(entry.Flag), test.want)
			}
		})
	}
}

func TestQuiescenceStoresBoundsAtDepthZero(t *testing.T) {
	tests := []struct {
		name  string
		alpha int32
		beta  int32
		bound int8
	}{
		{"exact", -100, 100, ExactFlag},
		{"alpha", 100, 101, AlphaFlag},
		{"beta", -1, 0, BetaFlag},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			b := gm.ParseFen("7k/8/8/8/8/8/8/K7 w - - 0 1")
			tt := newTestTT()
			prepareQuiescenceTest(t, &b, tt)
			rawStaticEval := Evaluation(&b, false)

			var pv PVLine
			got := quiescence(&b, test.alpha, test.beta, &pv, 30, 0, -1)
			entry, found := SearchState.tt.ProbeEntry(b.Hash())
			if !found {
				t.Fatal("qsearch result was not stored")
			}
			if entry.Depth != 0 || int32(entry.Score) != got || int32(entry.StaticEval) != rawStaticEval || ttBound(entry.Flag) != test.bound || entry.Move != 0 {
				t.Fatalf("stored entry = %+v, score %d static %d bound %d", *entry, got, rawStaticEval, test.bound)
			}
		})
	}
}

func TestQuiescenceStoresCutoffMove(t *testing.T) {
	b := gm.ParseFen("8/P6k/8/8/8/8/8/7K w - - 0 1")
	tt := newTestTT()
	prepareQuiescenceTest(t, &b, tt)
	rawStaticEval := Evaluation(&b, false)

	var pv PVLine
	got := quiescence(&b, rawStaticEval, rawStaticEval+1, &pv, 30, 0, -1)
	entry, found := SearchState.tt.ProbeEntry(b.Hash())
	if !found || got < rawStaticEval+1 || ttBound(entry.Flag) != BetaFlag || entry.Move.PromotionPieceType() != gm.PieceTypeQueen {
		t.Fatalf("stored cutoff = found %v score %d entry %+v", found, got, *entry)
	}
}

func TestQuiescenceFailLowStoresNoMove(t *testing.T) {
	b := gm.ParseFen("8/P6k/8/8/8/8/8/7K w - - 0 1")
	tt := newTestTT()
	prepareQuiescenceTest(t, &b, tt)

	var pv PVLine
	got := quiescence(&b, Checkmate, MaxScore, &pv, 30, 0, -1)
	entry, found := SearchState.tt.ProbeEntry(b.Hash())
	if !found || got >= Checkmate || ttBound(entry.Flag) != AlphaFlag || entry.Move != 0 || SearchState.cutStats.QNodes < 2 {
		t.Fatalf("stored fail-low = found %v score %d qnodes %d entry %+v", found, got, SearchState.cutStats.QNodes, *entry)
	}
}

func TestQuiescenceStoresNoStaticEvalInCheck(t *testing.T) {
	b := gm.ParseFen("7k/6Q1/6K1/8/8/8/8/8 b - - 0 1")
	tt := newTestTT()
	prepareQuiescenceTest(t, &b, tt)

	var pv PVLine
	quiescence(&b, -MaxScore, MaxScore, &pv, 30, 0, -1)
	entry, found := SearchState.tt.ProbeEntry(b.Hash())
	if !found || int32(entry.StaticEval) != -MaxScore {
		t.Fatalf("in-check static eval = %d, found %v; want %d, true", entry.StaticEval, found, -MaxScore)
	}
}

func TestQuiescenceOrdersTTTacticalFirst(t *testing.T) {
	b := gm.ParseFen("4k3/8/8/3p1p2/4Q3/8/8/4K3 w - - 0 1")
	prepareQuiescenceTest(t, &b, TransTable{})
	moves := b.GenerateTacticalsInto(make([]gm.Move, 0, 32))
	if len(moves) < 2 {
		t.Fatalf("generated %d tacticals, want at least 2", len(moves))
	}
	ttMove := moves[len(moves)-1]
	moveList, _ := scoreMovesListTacticals(moves, 0, ttMove)
	orderNextMove(0, &moveList)
	if moveList.moves[0].move != ttMove {
		t.Fatalf("first qsearch move %s, want TT move %s", moveList.moves[0].move, ttMove)
	}
}
