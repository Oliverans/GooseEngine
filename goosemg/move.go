package goosemg

import (
	"math/bits"
	"strings"
)

// Move encodes a chess move in a 32-bit value.
type Move uint32

// Bitfield layout within Move (from LSB to MSB)
const (
	moveFromShift    = 0  // 6 bits
	moveToShift      = 6  // 6 bits
	movePieceShift   = 12 // 4 bits
	moveCaptureShift = 16 // 4 bits
	movePromoteShift = 20 // 4 bits
	moveFlagShift    = 24 // 2 bits
)

// (Field masks omitted; getters use shifts directly for performance.)

// Move flags
const (
	FlagNone      = 0
	FlagCastle    = 1
	FlagEnPassant = 2
	// (Promotion is indicated by a non-zero promotion piece)
)

// NewMove constructs a Move value from components.
func NewMove(from, to Square, piece, captured Piece, promotion Piece, flag uint8) Move {
	m := uint32(from&0x3F) |
		(uint32(to&0x3F) << moveToShift) |
		(uint32(piece&0xF) << movePieceShift) |
		(uint32(captured&0xF) << moveCaptureShift) |
		(uint32(promotion&0xF) << movePromoteShift) |
		(uint32(flag&0x3) << moveFlagShift)
	return Move(m)
}

// From returns the source square of the move.
func (m Move) From() Square { return Square((uint32(m) >> moveFromShift) & 0x3F) }

// To returns the destination square of the move.
func (m Move) To() Square { return Square((uint32(m) >> moveToShift) & 0x3F) }

// MovedPiece returns the piece code that is moved.
func (m Move) MovedPiece() Piece { return Piece((uint32(m) >> movePieceShift) & 0xF) }

// CapturedPiece returns the piece code that was captured (or NoPiece if none).
func (m Move) CapturedPiece() Piece { return Piece((uint32(m) >> moveCaptureShift) & 0xF) }

// PromotionPiece returns the promotion piece code (or NoPiece if not a promotion).
func (m Move) PromotionPiece() Piece { return Piece((uint32(m) >> movePromoteShift) & 0xF) }

// PromotionPieceType returns the colorless type of the promoted piece (or PieceTypeNone).
func (m Move) PromotionPieceType() PieceType { return m.PromotionPiece().Type() }

// Flags returns the special move flags.
func (m Move) Flags() uint8 { return uint8((uint32(m) >> moveFlagShift) & 0x3) }

// String produces a simple string representation of the move (e.g. "e2e4", "e7e8Q").
func (m Move) String() string {
	fromSq := m.From()
	toSq := m.To()
	promo := m.PromotionPiece()

	// Convert squares to algebraic coordinates (e.g., 0 -> "a1")
	fileFrom := fromSq % 8
	rankFrom := fromSq / 8
	fileTo := toSq % 8
	rankTo := toSq / 8

	str := string([]byte{'a' + byte(fileFrom), '1' + byte(rankFrom)}) +
		string([]byte{'a' + byte(fileTo), '1' + byte(rankTo)})
	if promo != NoPiece {
		// Append promotion piece letter
		ch := charFromPiece(promo)
		str += strings.ToLower(string(ch))
	}
	return str
}

