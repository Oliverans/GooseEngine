package engine

import (
	"math/bits"

	gm "chess-engine/goosemg"
)

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
	if (i >= min) && (i <= max) {
		return true
	} else {
		return false
	}
}

var (
	bitboardFileA uint64 = 0x0101010101010101
	bitboardFileH uint64 = 0x8080808080808080
)
var ClearRank [8]uint64
var MaskRank [8]uint64
var ranksAbove = [8]uint64{0xffffffffffffffff, 0xffffffffffffff00, 0xffffffffffff0000, 0xffffffffff000000, 0xffffffff00000000, 0xffffff0000000000, 0xffff000000000000, 0xff00000000000000}
var ranksBelow = [8]uint64{0xff, 0xffff, 0xffffff, 0xffffffff, 0xffffffffff, 0xffffffffffff, 0xffffffffffffff, 0xffffffffffffffff}

func getFileOfSquare(sq int) uint64 {
	return onlyFile[sq%8]
}

func getKingSafetyTable(b *gm.Board, inner bool, wPawnAttackBB uint64, bPawnAttackBB uint64) [2]uint64 {
	var kingZoneTable [2]uint64
	kingBoards := [2]uint64{
		0: b.White.Kings,
		1: b.Black.Kings,
	}

	for i, board := range kingBoards {
		kingZoneBBInner := board
		kingSquare := bits.TrailingZeros64(kingZoneBBInner)
		rank := kingSquare / 8
		file := kingSquare % 8

		// If we're at the bottom or top rank, we should still keep the size of the king zone at a minimum of 3 wide/high
		if rank == 0 {
			kingZoneBBInner = kingZoneBBInner | (kingZoneBBInner << 8) | (kingZoneBBInner << 16)
		} else if rank == 7 {
			kingZoneBBInner = kingZoneBBInner | (kingZoneBBInner >> 8) | (kingZoneBBInner >> 16)
		} else {
			kingZoneBBInner = kingZoneBBInner | (kingZoneBBInner << 8) | (kingZoneBBInner >> 8)
		}

		if file == 0 {
			kingZoneBBInner = kingZoneBBInner | (kingZoneBBInner << 1) | (kingZoneBBInner << 2)
		} else if file == 7 {
			kingZoneBBInner = kingZoneBBInner | (kingZoneBBInner >> 1) | (kingZoneBBInner >> 2)
		} else {
			kingZoneBBInner = kingZoneBBInner | (((kingZoneBBInner & ^bitboardFileA) >> 1) | ((kingZoneBBInner & ^bitboardFileH) << 1))
		}

		if i == 0 {
			kingZoneBBInner &^= wPawnAttackBB
		} else {
			kingZoneBBInner &^= bPawnAttackBB
		}

		kingZoneTable[i] = kingZoneBBInner
	}

	if !inner {
		for i, board := range kingZoneTable {
			kingZoneBBOuter := board
			kingZoneBBOuter = kingZoneBBOuter | (kingZoneBBOuter << 8) | (kingZoneBBOuter >> 8)
			kingZoneBBOuter = kingZoneBBOuter | (((kingZoneBBOuter & ^bitboardFileA) >> 1) | ((kingZoneBBOuter & ^bitboardFileH) << 1))
			kingZoneBBOuter = kingZoneBBOuter &^ kingZoneTable[i]
			kingZoneTable[i] = kingZoneBBOuter
		}
	}
	return kingZoneTable
}

