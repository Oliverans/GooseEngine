// tuner/linear_eval.go
package tuner

import (
	"math/bits"

	eng "chess-engine/engine"
	gm "chess-engine/goosemg"
)

type mobilityCounts struct {
	Knight [9]int
	Bishop [14]int
	Rook   [15]int
	Queen  [22]int
}

type mobilityValues struct {
	KnightMG float64
	KnightEG float64
	BishopMG float64
	BishopEG float64
	RookMG   float64
	RookEG   float64
	QueenMG  float64
	QueenEG  float64
}

// LinearEval implements a linear tunable evaluation with MG/EG tapering.
// Parameters are organized into 4 tiers for toggle control:
//   - Tier 1: Core (PST, Material, Mobility, Outposts, RookFiles, StackedRooks, MobCenter)
//   - Tier 2: Pawns (Passers, PawnStruct)
//   - Tier 3: King Safety (Table, Corr, Endgame, Tropism, PawnStorm, WeakKingSquares)
//   - Tier 4: Misc (BishopPair, Imbalance, Space, Tempo)
type LinearEval struct {
	PST *PST

	// Tier 1: Material weights (centipawns per piece), MG/EG. Index by P..K.
	MatMG [6]float64
	MatEG [6]float64

	// Tier 2: Passed pawn square weights (white perspective), MG/EG. Index by square 0..63.
	PasserMG [64]float64
	PasserEG [64]float64

	// Tier 1/4 scalar terms (BishopPair → Tier 4, rest → Tier 1)
	BishopPairMG       float64 // Tier 4
	BishopPairEG       float64 // Tier 4
	RookSemiOpenFileMG float64 // Tier 1
	RookOpenFileMG     float64 // Tier 1
	SeventhRankEG      float64 // Tier 1
	QueenCentralizedEG float64 // Tier 1

	// Tier 2: Pawn structure scalars (MG/EG)
	DoubledMG            float64
	DoubledEG            float64
	IsolatedMG           float64
	IsolatedEG           float64
	ConnectedMG          float64
	ConnectedEG          float64
	PhalanxMG            float64
	PhalanxEG            float64
	BlockedMG            float64
	BlockedEG            float64
	WeakLeverMG          float64
	WeakLeverEG          float64
	BackwardMG           float64
	BackwardEG           float64
	CandidatePassedPctMG float64
	CandidatePassedPctEG float64

	// Tier 1: Mobility tables (per piece, MG/EG)
	KnightMobilityMG [9]float64
	KnightMobilityEG [9]float64
	BishopMobilityMG [14]float64
	BishopMobilityEG [14]float64
	RookMobilityMG   [15]float64
	RookMobilityEG   [15]float64
	QueenMobilityMG  [22]float64
	QueenMobilityEG  [22]float64

	// Flat parameter buffer
	theta []float64

	// Tier 3: King safety table (100 entries)
	KingSafety [100]float64
	// Tier 3: King correlates (MG-only)
	KingSemiOpenFilePenalty float64
	KingOpenFilePenalty     float64
	KingMinorPieceDefense   float64
	KingPawnDefenseMG       float64
	KingEndgameCenterEG     float64
	KingMopUpEG             float64

	// Tier 1/3 Extras (Outposts/StackedRooks/MobCenter → Tier 1, Tropism/Storm → Tier 3)
	KnightOutpostMG float64 // Tier 1
	KnightOutpostEG float64 // Tier 1
	BishopOutpostMG float64 // Tier 1
	BishopOutpostEG float64 // Tier 1
	BadBishopMG     float64 // Tier 1
	BadBishopEG     float64 // Tier 1
	StackedRooksMG  float64 // Tier 1
	KnightTropismMG float64 // Tier 3
	KnightTropismEG float64 // Tier 3
	// Tier 3: Pawn storm + proximity
	PawnStormBaseMG       [8]float64
	PawnStormFreePct      [8]float64
	PawnStormLeverPct     [8]float64
	PawnStormWeakLeverPct [8]float64
	PawnStormBlockedPct   [8]float64
	PawnStormOppositeMult float64
	PawnProximityMG       float64
	// Tier 1: Mobility center scaling
	KnightMobCenterMG float64
	BishopMobCenterMG float64

	// Tier 4: Material imbalance scalars
	ImbalanceKnightPerPawnMG float64
	ImbalanceKnightPerPawnEG float64
	ImbalanceBishopPerPawnMG float64
	ImbalanceBishopPerPawnEG float64

	// Per-position scratch cache to avoid recomputing expensive engine wrappers
	cache struct {
		pos                                       *gm.Board
		pawnMG                                    [8]int
		pawnEG                                    [8]int
		mobCounts                                 mobilityCounts
		mobValues                                 mobilityValues
		ksOneHot                                  [100]int
		ksSemiOpen, ksOpen, ksMinorDef, ksPawnDef int
		exMG                                      [3]int
		exEG                                      [2]int
		imbMG                                     [2]int
		imbEG                                     [2]int
	}

	// Toggles and consolidated θ layout
	Toggles PhaseToggles

	// Debug flag to enable detailed evaluation output
	Debug  bool
	layout Layout

	// Tier 3/4: Space/Tempo → Tier 4, WeakKingSquares → Tier 3
	SpaceMG           float64 // Tier 4
	SpaceEG           float64 // Tier 4
	WeakKingSquaresMG float64 // Tier 3
	Tempo             float64 // Tier 4
}

