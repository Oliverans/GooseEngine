package goose_engine_mg_test

import (
    "testing"
    // Replace "github.com/Oliverans/GooseEngineMG/goosemg" with the module path of the engine package when integrating.
    myengine "github.com/Oliverans/GooseEngineMG/goosemg"
)

func TestPerftInitialPosition(t *testing.T) {
	board, err := myengine.ParseFEN(myengine.FENStartPos)
	if err != nil {
		t.Fatalf("ParseFEN failed for initial position: %v", err)
	}
	if got := myengine.Perft(board, 1); got != 20 {
		t.Fatalf("perft depth1: got %d want %d", got, 20)
	}
	if got := myengine.Perft(board, 2); got != 400 {
		t.Fatalf("perft depth2: got %d want %d", got, 400)
	}
}

func TestPerftKiwipete(t *testing.T) {
	// Canonical Kiwipete position
	fen := "r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1"
	board, err := myengine.ParseFEN(fen)
	if err != nil {
		t.Fatalf("ParseFEN failed for Kiwipete position: %v", err)
	}
	if got := myengine.Perft(board, 1); got != 48 {
		// Diagnostics: list generated moves and counts by piece
		pseudo := board.GeneratePseudoMoves()
		t.Logf("diagnostic: pseudo moves=%d", len(pseudo))
		moves := board.GenerateMoves()
		t.Logf("diagnostic: len(moves)=%d", len(moves))
		var cp, cn, cb, cr, cq, ck int
		var cap, ep, cas, promo int
		for _, m := range moves {
			p := m.MovedPiece()
			switch p {
			case myengine.WhitePawn, myengine.BlackPawn:
				cp++
			case myengine.WhiteKnight, myengine.BlackKnight:
				cn++
			case myengine.WhiteBishop, myengine.BlackBishop:
				cb++
			case myengine.WhiteRook, myengine.BlackRook:
				cr++
			case myengine.WhiteQueen, myengine.BlackQueen:
				cq++
			case myengine.WhiteKing, myengine.BlackKing:
				ck++
			}
			if m.CapturedPiece() != myengine.NoPiece {
				cap++
			}
			if m.Flags() == myengine.FlagEnPassant {
				ep++
			}
			if m.Flags() == myengine.FlagCastle {
				cas++
			}
			if m.PromotionPiece() != myengine.NoPiece {
				promo++
			}
		}
		t.Logf("by piece: P=%d N=%d B=%d R=%d Q=%d K=%d", cp, cn, cb, cr, cq, ck)
		t.Logf("special: captures=%d ep=%d castles=%d promotions=%d", cap, ep, cas, promo)
		t.Logf("legal moves list:")
		for _, m := range moves {
			t.Logf("  %s mp=%v cap=%v flag=%d", m.String(), m.MovedPiece(), m.CapturedPiece(), m.Flags())
		}
		t.Logf("pseudo moves list:")
		for _, m := range pseudo {
			t.Logf("  %s mp=%v cap=%v flag=%d", m.String(), m.MovedPiece(), m.CapturedPiece(), m.Flags())
		}
		t.Fatalf("Kiwipete depth1: got %d want %d", got, 48)
	}
	if got := myengine.Perft(board, 2); got != 2039 {
		t.Fatalf("Kiwipete depth2: got %d want %d", got, 2039)
	}
	if got := myengine.Perft(board, 3); got != 97862 {
		t.Fatalf("Kiwipete depth3: got %d want %d", got, 97862)
	}
}

func TestPerftEnPassantPosition(t *testing.T) {
	fen := "k7/8/8/3pP3/8/8/8/7K w - d6 0 2"
	board, err := myengine.ParseFEN(fen)
	if err != nil {
		t.Fatalf("ParseFEN failed: %v", err)
	}
	if got := myengine.Perft(board, 1); got != 5 {
		t.Fatalf("EP depth1: got %d want %d", got, 5)
	}
	if got := myengine.Perft(board, 2); got != 19 {
		t.Fatalf("EP depth2: got %d want %d", got, 19)
	}
}

func TestPerftPromotionPosition(t *testing.T) {
	fen := "1n5k/P7/8/8/8/8/8/7K w - - 0 1"
	board, err := myengine.ParseFEN(fen)
	if err != nil {
		t.Fatalf("ParseFEN failed: %v", err)
	}
	if got := myengine.Perft(board, 1); got != 11 {
		t.Fatalf("Promotion depth1: got %d want %d", got, 11)
	}
}

func TestPerftInitialDepth3(t *testing.T) {
    board, err := myengine.ParseFEN(myengine.FENStartPos)
    if err != nil {
        t.Fatalf("ParseFEN failed: %v", err)
    }
    if got := myengine.Perft(board, 3); got != 8902 {
        t.Fatalf("Initial depth3: got %d want %d", got, 8902)
    }
}

func TestPerftInitialDeep(t *testing.T) {
    board, err := myengine.ParseFEN(myengine.FENStartPos)
    if err != nil { t.Fatalf("ParseFEN failed: %v", err) }

    // Depth 4
    if got := myengine.Perft(board, 4); got != 197281 {
        t.Fatalf("Initial depth4: got %d want %d", got, 197281)
    }

    // Depth 5 can be heavier; allow skipping under -short
    if testing.Short() {
        t.Skip("skipping depth 5 perft in short mode")
    }
    if got := myengine.Perft(board, 5); got != 4865609 {
        t.Fatalf("Initial depth5: got %d want %d", got, 4865609)
    }
}

