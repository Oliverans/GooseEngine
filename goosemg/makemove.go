package goosemg

import "math/bits"

// MoveState holds the minimal state needed to undo a move.
type MoveState struct {
	move          Move
	captured      Piece
	prevCastling  CastlingRights
	prevEnPassant Square
	prevHalfmove  int
	prevFullmove  int
	prevZobrist   uint64
	rookFrom      Square // for castling undo
	rookTo        Square // for castling undo
}

// NullState stores the minimal information needed to undo a null move.
type NullState struct {
	prevEnPassant Square
	prevHalfmove  int
	prevFullmove  int
	prevZobrist   uint64
	prevSide      Color
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// MakeMove applies a move to the board. It returns ok=false if the move leaves the mover's king in check,
// restoring the original position.
func (b *Board) MakeMove(m Move) (ok bool, st MoveState) {
	st.move = m
	st.prevCastling = b.castlingRights
	st.prevEnPassant = b.enPassantSquare
	st.prevHalfmove = b.halfmoveClock
	st.prevFullmove = b.fullmoveNumber
	st.prevZobrist = b.zobristKey
	st.rookFrom, st.rookTo = NoSquare, NoSquare
	st.captured = NoPiece

	from := m.From()
	to := m.To()
	moved := m.MovedPiece()
	captured := m.CapturedPiece()
	promo := m.PromotionPiece()
	flag := m.Flags()

	// Remove previous en passant from Zobrist if present
	if b.enPassantSquare != NoSquare {
		file := int(b.enPassantSquare % 8)
		b.zobristKey ^= zobristEnPassant[file]
	}
	b.enPassantSquare = NoSquare

	// Fast-path updates (avoid generic add/remove where possible)
	us := b.side(b.sideToMove)
	them := b.side(b.sideToMove ^ 1)
	fromBB := uint64(1) << uint(from)
	toBB := uint64(1) << uint(to)

	// Handle capture (including en passant)
	if flag == FlagEnPassant {
		// Captured pawn is behind 'to'
		var capSq Square
		var capPiece Piece
		if b.sideToMove == White {
			capSq = to - 8
			capPiece = BlackPawn
		} else {
			capSq = to + 8
			capPiece = WhitePawn
		}
		st.captured = capPiece
		capBB := uint64(1) << uint(capSq)
		// Remove captured pawn
		b.pieces[int(capSq)] = NoPiece
		them.All &^= capBB
		them.Pawns &^= capBB
		b.zobristKey ^= zobristPiece[capPiece][int(capSq)]
	} else if captured != NoPiece {
		// Remove captured piece at 'to'
		st.captured = captured
		b.pieces[int(to)] = NoPiece
		them.All &^= toBB
		switch typeOf(captured) {
		case 1:
			them.Pawns &^= toBB
		case 2:
			them.Knights &^= toBB
		case 3:
			them.Bishops &^= toBB
		case 4:
			them.Rooks &^= toBB
		case 5:
			them.Queens &^= toBB
		case 6:
			them.Kings &^= toBB
		}
		b.zobristKey ^= zobristPiece[captured][int(to)]
	}

	// Move the piece (or promote)
	if promo != NoPiece {
		// Remove pawn at from
		b.pieces[int(from)] = NoPiece
		us.All &^= fromBB
		us.Pawns &^= fromBB
		b.zobristKey ^= zobristPiece[moved][int(from)]
		// Add promoted piece at to
		b.pieces[int(to)] = promo
		us.All |= toBB
		switch typeOf(promo) {
		case 2:
			us.Knights |= toBB
		case 3:
			us.Bishops |= toBB
		case 4:
			us.Rooks |= toBB
		case 5:
			us.Queens |= toBB
		case 6:
			us.Kings |= toBB
		}
		b.zobristKey ^= zobristPiece[promo][int(to)]
	} else {
		// Quiet move of the piece from -> to
		b.pieces[int(from)] = NoPiece
		b.pieces[int(to)] = moved
		us.All ^= (fromBB | toBB)
		switch typeOf(moved) {
		case 1:
			us.Pawns ^= (fromBB | toBB)
		case 2:
			us.Knights ^= (fromBB | toBB)
		case 3:
			us.Bishops ^= (fromBB | toBB)
		case 4:
			us.Rooks ^= (fromBB | toBB)
		case 5:
			us.Queens ^= (fromBB | toBB)
		case 6:
			us.Kings ^= (fromBB | toBB)
		}
		// Zobrist piece move
		b.zobristKey ^= zobristPiece[moved][int(from)]
		b.zobristKey ^= zobristPiece[moved][int(to)]
	}

	// Castling rook movement
	if flag == FlagCastle {
		if moved == WhiteKing {
			if to == 6 { // g1
				// Move rook h1->f1
				b.pieces[7] = NoPiece
				b.pieces[5] = WhiteRook
				rb := uint64(1) << 7
				nb := uint64(1) << 5
				us.All ^= (rb | nb)
				us.Rooks ^= (rb | nb)
				b.zobristKey ^= zobristPiece[WhiteRook][7]
				b.zobristKey ^= zobristPiece[WhiteRook][5]
				st.rookFrom, st.rookTo = 7, 5
			} else if to == 2 { // c1
				b.pieces[0] = NoPiece
				b.pieces[3] = WhiteRook
				rb := uint64(1) << 0
				nb := uint64(1) << 3
				us.All ^= (rb | nb)
				us.Rooks ^= (rb | nb)
				b.zobristKey ^= zobristPiece[WhiteRook][0]
				b.zobristKey ^= zobristPiece[WhiteRook][3]
				st.rookFrom, st.rookTo = 0, 3
			}
		} else if moved == BlackKing {
			if to == 62 { // g8
				b.pieces[63] = NoPiece
				b.pieces[61] = BlackRook
				rb := uint64(1) << 63
				nb := uint64(1) << 61
				us.All ^= (rb | nb)
				us.Rooks ^= (rb | nb)
				b.zobristKey ^= zobristPiece[BlackRook][63]
				b.zobristKey ^= zobristPiece[BlackRook][61]
				st.rookFrom, st.rookTo = 63, 61
			} else if to == 58 { // c8
				b.pieces[56] = NoPiece
				b.pieces[59] = BlackRook
				rb := uint64(1) << 56
				nb := uint64(1) << 59
				us.All ^= (rb | nb)
				us.Rooks ^= (rb | nb)
				b.zobristKey ^= zobristPiece[BlackRook][56]
				b.zobristKey ^= zobristPiece[BlackRook][59]
				st.rookFrom, st.rookTo = 56, 59
			}
		}
	}

	// Update castling rights
	newCR := b.castlingRights
	switch moved {
	case WhiteKing:
		newCR &^= (CastlingWhiteK | CastlingWhiteQ)
	case BlackKing:
		newCR &^= (CastlingBlackK | CastlingBlackQ)
	}
	if moved == WhiteRook {
		if from == 0 {
			newCR &^= CastlingWhiteQ
		} else if from == 7 {
			newCR &^= CastlingWhiteK
		}
	} else if moved == BlackRook {
		if from == 56 {
			newCR &^= CastlingBlackQ
		} else if from == 63 {
			newCR &^= CastlingBlackK
		}
	}
	// Rook captured on original squares removes rights
	if st.captured != NoPiece && typeOf(st.captured) == 4 {
		capSq := to
		switch capSq {
		case 0:
			newCR &^= CastlingWhiteQ
		case 7:
			newCR &^= CastlingWhiteK
		case 56:
			newCR &^= CastlingBlackQ
		case 63:
			newCR &^= CastlingBlackK
		}
	}
	if newCR != b.castlingRights {
		b.zobristKey ^= zobristCastle[int(b.castlingRights)]
		b.zobristKey ^= zobristCastle[int(newCR)]
		b.castlingRights = newCR
	}

	// Set en passant square if double pawn push
	if typeOf(moved) == 1 { // pawn
		fromRank := int(from) / 8
		toRank := int(to) / 8
		if abs(toRank-fromRank) == 2 {
			var ep Square
			if b.sideToMove == White {
				ep = from + 8
			} else {
				ep = from - 8
			}
			b.enPassantSquare = ep
			file := int(ep % 8)
			b.zobristKey ^= zobristEnPassant[file]
		}
	}

	// Toggle side to move (+ Zobrist) before legality check so Unmake can rely on the toggled state
	b.sideToMove = 1 - b.sideToMove
	b.zobristKey ^= zobristSide

	// Reject illegal move that leaves mover in check (direct attack query, avoid wrapper overhead)
	moverColor := 1 - b.sideToMove
	// Compute current occupancy and king square for mover
	occ := b.White.All | b.Black.All
	kingBB := b.side(moverColor).Kings
	if kingBB != 0 {
		ks := bits.TrailingZeros64(kingBB)
		// Gate the king-safety check: required for king moves, en passant, or when the moved piece
		// originates from a square on any rook/bishop ray from our king (potential discovered check).
		needCheck := true
		if typeOf(moved) != 6 && flag != FlagEnPassant { // not a king move and not EP
			rays := kingRaysUnion[ks]
			if ((rays >> uint(from)) & 1) == 0 {
				needCheck = false
			}
		}
		if needCheck && b.isSquareAttackedWithOcc(ks, 1-moverColor, occ) {
			b.UnmakeMove(m, st)
			return false, st
		}
	} else {
		// Shouldn't happen in valid positions; treat as illegal
		b.UnmakeMove(m, st)
		return false, st
	}

	// Halfmove clock
	if typeOf(moved) == 1 || st.captured != NoPiece {
		b.halfmoveClock = 0
	} else {
		b.halfmoveClock++
	}

	// Fullmove number increments after a legal Black move
	if moverColor == Black {
		b.fullmoveNumber++
	}

	b.syncTurnFlag()
	return true, st
}

// UnmakeMove undoes a previously made move, restoring board state.
func (b *Board) UnmakeMove(m Move, st MoveState) {
	// Toggle side back
	b.sideToMove = 1 - b.sideToMove
	b.zobristKey ^= zobristSide

	// Remove current en passant from Zobrist
	if b.enPassantSquare != NoSquare {
		file := int(b.enPassantSquare % 8)
		b.zobristKey ^= zobristEnPassant[file]
	}

	from := m.From()
	to := m.To()
	moved := m.MovedPiece()
	promo := m.PromotionPiece()
	flag := m.Flags()

	// Undo castling rook movement if any (inline)
	us := b.side(b.sideToMove)
	them := b.side(b.sideToMove ^ 1)
	if flag == FlagCastle && st.rookFrom != NoSquare && st.rookTo != NoSquare {
		// move rook back st.rookTo -> st.rookFrom
		fromR := int(st.rookFrom)
		toR := int(st.rookTo)
		rbFrom := uint64(1) << uint(fromR)
		rbTo := uint64(1) << uint(toR)
		rook := WhiteRook
		if moved&8 != 0 {
			rook = BlackRook
		}
		b.pieces[toR] = NoPiece
		b.pieces[fromR] = rook
		us.All ^= (rbFrom | rbTo)
		us.Rooks ^= (rbFrom | rbTo)
		// Zobrist adjusted at end by prevZobrist
	}

	// Move piece back (handle promotion) inline
	fromBB := uint64(1) << uint(from)
	toBB := uint64(1) << uint(to)
	// Clear current 'to'
	b.pieces[int(to)] = NoPiece
	if promo != NoPiece {
		// Place pawn back at from
		pawn := WhitePawn
		if moved&8 != 0 {
			pawn = BlackPawn
		}
		b.pieces[int(from)] = pawn
		us.All ^= (fromBB | toBB)
		// remove promo from to, add pawn at from
		switch typeOf(promo) {
		case 2:
			us.Knights &^= toBB
		case 3:
			us.Bishops &^= toBB
		case 4:
			us.Rooks &^= toBB
		case 5:
			us.Queens &^= toBB
		case 6:
			us.Kings &^= toBB
		}
		us.Pawns |= fromBB
	} else {
		// Move piece back to from
		b.pieces[int(from)] = moved
		us.All ^= (fromBB | toBB)
		switch typeOf(moved) {
		case 1:
			us.Pawns ^= (fromBB | toBB)
		case 2:
			us.Knights ^= (fromBB | toBB)
		case 3:
			us.Bishops ^= (fromBB | toBB)
		case 4:
			us.Rooks ^= (fromBB | toBB)
		case 5:
			us.Queens ^= (fromBB | toBB)
		case 6:
			us.Kings ^= (fromBB | toBB)
		}
	}

	// Restore captured piece
	if st.captured != NoPiece {
		if flag == FlagEnPassant {
			var capSq Square
			if moved&8 == 0 { // white moved originally
				capSq = to - 8
			} else {
				capSq = to + 8
			}
			capIdx := int(capSq)
			capBB := uint64(1) << uint(capSq)
			b.pieces[capIdx] = st.captured
			them.All |= capBB
			// Only pawns can be captured by EP
			them.Pawns |= capBB
		} else {
			// Normal capture: restore at 'to'
			b.pieces[int(to)] = st.captured
			them.All |= toBB
			switch typeOf(st.captured) {
			case 1:
				them.Pawns |= toBB
			case 2:
				them.Knights |= toBB
			case 3:
				them.Bishops |= toBB
			case 4:
				them.Rooks |= toBB
			case 5:
				them.Queens |= toBB
			case 6:
				them.Kings |= toBB
			}
		}
	}

	// Restore clocks, EP, castling rights
	if b.castlingRights != st.prevCastling {
		b.zobristKey ^= zobristCastle[int(b.castlingRights)]
		b.zobristKey ^= zobristCastle[int(st.prevCastling)]
	}
	b.castlingRights = st.prevCastling
	b.enPassantSquare = st.prevEnPassant
	if b.enPassantSquare != NoSquare {
		file := int(b.enPassantSquare % 8)
		b.zobristKey ^= zobristEnPassant[file]
	}
	b.halfmoveClock = st.prevHalfmove
	b.fullmoveNumber = st.prevFullmove

	// Ensure exact Zobrist restoration
	b.zobristKey = st.prevZobrist
	b.syncTurnFlag()
}

// MakeNullMove performs a null move: it switches the side to move without moving any piece.
// It clears any en passant square, updates zobrist side/en-passant keys, and advances clocks
// as a reversible quiet half-move. The returned state can be used to restore via UnmakeNullMove.
func (b *Board) MakeNullMove() (st NullState) {
	st.prevEnPassant = b.enPassantSquare
	st.prevHalfmove = b.halfmoveClock
	st.prevFullmove = b.fullmoveNumber
	st.prevZobrist = b.zobristKey
	st.prevSide = b.sideToMove

	// Remove current en passant from Zobrist if present
	if b.enPassantSquare != NoSquare {
		file := int(b.enPassantSquare % 8)
		b.zobristKey ^= zobristEnPassant[file]
	}
	b.enPassantSquare = NoSquare

	// Advance halfmove clock by a reversible quiet half-move
	b.halfmoveClock++

	// Toggle side and Zobrist side
	b.sideToMove = 1 - b.sideToMove
	b.zobristKey ^= zobristSide

	// Increment fullmove number after a Black move (i.e., if previous mover was Black)
	if st.prevSide == Black {
		b.fullmoveNumber++
	}
	b.syncTurnFlag()
	return st
}

// UnmakeNullMove restores the board to the state prior to MakeNullMove.
func (b *Board) UnmakeNullMove(st NullState) {
	b.enPassantSquare = st.prevEnPassant
	b.halfmoveClock = st.prevHalfmove
	b.fullmoveNumber = st.prevFullmove
	b.sideToMove = st.prevSide
	// Ensure exact Zobrist restoration
	b.zobristKey = st.prevZobrist
	b.syncTurnFlag()
}