// Eval computes a white-positive evaluation using linear terms with MG/EG tapering.
func (le *LinearEval) Eval(pos *Position) float64 {
	if le == nil || le.PST == nil || pos == nil {
		return 0
	}
	le.ensureToggles()
	mgf, egf := boardTaperedPhases(pos)
	var mg, eg float64

	// ===== TIER 1: Core (PST, Material, Mobility, Piece Activity) =====
	if le.Toggles.Tier1Eval {
		// PST
		mtg, etg := evalPSTBoard(le.PST, pos)
		mg += mtg
		eg += etg
		if le.Debug {
			println("################### PST EVALUATION ###################")
			println("PST: MG=", mtg, " EG=", etg)
		}

		// Material (white minus black counts)
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
		if le.Debug {
			println("################### MATERIAL EVALUATION ###################")
			println("White counts: P=", wCnt[P], " N=", wCnt[N], " B=", wCnt[B],
				" R=", wCnt[R], " Q=", wCnt[Q])
			println("Black counts: P=", bCnt[P], " N=", bCnt[N], " B=", bCnt[B],
				" R=", bCnt[R], " Q=", bCnt[Q])
			matMG, matEG := 0.0, 0.0
			for pt := 0; pt < 6; pt++ {
				diff := float64(wCnt[pt] - bCnt[pt])
				matMG += diff * le.MatMG[pt]
				matEG += diff * le.MatEG[pt]
			}
			println("Material: MG=", matMG, " EG=", matEG)
		}

		// Mobility
		mobCounts, mobValues := mobilityTableCountsAndValues(pos, le)
		le.cache.mobCounts = mobCounts
		le.cache.mobValues = mobValues

		mobMG := mobValues.KnightMG + mobValues.BishopMG + mobValues.RookMG + mobValues.QueenMG
		mobEG := mobValues.KnightEG + mobValues.BishopEG + mobValues.RookEG + mobValues.QueenEG

		// Center-based MG mobility scaling for N/B
		knDelta, biDelta := eng.CenterMobilityScales((*gm.Board)(pos))
		if knDelta != 0 {
			mobMG += mobValues.KnightMG * float64(knDelta) * le.KnightMobCenterMG
		}
		if biDelta != 0 {
			mobMG += mobValues.BishopMG * float64(biDelta) * le.BishopMobCenterMG
		}

		mg += mobMG
		eg += mobEG
		if le.Debug {
			println("################### MOBILITY EVALUATION ###################")
			println("Knight MG/EG:", mobValues.KnightMG, mobValues.KnightEG)
			println("Bishop MG/EG:", mobValues.BishopMG, mobValues.BishopEG)
			println("Rook MG/EG:", mobValues.RookMG, mobValues.RookEG)
			println("Queen MG/EG:", mobValues.QueenMG, mobValues.QueenEG)
		}

		// Outposts (Knight and Bishop) + StackedRooks
		exMG, exEG := eng.ExtrasDiffs((*gm.Board)(pos))
		le.cache.exMG = exMG
		le.cache.exEG = exEG
		mg += float64(exMG[0]) * le.KnightOutpostMG
		eg += float64(exEG[0]) * le.KnightOutpostEG
		mg += float64(exMG[2]) * le.BishopOutpostMG
		eg += float64(exEG[1]) * le.BishopOutpostEG
		mg += float64(exMG[1]) * le.StackedRooksMG
		// Bad bishops (fixed-pawn count scaled by tunable weights)
		badDiff := eng.BadBishopUnitDiff((*gm.Board)(pos))
		mg += float64(badDiff) * le.BadBishopMG
		eg += float64(badDiff) * le.BadBishopEG

		// Rook file bonuses (semi-open/open) in MG
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
		seventhUnits := w7 - b7
		if w7 >= 2 {
			seventhUnits += 2
		}
		if b7 >= 2 {
			seventhUnits -= 2
		}
		eg += float64(seventhUnits) * le.SeventhRankEG

		// Queen centralization (EG)
		const centerMask uint64 = 0x183c3c180000
		wc := 0.0
		if (pos.White.Queens & centerMask) != 0 {
			wc = 1.0
		}
		bc := 0.0
		if (pos.Black.Queens & centerMask) != 0 {
			bc = 1.0
		}
		eg += (wc - bc) * le.QueenCentralizedEG
	}

	// ===== TIER 2: Pawn Structure (Passers + PawnStruct) =====
	if le.Toggles.Tier2Eval {
		// Passed pawns (square-based)
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
		// Candidate passed pawns (lever/capture potential)
		pawnEntry := eng.GetPawnEntry((*gm.Board)(pos), false)
		candMG, candEG := candidatePasserBonus(pos, pawnEntry, wPassed, bPassed, le.PasserMG, le.PasserEG, le.CandidatePassedPctMG, le.CandidatePassedPctEG)
		mg += candMG
		eg += candEG

		// Pawn structure scalars (via engine wrappers)
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
		mg += float64(mgDiffs[6]) * le.WeakLeverMG
		eg += float64(egDiffs[6]) * le.WeakLeverEG
		mg += float64(mgDiffs[7]) * le.BackwardMG
		eg += float64(egDiffs[7]) * le.BackwardEG
		// cache pawn diffs
		le.cache.pos = (*gm.Board)(pos)
		le.cache.pawnMG = mgDiffs
		le.cache.pawnEG = egDiffs
		if le.Debug {
			println("################### PAWN STRUCTURE EVALUATION ###################")
			println("Feature differences (white - black):")
			println("  Doubled:   MG=", mgDiffs[0], " EG=", egDiffs[0])
			println("  Isolated:  MG=", mgDiffs[1], " EG=", egDiffs[1])
			println("  Connected: MG=", mgDiffs[2], " EG=", egDiffs[2])
			println("  Phalanx:   MG=", mgDiffs[3], " EG=", egDiffs[3])
			println("  Blocked:   MG=", mgDiffs[4], " EG=", egDiffs[4])
			println("  WeakLever: MG=", mgDiffs[6], " EG=", egDiffs[6])
			println("  Backward:  MG=", mgDiffs[7], " EG=", egDiffs[7])
			println("  Candidate: MG=", candMG, " EG=", candEG)
		}
	}

	// ===== TIER 3: King Safety (Table, Corr, Endgame, Tropism, PawnStorm, WeakKingSquares) =====
	if le.Toggles.Tier3Eval {
		// King safety table
		ks := eng.KingSafetyOneHot((*gm.Board)(pos))
		le.cache.ksOneHot = ks
		for i := 0; i < 100; i++ {
			mg += float64(ks[i]) * le.KingSafety[i]
			eg += float64(ks[i]) * (le.KingSafety[i] / 4.0)
		}

		// King safety correlates
		semiOpenDiff, openDiff, minorDefDiff, pawnDefDiff := eng.KingSafetyCorrelates((*gm.Board)(pos))
		le.cache.ksSemiOpen, le.cache.ksOpen, le.cache.ksMinorDef, le.cache.ksPawnDef = semiOpenDiff, openDiff, minorDefDiff, pawnDefDiff
		mg += float64(semiOpenDiff) * le.KingSemiOpenFilePenalty
		mg += float64(openDiff) * le.KingOpenFilePenalty
		mg += float64(minorDefDiff) * le.KingMinorPieceDefense
		mg += float64(pawnDefDiff) * le.KingPawnDefenseMG

		// King endgame terms
		cmdDiff, mopDiff := eng.EndgameKingTerms((*gm.Board)(pos))
		eg += float64(cmdDiff) * le.KingEndgameCenterEG
		eg += float64(mopDiff) * le.KingMopUpEG
		eg += float64(eng.KingPasserProximityTerm((*gm.Board)(pos)))

		// Knight tropism
		tMG, tEG := eng.KnightTropismDiffs((*gm.Board)(pos))
		mg += float64(tMG) * le.KnightTropismMG
		eg += float64(tEG) * le.KnightTropismEG

		// Pawn storm
		freeDiff, leverDiff, weakLeverDiff, blockedDiff, oppositeSide :=
			eng.PawnStormCategoryDiffs((*gm.Board)(pos))
		baseMG := le.PawnStormBaseMG
		stormSum := 0.0
		for rank := 0; rank < 8; rank++ {
			if baseMG[rank] == 0 {
				continue
			}
			base := baseMG[rank]
			stormSum += float64(freeDiff[rank]) * (base * le.PawnStormFreePct[rank] / 100.0)
			stormSum += float64(leverDiff[rank]) * (base * le.PawnStormLeverPct[rank] / 100.0)
			stormSum += float64(weakLeverDiff[rank]) * (base * le.PawnStormWeakLeverPct[rank] / 100.0)
			stormSum += float64(blockedDiff[rank]) * (base * le.PawnStormBlockedPct[rank] / 100.0)
		}
		if oppositeSide {
			stormSum *= le.PawnStormOppositeMult / 100.0
		}
		mg += stormSum

		// Weak king squares
		_, weakKingDiff := eng.SpaceAndWeakKingDiffs((*gm.Board)(pos))
		mg += float64(weakKingDiff) * le.WeakKingSquaresMG
	}

	// ===== TIER 4: Misc (BishopPair, Imbalance, Space, Tempo) =====
	if le.Toggles.Tier4Eval {
		// Bishop pair (engine-logic + MG center scaling)
		bpMG, bpEG := eng.BishopPairDiffsScaled((*gm.Board)(pos))
		mg += float64(bpMG) * le.BishopPairMG / 100.0
		eg += float64(bpEG) * le.BishopPairEG

		// Material imbalance scalars
		imbMG, imbEG := eng.ImbalanceDiffs((*gm.Board)(pos))
		le.cache.imbMG = imbMG
		le.cache.imbEG = imbEG
		mg += float64(imbMG[0]) * le.ImbalanceKnightPerPawnMG
		eg += float64(imbEG[0]) * le.ImbalanceKnightPerPawnEG
		mg += float64(imbMG[1]) * le.ImbalanceBishopPerPawnMG
		eg += float64(imbEG[1]) * le.ImbalanceBishopPerPawnEG

		// Space
		spaceDiff, _ := eng.SpaceAndWeakKingDiffs((*gm.Board)(pos))
		mg += float64(spaceDiff) * le.SpaceMG
		eg += float64(spaceDiff) * le.SpaceEG

		// Tempo
		if pos.Wtomove {
			mg += le.Tempo
			eg += le.Tempo
		} else {
			mg -= le.Tempo
			eg -= le.Tempo
		}
	}

	if le.Debug {
		println("################### FINAL SCORE ###################")
		println("MG score: ", mg)
		println("EG score: ", eg)
		println("MG phase: ", mgf)
		println("EG phase: ", egf)
		finalScore := mg*mgf + eg*egf
		println("Tapered score: ", finalScore)
		println("!!!--- NOTE: Score is shown from white's perspective ---!!!")
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
	le.ensureLayout()
	if len(g) < le.layout.Total {
		return
	}

	mgf, egf := boardTaperedPhases(pos)

	add := func(pt int, sq int, sgn float64) {
		off := pt*slotsPerPiece + sq
		g[mgBase+off] += sgn * scale * mgf
		g[egBase+off] += sgn * scale * egf
	}

	// ===== TIER 1: Core (PST, Material, Mobility, Piece Activity) =====
	// PST gradients
	if le.Toggles.Tier1Train {
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

	// Material gradients (still Tier1)
	matMGBase := egBase + pieceCount*slotsPerPiece
	matEGBase := matMGBase + 6
	if le.Toggles.Tier1Train {
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

	// ===== TIER 2: Pawn Structure (Passers + PawnStruct) =====
	// Passed pawns gradients (square-based)
	passMGBase := matEGBase + 6
	passEGBase := passMGBase + 64
	if le.Toggles.Tier2Train {
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
		pawnEntry := eng.GetPawnEntry((*gm.Board)(pos), false)
		candMGIdx := le.layout.PawnStructStart + 14
		candEGIdx := le.layout.PawnStructStart + 15
		candidatePasserGrad(pos, pawnEntry, wPassed, bPassed, le.PasserMG, le.PasserEG, passMGBase, passEGBase, candMGIdx, candEGIdx, g, scale, mgf, egf, le.CandidatePassedPctMG, le.CandidatePassedPctEG)
	}

	// P1 scalars: BishopPair (Tier4), RookFiles/SeventhRank/QueenCentralized (Tier1)
	off := passEGBase + 64
	// BishopPair → Tier4
	if le.Toggles.Tier4Train {
		bpMG, bpEG := eng.BishopPairDiffsScaled((*gm.Board)(pos))
		g[off+0] += scale * mgf * (float64(bpMG) / 100.0)
		g[off+1] += scale * egf * float64(bpEG)
	}
	// RookFiles, SeventhRank, QueenCentralized → Tier1
	if le.Toggles.Tier1Train {
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
		seventhUnits := w7 - b7
		if w7 >= 2 {
			seventhUnits += 2
		}
		if b7 >= 2 {
			seventhUnits -= 2
		}
		g[off+4] += scale * egf * float64(seventhUnits)
		// Queen centralization (EG)
		const centerMask uint64 = 0x183c3c180000
		wc := 0.0
		if (pos.White.Queens & centerMask) != 0 {
			wc = 1.0
		}
		bc := 0.0
		if (pos.Black.Queens & centerMask) != 0 {
			bc = 1.0
		}
		g[off+5] += scale * egf * (wc - bc)
	}

	// Pawn structure gradients (Tier2)
	off += 6
	var mgDiffs [8]int
	var egDiffs [8]int
	if le.cache.pos == (*gm.Board)(pos) {
		mgDiffs, egDiffs = le.cache.pawnMG, le.cache.pawnEG
	} else {
		mgDiffs, egDiffs = eng.PawnStructDiffs((*gm.Board)(pos))
	}
	if le.Toggles.Tier2Train {
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
		g[off+10] += scale * mgf * float64(mgDiffs[6])
		g[off+11] += scale * egf * float64(egDiffs[6])
		g[off+12] += scale * mgf * float64(mgDiffs[7])
		g[off+13] += scale * egf * float64(egDiffs[7])
	}

	off += 16

	// Mobility gradients (Tier1)
	var mobCounts mobilityCounts
	var mobValues mobilityValues
	if le.cache.pos == (*gm.Board)(pos) {
		mobCounts = le.cache.mobCounts
		mobValues = le.cache.mobValues
	} else {
		mobCounts, mobValues = mobilityTableCountsAndValues(pos, le)
	}
	knDelta, biDelta := eng.CenterMobilityScales((*gm.Board)(pos))
	knScale := 1.0 + le.KnightMobCenterMG*float64(knDelta)
	biScale := 1.0 + le.BishopMobCenterMG*float64(biDelta)

	mobMGBase := off
	if le.Toggles.Tier1Train {
		idx := mobMGBase
		for i := 0; i < len(mobCounts.Knight); i++ {
			g[idx+i] += scale * mgf * float64(mobCounts.Knight[i]) * knScale
		}
		idx += len(mobCounts.Knight)
		for i := 0; i < len(mobCounts.Bishop); i++ {
			g[idx+i] += scale * mgf * float64(mobCounts.Bishop[i]) * biScale
		}
		idx += len(mobCounts.Bishop)
		for i := 0; i < len(mobCounts.Rook); i++ {
			g[idx+i] += scale * mgf * float64(mobCounts.Rook[i])
		}
		idx += len(mobCounts.Rook)
		for i := 0; i < len(mobCounts.Queen); i++ {
			g[idx+i] += scale * mgf * float64(mobCounts.Queen[i])
		}
	}
	off += len(mobCounts.Knight) + len(mobCounts.Bishop) + len(mobCounts.Rook) + len(mobCounts.Queen)

	mobEGBase := off
	if le.Toggles.Tier1Train {
		idx := mobEGBase
		for i := 0; i < len(mobCounts.Knight); i++ {
			g[idx+i] += scale * egf * float64(mobCounts.Knight[i])
		}
		idx += len(mobCounts.Knight)
		for i := 0; i < len(mobCounts.Bishop); i++ {
			g[idx+i] += scale * egf * float64(mobCounts.Bishop[i])
		}
		idx += len(mobCounts.Bishop)
		for i := 0; i < len(mobCounts.Rook); i++ {
			g[idx+i] += scale * egf * float64(mobCounts.Rook[i])
		}
		idx += len(mobCounts.Rook)
		for i := 0; i < len(mobCounts.Queen); i++ {
			g[idx+i] += scale * egf * float64(mobCounts.Queen[i])
		}
	}
	off += len(mobCounts.Knight) + len(mobCounts.Bishop) + len(mobCounts.Rook) + len(mobCounts.Queen)

	// ===== TIER 3: King Safety (Table, Corr, Endgame, Tropism, PawnStorm, WeakKingSquares) =====
	// King safety table (MG + EG with /4 factor)
	var ks [100]int
	if le.cache.pos == (*gm.Board)(pos) {
		ks = le.cache.ksOneHot
	} else {
		ks = eng.KingSafetyOneHot((*gm.Board)(pos))
	}
	if le.Toggles.Tier3Train {
		for i := 0; i < 100; i++ {
			g[off+i] += scale * (mgf*float64(ks[i]) + egf*(float64(ks[i])/4.0))
		}
	}
	off += 100
	// King safety correlates (MG-only)
	var semiOpenDiff, openDiff, minorDefDiff, pawnDefDiff int
	if le.cache.pos == (*gm.Board)(pos) {
		semiOpenDiff, openDiff, minorDefDiff, pawnDefDiff = le.cache.ksSemiOpen, le.cache.ksOpen, le.cache.ksMinorDef, le.cache.ksPawnDef
	} else {
		semiOpenDiff, openDiff, minorDefDiff, pawnDefDiff = eng.KingSafetyCorrelates((*gm.Board)(pos))
	}
	if le.Toggles.Tier3Train {
		g[off+0] += scale * mgf * float64(semiOpenDiff)
		g[off+1] += scale * mgf * float64(openDiff)
		g[off+2] += scale * mgf * float64(minorDefDiff)
		g[off+3] += scale * mgf * float64(pawnDefDiff)
	}

	// Endgame king terms (EG-only, still Tier3)
	off += 4
	if le.Toggles.Tier3Train {
		cmdDiff, mopDiff := eng.EndgameKingTerms((*gm.Board)(pos))
		g[off+0] += scale * egf * float64(cmdDiff)
		g[off+1] += scale * egf * float64(mopDiff)
	}
	off += 2

	// Extras block: Split across tiers
	// Tier1: Outposts (off+0/1 knight, off+5/6 bishop), StackedRooks (off+4), MobCenter (off+41/42)
	// Tier3: Tropism (off+2/3), PawnStorm (off+7-39, base at off+43-50)
	var exMG [3]int
	var exEG [2]int
	extrasTrain := le.Toggles.Tier1Train || le.Toggles.Tier3Train
	if extrasTrain {
		if le.cache.pos == (*gm.Board)(pos) {
			exMG, exEG = le.cache.exMG, le.cache.exEG
		} else {
			exMG, exEG = eng.ExtrasDiffs((*gm.Board)(pos))
		}
		// Tier1: Outposts + StackedRooks + MobCenter
		if le.Toggles.Tier1Train {
			tog := le.Toggles.ParamTrain
			if tog.ExtraKnightOutpostMG {
				g[off+0] += scale * mgf * float64(exMG[0]) // KnightOutpostMG
			}
			if tog.ExtraKnightOutpostEG {
				g[off+1] += scale * egf * float64(exEG[0]) // KnightOutpostEG
			}
			if tog.ExtraBishopOutpostMG {
				g[off+5] += scale * mgf * float64(exMG[2]) // BishopOutpostMG
			}
			if tog.ExtraBishopOutpostEG {
				g[off+6] += scale * egf * float64(exEG[1]) // BishopOutpostEG
			}
			g[off+4] += scale * mgf * float64(exMG[1]) // StackedRooksMG
			knDelta, biDelta := eng.CenterMobilityScales((*gm.Board)(pos))
			if tog.ExtraKnightMobCenterMG {
				g[off+41] += scale * mgf * float64(knDelta) * mobValues.KnightMG // KnightMobCenterMG
			}
			if tog.ExtraBishopMobCenterMG {
				g[off+42] += scale * mgf * float64(biDelta) * mobValues.BishopMG // BishopMobCenterMG
			}
			badDiff := eng.BadBishopUnitDiff((*gm.Board)(pos))
			g[off+51] += scale * mgf * float64(badDiff) // BadBishopMG
			g[off+52] += scale * egf * float64(badDiff) // BadBishopEG
		}
		// Tier3: Tropism + PawnStorm
		if le.Toggles.Tier3Train {
			tMG, tEG := eng.KnightTropismDiffs((*gm.Board)(pos))
			g[off+2] += scale * mgf * float64(tMG) // KnightTropismMG (layout offset 2)
			g[off+3] += scale * egf * float64(tEG) // KnightTropismEG (layout offset 3)

			// Pawn storm percentages
			freeDiff, leverDiff, weakLeverDiff, blockedDiff, oppositeSide :=
				eng.PawnStormCategoryDiffs((*gm.Board)(pos))

			baseMG := le.PawnStormBaseMG
			opMult := 1.0
			if oppositeSide {
				opMult = le.PawnStormOppositeMult / 100.0
			}

			var stormSum float64

			// Compute gradients for each percentage parameter
			for rank := 0; rank < 8; rank++ {
				base := baseMG[rank]
				baseGrad := float64(freeDiff[rank]) * (le.PawnStormFreePct[rank] / 100.0)
				baseGrad += float64(leverDiff[rank]) * (le.PawnStormLeverPct[rank] / 100.0)
				baseGrad += float64(weakLeverDiff[rank]) * (le.PawnStormWeakLeverPct[rank] / 100.0)
				baseGrad += float64(blockedDiff[rank]) * (le.PawnStormBlockedPct[rank] / 100.0)
				g[off+43+rank] += scale * mgf * baseGrad * opMult
				if base == 0 {
					continue
				}
				basePct := base / 100.0

				// PawnStormFreePct is treated as a fixed baseline (no gradient updates).
				g[off+15+rank] += scale * mgf * float64(leverDiff[rank]) * basePct * opMult
				g[off+23+rank] += scale * mgf * float64(weakLeverDiff[rank]) * basePct * opMult
				g[off+31+rank] += scale * mgf * float64(blockedDiff[rank]) * basePct * opMult

				stormSum += float64(freeDiff[rank]) * (base * le.PawnStormFreePct[rank] / 100.0)
				stormSum += float64(leverDiff[rank]) * (base * le.PawnStormLeverPct[rank] / 100.0)
				stormSum += float64(weakLeverDiff[rank]) * (base * le.PawnStormWeakLeverPct[rank] / 100.0)
				stormSum += float64(blockedDiff[rank]) * (base * le.PawnStormBlockedPct[rank] / 100.0)
			}

			if oppositeSide {
				g[off+39] += scale * mgf * stormSum / 100.0
			}
		}
	}

	off += 53 // Extras block

	// ---- Tier 4: Imbalance ----
	var imbMG [2]int
	var imbEG [2]int
	if le.cache.pos == (*gm.Board)(pos) {
		imbMG, imbEG = le.cache.imbMG, le.cache.imbEG
	} else {
		imbMG, imbEG = eng.ImbalanceDiffs((*gm.Board)(pos))
	}
	if le.Toggles.Tier4Train {
		g[off+0] += scale * mgf * float64(imbMG[0])
		g[off+1] += scale * egf * float64(imbEG[0])
		g[off+2] += scale * mgf * float64(imbMG[1])
		g[off+3] += scale * egf * float64(imbEG[1])
	}
	off += 4

	// ---- Tier 3: WeakKingSquares / Tier 4: Space + Tempo ----
	spaceDiff, weakKingDiff := eng.SpaceAndWeakKingDiffs((*gm.Board)(pos))

	// Tier 4: Space + Tempo
	if le.Toggles.Tier4Train {
		g[off+0] += scale * mgf * float64(spaceDiff)
		g[off+1] += scale * egf * float64(spaceDiff)
		if pos.Wtomove {
			g[off+3] += scale * (mgf + egf)
		} else {
			g[off+3] -= scale * (mgf + egf)
		}
	}

	// Tier 3: WeakKingSquares
	if le.Toggles.Tier3Train {
		g[off+2] += scale * mgf * float64(weakKingDiff)
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

func mobilityIndex(cnt, max int) int {
	if cnt < 0 {
		return 0
	}
	if cnt > max {
		return max
	}
	return cnt
}

func mobilityTableCountsAndValues(pos *Position, le *LinearEval) (counts mobilityCounts, values mobilityValues) {
	if pos == nil || le == nil {
		return counts, values
	}
	wPawnAttackBB_E, wPawnAttackBB_W := eng.PawnCaptureBitboards(pos.White.Pawns, true)
	bPawnAttackBB_E, bPawnAttackBB_W := eng.PawnCaptureBitboards(pos.Black.Pawns, false)
	wPawnAttackBB := wPawnAttackBB_E | wPawnAttackBB_W
	bPawnAttackBB := bPawnAttackBB_E | bPawnAttackBB_W
	all := pos.White.All | pos.Black.All

	for x := pos.White.Knights; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		attacked := eng.KnightMasks[sq]
		mobSquares := attacked &^ bPawnAttackBB &^ pos.White.All
		cnt := bits.OnesCount64(mobSquares)
		idx := mobilityIndex(cnt, len(le.KnightMobilityMG)-1)
		counts.Knight[idx]++
		values.KnightMG += le.KnightMobilityMG[idx]
		values.KnightEG += le.KnightMobilityEG[idx]
	}
	for x := pos.Black.Knights; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		attacked := eng.KnightMasks[sq]
		mobSquares := attacked &^ wPawnAttackBB &^ pos.Black.All
		cnt := bits.OnesCount64(mobSquares)
		idx := mobilityIndex(cnt, len(le.KnightMobilityMG)-1)
		counts.Knight[idx]--
		values.KnightMG -= le.KnightMobilityMG[idx]
		values.KnightEG -= le.KnightMobilityEG[idx]
	}

	for x := pos.White.Bishops; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		occupied := all &^ eng.PositionBB[sq]
		attacked := gm.CalculateBishopMoveBitboard(uint8(sq), occupied)
		mobSquares := attacked &^ bPawnAttackBB &^ pos.White.All
		cnt := bits.OnesCount64(mobSquares)
		idx := mobilityIndex(cnt, len(le.BishopMobilityMG)-1)
		counts.Bishop[idx]++
		values.BishopMG += le.BishopMobilityMG[idx]
		values.BishopEG += le.BishopMobilityEG[idx]
	}
	for x := pos.Black.Bishops; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		occupied := all &^ eng.PositionBB[sq]
		attacked := gm.CalculateBishopMoveBitboard(uint8(sq), occupied)
		mobSquares := attacked &^ wPawnAttackBB &^ pos.Black.All
		cnt := bits.OnesCount64(mobSquares)
		idx := mobilityIndex(cnt, len(le.BishopMobilityMG)-1)
		counts.Bishop[idx]--
		values.BishopMG -= le.BishopMobilityMG[idx]
		values.BishopEG -= le.BishopMobilityEG[idx]
	}

	for x := pos.White.Rooks; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		occupied := all &^ eng.PositionBB[sq]
		attacked := gm.CalculateRookMoveBitboard(uint8(sq), occupied)
		mobSquares := attacked &^ bPawnAttackBB &^ pos.White.All
		cnt := bits.OnesCount64(mobSquares)
		idx := mobilityIndex(cnt, len(le.RookMobilityMG)-1)
		counts.Rook[idx]++
		values.RookMG += le.RookMobilityMG[idx]
		values.RookEG += le.RookMobilityEG[idx]
	}
	for x := pos.Black.Rooks; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		occupied := all &^ eng.PositionBB[sq]
		attacked := gm.CalculateRookMoveBitboard(uint8(sq), occupied)
		mobSquares := attacked &^ wPawnAttackBB &^ pos.Black.All
		cnt := bits.OnesCount64(mobSquares)
		idx := mobilityIndex(cnt, len(le.RookMobilityMG)-1)
		counts.Rook[idx]--
		values.RookMG -= le.RookMobilityMG[idx]
		values.RookEG -= le.RookMobilityEG[idx]
	}

	for x := pos.White.Queens; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		occupied := all &^ eng.PositionBB[sq]
		bishopAttacks := gm.CalculateBishopMoveBitboard(uint8(sq), occupied)
		rookAttacks := gm.CalculateRookMoveBitboard(uint8(sq), occupied)
		attacked := bishopAttacks | rookAttacks
		mobSquares := attacked &^ bPawnAttackBB &^ pos.White.All
		cnt := bits.OnesCount64(mobSquares)
		idx := mobilityIndex(cnt, len(le.QueenMobilityMG)-1)
		counts.Queen[idx]++
		values.QueenMG += le.QueenMobilityMG[idx]
		values.QueenEG += le.QueenMobilityEG[idx]
	}
	for x := pos.Black.Queens; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		occupied := all &^ eng.PositionBB[sq]
		bishopAttacks := gm.CalculateBishopMoveBitboard(uint8(sq), occupied)
		rookAttacks := gm.CalculateRookMoveBitboard(uint8(sq), occupied)
		attacked := bishopAttacks | rookAttacks
		mobSquares := attacked &^ wPawnAttackBB &^ pos.Black.All
		cnt := bits.OnesCount64(mobSquares)
		idx := mobilityIndex(cnt, len(le.QueenMobilityMG)-1)
		counts.Queen[idx]--
		values.QueenMG -= le.QueenMobilityMG[idx]
		values.QueenEG -= le.QueenMobilityEG[idx]
	}

	return counts, values
}

