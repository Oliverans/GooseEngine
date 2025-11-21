package engine

import (
	gm "chess-engine/goosemg"
	"math/bits"
)

// PawnStructDiffs computes unit feature differences (white minus black or vice-versa as noted)
// for pawn-structure terms, separately for MG and EG, using the engine's bitboard helpers and masks.
// The order of features per phase is:
//
//	0: Doubled      (diff = bCount - wCount)
//	1: Isolated     (diff = bCount - wCount)
//	2: Connected    (diff = wCount - bCount)
//	3: Phalanx      (diff = wCount - bCount)
//	4: Blocked      (diff = wCount - bCount)  // advanced blocked pawns
//	5: PawnLever    (diff = wCount - bCount)
//	6: WeakLever    (diff = bCount - wCount)  // unsupported multi-lever pawn
//	7: Backward     (diff = bCount - wCount)
//
// MG and EG vectors differ for Connected/Phalanx (EG uses endgame masks); others are same counts.
func PawnStructDiffs(b *gm.Board) (mg [8]int, eg [8]int) {
	// Doubled counts per file
	wp, bp := b.White.Pawns, b.Black.Pawns
	var wDoub, bDoub int
	for f := 0; f < 8; f++ {
		wn := bits.OnesCount64(wp & onlyFile[f])
		bn := bits.OnesCount64(bp & onlyFile[f])
		if wn > 1 {
			wDoub += wn - 1
		}
		if bn > 1 {
			bDoub += bn - 1
		}
	}
	doubDiff := bDoub - wDoub
	mg[0], eg[0] = doubDiff, doubDiff

	// Isolated: count pawns with no friendly pawns on adjacent files (same-file ignored)
	wIso, bIso := 0, 0
	for x := wp; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		f := sq & 7
		var adj uint64
		if f > 0 {
			adj |= onlyFile[f-1]
		}
		if f < 7 {
			adj |= onlyFile[f+1]
		}
		if (wp & adj) == 0 {
			wIso++
		}
	}
	for x := bp; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		f := sq & 7
		var adj uint64
		if f > 0 {
			adj |= onlyFile[f-1]
		}
		if f < 7 {
			adj |= onlyFile[f+1]
		}
		if (bp & adj) == 0 {
			bIso++
		}
	}
	isoDiff := bIso - wIso
	mg[1], eg[1] = isoDiff, isoDiff

	// Pawn attack maps
	wPawnAttackBB := ((wp &^ bitboardFileA) << 7) | ((wp &^ bitboardFileH) << 9)
	bPawnAttackBB := ((bp &^ bitboardFileH) >> 7) | ((bp &^ bitboardFileA) >> 9)

	// Connected pawns (unit counts):
	// MG: pawns defended by a pawn (i.e., on pawn attack map)
	wConnMG := bits.OnesCount64(b.White.Pawns & wPawnAttackBB)
	bConnMG := bits.OnesCount64(b.Black.Pawns & bPawnAttackBB)
	mg[2] = wConnMG - bConnMG
	// EG: same, but exclude invalid endgame squares per engine masks
	wConnEG := bits.OnesCount64((b.White.Pawns & wPawnAttackBB) &^ wPhalanxOrConnectedEndgameInvalidSquares)
	bConnEG := bits.OnesCount64((b.Black.Pawns & bPawnAttackBB) &^ bPhalanxOrConnectedEndgameInvalidSquares)
	eg[2] = wConnEG - bConnEG

	// Phalanx pawns (unit counts): adjacent pawns on same rank, both ends counted
	wEast := (b.White.Pawns &^ bitboardFileH) << 1
	wWest := (b.White.Pawns &^ bitboardFileA) >> 1
	bEast := (b.Black.Pawns &^ bitboardFileH) << 1
	bWest := (b.Black.Pawns &^ bitboardFileA) >> 1
	wPh := (b.White.Pawns & wEast) | (b.White.Pawns & wWest)
	bPh := (b.Black.Pawns & bEast) | (b.Black.Pawns & bWest)
	wPhCntMG := bits.OnesCount64(wPh &^ secondRankMask)
	bPhCntMG := bits.OnesCount64(bPh &^ seventhRankMask)
	mg[3] = wPhCntMG - bPhCntMG
	// EG uses same counts (engine applies same rank exclusions)
	eg[3] = mg[3]

	// Blocked advanced pawns via engine helper (own advanced pawns blocked)
	wBlkBB, bBlkBB := getBlockedPawnsBitboards(b)
	blkDiff := bits.OnesCount64(wBlkBB) - bits.OnesCount64(bBlkBB)
	mg[4], eg[4] = blkDiff, blkDiff

	// Pawn levers: enemy pawns hitting the square in front of our pawn
	wLeverBB, bLeverBB, wMultiLever, bMultiLever := getPawnLeverBitboards(b)
	wLeverCnt := bits.OnesCount64(wLeverBB)
	bLeverCnt := bits.OnesCount64(bLeverBB)
	leverDiff := wLeverCnt - bLeverCnt
	mg[5], eg[5] = leverDiff, leverDiff

	// Weak levers: multi-lever targets that lack pawn support
	wSupported := wPawnAttackBB & b.White.Pawns
	bSupported := bPawnAttackBB & b.Black.Pawns
	wWeak := wMultiLever &^ wSupported
	bWeak := bMultiLever &^ bSupported
	weakDiff := bits.OnesCount64(bWeak) - bits.OnesCount64(wWeak)
	mg[6], eg[6] = weakDiff, weakDiff

	// Backward pawns via engine helper
	// Need isolated and passed bitboards to determine backwardness
	wIsoBB, bIsoBB := getIsolatedPawnsBitboards(b)
	wPassedBB, bPassedBB := getPassedPawnsBitboards(b, wPawnAttackBB, bPawnAttackBB)
	wBackBB, bBackBB := getBackwardPawnsBitboards(b, wPawnAttackBB, bPawnAttackBB, wIsoBB, bIsoBB, wPassedBB, bPassedBB)
	backDiff := bits.OnesCount64(bBackBB) - bits.OnesCount64(wBackBB)
	mg[7], eg[7] = backDiff, backDiff
	return mg, eg
}

