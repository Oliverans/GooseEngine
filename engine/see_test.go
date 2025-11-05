package engine

import (
    "bufio"
    "math/bits"
    "os"
    "path/filepath"
    "strconv"
    "strings"
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

func TestSEE_PromotionCapture_QueenLoses_RookOK(t *testing.T) {
    // Position: White pawn on e7, Black rook on f8, Black king on g8, White to move.
    // e7xf8=Q should be losing by SEE (queen gets recaptured by king), while e7xf8=R is roughly break-even.
    board, err := gm.ParseFEN("5rk1/4P3/8/8/8/8/8/6K1 w - - 0 1")
    if err != nil {
        t.Fatalf("parse FEN: %v", err)
    }

    from := square("e7")
    to := square("f8")

    // Promotion capture to queen
    moveQ := gm.NewMove(from, to, gm.WhitePawn, gm.NoPiece, gm.WhiteQueen, 0)
    scoreQ := see(board, moveQ, false)
    if scoreQ >= 0 {
        t.Fatalf("expected negative SEE for e7xf8=Q, got %d", scoreQ)
    }

    // Promotion capture to rook
    moveR := gm.NewMove(from, to, gm.WhitePawn, gm.NoPiece, gm.WhiteRook, 0)
    scoreR := see(board, moveR, false)
    if scoreR < 0 {
        t.Fatalf("expected non-negative SEE for e7xf8=R, got %d", scoreR)
    }
}

func TestSEE_QuietPromotion_ReturnsZero(t *testing.T) {
    // Position: White pawn on e7, empty e8. Quiet promotion should yield SEE 0.
    board, err := gm.ParseFEN("8/4P3/8/8/8/8/8/6K1 w - - 0 1")
    if err != nil {
        t.Fatalf("parse FEN: %v", err)
    }

    move := gm.NewMove(square("e7"), square("e8"), gm.WhitePawn, gm.NoPiece, gm.WhiteQueen, 0)
    score := see(board, move, false)
    if score != 0 {
        t.Fatalf("expected SEE 0 for quiet promotion, got %d", score)
    }
}

func TestSEE_SimpleWinningCapture(t *testing.T) {
    // Position: White knight c3, Black pawn d5. Nc3xd5 wins a pawn (+100) by SEE.
    board, err := gm.ParseFEN("8/8/8/3p4/8/2N5/8/6K1 w - - 0 1")
    if err != nil {
        t.Fatalf("parse FEN: %v", err)
    }

    move := gm.NewMove(square("c3"), square("d5"), gm.WhiteKnight, gm.BlackPawn, gm.NoPiece, 0)
    score := see(board, move, false)
    if score != SeePieceValue[gm.PieceTypePawn] {
        t.Fatalf("expected SEE %d, got %d", SeePieceValue[gm.PieceTypePawn], score)
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

// Test suite loader for SEE from engine/test_suite.txt
// Each line format: FEN ; move ; expectedScore ; comment
func TestSEE_FromSuiteFile(t *testing.T) {
    // Try both paths to support running from repo root or package dir
    candidates := []string{"test_suite.txt", filepath.Join("engine", "test_suite.txt")}
    var f *os.File
    var err error
    for _, p := range candidates {
        f, err = os.Open(p)
        if err == nil {
            defer f.Close()
            break
        }
    }
    if f == nil {
        t.Fatalf("could not open test_suite.txt: %v", err)
    }

    scanner := bufio.NewScanner(f)
    debugMode := strings.ToLower(os.Getenv("SEE_DEBUG")) // "", "all", "fail"
    lineNo := 0
    for scanner.Scan() {
        lineNo++
        line := strings.TrimSpace(scanner.Text())
        if line == "" || strings.HasPrefix(line, "#") {
            continue
        }
        parts := strings.Split(line, ";")
        if len(parts) < 3 {
            t.Fatalf("line %d: expected at least 3 ';'-separated fields, got %d: %q", lineNo, len(parts), line)
        }
        fen := strings.TrimSpace(parts[0])
        uci := strings.TrimSpace(parts[1])
        expStr := strings.TrimSpace(parts[2])
        expected, err := strconv.Atoi(expStr)
        if err != nil {
            t.Fatalf("line %d: invalid expected score %q: %v", lineNo, expStr, err)
        }

        board, err := gm.ParseFEN(fen)
        if err != nil {
            t.Fatalf("line %d: parse FEN failed: %v", lineNo, err)
        }
        mv, ok := findLegalMoveByUCI(board, uci)
        if !ok {
            t.Fatalf("line %d: could not find legal move %q in position", lineNo, uci)
        }

        // Decide debug per-line
        perLineDebug := false
        if len(parts) >= 4 {
            comment := strings.ToUpper(strings.TrimSpace(parts[3]))
            if strings.Contains(comment, "DEBUG") || strings.Contains(comment, "!") {
                perLineDebug = true
            }
        }
        if debugMode == "all" || perLineDebug {
            _ = see(board, mv, true)
        }

        got := see(board, mv, false)
        if got != expected {
            if debugMode == "fail" || debugMode == "all" {
                // Re-run with verbose to aid debugging
                t.Logf("line %d failed: SEE(%s) = %d, expected %d; verbose trace:", lineNo, uci, got, expected)
                _ = see(board, mv, true)
            }
            t.Fatalf("line %d: SEE(%s) = %d, expected %d (FEN: %s)", lineNo, uci, got, expected, fen)
        }
    }
    if err := scanner.Err(); err != nil {
        t.Fatalf("scanner error: %v", err)
    }
}

func findLegalMoveByUCI(b *gm.Board, uci string) (gm.Move, bool) {
    uci = strings.TrimSpace(strings.ToLower(uci))
    if len(uci) < 4 || len(uci) > 5 {
        return 0, false
    }
    fromAlg := uci[0:2]
    toAlg := uci[2:4]
    var promoType gm.PieceType
    if len(uci) == 5 {
        switch uci[4] {
        case 'q':
            promoType = gm.PieceTypeQueen
        case 'r':
            promoType = gm.PieceTypeRook
        case 'b':
            promoType = gm.PieceTypeBishop
        case 'n':
            promoType = gm.PieceTypeKnight
        default:
            return 0, false
        }
    }

    fromSq, err1 := algebraicToSquare(fromAlg)
    toSq, err2 := algebraicToSquare(toAlg)
    if err1 != nil || err2 != nil {
        return 0, false
    }

    moves := b.GenerateLegalMoves()
    for _, m := range moves {
        if m.From() != fromSq || m.To() != toSq {
            continue
        }
        if promoType != gm.PieceTypeNone && m.PromotionPieceType() != promoType {
            continue
        }
        return m, true
    }
    return 0, false
}

func algebraicToSquare(alg string) (gm.Square, error) {
    if len(alg) != 2 {
        return 0, strconv.ErrSyntax
    }
    file := int(alg[0] - 'a')
    rank := int(alg[1] - '1')
    if file < 0 || file > 7 || rank < 0 || rank > 7 {
        return 0, strconv.ErrSyntax
    }
    return gm.Square(rank*8 + file), nil
}
