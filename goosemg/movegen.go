package goosemg

import "math/bits"

// Precomputed attack masks for knights and kings from each square.
var knightMoves [64]uint64
var kingMoves [64]uint64

// Pawn attack masks: pawnAttacks[color][sq] gives bitboard of squares that a pawn of 'color' attacks from 'sq'.
var pawnAttacks [2][64]uint64

// Precomputed rays for sliders. For each square and direction, the bitboard of
// squares in that ray (excluding the origin square).
// Rook directions: 0=N, 1=S, 2=E, 3=W
var rookRays [64][4]uint64

// Bishop directions: 0=NE, 1=NW, 2=SE, 3=SW
var bishopRays [64][4]uint64

// Precomputed union of all rook and bishop rays from each square (for quick king-ray tests)
var kingRaysUnion [64]uint64

// Masks and lookup tables for magic-like slider attacks (using software pext).
var rookMask [64]uint64
var bishopMask [64]uint64
var rookAttTable [64][]uint64
var bishopAttTable [64][]uint64

func init() {
	initAttackTables()
	initRays()
	initSliderTables()
}

// initAttackTables precomputes move attack bitboards for knights, kings, and pawn captures.
func initAttackTables() {
	// Knight moves
	knightOffsets := [8][2]int{
		{2, 1}, {2, -1}, {-2, 1}, {-2, -1},
		{1, 2}, {1, -2}, {-1, 2}, {-1, -2},
	}
	for sq := 0; sq < 64; sq++ {
		file := sq % 8
		rank := sq / 8
		var mask uint64
		for _, off := range knightOffsets {
			rf := rank + off[0]
			ff := file + off[1]
			if rf >= 0 && rf < 8 && ff >= 0 && ff < 8 {
				target := rf*8 + ff
				mask |= uint64(1) << target
			}
		}
		knightMoves[sq] = mask
	}

	// King moves
	kingOffsets := [8][2]int{
		{1, 0}, {-1, 0}, {0, 1}, {0, -1},
		{1, 1}, {1, -1}, {-1, 1}, {-1, -1},
	}
	for sq := 0; sq < 64; sq++ {
		file := sq % 8
		rank := sq / 8
		var mask uint64
		for _, off := range kingOffsets {
			rf := rank + off[0]
			ff := file + off[1]
			if rf >= 0 && rf < 8 && ff >= 0 && ff < 8 {
				target := rf*8 + ff
				mask |= uint64(1) << target
			}
		}
		kingMoves[sq] = mask
	}

	// Pawn attacks
	for sq := 0; sq < 64; sq++ {
		file := sq % 8
		rank := sq / 8

		// White pawn attacks (moves upward)
		if rank < 7 {
			if file > 0 {
				pawnAttacks[White][sq] |= uint64(1) << ((rank+1)*8 + file - 1)
			}
			if file < 7 {
				pawnAttacks[White][sq] |= uint64(1) << ((rank+1)*8 + file + 1)
			}
		}

		// Black pawn attacks (moves downward)
		if rank > 0 {
			if file > 0 {
				pawnAttacks[Black][sq] |= uint64(1) << ((rank-1)*8 + file - 1)
			}
			if file < 7 {
				pawnAttacks[Black][sq] |= uint64(1) << ((rank-1)*8 + file + 1)
			}
		}
	}
}

// initRays precomputes directional rays for rook and bishop moves.
func initRays() {
	for sq := 0; sq < 64; sq++ {
		file := sq % 8
		rank := sq / 8

		// Rook rays

		// N
		var ray uint64
		for r := rank + 1; r < 8; r++ {
			t := r*8 + file
			ray |= 1 << uint(t)
		}
		rookRays[sq][0] = ray

		// S
		ray = 0
		for r := rank - 1; r >= 0; r-- {
			t := r*8 + file
			ray |= 1 << uint(t)
			if r == 0 {
				break
			}
		}
		rookRays[sq][1] = ray

		// E
		ray = 0
		for f := file + 1; f < 8; f++ {
			t := rank*8 + f
			ray |= 1 << uint(t)
		}
		rookRays[sq][2] = ray

		// W
		ray = 0
		for f := file - 1; f >= 0; f-- {
			t := rank*8 + f
			ray |= 1 << uint(t)
			if f == 0 {
				break
			}
		}
		rookRays[sq][3] = ray

		// Bishop rays

		// NE
		ray = 0
		for r, f := rank+1, file+1; r < 8 && f < 8; r, f = r+1, f+1 {
			t := r*8 + f
			ray |= 1 << uint(t)
		}
		bishopRays[sq][0] = ray

		// NW
		ray = 0
		for r, f := rank+1, file-1; r < 8 && f >= 0; r, f = r+1, f-1 {
			t := r*8 + f
			ray |= 1 << uint(t)
			if f == 0 {
				break
			}
		}
		bishopRays[sq][1] = ray

		// SE
		ray = 0
		for r, f := rank-1, file+1; r >= 0 && f < 8; r, f = r-1, f+1 {
			t := r*8 + f
			ray |= 1 << uint(t)
			if r == 0 {
				break
			}
		}
		bishopRays[sq][2] = ray

		// SW
		ray = 0
		for r, f := rank-1, file-1; r >= 0 && f >= 0; r, f = r-1, f-1 {
			t := r*8 + f
			ray |= 1 << uint(t)
			if r == 0 || f == 0 {
				break
			}
		}
		bishopRays[sq][3] = ray

		// Union of all rook and bishop rays from this square
		kingRaysUnion[sq] =
			rookRays[sq][0] | rookRays[sq][1] | rookRays[sq][2] | rookRays[sq][3] |
				bishopRays[sq][0] | bishopRays[sq][1] | bishopRays[sq][2] | bishopRays[sq][3]
	}
}

