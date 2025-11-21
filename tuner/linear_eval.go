// tuner/linear_eval.go
package tuner

import (
	"math/bits"

	eng "chess-engine/engine"
	gm "chess-engine/goosemg"
)

// LinearEval: PST + Material + Passed Pawns (square-based)
type LinearEval struct {
	PST *PST

	// Material weights (centipawns per piece), MG/EG. Index by P..K.
	MatMG [6]float64
	MatEG [6]float64

	// Passed pawn square weights (white perspective), MG/EG. Index by square 0..63.
	PasserMG [64]float64
	PasserEG [64]float64

	// Phase 1 scalar terms
	BishopPairMG        float64
	BishopPairEG        float64
	RookSemiOpenFileMG  float64
	RookOpenFileMG      float64
	SeventhRankEG       float64
	QueenCentralizedEG  float64
	QueenInfiltrationMG float64
	QueenInfiltrationEG float64

	// Phase 2: Pawn structure scalars (MG/EG)
	DoubledMG   float64
	DoubledEG   float64
	IsolatedMG  float64
	IsolatedEG  float64
	ConnectedMG float64
	ConnectedEG float64
	PhalanxMG   float64
	PhalanxEG   float64
	BlockedMG   float64
	BlockedEG   float64
	PawnLeverMG float64
	PawnLeverEG float64
	WeakLeverMG float64
	WeakLeverEG float64
	BackwardMG  float64
	BackwardEG  float64

	// Phase 3: Mobility weights only
	MobilityMG [7]float64 // keyed by gm.PieceType
	MobilityEG [7]float64

	// Flat parameter buffer
	theta []float64

	// Phase 4: King safety table (100 entries)
	KingSafety [100]float64
	// Phase 4 correlates (MG-only)
	KingSemiOpenFilePenalty float64
	KingOpenFilePenalty     float64
	KingMinorPieceDefense   float64
	KingPawnDefenseMG       float64

	// Phase 5: Extras (piece-related scalars)
	KnightOutpostMG  float64
	KnightOutpostEG  float64
	BishopOutpostMG  float64
	KnightThreatsMG  float64
	KnightThreatsEG  float64
	StackedRooksMG   float64
	RookXrayQueenMG  float64
	ConnectedRooksMG float64
	// New extras/tunables
	BishopXrayKingMG  float64
	BishopXrayRookMG  float64
	BishopXrayQueenMG float64
	PawnStormMG       float64
	PawnProximityMG   float64
	PawnLeverStormMG  float64
	KnightMobCenterMG float64
	BishopMobCenterMG float64

	// Phase 6: material imbalance scalars
	ImbalanceKnightPerPawnMG    float64
	ImbalanceKnightPerPawnEG    float64
	ImbalanceBishopPerPawnMG    float64
	ImbalanceBishopPerPawnEG    float64
	ImbalanceMinorsForMajorMG   float64
	ImbalanceMinorsForMajorEG   float64
	ImbalanceRedundantRookMG    float64
	ImbalanceRedundantRookEG    float64
	ImbalanceRookQueenOverlapMG float64
	ImbalanceRookQueenOverlapEG float64
	ImbalanceQueenManyMinorsMG  float64
	ImbalanceQueenManyMinorsEG  float64

	// Per-position scratch cache to avoid recomputing expensive engine wrappers
	cache struct {
		pos                                       *gm.Board
		pawnMG                                    [8]int
		pawnEG                                    [8]int
		mobMG                                     [7]int
		mobEG                                     [7]int
		ksOneHot                                  [100]int
		ksSemiOpen, ksOpen, ksMinorDef, ksPawnDef int
		exMG                                      [7]int
		exEG                                      [2]int
		imbMG                                     [6]int
		imbEG                                     [6]int
	}

	// Toggles and consolidated θ layout
	Toggles PhaseToggles
	layout  Layout

	// Phase 6: Weak squares + Tempo
	WeakSquaresMG     float64
	WeakKingSquaresMG float64
	Tempo             float64
}

