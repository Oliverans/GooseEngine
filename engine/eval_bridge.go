package engine

import (
	gm "chess-engine/goosemg"
	"math/bits"
)

func pawnAttackBitboards(b *gm.Board) (wPawnAttackBB, bPawnAttackBB, wPawnAttackBB_E, wPawnAttackBB_W, bPawnAttackBB_E, bPawnAttackBB_W uint64) {
	wPawnAttackBB_E, wPawnAttackBB_W = PawnCaptureBitboards(b.White.Pawns, true)
	bPawnAttackBB_E, bPawnAttackBB_W = PawnCaptureBitboards(b.Black.Pawns, false)
	wPawnAttackBB = wPawnAttackBB_E | wPawnAttackBB_W
	bPawnAttackBB = bPawnAttackBB_E | bPawnAttackBB_W
	return
}

// PawnStructDiffs computes unit feature differences (white minus black or vice-versa as noted)
// for pawn-structure terms (Tier 2), separately for MG and EG, using the engine's bitboard helpers and masks.
// The feature indices are:
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
	doubDiff := bDoub - wDoub // FIXED: Now (White - Black) to match phalanx/blocked convention
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
	isoDiff := bIso - wIso //wIso - bIso // FIXED: Now (White - Black) to match phalanx/blocked convention
	mg[1], eg[1] = isoDiff, isoDiff

	// Pawn attack maps
	wPawnAttackBB, bPawnAttackBB, wPawnAttackBB_E, wPawnAttackBB_W, bPawnAttackBB_E, bPawnAttackBB_W := pawnAttackBitboards(b)

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
	thirdAndFourthRank := onlyRank[2] | onlyRank[3]
	fifthAndSixthRank := onlyRank[4] | onlyRank[5]
	blkDiff := bits.OnesCount64(wBlkBB&fifthAndSixthRank) - bits.OnesCount64(bBlkBB&thirdAndFourthRank)
	mg[4], eg[4] = blkDiff, blkDiff

	// Pawn levers: enemy pawns hitting the square in front of our pawn
	wLeverBB, bLeverBB, wLeverPushed, bLeverPushed, wWeakLever, bWeakLever := getPawnLeverBitboards(b, wPawnAttackBB, bPawnAttackBB, wPawnAttackBB_E, wPawnAttackBB_W, bPawnAttackBB_E, bPawnAttackBB_W)
	_ = wLeverPushed
	_ = bLeverPushed
	wLeverCnt := bits.OnesCount64(wLeverBB)
	bLeverCnt := bits.OnesCount64(bLeverBB)
	leverDiff := wLeverCnt - bLeverCnt
	mg[5], eg[5] = leverDiff, leverDiff

	// Weak levers: multi-lever targets that lack pawn support
	weakDiff := bits.OnesCount64(bWeakLever) - bits.OnesCount64(wWeakLever)
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
func ImbalanceDiffs(b *gm.Board) (mg [2]int, eg [2]int) {
	pieceCount := countPieceTypes(b)
	const White = 0
	const Black = 1
	wp := pieceCount[White][gm.PieceTypePawn]
	wn := pieceCount[White][gm.PieceTypeKnight]
	wb := pieceCount[White][gm.PieceTypeBishop]

	bp := pieceCount[Black][gm.PieceTypePawn]
	bn := pieceCount[Black][gm.PieceTypeKnight]
	bb := pieceCount[Black][gm.PieceTypeBishop]

	wPawnDelta := wp - ImbalanceRefPawnCount
	bPawnDelta := bp - ImbalanceRefPawnCount

	// Knight/Bishop per pawn
	mg[0] = (wPawnDelta * wn) - (bPawnDelta * bn)
	eg[0] = mg[0]
	mg[1] = (wPawnDelta * wb) - (bPawnDelta * bb)
	eg[1] = mg[1]
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
//	semiOpenDiff = (# black king-adjacent or king-file semi-open files) - (# white ...)
//	openDiff     = (# black king-adjacent or king-file open files) - (# white ...)
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
	wAdjFiles := wKingFile | ((wKingFile & ^bitboardFileA) >> 1) | ((wKingFile & ^bitboardFileH) << 1)
	bAdjFiles := bKingFile | ((bKingFile & ^bitboardFileA) >> 1) | ((bKingFile & ^bitboardFileH) << 1)

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

// EndgameKingTerms exposes the EG-only king terms (centralization penalty or mop-up bonus)
// with the same conditions used in evaluation.go. Returns signed diffs (white - black).
func EndgameKingTerms(b *gm.Board) (centralizationEG int, mopUpEG int) {
	piecePhase := GetPiecePhase(b)
	qCount := bits.OnesCount64(b.White.Queens | b.Black.Queens)

	if !((piecePhase < 16 && qCount == 0) || piecePhase < 10) {
		return 0, 0
	}

	wPieceCount := bits.OnesCount64(b.White.Bishops | b.White.Knights | b.White.Rooks | b.White.Queens)
	bPieceCount := bits.OnesCount64(b.Black.Bishops | b.Black.Knights | b.Black.Rooks | b.Black.Queens)

	if wPieceCount > 0 && bPieceCount == 0 {
		mopUpEG = getKingMopUpBonus(b, true, b.White.Queens > 0, b.White.Rooks > 0)
	} else if wPieceCount == 0 && bPieceCount > 0 {
		mopUpEG = -getKingMopUpBonus(b, false, b.Black.Queens > 0, b.Black.Rooks > 0)
	} else {
		centralizationEG = kingEndGameCentralizationPenalty(b)
	}
	return
}

// SpaceAndWeakKingDiffs mirrors the engine's spaceEvaluation and weakKingSquaresPenalty helpers.
// spaceDiff    = (# safe white space squares) - (# safe black space squares)
// weakKingDiff = (# black weak king-ring squares) - (# white weak king-ring squares)
func SpaceAndWeakKingDiffs(b *gm.Board) (spaceDiff int, weakKingDiff int) {
	wp, bp := b.White.Pawns, b.Black.Pawns
	wPawnAttackBB := ((wp &^ bitboardFileA) << 7) | ((wp &^ bitboardFileH) << 9)
	bPawnAttackBB := ((bp &^ bitboardFileH) >> 7) | ((bp &^ bitboardFileA) >> 9)

	// Knight/bishop control maps (Q/K excluded, matching spaceEvaluation inputs)
	var knightMovementBB [2]uint64
	var bishopMovementBB [2]uint64
	all := b.White.All | b.Black.All
	for x := b.White.Knights; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		knightMovementBB[0] |= KnightMasks[sq]
	}
	for x := b.Black.Knights; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		knightMovementBB[1] |= KnightMasks[sq]
	}
	for x := b.White.Bishops; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		bishopMovementBB[0] |= gm.CalculateBishopMoveBitboard(uint8(sq), all&^PositionBB[sq])
	}
	for x := b.Black.Bishops; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		bishopMovementBB[1] |= gm.CalculateBishopMoveBitboard(uint8(sq), all&^PositionBB[sq])
	}

	// Space evaluation is gated by piecePhase (skip in early opening)
	if GetPiecePhase(b) >= 6 {
		wSpaceZone := wSpaceZoneMask &^ b.Black.Pawns
		bSpaceZone := bSpaceZoneMask &^ b.White.Pawns

		wControl := wPawnAttackBB | knightMovementBB[0] | bishopMovementBB[0]
		bControl := bPawnAttackBB | knightMovementBB[1] | bishopMovementBB[1]

		wSafe := wSpaceZone & wControl &^ bPawnAttackBB
		bSafe := bSpaceZone & bControl &^ wPawnAttackBB

		// Count occupied squares in the space zone as "used" space.
		wSafe |= wSpaceZone & b.White.All
		bSafe |= bSpaceZone & b.Black.All

		spaceDiff = bits.OnesCount64(wSafe) - bits.OnesCount64(bSafe)
	}

	inner := getKingSafetyTable(b, true, wPawnAttackBB, bPawnAttackBB)
	wWeak := inner[0] &^ wPawnAttackBB &^ b.White.All
	bWeak := inner[1] &^ bPawnAttackBB &^ b.Black.All
	weakKingDiff = bits.OnesCount64(bWeak) - bits.OnesCount64(wWeak)
	return
}