// initSliderTables builds per-square occupancy masks and attack tables.
func initSliderTables() {
	for sq := 0; sq < 64; sq++ {
		file := sq % 8
		rank := sq / 8

		// Rook mask excludes edge squares
		var rm uint64

		// North (exclude last rank)
		for r := rank + 1; r < 7; r++ {
			rm |= 1 << uint(r*8+file)
		}
		// South (exclude rank 0)
		for r := rank - 1; r > 0; r-- {
			rm |= 1 << uint(r*8+file)
		}
		// East (exclude file 7)
		for f := file + 1; f < 7; f++ {
			rm |= 1 << uint(rank*8+f)
		}
		// West (exclude file 0)
		for f := file - 1; f > 0; f-- {
			rm |= 1 << uint(rank*8+f)
		}
		rookMask[sq] = rm

		// Bishop mask excludes edges
		var bm uint64

		// NE
		for r, f := rank+1, file+1; r < 7 && f < 7; r, f = r+1, f+1 {
			bm |= 1 << uint(r*8+f)
		}
		// NW
		for r, f := rank+1, file-1; r < 7 && f > 0; r, f = r+1, f-1 {
			bm |= 1 << uint(r*8+f)
		}
		// SE
		for r, f := rank-1, file+1; r > 0 && f < 7; r, f = r-1, f+1 {
			bm |= 1 << uint(r*8+f)
		}
		// SW
		for r, f := rank-1, file-1; r > 0 && f > 0; r, f = r-1, f-1 {
			bm |= 1 << uint(r*8+f)
		}
		bishopMask[sq] = bm

		// Build attack tables by iterating all subsets of mask using software pdep
		rBits := bits.OnesCount64(rm)
		bBits := bits.OnesCount64(bm)
		rookAttTable[sq] = make([]uint64, 1<<rBits)
		bishopAttTable[sq] = make([]uint64, 1<<bBits)

		// Rook subsets
		for idx := 0; idx < (1 << rBits); idx++ {
			occ := pdep(uint64(idx), rm)
			rookAttTable[sq][idx] = rookAttacks(sq, occ)
		}
		// Bishop subsets
		for idx := 0; idx < (1 << bBits); idx++ {
			occ := pdep(uint64(idx), bm)
			bishopAttTable[sq][idx] = bishopAttacks(sq, occ)
		}
	}
}

// software pext: extract bits of x at positions where mask has 1s, packed into low bits
func pext(x, mask uint64) uint64 {
	var res uint64
	var idx uint
	m := mask
	for m != 0 {
		lsb := m & -m
		bit := uint(bits.TrailingZeros64(lsb))
		if (x>>bit)&1 != 0 {
			res |= 1 << idx
		}
		idx++
		m &= m - 1
	}
	return res
}

// software pdep: deposit low bits of x into positions of mask
func pdep(x, mask uint64) uint64 {
	var res uint64
	var idx uint
	m := mask
	for m != 0 {
		lsb := m & -m
		bit := uint(bits.TrailingZeros64(lsb))
		if (x>>idx)&1 != 0 {
			res |= 1 << bit
		}
		idx++
		m &= m - 1
	}
	return res
}

func rookAttacksMagic(sq int, occ uint64) uint64 {
	idx := pext(occ, rookMask[sq])
	return rookAttTable[sq][idx]
}

func bishopAttacksMagic(sq int, occ uint64) uint64 {
	idx := pext(occ, bishopMask[sq])
	return bishopAttTable[sq][idx]
}

// computeCheckAndPins computes check state and pin masks for the side to move.
// Returns:
// - inCheck: whether king is in check
// - doubleCheck: whether there are two or more checkers
// - checkMask: if single check, the set of squares that non-king moves may move to (block or capture)
// - pinLine: for each square (index), a mask of squares along the pin line the piece is allowed to move to; 0 means not pinned
func (b *Board) computeCheckAndPins(side Color, occ uint64) (inCheck bool, doubleCheck bool, checkMask uint64, pinLine [64]uint64) {
	us := int(side)
	them := 1 - us

	kingBB := b.kings[us]
	if kingBB == 0 {
		return false, false, 0, pinLine
	}
	ksq := bits.TrailingZeros64(kingBB)

	// Compute checkers
	var checkers uint64

	// Pawn attackers
	if side == White {
		// black pawns attack down; from white king's perspective, use White table
		checkers |= pawnAttacks[White][ksq] & b.pawns[them]
	} else {
		checkers |= pawnAttacks[Black][ksq] & b.pawns[them]
	}

	// Knights
	checkers |= knightMoves[ksq] & b.knights[them]

	// Bishops/Queens along diagonals
	diagAtk := bishopAttacks(ksq, occ)
	checkers |= diagAtk & (b.bishops[them] | b.queens[them])

	// Rooks/Queens along ranks/files
	orthoAtk := rookAttacks(ksq, occ)
	checkers |= orthoAtk & (b.rooks[them] | b.queens[them])

	inCheck = checkers != 0
	doubleCheck = inCheck && (checkers&(checkers-1)) != 0

	// If single check, compute mask of squares that block/capture
	if inCheck && !doubleCheck {
		c := bits.TrailingZeros64(checkers)
		cp := b.pieces[c]
		cbb := uint64(1) << uint(c)

		switch typeOf(cp) {
		case 2: // knight
			checkMask = cbb
		case 1: // pawn
			checkMask = cbb
		case 4: // rook
			// Determine direction from king to checker
			for d := 0; d < 4; d++ {
				if (rookRays[ksq][d] & cbb) != 0 {
					checkMask = rookRays[ksq][d] &^ rookRays[c][d]
					break
				}
			}
		case 3: // bishop
			for d := 0; d < 4; d++ {
				if (bishopRays[ksq][d] & cbb) != 0 {
					checkMask = bishopRays[ksq][d] &^ bishopRays[c][d]
					break
				}
			}
		case 5: // queen
			// Could be along rook or bishop direction
			for d := 0; d < 4; d++ {
				if (rookRays[ksq][d] & cbb) != 0 {
					checkMask = rookRays[ksq][d] &^ rookRays[c][d]
					break
				}
				if (bishopRays[ksq][d] & cbb) != 0 {
					checkMask = bishopRays[ksq][d] &^ bishopRays[c][d]
					break
				}
			}
		default:
			checkMask = cbb
		}
	}

	// Compute pins: for each ray from king, if first piece is ours and beyond it first opponent slider aligns, mark pin line

	// Rook-like directions
	for d := 0; d < 4; d++ {
		ray := rookRays[ksq][d]
		blockers := ray & occ
		if blockers == 0 {
			continue
		}

		var first int
		if d == 0 || d == 2 { // N, E increasing
			first = bits.TrailingZeros64(blockers)
		} else { // S, W decreasing
			first = 63 - bits.LeadingZeros64(blockers)
		}

		firstBB := uint64(1) << uint(first)
		if (firstBB & b.occupancy[us]) == 0 {
			continue
		}

		// Look beyond
		beyond := rookRays[first][d] & occ
		if beyond == 0 {
			continue
		}

		var next int
		if d == 0 || d == 2 {
			next = bits.TrailingZeros64(beyond)
		} else {
			next = 63 - bits.LeadingZeros64(beyond)
		}

		// If opponent rook or queen, then first is pinned
		p := b.pieces[next]
		if (typeOf(p) == 4 || typeOf(p) == 5) && colorOf(p) != side {
			pinLine[first] = rookRays[ksq][d] &^ rookRays[next][d]
		}
	}

	// Bishop-like directions
	for d := 0; d < 4; d++ {
		ray := bishopRays[ksq][d]
		blockers := ray & occ
		if blockers == 0 {
			continue
		}

		var first int
		// NE,NW increasing; SE,SW decreasing as per our ray construction
		if d == 0 || d == 1 { // NE, NW increasing
			first = bits.TrailingZeros64(blockers)
		} else { // SE, SW decreasing
			first = 63 - bits.LeadingZeros64(blockers)
		}

		firstBB := uint64(1) << uint(first)
		if (firstBB & b.occupancy[us]) == 0 {
			continue
		}

		beyond := bishopRays[first][d] & occ
		if beyond == 0 {
			continue
		}

		var next int
		if d == 0 || d == 1 {
			next = bits.TrailingZeros64(beyond)
		} else {
			next = 63 - bits.LeadingZeros64(beyond)
		}

		p := b.pieces[next]
		if (typeOf(p) == 3 || typeOf(p) == 5) && colorOf(p) != side {
			pinLine[first] = bishopRays[ksq][d] &^ bishopRays[next][d]
		}
	}

	return inCheck, doubleCheck, checkMask, pinLine
}

