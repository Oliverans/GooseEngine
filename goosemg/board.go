package goosemg

import "math/bits"

// Piece constants and types for pieces and colors
type Piece uint8

const (
	NoPiece     Piece = 0
	WhitePawn   Piece = 1
	WhiteKnight Piece = 2
	WhiteBishop Piece = 3
	WhiteRook   Piece = 4
	WhiteQueen  Piece = 5
	WhiteKing   Piece = 6

	// Black pieces are encoded as (white piece type | 8) so that
	// - piece & 7 gives the type in [1..6]
	// - piece & 8 != 0 indicates Black
	BlackPawn   Piece = 1 | 8
	BlackKnight Piece = 2 | 8
	BlackBishop Piece = 3 | 8
	BlackRook   Piece = 4 | 8
	BlackQueen  Piece = 5 | 8
	BlackKing   Piece = 6 | 8
)

// PieceType is a colorless representation of a chess piece used for table lookups.
type PieceType uint8

const (
	PieceTypeNone   PieceType = 0
	PieceTypePawn   PieceType = 1
	PieceTypeKnight PieceType = 2
	PieceTypeBishop PieceType = 3
	PieceTypeRook   PieceType = 4
	PieceTypeQueen  PieceType = 5
	PieceTypeKing   PieceType = 6
)

// Type returns the colorless type of the piece (ignores side).
func (p Piece) Type() PieceType { return PieceType(p & 7) }

// Color returns the side that owns the piece. NoPiece defaults to White.
func (p Piece) Color() Color { return colorOf(p) }

// PieceFromType combines a colorless type with a side to produce a concrete Piece.
func PieceFromType(color Color, pt PieceType) Piece {
	switch pt {
	case PieceTypePawn:
		if color == White {
			return WhitePawn
		}
		return BlackPawn
	case PieceTypeKnight:
		if color == White {
			return WhiteKnight
		}
		return BlackKnight
	case PieceTypeBishop:
		if color == White {
			return WhiteBishop
		}
		return BlackBishop
	case PieceTypeRook:
		if color == White {
			return WhiteRook
		}
		return BlackRook
	case PieceTypeQueen:
		if color == White {
			return WhiteQueen
		}
		return BlackQueen
	case PieceTypeKing:
		if color == White {
			return WhiteKing
		}
		return BlackKing
	default:
		return NoPiece
	}
}

type Color uint8

const (
	White Color = 0
	Black Color = 1
)

// Castling rights bit flags
type CastlingRights uint8

const (
	// White king-side (short) castling
	CastlingWhiteK CastlingRights = 1 << iota
	// White queen-side (long) castling
	CastlingWhiteQ
	// Black king-side castling
	CastlingBlackK
	// Black queen-side castling
	CastlingBlackQ
)

// Square represents a board position (0-63).
type Square int

const NoSquare Square = -1

// Bitboards exposes the per-piece bitboards for a color in a dragontooth-compatible layout.
type Bitboards struct {
	Pawns   uint64
	Knights uint64
	Bishops uint64
	Rooks   uint64
	Queens  uint64
	Kings   uint64
	All     uint64
}

// Board represents the chess board state, including piece placement and game state.
type Board struct {
	// Piece bitboards for each piece type and color (index 0 = white, 1 = black)
	pawns   [2]uint64
	knights [2]uint64
	bishops [2]uint64
	rooks   [2]uint64
	queens  [2]uint64
	kings   [2]uint64

	// Occupancy bitboards for each side
	occupancy [2]uint64 // occupancy[White], occupancy[Black]
	// (overall occupancy can be derived as occupancy[White] | occupancy[Black])

	// Piece placement array for each square (0 = NoPiece, otherwise a Piece constant)
	pieces [64]Piece

	// Side to move (which player's turn it is)
	sideToMove Color

	// Castling rights for both sides (bitmask using CastlingRights flags)
	castlingRights CastlingRights

	// En passant target square (if a pawn moved two steps last move, otherwise NoSquare)
	enPassantSquare Square

	// Halfmove clock (number of half-moves since last capture or pawn advance, for 50-move rule)
	halfmoveClock int

	// Fullmove number (starts at 1, incremented after Black's move)
	fullmoveNumber int

	// Zobrist hash key for the current position (for move repetition and hashing)
	zobristKey uint64
}

// HasLegalMoves reports whether the side to move has any legal moves.
func (b *Board) HasLegalMoves() bool {
	buf := make([]Move, 0, 64)
	moves := b.GenerateMovesInto(buf)
	return len(moves) > 0
}

