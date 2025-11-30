package engine

import (
	"math/bits"

	gm "chess-engine/goosemg"
)

// Knight move masks for each square (precomputed bitboards)
var KnightMasks = [64]uint64{ /* ... (omitted for brevity) ... */ }

func InBetween(i, min, max int) bool {
	return i >= min && i <= max
}

// File bitboard masks for files A and H (for shifting operations)
var (
	bitboardFileA uint64 = 0x0101010101010101
	bitboardFileH uint64 = 0x8080808080808080
)
var ClearRank [8]uint64 // (not used in evaluation.go but may be set elsewhere)
var MaskRank [8]uint64
var ranksAbove = [8]uint64{
	0xffffffffffffffff, 0xffffffffffffff00, 0xffffffffffff0000, 0xffffffffff000000,
	0xffffffff00000000, 0xffffff0000000000, 0xffff000000000000, 0xff00000000000000,
}
var ranksBelow = [8]uint64{
	0x00000000000000ff, 0x000000000000ffff, 0x0000000000ffffff, 0x00000000ffffffff,
	0x000000ffffffffff, 0x0000ffffffffffff, 0x00ffffffffffffff, 0xffffffffffffffff,
}

// Return the file mask for a given square index (0-63)
func getFileOfSquare(sq int) uint64 {
	return onlyFile[sq%8]
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
		if rank == 0 {
			zone |= zone<<8 | zone<<16
		} else if rank == 7 {
			zone |= zone>>8 | zone>>16
		} else {
			zone |= zone<<8 | zone>>8
		}
		// Always include one file to left and right (with bounds check)
		if file == 0 {
			zone |= zone<<1 | zone<<2
		} else if file == 7 {
			zone |= zone>>1 | zone>>2
		} else {
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
	// White potential outposts: squares attacked by a white pawn and not occupied by a white pawn
	wCandidates := (wPawnAttackBB & wAllowedOutpostMask) &^ b.White.Pawns
	var wOutpostBB uint64
	for x := wCandidates; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		file := sq % 8
		rank := sq / 8
		// Check adjacent files for enemy pawns in front of this square
		var adjFilesMask uint64
		if file > 0 {
			adjFilesMask |= onlyFile[file-1]
		}
		if file < 7 {
			adjFilesMask |= onlyFile[file+1]
		}
		if rank < 7 {
			// no enemy pawn on adjacent files in any rank above
			if b.Black.Pawns&adjFilesMask&ranksAbove[rank+1] == 0 {
				wOutpostBB |= PositionBB[sq]
			}
		} else {
			// rank 7 pawn automatically an outpost (no rank above)
			wOutpostBB |= PositionBB[sq]
		}
	}
	// Black potential outposts (symmetric)
	bCandidates := (bPawnAttackBB & bAllowedOutpostMask) &^ b.Black.Pawns
	var bOutpostBB uint64
	for x := bCandidates; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		file := sq % 8
		rank := sq / 8
		var adjFilesMask uint64
		if file > 0 {
			adjFilesMask |= onlyFile[file-1]
		}
		if file < 7 {
			adjFilesMask |= onlyFile[file+1]
		}
		if rank > 0 {
			if b.White.Pawns&adjFilesMask&ranksBelow[rank-1] == 0 {
				bOutpostBB |= PositionBB[sq]
			}
		} else {
			bOutpostBB |= PositionBB[sq]
		}
	}
	outposts[0] = wOutpostBB
	outposts[1] = bOutpostBB
	return
}

// Determine a basic material value (for x-ray logic)
func getPieceValue(pieceBB uint64, side *gm.Bitboards) int {
	switch {
	case pieceBB&side.Pawns != 0:
		return 1
	case pieceBB&side.Knights != 0:
		return 3
	case pieceBB&side.Bishops != 0:
		return 3
	case pieceBB&side.Rooks != 0:
		return 5
	case pieceBB&side.Queens != 0:
		return 9
	default:
		return 0
	}
}

// =============================================================================
// PAWN HASH TABLE
// =============================================================================

const PawnHashSize = 1 << 16 // 65536 entries (~8MB)

// PawnHashEntry stores cached pawn structure analysis
type PawnHashEntry struct {
	// Key for verifying collisions (pawn bitboards)
	WhitePawns uint64
	BlackPawns uint64

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
	WLeverBB     uint64
	BLeverBB     uint64
	WWeakLeverBB uint64
	BWeakLeverBB uint64

	// Precomputed pawn scores
	PawnScoreMG int
	PawnScoreEG int

	Valid bool // flag to mark valid entries
}

var PawnHashTable [PawnHashSize]PawnHashEntry

