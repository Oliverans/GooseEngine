package engine

import (
	"testing"

	gm "chess-engine/goosemg"
)

func prepareCaptureHistoryTest(t *testing.T) {
	t.Helper()
	saved := SearchState.captureHistory
	captureHistoryClear()
	t.Cleanup(func() { SearchState.captureHistory = saved })
}

func TestCaptureHistoryKeying(t *testing.T) {
	prepareCaptureHistoryTest(t)

	white := gm.NewMove(8, 17, gm.WhitePawn, gm.BlackPawn, gm.NoPiece, gm.FlagNone)
	black := gm.NewMove(24, 17, gm.BlackPawn, gm.WhitePawn, gm.NoPiece, gm.FlagNone)
	otherTarget := gm.NewMove(8, 16, gm.WhitePawn, gm.BlackPawn, gm.NoPiece, gm.FlagNone)
	otherVictim := gm.NewMove(8, 17, gm.WhitePawn, gm.BlackKnight, gm.NoPiece, gm.FlagNone)

	if !captureHistoryUpdate(white, 400) {
		t.Fatal("capture history update rejected a capture")
	}
	if captureHistoryScore(white) != 400 {
		t.Fatalf("capture history score = %d, want 400", captureHistoryScore(white))
	}
	for _, move := range []gm.Move{black, otherTarget, otherVictim} {
		if captureHistoryScore(move) != 0 {
			t.Fatalf("unrelated capture %s shared history", move)
		}
	}
}

func TestCaptureHistoryEnPassantAndPromotion(t *testing.T) {
	prepareCaptureHistoryTest(t)

	enPassant := gm.NewMove(36, 43, gm.WhitePawn, gm.NoPiece, gm.NoPiece, gm.FlagEnPassant)
	if !captureHistoryUpdate(enPassant, 250) || captureHistoryScore(enPassant) != 250 {
		t.Fatalf("en passant history = %d, want 250", captureHistoryScore(enPassant))
	}

	promotion := gm.NewMove(54, 63, gm.WhitePawn, gm.BlackRook, gm.WhiteQueen, gm.FlagNone)
	if captureHistoryUpdate(promotion, 500) || captureHistoryScore(promotion) != 0 {
		t.Fatal("promotion entered capture history")
	}
}

func TestCaptureHistoryGravityAndLifecycle(t *testing.T) {
	prepareCaptureHistoryTest(t)

	move := gm.NewMove(8, 17, gm.WhitePawn, gm.BlackPawn, gm.NoPiece, gm.FlagNone)
	captureHistoryUpdate(move, 100)
	captureHistoryUpdate(move, 100)
	if captureHistoryScore(move) != 199 {
		t.Fatalf("gravity score = %d, want 199", captureHistoryScore(move))
	}

	captureHistoryAge()
	if captureHistoryScore(move) != 99 {
		t.Fatalf("aged score = %d, want 99", captureHistoryScore(move))
	}

	captureHistoryUpdate(move, captureHistoryMax*2)
	if score := captureHistoryScore(move); score > captureHistoryMax {
		t.Fatalf("positive score exceeded bound: %d", score)
	}
	captureHistoryUpdate(move, -captureHistoryMax*2)
	if score := captureHistoryScore(move); score < -captureHistoryMax {
		t.Fatalf("negative score exceeded bound: %d", score)
	}

	captureHistoryClear()
	if captureHistoryScore(move) != 0 {
		t.Fatal("capture history clear retained an entry")
	}

	captureHistoryUpdate(move, 100)
	ResetForNewGame()
	if captureHistoryScore(move) != 0 {
		t.Fatal("new game reset retained capture history")
	}
}

func TestCaptureHistoryOrderingPreservesSEEStages(t *testing.T) {
	prepareCaptureHistoryTest(t)

	low := gm.NewMove(8, 17, gm.WhitePawn, gm.BlackPawn, gm.NoPiece, gm.FlagNone)
	high := gm.NewMove(9, 18, gm.WhitePawn, gm.BlackPawn, gm.NoPiece, gm.FlagNone)
	captureHistoryUpdate(low, -1000)
	captureHistoryUpdate(high, 1000)

	board := gm.ParseFen(gm.FENStartPos)
	mainScores := make([]move, 2)
	mainList := scoreMovesListInto(&board, []gm.Move{low, high}, 0, 0, 0, 0, mainScores)
	if mainList.moves[1].score <= mainList.moves[0].score {
		t.Fatal("main capture ordering ignored capture history")
	}
	if !isGoodTactical(mainList.moves[0]) || !isGoodTactical(mainList.moves[1]) {
		t.Fatal("negative history moved an equal capture into the bad stage")
	}

	qList, _ := scoreMovesListTacticals([]gm.Move{low, high}, 0, 0)
	if qList.moves[1].score <= qList.moves[0].score {
		t.Fatal("qsearch capture ordering ignored capture history")
	}
	if isGoodTactical(move{score: scoreWinningCapture, seeScore: -1}) {
		t.Fatal("ordering score moved a losing capture into the good stage")
	}
}