// InCheckmate reports whether the side to move is checkmated.
func (b *Board) InCheckmate() bool {
	return b.InCheck(b.sideToMove) && !b.HasLegalMoves()
}

// InStalemate reports whether the side to move is stalemated.
func (b *Board) InStalemate() bool {
	return !b.InCheck(b.sideToMove) && !b.HasLegalMoves()
}

// IsDrawBy50 reports a 50-move rule draw (halfmoveClock counts half-moves).
func (b *Board) IsDrawBy50() bool {
	return b.halfmoveClock >= 100
}

// HalfmoveClock accessor for testing/consumers that want read-only access.
func (b *Board) HalfmoveClock() int { return b.halfmoveClock }

// FullmoveNumber returns the full move counter (incremented after Black's move).
func (b *Board) FullmoveNumber() int { return b.fullmoveNumber }

// EnPassantSquare returns the current en-passant target square or NoSquare.
func (b *Board) EnPassantSquare() Square { return b.enPassantSquare }

// SideToMove reports which side is to play.
func (b *Board) SideToMove() Color { return b.sideToMove }

// SetSideToMove updates the side to play. Use with care; normal move making toggles automatically.
func (b *Board) SetSideToMove(c Color) {
	if b.sideToMove == c {
		return
	}
	b.sideToMove = c
	b.zobristKey ^= zobristSide
}

// Hash returns the current Zobrist hash key.
func (b *Board) Hash() uint64 { return b.zobristKey }

// Bitboards returns the per-piece bitboards for the requested side.
func (b *Board) Bitboards(color Color) Bitboards {
	idx := int(color)
	return Bitboards{
		Pawns:   b.pawns[idx],
		Knights: b.knights[idx],
		Bishops: b.bishops[idx],
		Rooks:   b.rooks[idx],
		Queens:  b.queens[idx],
		Kings:   b.kings[idx],
		All:     b.occupancy[idx],
	}
}

// WhiteBitboards returns White's bitboards (copy).
func (b *Board) WhiteBitboards() Bitboards { return b.Bitboards(White) }

// BlackBitboards returns Black's bitboards (copy).
func (b *Board) BlackBitboards() Bitboards { return b.Bitboards(Black) }

// IsDrawByRepetition reports a draw by threefold repetition based on the provided
// history of Zobrist keys. The check counts occurrences of the current position's
// Zobrist key in the history plus the current position itself. If it appears
// three or more times, it returns true.
//
// Notes:
//   - The caller should typically pass keys since the last irreversible move
//     (capture or pawn move) for efficiency, though including a longer history is fine.
//   - Zobrist key already encodes side to move, castling rights and en passant file,
//     which are required for the repetition rule.
func (b *Board) IsDrawByRepetition(history []uint64) bool {
	target := b.zobristKey
	// Do not double-count if the last history entry is the current position.
	end := len(history)
	if end > 0 && history[end-1] == target {
		end--
	}
	matches := 0
	for i := 0; i < end; i++ {
		if history[i] == target {
			matches++
			if matches >= 2 { // plus current occurrence makes threefold
				return true
			}
		}
	}
	return false
}

// ==========================
// Move helpers for drivers
// ==========================

// PushMove attempts to make the move, and if legal, appends the resulting Zobrist
// key to the provided history and pushes the MoveState onto the stack for later undo.
// Returns true on success; on failure, board state is unchanged and nothing is appended.
func (b *Board) PushMove(m Move, stack *[]MoveState, history *[]uint64) bool {
	ok, st := b.MakeMove(m)
	if !ok {
		return false
	}
	*stack = append(*stack, st)
	*history = append(*history, b.zobristKey)
	return true
}

// PopMove undoes the last move pushed with PushMove, restoring the board state
// and truncating the history by one entry.
// It panics if the stack is empty.
func (b *Board) PopMove(stack *[]MoveState, history *[]uint64) {
	n := len(*stack)
	if n == 0 {
		panic("PopMove: empty stack")
	}
	st := (*stack)[n-1]
	*stack = (*stack)[:n-1]
	b.UnmakeMove(st.move, st)
	if len(*history) > 0 {
		*history = (*history)[:len(*history)-1]
	}
}

// ==========================
// Bitboard helpers
// ==========================

// bb returns a bitboard with the given square bit set.
func bb(sq Square) uint64 { return 1 << uint64(sq) }

