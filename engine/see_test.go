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

func quietSEEReference(b *gm.Board, move gm.Move) int {
	const maxDepth = 32

	var gain [maxDepth]int
	from := uint8(move.From())
	to := uint8(move.To())
	us := colorIndex(b.Wtomove)
	them := us ^ 1

	var pieces [2]gm.Bitboards
	pieces[colorWhite] = b.White
	pieces[colorBlack] = b.Black

	movingPiece := pieceAtSquare(from, &pieces[us])
	if movingPiece == gm.PieceTypeNone {
		return 0
	}

	occupied := pieces[colorWhite].All | pieces[colorBlack].All
	occupied &^= PositionBB[int(from)]
	occupied |= PositionBB[int(to)]

	diagSliders := pieces[colorWhite].Bishops | pieces[colorWhite].Queens |
		pieces[colorBlack].Bishops | pieces[colorBlack].Queens
	orthoSliders := pieces[colorWhite].Rooks | pieces[colorWhite].Queens |
		pieces[colorBlack].Rooks | pieces[colorBlack].Queens

	attackers := allAttackersTo(to, occupied, &pieces)
	onSquare := movingPiece
	side := them
	depth := 0

	for {
		attackers &= occupied
		sideAttackers := attackers & pieces[side].All
		if sideAttackers == 0 {
			break
		}

		attackerBB, attackerPiece := minAttacker(sideAttackers, pieces[side])
		if attackerBB == 0 {
			break
		}

		promotedTo := attackerPiece
		promoBonus := 0
		if attackerPiece == gm.PieceTypePawn && (to < 8 || to > 55) {
			promotedTo = gm.PieceTypeQueen
			promoBonus = SeePieceValue[gm.PieceTypeQueen] - SeePieceValue[gm.PieceTypePawn]
		}

		depth++
		gain[depth] = SeePieceValue[onSquare] + promoBonus - gain[depth-1]
		if gain[depth] <= -gain[depth-1] {
			break
		}

		occupied &^= attackerBB

		switch attackerPiece {
		case gm.PieceTypeKnight:
		case gm.PieceTypePawn, gm.PieceTypeBishop:
			attackers |= gm.CalculateBishopMoveBitboard(to, occupied) & diagSliders
		case gm.PieceTypeRook:
			attackers |= gm.CalculateRookMoveBitboard(to, occupied) & orthoSliders
		default:
			attackers |= gm.CalculateBishopMoveBitboard(to, occupied) & diagSliders
			attackers |= gm.CalculateRookMoveBitboard(to, occupied) & orthoSliders
		}

		onSquare = promotedTo
		side ^= 1
	}

	for depth > 0 {
		gain[depth-1] = -max(-gain[depth-1], gain[depth])
		depth--
	}

	return gain[0]
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

func TestSEEGEQuietValues(t *testing.T) {
	InitPositionBB()

	cases := []struct {
		fen  string
		uci  string
		want int
	}{
		{"4k3/8/8/8/5N2/8/8/4K3 w - - 0 1", "f4d5", 0},
		{"4k3/8/8/3p4/8/8/4P3/4K3 w - - 0 1", "e2e4", -100},
		{"4k3/8/4p3/8/5N2/8/8/4K3 w - - 0 1", "f4d5", -300},
		{"4k3/8/4p3/8/2P2N2/8/8/4K3 w - - 0 1", "f4d5", -200},
		{"4k3/8/2p1p3/8/2P2N2/8/B7/4K3 w - - 0 1", "f4d5", -200},
		{"k2q4/3r4/8/8/5N2/3R4/8/K2Q4 w - - 0 1", "f4d5", 0},
		{"4k3/8/8/2p2n2/8/4P3/8/4K3 b - - 0 1", "f5d4", -200},
		{"4k3/8/8/8/8/8/pR6/4K3 w - - 0 1", "b2b1", -1300},
	}

	for _, test := range cases {
		b := gm.ParseFen(test.fen)
		move := seeTestMove(t, &b, test.uci)
		if got := quietSEEReference(&b, move); got != test.want {
			t.Fatalf("quietSEEReference(%s) = %d, want %d", test.uci, got, test.want)
		}
		for _, threshold := range []int{test.want - 1, test.want, test.want + 1} {
			want := test.want >= threshold
			if got := seeGE(&b, move, threshold); got != want {
				t.Errorf("seeGE(%s, %d) = %v, want %v", test.uci, threshold, got, want)
			}
		}
	}
}

func TestSEEGECaptureInvariant(t *testing.T) {
	InitPositionBB()

	cases := []struct {
		fen string
		uci string
	}{
		{"4k3/8/8/4n3/8/8/8/4R1K1 w - - 0 1", "e1e5"},
		{"4k3/8/2p5/3p4/8/8/8/3QK3 w - - 0 1", "d1d5"},
		{"4k3/8/3p4/4p3/8/8/4R3/4R1K1 w - - 0 1", "e2e5"},
		{"4k3/8/8/3pP3/8/8/8/4K3 w - d6 0 1", "e5d6"},
		{"r2qk3/1P6/8/8/8/8/8/4K3 w - - 0 1", "b7a8q"},
		{"Q1b2r2/p1pP3p/2n5/n1P2k2/NpP5/PP2P1pP/2RB4/R3K1N1 w Q - 1 30", "a8c8"},
	}

	for _, test := range cases {
		b := gm.ParseFen(test.fen)
		move := seeTestMove(t, &b, test.uci)
		score := see(&b, move, false)
		for _, threshold := range []int{-1000, score - 1, score, score + 1, 0, 1000} {
			want := score >= threshold
			if got := seeGE(&b, move, threshold); got != want {
				t.Errorf("seeGE(%s, %d) = %v, want %v for SEE %d", test.uci, threshold, got, want, score)
			}
		}
	}
}

func TestSEEGEQuietReference(t *testing.T) {
	InitPositionBB()

	fens := []string{
		gm.Startpos,
		"r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1",
		"8/2p5/3p4/KP5r/1R3p1k/8/4P1P1/8 w - - 0 1",
		"r3k2r/Pppp1ppp/1b3nbN/nP6/BBP1P3/q4N2/Pp1P2PP/R2Q1RK1 w kq - 0 1",
		"rnbq1k1r/pp1Pbppp/2p5/8/2B5/8/PPP1NnPP/RNBQK2R w KQ - 1 8",
		"r4rk1/1pp1qppp/p1np1n2/2b1p1B1/2B1P1b1/P1NP1N2/1PP1QPPP/R4RK1 w - - 0 10",
		"r3k2r/1bp1qpb1/p1np1np1/4p2p/2P1P3/1PN2N1P/PB1PQPB1/R3K2R w KQkq - 0 1",
		"2kr3r/pbpn1pq1/1p2pn1p/3p2p1/2PP4/P1N1P1P1/1PQ1NPBP/R4RK1 w - - 0 1",
		"r2qk2r/ppp1bppp/2n1bn2/3pp3/8/2NPBNP1/PPP1PPBP/R2QK2R w KQkq - 0 1",
		"r1bq1rk1/ppp2ppp/2nb1n2/3pp3/2B1P3/2NP1N2/PPP2PPP/R1BQ1RK1 w - - 0 1",
		"4k3/8/2p1p3/8/2P2N2/8/B7/4K3 w - - 0 1",
		"4k3/8/8/2p2n2/8/4P3/8/4K3 b - - 0 1",
		"4k3/8/8/8/8/8/pR6/4K3 w - - 0 1",
	}

	for _, fen := range fens {
		b := gm.ParseFen(fen)
		for _, move := range b.GenerateLegalMoves() {
			if gm.IsCapture(move, &b) || move.PromotionPieceType() != gm.PieceTypeNone || move.Flags() == gm.FlagCastle {
				continue
			}
			score := quietSEEReference(&b, move)
			thresholds := []int{score - 1, score, score + 1, -405, -315, -270, -225, -180, -135, -90, -45, 0, 1}
			for _, threshold := range thresholds {
				want := score >= threshold
				if got := seeGE(&b, move, threshold); got != want {
					t.Fatalf("%s %s threshold %d: got %v, want %v for SEE %d", fen, move.String(), threshold, got, want, score)
				}
			}
		}
	}
}
