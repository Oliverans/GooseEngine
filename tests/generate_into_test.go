package goose_engine_mg_test

import (
    "testing"
    myengine "github.com/Oliverans/GooseEngineMG/goosemg"
)

// Ensure GenerateMovesInto reuses the provided buffer and avoids allocations when capacity suffices.
func TestGenerateMovesInto_NoAlloc(t *testing.T) {
    b, err := myengine.ParseFEN(myengine.FENStartPos)
    if err != nil { t.Fatal(err) }

    // Preallocate a buffer large enough for expected moves
    buf := make([]myengine.Move, 0, 256)

    allocs := testing.AllocsPerRun(100, func() {
        buf = b.GenerateMovesInto(buf)
        if len(buf) != 20 {
            t.Fatalf("expected 20 moves, got %d", len(buf))
        }
        // Reset length for next run while keeping capacity
        buf = buf[:0]
    })
    if allocs != 0 {
        t.Fatalf("expected 0 allocs, got %f", allocs)
    }
}

func TestGeneratePseudoMovesInto_NoAlloc(t *testing.T) {
    b, err := myengine.ParseFEN(myengine.FENStartPos)
    if err != nil { t.Fatal(err) }

    buf := make([]myengine.Move, 0, 256)

    allocs := testing.AllocsPerRun(100, func() {
        buf = b.GeneratePseudoMovesInto(buf)
        if len(buf) != 20 { // initial position pseudo moves should also be 20
            t.Fatalf("expected 20 pseudo moves, got %d", len(buf))
        }
        buf = buf[:0]
    })
    if allocs != 0 {
        t.Fatalf("expected 0 allocs, got %f", allocs)
    }
}

func TestGenerateCapturesInto_NoAlloc(t *testing.T) {
    // En passant position has exactly 1 capture available
    fen := "k7/8/8/3pP3/8/8/8/7K w - d6 0 2"
    b, err := myengine.ParseFEN(fen)
    if err != nil { t.Fatal(err) }

    buf := make([]myengine.Move, 0, 256)
    allocs := testing.AllocsPerRun(100, func() {
        buf = b.GenerateCapturesInto(buf)
        if len(buf) != 1 {
            t.Fatalf("expected 1 capture (EP), got %d", len(buf))
        }
        buf = buf[:0]
    })
    if allocs != 0 {
        t.Fatalf("expected 0 allocs, got %f", allocs)
    }
}

func TestGenerateQuietsInto_NoAlloc(t *testing.T) {
    b, err := myengine.ParseFEN(myengine.FENStartPos)
    if err != nil { t.Fatal(err) }

    buf := make([]myengine.Move, 0, 256)
    allocs := testing.AllocsPerRun(100, func() {
        buf = b.GenerateQuietsInto(buf)
        if len(buf) != 20 {
            t.Fatalf("expected 20 quiet moves in initial position, got %d", len(buf))
        }
        buf = buf[:0]
    })
    if allocs != 0 {
        t.Fatalf("expected 0 allocs, got %f", allocs)
    }
}