// popLSB removes and returns the least significant set bit from the mask.
func popLSB(mask *uint64) int {
	x := *mask & -(*mask)
	idx := bits.TrailingZeros64(x)
	*mask &= *mask - 1
	return idx
}

// ==========================
// Board occupancy helpers
// ==========================

// AllOccupancy returns a bitboard of all occupied squares.
func (b *Board) AllOccupancy() uint64 { return b.occupancy[0] | b.occupancy[1] }

// ColorOccupancy returns the occupancy bitboard for the given color.
func (b *Board) ColorOccupancy(c Color) uint64 { return b.occupancy[int(c)] }

// PieceAt returns the piece on a square.
func (b *Board) PieceAt(sq Square) Piece { return b.pieces[int(sq)] }

// colorOf returns the color of a piece. NoPiece is treated as White.
func colorOf(p Piece) Color {
	if p&8 != 0 {
		return Black
	}
	return White
}

// typeOf returns the piece type in [1..6] with color stripped.
func typeOf(p Piece) Piece { return p & 7 }

// addPiece places a piece on an empty square and updates bitboards, occupancy and zobrist.
func (b *Board) addPiece(sq Square, p Piece) {
	if p == NoPiece {
		return
	}
	idx := int(sq)
	b.pieces[idx] = p
	c := colorOf(p)
	ci := int(c)
	b.occupancy[ci] |= bb(sq)
	switch typeOf(p) {
	case 1:
		b.pawns[ci] |= bb(sq)
	case 2:
		b.knights[ci] |= bb(sq)
	case 3:
		b.bishops[ci] |= bb(sq)
	case 4:
		b.rooks[ci] |= bb(sq)
	case 5:
		b.queens[ci] |= bb(sq)
	case 6:
		b.kings[ci] |= bb(sq)
	}
	// Zobrist: XOR in piece on square
	b.zobristKey ^= zobristPiece[p][idx]
}

// removePiece removes a piece from a square and updates bitboards, occupancy and zobrist.
func (b *Board) removePiece(sq Square) Piece {
	idx := int(sq)
	p := b.pieces[idx]
	if p == NoPiece {
		return NoPiece
	}
	c := colorOf(p)
	ci := int(c)
	mask := ^bb(sq)
	b.pieces[idx] = NoPiece
	b.occupancy[ci] &= mask
	switch typeOf(p) {
	case 1:
		b.pawns[ci] &= mask
	case 2:
		b.knights[ci] &= mask
	case 3:
		b.bishops[ci] &= mask
	case 4:
		b.rooks[ci] &= mask
	case 5:
		b.queens[ci] &= mask
	case 6:
		b.kings[ci] &= mask
	}
	// Zobrist: XOR out piece on square
	b.zobristKey ^= zobristPiece[p][idx]
	return p
}

// SetPiece sets a piece on a square, replacing any existing piece, and keeps state in sync.
func (b *Board) SetPiece(sq Square, p Piece) {
	b.removePiece(sq)
	b.addPiece(sq, p)
}

// ClearSquare removes any piece from the given square.
func (b *Board) ClearSquare(sq Square) { _ = b.removePiece(sq) }

// MovePiece moves a piece from one square to another. If a piece exists on 'to', it is captured.
func (b *Board) MovePiece(from, to Square) {
	moving := b.removePiece(from)
	// capture if any
	_ = b.removePiece(to)
	b.addPiece(to, moving)
}

// Validate checks internal consistency between pieces[], per-piece bitboards, and occupancy.
// Returns true if consistent, false otherwise.
func (b *Board) Validate() bool {
	var occ [2]uint64
	var pawns, knights, bishops, rooks, queens, kings [2]uint64
	for sq := 0; sq < 64; sq++ {
		p := b.pieces[sq]
		if p == NoPiece {
			continue
		}
		c := colorOf(p)
		ci := int(c)
		bit := uint64(1) << uint(sq)
		occ[ci] |= bit
		switch typeOf(p) {
		case 1:
			pawns[ci] |= bit
		case 2:
			knights[ci] |= bit
		case 3:
			bishops[ci] |= bit
		case 4:
			rooks[ci] |= bit
		case 5:
			queens[ci] |= bit
		case 6:
			kings[ci] |= bit
		}
	}
	if occ != b.occupancy {
		return false
	}
	if pawns != b.pawns || knights != b.knights || bishops != b.bishops || rooks != b.rooks || queens != b.queens || kings != b.kings {
		return false
	}
	// Cross-check Zobrist
	if b.zobristKey != b.ComputeZobrist() {
		return false
	}
	return true
}