// ImbalanceDiffs exposes the unit contributions (before applying engine/tuner scalars)
// for the Kaufman-style material imbalance terms used in evaluation.go. Index mapping:
//
//	0: KnightPerPawn
//	1: BishopPerPawn
//	2: MinorsForMajor
//	3: RedundantRook
//	4: RookQueenOverlap
//	5: QueenManyMinors
func ImbalanceDiffs(b *gm.Board) (mg [6]int, eg [6]int) {
	pieceCount := countPieceTypes(b)
	const White = 0
	const Black = 1
	wp := pieceCount[White][gm.PieceTypePawn]
	wn := pieceCount[White][gm.PieceTypeKnight]
	wb := pieceCount[White][gm.PieceTypeBishop]
	wr := pieceCount[White][gm.PieceTypeRook]
	wq := pieceCount[White][gm.PieceTypeQueen]

	bp := pieceCount[Black][gm.PieceTypePawn]
	bn := pieceCount[Black][gm.PieceTypeKnight]
	bb := pieceCount[Black][gm.PieceTypeBishop]
	br := pieceCount[Black][gm.PieceTypeRook]
	bq := pieceCount[Black][gm.PieceTypeQueen]

	wPawnDelta := wp - ImbalanceRefPawnCount
	bPawnDelta := bp - ImbalanceRefPawnCount

	// Knight/Bishop per pawn
	mg[0] = (wPawnDelta * wn) - (bPawnDelta * bn)
	eg[0] = mg[0]
	mg[1] = (wPawnDelta * wb) - (bPawnDelta * bb)
	eg[1] = mg[1]

	totalPawns := wp + bp
	wMinors := wn + wb
	bMinors := bn + bb

	if totalPawns >= 11 && (wq+bq) > 0 {
		if wr > br && wMinors < bMinors {
			out := bMinors - wMinors
			mg[2] += out
			eg[2] += out
		}
		if br > wr && bMinors < wMinors {
			out := wMinors - bMinors
			mg[2] -= out
			eg[2] -= out
		}
	}

	if wr > 1 {
		extra := wr - 1
		mg[3] -= extra
		eg[3] -= extra
	}
	if br > 1 {
		extra := br - 1
		mg[3] += extra
		eg[3] += extra
	}

	if wq >= 1 && wr >= 2 {
		mg[4] -= wr
		eg[4] -= wr
	}
	if bq >= 1 && br >= 2 {
		mg[4] += br
		eg[4] += br
	}

	if wq > 0 && wMinors >= 3 {
		extra := wMinors - 2
		mg[5] -= extra
		eg[5] -= extra
	}
	if bq > 0 && bMinors >= 3 {
		extra := bMinors - 2
		mg[5] += extra
		eg[5] += extra
	}
	return mg, eg
}