// Eval computes a white-positive evaluation using linear terms with MG/EG tapering.
func (le *LinearEval) Eval(pos *Position) float64 {
	if le == nil || le.PST == nil || pos == nil {
		return 0
	}
	le.ensureToggles()
	mgf, egf := boardTaperedPhases(pos)
	var mg, eg float64
	if le.Toggles.PSTEval {
		mtg, etg := evalPSTBoard(le.PST, pos)
		mg += mtg
		eg += etg
	}

	// Material (white minus black counts)
	if le.Toggles.MaterialEval {
		var wCnt, bCnt [6]int
		wCnt[P] = bits.OnesCount64(pos.White.Pawns)
		wCnt[N] = bits.OnesCount64(pos.White.Knights)
		wCnt[B] = bits.OnesCount64(pos.White.Bishops)
		wCnt[R] = bits.OnesCount64(pos.White.Rooks)
		wCnt[Q] = bits.OnesCount64(pos.White.Queens)
		wCnt[K] = bits.OnesCount64(pos.White.Kings)
		bCnt[P] = bits.OnesCount64(pos.Black.Pawns)
		bCnt[N] = bits.OnesCount64(pos.Black.Knights)
		bCnt[B] = bits.OnesCount64(pos.Black.Bishops)
		bCnt[R] = bits.OnesCount64(pos.Black.Rooks)
		bCnt[Q] = bits.OnesCount64(pos.Black.Queens)
		bCnt[K] = bits.OnesCount64(pos.Black.Kings)
		for pt := 0; pt < 6; pt++ {
			diff := float64(wCnt[pt] - bCnt[pt])
			mg += diff * le.MatMG[pt]
			eg += diff * le.MatEG[pt]
		}
	}

	// Passed pawns (square-based)
	if le.Toggles.PassersEval {
		wPassed, bPassed := passedPawns(pos)
		for m := wPassed; m != 0; m &= m - 1 {
			sq := bits.TrailingZeros64(m)
			mg += le.PasserMG[sq]
			eg += le.PasserEG[sq]
		}
		for m := bPassed; m != 0; m &= m - 1 {
			sq := bits.TrailingZeros64(m)
			rev := flipView[sq]
			mg -= le.PasserMG[rev]
			eg -= le.PasserEG[rev]
		}
	}

	// Phase 1 scalars
	if le.Toggles.P1Eval {
		// Bishop pair (engine-logic + MG center scaling)
		bpMG, bpEG := eng.BishopPairDiffsScaled((*gm.Board)(pos))
		mg += float64(bpMG) * le.BishopPairMG
		eg += float64(bpEG) * le.BishopPairEG

		// Rook file bonuses (semi-open/open) in MG: match engine masks
		var whiteFiles uint64 = 0
		for x := pos.White.Pawns; x != 0; x &= x - 1 {
			sq := bits.TrailingZeros64(x)
			whiteFiles |= (uint64(0x0101010101010101) << uint(sq&7))
		}
		var blackFiles uint64 = 0
		for x := pos.Black.Pawns; x != 0; x &= x - 1 {
			sq := bits.TrailingZeros64(x)
			blackFiles |= (uint64(0x0101010101010101) << uint(sq&7))
		}
		wSemiOpenFiles := ^whiteFiles & blackFiles
		bSemiOpenFiles := ^blackFiles & whiteFiles
		openFiles := ^whiteFiles & ^blackFiles
		wSemi := bits.OnesCount64(uint64(pos.White.Rooks) & wSemiOpenFiles)
		bSemi := bits.OnesCount64(uint64(pos.Black.Rooks) & bSemiOpenFiles)
		wOpen := bits.OnesCount64(uint64(pos.White.Rooks) & openFiles)
		bOpen := bits.OnesCount64(uint64(pos.Black.Rooks) & openFiles)
		mg += float64(wSemi-bSemi) * le.RookSemiOpenFileMG
		mg += float64(wOpen-bOpen) * le.RookOpenFileMG
		// Rooks on 7th rank (EG)
		w7, b7 := 0, 0
		for wr := pos.White.Rooks; wr != 0; wr &= wr - 1 {
			if (bits.TrailingZeros64(wr) / 8) == 6 {
				w7++
			}
		}
		for br := pos.Black.Rooks; br != 0; br &= br - 1 {
			if (bits.TrailingZeros64(br) / 8) == 1 {
				b7++
			}
		}
		eg += float64(w7-b7) * le.SeventhRankEG
		// Queen centralization (EG)
		const centerMask uint64 = 0x183c3c180000
		wc := float64(bits.OnesCount64(pos.White.Queens & centerMask))
		bc := float64(bits.OnesCount64(pos.Black.Queens & centerMask))
		eg += (wc - bc) * le.QueenCentralizedEG
		// Queen infiltration (MG/EG), aligned with engine: queen occupies
		// enemy weak squares in enemy half, outside enemy pawn attack span.
		wInf, bInf := eng.QueenInfiltrationCounts((*gm.Board)(pos))
		diffInf := float64(wInf - bInf)
		mg += diffInf * le.QueenInfiltrationMG
		eg += diffInf * le.QueenInfiltrationEG
	}

	// Phase 2: Pawn structure scalars (via engine wrappers)
	if le.Toggles.PawnStructEval {
		mgDiffs, egDiffs := eng.PawnStructDiffs((*gm.Board)(pos))
		mg += float64(mgDiffs[0]) * le.DoubledMG
		eg += float64(egDiffs[0]) * le.DoubledEG
		mg += float64(mgDiffs[1]) * le.IsolatedMG
		eg += float64(egDiffs[1]) * le.IsolatedEG
		mg += float64(mgDiffs[2]) * le.ConnectedMG
		eg += float64(egDiffs[2]) * le.ConnectedEG
		mg += float64(mgDiffs[3]) * le.PhalanxMG
		eg += float64(egDiffs[3]) * le.PhalanxEG
		mg += float64(mgDiffs[4]) * le.BlockedMG
		eg += float64(egDiffs[4]) * le.BlockedEG
		mg += float64(mgDiffs[5]) * le.PawnLeverMG
		eg += float64(egDiffs[5]) * le.PawnLeverEG
		mg += float64(mgDiffs[6]) * le.WeakLeverMG
		eg += float64(egDiffs[6]) * le.WeakLeverEG
		mg += float64(mgDiffs[7]) * le.BackwardMG
		eg += float64(egDiffs[7]) * le.BackwardEG
		// cache pawn diffs
		le.cache.pos = (*gm.Board)(pos)
		le.cache.pawnMG = mgDiffs
		le.cache.pawnEG = egDiffs
	}

	// Phase 3: Mobility
	if le.Toggles.MobilityEval {
		mobMG, mobEG, _, _ := eng.MobAtkDiffs((*gm.Board)(pos))
		le.cache.mobMG = mobMG
		le.cache.mobEG = mobEG
		for pt := 1; pt <= 6; pt++ {
			mg += float64(mobMG[pt]) * le.MobilityMG[pt]
			eg += float64(mobEG[pt]) * le.MobilityEG[pt]
		}
		// Center-based MG mobility scaling for N/B using engine signals (delta from 100)
		knDelta, biDelta := eng.CenterMobilityScales((*gm.Board)(pos))
		if knDelta != 0 {
			mg += float64(mobMG[N]) * float64(knDelta) * le.KnightMobCenterMG
		}
		if biDelta != 0 {
			mg += float64(mobMG[B]) * float64(biDelta) * le.BishopMobCenterMG
		}
	}

	// Phase 4: King safety table and correlates
	if le.Toggles.KingTableEval {
		ks := eng.KingSafetyOneHot((*gm.Board)(pos))
		le.cache.ksOneHot = ks
		for i := 0; i < 100; i++ {
			mg += float64(ks[i]) * le.KingSafety[i]
			eg += float64(ks[i]) * (le.KingSafety[i] / 4.0)
		}
	}
	if le.Toggles.KingCorrEval {
		semiOpenDiff, openDiff, minorDefDiff, pawnDefDiff := eng.KingSafetyCorrelates((*gm.Board)(pos))
		le.cache.ksSemiOpen, le.cache.ksOpen, le.cache.ksMinorDef, le.cache.ksPawnDef = semiOpenDiff, openDiff, minorDefDiff, pawnDefDiff
		mg += float64(semiOpenDiff) * le.KingSemiOpenFilePenalty
		mg += float64(openDiff) * le.KingOpenFilePenalty
		mg += float64(minorDefDiff) * le.KingMinorPieceDefense
		mg += float64(pawnDefDiff) * le.KingPawnDefenseMG
	}

	// Phase 5: Extras
	if le.Toggles.ExtrasEval {
		exMG, exEG := eng.ExtrasDiffs((*gm.Board)(pos))
		le.cache.exMG = exMG
		le.cache.exEG = exEG
		mg += float64(exMG[0]) * le.KnightOutpostMG
		eg += float64(exEG[0]) * le.KnightOutpostEG
		mg += float64(exMG[1]) * le.KnightThreatsMG
		eg += float64(exEG[1]) * le.KnightThreatsEG
		mg += float64(exMG[2]) * le.StackedRooksMG
		mg += float64(exMG[3]) * le.RookXrayQueenMG
		mg += float64(exMG[4]) * le.ConnectedRooksMG
		// seventh-rank MG not used; skip exMG[5]
		mg += float64(exMG[6]) * le.BishopOutpostMG
		// Bishop x-ray counts (MG-only)
		bxK, bxR, bxQ := eng.BishopXrayCounts((*gm.Board)(pos))
		mg += float64(bxK) * le.BishopXrayKingMG
		mg += float64(bxR) * le.BishopXrayRookMG
		mg += float64(bxQ) * le.BishopXrayQueenMG
		// Pawn storm / proximity / lever-storm (MG-only)
		stDiff, prDiff, lvDiff := eng.PawnStormProxLeverDiffs((*gm.Board)(pos))
		mg += float64(stDiff) * le.PawnStormMG
		mg += float64(prDiff) * le.PawnProximityMG
		mg += float64(lvDiff) * le.PawnLeverStormMG
	}

	// Phase 6: Material imbalance scalars
	if le.Toggles.ImbalanceEval {
		imbMG, imbEG := eng.ImbalanceDiffs((*gm.Board)(pos))
		le.cache.imbMG = imbMG
		le.cache.imbEG = imbEG
		mg += float64(imbMG[0]) * le.ImbalanceKnightPerPawnMG
		eg += float64(imbEG[0]) * le.ImbalanceKnightPerPawnEG
		mg += float64(imbMG[1]) * le.ImbalanceBishopPerPawnMG
		eg += float64(imbEG[1]) * le.ImbalanceBishopPerPawnEG
		mg += float64(imbMG[2]) * le.ImbalanceMinorsForMajorMG
		eg += float64(imbEG[2]) * le.ImbalanceMinorsForMajorEG
		mg += float64(imbMG[3]) * le.ImbalanceRedundantRookMG
		eg += float64(imbEG[3]) * le.ImbalanceRedundantRookEG
		mg += float64(imbMG[4]) * le.ImbalanceRookQueenOverlapMG
		eg += float64(imbEG[4]) * le.ImbalanceRookQueenOverlapEG
		mg += float64(imbMG[5]) * le.ImbalanceQueenManyMinorsMG
		eg += float64(imbEG[5]) * le.ImbalanceQueenManyMinorsEG
	}

	// Phase 6: Weak squares + Tempo
	if le.Toggles.WeakTempoEval {
		ws, wks := eng.WeakSquaresCounts((*gm.Board)(pos))
		mg += float64(ws) * le.WeakSquaresMG
		mg += float64(wks) * le.WeakKingSquaresMG
		if pos.Wtomove {
			mg += le.Tempo
			eg += le.Tempo
		} else {
			mg -= le.Tempo
			eg -= le.Tempo
		}
	}

	return mgf*mg + egf*eg
}

