package engine

import (
	"math/bits"

	gm "chess-engine/goosemg"
)

// Knight move masks for each square (precomputed bitboards)
var KnightMasks = [64]uint64{
	0x0000000000020400, 0x0000000000050800, 0x00000000000a1100, 0x0000000000142200,
	0x0000000000284400, 0x0000000000508800, 0x0000000000a01000, 0x0000000000402000,
	0x0000000002040004, 0x0000000005080008, 0x000000000a110011, 0x0000000014220022,
	0x0000000028440044, 0x0000000050880088, 0x00000000a0100010, 0x0000000040200020,
	0x0000000204000402, 0x0000000508000805, 0x0000000a1100110a, 0x0000001422002214,
	0x0000002844004428, 0x0000005088008850, 0x000000a0100010a0, 0x0000004020002040,
	0x0000020400040200, 0x0000050800080500, 0x00000a1100110a00, 0x0000142200221400,
	0x0000284400442800, 0x0000508800885000, 0x0000a0100010a000, 0x0000402000204000,
	0x0002040004020000, 0x0005080008050000, 0x000a1100110a0000, 0x0014220022140000,
	0x0028440044280000, 0x0050880088500000, 0x00a0100010a00000, 0x0040200020400000,
	0x0204000402000000, 0x0508000805000000, 0x0a1100110a000000, 0x1422002214000000,
	0x2844004428000000, 0x5088008850000000, 0xa0100010a0000000, 0x4020002040000000,
	0x0400040200000000, 0x0800080500000000, 0x1100110a00000000, 0x2200221400000000,
	0x4400442800000000, 0x8800885000000000, 0x100010a000000000, 0x2000204000000000,
	0x0004020000000000, 0x0008050000000000, 0x00110a0000000000, 0x0022140000000000,
	0x0044280000000000, 0x0088500000000000, 0x0010a00000000000, 0x0020400000000000,
}

func InBetween(i, min, max int) bool {
	return i >= min && i <= max
}

// File bitboard masks for files A and H (for shifting operations)
const (
	bitboardFileA uint64 = 0x0101010101010101
	bitboardFileH uint64 = 0x8080808080808080
)

var ranksAbove = [8]uint64{
	0xffffffffffffffff, 0xffffffffffffff00, 0xffffffffffff0000, 0xffffffffff000000,
	0xffffffff00000000, 0xffffff0000000000, 0xffff000000000000, 0xff00000000000000,
}
var ranksBelow = [8]uint64{
	0x00000000000000ff, 0x000000000000ffff, 0x0000000000ffffff, 0x00000000ffffffff,
	0x000000ffffffffff, 0x0000ffffffffffff, 0x00ffffffffffffff, 0xffffffffffffffff,
}

func PawnCaptureBitboards(pawns uint64, white bool) (east uint64, west uint64) {
	if white {
		east = (pawns << 9) & ^bitboardFileA // file + 1
		west = (pawns << 7) & ^bitboardFileH // file - 1
	} else {
		east = (pawns >> 7) & ^bitboardFileA // file + 1
		west = (pawns >> 9) & ^bitboardFileH // file - 1
	}
	return
}

// Compute king safety zone bitboards (inner 1-ring or outer 2-ring)
func getKingSafetyTable(b *gm.Board, inner bool, wPawnAttackBB, bPawnAttackBB uint64) [2]uint64 {
	var kingZone [2]uint64
	kingSquares := [2]uint64{b.White.Kings, b.Black.Kings}
	for side := 0; side < 2; side++ {
		// Start with king's square
		zone := kingSquares[side]
		kingSq := bits.TrailingZeros64(zone)
		rank := kingSq / 8
		file := kingSq % 8
		// Always include one rank above and below (or within board bounds)
		switch rank {
		case 0:
			zone |= zone<<8 | zone<<16
		case 7:
			zone |= zone>>8 | zone>>16
		default:
			zone |= zone<<8 | zone>>8
		}
		// Always include one file to left and right (with bounds check)
		switch file {
		case 0:
			zone |= zone<<1 | zone<<2
		case 7:
			zone |= zone>>1 | zone>>2
		default:
			zone |= ((zone &^ bitboardFileA) >> 1) | ((zone &^ bitboardFileH) << 1)
		}
		// Exclude friendly pawn-attacked squares for inner zone
		if side == 0 {
			zone &^= wPawnAttackBB
		} else {
			zone &^= bPawnAttackBB
		}
		kingZone[side] = zone
	}
	if !inner {
		// Compute outer ring by expanding the inner zone and removing inner zone itself
		for side := 0; side < 2; side++ {
			zoneInner := kingZone[side]
			zoneOuter := zoneInner
			zoneOuter |= zoneOuter<<8 | zoneOuter>>8
			zoneOuter |= ((zoneOuter &^ bitboardFileA) >> 1) | ((zoneOuter &^ bitboardFileH) << 1)
			kingZone[side] = zoneOuter &^ zoneInner
		}
	}
	return kingZone
}