// MobAtkDiffs computes:
// - mobility counts per piece type for MG/EG (white minus black)
// - attacker counts on opponent king inner/outer zones per piece type (white minus black)
// Returns four [7]int arrays keyed by gm.PieceType.
func MobAtkDiffs(b *gm.Board) (mobMG [7]int, mobEG [7]int, attInner [7]int, attOuter [7]int) {
	// Pawn attack maps
	wp, bp := b.White.Pawns, b.Black.Pawns
	wPawnAttackBB := ((wp &^ bitboardFileA) << 7) | ((wp &^ bitboardFileH) << 9)
	bPawnAttackBB := ((bp &^ bitboardFileH) >> 7) | ((bp &^ bitboardFileA) >> 9)

	// King zones
	innerZones := getKingSafetyTable(b, true, wPawnAttackBB, bPawnAttackBB)
	outerZones := getKingSafetyTable(b, false, wPawnAttackBB, bPawnAttackBB)

	all := b.White.All | b.Black.All

	// Helpers to add counts
	addMob := func(pt gm.PieceType, white bool, cntMG, cntEG int) {
		idx := int(pt)
		if white {
			mobMG[idx] += cntMG
			mobEG[idx] += cntEG
		} else {
			mobMG[idx] -= cntMG
			mobEG[idx] -= cntEG
		}
	}
	addAtk := func(pt gm.PieceType, white bool, innerCnt, outerCnt int) {
		idx := int(pt)
		if white {
			attInner[idx] += innerCnt
			attOuter[idx] += outerCnt
		} else {
			attInner[idx] -= innerCnt
			attOuter[idx] -= outerCnt
		}
	}

	// Pawn attacks (attackers only; mobility uses 0 for pawns)
	// White pawns attack black king zones
	wInner := bits.OnesCount64(wPawnAttackBB & innerZones[1])
	wOuter := bits.OnesCount64(wPawnAttackBB & outerZones[1])
	addAtk(gm.PieceTypePawn, true, wInner, wOuter)
	// Black pawns attack white king zones
	bInner := bits.OnesCount64(bPawnAttackBB & innerZones[0])
	bOuter := bits.OnesCount64(bPawnAttackBB & outerZones[0])
	addAtk(gm.PieceTypePawn, false, bInner, bOuter)

	// Knights
	for x := b.White.Knights; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		attacked := KnightMasks[sq]
		mobSquares := attacked &^ bPawnAttackBB &^ b.White.All
		cnt := bits.OnesCount64(mobSquares)
		addMob(gm.PieceTypeKnight, true, cnt, cnt)
		addAtk(gm.PieceTypeKnight, true,
			bits.OnesCount64(attacked&innerZones[1]),
			bits.OnesCount64(attacked&outerZones[1]))
	}
	for x := b.Black.Knights; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		attacked := KnightMasks[sq]
		mobSquares := attacked &^ wPawnAttackBB &^ b.Black.All
		cnt := bits.OnesCount64(mobSquares)
		addMob(gm.PieceTypeKnight, false, cnt, cnt)
		addAtk(gm.PieceTypeKnight, false,
			bits.OnesCount64(attacked&innerZones[0]),
			bits.OnesCount64(attacked&outerZones[0]))
	}

	// Bishops
	for x := b.White.Bishops; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		attacked := gm.CalculateBishopMoveBitboard(uint8(sq), all&^PositionBB[sq])
		mobSquares := attacked &^ bPawnAttackBB &^ b.White.All
		cnt := bits.OnesCount64(mobSquares)
		addMob(gm.PieceTypeBishop, true, cnt, cnt)
		addAtk(gm.PieceTypeBishop, true,
			bits.OnesCount64(attacked&innerZones[1]),
			bits.OnesCount64(attacked&outerZones[1]))
	}
	for x := b.Black.Bishops; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		attacked := gm.CalculateBishopMoveBitboard(uint8(sq), all&^PositionBB[sq])
		mobSquares := attacked &^ wPawnAttackBB &^ b.Black.All
		cnt := bits.OnesCount64(mobSquares)
		addMob(gm.PieceTypeBishop, false, cnt, cnt)
		addAtk(gm.PieceTypeBishop, false,
			bits.OnesCount64(attacked&innerZones[0]),
			bits.OnesCount64(attacked&outerZones[0]))
	}

	// Rooks
	for x := b.White.Rooks; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		attacked := gm.CalculateRookMoveBitboard(uint8(sq), all&^PositionBB[sq])
		mobSquares := attacked &^ bPawnAttackBB &^ b.White.All
		cnt := bits.OnesCount64(mobSquares)
		addMob(gm.PieceTypeRook, true, cnt, cnt)
		addAtk(gm.PieceTypeRook, true,
			bits.OnesCount64(attacked&innerZones[1]),
			bits.OnesCount64(attacked&outerZones[1]))
	}
	for x := b.Black.Rooks; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		attacked := gm.CalculateRookMoveBitboard(uint8(sq), all&^PositionBB[sq])
		mobSquares := attacked &^ wPawnAttackBB &^ b.Black.All
		cnt := bits.OnesCount64(mobSquares)
		addMob(gm.PieceTypeRook, false, cnt, cnt)
		addAtk(gm.PieceTypeRook, false,
			bits.OnesCount64(attacked&innerZones[0]),
			bits.OnesCount64(attacked&outerZones[0]))
	}

	// Queens
	for x := b.White.Queens; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		rook := gm.CalculateRookMoveBitboard(uint8(sq), all&^PositionBB[sq])
		bishop := gm.CalculateBishopMoveBitboard(uint8(sq), all&^PositionBB[sq])
		attacked := rook | bishop
		mobSquares := attacked &^ bPawnAttackBB &^ b.White.All
		cnt := bits.OnesCount64(mobSquares)
		addMob(gm.PieceTypeQueen, true, cnt, cnt)
		addAtk(gm.PieceTypeQueen, true,
			bits.OnesCount64(attacked&innerZones[1]),
			bits.OnesCount64(attacked&outerZones[1]))
	}
	for x := b.Black.Queens; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		rook := gm.CalculateRookMoveBitboard(uint8(sq), all&^PositionBB[sq])
		bishop := gm.CalculateBishopMoveBitboard(uint8(sq), all&^PositionBB[sq])
		attacked := rook | bishop
		mobSquares := attacked &^ wPawnAttackBB &^ b.Black.All
		cnt := bits.OnesCount64(mobSquares)
		addMob(gm.PieceTypeQueen, false, cnt, cnt)
		addAtk(gm.PieceTypeQueen, false,
			bits.OnesCount64(attacked&innerZones[0]),
			bits.OnesCount64(attacked&outerZones[0]))
	}

	// Kings (attackers only; mobility weights are 0 by default)
	if b.White.Kings != 0 {
		sq := bits.TrailingZeros64(b.White.Kings)
		attacked := KingMoves[sq]
		addAtk(gm.PieceTypeKing, true,
			bits.OnesCount64(attacked&innerZones[1]),
			bits.OnesCount64(attacked&outerZones[1]))
	}
	if b.Black.Kings != 0 {
		sq := bits.TrailingZeros64(b.Black.Kings)
		attacked := KingMoves[sq]
		addAtk(gm.PieceTypeKing, false,
			bits.OnesCount64(attacked&innerZones[0]),
			bits.OnesCount64(attacked&outerZones[0]))
	}

	return mobMG, mobEG, attInner, attOuter
}