func candidatePasserBonus(pos *Position, entry *eng.PawnHashEntry, wPassed, bPassed uint64, passerMG, passerEG [64]float64, candPctMG, candPctEG float64) (mg, eg float64) {
	if entry == nil {
		return 0, 0
	}
	occ := pos.White.All | pos.Black.All
	pctMG := candPctMG / 100.0
	pctEG := candPctEG / 100.0

	for x := (entry.WLeverBB | entry.WLeverPushedBB) &^ wPassed; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		pawnBB := eng.PositionBB[sq]
		bestMG, bestEG := 0.0, 0.0

		captureOrigins := pawnBB & entry.WLeverBB
		if pawnBB&entry.WLeverPushedBB != 0 && sq < 56 {
			if front := eng.PositionBB[sq+8]; front&occ == 0 {
				captureOrigins |= front
			}
		}

		for originsBB := captureOrigins; originsBB != 0; originsBB &= originsBB - 1 {
			fromSq := bits.TrailingZeros64(originsBB)
			attacksE, attacksW := eng.PawnCaptureBitboards(eng.PositionBB[fromSq], true)

			for targetsBB := (attacksE | attacksW) & pos.Black.Pawns; targetsBB != 0; targetsBB &= targetsBB - 1 {
				capSq := bits.TrailingZeros64(targetsBB)
				if (pos.Black.Pawns&^eng.PositionBB[capSq])&eng.PassedMaskWhite[capSq] == 0 {
					candMG := passerMG[capSq] * pctMG
					if candMG > bestMG {
						bestMG = candMG
					}
					candEG := passerEG[capSq] * pctEG
					if candEG > bestEG {
						bestEG = candEG
					}
				}
			}
		}

		if bestMG != 0 || bestEG != 0 {
			mg += bestMG
			eg += bestEG
		}
	}

	for x := (entry.BLeverBB | entry.BLeverPushedBB) &^ bPassed; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		pawnBB := eng.PositionBB[sq]
		bestMG, bestEG := 0.0, 0.0

		captureOrigins := pawnBB & entry.BLeverBB
		if pawnBB&entry.BLeverPushedBB != 0 && sq >= 8 {
			if front := eng.PositionBB[sq-8]; front&occ == 0 {
				captureOrigins |= front
			}
		}

		for originsBB := captureOrigins; originsBB != 0; originsBB &= originsBB - 1 {
			fromSq := bits.TrailingZeros64(originsBB)
			attacksE, attacksW := eng.PawnCaptureBitboards(eng.PositionBB[fromSq], false)

			for targetsBB := (attacksE | attacksW) & pos.White.Pawns; targetsBB != 0; targetsBB &= targetsBB - 1 {
				capSq := bits.TrailingZeros64(targetsBB)
				if (pos.White.Pawns&^eng.PositionBB[capSq])&eng.PassedMaskBlack[capSq] == 0 {
					rev := flipView[capSq]
					candMG := passerMG[rev] * pctMG
					if candMG > bestMG {
						bestMG = candMG
					}
					candEG := passerEG[rev] * pctEG
					if candEG > bestEG {
						bestEG = candEG
					}
				}
			}
		}

		if bestMG != 0 || bestEG != 0 {
			mg -= bestMG
			eg -= bestEG
		}
	}

	return mg, eg
}