// Compute outpost candidate squares for knights/bishops for each side
func getOutpostsBB(b *gm.Board, wPawnAttackBB, bPawnAttackBB uint64) (outposts [2]uint64) {
	// A candidate is a square a friendly pawn covers, inside the outpost zone,
	// not already occupied by a friendly pawn. It survives if no enemy pawn can
	// ever advance to attack it -- see outpostBlockersWhite/Black, which hold
	// that per-square constant so it is not rebuilt on every node.
	wCandidates := (wPawnAttackBB & wAllowedOutpostMask) &^ b.White.Pawns
	var wOutpostBB uint64
	for x := wCandidates; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		if b.Black.Pawns&outpostBlockersWhite[sq] == 0 {
			wOutpostBB |= PositionBB[sq]
		}
	}
	// Black potential outposts (symmetric)
	bCandidates := (bPawnAttackBB & bAllowedOutpostMask) &^ b.Black.Pawns
	var bOutpostBB uint64
	for x := bCandidates; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		if b.White.Pawns&outpostBlockersBlack[sq] == 0 {
			bOutpostBB |= PositionBB[sq]
		}
	}
	outposts[0] = wOutpostBB
	outposts[1] = bOutpostBB
	return
}

// OutpostsBB exposes engine outpost-square calculation for consumers that need
// the exact same definition as evaluation.
func OutpostsBB(b *gm.Board, wPawnAttackBB, bPawnAttackBB uint64) [2]uint64 {
	return getOutpostsBB(b, wPawnAttackBB, bPawnAttackBB)
}

// =============================================================================
// PAWN HASH TABLE
// =============================================================================

const PawnHashSize = 1 << 16 // 65536 entries (~8MB)

// PawnHashEntry stores cached pawn structure analysis
type PawnHashEntry struct {
	// Key for verifying collisions (pawn bitboards).
	//
	// Valid belongs with the key, not at the end of the struct. ProbePawnHash
	// tests it before either bitboard, so when it lived after PawnScoreEG every
	// probe -- including every miss, the expensive path -- pulled in the last
	// cache line for one bool and then the first line for the key. Two lines to
	// answer a question that needs 17 bytes. Here all three share line 0.
	//
	// This costs nothing: the struct was already padding the trailing bool out
	// to 8 bytes, so the seven bytes now sitting beside Valid are the same seven
	// bytes, just moved. Total size is unchanged at 208.
	//
	// It cannot simply be dropped in favour of a zero check. A zeroed slot would
	// then read as a valid pawnless position, but the true entry for one has
	// OpenFiles = ^0, not 0.
	WhitePawns uint64
	BlackPawns uint64
	Valid      bool

	// Pawn attack maps
	WPawnAttackBB uint64
	BPawnAttackBB uint64

	// File structure masks
	OpenFiles      uint64
	WSemiOpenFiles uint64
	BSemiOpenFiles uint64

	// Pawn structure bitboards
	WPassedBB    uint64
	BPassedBB    uint64
	WIsolatedBB  uint64
	BIsolatedBB  uint64
	WBackwardBB  uint64
	BBackwardBB  uint64
	WBlockedBB   uint64
	BBlockedBB   uint64
	// Pawns with an enemy pawn somewhere ahead on the same file. Weaker than
	// "blocked", which is the immediate case, and weaker than having stoppers,
	// which also counts the adjacent files. Depends only on pawns, so it caches.
	WOpposedBB uint64
	BOpposedBB uint64
	WLeverBB     uint64
	BLeverBB     uint64
	WWeakLeverBB uint64
	BWeakLeverBB uint64

	// Space bonus per side before the material weight is applied. Depends only
	// on pawns and file state, so it caches here; spaceEvaluation supplies the
	// piece-count scaling per node.
	WSpaceBonus int
	BSpaceBonus int

	// Precomputed pawn scores
	PawnScoreMG int
	PawnScoreEG int
}

