package goosemg

import (
	"errors"
	"strings"
)

// Startpos constant.
const Startpos = FENStartPos

// FEN parser that panics on invalid input.
func ParseFen(fen string) Board {
	b, err := ParseFEN(fen)
	if err != nil {
		panic(err)
	}
	return *b
}

// ToFen exposes the camel-case variant expected by existing engine code.
func (b *Board) ToFen() string { return b.ToFEN() }

// Apply plays a move and returns an undo closure
func (b *Board) Apply(m Move) func() {
	ok, st := b.MakeMove(m)
	if !ok {
		panic("goosemg.Apply: illegal move applied")
	}
	return func() { b.UnmakeMove(m, st) }
}

// ApplyNullMove performs a null move and returns the corresponding undo closure.
func (b *Board) ApplyNullMove() func() {
	st := b.MakeNullMove()
	return func() { b.UnmakeNullMove(st) }
}

// OurKingInCheck reports whether the side to move has its king in check.
func (b *Board) OurKingInCheck() bool { return b.InCheck(b.sideToMove) }

// IsCapture reports whether the given move captures a piece (including en passant).
func IsCapture(m Move, b *Board) bool {
	toBB := uint64(1) << uint(m.To())
	if (toBB & (b.White.All | b.Black.All)) != 0 {
		return true
	}
	if b.enPassantSquare == NoSquare {
		return false
	}
	fromBB := uint64(1) << uint(m.From())
	originIsPawn := (fromBB & (b.White.Pawns | b.Black.Pawns)) != 0
	epBB := uint64(1) << uint(b.enPassantSquare)
	return originIsPawn && (toBB&epBB) != 0
}

// ParseMove converts a UCI string (e2e4, e7e8q, 0000) into a Move.
func ParseMove(movestr string) (Move, error) {
	movestr = strings.TrimSpace(strings.ToLower(movestr))
	if movestr == "0000" {
		return 0, nil
	}
	if len(movestr) < 4 || len(movestr) > 5 {
		return 0, errors.New("invalid move length")
	}
	from, err := algebraicToIndex(movestr[0:2])
	if err != nil {
		return 0, err
	}
	to, err := algebraicToIndex(movestr[2:4])
	if err != nil {
		return 0, err
	}
	var promo Piece
	if len(movestr) == 5 {
		switch movestr[4] {
		case 'q':
			promo = PieceFromType(White, PieceTypeQueen)
		case 'r':
			promo = PieceFromType(White, PieceTypeRook)
		case 'b':
			promo = PieceFromType(White, PieceTypeBishop)
		case 'n':
			promo = PieceFromType(White, PieceTypeKnight)
		default:
			return 0, errors.New("invalid promotion piece")
		}
		// Promotion piece color will be adjusted by callers via PieceFromType if necessary.
	}
	move := NewMove(Square(from), Square(to), NoPiece, NoPiece, promo, 0)
	return move, nil
}

func algebraicToIndex(alg string) (int, error) {
	if len(alg) != 2 {
		return 0, errors.New("invalid algebraic square length")
	}
	file := alg[0]
	rank := alg[1]
	if file < 'a' || file > 'h' || rank < '1' || rank > '8' {
		return 0, errors.New("invalid algebraic square")
	}
	return int(file-'a') + int(rank-'1')*8, nil
}
