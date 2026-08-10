package engine

import (
	"math/bits"

	gm "chess-engine/goosemg"
)

var KingMoves [65]uint64

var SeePieceValue = [7]int{
	gm.PieceTypeKing:   5000,
	gm.PieceTypePawn:   100,
	gm.PieceTypeKnight: 300,
	gm.PieceTypeBishop: 300,
	gm.PieceTypeRook:   500,
	gm.PieceTypeQueen:  900,
}

const (
	colorWhite = iota
	colorBlack
)

// see is a static exchange evaluation on the move's target square.
//
// Conventions worth knowing before changing anything here:
//
//   - Legality is ignored. A pinned defender still "recaptures", and a recapture
//     that would expose its own king is still counted. Modelling that would need
//     make/unmake per exchange step, which defeats the purpose of SEE. Stockfish
//     makes the same trade.
//   - The king is handled by giving it a 5000 sentinel value rather than by an
//     explicit rule: if it captures into a defended square the following
//     "recapture" is worth 5000, so the fold makes the king decline.
//   - Promotions are credited. The initial move promotes to whatever the move
//     encodes; a pawn recapturing onto the back rank is assumed to take a queen.
//     SEE cannot see check, so underpromotion tactics are search's problem, and
//     assuming a queen errs on the conservative side for the original capture.
func see(b *gm.Board, move gm.Move, debug bool) int {
	const maxDepth = 32

	var gain [maxDepth]int

	from := uint8(move.From())
	to := uint8(move.To())

	us := colorIndex(b.Wtomove)
	them := us ^ 1

	var pieces [2]gm.Bitboards
	pieces[colorWhite] = b.White
	pieces[colorBlack] = b.Black

	occupied := pieces[colorWhite].All | pieces[colorBlack].All

	movingPiece := pieceAtSquare(from, &pieces[us])
	if movingPiece == gm.PieceTypeNone {
		return 0
	}

	capturedPiece := pieceAtSquare(to, &pieces[them])
	captureSquare := to

	if capturedPiece == gm.PieceTypeNone {
		if move.Flags() != gm.FlagEnPassant {
			return 0
		}
		if b.Wtomove {
			if to < 8 {
				return 0
			}
			captureSquare = to - 8
		} else {
			if to > 55 {
				return 0
			}
			captureSquare = to + 8
		}
		if pieces[them].Pawns&PositionBB[int(captureSquare)] == 0 {
			return 0
		}
		capturedPiece = gm.PieceTypePawn
	}

	// Occupancy as it stands after the initial capture. The piece bitboards are
	// deliberately left stale from here on: anything that has been used is
	// already cleared from occupied, and the attacker set is masked by occupied
	// on every iteration, so stale entries can never be picked twice.
	occupied &^= PositionBB[int(from)] | PositionBB[int(captureSquare)]
	occupied |= PositionBB[int(to)]

	// onSquare is whatever now stands on the target and is therefore what the
	// next capture wins.
	onSquare := movingPiece
	gain[0] = SeePieceValue[capturedPiece]
	if promo := move.PromotionPieceType(); promo != gm.PieceTypeNone {
		gain[0] += SeePieceValue[promo] - SeePieceValue[gm.PieceTypePawn]
		onSquare = promo
	}

	diagSliders := pieces[colorWhite].Bishops | pieces[colorWhite].Queens |
		pieces[colorBlack].Bishops | pieces[colorBlack].Queens
	orthoSliders := pieces[colorWhite].Rooks | pieces[colorWhite].Queens |
		pieces[colorBlack].Rooks | pieces[colorBlack].Queens

	attackers := allAttackersTo(to, occupied, &pieces)

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
		if depth >= maxDepth {
			depth = maxDepth - 1
		}
		gain[depth] = SeePieceValue[onSquare] + promoBonus - gain[depth-1]

		// If capturing cannot beat simply stopping, the side to move stops. The
		// fold below can only lower gain[depth] further, so this is safe.
		if gain[depth] <= -gain[depth-1] {
			break
		}

		occupied &^= attackerBB

		// Only a slider standing behind the vacated square can appear, and the
		// attacker's own type tells us which ray it was standing on: a pawn or
		// bishop attacking the target must be on a diagonal, a rook on a rank or
		// file, and a knight on neither. Only a queen or king is ambiguous.
		switch attackerPiece {
		case gm.PieceTypeKnight:
			// A knight never stands on a ray from the target.
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

// allAttackersTo builds the attacker set for both colours in a single pass.
func allAttackersTo(target uint8, occupied uint64, pieces *[2]gm.Bitboards) uint64 {
	targetBB := PositionBB[int(target)]

	attackers := pawnAttackers(targetBB, pieces[colorWhite].Pawns, true)
	attackers |= pawnAttackers(targetBB, pieces[colorBlack].Pawns, false)
	attackers |= KnightMasks[int(target)] & (pieces[colorWhite].Knights | pieces[colorBlack].Knights)
	attackers |= KingMoves[int(target)] & (pieces[colorWhite].Kings | pieces[colorBlack].Kings)

	attackers |= gm.CalculateBishopMoveBitboard(target, occupied) &
		(pieces[colorWhite].Bishops | pieces[colorWhite].Queens |
			pieces[colorBlack].Bishops | pieces[colorBlack].Queens)
	attackers |= gm.CalculateRookMoveBitboard(target, occupied) &
		(pieces[colorWhite].Rooks | pieces[colorWhite].Queens |
			pieces[colorBlack].Rooks | pieces[colorBlack].Queens)

	return attackers
}

func pawnAttackers(targetBB uint64, pawns uint64, white bool) uint64 {
	if white {
		return (((targetBB >> 7) & ^bitboardFileA) & pawns) | (((targetBB >> 9) & ^bitboardFileH) & pawns)
	}
	return (((targetBB << 7) & ^bitboardFileH) & pawns) | (((targetBB << 9) & ^bitboardFileA) & pawns)
}

func pieceAtSquare(square uint8, bitboards *gm.Bitboards) gm.PieceType {
	mask := PositionBB[int(square)]
	switch {
	case bitboards.Pawns&mask != 0:
		return gm.PieceTypePawn
	case bitboards.Knights&mask != 0:
		return gm.PieceTypeKnight
	case bitboards.Bishops&mask != 0:
		return gm.PieceTypeBishop
	case bitboards.Rooks&mask != 0:
		return gm.PieceTypeRook
	case bitboards.Queens&mask != 0:
		return gm.PieceTypeQueen
	case bitboards.Kings&mask != 0:
		return gm.PieceTypeKing
	default:
		return gm.PieceTypeNone
	}
}

func colorIndex(white bool) int {
	if white {
		return colorWhite
	}
	return colorBlack
}

func minAttacker(attadef uint64, bb gm.Bitboards) (uint64, gm.PieceType) {
	var subset uint64
	var piece gm.PieceType

	if attadef&bb.Pawns > 0 {
		subset = attadef & bb.Pawns
		piece = gm.PieceTypePawn
	} else if attadef&bb.Knights > 0 {
		subset = attadef & bb.Knights
		piece = gm.PieceTypeKnight
	} else if attadef&bb.Bishops > 0 {
		subset = attadef & bb.Bishops
		piece = gm.PieceTypeBishop
	} else if attadef&bb.Rooks > 0 {
		subset = attadef & bb.Rooks
		piece = gm.PieceTypeRook
	} else if attadef&bb.Queens > 0 {
		subset = attadef & bb.Queens
		piece = gm.PieceTypeQueen
	} else if attadef&bb.Kings > 0 {
		subset = attadef & bb.Kings
		piece = gm.PieceTypeKing
	}

	if subset != 0 {
		return PositionBB[bits.TrailingZeros64(subset)], piece
	}

	return 0, gm.PieceTypeNone
}