func (le *LinearEval) Grad(pos *Position, scale float64, g []float64) {
	if le == nil || le.PST == nil || pos == nil || g == nil {
		return
	}
	le.ensureToggles()
	const pieceCount = 6
	const slotsPerPiece = 64
	const mgBase = 0
	const egBase = pieceCount * slotsPerPiece

	// Guard against short gradient vectors.
	const matSlots = 6
	const passSlots = 64
	const extraScalars = 8
	const pawnStructScalars = 16
	const phase3 = 14 // mobility MG/EG (14)
	const ksSlots = 100
	const ksCorr = 4
	const extras5 = 16
	const imbalanceScalars = 12
	const weakTempo = 3
	minLen := egBase + pieceCount*slotsPerPiece + matSlots*2 + passSlots*2 + extraScalars + pawnStructScalars + phase3 + ksSlots + ksCorr + extras5 + imbalanceScalars + weakTempo
	if len(g) < minLen {
		return
	}

	mgf, egf := boardTaperedPhases(pos)

	add := func(pt int, sq int, sgn float64) {
		off := pt*slotsPerPiece + sq
		g[mgBase+off] += sgn * scale * mgf
		g[egBase+off] += sgn * scale * egf
	}

	// PST gradients
	if le.Toggles.PSTTrain {
		for bb := pos.White.Pawns; bb != 0; bb &= bb - 1 {
			add(P, bits.TrailingZeros64(bb), +1)
		}
		for bb := pos.White.Knights; bb != 0; bb &= bb - 1 {
			add(N, bits.TrailingZeros64(bb), +1)
		}
		for bb := pos.White.Bishops; bb != 0; bb &= bb - 1 {
			add(B, bits.TrailingZeros64(bb), +1)
		}
		for bb := pos.White.Rooks; bb != 0; bb &= bb - 1 {
			add(R, bits.TrailingZeros64(bb), +1)
		}
		for bb := pos.White.Queens; bb != 0; bb &= bb - 1 {
			add(Q, bits.TrailingZeros64(bb), +1)
		}
		for bb := pos.White.Kings; bb != 0; bb &= bb - 1 {
			add(K, bits.TrailingZeros64(bb), +1)
		}
		mirror := func(sq int) int { return flipView[sq] }
		for bb := pos.Black.Pawns; bb != 0; bb &= bb - 1 {
			add(P, mirror(bits.TrailingZeros64(bb)), -1)
		}
		for bb := pos.Black.Knights; bb != 0; bb &= bb - 1 {
			add(N, mirror(bits.TrailingZeros64(bb)), -1)
		}
		for bb := pos.Black.Bishops; bb != 0; bb &= bb - 1 {
			add(B, mirror(bits.TrailingZeros64(bb)), -1)
		}
		for bb := pos.Black.Rooks; bb != 0; bb &= bb - 1 {
			add(R, mirror(bits.TrailingZeros64(bb)), -1)
		}
		for bb := pos.Black.Queens; bb != 0; bb &= bb - 1 {
			add(Q, mirror(bits.TrailingZeros64(bb)), -1)
		}
		for bb := pos.Black.Kings; bb != 0; bb &= bb - 1 {
			add(K, mirror(bits.TrailingZeros64(bb)), -1)
		}
	}

	// Group gradient scaling to balance magnitudes during training
	const scaleMat = 1.0
	const scalePass = 1.0

	// Material gradients
	matMGBase := egBase + pieceCount*slotsPerPiece
	matEGBase := matMGBase + 6
	if le.Toggles.MaterialTrain {
		var wCnt, bCnt [6]int
		wCnt[P] = bits.OnesCount64(pos.White.Pawns)
		wCnt[N] = bits.OnesCount64(pos.White.Knights)
		wCnt[B] = bits.OnesCount64(pos.White.Bishops)
		wCnt[R] = bits.OnesCount64(pos.White.Rooks)
		wCnt[Q] = bits.OnesCount64(pos.White.Queens)
		wCnt[K] = bits.OnesCount64(pos.White.Kings)
		bCnt[P] = bits.OnesCount64(pos.Black.Pawns)
		bCnt[N] = bits.OnesCount64(pos.Black.Knights)
		bCnt[B] = bits.OnesCount64(pos.Black.Bishops)
		bCnt[R] = bits.OnesCount64(pos.Black.Rooks)
		bCnt[Q] = bits.OnesCount64(pos.Black.Queens)
		bCnt[K] = bits.OnesCount64(pos.Black.Kings)
		for pt := 0; pt < 6; pt++ {
			diff := float64(wCnt[pt] - bCnt[pt])
			g[matMGBase+pt] += scale * mgf * scaleMat * diff
			g[matEGBase+pt] += scale * egf * scaleMat * diff
		}
	}

	// Passed pawns gradients (square-based)
	passMGBase := matEGBase + 6
	passEGBase := passMGBase + 64
	if le.Toggles.PassersTrain {
		wPassed, bPassed := passedPawns(pos)
		for m := wPassed; m != 0; m &= m - 1 {
			sq := bits.TrailingZeros64(m)
			g[passMGBase+sq] += scale * mgf * scalePass
			g[passEGBase+sq] += scale * egf * scalePass
		}
		for m := bPassed; m != 0; m &= m - 1 {
			sq := bits.TrailingZeros64(m)
			rev := flipView[sq]
			g[passMGBase+rev] -= scale * mgf * scalePass
			g[passEGBase+rev] -= scale * egf * scalePass
		}
	}

	// Scalars immediately after passers EG block
	off := passEGBase + 64
	if le.Toggles.P1Train {
		// Bishop pair
		bpDiff := 0.0
		if bits.OnesCount64(pos.White.Bishops) >= 2 {
			bpDiff += 1
		}
		if bits.OnesCount64(pos.Black.Bishops) >= 2 {
			bpDiff -= 1
		}
		g[off+0] += scale * mgf * bpDiff
		g[off+1] += scale * egf * bpDiff
		// Rook files (MG)
		sw, sb, ow, ob := 0, 0, 0, 0
		for wr := pos.White.Rooks; wr != 0; wr &= wr - 1 {
			sq := bits.TrailingZeros64(wr)
			file := sq & 7
			mask := uint64(0x0101010101010101) << uint(file)
			hasWP := (pos.White.Pawns & mask) != 0
			hasBP := (pos.Black.Pawns & mask) != 0
			if !hasWP {
				sw++
			}
			if !hasWP && !hasBP {
				ow++
			}
		}
		for br := pos.Black.Rooks; br != 0; br &= br - 1 {
			sq := bits.TrailingZeros64(br)
			file := sq & 7
			mask := uint64(0x0101010101010101) << uint(file)
			hasWP := (pos.White.Pawns & mask) != 0
			hasBP := (pos.Black.Pawns & mask) != 0
			if !hasBP {
				sb++
			}
			if !hasWP && !hasBP {
				ob++
			}
		}
		g[off+2] += scale * mgf * float64(sw-sb)
		g[off+3] += scale * mgf * float64(ow-ob)
		// Rooks on 7th (EG)
		w7, b7 := 0, 0
		for wr := pos.White.Rooks; wr != 0; wr &= wr - 1 {
			if (bits.TrailingZeros64(wr) / 8) == 6 {
				w7++
			}
		}
		for br := pos.Black.Rooks; br != 0; br &= br - 1 {
			if (bits.TrailingZeros64(br) / 8) == 1 {
				b7++
			}
		}
		g[off+4] += scale * egf * float64(w7-b7)
		// Queen centralization (EG)
		const centerMask uint64 = 0x183c3c180000
		wc := float64(bits.OnesCount64(pos.White.Queens & centerMask))
		bc := float64(bits.OnesCount64(pos.Black.Queens & centerMask))
		g[off+5] += scale * egf * (wc - bc)
		// Queen infiltration (MG/EG)
		wInf, bInf := 0, 0
		for q := pos.White.Queens; q != 0; q &= q - 1 {
			if (bits.TrailingZeros64(q) / 8) >= 5 {
				wInf++
			}
		}
		for q := pos.Black.Queens; q != 0; q &= q - 1 {
			if (bits.TrailingZeros64(q) / 8) <= 2 {
				bInf++
			}
		}
		diffInf := float64(wInf - bInf)
		g[off+6] += scale * mgf * diffInf
		g[off+7] += scale * egf * diffInf
	}

	// Phase 2 pawn structure gradients (append after Phase 1 scalars)
	off += 8
	var mgDiffs [8]int
	var egDiffs [8]int
	if le.cache.pos == (*gm.Board)(pos) {
		mgDiffs, egDiffs = le.cache.pawnMG, le.cache.pawnEG
	} else {
		mgDiffs, egDiffs = eng.PawnStructDiffs((*gm.Board)(pos))
	}
	if le.Toggles.PawnStructTrain {
		g[off+0] += scale * mgf * float64(mgDiffs[0])
		g[off+1] += scale * egf * float64(egDiffs[0])
		g[off+2] += scale * mgf * float64(mgDiffs[1])
		g[off+3] += scale * egf * float64(egDiffs[1])
		g[off+4] += scale * mgf * float64(mgDiffs[2])
		g[off+5] += scale * egf * float64(egDiffs[2])
		g[off+6] += scale * mgf * float64(mgDiffs[3])
		g[off+7] += scale * egf * float64(egDiffs[3])
		g[off+8] += scale * mgf * float64(mgDiffs[4])
		g[off+9] += scale * egf * float64(egDiffs[4])
		g[off+10] += scale * mgf * float64(mgDiffs[5])
		g[off+11] += scale * egf * float64(egDiffs[5])
		g[off+12] += scale * mgf * float64(mgDiffs[6])
		g[off+13] += scale * egf * float64(egDiffs[6])
		g[off+14] += scale * mgf * float64(mgDiffs[7])
		g[off+15] += scale * egf * float64(egDiffs[7])
	}

	off += 16

	// Phase 3 gradients (mobility only, 14 slots total)
	var mobMG, mobEG [7]int
	if le.cache.pos == (*gm.Board)(pos) {
		mobMG, mobEG = le.cache.mobMG, le.cache.mobEG
	} else {
		mobMG, mobEG, _, _ = eng.MobAtkDiffs((*gm.Board)(pos))
	}
	mobMGBase := off
	if le.Toggles.MobilityTrain {
		for pt := 0; pt < 7; pt++ {
			g[mobMGBase+pt] += scale * mgf * float64(mobMG[pt])
		}
	}
	off += 7
	mobEGBase := off
	if le.Toggles.MobilityTrain {
		for pt := 0; pt < 7; pt++ {
			g[mobEGBase+pt] += scale * egf * float64(mobEG[pt])
		}
	}
	off += 7

	// Phase 4: KingSafetyTable (MG + EG with /4 factor)
	var ks [100]int
	if le.cache.pos == (*gm.Board)(pos) {
		ks = le.cache.ksOneHot
	} else {
		ks = eng.KingSafetyOneHot((*gm.Board)(pos))
	}
	if le.Toggles.KingTableTrain {
		for i := 0; i < 100; i++ {
			g[off+i] += scale * (mgf*float64(ks[i]) + egf*(float64(ks[i])/4.0))
		}
	}
	off += 100
	// Phase 4 correlates (MG-only)
	var semiOpenDiff, openDiff, minorDefDiff, pawnDefDiff int
	if le.cache.pos == (*gm.Board)(pos) {
		semiOpenDiff, openDiff, minorDefDiff, pawnDefDiff = le.cache.ksSemiOpen, le.cache.ksOpen, le.cache.ksMinorDef, le.cache.ksPawnDef
	} else {
		semiOpenDiff, openDiff, minorDefDiff, pawnDefDiff = eng.KingSafetyCorrelates((*gm.Board)(pos))
	}
	if le.Toggles.KingCorrTrain {
		g[off+0] += scale * mgf * float64(semiOpenDiff)
		g[off+1] += scale * mgf * float64(openDiff)
		g[off+2] += scale * mgf * float64(minorDefDiff)
		g[off+3] += scale * mgf * float64(pawnDefDiff)
	}

	// Phase 5: Extras
	off += 4
	var exMG [7]int
	var exEG [2]int
	if le.cache.pos == (*gm.Board)(pos) {
		exMG, exEG = le.cache.exMG, le.cache.exEG
	} else {
		exMG, exEG = eng.ExtrasDiffs((*gm.Board)(pos))
	}
	if le.Toggles.ExtrasTrain {
		g[off+0] += scale * mgf * float64(exMG[0])
		g[off+1] += scale * egf * float64(exEG[0])
		g[off+2] += scale * mgf * float64(exMG[1])
		g[off+3] += scale * egf * float64(exEG[1])
		g[off+4] += scale * mgf * float64(exMG[2])
		g[off+5] += scale * mgf * float64(exMG[3]) // RookXrayQueenMG
		g[off+6] += scale * mgf * float64(exMG[4]) // ConnectedRooksMG
		g[off+7] += scale * mgf * float64(exMG[6]) // BishopOutpostMG
		// Additional extras from engine bridges
		bxK, bxR, bxQ := eng.BishopXrayCounts((*gm.Board)(pos))
		stDiff, prDiff, lvDiff := eng.PawnStormProxLeverDiffs((*gm.Board)(pos))
		g[off+8] += scale * mgf * float64(bxK)
		g[off+9] += scale * mgf * float64(bxR)
		g[off+10] += scale * mgf * float64(bxQ)
		g[off+11] += scale * mgf * float64(stDiff)
		g[off+12] += scale * mgf * float64(prDiff)
		g[off+13] += scale * mgf * float64(lvDiff)
		var mMG [7]int
		if le.cache.pos == (*gm.Board)(pos) {
			mMG = le.cache.mobMG
		} else {
			mMG, _, _, _ = eng.MobAtkDiffs((*gm.Board)(pos))
		}
		knDelta, biDelta := eng.CenterMobilityScales((*gm.Board)(pos))
		g[off+14] += scale * mgf * float64(mMG[N]*knDelta)
		g[off+15] += scale * mgf * float64(mMG[B]*biDelta)
	}

	off += 16

	// Phase 6: Material imbalance
	var imbMG [6]int
	var imbEG [6]int
	if le.cache.pos == (*gm.Board)(pos) {
		imbMG, imbEG = le.cache.imbMG, le.cache.imbEG
	} else {
		imbMG, imbEG = eng.ImbalanceDiffs((*gm.Board)(pos))
	}
	if le.Toggles.ImbalanceTrain {
		g[off+0] += scale * mgf * float64(imbMG[0])
		g[off+1] += scale * egf * float64(imbEG[0])
		g[off+2] += scale * mgf * float64(imbMG[1])
		g[off+3] += scale * egf * float64(imbEG[1])
		g[off+4] += scale * mgf * float64(imbMG[2])
		g[off+5] += scale * egf * float64(imbEG[2])
		g[off+6] += scale * mgf * float64(imbMG[3])
		g[off+7] += scale * egf * float64(imbEG[3])
		g[off+8] += scale * mgf * float64(imbMG[4])
		g[off+9] += scale * egf * float64(imbEG[4])
		g[off+10] += scale * mgf * float64(imbMG[5])
		g[off+11] += scale * egf * float64(imbEG[5])
	}
	off += 12

	// Phase 7: Weak squares + Tempo
	if le.Toggles.WeakTempoTrain {
		ws, wks := eng.WeakSquaresCounts((*gm.Board)(pos))
		g[off+0] += scale * mgf * float64(ws)
		g[off+1] += scale * mgf * float64(wks)
		if pos.Wtomove {
			g[off+2] += scale * (mgf + egf)
		} else {
			g[off+2] -= scale * (mgf + egf)
		}
	}
}

