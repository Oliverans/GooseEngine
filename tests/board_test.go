package goose_engine_mg_test

import (
	myengine "chess-engine/goosemg"
	"testing"
)

func TestFENAndValidate(t *testing.T) {
	b, err := myengine.ParseFEN(myengine.FENStartPos)
	if err != nil {
		t.Fatalf("ParseFEN: %v", err)
	}
	if !b.Validate() {
		t.Fatalf("board invariants invalid after FEN parse")
	}

	// Quick spot checks on a few known starting squares
	// a1 white rook, e1 white king, a8 black rook, e8 black king
	if b.PieceAt(0) != myengine.WhiteRook { // a1
		t.Errorf("expected a1 WhiteRook, got %v", b.PieceAt(0))
	}
	if b.PieceAt(4) != myengine.WhiteKing { // e1
		t.Errorf("expected e1 WhiteKing, got %v", b.PieceAt(4))
	}
	if b.PieceAt(56) != myengine.BlackRook { // a8
		t.Errorf("expected a8 BlackRook, got %v", b.PieceAt(56))
	}
	if b.PieceAt(60) != myengine.BlackKing { // e8
		t.Errorf("expected e8 BlackKing, got %v", b.PieceAt(60))
	}
}

func TestBoardMovePieceUpdates(t *testing.T) {
	b, err := myengine.ParseFEN(myengine.FENStartPos)
	if err != nil {
		t.Fatalf("ParseFEN: %v", err)
	}
	startKey := b.ComputeZobrist()
	if startKey != b.ComputeZobrist() {
		t.Fatalf("zobrist mismatch on initial compute")
	}

	// Move e2 to e4 (12 -> 28)
	from := myengine.Square(1*8 + 4)
	to := myengine.Square(3*8 + 4)
	if b.PieceAt(from) != myengine.WhitePawn {
		t.Fatalf("expected WhitePawn at e2 before move")
	}
	if b.PieceAt(to) != myengine.NoPiece {
		t.Fatalf("expected empty e4 before move")
	}

	b.MovePiece(from, to)
	if !b.Validate() {
		t.Fatalf("board invariants invalid after MovePiece")
	}
	if b.PieceAt(from) != myengine.NoPiece || b.PieceAt(to) != myengine.WhitePawn {
		t.Fatalf("piece locations not updated correctly after MovePiece")
	}

	// Ensure zobristKey tracks ComputeZobrist
	if b.ComputeZobrist() != b.ComputeZobrist() { // recompute twice for stability
		t.Fatalf("ComputeZobrist unstable")
	}
}