// ==========================
// Sliding attacks
// ==========================

// rookAttacks returns rook attack bitboard from sq given current occupancy.
func rookAttacks(sq int, occ uint64) uint64 {
	var attacks uint64

	// N (increasing indices)
	ray := rookRays[sq][0]
	blockers := ray & occ
	if blockers != 0 {
		first := bits.TrailingZeros64(blockers)
		ray &^= rookRays[first][0]
	}
	attacks |= ray

	// S (decreasing indices)
	ray = rookRays[sq][1]
	blockers = ray & occ
	if blockers != 0 {
		first := 63 - bits.LeadingZeros64(blockers)
		ray &^= rookRays[first][1]
	}
	attacks |= ray

	// E (increasing)
	ray = rookRays[sq][2]
	blockers = ray & occ
	if blockers != 0 {
		first := bits.TrailingZeros64(blockers)
		ray &^= rookRays[first][2]
	}
	attacks |= ray

	// W (decreasing)
	ray = rookRays[sq][3]
	blockers = ray & occ
	if blockers != 0 {
		first := 63 - bits.LeadingZeros64(blockers)
		ray &^= rookRays[first][3]
	}
	attacks |= ray

	return attacks
}

// bishopAttacks returns bishop attack bitboard from sq given current occupancy.
func bishopAttacks(sq int, occ uint64) uint64 {
	var attacks uint64

	// NE (increasing)
	ray := bishopRays[sq][0]
	blockers := ray & occ
	if blockers != 0 {
		first := bits.TrailingZeros64(blockers)
		ray &^= bishopRays[first][0]
	}
	attacks |= ray

	// NW (increasing)
	ray = bishopRays[sq][1]
	blockers = ray & occ
	if blockers != 0 {
		first := bits.TrailingZeros64(blockers)
		ray &^= bishopRays[first][1]
	}
	attacks |= ray

	// SE (decreasing)
	ray = bishopRays[sq][2]
	blockers = ray & occ
	if blockers != 0 {
		first := 63 - bits.LeadingZeros64(blockers)
		ray &^= bishopRays[first][2]
	}
	attacks |= ray

	// SW (decreasing)
	ray = bishopRays[sq][3]
	blockers = ray & occ
	if blockers != 0 {
		first := 63 - bits.LeadingZeros64(blockers)
		ray &^= bishopRays[first][3]
	}
	attacks |= ray

	return attacks
}

// queenAttacks is a convenience to combine rook and bishop attacks.
// queenAttacks was previously a thin wrapper; callers now directly OR rook/bishop attacks.

// ==========================
// Attack queries
// ==========================

// IsSquareAttacked reports whether the given square is attacked by the given color.
func (b *Board) IsSquareAttacked(sq Square, by Color) bool {
	return b.isSquareAttackedWithOcc(int(sq), by, b.AllOccupancy())
}

func (b *Board) isSquareAttackedWithOcc(s int, by Color, occ uint64) bool {
	byIdx := int(by)

	// Pawn attacks via reverse mask (fewer branches)
	if by == White {
		if (pawnAttacks[Black][s] & b.pawns[byIdx]) != 0 {
			return true
		}
	} else {
		if (pawnAttacks[White][s] & b.pawns[byIdx]) != 0 {
			return true
		}
	}

	// Knights
	if knightMoves[s]&b.knights[byIdx] != 0 {
		return true
	}

	// Kings
	if kingMoves[s]&b.kings[byIdx] != 0 {
		return true
	}

	// Slider identity checks using first blockers (unrolled, bitboard membership)
	rq := b.rooks[byIdx] | b.queens[byIdx]
	bq := b.bishops[byIdx] | b.queens[byIdx]

	// Rooks: N (0)
	if blockers := rookRays[s][0] & occ; blockers != 0 {
		lsb := blockers & -blockers
		if lsb&rq != 0 {
			return true
		}
	}
	// Rooks: S (1)
	if blockers := rookRays[s][1] & occ; blockers != 0 {
		first := 63 - bits.LeadingZeros64(blockers)
		if (uint64(1)<<uint(first))&rq != 0 {
			return true
		}
	}
	// Rooks: E (2)
	if blockers := rookRays[s][2] & occ; blockers != 0 {
		lsb := blockers & -blockers
		if lsb&rq != 0 {
			return true
		}
	}
	// Rooks: W (3)
	if blockers := rookRays[s][3] & occ; blockers != 0 {
		first := 63 - bits.LeadingZeros64(blockers)
		if (uint64(1)<<uint(first))&rq != 0 {
			return true
		}
	}

	// Bishops: NE (0)
	if blockers := bishopRays[s][0] & occ; blockers != 0 {
		lsb := blockers & -blockers
		if lsb&bq != 0 {
			return true
		}
	}
	// Bishops: NW (1)
	if blockers := bishopRays[s][1] & occ; blockers != 0 {
		lsb := blockers & -blockers
		if lsb&bq != 0 {
			return true
		}
	}
	// Bishops: SE (2)
	if blockers := bishopRays[s][2] & occ; blockers != 0 {
		first := 63 - bits.LeadingZeros64(blockers)
		if (uint64(1)<<uint(first))&bq != 0 {
			return true
		}
	}
	// Bishops: SW (3)
	if blockers := bishopRays[s][3] & occ; blockers != 0 {
		first := 63 - bits.LeadingZeros64(blockers)
		if (uint64(1)<<uint(first))&bq != 0 {
			return true
		}
	}

	return false
}

// InCheck reports whether the specified color's king is currently in check.
func (b *Board) InCheck(color Color) bool {
	ci := int(color)
	kingBB := b.kings[ci]
	if kingBB == 0 {
		return false
	}
	// Find king square (there is exactly one)
	ks := bits.TrailingZeros64(kingBB)
	return b.IsSquareAttacked(Square(ks), 1-color)
}

// GenerateMoves generates all legal moves for the current side to move.
// (Currently returns an empty list with TODOs for future implementation.)

// filter modes for selective generation
const (
	genAll = iota
	genCaptures
	genQuiets
)