// KingSafetyOneHot returns a one-hot diff vector over KingSafetyTable indices [0..99],
// equal to +1 at the white attack-unit count index and -1 at the black index.
// Attack-unit counts are computed using engine attackerInner/attackerOuter weights.
func KingSafetyOneHot(b *gm.Board) (onehot [100]int) {
	// Pawn attack maps
	wp, bp := b.White.Pawns, b.Black.Pawns
	wPawnAttackBB := ((wp &^ bitboardFileA) << 7) | ((wp &^ bitboardFileH) << 9)
	bPawnAttackBB := ((bp &^ bitboardFileH) >> 7) | ((bp &^ bitboardFileA) >> 9)

	inner := getKingSafetyTable(b, true, wPawnAttackBB, bPawnAttackBB)
	outer := getKingSafetyTable(b, false, 0, 0)
	all := b.White.All | b.Black.All

	var attackUnit [2]int

	add := func(white bool, innerCnt, outerCnt, innerW, outerW int) {
		u := innerCnt*innerW + outerCnt*outerW
		if white {
			attackUnit[0] += u
		} else {
			attackUnit[1] += u
		}
	}

	// Pawns
	wi := bits.OnesCount64(wPawnAttackBB & inner[1])
	wo := bits.OnesCount64(wPawnAttackBB & outer[1])
	bi := bits.OnesCount64(bPawnAttackBB & inner[0])
	bo := bits.OnesCount64(bPawnAttackBB & outer[0])
	add(true, wi, wo, attackerInner[gm.PieceTypePawn], attackerOuter[gm.PieceTypePawn])
	add(false, bi, bo, attackerInner[gm.PieceTypePawn], attackerOuter[gm.PieceTypePawn])

	// Knights
	for x := b.White.Knights; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		atk := KnightMasks[sq]
		add(true,
			bits.OnesCount64(atk&inner[1]),
			bits.OnesCount64(atk&outer[1]),
			attackerInner[gm.PieceTypeKnight], attackerOuter[gm.PieceTypeKnight])
	}
	for x := b.Black.Knights; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		atk := KnightMasks[sq]
		add(false,
			bits.OnesCount64(atk&inner[0]),
			bits.OnesCount64(atk&outer[0]),
			attackerInner[gm.PieceTypeKnight], attackerOuter[gm.PieceTypeKnight])
	}
	// Bishops
	for x := b.White.Bishops; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		atk := gm.CalculateBishopMoveBitboard(uint8(sq), all&^PositionBB[sq])
		add(true,
			bits.OnesCount64(atk&inner[1]),
			bits.OnesCount64(atk&outer[1]),
			attackerInner[gm.PieceTypeBishop], attackerOuter[gm.PieceTypeBishop])
	}
	for x := b.Black.Bishops; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		atk := gm.CalculateBishopMoveBitboard(uint8(sq), all&^PositionBB[sq])
		add(false,
			bits.OnesCount64(atk&inner[0]),
			bits.OnesCount64(atk&outer[0]),
			attackerInner[gm.PieceTypeBishop], attackerOuter[gm.PieceTypeBishop])
	}
	// Rooks
	for x := b.White.Rooks; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		atk := gm.CalculateRookMoveBitboard(uint8(sq), all&^PositionBB[sq])
		add(true,
			bits.OnesCount64(atk&inner[1]),
			bits.OnesCount64(atk&outer[1]),
			attackerInner[gm.PieceTypeRook], attackerOuter[gm.PieceTypeRook])
	}
	for x := b.Black.Rooks; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		atk := gm.CalculateRookMoveBitboard(uint8(sq), all&^PositionBB[sq])
		add(false,
			bits.OnesCount64(atk&inner[0]),
			bits.OnesCount64(atk&outer[0]),
			attackerInner[gm.PieceTypeRook], attackerOuter[gm.PieceTypeRook])
	}
	// Queens
	for x := b.White.Queens; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		rook := gm.CalculateRookMoveBitboard(uint8(sq), all&^PositionBB[sq])
		bishop := gm.CalculateBishopMoveBitboard(uint8(sq), all&^PositionBB[sq])
		atk := rook | bishop
		add(true,
			bits.OnesCount64(atk&inner[1]),
			bits.OnesCount64(atk&outer[1]),
			attackerInner[gm.PieceTypeQueen], attackerOuter[gm.PieceTypeQueen])
	}
	for x := b.Black.Queens; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		rook := gm.CalculateRookMoveBitboard(uint8(sq), all&^PositionBB[sq])
		bishop := gm.CalculateBishopMoveBitboard(uint8(sq), all&^PositionBB[sq])
		atk := rook | bishop
		add(false,
			bits.OnesCount64(atk&inner[0]),
			bits.OnesCount64(atk&outer[0]),
			attackerInner[gm.PieceTypeQueen], attackerOuter[gm.PieceTypeQueen])
	}

	wc := attackUnit[0]
	bc := attackUnit[1]
	if wc < 0 {
		wc = 0
	}
	if bc < 0 {
		bc = 0
	}
	if wc > 99 {
		wc = 99
	}
	if bc > 99 {
		bc = 99
	}
	onehot[wc] += 1
	onehot[bc] -= 1
	return onehot
}