var PawnHashTable [PawnHashSize]PawnHashEntry

// Compute index into pawn hash table from pawn bitboards (mix bits for distribution)
func pawnStructureKey(whitePawns, blackPawns uint64) uint64 {
	const goldenRatio = 0x9E3779B97F4A7C15
	hash := whitePawns ^ (blackPawns * goldenRatio)
	hash ^= hash >> 33
	hash *= 0xFF51AFD7ED558CCD
	hash ^= hash >> 33
	return hash
}

func pawnHashIndex(whitePawns, blackPawns uint64) uint64 {
	return pawnStructureKey(whitePawns, blackPawns) & (PawnHashSize - 1)
}

// ProbePawnHash returns pawn entry and a hit flag if found
func ProbePawnHash(b *gm.Board) (*PawnHashEntry, bool) {
	idx := pawnHashIndex(b.White.Pawns, b.Black.Pawns)
	entry := &PawnHashTable[idx]
	if entry.Valid &&
		entry.WhitePawns == b.White.Pawns && entry.BlackPawns == b.Black.Pawns {
		return entry, true
	}
	return entry, false
}

// StorePawnHash writes a computed pawn entry to the table
func StorePawnHash(b *gm.Board, entry *PawnHashEntry) {
	idx := pawnHashIndex(b.White.Pawns, b.Black.Pawns)
	entry.WhitePawns = b.White.Pawns
	entry.BlackPawns = b.Black.Pawns
	entry.Valid = true
	PawnHashTable[idx] = *entry
}

// ClearPawnHash resets the pawn hash table (use at start of a new game)
func ClearPawnHash() {
	for i := range PawnHashTable {
		PawnHashTable[i] = PawnHashEntry{}
	}
}

