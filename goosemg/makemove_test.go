package goosemg

import "testing"

func TestUnmakeNullMoveRestoresTurnView(t *testing.T) {
	board := ParseFen(Startpos)
	wantHash := board.Hash()

	state := board.MakeNullMove()
	board.UnmakeNullMove(state)

	if !board.Wtomove || board.SideToMove() != White {
		t.Fatalf("restored turn = (%v, %v), want white", board.Wtomove, board.SideToMove())
	}
	if board.Hash() != wantHash {
		t.Fatalf("restored hash = %x, want %x", board.Hash(), wantHash)
	}
}