// generateMovesFilteredInto is the core generator. It appends legal moves matching the filter into dst.
func (b *Board) generateMovesFilteredInto(dst []Move, filter int) []Move {
	moves := dst[:0]
	side := b.sideToMove
	us := int(side)
	them := 1 - us

	ownOcc := b.occupancy[us]
	oppOcc := b.occupancy[them]
	allOcc := ownOcc | oppOcc

	// Precompute our king square for local safety checks (e.g., EP simulation)
	kingBB := b.kings[us]
	ks := -1
	if kingBB != 0 {
		ks = bits.TrailingZeros64(kingBB)
	}

	// Compute check/pin state for pruning
	inCheck, doubleCheck, checkMask, pinLine := b.computeCheckAndPins(side, allOcc)

	// Pawns
	pawns := b.pawns[us]
	for pawns != 0 {
		from := popLSB(&pawns)
		fromSq := Square(from)
		movedPiece := b.pieces[from]
		pinMask := pinLine[from]

		if side == White {
			one := from + 8
			if one < 64 && ((allOcc>>uint(one))&1) == 0 {
				// Promotion or quiet push
				if one/8 == 7 {
					// promotions: Q R B N
					toBB := uint64(1) << uint(one)
					if !doubleCheck && (pinMask == 0 || (toBB&pinMask) != 0) && (!inCheck || (toBB&checkMask) != 0) {
						if filter != genCaptures {
							moves = append(moves,
								NewMove(fromSq, Square(one), movedPiece, NoPiece, WhiteQueen, FlagNone),
								NewMove(fromSq, Square(one), movedPiece, NoPiece, WhiteRook, FlagNone),
								NewMove(fromSq, Square(one), movedPiece, NoPiece, WhiteBishop, FlagNone),
								NewMove(fromSq, Square(one), movedPiece, NoPiece, WhiteKnight, FlagNone),
							)
						}
					}
				} else {
					toBB := uint64(1) << uint(one)
					if !doubleCheck && (pinMask == 0 || (toBB&pinMask) != 0) && (!inCheck || (toBB&checkMask) != 0) {
						if filter != genCaptures {
							moves = append(moves, NewMove(fromSq, Square(one), movedPiece, NoPiece, NoPiece, FlagNone))
						}
					}
					// double push
					if from/8 == 1 {
						two := from + 16
						if ((allOcc >> uint(two)) & 1) == 0 {
							toBB2 := uint64(1) << uint(two)
							if !doubleCheck && (pinMask == 0 || (toBB2&pinMask) != 0) && (!inCheck || (toBB2&checkMask) != 0) {
								if filter != genCaptures {
									moves = append(moves, NewMove(fromSq, Square(two), movedPiece, NoPiece, NoPiece, FlagNone))
								}
							}
						}
					}
				}
			}

			// Captures
			caps := pawnAttacks[White][from]

			// normal captures (exclude EP square)
			capTargets := caps & oppOcc
			for capTargets != 0 {
				to := popLSB(&capTargets)
				toSq := Square(to)
				capPiece := b.pieces[to]
				toBB := uint64(1) << uint(to)

				if doubleCheck || (pinMask != 0 && (toBB&pinMask) == 0) || (inCheck && (toBB&checkMask) == 0) {
					continue
				}

				if to/8 == 7 {
					if filter != genQuiets {
						moves = append(moves,
							NewMove(fromSq, toSq, movedPiece, capPiece, WhiteQueen, FlagNone),
							NewMove(fromSq, toSq, movedPiece, capPiece, WhiteRook, FlagNone),
							NewMove(fromSq, toSq, movedPiece, capPiece, WhiteBishop, FlagNone),
							NewMove(fromSq, toSq, movedPiece, capPiece, WhiteKnight, FlagNone),
						)
					}
				} else {
					if filter != genQuiets {
						moves = append(moves, NewMove(fromSq, toSq, movedPiece, capPiece, NoPiece, FlagNone))
					}
				}
			}

			// en passant (simulate occupancy change + king safety)
			if b.enPassantSquare != NoSquare {
				ep := int(b.enPassantSquare)
				if (caps & (1 << uint(ep))) != 0 {
					toBB := uint64(1) << uint(ep)
					if !(doubleCheck || (pinMask != 0 && (toBB&pinMask) == 0)) {
						if filter != genQuiets {
							// simulate: remove from, remove captured pawn at ep-8, add to
							occp := allOcc
							occp &^= (uint64(1) << uint(from))
							capSq := ep - 8
							occp &^= (uint64(1) << uint(capSq))
							occp |= (uint64(1) << uint(ep))
							if ks >= 0 {
								if !b.isSquareAttackedWithOcc(ks, Color(them), occp) {
									moves = append(moves, NewMove(fromSq, Square(ep), movedPiece, BlackPawn, NoPiece, FlagEnPassant))
								}
							}
						}
					}
				}
			}
		} else {
			// Black pawns
			one := from - 8
			if one >= 0 && ((allOcc>>uint(one))&1) == 0 {
				if one/8 == 0 {
					toBB := uint64(1) << uint(one)
					if !doubleCheck && (pinMask == 0 || (toBB&pinMask) != 0) && (!inCheck || (toBB&checkMask) != 0) {
						if filter != genCaptures {
							moves = append(moves,
								NewMove(fromSq, Square(one), movedPiece, NoPiece, BlackQueen, FlagNone),
								NewMove(fromSq, Square(one), movedPiece, NoPiece, BlackRook, FlagNone),
								NewMove(fromSq, Square(one), movedPiece, NoPiece, BlackBishop, FlagNone),
								NewMove(fromSq, Square(one), movedPiece, NoPiece, BlackKnight, FlagNone),
							)
						}
					}
				} else {
					toBB := uint64(1) << uint(one)
					if !doubleCheck && (pinMask == 0 || (toBB&pinMask) != 0) && (!inCheck || (toBB&checkMask) != 0) {
						if filter != genCaptures {
							moves = append(moves, NewMove(fromSq, Square(one), movedPiece, NoPiece, NoPiece, FlagNone))
						}
					}
					if from/8 == 6 {
						two := from - 16
						if ((allOcc >> uint(two)) & 1) == 0 {
							toBB2 := uint64(1) << uint(two)
							if !doubleCheck && (pinMask == 0 || (toBB2&pinMask) != 0) && (!inCheck || (toBB2&checkMask) != 0) {
								if filter != genCaptures {
									moves = append(moves, NewMove(fromSq, Square(two), movedPiece, NoPiece, NoPiece, FlagNone))
								}
							}
						}
					}
				}
			}

			caps := pawnAttacks[Black][from]
			capTargets := caps & oppOcc
			for capTargets != 0 {
				to := popLSB(&capTargets)
				toSq := Square(to)
				capPiece := b.pieces[to]
				toBB := uint64(1) << uint(to)

				if doubleCheck || (pinMask != 0 && (toBB&pinMask) == 0) || (inCheck && (toBB&checkMask) == 0) {
					continue
				}

				if to/8 == 0 {
					if filter != genQuiets {
						moves = append(moves,
							NewMove(fromSq, toSq, movedPiece, capPiece, BlackQueen, FlagNone),
							NewMove(fromSq, toSq, movedPiece, capPiece, BlackRook, FlagNone),
							NewMove(fromSq, toSq, movedPiece, capPiece, BlackBishop, FlagNone),
							NewMove(fromSq, toSq, movedPiece, capPiece, BlackKnight, FlagNone),
						)
					}
				} else {
					if filter != genQuiets {
						moves = append(moves, NewMove(fromSq, toSq, movedPiece, capPiece, NoPiece, FlagNone))
					}
				}
			}

			if b.enPassantSquare != NoSquare {
				ep := int(b.enPassantSquare)
				if (caps & (1 << uint(ep))) != 0 {
					toBB := uint64(1) << uint(ep)
					if !(doubleCheck || (pinMask != 0 && (toBB&pinMask) == 0)) {
						if filter != genQuiets {
							// simulate: remove from, remove captured pawn at ep+8, add to
							occp := allOcc
							occp &^= (uint64(1) << uint(from))
							capSq := ep + 8
							occp &^= (uint64(1) << uint(capSq))
							occp |= (uint64(1) << uint(ep))
							if ks >= 0 {
								if !b.isSquareAttackedWithOcc(ks, Color(them), occp) {
									moves = append(moves, NewMove(fromSq, Square(ep), movedPiece, WhitePawn, NoPiece, FlagEnPassant))
								}
							}
						}
					}
				}
			}
		}
	}

	// Knights
	if !doubleCheck { // only king can move in double check
		knights := b.knights[us]
		for knights != 0 {
			from := popLSB(&knights)
			fromSq := Square(from)
			movedPiece := b.pieces[from]
			pinMask := pinLine[from]

			targets := knightMoves[from] &^ ownOcc
			if pinMask != 0 {
				targets &= pinMask
			}
			if inCheck {
				targets &= checkMask
			}
			if filter == genCaptures {
				targets &= oppOcc
			}

			for t := targets; t != 0; {
				to := popLSB(&t)
				var cap Piece = NoPiece
				isCap := ((oppOcc >> uint(to)) & 1) != 0
				if isCap {
					cap = b.pieces[to]
				}
				if (filter == genCaptures && !isCap) || (filter == genQuiets && isCap) {
					continue
				}
				moves = append(moves, NewMove(fromSq, Square(to), movedPiece, cap, NoPiece, FlagNone))
			}
		}
	}

	// Bishops
	if !doubleCheck {
		bishops := b.bishops[us]
		for bishops != 0 {
			from := popLSB(&bishops)
			fromSq := Square(from)
			movedPiece := b.pieces[from]
			pinMask := pinLine[from]

			targets := bishopAttacksMagic(from, allOcc) &^ ownOcc
			if pinMask != 0 {
				targets &= pinMask
			}
			if inCheck {
				targets &= checkMask
			}
			if filter == genCaptures {
				targets &= oppOcc
			} else if filter == genQuiets {
				targets &^= oppOcc
			}

			for t := targets; t != 0; {
				to := popLSB(&t)
				var cap Piece = NoPiece
				isCap := ((oppOcc >> uint(to)) & 1) != 0
				if isCap {
					cap = b.pieces[to]
				}
				if (filter == genCaptures && !isCap) || (filter == genQuiets && isCap) {
					continue
				}
				m := NewMove(fromSq, Square(to), movedPiece, cap, NoPiece, FlagNone)
				moves = append(moves, m)
			}
		}
	}

	// Rooks
	if !doubleCheck {
		rooks := b.rooks[us]
		for rooks != 0 {
			from := popLSB(&rooks)
			fromSq := Square(from)
			movedPiece := b.pieces[from]
			pinMask := pinLine[from]

			targets := rookAttacksMagic(from, allOcc) &^ ownOcc
			if pinMask != 0 {
				targets &= pinMask
			}
			if inCheck {
				targets &= checkMask
			}
			if filter == genCaptures {
				targets &= oppOcc
			} else if filter == genQuiets {
				targets &^= oppOcc
			}

			for t := targets; t != 0; {
				to := popLSB(&t)
				var cap Piece = NoPiece
				isCap := ((oppOcc >> uint(to)) & 1) != 0
				if isCap {
					cap = b.pieces[to]
				}
				if (filter == genCaptures && !isCap) || (filter == genQuiets && isCap) {
					continue
				}
				m := NewMove(fromSq, Square(to), movedPiece, cap, NoPiece, FlagNone)
				moves = append(moves, m)
			}
		}
	}

	// Queens
	if !doubleCheck {
		queens := b.queens[us]
		for queens != 0 {
			from := popLSB(&queens)
			fromSq := Square(from)
			movedPiece := b.pieces[from]
			pinMask := pinLine[from]

			targets := (rookAttacksMagic(from, allOcc) | bishopAttacksMagic(from, allOcc)) &^ ownOcc
			if pinMask != 0 {
				targets &= pinMask
			}
			if inCheck {
				targets &= checkMask
			}
			if filter == genCaptures {
				targets &= oppOcc
			} else if filter == genQuiets {
				targets &^= oppOcc
			}

			for t := targets; t != 0; {
				to := popLSB(&t)
				var cap Piece = NoPiece
				isCap := ((oppOcc >> uint(to)) & 1) != 0
				if isCap {
					cap = b.pieces[to]
				}
				if (filter == genCaptures && !isCap) || (filter == genQuiets && isCap) {
					continue
				}
				m := NewMove(fromSq, Square(to), movedPiece, cap, NoPiece, FlagNone)
				moves = append(moves, m)
			}
		}
	}

	// King (normal moves)
	kbb := b.kings[us]
	if kbb != 0 {
		from := bits.TrailingZeros64(kbb)
		if from >= 0 {
			fromSq := Square(from)
			movedPiece := b.pieces[from]
			targets := kingMoves[from] &^ ownOcc

			for t := targets; t != 0; {
				to := popLSB(&t)
				isCap := ((oppOcc >> uint(to)) & 1) != 0
				if (filter == genCaptures && !isCap) || (filter == genQuiets && isCap) {
					continue
				}

				occp := allOcc
				occp &^= (uint64(1) << uint(from))
				if isCap {
					occp &^= (uint64(1) << uint(to))
				}
				occp |= (uint64(1) << uint(to))

				if b.isSquareAttackedWithOcc(to, Color(them), occp) {
					continue
				}

				var cap Piece
				if isCap {
					cap = b.pieces[to]
				} else {
					cap = NoPiece
				}
				moves = append(moves, NewMove(fromSq, Square(to), movedPiece, cap, NoPiece, FlagNone))
			}

			// Castling candidates
			occ := allOcc
			if side == White {
				// King side: e1 to g1 (4->6)
				if b.castlingRights&CastlingWhiteK != 0 {
					if b.pieces[5] == NoPiece && b.pieces[6] == NoPiece && b.pieces[7] == WhiteRook &&
						!inCheck && !b.isSquareAttackedWithOcc(5, Black, occ) && !b.isSquareAttackedWithOcc(6, Black, occ) {
						if filter != genCaptures {
							moves = append(moves, NewMove(4, 6, WhiteKing, NoPiece, NoPiece, FlagCastle))
						}
					}
				}
				// Queen side: e1 to c1 (4->2)
				if b.castlingRights&CastlingWhiteQ != 0 {
					if b.pieces[1] == NoPiece && b.pieces[2] == NoPiece && b.pieces[3] == NoPiece && b.pieces[0] == WhiteRook &&
						!inCheck && !b.isSquareAttackedWithOcc(3, Black, occ) && !b.isSquareAttackedWithOcc(2, Black, occ) {
						if filter != genCaptures {
							moves = append(moves, NewMove(4, 2, WhiteKing, NoPiece, NoPiece, FlagCastle))
						}
					}
				}
			} else {
				// Black
				// King side: e8 to g8 (60->62)
				if b.castlingRights&CastlingBlackK != 0 {
					if b.pieces[61] == NoPiece && b.pieces[62] == NoPiece && b.pieces[63] == BlackRook &&
						!inCheck && !b.isSquareAttackedWithOcc(61, White, occ) && !b.isSquareAttackedWithOcc(62, White, occ) {
						if filter != genCaptures {
							moves = append(moves, NewMove(60, 62, BlackKing, NoPiece, NoPiece, FlagCastle))
						}
					}
				}
				// Queen side: e8 to c8 (60->58)
				if b.castlingRights&CastlingBlackQ != 0 {
					if b.pieces[57] == NoPiece && b.pieces[58] == NoPiece && b.pieces[59] == NoPiece && b.pieces[56] == BlackRook &&
						!inCheck && !b.isSquareAttackedWithOcc(59, White, occ) && !b.isSquareAttackedWithOcc(58, White, occ) {
						if filter != genCaptures {
							moves = append(moves, NewMove(60, 58, BlackKing, NoPiece, NoPiece, FlagCastle))
						}
					}
				}
			}
		}
	}

	return moves
}

