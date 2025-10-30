package goose_engine_mg_test

import (
    myengine "github.com/Oliverans/GooseEngineMG/goosemg"
    "testing"
)

func TestCheckmate_FoolsMate(t *testing.T) {
	// Fool's mate: Black just played Qh4#, White to move and is checkmated
	fen := "rnb1kbnr/pppp1ppp/8/4p3/6Pq/5P2/PPPPP2P/RNBQKBNR w KQkq - 1 3"
	b, err := myengine.ParseFEN(fen)
	if err != nil {
		t.Fatalf("ParseFEN failed: %v", err)
	}
	if !b.InCheck(myengine.White) {
		t.Fatalf("expected White to be in check")
	}
	if b.HasLegalMoves() {
		t.Fatalf("expected no legal moves for White in mate")
	}
	if !b.InCheckmate() {
		t.Fatalf("expected checkmate for White")
	}
	if b.InStalemate() {
		t.Fatalf("not stalemate in mate position")
	}
}

func TestStalemate_Basic(t *testing.T) {
	// Classic stalemate: Black to move with no legal moves and not in check
	fen := "7k/5Q2/6K1/8/8/8/8/8 b - - 0 1"
	b, err := myengine.ParseFEN(fen)
	if err != nil {
		t.Fatalf("ParseFEN failed: %v", err)
	}
	if b.InCheck(myengine.Black) {
		t.Fatalf("expected Black not in check")
	}
	if b.HasLegalMoves() {
		t.Fatalf("expected no legal moves for Black in stalemate")
	}
	if !b.InStalemate() {
		t.Fatalf("expected stalemate for Black")
	}
}

// Mate-in-one: make the mating move and verify the updated board detects checkmate
func TestMateInOne_MakeAndDetect(t *testing.T) {
	// White to move: Qxg7# with bishop on c3 protecting g7, black king on h8
	fen := "7k/6pp/6Q1/8/8/2B5/8/6K1 w - - 0 1"
	b, err := myengine.ParseFEN(fen)
	if err != nil {
		t.Fatalf("ParseFEN failed: %v", err)
	}

	// Find Qg6xg7
	from := myengine.Square(5*8 + 6) // g6
	to := myengine.Square(6*8 + 6)   // g7
	var move myengine.Move
	found := false
	for _, m := range b.GenerateMoves() {
		if m.From() == from && m.To() == to && m.CapturedPiece() == myengine.BlackPawn {
			move = m
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected to find Qxg7# move in legal moves")
	}

	ok, st := b.MakeMove(move)
	if !ok {
		t.Fatalf("MakeMove for Qxg7 should be legal")
	}
	defer b.UnmakeMove(move, st)

	if !b.InCheckmate() {
		t.Fatalf("expected checkmate after Qxg7#")
	}
	if b.InStalemate() {
		t.Fatalf("not stalemate after mate")
	}
}

func TestPerftDivide_InitialDepth2(t *testing.T) {
	b, err := myengine.ParseFEN(myengine.FENStartPos)
	if err != nil {
		t.Fatalf("ParseFEN failed: %v", err)
	}
	div := myengine.PerftDivide(b, 2)
	if len(div) != 20 {
		t.Fatalf("divide length: got %d want %d", len(div), 20)
	}
	var sum uint64
	var min, max uint64
	first := true
	for _, v := range div {
		sum += v
		if first {
			min, max = v, v
			first = false
		} else {
			if v < min {
				min = v
			}
			if v > max {
				max = v
			}
		}
	}
	if sum != 400 {
		t.Fatalf("divide sum: got %d want %d", sum, 400)
	}
	if min != 20 || max != 20 {
		t.Fatalf("expected all child counts to be 20, got min=%d max=%d", min, max)
	}
}