// ComputePawnEntry calculates all pawn structure data from scratch (on a cache miss)
func ComputePawnEntry(b *gm.Board, debug bool) PawnHashEntry {
	var entry PawnHashEntry

	// 1. Pawn attack bitboards
	wPawnAttackBB_E, wPawnAttackBB_W := PawnCaptureBitboards(b.White.Pawns, true)  // east/west attacks by white pawns
	bPawnAttackBB_E, bPawnAttackBB_W := PawnCaptureBitboards(b.Black.Pawns, false) // east/west attacks by black pawns
	entry.WPawnAttackBB = wPawnAttackBB_E | wPawnAttackBB_W
	entry.BPawnAttackBB = bPawnAttackBB_E | bPawnAttackBB_W

	// 2. File open/semi-open masks
	var whiteFiles, blackFiles uint64
	for x := b.White.Pawns; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		whiteFiles |= onlyFile[sq%8]
	}
	for x := b.Black.Pawns; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		blackFiles |= onlyFile[sq%8]
	}
	entry.OpenFiles = ^whiteFiles & ^blackFiles
	entry.WSemiOpenFiles = ^whiteFiles & blackFiles
	entry.BSemiOpenFiles = ^blackFiles & whiteFiles

	// 3. Pawn structure bitboards
	entry.WIsolatedBB, entry.BIsolatedBB = getIsolatedPawnsBitboards(b)
	entry.WPassedBB, entry.BPassedBB = getPassedPawnsBitboards(b)
	entry.WBlockedBB, entry.BBlockedBB = getBlockedPawnsBitboards(b)
	// The fills already exclude the origin square, so no extra shift is needed
	// to make this "strictly ahead".
	entry.WOpposedBB = b.White.Pawns & calculatePawnSouthFill(b.Black.Pawns)
	entry.BOpposedBB = b.Black.Pawns & calculatePawnNorthFill(b.White.Pawns)

	entry.WSpaceBonus = spaceBonusFor(b.White.Pawns, entry.BPawnAttackBB, wSpaceZoneMask,
		entry.WSemiOpenFiles, entry.OpenFiles, true)
	entry.BSpaceBonus = spaceBonusFor(b.Black.Pawns, entry.WPawnAttackBB, bSpaceZoneMask,
		entry.BSemiOpenFiles, entry.OpenFiles, false)
	entry.WBackwardBB, entry.BBackwardBB = getBackwardPawnsBitboards(b, entry.WPawnAttackBB, entry.BPawnAttackBB, entry.WIsolatedBB, entry.BIsolatedBB, entry.WPassedBB, entry.BPassedBB)
	// Lever-PUSH bitboards depend on full-board occupancy (empty push square)
	// and are therefore never cached here; see LeverPushBitboards.
	wLever, bLever, _, _, wWeakLever, bWeakLever := getPawnLeverBitboards(b, entry.WPawnAttackBB, entry.BPawnAttackBB, wPawnAttackBB_E, wPawnAttackBB_W, bPawnAttackBB_E, bPawnAttackBB_W)
	entry.WLeverBB = wLever
	entry.BLeverBB = bLever
	entry.WWeakLeverBB = wWeakLever
	entry.BWeakLeverBB = bWeakLever

	// 4. Pawn score components
	pawnPsqtMG, pawnPsqtEG := countPieceTables(&b.White.Pawns, &b.Black.Pawns, &PSQT_MG[gm.PieceTypePawn], &PSQT_EG[gm.PieceTypePawn])
	isoMG, isoEG := isolatedPawnPenalty(&entry)
	doubledMG, doubledEG := pawnDoublingPenalties(b, &entry)
	connMG, connEG := connectedPawnBonus(b, &entry)
	passedMG, passedEG := passedPawnBonus(entry.WPassedBB, entry.BPassedBB)
	// NOTE: the candidate-passed term depends on FULL-BOARD occupancy (piece
	// on the lever-push square), so it must not be cached in this pawn-keyed
	// entry. It is computed per evaluation, like the pawn storm.
	blockedMG, blockedEG := blockedPawnPenalty(entry.WBlockedBB, entry.BBlockedBB)
	backMG, backEG := backwardPawnPenalty(&entry)
	weakLeverMG, weakLeverEG := pawnWeakLeverPenalty(entry.WWeakLeverBB, entry.BWeakLeverBB)

	// Sum all pawn contributions
	entry.PawnScoreMG = pawnPsqtMG + isoMG + doubledMG + connMG + passedMG + blockedMG + backMG + weakLeverMG
	entry.PawnScoreEG = pawnPsqtEG + isoEG + doubledEG + connEG + passedEG + blockedEG + backEG + weakLeverEG

	return entry
}

// LeverPushBitboards computes the lever-push pawns for the current full
// board. Occupancy-dependent (push square must be empty of ANY piece),
// hence never cached in the pawn hash.
func LeverPushBitboards(b *gm.Board) (wLeverPush, bLeverPush uint64) {
	empty := ^(b.White.All | b.Black.All)

	wOne := (b.White.Pawns << 8) & empty
	wOneAttE, wOneAttW := PawnCaptureBitboards(wOne, true)
	wHitAfterPush := (wOneAttE | wOneAttW) & b.Black.Pawns
	wPushedLeverSources := ((wHitAfterPush &^ bitboardFileH) >> 7) | ((wHitAfterPush &^ bitboardFileA) >> 9)
	wLeverPush = (wPushedLeverSources & wOne) >> 8

	bOne := (b.Black.Pawns >> 8) & empty
	bOneAttE, bOneAttW := PawnCaptureBitboards(bOne, false)
	bHitAfterPush := (bOneAttE | bOneAttW) & b.White.Pawns
	bPushedLeverSources := ((bHitAfterPush &^ bitboardFileA) << 7) | ((bHitAfterPush &^ bitboardFileH) << 9)
	bLeverPush = (bPushedLeverSources & bOne) << 8
	return
}