// GenerateMoves generates all legal moves for the current side to move.
// It allocates a new slice; prefer GenerateMovesInto to reuse buffers in hot paths.
func (b *Board) GenerateMoves() []Move { return b.GenerateMovesInto(make([]Move, 0, 128)) }

// GenerateMovesInto appends all legal moves for the side to move into dst and returns it.
// The dst slice is truncated (len=0) and reused to avoid allocations when capacity suffices.
func (b *Board) GenerateMovesInto(dst []Move) []Move {
	return b.generateMovesFilteredInto(dst, genAll)
}

// GenerateCapturesInto appends all legal captures (including en passant and capture promotions).
func (b *Board) GenerateCapturesInto(dst []Move) []Move {
	return b.generateMovesFilteredInto(dst, genCaptures)
}

// GenerateQuietsInto appends all legal non-capturing moves (includes non-capturing promotions and castling).
func (b *Board) GenerateQuietsInto(dst []Move) []Move {
	return b.generateMovesFilteredInto(dst, genQuiets)
}

// GenerateCaptures returns a newly allocated slice of legal capture moves.
func (b *Board) GenerateCaptures() []Move { return b.GenerateCapturesInto(make([]Move, 0, 128)) }

// GenerateQuiets returns a newly allocated slice of legal non-capturing moves.
func (b *Board) GenerateQuiets() []Move { return b.GenerateQuietsInto(make([]Move, 0, 128)) }

