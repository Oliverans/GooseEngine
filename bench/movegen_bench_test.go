package bench

import (
    "testing"
    eng "github.com/Oliverans/GooseEngineMG/goosemg"
)

func benchGenerateMoves(b *testing.B, fen string) {
    board, err := eng.ParseFEN(fen)
    if err != nil { b.Fatalf("ParseFEN: %v", err) }
    buf := make([]eng.Move, 0, 512)
    b.ReportAllocs()
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        buf = board.GenerateMovesInto(buf)
        buf = buf[:0]
    }
}

func BenchmarkGenerateMoves_Initial(b *testing.B) {
    benchGenerateMoves(b, eng.FENStartPos)
}

func BenchmarkGenerateMoves_Kiwipete(b *testing.B) {
    fen := "r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1"
    benchGenerateMoves(b, fen)
}

func BenchmarkGenerateMoves_Pos6(b *testing.B) {
    fen := "r4rk1/1pp1qppp/p1np1n2/2b1p3/2B1P3/2NP1N2/PPP1QPPP/R4RK1 w - - 0 10"
    benchGenerateMoves(b, fen)
}

func benchCaptures(b *testing.B, fen string) {
    board, err := eng.ParseFEN(fen)
    if err != nil { b.Fatalf("ParseFEN: %v", err) }
    buf := make([]eng.Move, 0, 256)
    b.ReportAllocs(); b.ResetTimer()
    for i := 0; i < b.N; i++ {
        buf = board.GenerateCapturesInto(buf)
        buf = buf[:0]
    }
}

func BenchmarkGenerateCaptures_EP(b *testing.B) {
    fen := "k7/8/8/3pP3/8/8/8/7K w - d6 0 2"
    benchCaptures(b, fen)
}

func benchQuiets(b *testing.B, fen string) {
    board, err := eng.ParseFEN(fen)
    if err != nil { b.Fatalf("ParseFEN: %v", err) }
    buf := make([]eng.Move, 0, 256)
    b.ReportAllocs(); b.ResetTimer()
    for i := 0; i < b.N; i++ {
        buf = board.GenerateQuietsInto(buf)
        buf = buf[:0]
    }
}

func BenchmarkGenerateQuiets_Initial(b *testing.B) {
    benchQuiets(b, eng.FENStartPos)
}

func BenchmarkMakeUnmake_AllMoves_Initial(b *testing.B) {
    board, err := eng.ParseFEN(eng.FENStartPos)
    if err != nil { b.Fatalf("ParseFEN: %v", err) }
    moves := board.GenerateMoves()
    b.ReportAllocs(); b.ResetTimer()
    for i := 0; i < b.N; i++ {
        for _, m := range moves {
            ok, st := board.MakeMove(m)
            if !ok { b.Fatalf("illegal move in cached list: %v", m) }
            board.UnmakeMove(m, st)
        }
    }
}