// CandidatePassedTerm computes the candidate-passed-pawn term for the current
// full board. Occupancy-dependent, hence never cached in the pawn hash.
func CandidatePassedTerm(b *gm.Board, entry *PawnHashEntry) (mg, eg int, wCand, bCand uint64) {
	wLeverPush, bLeverPush := LeverPushBitboards(b)
	return candidatePassedBonus(b, entry.WPassedBB, entry.BPassedBB,
		entry.WLeverBB, entry.BLeverBB, wLeverPush, bLeverPush)
}

// GetPawnEntry returns a pointer to the pawn hash entry for the current position, computing it if needed.
func GetPawnEntry(b *gm.Board, debug bool) *PawnHashEntry {
	entry, hit := ProbePawnHash(b)
	if hit {
		return entry
	}
	// ProbePawnHash already returns the slot this position maps to, so write
	// through that pointer. Going via StorePawnHash recomputed pawnHashIndex a
	// second time and copied the 208-byte entry into the table on top of the
	// copy out of ComputePawnEntry, and the old tail recomputed the index a
	// third time to build the return value.
	*entry = ComputePawnEntry(b, debug)
	entry.WhitePawns = b.White.Pawns
	entry.BlackPawns = b.Black.Pawns
	entry.Valid = true
	return entry
}

func getIsolatedPawnsBitboards(b *gm.Board) (wIsolated uint64, bIsolated uint64) {
	// A pawn is isolated if no friendly pawns exist on adjacent files (same file does NOT help).
	for x := b.White.Pawns; x != 0; x &= x - 1 {
		idx := bits.TrailingZeros64(x)
		file := idx % 8
		adj := isolatedPawnTable[file]
		if (adj & b.White.Pawns) == 0 {
			wIsolated |= PositionBB[idx]
		}
	}
	for x := b.Black.Pawns; x != 0; x &= x - 1 {
		idx := bits.TrailingZeros64(x)
		file := idx % 8
		adj := isolatedPawnTable[file]
		if (adj & b.Black.Pawns) == 0 {
			bIsolated |= PositionBB[idx]
		}
	}
	return wIsolated, bIsolated
}

func getPassedPawnsBitboards(b *gm.Board) (wPassed uint64, bPassed uint64) {
	for x := b.White.Pawns; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		file := sq & 7
		rank := sq / 8
		var mask uint64
		for r := rank + 1; r <= 7; r++ {
			if file > 0 {
				mask |= onlyFile[file-1] & onlyRank[r]
			}
			mask |= onlyFile[file] & onlyRank[r]
			if file < 7 {
				mask |= onlyFile[file+1] & onlyRank[r]
			}
		}
		if (b.Black.Pawns & mask) == 0 {
			wPassed |= PositionBB[sq]
		}
	}

	for x := b.Black.Pawns; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		file := sq & 7
		rank := sq / 8
		var mask uint64
		for r := 0; r < rank; r++ {
			if file > 0 {
				mask |= onlyFile[file-1] & onlyRank[r]
			}
			mask |= onlyFile[file] & onlyRank[r]
			if file < 7 {
				mask |= onlyFile[file+1] & onlyRank[r]
			}
		}
		if (b.White.Pawns & mask) == 0 {
			bPassed |= PositionBB[sq]
		}
	}
	return wPassed, bPassed
}

func getBlockedPawnsBitboards(b *gm.Board) (wBlocked uint64, bBlocked uint64) {
	for x := b.White.Pawns; x != 0; x &= x - 1 {
		sqBB := PositionBB[bits.TrailingZeros64(x)]
		above := sqBB << 8
		if b.Black.Pawns&above > 0 {
			wBlocked |= sqBB
		}
	}
	for x := b.Black.Pawns; x != 0; x &= x - 1 {
		sqBB := PositionBB[bits.TrailingZeros64(x)]
		below := sqBB >> 8
		if (b.White.Pawns & below) > 0 {
			bBlocked |= sqBB
		}
	}
	return wBlocked, bBlocked
}