// ---- Helpers ----

// boardTaperedPhases computes MG/EG phase factors based on current material.
func boardTaperedPhases(b *gm.Board) (mgf, egf float64) {
	var knights, bishops, rooks, queens int
	knights = bits.OnesCount64(b.White.Knights) + bits.OnesCount64(b.Black.Knights)
	bishops = bits.OnesCount64(b.White.Bishops) + bits.OnesCount64(b.Black.Bishops)
	rooks = bits.OnesCount64(b.White.Rooks) + bits.OnesCount64(b.Black.Rooks)
	queens = bits.OnesCount64(b.White.Queens) + bits.OnesCount64(b.Black.Queens)
	phase := 0
	phase += knights * KnightPhase
	phase += bishops * BishopPhase
	phase += rooks * RookPhase
	phase += queens * QueenPhase
	curr := TotalPhase - phase
	mgf = 1.0 - float64(curr)/24.0
	if mgf < 0 {
		mgf = 0
	}
	if mgf > 1 {
		mgf = 1
	}
	egf = float64(curr) / 24.0
	if egf < 0 {
		egf = 0
	}
	if egf > 1 {
		egf = 1
	}
	return
}

func sumTableForBB(tbl [64]float64, bb uint64) float64 {
	s := 0.0
	for x := bb; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		s += tbl[sq]
	}
	return s
}