// Compute index into pawn hash table from pawn bitboards (mix bits for distribution)
func pawnHashIndex(whitePawns, blackPawns uint64) uint64 {
	const goldenRatio = 0x9E3779B97F4A7C15
	hash := whitePawns ^ (blackPawns * goldenRatio)
	hash ^= hash >> 33
	hash *= 0xFF51AFD7ED558CCD
	hash ^= hash >> 33
	return hash & (PawnHashSize - 1)
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
	entry.WPassedBB, entry.BPassedBB = getPassedPawnsBitboards(b, entry.WPawnAttackBB, entry.BPawnAttackBB)
	entry.WBlockedBB, entry.BBlockedBB = getBlockedPawnsBitboards(b)
	entry.WBackwardBB, entry.BBackwardBB = getBackwardPawnsBitboards(b, entry.WPawnAttackBB, entry.BPawnAttackBB, entry.WIsolatedBB, entry.BIsolatedBB, entry.WPassedBB, entry.BPassedBB)
	wLever, bLever, wMultiLever, bMultiLever := getPawnLeverBitboards(b)
	entry.WLeverBB = wLever
	entry.BLeverBB = bLever
	// Weak levers: multi-lever pawns not supported by a friendly pawn
	wSupported := entry.WPawnAttackBB & b.White.Pawns
	bSupported := entry.BPawnAttackBB & b.Black.Pawns
	entry.WWeakLeverBB = wMultiLever &^ wSupported
	entry.BWeakLeverBB = bMultiLever &^ bSupported

	// 4. Pawn score components
	pawnPsqtMG, pawnPsqtEG := countPieceTables(&b.White.Pawns, &b.Black.Pawns, &PSQT_MG[gm.PieceTypePawn], &PSQT_EG[gm.PieceTypePawn])
	isoMG, isoEG := isolatedPawnPenalty(entry.WIsolatedBB, entry.BIsolatedBB)
	doubledMG, doubledEG := pawnDoublingPenalties(b)
	connMG, connEG, phalMG, phalEG := connectedOrPhalanxPawnBonus(b, entry.WPawnAttackBB, entry.BPawnAttackBB)
	passedMG, passedEG := passedPawnBonus(entry.WPassedBB, entry.BPassedBB)
	blockedMG, blockedEG := blockedPawnBonus(entry.WBlockedBB, entry.BBlockedBB)
	backMG, backEG := backwardPawnPenalty(entry.WBackwardBB, entry.BBackwardBB)
	weakLeverMG, weakLeverEG := pawnWeakLeverPenalty(entry.WWeakLeverBB, entry.BWeakLeverBB)

	// Sum all pawn contributions
	entry.PawnScoreMG = pawnPsqtMG + isoMG + doubledMG + connMG + phalMG + passedMG + blockedMG + backMG + weakLeverMG
	entry.PawnScoreEG = pawnPsqtEG + isoEG + doubledEG + connEG + phalEG + passedEG + blockedEG + backEG + weakLeverEG

	if debug {
		println("################### PAWN PARAMETERS ###################")
		println("Pawn MG:\t", "PSQT: ", pawnPsqtMG, "\tIsolated: ", isoMG, "\tDoubled: ", doubledMG,
			"\tConnected: ", connMG, "\tPhalanx: ", phalMG, "\tPassed: ", passedMG,
			"\tBlocked: ", blockedMG, "\tBackward: ", backMG, "\tWeakLever: ", weakLeverMG)
		println("Pawn EG:\t", "PSQT: ", pawnPsqtEG, "\tIsolated: ", isoEG, "\tDoubled: ", doubledEG,
			"\tConnected: ", connEG, "\tPhalanx: ", phalEG, "\tPassed: ", passedEG,
			"\tBlocked: ", blockedEG, "\tBackward: ", backEG, "\tWeakLever: ", weakLeverEG)
	}
	return entry
}

// GetPawnEntry returns a pointer to the pawn hash entry for the current position, computing it if needed.
func GetPawnEntry(b *gm.Board, debug bool) *PawnHashEntry {
	entry, hit := ProbePawnHash(b)
	if hit {
		return entry
	}
	newEntry := ComputePawnEntry(b, debug)
	StorePawnHash(b, &newEntry)
	idx := pawnHashIndex(b.White.Pawns, b.Black.Pawns)
	return &PawnHashTable[idx]
}

