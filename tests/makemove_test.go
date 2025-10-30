package goose_engine_mg_test

import (
    "testing"
    myengine "github.com/Oliverans/GooseEngineMG/goosemg"
)

func TestMakeUnmake_NormalMove(t *testing.T) {
    b, err := myengine.ParseFEN(myengine.FENStartPos)
    if err != nil { t.Fatal(err) }
    startFEN := b.ToFEN()
    startZ := b.ComputeZobrist()

    from := myengine.Square(1*8 + 4) // e2
    to := myengine.Square(3*8 + 4)   // e4
    m := myengine.NewMove(from, to, myengine.WhitePawn, myengine.NoPiece, myengine.NoPiece, myengine.FlagNone)
    ok, st := b.MakeMove(m)
    if !ok { t.Fatalf("MakeMove failed for normal move") }
    if !b.Validate() { t.Fatalf("board invalid after MakeMove") }

    b.UnmakeMove(m, st)
    if !b.Validate() { t.Fatalf("board invalid after UnmakeMove") }
    if b.ToFEN() != startFEN { t.Fatalf("FEN mismatch after unmake: got %q want %q", b.ToFEN(), startFEN) }
    if b.ComputeZobrist() != startZ { t.Fatalf("zobrist mismatch after unmake") }
}

func TestMakeUnmake_Capture(t *testing.T) {
    b, err := myengine.ParseFEN("8/7r/8/8/8/8/8/R3K3 w - - 0 1")
    if err != nil { t.Fatal(err) }
    startZ := b.ComputeZobrist()
    // a1 rook captures h7 rook along rank
    from := myengine.Square(0)
    to := myengine.Square(6*8 + 7)
    m := myengine.NewMove(from, to, myengine.WhiteRook, myengine.BlackRook, myengine.NoPiece, myengine.FlagNone)
    ok, st := b.MakeMove(m)
    if !ok { t.Fatalf("MakeMove failed for capture move") }
    if !b.Validate() { t.Fatalf("board invalid after capture MakeMove") }
    b.UnmakeMove(m, st)
    if !b.Validate() { t.Fatalf("board invalid after capture UnmakeMove") }
    if b.ComputeZobrist() != startZ { t.Fatalf("zobrist mismatch after capture unmake") }
}

func TestMakeUnmake_EnPassant(t *testing.T) {
    // Position where white can capture en passant on d6
    fen := "k7/8/8/3pP3/8/8/8/7K w - d6 0 2"
    b, err := myengine.ParseFEN(fen)
    if err != nil { t.Fatal(err) }
    startZ := b.ComputeZobrist()
    from := myengine.Square(4*8 + 4) // e5
    to := myengine.Square(5*8 + 3)   // d6 (ep target)
    m := myengine.NewMove(from, to, myengine.WhitePawn, myengine.BlackPawn, myengine.NoPiece, myengine.FlagEnPassant)
    ok, st := b.MakeMove(m)
    if !ok { t.Fatalf("MakeMove failed for en passant") }
    if !b.Validate() { t.Fatalf("board invalid after en passant MakeMove") }
    b.UnmakeMove(m, st)
    if !b.Validate() { t.Fatalf("board invalid after en passant UnmakeMove") }
    if b.ComputeZobrist() != startZ { t.Fatalf("zobrist mismatch after ep unmake") }
}

func TestMakeUnmake_Castling(t *testing.T) {
    // Minimal castle-ready position for white: pieces on e1 and h1, empty between, rights K
    fen := "4k3/8/8/8/8/8/8/4K2R w K - 0 1"
    b, err := myengine.ParseFEN(fen)
    if err != nil { t.Fatal(err) }
    startZ := b.ComputeZobrist()
    from := myengine.Square(4)  // e1
    to := myengine.Square(6)    // g1
    m := myengine.NewMove(from, to, myengine.WhiteKing, myengine.NoPiece, myengine.NoPiece, myengine.FlagCastle)
    ok, st := b.MakeMove(m)
    if !ok { t.Fatalf("MakeMove failed for castling") }
    if !b.Validate() { t.Fatalf("board invalid after castling MakeMove") }
    // Rook should be on f1 (5)
    if got := b.PieceAt(5); got != myengine.WhiteRook {
        t.Fatalf("expected rook on f1 after castling, got %v", got)
    }
    b.UnmakeMove(m, st)
    if !b.Validate() { t.Fatalf("board invalid after castling UnmakeMove") }
    if b.ComputeZobrist() != startZ { t.Fatalf("zobrist mismatch after castling unmake") }
}