// KnightTropismDiffs exposes knight king-tropism MG/EG diffs.
func KnightTropismDiffs(b *gm.Board) (mg int, eg int) {
	return knightKingTropism(b)
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
	wPawnAttackBB, bPawnAttackBB, wPawnAttackBB_E, wPawnAttackBB_W, bPawnAttackBB_E, bPawnAttackBB_W := pawnAttackBitboards(b)
	wLever, bLever, _, _, _, _ := getPawnLeverBitboards(b, wPawnAttackBB, bPawnAttackBB, wPawnAttackBB_E, wPawnAttackBB_W, bPawnAttackBB_E, bPawnAttackBB_W)

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

// BadBishopDiffs returns MG/EG diffs for the bad-bishop term using blocked pawns.
func BadBishopDiffs(b *gm.Board) (mg int, eg int) {
	entry := GetPawnEntry(b, false)
	wLightFixed := bits.OnesCount64(entry.WBlockedBB & lightSquares)
	wDarkFixed := bits.OnesCount64(entry.WBlockedBB & darkSquares)
	bLightFixed := bits.OnesCount64(entry.BBlockedBB & lightSquares)
	bDarkFixed := bits.OnesCount64(entry.BBlockedBB & darkSquares)

	for x := b.White.Bishops; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		bm, be := badBishopPenalty(sq, wDarkFixed, wLightFixed)
		mg += bm
		eg += be
	}
	for x := b.Black.Bishops; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		bm, be := badBishopPenalty(sq, bDarkFixed, bLightFixed)
		mg -= bm
		eg -= be
	}
	return mg, eg
}

