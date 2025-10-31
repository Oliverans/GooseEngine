package bench

import (
	"testing"

	eng "chess-engine/goosemg"
)

func benchPerft(b *testing.B, fen string, depth int) {
	board, err := eng.ParseFEN(fen)
	if err != nil {
		b.Fatalf("ParseFEN: %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = eng.Perft(board, depth)
	}
}

func BenchmarkPerft_Initial_D4(b *testing.B) {
	benchPerft(b, eng.FENStartPos, 4)
}

func BenchmarkPerft_Kiwipete_D3(b *testing.B) {
	fen := "r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1"
	benchPerft(b, fen, 3)
}