// KingSafetyCorrelates returns MG-unit diffs for correlated king-safety features:
//
//	semiOpenDiff = (# black king-adjacent semi-open files) - (# white ...)
//	openDiff     = (# black king-adjacent open files) - (# white ...)
//	minorDefDiff = (# white minor (N/B) defenders in inner ring) - (# black ...)
//	pawnDefDiff  = min(3, white pawns in inner ring) - min(3, black ...)
func KingSafetyCorrelates(b *gm.Board) (semiOpenDiff, openDiff, minorDefDiff, pawnDefDiff int) {
	// Build open/semi-open per file via pawn occupancy
	var whiteFiles uint64
	for x := b.White.Pawns; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		whiteFiles |= onlyFile[sq%8]
	}
	var blackFiles uint64
	for x := b.Black.Pawns; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		blackFiles |= onlyFile[sq%8]
	}
	anyPawns := whiteFiles | blackFiles

	// King-adjacent files masks
	wKingFile := onlyFile[bits.TrailingZeros64(b.White.Kings)%8]
	bKingFile := onlyFile[bits.TrailingZeros64(b.Black.Kings)%8]
	wAdjFiles := ((wKingFile & ^bitboardFileA) >> 1) | ((wKingFile & ^bitboardFileH) << 1)
	bAdjFiles := ((bKingFile & ^bitboardFileA) >> 1) | ((bKingFile & ^bitboardFileH) << 1)

	// Count per side
	countSemiOpen := func(adj uint64, ownFiles uint64) int {
		c := 0
		for f := 0; f < 8; f++ {
			fm := onlyFile[f]
			if (adj&fm) != 0 && (ownFiles&fm) == 0 {
				c++
			}
		}
		return c
	}
	countOpen := func(adj uint64) int {
		c := 0
		for f := 0; f < 8; f++ {
			fm := onlyFile[f]
			if (adj&fm) != 0 && (anyPawns&fm) == 0 {
				c++
			}
		}
		return c
	}
	wSemi := countSemiOpen(wAdjFiles, whiteFiles)
	bSemi := countSemiOpen(bAdjFiles, blackFiles)
	wOpen := countOpen(wAdjFiles)
	bOpen := countOpen(bAdjFiles)
	semiOpenDiff = bSemi - wSemi
	openDiff = bOpen - wOpen

	// Inner rings and minor defenders
	wp, bp := b.White.Pawns, b.Black.Pawns
	wPawnAttackBB := ((wp &^ bitboardFileA) << 7) | ((wp &^ bitboardFileH) << 9)
	bPawnAttackBB := ((bp &^ bitboardFileH) >> 7) | ((bp &^ bitboardFileA) >> 9)
	inner := getKingSafetyTable(b, true, wPawnAttackBB, bPawnAttackBB)
	all := b.White.All | b.Black.All

	wDef := 0
	for x := b.White.Knights; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		atk := KnightMasks[sq]
		wDef += bits.OnesCount64(atk & inner[0])
	}
	for x := b.White.Bishops; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		atk := gm.CalculateBishopMoveBitboard(uint8(sq), all&^PositionBB[sq])
		wDef += bits.OnesCount64(atk & inner[0])
	}
	bDef := 0
	for x := b.Black.Knights; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		atk := KnightMasks[sq]
		bDef += bits.OnesCount64(atk & inner[1])
	}
	for x := b.Black.Bishops; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		atk := gm.CalculateBishopMoveBitboard(uint8(sq), all&^PositionBB[sq])
		bDef += bits.OnesCount64(atk & inner[1])
	}
	minorDefDiff = wDef - bDef

	// Pawn defense
	wP := bits.OnesCount64(b.White.Pawns & inner[0])
	bP := bits.OnesCount64(b.Black.Pawns & inner[1])
	if wP > 3 {
		wP = 3
	}
	if bP > 3 {
		bP = 3
	}
	pawnDefDiff = wP - bP
	return
}