// GivesCheck reports whether the move (assumed legal for the current side to move)
// results in the opponent's king being in check. It performs a lightweight
// post-move attack query without mutating board state.
func (b *Board) GivesCheck(m Move) bool {
	us := int(b.sideToMove)
	them := 1 - us

	kingBB := b.kings[them]
	if kingBB == 0 {
		return false
	}
	ksq := bits.TrailingZeros64(kingBB)

	from := m.From()
	to := m.To()
	moved := m.MovedPiece()
	promo := m.PromotionPiece()
	flag := m.Flags()
	captured := m.CapturedPiece()

	fromBB := uint64(1) << uint(from)
	toBB := uint64(1) << uint(to)

	// Local copies of our piece bitboards and occupancy.
	pawnsUs := b.pawns[us]
	knightsUs := b.knights[us]
	bishopsUs := b.bishops[us]
	rooksUs := b.rooks[us]
	queensUs := b.queens[us]
	kingsUs := b.kings[us]

	occUs := b.occupancy[us]
	occThem := b.occupancy[them]

	// Handle capture (including en passant) on opponent occupancy.
	if flag == FlagEnPassant {
		var capSq Square
		if b.sideToMove == White {
			capSq = to - 8
		} else {
			capSq = to + 8
		}
		occThem &^= uint64(1) << uint(capSq)
	} else if captured != NoPiece {
		occThem &^= toBB
	}

	// Remove the moving piece from its origin.
	occUs &^= fromBB
	switch typeOf(moved) {
	case 1:
		pawnsUs &^= fromBB
	case 2:
		knightsUs &^= fromBB
	case 3:
		bishopsUs &^= fromBB
	case 4:
		rooksUs &^= fromBB
	case 5:
		queensUs &^= fromBB
	case 6:
		kingsUs &^= fromBB
	}

	// Add the piece on its destination (with promotion applied).
	pieceTo := moved
	if promo != NoPiece {
		pieceTo = promo
	}
	toType := typeOf(pieceTo)
	occUs |= toBB
	switch toType {
	case 1:
		pawnsUs |= toBB
	case 2:
		knightsUs |= toBB
	case 3:
		bishopsUs |= toBB
	case 4:
		rooksUs |= toBB
	case 5:
		queensUs |= toBB
	case 6:
		kingsUs |= toBB
	}

	// Castling rook movement.
	if flag == FlagCastle {
		rFrom, rTo := NoSquare, NoSquare
		if moved == WhiteKing {
			if to == 6 {
				rFrom, rTo = 7, 5
			} else if to == 2 {
				rFrom, rTo = 0, 3
			}
		} else if moved == BlackKing {
			if to == 62 {
				rFrom, rTo = 63, 61
			} else if to == 58 {
				rFrom, rTo = 56, 59
			}
		}
		if rFrom != NoSquare {
			rFromBB := uint64(1) << uint(rFrom)
			rToBB := uint64(1) << uint(rTo)
			rooksUs &^= rFromBB
			occUs &^= rFromBB
			rooksUs |= rToBB
			occUs |= rToBB
		}
	}

	occAll := occUs | occThem

	// Pawn attacks (use reverse tables like isSquareAttackedWithOcc).
	if b.sideToMove == White {
		if pawnAttacks[Black][ksq]&pawnsUs != 0 {
			return true
		}
	} else {
		if pawnAttacks[White][ksq]&pawnsUs != 0 {
			return true
		}
	}

	// Knights.
	if knightMoves[ksq]&knightsUs != 0 {
		return true
	}

	// Kings (needed for castling delivering check).
	if kingMoves[ksq]&kingsUs != 0 {
		return true
	}

	// Rook/queen attacks.
	rq := rooksUs | queensUs
	if rq != 0 {
		// N
		if blockers := rookRays[ksq][0] & occAll; blockers != 0 {
			lsb := blockers & -blockers
			if lsb&rq != 0 {
				return true
			}
		}
		// S
		if blockers := rookRays[ksq][1] & occAll; blockers != 0 {
			first := 63 - bits.LeadingZeros64(blockers)
			if (uint64(1)<<uint(first))&rq != 0 {
				return true
			}
		}
		// E
		if blockers := rookRays[ksq][2] & occAll; blockers != 0 {
			lsb := blockers & -blockers
			if lsb&rq != 0 {
				return true
			}
		}
		// W
		if blockers := rookRays[ksq][3] & occAll; blockers != 0 {
			first := 63 - bits.LeadingZeros64(blockers)
			if (uint64(1)<<uint(first))&rq != 0 {
				return true
			}
		}
	}

	// Bishop/queen attacks.
	bq := bishopsUs | queensUs
	if bq != 0 {
		// NE
		if blockers := bishopRays[ksq][0] & occAll; blockers != 0 {
			lsb := blockers & -blockers
			if lsb&bq != 0 {
				return true
			}
		}
		// NW
		if blockers := bishopRays[ksq][1] & occAll; blockers != 0 {
			lsb := blockers & -blockers
			if lsb&bq != 0 {
				return true
			}
		}
		// SE
		if blockers := bishopRays[ksq][2] & occAll; blockers != 0 {
			first := 63 - bits.LeadingZeros64(blockers)
			if (uint64(1)<<uint(first))&bq != 0 {
				return true
			}
		}
		// SW
		if blockers := bishopRays[ksq][3] & occAll; blockers != 0 {
			first := 63 - bits.LeadingZeros64(blockers)
			if (uint64(1)<<uint(first))&bq != 0 {
				return true
			}
		}
	}

	return false
}
