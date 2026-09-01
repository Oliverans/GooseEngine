package engine

import (
	gm "chess-engine/goosemg"
	"testing"
)

func TestMovePickerTTIsGenerationFree(t *testing.T) {
	ResetForNewGame()
	ResetCutStats()
	b, err := gm.ParseFEN(gm.FENStartPos)
	if err != nil {
		t.Fatal(err)
	}

	var ttMove gm.Move
	for _, move := range b.GenerateLegalMoves() {
		if move.String() == "e2e4" {
			ttMove = move
			break
		}
	}
	if ttMove == 0 {
		t.Fatal("missing e2e4")
	}

	picker := newMovePicker(b, 5, 0, ttMove, 0)
	picked, index, ok := picker.Next()
	move := picked.move
	if !ok || move != ttMove || index != 0 {
		t.Fatalf("got move=%s index=%d ok=%v", move.String(), index, ok)
	}
	if SearchState.cutStats.MovesGenerated != 0 {
		t.Fatalf("generated %d moves before TT", SearchState.cutStats.MovesGenerated)
	}
}

func TestMovePickerGeneratesTacticalsBeforeQuiets(t *testing.T) {
	ResetForNewGame()
	ResetCutStats()
	b, err := gm.ParseFEN("1n5k/P7/8/8/8/8/8/7K w - - 0 1")
	if err != nil {
		t.Fatal(err)
	}

	picker := newMovePicker(b, 5, 0, 0, 0)
	picked, index, ok := picker.Next()
	move := picked.move
	if !ok || move.PromotionPieceType() != gm.PieceTypeQueen || index != 0 {
		t.Fatalf("got move=%s index=%d ok=%v", move.String(), index, ok)
	}
	if SearchState.cutStats.MovesGenerated != 5 {
		t.Fatalf("generated %d moves before quiets", SearchState.cutStats.MovesGenerated)
	}
}

func TestMovePickerMatchesLegalMoves(t *testing.T) {
	ResetForNewGame()
	ResetCutStats()
	b, err := gm.ParseFEN("1n5k/P7/8/8/8/8/8/7K w - - 0 1")
	if err != nil {
		t.Fatal(err)
	}

	want := make(map[gm.Move]bool)
	for _, move := range b.GenerateLegalMoves() {
		want[move] = true
	}

	picker := newMovePicker(b, 5, 0, 0, 0)
	got := make(map[gm.Move]bool)
	underPromotionSeen := false
	for expectedIndex := uint8(0); ; expectedIndex++ {
		picked, index, ok := picker.Next()
		if !ok {
			break
		}
		move := picked.move
		if index != expectedIndex {
			t.Fatalf("move %s: got index %d want %d", move.String(), index, expectedIndex)
		}
		if got[move] {
			t.Fatalf("duplicate move %s", move.String())
		}
		got[move] = true

		promotion := move.PromotionPieceType()
		if promotion != gm.PieceTypeNone && promotion != gm.PieceTypeQueen {
			underPromotionSeen = true
		} else if underPromotionSeen {
			t.Fatalf("move %s appeared after an underpromotion", move.String())
		}
	}

	if len(got) != len(want) {
		t.Fatalf("got %d moves want %d", len(got), len(want))
	}
	for move := range want {
		if !got[move] {
			t.Fatalf("missing move %s", move.String())
		}
	}
}

func TestMovePickerCarriesNegativeCaptureSEE(t *testing.T) {
	ResetForNewGame()
	InitPositionBB()
	b := gm.ParseFen("4k3/8/2p5/3p4/8/8/8/3QK3 w - - 0 1")

	picker := newMovePicker(&b, 5, 0, 0, 0)
	for {
		picked, _, ok := picker.Next()
		if !ok {
			t.Fatal("missing d1d5")
		}
		if picked.move.String() == "d1d5" {
			if picked.seeScore != -800 {
				t.Fatalf("SEE = %d, want -800", picked.seeScore)
			}
			return
		}
	}
}

func TestMovePickerCarriesPriorityScores(t *testing.T) {
	ResetForNewGame()
	b := gm.ParseFen(gm.FENStartPos)
	killer := seeTestMove(t, &b, "g1f3")
	counter := seeTestMove(t, &b, "b1c3")
	prevMove := gm.NewMove(52, 36, gm.BlackPawn, gm.NoPiece, gm.NoPiece, gm.FlagNone)
	SearchState.killer.KillerMoves[0][0] = killer
	SearchState.counterMoves[0][prevMove.From()][prevMove.To()] = counter

	found := 0
	picker := newMovePicker(&b, 1, 0, 0, prevMove)
	for {
		picked, _, ok := picker.Next()
		if !ok {
			break
		}
		switch picked.move {
		case killer:
			if picked.score != scoreKiller1 || seePruningLowPriority(picked.score) {
				t.Fatalf("killer score = %d", picked.score)
			}
			found++
		case counter:
			if picked.score != scoreCounterMove || seePruningLowPriority(picked.score) {
				t.Fatalf("counter score = %d", picked.score)
			}
			found++
		}
	}

	if found != 2 {
		t.Fatalf("found %d priority moves, want 2", found)
	}
}