// getIsolatedPawnsBitboards returns bitboards of isolated pawns for each side.
func getIsolatedPawnsBitboards(b *gm.Board) (wIsolated uint64, bIsolated uint64) {
	// A pawn is isolated if no friendly pawns exist on adjacent files
	for x := b.White.Pawns; x != 0; x &= x - 1 {
		idx := bits.TrailingZeros64(x)
		file := idx % 8
		neighbors := bits.OnesCount64(isolatedPawnTable[file]&b.White.Pawns) - 1
		if neighbors == 0 {
			wIsolated |= PositionBB[idx]
		}
	}
	for x := b.Black.Pawns; x != 0; x &= x - 1 {
		idx := bits.TrailingZeros64(x)
		file := idx % 8
		neighbors := bits.OnesCount64(isolatedPawnTable[file]&b.Black.Pawns) - 1
		if neighbors == 0 {
			bIsolated |= PositionBB[idx]
		}
	}
	return wIsolated, bIsolated
}

// getPassedPawnsBitboards returns bitboards of passed pawns for each side.
func getPassedPawnsBitboards(b *gm.Board, wPawnAttackBB uint64, bPawnAttackBB uint64) (wPassed uint64, bPassed uint64) {
	for x := b.White.Pawns; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		pawnFile := onlyFile[sq%8]
		checkAbove := ranksAbove[(sq/8)+1]
		span := pawnFile & checkAbove
		if bits.OnesCount64(bPawnAttackBB&span) == 0 && bits.OnesCount64(b.Black.Pawns&span) == 0 {
			wPassed |= PositionBB[sq]
		}
	}
	for x := b.Black.Pawns; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		pawnFile := onlyFile[sq%8]
		checkBelow := ranksBelow[(sq/8)-1]
		span := pawnFile & checkBelow
		if bits.OnesCount64(wPawnAttackBB&span) == 0 && bits.OnesCount64(b.White.Pawns&span) == 0 {
			bPassed |= PositionBB[sq]
		}
	}
	return wPassed, bPassed
}

// getBlockedPawnsBitboards returns bitboards of advanced pawns blocked directly by enemy pawns.
func getBlockedPawnsBitboards(b *gm.Board) (wBlocked uint64, bBlocked uint64) {
	thirdAndFourthRank := onlyRank[2] | onlyRank[3]
	fifthAndSixthRank := onlyRank[4] | onlyRank[5]

	for x := b.White.Pawns; x != 0; x &= x - 1 {
		sqBB := PositionBB[bits.TrailingZeros64(x)]
		above := sqBB << 8
		if (fifthAndSixthRank&sqBB) > 0 && (b.Black.Pawns&above) > 0 {
			wBlocked |= sqBB
		}
	}
	for x := b.Black.Pawns; x != 0; x &= x - 1 {
		sqBB := PositionBB[bits.TrailingZeros64(x)]
		above := sqBB >> 8
		if (thirdAndFourthRank&sqBB) > 0 && (b.White.Pawns&above) > 0 {
			bBlocked |= sqBB
		}
	}
	return wBlocked, bBlocked
}

// getBackwardPawnsBitboards returns bitboards of backward pawns for each side.
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

// getPawnLeverBitboards identifies lever pawns for each side and also returns
// pawns whose advance squares are targeted by multiple enemy pawns (pre-condition
// for weak levers).
func getPawnLeverBitboards(b *gm.Board) (wLever uint64, bLever uint64, wMultiLever uint64, bMultiLever uint64) {
	wPawnAttackWest, wPawnAttackEast := PawnCaptureBitboards(b.White.Pawns, true)
	bPawnAttackWest, bPawnAttackEast := PawnCaptureBitboards(b.Black.Pawns, false)

	wHitsBPawnWest := wPawnAttackWest & b.Black.Pawns
	wHitsBPawnEast := wPawnAttackEast & b.Black.Pawns
	wLeverFromWest := ((wHitsBPawnWest &^ bitboardFileH) >> 7) & b.White.Pawns
	wLeverFromEast := ((wHitsBPawnEast &^ bitboardFileA) >> 9) & b.White.Pawns
	wLever = wLeverFromWest | wLeverFromEast

	bHitsWPawnWest := bPawnAttackWest & b.White.Pawns
	bHitsWPawnEast := bPawnAttackEast & b.White.Pawns

	bLeverFromWest := ((bHitsWPawnWest &^ bitboardFileH) << 9) & b.Black.Pawns
	bLeverFromEast := ((bHitsWPawnEast &^ bitboardFileA) << 7) & b.Black.Pawns
	bLever = bLeverFromWest | bLeverFromEast

	// Multi-lever: pawns whose advance square is attacked by BOTH enemy pawn directions
	wFront := b.White.Pawns << 8
	bFront := b.Black.Pawns >> 8
	wMultiTargets := wFront & bPawnAttackWest & bPawnAttackEast
	bMultiTargets := bFront & wPawnAttackWest & wPawnAttackEast
	wMultiLever = (wMultiTargets >> 8) & b.White.Pawns
	bMultiLever = (bMultiTargets << 8) & b.Black.Pawns

	return wLever, bLever, wMultiLever, bMultiLever
}

