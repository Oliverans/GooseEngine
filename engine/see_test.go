package engine

import (
	"testing"

	gm "chess-engine/goosemg"
)

func seeTestMove(t *testing.T, b *gm.Board, uci string) gm.Move {
	t.Helper()
	from := int(uci[0]-'a') + int(uci[1]-'1')*8
	to := int(uci[2]-'a') + int(uci[3]-'1')*8
	var promo gm.PieceType
	if len(uci) == 5 {
		switch uci[4] {
		case 'q':
			promo = gm.PieceTypeQueen
		case 'r':
			promo = gm.PieceTypeRook
		case 'b':
			promo = gm.PieceTypeBishop
		case 'n':
			promo = gm.PieceTypeKnight
		}
	}
	for _, m := range b.GenerateLegalMoves() {
		if int(m.From()) == from && int(m.To()) == to && m.PromotionPieceType() == promo {
			return m
		}
	}
	t.Fatalf("move %s is not legal in %s", uci, b.ToFEN())
	return 0
}

// Hand-verified exchange values. The promotion cases guard a real bug: gain[0]
// used to omit (promoted piece - pawn) while still charging the recapture for
// the promoted piece, which flipped the sign on winning capture-promotions and
// got them pruned out of quiescence by the SEE margin.
func TestSEEExchangeValues(t *testing.T) {
	InitPositionBB()

	cases := []struct {
		fen  string
		uci  string
		want int
		note string
	}{
		{"4k3/8/8/4n3/8/8/8/4R1K1 w - - 0 1", "e1e5", 300, "Rxn, undefended"},
		{"8/8/4k3/4r3/8/8/8/4R1K1 w - - 0 1", "e1e5", 0, "RxR, KxR"},
		{"8/8/4k3/4p3/8/8/8/4R1K1 w - - 0 1", "e1e5", -400, "Rxp defended by king"},
		{"4k3/8/2p5/3p4/4P3/8/8/4K3 w - - 0 1", "e4d5", 0, "pxp, pxp"},
		{"4k3/8/2p5/3p4/8/8/8/3QK3 w - - 0 1", "d1d5", -800, "Qxp defended by pawn"},

		// X-ray: the second rook must be discovered behind the first.
		{"4k3/8/3p4/4p3/8/8/4R3/4R1K1 w - - 0 1", "e2e5", -300, "R,p,R battery"},
		// The king may not recapture into the second rook; the 5000 sentinel
		// makes the fold decline it.
		{"8/8/4k3/4p3/8/8/4R3/4R1K1 w - - 0 1", "e2e5", 100, "king cannot recapture"},

		{"4k3/8/8/3pP3/8/8/8/4K3 w - d6 0 1", "e5d6", 100, "en passant, undefended"},
		{"4k3/8/8/3pP3/8/8/8/4K3 b - - 0 1", "d5d4", 0, "quiet move scores 0"},

		{"r3k3/1P6/8/8/8/8/8/4K3 w - - 0 1", "b7a8q", 1300, "promo capture, undefended: R+(Q-P)"},
		{"r2qk3/1P6/8/8/8/8/8/4K3 w - - 0 1", "b7a8q", 400, "promo capture, Q recaptures: R+(Q-P)-Q"},
		{"r2qk3/1P6/8/8/8/8/8/4K3 w - - 0 1", "b7a8n", 400, "underpromo to N: R+(N-P)-N"},
	}

	for _, c := range cases {
		b := gm.ParseFen(c.fen)
		m := seeTestMove(t, &b, c.uci)
		if got := see(&b, m, false); got != c.want {
			t.Errorf("see(%s) = %d, want %d (%s) in %s", c.uci, got, c.want, c.note, c.fen)
		}
	}
}

// A pawn recapturing onto the back rank promotes, so the exchange must account
// for the queen it becomes rather than treating it as a pawn.
func TestSEEPromotesRecapturingPawn(t *testing.T) {
	InitPositionBB()
	// Qxc8 bxc8 dxc8=Q: black declines the exchange because the d7 pawn
	// recaptures as a queen, so the bishop is simply won.
	b := gm.ParseFen("Q1b2r2/p1pP3p/2n5/n1P2k2/NpP5/PP2P1pP/2RB4/R3K1N1 w Q - 1 30")
	m := seeTestMove(t, &b, "a8c8")
	if got, want := see(&b, m, false), 300; got != want {
		t.Errorf("see(a8c8) = %d, want %d", got, want)
	}
}