// GenerateChecksInto appends all legal checking moves (moves that give check) into dst and returns it.
// Implementation: generate legal moves then filter by making the move and checking opponent king safety.
func (b *Board) GenerateChecksInto(dst []Move) []Move {
	// Generate all legal moves into dst
	moves := b.GenerateMovesInto(dst)
	if len(moves) == 0 {
		return moves[:0]
	}

	us := int(b.sideToMove)
	them := 1 - us
	occ := b.AllOccupancy()
	kbb := b.kings[them]
	if kbb == 0 {
		return moves[:0]
	}
	ks := bits.TrailingZeros64(kbb)
	kBit := uint64(1) << uint(ks)
	rq := b.rooks[us] | b.queens[us]
	bq := b.bishops[us] | b.queens[us]

	// In-place filter
	out := moves[:0]
	for _, m := range moves {
		from := int(m.From())
		to := int(m.To())
		moved := m.MovedPiece()
		cap := m.CapturedPiece()
		promo := m.PromotionPiece()
		flag := m.Flags()

		// Build temporary occupancy after the move
		fromBB := uint64(1) << uint(from)
		toBB := uint64(1) << uint(to)
		occp := occ &^ fromBB

		if flag == FlagEnPassant {
			var capSq int
			if b.sideToMove == White {
				capSq = to - 8
			} else {
				capSq = to + 8
			}
			occp &^= (uint64(1) << uint(capSq))
			occp |= toBB
		} else {
			// Normal move/capture/promotion/castling: piece ends on 'to'
			// (If capture, destination was already occupied; leaving it set is correct.)
			_ = cap // capture presence does not change occupancy bit at 'to' after the move
			occp |= toBB

			// Adjust rook for castling
			if flag == FlagCastle {
				if b.sideToMove == White {
					if to == 6 { // e1->g1, rook h1->f1
						occp &^= (uint64(1) << 7)
						occp |= (uint64(1) << 5)
					} else if to == 2 { // e1->c1, rook a1->d1
						occp &^= (uint64(1) << 0)
						occp |= (uint64(1) << 3)
					}
				} else {
					if to == 62 { // e8->g8, rook h8->f8
						occp &^= (uint64(1) << 63)
						occp |= (uint64(1) << 61)
					} else if to == 58 { // e8->c8, rook a8->d8
						occp &^= (uint64(1) << 56)
						occp |= (uint64(1) << 59)
					}
				}
			}
		}

		// Direct checking by the piece that lands on 'to'
		dpiece := moved
		if promo != NoPiece {
			dpiece = promo
		}

		gives := false
		switch typeOf(dpiece) {
		case 1: // pawn
			if b.sideToMove == White {
				gives = (pawnAttacks[White][to] & kBit) != 0
			} else {
				gives = (pawnAttacks[Black][to] & kBit) != 0
			}
		case 2: // knight
			gives = (knightMoves[to] & kBit) != 0
		case 3: // bishop
			gives = (bishopAttacksMagic(to, occp) & kBit) != 0
		case 4: // rook
			gives = (rookAttacksMagic(to, occp) & kBit) != 0
		case 5: // queen
			gives = ((rookAttacksMagic(to, occp) | bishopAttacksMagic(to, occp)) & kBit) != 0
		case 6: // king
			gives = (kingMoves[to] & kBit) != 0
		}

		// Castling: the rook may give check from its post-castle square
		if !gives && flag == FlagCastle {
			rTo := -1
			if b.sideToMove == White {
				if to == 6 {
					rTo = 5
				} else if to == 2 {
					rTo = 3
				}
			} else {
				if to == 62 {
					rTo = 61
				} else if to == 58 {
					rTo = 59
				}
			}
			if rTo >= 0 {
				if (rookAttacksMagic(rTo, occp) & kBit) != 0 {
					gives = true
				}
			}
		}

		// Discovered check: after the move, do our sliders now attack the enemy king?
		if !gives {
			if (rookAttacksMagic(ks, occp)&rq) != 0 || (bishopAttacksMagic(ks, occp)&bq) != 0 {
				gives = true
			}
		}

		if gives {
			out = append(out, m)
		}
	}
	return out
}

