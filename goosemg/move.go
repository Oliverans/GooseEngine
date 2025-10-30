package goosemg

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
		str += string(ch)
	}
	return str
}