func candidatePasserGrad(pos *Position, entry *eng.PawnHashEntry, wPassed, bPassed uint64, passerMG, passerEG [64]float64, passMGBase, passEGBase, candMGIdx, candEGIdx int, g []float64, scale, mgf, egf, candPctMG, candPctEG float64) {
	if entry == nil {
		return
	}
	occ := pos.White.All | pos.Black.All
	pctMG := candPctMG / 100.0
	pctEG := candPctEG / 100.0

	for x := (entry.WLeverBB | entry.WLeverPushedBB) &^ wPassed; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		pawnBB := eng.PositionBB[sq]
		bestMGVal, bestEGVal := 0.0, 0.0
		bestMGUnit, bestEGUnit := 0.0, 0.0
		bestMGsq, bestEGsq := -1, -1

		captureOrigins := pawnBB & entry.WLeverBB
		if pawnBB&entry.WLeverPushedBB != 0 && sq < 56 {
			if front := eng.PositionBB[sq+8]; front&occ == 0 {
				captureOrigins |= front
			}
		}

		for originsBB := captureOrigins; originsBB != 0; originsBB &= originsBB - 1 {
			fromSq := bits.TrailingZeros64(originsBB)
			attacksE, attacksW := eng.PawnCaptureBitboards(eng.PositionBB[fromSq], true)

			for targetsBB := (attacksE | attacksW) & pos.Black.Pawns; targetsBB != 0; targetsBB &= targetsBB - 1 {
				capSq := bits.TrailingZeros64(targetsBB)
				if (pos.Black.Pawns&^eng.PositionBB[capSq])&eng.PassedMaskWhite[capSq] == 0 {
					candMG := passerMG[capSq] * pctMG
					if candMG > bestMGVal {
						bestMGVal = candMG
						bestMGsq = capSq
						bestMGUnit = passerMG[capSq]
					}
					candEG := passerEG[capSq] * pctEG
					if candEG > bestEGVal {
						bestEGVal = candEG
						bestEGsq = capSq
						bestEGUnit = passerEG[capSq]
					}
				}
			}
		}

		if bestMGsq >= 0 {
			g[passMGBase+bestMGsq] += scale * mgf * pctMG
			g[candMGIdx] += scale * mgf * (bestMGUnit / 100.0)
		}
		if bestEGsq >= 0 {
			g[passEGBase+bestEGsq] += scale * egf * pctEG
			g[candEGIdx] += scale * egf * (bestEGUnit / 100.0)
		}
	}

	for x := (entry.BLeverBB | entry.BLeverPushedBB) &^ bPassed; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		pawnBB := eng.PositionBB[sq]
		bestMGVal, bestEGVal := 0.0, 0.0
		bestMGUnit, bestEGUnit := 0.0, 0.0
		bestMGsq, bestEGsq := -1, -1

		captureOrigins := pawnBB & entry.BLeverBB
		if pawnBB&entry.BLeverPushedBB != 0 && sq >= 8 {
			if front := eng.PositionBB[sq-8]; front&occ == 0 {
				captureOrigins |= front
			}
		}

		for originsBB := captureOrigins; originsBB != 0; originsBB &= originsBB - 1 {
			fromSq := bits.TrailingZeros64(originsBB)
			attacksE, attacksW := eng.PawnCaptureBitboards(eng.PositionBB[fromSq], false)

			for targetsBB := (attacksE | attacksW) & pos.White.Pawns; targetsBB != 0; targetsBB &= targetsBB - 1 {
				capSq := bits.TrailingZeros64(targetsBB)
				if (pos.White.Pawns&^eng.PositionBB[capSq])&eng.PassedMaskBlack[capSq] == 0 {
					rev := flipView[capSq]
					candMG := passerMG[rev] * pctMG
					if candMG > bestMGVal {
						bestMGVal = candMG
						bestMGsq = rev
						bestMGUnit = passerMG[rev]
					}
					candEG := passerEG[rev] * pctEG
					if candEG > bestEGVal {
						bestEGVal = candEG
						bestEGsq = rev
						bestEGUnit = passerEG[rev]
					}
				}
			}
		}

		if bestMGsq >= 0 {
			g[passMGBase+bestMGsq] -= scale * mgf * pctMG
			g[candMGIdx] -= scale * mgf * (bestMGUnit / 100.0)
		}
		if bestEGsq >= 0 {
			g[passEGBase+bestEGsq] -= scale * egf * pctEG
			g[candEGIdx] -= scale * egf * (bestEGUnit / 100.0)
		}
	}
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

// SetDebug enables or disables debug output for evaluation
func (le *LinearEval) SetDebug(debug bool) {
	if le != nil {
		le.Debug = debug
	}
}