// Additional standard perft positions from Chess Programming Wiki
func TestPerft_Position3(t *testing.T) {
	fen := "8/2p5/3p4/KP5r/1R3p1k/8/4P1P1/8 w - - 0 1"
	b, err := myengine.ParseFEN(fen)
	if err != nil {
		t.Fatalf("ParseFEN failed: %v", err)
	}
	if got := myengine.Perft(b, 1); got != 14 {
		t.Fatalf("Pos3 d1: got %d want %d", got, 14)
	}
	if got := myengine.Perft(b, 2); got != 191 {
		t.Fatalf("Pos3 d2: got %d want %d", got, 191)
	}
	if got := myengine.Perft(b, 3); got != 2812 {
		t.Fatalf("Pos3 d3: got %d want %d", got, 2812)
	}
}

func TestPerft_Position4(t *testing.T) {
	fen := "r3k2r/Pppp1ppp/1b3nbN/nP6/BBP1P3/q4N2/Pp1P2PP/R2Q1RK1 w kq - 0 1"
	b, err := myengine.ParseFEN(fen)
	if err != nil {
		t.Fatalf("ParseFEN failed: %v", err)
	}
	if got := myengine.Perft(b, 1); got != 6 {
		pseudo := b.GeneratePseudoMoves()
		moves := b.GenerateMoves()
		t.Logf("Pos4 diagnostic: pseudo=%d legal=%d", len(pseudo), len(moves))
		var cp, cn, cb, cr, cq, ck int
		for _, m := range moves {
			switch m.MovedPiece() {
			case myengine.WhitePawn, myengine.BlackPawn:
				cp++
			case myengine.WhiteKnight, myengine.BlackKnight:
				cn++
			case myengine.WhiteBishop, myengine.BlackBishop:
				cb++
			case myengine.WhiteRook, myengine.BlackRook:
				cr++
			case myengine.WhiteQueen, myengine.BlackQueen:
				cq++
			case myengine.WhiteKing, myengine.BlackKing:
				ck++
			}
		}
		t.Logf("Pos4 by piece: P=%d N=%d B=%d R=%d Q=%d K=%d", cp, cn, cb, cr, cq, ck)
		t.Logf("Pos4 legal moves list:")
		for _, m := range moves {
			t.Logf("  %s mp=%v cap=%v flag=%d", m.String(), m.MovedPiece(), m.CapturedPiece(), m.Flags())
		}
		t.Fatalf("Pos4 d1: got %d want %d", got, 6)
	}
	if got := myengine.Perft(b, 2); got != 264 {
		t.Fatalf("Pos4 d2: got %d want %d", got, 264)
	}
	if got := myengine.Perft(b, 3); got != 9467 {
		t.Fatalf("Pos4 d3: got %d want %d", got, 9467)
	}
}

func TestPerft_Position5(t *testing.T) {
	fen := "rnbq1k1r/pp1Pbppp/2p5/8/2B5/8/PPP1NnPP/RNBQK2R w KQ - 0 1"
	b, err := myengine.ParseFEN(fen)
	if err != nil {
		t.Fatalf("ParseFEN failed: %v", err)
	}
	if got := myengine.Perft(b, 1); got != 44 {
		t.Fatalf("Pos5 d1: got %d want %d", got, 44)
	}
	if got := myengine.Perft(b, 2); got != 1486 {
		t.Fatalf("Pos5 d2: got %d want %d", got, 1486)
	}
	if got := myengine.Perft(b, 3); got != 62379 {
		t.Fatalf("Pos5 d3: got %d want %d", got, 62379)
	}
}

func TestPerft_Position6(t *testing.T) {
	fen := "r4rk1/1pp1qppp/p1np1n2/2b1p1B1/2B1P1b1/P1NP1N2/1PP1QPPP/R4RK1 w - - 0 10"
	b, err := myengine.ParseFEN(fen)
	if err != nil {
		t.Fatalf("ParseFEN failed: %v", err)
	}
	if got := myengine.Perft(b, 1); got != 46 {
		pseudo := b.GeneratePseudoMoves()
		moves := b.GenerateMoves()
		t.Logf("Pos6 diagnostic: pseudo=%d legal=%d", len(pseudo), len(moves))
		var cp, cn, cb, cr, cq, ck int
		for _, m := range moves {
			switch m.MovedPiece() {
			case myengine.WhitePawn, myengine.BlackPawn:
				cp++
			case myengine.WhiteKnight, myengine.BlackKnight:
				cn++
			case myengine.WhiteBishop, myengine.BlackBishop:
				cb++
			case myengine.WhiteRook, myengine.BlackRook:
				cr++
			case myengine.WhiteQueen, myengine.BlackQueen:
				cq++
			case myengine.WhiteKing, myengine.BlackKing:
				ck++
			}
		}
		t.Logf("Pos6 by piece: P=%d N=%d B=%d R=%d Q=%d K=%d", cp, cn, cb, cr, cq, ck)
		t.Logf("Pos6 legal moves list:")
		for _, m := range moves {
			t.Logf("  %s mp=%v cap=%v flag=%d", m.String(), m.MovedPiece(), m.CapturedPiece(), m.Flags())
		}
		t.Fatalf("Pos6 d1: got %d want %d", got, 46)
	}
	if got := myengine.Perft(b, 2); got != 2079 {
		t.Fatalf("Pos6 d2: got %d want %d", got, 2079)
	}
	if got := myengine.Perft(b, 3); got != 89890 {
		t.Fatalf("Pos6 d3: got %d want %d", got, 89890)
	}
}