func getBackwardPawnsBitboards(b *gm.Board, wPawnAttackBB uint64, bPawnAttackBB uint64, wIsolated uint64, bIsolated uint64, wPassed uint64, bPassed uint64) (wBackward uint64, bBackward uint64) {
	// === WHITE ===
	wCandidates := b.White.Pawns &^ (wIsolated | wPassed)

	// A pawn has support if a friendly pawn is BEHIND it on an adjacent file.
	// Compute squares AHEAD of each pawn, then shift to adjacent files.
	// If pawn X is in this set, some pawn Y is behind X on an adjacent file.
	wNorthFill := calculatePawnNorthFill(b.White.Pawns)
	wAheadAdj := ((wNorthFill &^ bitboardFileA) >> 1) | ((wNorthFill &^ bitboardFileH) << 1)

	// Pawns NOT in wAheadAdj have no support behind them
	wUnsupported := wCandidates &^ wAheadAdj

	// Backward = unsupported AND advance square is enemy-controlled
	wFront := wUnsupported << 8
	wFrontEnemyCtrl := wFront & bPawnAttackBB
	wBackward = (wFrontEnemyCtrl >> 8) & wUnsupported

	// === BLACK (mirror) ===
	bCandidates := b.Black.Pawns &^ (bIsolated | bPassed)

	// For black, "behind" is higher ranks, so use south fill
	bSouthFill := calculatePawnSouthFill(b.Black.Pawns)
	bAheadAdj := ((bSouthFill &^ bitboardFileA) >> 1) | ((bSouthFill &^ bitboardFileH) << 1)

	bUnsupported := bCandidates &^ bAheadAdj

	bFront := bUnsupported >> 8
	bFrontEnemyCtrl := bFront & wPawnAttackBB
	bBackward = (bFrontEnemyCtrl << 8) & bUnsupported

	return wBackward, bBackward
}

func getPawnLeverBitboards(
	b *gm.Board,
	wPawnAttackBB uint64, bPawnAttackBB uint64,
	wPawnAttackBB_E uint64, wPawnAttackBB_W uint64,
	bPawnAttackBB_E uint64, bPawnAttackBB_W uint64,
) (
	wLever uint64, bLever uint64,
	wLeverPush uint64, bLeverPush uint64,
	wWeakLever uint64, bWeakLever uint64,
) {
	wHitTargets := wPawnAttackBB & b.Black.Pawns
	wLever = (((wHitTargets &^ bitboardFileH) >> 7) |
		((wHitTargets &^ bitboardFileA) >> 9)) & b.White.Pawns

	bHitTargets := bPawnAttackBB & b.White.Pawns
	bLever = (((bHitTargets &^ bitboardFileA) << 7) |
		((bHitTargets &^ bitboardFileH) << 9)) & b.Black.Pawns

	wDoubleAtt := wPawnAttackBB_E & wPawnAttackBB_W // squares attacked by two white pawns
	bDoubleAtt := bPawnAttackBB_E & bPawnAttackBB_W // squares attacked by two black pawns

	wWeakLever = wLever & bDoubleAtt &^ wPawnAttackBB
	bWeakLever = bLever & wDoubleAtt &^ bPawnAttackBB

	// Push levers are occupancy-dependent; delegated so per-eval callers can
	// recompute them without the pawn-keyed cache.
	wLeverPush, bLeverPush = LeverPushBitboards(b)

	return
}