func sumTableForBBMirrored(tbl [64]float64, bb uint64) float64 {
	s := 0.0
	for x := bb; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		rev := flipView[sq]
		s += tbl[rev]
	}
	return s
}

// evalPSTBoard computes MG/EG components directly from the engine board.
func evalPSTBoard(pst *PST, b *gm.Board) (mg, eg float64) {
	mg += sumTableForBB(pst.MG[P], b.White.Pawns)
	eg += sumTableForBB(pst.EG[P], b.White.Pawns)
	mg += sumTableForBB(pst.MG[N], b.White.Knights)
	eg += sumTableForBB(pst.EG[N], b.White.Knights)
	mg += sumTableForBB(pst.MG[B], b.White.Bishops)
	eg += sumTableForBB(pst.EG[B], b.White.Bishops)
	mg += sumTableForBB(pst.MG[R], b.White.Rooks)
	eg += sumTableForBB(pst.EG[R], b.White.Rooks)
	mg += sumTableForBB(pst.MG[Q], b.White.Queens)
	eg += sumTableForBB(pst.EG[Q], b.White.Queens)
	mg += sumTableForBB(pst.MG[K], b.White.Kings)
	eg += sumTableForBB(pst.EG[K], b.White.Kings)

	mg -= sumTableForBBMirrored(pst.MG[P], b.Black.Pawns)
	eg -= sumTableForBBMirrored(pst.EG[P], b.Black.Pawns)
	mg -= sumTableForBBMirrored(pst.MG[N], b.Black.Knights)
	eg -= sumTableForBBMirrored(pst.EG[N], b.Black.Knights)
	mg -= sumTableForBBMirrored(pst.MG[B], b.Black.Bishops)
	eg -= sumTableForBBMirrored(pst.EG[B], b.Black.Bishops)
	mg -= sumTableForBBMirrored(pst.MG[R], b.Black.Rooks)
	eg -= sumTableForBBMirrored(pst.EG[R], b.Black.Rooks)
	mg -= sumTableForBBMirrored(pst.MG[Q], b.Black.Queens)
	eg -= sumTableForBBMirrored(pst.EG[Q], b.Black.Queens)
	mg -= sumTableForBBMirrored(pst.MG[K], b.Black.Kings)
	eg -= sumTableForBBMirrored(pst.EG[K], b.Black.Kings)
	return mg, eg
}