// WeakSquaresCounts returns two MG unit diffs derived from engine weak-square logic:
//
//	weakDiff     = (# white weak squares) - (# black weak squares)
//	weakKingDiff = (# white weak king-ring squares) - (# black ...)
func WeakSquaresCounts(b *gm.Board) (weakDiff int, weakKingDiff int) {
	// Build pawn attack maps
	wp, bp := b.White.Pawns, b.Black.Pawns
	wPawnAttackBB := ((wp &^ bitboardFileA) << 7) | ((wp &^ bitboardFileH) << 9)
	bPawnAttackBB := ((bp &^ bitboardFileH) >> 7) | ((bp &^ bitboardFileA) >> 9)

	// King inner zones for each side
	inner := getKingSafetyTable(b, true, wPawnAttackBB, bPawnAttackBB)

	// Build raw movement attack maps for N/B/R only (Q/K excluded for defense in weak-squares logic)
	var movementBB [2][5]uint64 // indices: 0=N,1=B,2=R,3=Q,4=K
	all := b.White.All | b.Black.All
	// Knights
	for x := b.White.Knights; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		movementBB[0][0] |= KnightMasks[sq]
	}
	for x := b.Black.Knights; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		movementBB[1][0] |= KnightMasks[sq]
	}
	// Bishops
	for x := b.White.Bishops; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		movementBB[0][1] |= gm.CalculateBishopMoveBitboard(uint8(sq), all&^PositionBB[sq])
	}
	for x := b.Black.Bishops; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		movementBB[1][1] |= gm.CalculateBishopMoveBitboard(uint8(sq), all&^PositionBB[sq])
	}
	// Rooks
	for x := b.White.Rooks; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		movementBB[0][2] |= gm.CalculateRookMoveBitboard(uint8(sq), all&^PositionBB[sq])
	}
	for x := b.Black.Rooks; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		movementBB[1][2] |= gm.CalculateRookMoveBitboard(uint8(sq), all&^PositionBB[sq])
	}

	// Derive weak-square bitboards via engine helper
	weakSquares, weakKingSquares := getWeakSquares(movementBB, inner, wPawnAttackBB, bPawnAttackBB)
	wWeak := weakSquares[0] &^ weakKingSquares[0]
	bWeak := weakSquares[1] &^ weakKingSquares[1]
	// Match engine evaluation sign: score = (bWeak - wWeak) * weakSquaresPenaltyMG
	weakDiff = bits.OnesCount64(bWeak) - bits.OnesCount64(wWeak)
	weakKingDiff = bits.OnesCount64(weakKingSquares[1]) - bits.OnesCount64(weakKingSquares[0])
	return
}

// BishopPairDiffsScaled returns MG/EG diffs for the bishop-pair feature,
// matching engine logic and MG scaling:
//   - Award only if one side has >=2 bishops and the opponent has <2.
//   - MG is scaled by the bishop-pair center scale (percent from getCenterMobilityScales).
//   - EG is unscaled (±1 units).
func BishopPairDiffsScaled(b *gm.Board) (mg int, eg int) {
	wB := bits.OnesCount64(b.White.Bishops)
	bB := bits.OnesCount64(b.Black.Bishops)

	// Build file masks from pawns
	var whiteFiles uint64 = 0
	for x := b.White.Pawns; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		whiteFiles |= onlyFile[sq%8]
	}
	var blackFiles uint64 = 0
	for x := b.Black.Pawns; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		blackFiles |= onlyFile[sq%8]
	}
	wSemiOpen := ^whiteFiles & blackFiles
	bSemiOpen := ^blackFiles & whiteFiles
	openFiles := ^whiteFiles & ^blackFiles

	// Pawn levers for center state
	wLever, bLever, _, _ := getPawnLeverBitboards(b)

	lockedCenter, openIdx := getCenterState(b, openFiles, wSemiOpen, bSemiOpen, wLever, bLever)
	_, _, bpScaleMG := getCenterMobilityScales(lockedCenter, openIdx)

	if wB >= 2 && bB < 2 {
		mg += bpScaleMG
		eg += 1
	}
	if bB >= 2 && wB < 2 {
		mg -= bpScaleMG
		eg -= 1
	}
	return mg, eg
}