// BadBishopUnitDiff returns the fixed-pawn count on bishop colors (white minus black).
func BadBishopUnitDiff(b *gm.Board) int {
	entry := GetPawnEntry(b, false)
	wLightFixed := bits.OnesCount64(entry.WBlockedBB & lightSquares)
	wDarkFixed := bits.OnesCount64(entry.WBlockedBB & darkSquares)
	bLightFixed := bits.OnesCount64(entry.BBlockedBB & lightSquares)
	bDarkFixed := bits.OnesCount64(entry.BBlockedBB & darkSquares)

	diff := 0
	for x := b.White.Bishops; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		if PositionBB[sq]&darkSquares != 0 {
			diff += wDarkFixed
		} else {
			diff += wLightFixed
		}
	}
	for x := b.Black.Bishops; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		if PositionBB[sq]&darkSquares != 0 {
			diff -= bDarkFixed
		} else {
			diff -= bLightFixed
		}
	}
	return diff
}

// KingPasserProximityTerm returns the EG-only king proximity term for passed pawns.
func KingPasserProximityTerm(b *gm.Board) int {
	entry := GetPawnEntry(b, false)
	return kingPasserProximity(b, entry)
}

// PawnStormProxLeverDiffs exposes MG-only unit diffs for pawn storm/proximity/lever-storm terms.
func PawnStormProxLeverDiffs(b *gm.Board) (stormDiff int, proxDiff int, leverStormDiff int) {
	// King wings
	wWing, bWing := getKingWingMasks(b)
	// Storm/proximity bitboards (MG-only masks inside helpers)
	wStorm, bStorm := getPawnStormBitboards(b, wWing, bWing)
	wProx, bProx := getEnemyPawnProximityBitboards(b, wWing, bWing)
	// Lever bitboards
	wPawnAttackBB, bPawnAttackBB, wPawnAttackBB_E, wPawnAttackBB_W, bPawnAttackBB_E, bPawnAttackBB_W := pawnAttackBitboards(b)
	wLever, bLever, _, _, _, _ := getPawnLeverBitboards(b, wPawnAttackBB, bPawnAttackBB, wPawnAttackBB_E, wPawnAttackBB_W, bPawnAttackBB_E, bPawnAttackBB_W)
	// Advanced lever in storm zone
	wLeverStorm := bits.OnesCount64((wLever & bWing) & ranksAbove[3])
	bLeverStorm := bits.OnesCount64((bLever & wWing) & ranksBelow[4])
	// Diffs (match engine signs)
	stormDiff = bits.OnesCount64(wStorm) - bits.OnesCount64(bStorm)
	proxDiff = bits.OnesCount64(bProx) - bits.OnesCount64(wProx)
	leverStormDiff = bLeverStorm - wLeverStorm
	return
}