// passedPawns returns bitboards of white and black passed pawns.
func passedPawns(b *Position) (uint64, uint64) {
	// Helpers to build ahead masks per side
	fileMask := func(f int) uint64 { return uint64(0x0101010101010101) << uint(f) }
	// White passed: no black pawns on same/adjacent files ahead
	wPassed := uint64(0)
	for x := b.White.Pawns; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		f := sq & 7
		r := sq / 8
		var mask uint64
		for rr := r + 1; rr <= 7; rr++ {
			if f > 0 {
				mask |= (fileMask(f-1) & (uint64(0xFF) << (rr * 8)))
			}
			mask |= (fileMask(f) & (uint64(0xFF) << (rr * 8)))
			if f < 7 {
				mask |= (fileMask(f+1) & (uint64(0xFF) << (rr * 8)))
			}
		}
		if (b.Black.Pawns & mask) == 0 {
			wPassed |= (1 << uint(sq))
		}
	}
	// Black passed: no white pawns on same/adjacent files behind (towards rank 0)
	bPassed := uint64(0)
	for x := b.Black.Pawns; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		f := sq & 7
		r := sq / 8
		var mask uint64
		for rr := 0; rr < r; rr++ {
			if f > 0 {
				mask |= (fileMask(f-1) & (uint64(0xFF) << (rr * 8)))
			}
			mask |= (fileMask(f) & (uint64(0xFF) << (rr * 8)))
			if f < 7 {
				mask |= (fileMask(f+1) & (uint64(0xFF) << (rr * 8)))
			}
		}
		if (b.White.Pawns & mask) == 0 {
			bPassed |= (1 << uint(sq))
		}
	}
	return wPassed, bPassed
}