// QueenInfiltrationCounts exposes the engine's refined queen infiltration occupancy condition
// to the tuner. It returns 0/1 counts for each side indicating whether a queen occupies
// an enemy weak square in the enemy half outside the enemy pawn attack span.
func QueenInfiltrationCounts(b *gm.Board) (wInf int, bInf int) {
	// Build pawn attack maps (captures only)
	wp, bp := b.White.Pawns, b.Black.Pawns
	wPawnAttackBB := ((wp &^ bitboardFileA) << 7) | ((wp &^ bitboardFileH) << 9)
	bPawnAttackBB := ((bp &^ bitboardFileH) >> 7) | ((bp &^ bitboardFileA) >> 9)

	// King inner zones
	inner := getKingSafetyTable(b, true, wPawnAttackBB, bPawnAttackBB)

	// Movement maps for N/B/R only (align with weak-square logic)
	var movementBB [2][5]uint64
	all := b.White.All | b.Black.All
	for x := b.White.Knights; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		movementBB[0][0] |= KnightMasks[sq]
	}
	for x := b.Black.Knights; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		movementBB[1][0] |= KnightMasks[sq]
	}
	for x := b.White.Bishops; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		movementBB[0][1] |= gm.CalculateBishopMoveBitboard(uint8(sq), all&^PositionBB[sq])
	}
	for x := b.Black.Bishops; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		movementBB[1][1] |= gm.CalculateBishopMoveBitboard(uint8(sq), all&^PositionBB[sq])
	}
	for x := b.White.Rooks; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		movementBB[0][2] |= gm.CalculateRookMoveBitboard(uint8(sq), all&^PositionBB[sq])
	}
	for x := b.Black.Rooks; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		movementBB[1][2] |= gm.CalculateRookMoveBitboard(uint8(sq), all&^PositionBB[sq])
	}

	// Weak-square bitboards
	weakSquares, _ := getWeakSquares(movementBB, inner, wPawnAttackBB, bPawnAttackBB)

	// Build pawn attack spans limited to own half (as in engine evaluation)
	wSpan := calculatePawnFileFill(wPawnAttackBB, true) & ranksBelow[4]
	bSpan := calculatePawnFileFill(bPawnAttackBB, false) & ranksAbove[4]

	// White queen occupies enemy weak squares in enemy half, outside black span
	if (b.White.Queens & weakSquares[1] & ranksAbove[4] &^ bSpan) != 0 {
		wInf = 1
	}
	// Black queen occupies enemy weak squares in enemy half, outside white span
	if (b.Black.Queens & weakSquares[0] & ranksBelow[4] &^ wSpan) != 0 {
		bInf = 1
	}
	return
}

// BishopXrayCounts returns unit diffs for bishop x-ray targets (MG-only logic in engine):
//
//	kDiff = (# white bishop xrays vs king) - (# black bishop xrays vs king)
//	rDiff = (# ... vs rook)
//	qDiff = (# ... vs queen)
func BishopXrayCounts(b *gm.Board) (kDiff int, rDiff int, qDiff int) {
	all := b.White.All | b.Black.All
	whiteMask := all &^ b.White.Knights
	blackMask := all &^ b.Black.Knights
	// White bishops
	for x := b.White.Bishops; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		occupied := whiteMask &^ PositionBB[sq]
		bb := gm.CalculateBishopMoveBitboard(uint8(sq), occupied)
		if bb&b.Black.Kings != 0 {
			kDiff++
		}
		if bb&b.Black.Rooks != 0 {
			rDiff++
		}
		if bb&b.Black.Queens != 0 {
			qDiff++
		}
	}
	// Black bishops
	for x := b.Black.Bishops; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		occupied := blackMask &^ PositionBB[sq]
		bb := gm.CalculateBishopMoveBitboard(uint8(sq), occupied)
		if bb&b.White.Kings != 0 {
			kDiff--
		}
		if bb&b.White.Rooks != 0 {
			rDiff--
		}
		if bb&b.White.Queens != 0 {
			qDiff--
		}
	}
	return
}

// PawnStormProxLeverDiffs exposes MG-only unit diffs for pawn storm/proximity/lever-storm terms.
// Returns:
//
//	stormDiff = (# white storm advanced on enemy wing) - (# black storm ...)
//	proxDiff  = (# black enemy pawns near our king wing) - (# white ...)
//	leverStormDiff = (# black immediate levers in white king wing advanced) - (# white ...)
func PawnStormProxLeverDiffs(b *gm.Board) (stormDiff int, proxDiff int, leverStormDiff int) {
	// King wings
	wWing, bWing := getKingWingMasks(b)
	// Storm/proximity bitboards (MG-only masks inside helpers)
	wStorm, bStorm := getPawnStormBitboards(b, wWing, bWing)
	wProx, bProx := getEnemyPawnProximityBitboards(b, wWing, bWing)
	// Lever bitboards
	wLever, bLever, _, _ := getPawnLeverBitboards(b)
	// Advanced lever in storm zone
	wLeverStorm := bits.OnesCount64((wLever & bWing) & ranksAbove[3])
	bLeverStorm := bits.OnesCount64((bLever & wWing) & ranksBelow[4])
	// Diffs (match engine signs)
	stormDiff = bits.OnesCount64(wStorm) - bits.OnesCount64(bStorm)
	proxDiff = bits.OnesCount64(bProx) - bits.OnesCount64(wProx)
	leverStormDiff = bLeverStorm - wLeverStorm
	return
}

// CenterMobilityScales returns (knScaleDelta, biScaleDelta) where deltas are scale-100.
// Use these as per-position signals to modulate MG mobility weights in the tuner.
func CenterMobilityScales(b *gm.Board) (knDelta int, biDelta int) {
	// Build open/semi-open file masks from pawns
	var whiteFiles, blackFiles uint64
	for x := b.White.Pawns; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		whiteFiles |= onlyFile[sq%8]
	}
	for x := b.Black.Pawns; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		blackFiles |= onlyFile[sq%8]
	}
	wSemi := ^whiteFiles & blackFiles
	bSemi := ^blackFiles & whiteFiles
	openFiles := ^whiteFiles & ^blackFiles
	// Pawn levers for center state
	wLever, bLever, _, _ := getPawnLeverBitboards(b)
	lockedCenter, openIdx := getCenterState(b, openFiles, wSemi, bSemi, wLever, bLever)
	kn, bi, _ := getCenterMobilityScales(lockedCenter, openIdx)
	return kn - 100, bi - 100
}