// PawnStormCategoryDiffs returns unit counts for each pawn storm category.
// Returns diffs (white - black) for each rank, decomposed by pawn type:
//   - freeDiff: free advancing pawns (not blocked, not lever)
//   - leverDiff: pawns with lever opportunity
//   - weakLeverDiff: weak lever pawns (multi-lever, unsupported)
//   - blockedDiff: pawns blocked by enemy pawn directly ahead
//
// The bool indicates opposite-side castling (where storms are active); if kings
// are central or castled same-side, diffs are zeroed and the flag is false.
//
// All counts are rank-indexed arrays [8]int where index corresponds to attacker's rank.
func PawnStormCategoryDiffs(b *gm.Board) (freeDiff, leverDiff, weakLeverDiff, blockedDiff [8]int, oppositeSide bool) {
	// Get pawn hash entry for lever bitboards
	pawnEntry := GetPawnEntry(b, false)

	// Get king positions and zones
	wKingSq := bits.TrailingZeros64(b.White.Kings)
	bKingSq := bits.TrailingZeros64(b.Black.Kings)

	wKingFile := wKingSq % 8
	bKingFile := bKingSq % 8

	wQueenside := wKingFile <= 2
	wKingside := wKingFile >= 5
	bQueenside := bKingFile <= 2
	bKingside := bKingFile >= 5

	// If both kings are central or castled same-side, storms are inactive.
	if (!wQueenside && !wKingside && !bQueenside && !bKingside) || ((wQueenside && bQueenside) || (wKingside && bKingside)) {
		return
	}

	oppositeSide = (wQueenside && bKingside) || (wKingside && bQueenside)

	wKingZone := getKingFileZone(wKingFile)
	bKingZone := getKingFileZone(bKingFile)

	// Process white pawns attacking black king zone
	for x := b.White.Pawns & bKingZone; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		pawnBB := PositionBB[sq]
		rank := sq / 8

		// Skip ranks that aren't relevant for pawn storm
		if rank < 3 {
			continue
		}

		// Classify pawn and increment appropriate diff
		if PositionBB[sq+8]&b.Black.Pawns != 0 {
			// Blocked by enemy pawn directly ahead
			blockedDiff[rank]++
		} else if pawnBB&pawnEntry.WLeverBB != 0 {
			// Has lever opportunity
			leverDiff[rank]++
		} else if pawnBB&pawnEntry.WWeakLeverBB != 0 {
			// Weak lever (multi-lever, unsupported)
			weakLeverDiff[rank]++
		} else {
			// Free advancing pawn
			freeDiff[rank]++
		}
	}

	// Process black pawns attacking white king zone
	for x := b.Black.Pawns & wKingZone; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		pawnBB := PositionBB[sq]
		rank := sq / 8
		sideRank := 7 - rank // Black's perspective

		// Skip ranks that aren't relevant for pawn storm
		if sideRank < 3 {
			continue
		}

		// Classify pawn and decrement appropriate diff (black subtracts from white)
		if PositionBB[sq-8]&b.White.Pawns != 0 {
			// Blocked by enemy pawn directly ahead
			blockedDiff[sideRank]--
		} else if pawnBB&pawnEntry.BLeverBB != 0 {
			// Has lever opportunity
			leverDiff[sideRank]--
		} else if pawnBB&pawnEntry.BWeakLeverBB != 0 {
			// Weak lever (multi-lever, unsupported)
			weakLeverDiff[sideRank]--
		} else {
			// Free advancing pawn
			freeDiff[sideRank]--
		}
	}

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
	wPawnAttackBB, bPawnAttackBB, wPawnAttackBB_E, wPawnAttackBB_W, bPawnAttackBB_E, bPawnAttackBB_W := pawnAttackBitboards(b)
	wLever, bLever, _, _, _, _ := getPawnLeverBitboards(b, wPawnAttackBB, bPawnAttackBB, wPawnAttackBB_E, wPawnAttackBB_W, bPawnAttackBB_E, bPawnAttackBB_W)
	lockedCenter, openIdx := getCenterState(b, openFiles, wSemi, bSemi, wLever, bLever)
	kn, bi, _ := getCenterMobilityScales(lockedCenter, openIdx)
	return kn - 100, bi - 100
}

// ExtrasDiffs returns MG/EG unit diffs for additional piece-related features:
// idx mapping (MG array):
//
//	0: KnightOutpost (MG)
//	1: StackedRooks (MG)
//	2: BishopOutpost (MG)
//
// EG array:
//
//	0: KnightOutpost (EG)
//	1: BishopOutpost (EG)
func ExtrasDiffs(b *gm.Board) (mg [3]int, eg [2]int) {
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
	// Bishop outposts
	wBiOut := bits.OnesCount64(b.White.Bishops & wOut)
	bBiOut := bits.OnesCount64(b.Black.Bishops & bOut)
	mg[2] = wBiOut - bBiOut
	eg[1] = mg[2]

	// Stacked rooks (connected on files) - unit stacks diff via helper
	wFiles, bFiles := getRookConnectedFiles(b)
	wStacks := bits.OnesCount64(wFiles) / 8
	bStacks := bits.OnesCount64(bFiles) / 8
	mg[1] = wStacks - bStacks

	// Seventh-rank handled in P1 scalars (EG only); no MG contribution here
	return mg, eg
}