// GenerateChecks returns a newly allocated slice of legal checking moves.
func (b *Board) GenerateChecks() []Move { return b.GenerateChecksInto(make([]Move, 0, 128)) }

// GeneratePseudoMoves generates moves without the final make/unmake legality filter.
// It still enforces basic structural rules (no own-occupancy, blockers, and castling path emptiness),
// but it does not test whether the mover is in check before/after the move.
// GeneratePseudoMovesInto appends all pseudo-legal moves (no king-safety filtering) into dst and returns it.
// Pseudo-legal obeys piece rules and blockers; castling requires rights and empty path but ignores attack-on-path.
func (b *Board) GeneratePseudoMovesInto(dst []Move) []Move {
	moves := dst[:0]
	side := b.sideToMove
	us := int(side)
	them := 1 - us

	ownOcc := b.occupancy[us]
	oppOcc := b.occupancy[them]
	allOcc := ownOcc | oppOcc

	appendMove := func(m Move) { moves = append(moves, m) }

	// Pawns
	pawns := b.pawns[us]
	for pawns != 0 {
		from := popLSB(&pawns)
		fromSq := Square(from)
		movedPiece := b.pieces[from]

		if side == White {
			one := from + 8
			if one < 64 && ((allOcc>>uint(one))&1) == 0 {
				if one/8 == 7 {
					appendMove(NewMove(fromSq, Square(one), movedPiece, NoPiece, WhiteQueen, FlagNone))
					appendMove(NewMove(fromSq, Square(one), movedPiece, NoPiece, WhiteRook, FlagNone))
					appendMove(NewMove(fromSq, Square(one), movedPiece, NoPiece, WhiteBishop, FlagNone))
					appendMove(NewMove(fromSq, Square(one), movedPiece, NoPiece, WhiteKnight, FlagNone))
				} else {
					appendMove(NewMove(fromSq, Square(one), movedPiece, NoPiece, NoPiece, FlagNone))
					if from/8 == 1 {
						two := from + 16
						if ((allOcc >> uint(two)) & 1) == 0 {
							appendMove(NewMove(fromSq, Square(two), movedPiece, NoPiece, NoPiece, FlagNone))
						}
					}
				}
			}

			caps := pawnAttacks[White][from]
			capTargets := caps & oppOcc
			for capTargets != 0 {
				to := popLSB(&capTargets)
				toSq := Square(to)
				capPiece := b.pieces[to]
				if to/8 == 7 {
					appendMove(NewMove(fromSq, toSq, movedPiece, capPiece, WhiteQueen, FlagNone))
					appendMove(NewMove(fromSq, toSq, movedPiece, capPiece, WhiteRook, FlagNone))
					appendMove(NewMove(fromSq, toSq, movedPiece, capPiece, WhiteBishop, FlagNone))
					appendMove(NewMove(fromSq, toSq, movedPiece, capPiece, WhiteKnight, FlagNone))
				} else {
					appendMove(NewMove(fromSq, toSq, movedPiece, capPiece, NoPiece, FlagNone))
				}
			}
			if b.enPassantSquare != NoSquare {
				ep := int(b.enPassantSquare)
				if (caps & (1 << uint(ep))) != 0 {
					appendMove(NewMove(fromSq, Square(ep), movedPiece, BlackPawn, NoPiece, FlagEnPassant))
				}
			}
		} else {
			one := from - 8
			if one >= 0 && ((allOcc>>uint(one))&1) == 0 {
				if one/8 == 0 {
					appendMove(NewMove(fromSq, Square(one), movedPiece, NoPiece, BlackQueen, FlagNone))
					appendMove(NewMove(fromSq, Square(one), movedPiece, NoPiece, BlackRook, FlagNone))
					appendMove(NewMove(fromSq, Square(one), movedPiece, NoPiece, BlackBishop, FlagNone))
					appendMove(NewMove(fromSq, Square(one), movedPiece, NoPiece, BlackKnight, FlagNone))
				} else {
					appendMove(NewMove(fromSq, Square(one), movedPiece, NoPiece, NoPiece, FlagNone))
					if from/8 == 6 {
						two := from - 16
						if ((allOcc >> uint(two)) & 1) == 0 {
							appendMove(NewMove(fromSq, Square(two), movedPiece, NoPiece, NoPiece, FlagNone))
						}
					}
				}
			}

			caps := pawnAttacks[Black][from]
			capTargets := caps & oppOcc
			for capTargets != 0 {
				to := popLSB(&capTargets)
				toSq := Square(to)
				capPiece := b.pieces[to]
				if to/8 == 0 {
					appendMove(NewMove(fromSq, toSq, movedPiece, capPiece, BlackQueen, FlagNone))
					appendMove(NewMove(fromSq, toSq, movedPiece, capPiece, BlackRook, FlagNone))
					appendMove(NewMove(fromSq, toSq, movedPiece, capPiece, BlackBishop, FlagNone))
					appendMove(NewMove(fromSq, toSq, movedPiece, capPiece, BlackKnight, FlagNone))
				} else {
					appendMove(NewMove(fromSq, toSq, movedPiece, capPiece, NoPiece, FlagNone))
				}
			}
			if b.enPassantSquare != NoSquare {
				ep := int(b.enPassantSquare)
				if (caps & (1 << uint(ep))) != 0 {
					appendMove(NewMove(fromSq, Square(ep), movedPiece, WhitePawn, NoPiece, FlagEnPassant))
				}
			}
		}
	}

	// Knights
	knights := b.knights[us]
	for knights != 0 {
		from := popLSB(&knights)
		fromSq := Square(from)
		movedPiece := b.pieces[from]
		targets := knightMoves[from] &^ ownOcc
		for t := targets; t != 0; {
			to := popLSB(&t)
			var cap Piece = NoPiece
			if ((oppOcc >> uint(to)) & 1) != 0 {
				cap = b.pieces[to]
			}
			appendMove(NewMove(fromSq, Square(to), movedPiece, cap, NoPiece, FlagNone))
		}
	}

	// Bishops
	bishops := b.bishops[us]
	for bishops != 0 {
		from := popLSB(&bishops)
		fromSq := Square(from)
		movedPiece := b.pieces[from]
		targets := bishopAttacksMagic(from, allOcc) &^ ownOcc
		for t := targets; t != 0; {
			to := popLSB(&t)
			var cap Piece = NoPiece
			if ((oppOcc >> uint(to)) & 1) != 0 {
				cap = b.pieces[to]
			}
			appendMove(NewMove(fromSq, Square(to), movedPiece, cap, NoPiece, FlagNone))
		}
	}

	// Rooks
	rooks := b.rooks[us]
	for rooks != 0 {
		from := popLSB(&rooks)
		fromSq := Square(from)
		movedPiece := b.pieces[from]
		targets := rookAttacksMagic(from, allOcc) &^ ownOcc
		for t := targets; t != 0; {
			to := popLSB(&t)
			var cap Piece = NoPiece
			if ((oppOcc >> uint(to)) & 1) != 0 {
				cap = b.pieces[to]
			}
			appendMove(NewMove(fromSq, Square(to), movedPiece, cap, NoPiece, FlagNone))
		}
	}

	// Queens
	queens := b.queens[us]
	for queens != 0 {
		from := popLSB(&queens)
		fromSq := Square(from)
		movedPiece := b.pieces[from]
		targets := (rookAttacksMagic(from, allOcc) | bishopAttacksMagic(from, allOcc)) &^ ownOcc
		for t := targets; t != 0; {
			to := popLSB(&t)
			var cap Piece = NoPiece
			if ((oppOcc >> uint(to)) & 1) != 0 {
				cap = b.pieces[to]
			}
			appendMove(NewMove(fromSq, Square(to), movedPiece, cap, NoPiece, FlagNone))
		}
	}

	// King
	kingBB := b.kings[us]
	if kingBB != 0 {
		from := bits.TrailingZeros64(kingBB)
		if from >= 0 {
			fromSq := Square(from)
			movedPiece := b.pieces[from]
			targets := kingMoves[from] &^ ownOcc
			for t := targets; t != 0; {
				to := popLSB(&t)
				cap := b.pieces[to]
				appendMove(NewMove(fromSq, Square(to), movedPiece, cap, NoPiece, FlagNone))
			}

			// Castling (path + rights), no in-check checks here
			if side == White {
				if b.castlingRights&CastlingWhiteK != 0 {
					if b.pieces[5] == NoPiece && b.pieces[6] == NoPiece && b.pieces[7] == WhiteRook {
						appendMove(NewMove(4, 6, WhiteKing, NoPiece, NoPiece, FlagCastle))
					}
				}
				if b.castlingRights&CastlingWhiteQ != 0 {
					if b.pieces[1] == NoPiece && b.pieces[2] == NoPiece && b.pieces[3] == NoPiece && b.pieces[0] == WhiteRook {
						appendMove(NewMove(4, 2, WhiteKing, NoPiece, NoPiece, FlagCastle))
					}
				}
			} else {
				if b.castlingRights&CastlingBlackK != 0 {
					if b.pieces[61] == NoPiece && b.pieces[62] == NoPiece && b.pieces[63] == BlackRook {
						appendMove(NewMove(60, 62, BlackKing, NoPiece, NoPiece, FlagCastle))
					}
				}
				if b.castlingRights&CastlingBlackQ != 0 {
					if b.pieces[57] == NoPiece && b.pieces[58] == NoPiece && b.pieces[59] == NoPiece && b.pieces[56] == BlackRook {
						appendMove(NewMove(60, 58, BlackKing, NoPiece, NoPiece, FlagCastle))
					}
				}
			}
		}
	}

	return moves
}