func TestMovePickerCarriesHistoryScore(t *testing.T) {
	ResetForNewGame()
	b := gm.ParseFen(gm.FENStartPos)
	quiet := seeTestMove(t, &b, "e2e4")
	killer := seeTestMove(t, &b, "g1f3")
	prevMove := gm.NewMove(57, 42, gm.BlackKnight, gm.NoPiece, gm.NoPiece, gm.FlagNone)
	prevEntry := ContHistEntryFromMove(prevMove)

	SearchState.historyMoves[0][quiet.From()][quiet.To()] = 700
	SearchState.contHist1Ply[0][prevEntry.Piece][prevEntry.To][quiet.MovedPiece().Type()-1][quiet.To()] = 400
	SearchState.historyMoves[0][killer.From()][killer.To()] = -600
	SearchState.moveStack[1] = prevMove
	SearchState.killer.KillerMoves[0][0] = killer

	found := 0
	picker := newMovePicker(&b, 1, 0, 0, prevMove)
	for {
		picked, _, ok := picker.Next()
		if !ok {
			break
		}
		switch picked.move {
		case quiet:
			if picked.historyScore != 900 {
				t.Fatalf("quiet history = %d, want 900", picked.historyScore)
			}
			found++
		case killer:
			if picked.score != scoreKiller1 || picked.historyScore != -600 {
				t.Fatalf("killer score/history = %d/%d", picked.score, picked.historyScore)
			}
			found++
		}
	}

	if found != 2 {
		t.Fatalf("found %d moves, want 2", found)
	}
}

func TestMovePickerSkipQuietsKeepsBadCaptures(t *testing.T) {
	ResetForNewGame()
	InitPositionBB()
	b := gm.ParseFen("4k3/8/2p5/3p4/8/8/8/3QK3 w - - 0 1")
	picker := newMovePicker(&b, 3, 0, 0, 0)

	for {
		picked, _, ok := picker.Next()
		if !ok {
			t.Fatal("missing quiet move")
		}
		if !gm.IsCapture(picked.move, &b) && picked.move.PromotionPieceType() == gm.PieceTypeNone {
			if skipped := picker.SkipQuiets(); skipped == 0 {
				t.Fatal("no remaining quiets skipped")
			}
			break
		}
	}

	picked, _, ok := picker.Next()
	if !ok || picked.move.String() != "d1d5" {
		t.Fatalf("got move=%s ok=%v, want deferred d1d5", picked.move.String(), ok)
	}
}

func TestMovePickerSkipQuietsKeepsUnderpromotions(t *testing.T) {
	ResetForNewGame()
	b := gm.ParseFen("1n5k/P7/8/8/8/8/8/7K w - - 0 1")
	picker := newMovePicker(&b, 3, 0, 0, 0)
	picker.SkipQuiets()

	underpromotions := 0
	for {
		picked, _, ok := picker.Next()
		if !ok {
			break
		}
		promotion := picked.move.PromotionPieceType()
		if promotion == gm.PieceTypeNone {
			t.Fatalf("ordinary quiet %s was not skipped", picked.move.String())
		}
		if promotion != gm.PieceTypeQueen {
			underpromotions++
		}
	}

	if underpromotions == 0 {
		t.Fatal("underpromotions were skipped")
	}
}

func TestStagedPickerTerminalScores(t *testing.T) {
	tests := []struct {
		fen  string
		want int32
	}{
		{"7k/6Q1/6K1/8/8/8/8/8 b - - 0 1", -MaxScore},
		{"7k/5Q2/6K1/8/8/8/8/8 b - - 0 1", DrawScore},
	}

	for _, test := range tests {
		ResetForNewGame()
		b, err := gm.ParseFEN(test.fen)
		if err != nil {
			t.Fatal(err)
		}

		var pv PVLine
		got := alphabeta(b, -MaxScore, MaxScore, 1, 0, &pv, 0, false, false, 0, false, -1)
		if got != test.want {
			t.Fatalf("%s: got %d want %d", test.fen, got, test.want)
		}
	}
}