// getPawnStormBitboards marks pawns advanced on the enemy king wing (attacking direction).
func getPawnStormBitboards(b *gm.Board, wWing uint64, bWing uint64) (wStorm uint64, bStorm uint64) {
	// White storm on black king wing, advanced (rank 4+)
	wStorm = b.White.Pawns & bWing & ranksAbove[3]
	// Black storm on white king wing, advanced (rank <= 5 from white perspective)
	bStorm = b.Black.Pawns & wWing & ranksBelow[4]
	return wStorm, bStorm
}

// getEnemyPawnProximityBitboards marks enemy pawns advanced on our king wing (potential threats).
func getEnemyPawnProximityBitboards(b *gm.Board, wWing uint64, bWing uint64) (wProx uint64, bProx uint64) {
	// Enemy near our king wing
	wProx = b.Black.Pawns & wWing & ranksAbove[3]
	bProx = b.White.Pawns & bWing & ranksBelow[4]
	return wProx, bProx
}

// getCenterState evaluates the center structure and returns whether the core center is locked
// and an openness index in [0.0, 1.0] based on open/semi-open center files.
// locked considers facing central pawns (d/e files) without immediate central pawn levers.
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

// getCenterMobilityScales returns simple integer percentage scales for
// knight mobility, bishop mobility, and bishop-pair bonus based on
// center state (lockedCenter) and openness index (0..1).
func getCenterMobilityScales(lockedCenter bool, openIdx float64) (knMobScale int, biMobScale int, bpScaleMG int) {
	// Defaults: no scaling
	knMobScale = 100
	biMobScale = 100
	bpScaleMG = 100

	if lockedCenter {
		// Fully locked center favors knights, penalizes bishops and bishop pair
		knMobScale += 20
		biMobScale -= 10
		bpScaleMG -= 10
		return
	}

	if openIdx >= 0.75 {
		// Very open center favors bishops
		knMobScale -= 10
		biMobScale += 15
		bpScaleMG += 20
		return
	}

	if openIdx <= 0.25 {
		// Quite closed center mildly favors knights
		knMobScale += 10
		biMobScale -= 5
		bpScaleMG -= 5
	}
	return
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

// getRookConnectedFiles returns file masks where each side has two or more rooks
// connected on the same file with no blockers between them, ignoring own bishops/knights.
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

// getKingWingMasks returns wing masks (a-c or f-h) for each king, choosing the nearest wing for d/e.
func getKingWingMasks(b *gm.Board) (wWing uint64, bWing uint64) {
	wSq := bits.TrailingZeros64(b.White.Kings)
	bSq := bits.TrailingZeros64(b.Black.Kings)
	wFile := wSq % 8
	bFile := bSq % 8
	qSide := onlyFile[0] | onlyFile[1] | onlyFile[2]
	kSide := onlyFile[5] | onlyFile[6] | onlyFile[7]
	// d/e choose nearest wing
	if wFile <= 2 {
		wWing = qSide
	} else if wFile >= 5 {
		wWing = kSide
	} else if wFile == 3 { // d-file
		wWing = qSide
	} else { // e-file
		wWing = kSide
	}
	if bFile <= 2 {
		bWing = qSide
	} else if bFile >= 5 {
		bWing = kSide
	} else if bFile == 3 {
		bWing = qSide
	} else {
		bWing = kSide
	}
	return wWing, bWing
}

func calculatePawnFileFill(pawnBitboard uint64, isWhite bool) uint64 {
	if isWhite {
		pawnBitboard |= calculatePawnNorthFill(pawnBitboard)
	} else {
		pawnBitboard |= calculatePawnSouthFill(pawnBitboard)
	}
	return pawnBitboard
}

func calculatePawnNorthFill(pawnBitboard uint64) uint64 {
	pawnBitboard = (pawnBitboard << 8)
	pawnBitboard |= (pawnBitboard << 16)
	pawnBitboard |= (pawnBitboard << 32)
	return pawnBitboard
}

func calculatePawnSouthFill(pawnBitboard uint64) uint64 {
	pawnBitboard = (pawnBitboard >> 8)
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
	if debug {
		println("All: ", allPieces, "\twQueen: ", wQueens, "\twRooks: ", wRooks, "\twKnights: ", wKnights, "\twBishops: ", wBishops)
		println("All: ", allPieces, "\tbQueen: ", bQueens, "\tbRooks: ", bRooks, "\tbKnights: ", bKnights, "\tbBishops: ", bBishops)
	}

	/*
		GENERAL DRAWS:
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
		if allPieces == 1 { // single piece draw
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