// ExtrasDiffs returns MG/EG unit diffs for additional piece-related features:
// idx mapping (MG array):
//
//	0: KnightOutpost (MG)
//	1: KnightCanAttackPiece (MG)
//	2: StackedRooks (MG)
//	3: RookXrayQueen (MG)
//	4: ConnectedRooks (MG)
//	5: SeventhRank (MG)
//
// EG array:
//
//	0: KnightOutpost (EG)
//	1: KnightCanAttackPiece (EG)
//	Others are 0 (MG-only features in engine eval)
func ExtrasDiffs(b *gm.Board) (mg [7]int, eg [2]int) {
	// Outposts: recompute outpost bitboards
	wp, bp := b.White.Pawns, b.Black.Pawns
	wPawnAttackBB := ((wp &^ bitboardFileA) << 7) | ((wp &^ bitboardFileH) << 9)
	bPawnAttackBB := ((bp &^ bitboardFileH) >> 7) | ((bp &^ bitboardFileA) >> 9)
	out := getOutpostsBB(b, wPawnAttackBB, bPawnAttackBB)
	wOut, bOut := out[0], out[1]
	// Knight outpost counts
	wKnOut := bits.OnesCount64(b.White.Knights & wOut)
	bKnOut := bits.OnesCount64(b.Black.Knights & bOut)
	mg[0] = wKnOut - bKnOut
	eg[0] = mg[0]
	// Bishop outposts (MG only)
	wBiOut := bits.OnesCount64(b.White.Bishops & wOut)
	bBiOut := bits.OnesCount64(b.Black.Bishops & bOut)
	mg[6] = wBiOut - bBiOut

	// Knight threats (unit counts, not weighted):
	// Count unique "a knight can attack a piece" events per side, mirroring engine logic
	countKnightThreats := func(white bool) int {
		wPieces := (b.White.Bishops | b.White.Rooks | b.White.Queens)
		bPieces := (b.Black.Bishops | b.Black.Rooks | b.Black.Queens)
		cnt := 0
		if white {
			for x := b.White.Knights; x != 0; x &= x - 1 {
				from := bits.TrailingZeros64(x)
				knightMoves := KnightMasks[from] &^ b.White.All
				for y := knightMoves; y != 0; y &= y - 1 {
					to := bits.TrailingZeros64(y)
					threat := KnightMasks[to]
					if threat&bPieces != 0 {
						bPieces &^= threat
						cnt++
					}
				}
			}
		} else {
			for x := b.Black.Knights; x != 0; x &= x - 1 {
				from := bits.TrailingZeros64(x)
				knightMoves := KnightMasks[from] &^ b.Black.All
				for y := knightMoves; y != 0; y &= y - 1 {
					to := bits.TrailingZeros64(y)
					threat := KnightMasks[to]
					if threat&wPieces != 0 {
						wPieces &^= threat
						cnt++
					}
				}
			}
		}
		return cnt
	}
	wKth := countKnightThreats(true)
	bKth := countKnightThreats(false)
	mg[1] = wKth - bKth
	eg[1] = mg[1]

	// Stacked rooks (connected on files) — unit stacks diff via helper
	wFiles, bFiles := getRookConnectedFiles(b)
	wStacks := bits.OnesCount64(wFiles) / 8
	bStacks := bits.OnesCount64(bFiles) / 8
	mg[2] = wStacks - bStacks

	// Rook xray queen: count occurrences directly
	rxCnt := 0
	allPieces := b.White.All | b.Black.All
	whiteMask := allPieces &^ b.White.Knights
	blackMask := allPieces &^ b.Black.Knights
	for x := b.White.Rooks; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		occupied := whiteMask &^ PositionBB[sq]
		rookMoves := gm.CalculateRookMoveBitboard(uint8(sq), occupied)
		if rookMoves&b.Black.Queens != 0 {
			rxCnt++
		}
	}
	for x := b.Black.Rooks; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		occupied := blackMask &^ PositionBB[sq]
		rookMoves := gm.CalculateRookMoveBitboard(uint8(sq), occupied)
		if rookMoves&b.White.Queens != 0 {
			rxCnt--
		}
	}
	mg[3] = rxCnt

	// Connected rooks on same rank/file (bonus per connection) — approximate via adjacency on files
	// Use file-based connections from getRookConnectedFiles as a proxy for connected rooks
	// Count number of files where both ranks have rooks for each side
	countConn := func(files uint64) int { return bits.OnesCount64(files) / 8 }
	mg[4] = countConn(wFiles) - countConn(bFiles)

	// Rooks on 7th (MG): count white rooks on rank 7 (index 6), black rooks on rank 2 (index 1)
	w7, b7 := 0, 0
	for wr := b.White.Rooks; wr != 0; wr &= wr - 1 {
		if (bits.TrailingZeros64(wr) / 8) == 6 {
			w7++
		}
	}
	for br := b.Black.Rooks; br != 0; br &= br - 1 {
		if (bits.TrailingZeros64(br) / 8) == 1 {
			b7++
		}
	}
	mg[5] = w7 - b7
	return mg, eg
}
