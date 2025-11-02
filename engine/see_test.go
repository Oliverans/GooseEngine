package engine

import (
	"math/bits"
	"testing"

	gm "chess-engine/goosemg"
)

func TestSEEAccountsForRevealedSlider(t *testing.T) {
	board, err := gm.ParseFEN("6k1/4q1p1/4n3/8/2B5/8/8/6K1 w - - 0 1")
	if err != nil {
		t.Fatalf("parse FEN: %v", err)
	}

	move, err := gm.ParseMove("c4e6")
	if err != nil {
		t.Fatalf("parse move: %v", err)
	}

	score := see(board, move, false)
	if score != 0 {
		t.Fatalf("expected SEE score 0, got %d", score)
	}
}

func TestSEEHandlesEnPassantCapture(t *testing.T) {
	board, err := gm.ParseFEN("8/8/8/3pP3/8/8/8/6K1 w - d6 0 1")
	if err != nil {
		t.Fatalf("parse FEN: %v", err)
	}

	move := gm.NewMove(square("e5"), square("d6"), gm.WhitePawn, gm.BlackPawn, gm.NoPiece, gm.FlagEnPassant)
	if move.Flags()&gm.FlagEnPassant == 0 {
		t.Fatalf("expected en passant flag to be set, got %d", move.Flags())
	}
	if SeePieceValue[gm.PieceTypePawn] != 100 {
		t.Fatalf("unexpected pawn SEE value: %d", SeePieceValue[gm.PieceTypePawn])
	}
	if board.Black.Pawns&PositionBB[int(square("d5"))] == 0 {
		t.Fatalf("expected black pawn at d5, board has %064b (lsb index %d)", board.Black.Pawns, bits.TrailingZeros64(board.Black.Pawns))
	}
	score := see(board, move, false)

	expected := SeePieceValue[gm.PieceTypePawn]
	if score != expected {
		t.Fatalf("expected SEE score %d, got %d", expected, score)
	}
}

func square(coord string) gm.Square {
	if len(coord) != 2 {
		panic("invalid coordinate")
	}
	file := int(coord[0] - 'a')
	rank := int(coord[1] - '1')
	return gm.Square(rank*8 + file)
}

func init() {
	initPositionBB()
}