// getCenterState evaluates the center structure and returns whether the core center is locked
func getCenterState(
	b *gm.Board,
	openFiles uint64,
	wSemiOpenFiles uint64,
	bSemiOpenFiles uint64,
	wLeverBB uint64,
	bLeverBB uint64,
) (locked bool, openIdx float64) {
	// Masks
	centerFiles := onlyFile[2] | onlyFile[3] | onlyFile[4] | onlyFile[5] // c-f files

	// Facing central pawns on both d and e files (rank-agnostic):
	// A file is facing if a white pawn has a black pawn one rank ahead on the same file (or vice versa).
	wD := b.White.Pawns & onlyFile[3]
	wE := b.White.Pawns & onlyFile[4]
	bD := b.Black.Pawns & onlyFile[3]
	bE := b.Black.Pawns & onlyFile[4]
	facingD := (((wD << 8) & bD) != 0) || (((bD >> 8) & wD) != 0)
	facingE := (((wE << 8) & bE) != 0) || (((bE >> 8) & wE) != 0)
	facingBoth := facingD && facingE

	// Immediate central pawn levers (either side) — if exists, do not treat as locked
	centralLeverMask := centerFiles & (onlyRank[2] | onlyRank[3] | onlyRank[4] | onlyRank[5])
	hasCentralLever := ((wLeverBB | bLeverBB) & centralLeverMask) != 0

	// If there are open files in the center, it is not locked
	centerOpen := (openFiles & centerFiles) != 0
	locked = facingBoth && !hasCentralLever && !centerOpen

	// Openness index by center files c–f (per-file, not per-square),
	// using precomputed open/semi-open file masks
	openFilesCount := 0
	semiFilesCount := 0
	for f := 2; f <= 5; f++ { // c, d, e, f
		fileMask := onlyFile[f]
		if (openFiles & fileMask) != 0 {
			openFilesCount++
		} else if ((wSemiOpenFiles | bSemiOpenFiles) & fileMask) != 0 {
			semiFilesCount++
		}
	}

	idx := (float64(openFilesCount) + 0.5*float64(semiFilesCount)) / 4.0
	if idx < 0 {
		idx = 0
	}
	if idx > 1 {
		idx = 1
	}
	openIdx = idx
	return
}

// centerScales holds integer percentage scales derived from the centre state.
// Both phases are scaled: the endgame halves used to be left alone, so the
// engine's whole read of the centre decayed to nothing as the phase moved --
// exactly where a locked structure decides knight-versus-bishop endings.
type centerScales struct {
	knightMobilityMG, knightMobilityEG int
	bishopMobilityMG, bishopMobilityEG int
	bishopPairMG, bishopPairEG         int
}

// getCenterMobilityScales converts the centre state into percentage scales for
// knight mobility, bishop mobility and the bishop-pair bonus. The scaling is
// linear in openness rather than bucketed, so a single semi-open file can no
// longer flip a 25-point swing at a threshold.
func getCenterMobilityScales(lockedCenter bool, openIdx float64) centerScales {
	// openIdx is (open + 0.5*semiOpen)/4 over the c-f files, so it is always a
	// multiple of 0.125 and openIdx*8 is a whole number in 0..8. Working in
	// eighths keeps this in integer arithmetic: openness runs -4..+4, in quarters
	// of the full swing. Go truncates integer division toward zero, which is
	// symmetric about zero, so the two colours cannot round apart.
	openness := int(openIdx*8+0.5) - 4

	if lockedCenter {
		// A locked centre is maximally closed by definition. The file count can
		// still read up to openIdx 0.25 when c or f is semi-open, so pin it here
		// rather than let the count speak. Fires in 1.4% of positions.
		openness = -4
	}

	// Knights prefer a closed centre; bishops and the pair prefer an open one.
	return centerScales{
		knightMobilityMG: 100 - openness*CenterKnightMobilityMG/4,
		knightMobilityEG: 100 - openness*CenterKnightMobilityEG/4,
		bishopMobilityMG: 100 + openness*CenterBishopMobilityMG/4,
		bishopMobilityEG: 100 + openness*CenterBishopMobilityEG/4,
		bishopPairMG:     100 + openness*CenterBishopPairMG/4,
		bishopPairEG:     100 + openness*CenterBishopPairEG/4,
	}
}

func chebyshevDistance(sq1, sq2 int) int {
	file1, rank1 := sq1%8, sq1/8
	file2, rank2 := sq2%8, sq2/8
	fileDiff := absInt(file1 - file2)
	rankDiff := absInt(rank1 - rank2)
	if fileDiff > rankDiff {
		return fileDiff
	}
	return rankDiff
}

