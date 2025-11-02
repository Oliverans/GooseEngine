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

func see(b *gm.Board, move gm.Move, debug bool) int {
	const maxDepth = 32

	var gain [maxDepth]int

	fromSquare := move.From()
	toSquare := move.To()

	sideToMove := b.Wtomove
	us := colorIndex(sideToMove)
	them := us ^ 1

	var pieces [2]gm.Bitboards
	pieces[colorWhite] = b.White
	pieces[colorBlack] = b.Black

	occupied := pieces[colorWhite].All | pieces[colorBlack].All

	from := uint8(fromSquare)
	to := uint8(toSquare)

	movingPiece := pieceAtSquare(from, &pieces[us])
	if movingPiece == gm.PieceTypeNone {
		return 0
	}

	capturedPiece := pieceAtSquare(to, &pieces[them])
	captureSquare := to

	if capturedPiece == gm.PieceTypeNone {
		if move.Flags()&gm.FlagEnPassant == 0 {
			return 0
		}
		if sideToMove {
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

	removePiece(&pieces[them], capturedPiece, captureSquare)
	occupied &^= PositionBB[int(captureSquare)]

	removePiece(&pieces[us], movingPiece, from)
	occupied &^= PositionBB[int(from)]

	movingPieceAfter := movingPiece
	if promo := move.PromotionPieceType(); promo != gm.PieceTypeNone {
		movingPieceAfter = promo
	}

	addPiece(&pieces[us], movingPieceAfter, to)
	occupied |= PositionBB[int(to)]

	gain[0] = SeePieceValue[capturedPiece]

	capturedPieceType := movingPieceAfter
	side := them
	depth := 0

	for {
		attackers := attackersToSquare(to, occupied, pieces[side], side == colorWhite)
		attackers &^= PositionBB[int(to)]
		if attackers == 0 {
			break
		}

		attackerBB, attackerPiece := minAttacker(attackers, pieces[side])
		if attackerBB == 0 {
			break
		}

		attackSquare := uint8(bits.TrailingZeros64(attackerBB))

		depth++
		if depth >= maxDepth {
			depth = maxDepth - 1
		}
		gain[depth] = SeePieceValue[capturedPieceType] - gain[depth-1]

		if max(-gain[depth-1], gain[depth]) < 0 {
			break
		}

		removePiece(&pieces[side], attackerPiece, attackSquare)
		occupied &^= PositionBB[int(attackSquare)]

		opponent := side ^ 1
		removePiece(&pieces[opponent], capturedPieceType, to)
		occupied &^= PositionBB[int(to)]

		addPiece(&pieces[side], attackerPiece, to)
		occupied |= PositionBB[int(to)]

		capturedPieceType = attackerPiece
		side = opponent
	}

	for depth > 0 {
		gain[depth-1] = -max(-gain[depth-1], gain[depth])
		depth--
	}

	if debug {
		println("SEE gain:", gain[0])
	}

	return gain[0]
}

func attackersToSquare(target uint8, occupied uint64, pieces gm.Bitboards, white bool) uint64 {
	targetBB := PositionBB[int(target)]

	attackers := pawnAttackers(targetBB, pieces.Pawns, white)
	attackers |= KnightMasks[int(target)] & pieces.Knights
	attackers |= KingMoves[int(target)] & pieces.Kings

	bishopAttacks := gm.CalculateBishopMoveBitboard(target, occupied)
	rookAttacks := gm.CalculateRookMoveBitboard(target, occupied)

	attackers |= bishopAttacks & (pieces.Bishops | pieces.Queens)
	attackers |= rookAttacks & (pieces.Rooks | pieces.Queens)

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

func addPiece(bitboards *gm.Bitboards, piece gm.PieceType, square uint8) {
	mask := PositionBB[int(square)]
	bitboards.All |= mask
	switch piece {
	case gm.PieceTypePawn:
		bitboards.Pawns |= mask
	case gm.PieceTypeKnight:
		bitboards.Knights |= mask
	case gm.PieceTypeBishop:
		bitboards.Bishops |= mask
	case gm.PieceTypeRook:
		bitboards.Rooks |= mask
	case gm.PieceTypeQueen:
		bitboards.Queens |= mask
	case gm.PieceTypeKing:
		bitboards.Kings |= mask
	}
}

func removePiece(bitboards *gm.Bitboards, piece gm.PieceType, square uint8) {
	if piece == gm.PieceTypeNone {
		return
	}
	mask := ^PositionBB[int(square)]
	bitboards.All &= mask
	switch piece {
	case gm.PieceTypePawn:
		bitboards.Pawns &= mask
	case gm.PieceTypeKnight:
		bitboards.Knights &= mask
	case gm.PieceTypeBishop:
		bitboards.Bishops &= mask
	case gm.PieceTypeRook:
		bitboards.Rooks &= mask
	case gm.PieceTypeQueen:
		bitboards.Queens &= mask
	case gm.PieceTypeKing:
		bitboards.Kings &= mask
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