// GeneratePseudoMoves returns all pseudo-legal moves (allocates a new slice).
func (b *Board) GeneratePseudoMoves() []Move { return b.GeneratePseudoMovesInto(make([]Move, 0, 128)) }

// GenerateLegalMoves exposes the same API name as dragontoothmg for legal move generation.
func (b *Board) GenerateLegalMoves() []Move { return b.GenerateMoves() }

// CalculateRookMoveBitboard returns rook attacks from the given square for the supplied occupancy mask.
func CalculateRookMoveBitboard(square uint8, occupancy uint64) uint64 {
	return rookAttacksMagic(int(square), occupancy)
}

// CalculateBishopMoveBitboard returns bishop attacks from the given square for the supplied occupancy mask.
func CalculateBishopMoveBitboard(square uint8, occupancy uint64) uint64 {
	return bishopAttacksMagic(int(square), occupancy)
}

// Perft counts leaf nodes (move sequences) from the position for a given depth.
// Perft counts leaf nodes (move sequences) from the position for a given depth.
// Optimized to reuse per-depth buffers to avoid allocations.
func Perft(b *Board, depth int) uint64 {
	if depth == 0 {
		return 1
	}

	// Prepare a small pool of per-depth buffers
	pc := perftCtx{bufs: make([][]Move, depth+1)}
	return perftRec(b, depth, &pc)
}

type perftCtx struct {
	bufs [][]Move
}

func (pc *perftCtx) bufFor(depth int) []Move {
	if depth < 0 {
		depth = 0
	}
	if depth >= len(pc.bufs) {
		pc.bufs = append(pc.bufs, nil)
	}
	buf := pc.bufs[depth]
	if buf == nil {
		buf = make([]Move, 0, 256)
		pc.bufs[depth] = buf
	}
	return buf[:0]
}

func perftRec(b *Board, depth int, pc *perftCtx) uint64 {
	if depth == 0 {
		return 1
	}
	var nodes uint64
	moves := b.GenerateMovesInto(pc.bufFor(depth))
	for _, m := range moves {
		if ok, st := b.MakeMove(m); ok {
			nodes += perftRec(b, depth-1, pc)
			b.UnmakeMove(m, st)
		}
	}
	return nodes
}

// PerftDivide returns a map from each legal root move to the number of leaf nodes
// reachable from that move at the given depth. Useful for debugging.
func PerftDivide(b *Board, depth int) map[Move]uint64 {
	result := make(map[Move]uint64)
	if depth <= 0 {
		return result
	}
	moves := b.GenerateMoves()
	for _, m := range moves {
		if ok, st := b.MakeMove(m); ok {
			cnt := Perft(b, depth-1)
			b.UnmakeMove(m, st)
			result[m] = cnt
		}
	}
	return result
}
