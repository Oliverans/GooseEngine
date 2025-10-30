package goose_engine_mg_test

import (
	"testing"

	myengine "github.com/Oliverans/GooseEngineMG/goosemg"
)

// helper: parse empty board
func emptyBoard(t *testing.T) *myengine.Board {
	t.Helper()
	b, err := myengine.ParseFEN("8/8/8/8/8/8/8/8 w - - 0 1")
	if err != nil {
		t.Fatalf("ParseFEN empty: %v", err)
	}
	return b
}

func TestIsSquareAttacked_RookFiles(t *testing.T) {
	b := emptyBoard(t)
	// e1 white king, e8 black rook
	e1 := myengine.Square(0*8 + 4)
	e8 := myengine.Square(7*8 + 4)
	b.SetPiece(e1, myengine.WhiteKing)
	b.SetPiece(e8, myengine.BlackRook)
	if !b.InCheck(myengine.White) {
		t.Fatalf("expected White in check from rook on file")
	}
	if !b.IsSquareAttacked(e1, myengine.Black) {
		t.Fatalf("expected e1 attacked by Black")
	}
	// Add a blocker at e3 (white pawn)
	e3 := myengine.Square(2*8 + 4)
	b.SetPiece(e3, myengine.WhitePawn)
	if b.IsSquareAttacked(e1, myengine.Black) {
		t.Fatalf("did not expect e1 attacked after blocker added")
	}
}

func TestIsSquareAttacked_BishopDiagonals(t *testing.T) {
	b := emptyBoard(t)
	// e1 white king, b4 black bishop (b4 -> c3 -> d2 -> e1)
	e1 := myengine.Square(0*8 + 4)
	b4 := myengine.Square(3*8 + 1)
	b.SetPiece(e1, myengine.WhiteKing)
	b.SetPiece(b4, myengine.BlackBishop)
	if !b.IsSquareAttacked(e1, myengine.Black) || !b.InCheck(myengine.White) {
		t.Fatalf("expected e1 attacked by bishop along diagonal")
	}
	// Block at d2
	d2 := myengine.Square(1*8 + 3)
	b.SetPiece(d2, myengine.WhitePawn)
	if b.IsSquareAttacked(e1, myengine.Black) {
		t.Fatalf("did not expect e1 attacked after diagonal blocker")
	}
}

func TestIsSquareAttacked_PawnsKnightsKings(t *testing.T) {
	b := emptyBoard(t)
	// e4 white pawn, d5 black pawn attacks e4; f3 black knight attacks e1; d2 black king attacks e1
	e1 := myengine.Square(0*8 + 4)
	e4 := myengine.Square(3*8 + 4)
	d5 := myengine.Square(4*8 + 3)
	f3 := myengine.Square(2*8 + 5)
	d2 := myengine.Square(1*8 + 3)

	b.SetPiece(e1, myengine.WhiteKing)
	b.SetPiece(e4, myengine.WhitePawn)
	b.SetPiece(d5, myengine.BlackPawn)
	if !b.IsSquareAttacked(e4, myengine.Black) {
		t.Fatalf("expected e4 attacked by black pawn from d5")
	}
	b.SetPiece(f3, myengine.BlackKnight)
	if !b.IsSquareAttacked(e1, myengine.Black) {
		t.Fatalf("expected e1 attacked by black knight from f3")
	}
	b.SetPiece(d2, myengine.BlackKing)
	if !b.IsSquareAttacked(e1, myengine.Black) {
		t.Fatalf("expected e1 attacked by adjacent black king")
	}
}