func getOutpostsBB(b *gm.Board, wPawnAttackBB uint64, bPawnAttackBB uint64) (outpostSquares [2]uint64) {
	// Generate allowed ranks & files for outposts to be on
	wPotentialOutposts := (wPawnAttackBB & wAllowedOutpostMask) &^ b.White.Pawns
	bPotentialOutposts := (bPawnAttackBB & bAllowedOutpostMask) &^ b.Black.Pawns

	var wOutpostBB uint64
	var bOutpostBB uint64

	for x := wPotentialOutposts; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		sqBB := PositionBB[sq]
		if bits.OnesCount64(sqBB&wPotentialOutposts) > 0 {
			filesToCheck := (getFileOfSquare(sq-1) &^ bitboardFileH) | (getFileOfSquare(sq+1) &^ bitboardFileA)
			var ranksToCheckForEnemyPawns = ranksAbove[(sq/8)+1]
			if bits.OnesCount64(b.Black.Pawns&(filesToCheck&ranksToCheckForEnemyPawns)) == 0 {
				wOutpostBB = wOutpostBB | sqBB
			}
		}
	}

	for x := bPotentialOutposts; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		sqBB := PositionBB[sq]
		if bits.OnesCount64(sqBB&bPotentialOutposts) > 0 {
			filesToCheck := (getFileOfSquare(sq-1) &^ bitboardFileH) | (getFileOfSquare(sq+1) &^ bitboardFileA)
			var ranksToCheckForEnemyPawns = ranksBelow[(sq/8)-1]
			if bits.OnesCount64(b.White.Pawns&(filesToCheck&ranksToCheckForEnemyPawns)) == 0 {
				bOutpostBB = bOutpostBB | sqBB
			}
		}
	}

	outpostSquares[0] = wOutpostBB
	outpostSquares[1] = bOutpostBB
	return
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

// getBackwardPawnsBitboards returns bitboards of backward pawns for each side.
func getBackwardPawnsBitboards(b *gm.Board, wPawnAttackBB uint64, bPawnAttackBB uint64, wIsolated uint64, bIsolated uint64, wPassed uint64, bPassed uint64) (wBackward uint64, bBackward uint64) {
	// White candidates: exclude isolated and passed
	wCandidates := b.White.Pawns &^ (wIsolated | wPassed)
	// Friendly support behind on adjacent files (south fill for white)
	wSouthFill := calculatePawnSouthFill(b.White.Pawns)
	wAdjBehind := ((wSouthFill & ^bitboardFileA) >> 1) | ((wSouthFill & ^bitboardFileH) << 1)
	// Propagate forward to cover current pawn squares
	wAdjBehindForward := wAdjBehind | calculatePawnNorthFill(wAdjBehind)
	wCandidates &^= wAdjBehindForward
	// Require enemy pawn control of advance square
	wFront := b.White.Pawns << 8
	wFrontEnemyCtrl := wFront & bPawnAttackBB
	wBackward = wCandidates & wFrontEnemyCtrl

	// Black side (mirror)
	bCandidates := b.Black.Pawns &^ (bIsolated | bPassed)
	bNorthFill := calculatePawnNorthFill(b.Black.Pawns)
	bAdjBehind := ((bNorthFill & ^bitboardFileA) >> 1) | ((bNorthFill & ^bitboardFileH) << 1)
	bAdjBehindBackward := bAdjBehind | calculatePawnSouthFill(bAdjBehind)
	bCandidates &^= bAdjBehindBackward
	bFront := b.Black.Pawns >> 8
	bFrontEnemyCtrl := bFront & wPawnAttackBB
	bBackward = bCandidates & bFrontEnemyCtrl

	return wBackward, bBackward
}

// getPawnLeverBitboards marks pawns that can immediately capture an enemy pawn.
func getPawnLeverBitboards(b *gm.Board, wPawnAttackBB uint64, bPawnAttackBB uint64) (wLever uint64, bLever uint64) {
	wLever = wPawnAttackBB & b.Black.Pawns
	bLever = bPawnAttackBB & b.White.Pawns
	return wLever, bLever
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

// scoreRookStacksMG returns a midgame-only bonus for connected rook stacks per side.
func scoreRookStacksMG(wFiles uint64, bFiles uint64) (mg int) {
	wCount := bits.OnesCount64(wFiles) / 8
	bCount := bits.OnesCount64(bFiles) / 8
	mg = (wCount * StackedRooksMG) - (bCount * StackedRooksMG)
	return mg
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