func getRookConnectedFiles(b *gm.Board) (wFiles uint64, bFiles uint64) {
	allPieces := b.White.All | b.Black.All

	// Helper: evaluate one side
	evalSide := func(rooks uint64, ignore uint64) uint64 {
		var files uint64
		for file := 0; file < 8; file++ {
			fileMask := onlyFile[file]
			rOnFile := rooks & fileMask
			if bits.OnesCount64(rOnFile) < 2 {
				continue
			}
			// Find min/max rank rook squares on this file
			minR := 8
			maxR := -1
			for x := rOnFile; x != 0; x &= x - 1 {
				sq := bits.TrailingZeros64(x)
				r := sq / 8
				if r < minR {
					minR = r
				}
				if r > maxR {
					maxR = r
				}
			}
			if maxR-minR <= 1 {
				files |= fileMask
				continue
			}
			// Build between mask along file
			between := uint64(0)
			for r := minR + 1; r <= maxR-1; r++ {
				between |= PositionBB[file+8*r]
			}
			blockers := between & (allPieces &^ ignore)
			if blockers == 0 {
				files |= fileMask
			}
		}
		return files
	}

	wFiles = evalSide(b.White.Rooks, b.White.Bishops|b.White.Knights)
	bFiles = evalSide(b.Black.Rooks, b.Black.Bishops|b.Black.Knights)
	return wFiles, bFiles
}

func calculatePawnNorthFill(pawnBitboard uint64) uint64 {
	pawnBitboard = (pawnBitboard << 8)
	pawnBitboard |= (pawnBitboard << 8)
	pawnBitboard |= (pawnBitboard << 16)
	pawnBitboard |= (pawnBitboard << 32)
	return pawnBitboard
}

func calculatePawnSouthFill(pawnBitboard uint64) uint64 {
	pawnBitboard = (pawnBitboard >> 8)
	pawnBitboard |= (pawnBitboard >> 8)
	pawnBitboard |= (pawnBitboard >> 16)
	pawnBitboard |= (pawnBitboard >> 32)
	return pawnBitboard
}

func isTheoreticalDraw(board *gm.Board, debug bool) bool {
	pawnCount := bits.OnesCount64(board.White.Pawns | board.Black.Pawns)

	wKnights := bits.OnesCount64(board.White.Knights)
	wBishops := bits.OnesCount64(board.White.Bishops)
	wRooks := bits.OnesCount64(board.White.Rooks)
	wQueens := bits.OnesCount64(board.White.Queens)

	bKnights := bits.OnesCount64(board.Black.Knights)
	bBishops := bits.OnesCount64(board.Black.Bishops)
	bRooks := bits.OnesCount64(board.Black.Rooks)
	bQueens := bits.OnesCount64(board.Black.Queens)

	allPieces := bits.OnesCount64((board.White.All | board.Black.All) & ^(board.White.Kings | board.Black.Kings))
	/*
		GENERAL DRAWS:
			NO PIECES:
				- bare kings				✓
			ONE PIECE:
				- One knight				✓
				- One bishop				✓
			TWO PIECES:
				- two knights (same side)	✓
				- knight v knight			✓
				- bishop v bishop			✓
				- bishop v knight			✓
				- rook v rook				✓
				- queen v queen				✓

	*/
	if pawnCount == 0 {
		if allPieces == 0 { // bare kings
			return true
		} else if allPieces == 1 { // single piece draw
			if wKnights == 1 || wBishops == 1 || bKnights == 1 || bBishops == 1 {
				return true
			}
		} else if allPieces == 2 { // Draws with only two major/minor pieces (where it generally is a draw)
			if (wKnights == 2 || bKnights == 2) || ((wBishops+wKnights > 0 && wBishops+wKnights < 2) && (bBishops+bKnights > 0 && bBishops+bKnights < 2)) {
				return true
			} else if (wRooks == 1 && (bBishops == 1 || bKnights == 1 || bRooks == 1)) || (bRooks == 1 && (wBishops == 1 || wKnights == 1 || wRooks == 1)) {
				return true
			} else if wQueens == 1 && bQueens == 1 {
				return true
			}
		}
	}

	return false
}

func getKingFileZone(kingFile int) uint64 {
	var zone uint64
	for f := max(0, kingFile-1); f <= min(7, kingFile+1); f++ {
		zone |= onlyFile[f]
	}
	return zone
}
