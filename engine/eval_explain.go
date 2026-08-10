package engine

import (
	"math/bits"

	gm "chess-engine/goosemg"
)

// EvalBreakdown provides a coarse breakdown of evaluation terms (white minus black).
// These values are intended for relative deltas and strategic labeling, not exact accounting.
type EvalBreakdown struct {
	Material           int32
	Imbalance          int32
	PawnStructure      int32
	PassedPawns        int32
	KnightActivity     int32
	BishopActivity     int32
	RookActivity       int32
	QueenActivity      int32
	Activity           int32
	Space              int32
	KingAttackPressure int32
	KingPawnShield     int32
	KingEndgame        int32
	KingSafety         int32
	WPawnAttackBB      uint64
	BPawnAttackBB      uint64
	// Per-side king-attack diagnostics (not flipped). AttackOn*King is the clamped
	// king-danger accumulator bearing on that king; DangerTo*King is the centipawn
	// value it maps to once squared.
	AttackOnWhiteKing int32
	AttackOnBlackKing int32
	DangerToWhiteKing int32
	DangerToBlackKing int32
}

// EvalExplain returns the engine evaluation and a coarse breakdown of strategic terms.
// Breakdown terms are in white-minus-black perspective and are not flipped for side to move.
func EvalExplain(b *gm.Board) (totalCp int32, breakdown EvalBreakdown) {
	if b == nil {
		return 0, breakdown
	}

	initVariables(b)

	totalCp = Evaluation(b, false)

	pawnEntry := GetPawnEntry(b, false)
	wPawnAttackBB := pawnEntry.WPawnAttackBB
	bPawnAttackBB := pawnEntry.BPawnAttackBB

	openFiles := pawnEntry.OpenFiles
	wSemiOpenFiles := pawnEntry.WSemiOpenFiles
	bSemiOpenFiles := pawnEntry.BSemiOpenFiles

	outposts := getOutpostsBB(b, wPawnAttackBB, bPawnAttackBB)
	whiteOutposts := outposts[0]
	blackOutposts := outposts[1]

	wRookStackFiles, bRookStackFiles := getRookConnectedFiles(b)

	lockedCenter, openIdx := getCenterState(b, openFiles, wSemiOpenFiles, bSemiOpenFiles,
		pawnEntry.WLeverBB, pawnEntry.BLeverBB)
	scales := getCenterMobilityScales(lockedCenter, openIdx)

	var knightMovementBB [2]uint64
	var bishopMovementBB [2]uint64
	var rookMovementBB [2]uint64
	var queenMovementBB [2]uint64
	var kingAttackMobilityBB [2]uint64
	var danger kingDanger

	kingRing := getKingSafetyTable(b, true, 0, 0)

	allPieces := b.White.All | b.Black.All

	knightMG, knightEG := evaluateKnights(
		b,
		wPawnAttackBB, bPawnAttackBB,
		kingRing,
		scales,
		whiteOutposts, blackOutposts,
		&knightMovementBB,
		&kingAttackMobilityBB,
		&danger,
		false,
	)

	bishopMG, bishopEG := evaluateBishops(
		b,
		allPieces,
		wPawnAttackBB, bPawnAttackBB,
		kingRing,
		scales,
		whiteOutposts, blackOutposts,
		pawnEntry.WBlockedBB, pawnEntry.BBlockedBB,
		&bishopMovementBB,
		&kingAttackMobilityBB,
		&danger,
		false,
	)

	rookMG, rookEG := evaluateRooks(
		b,
		allPieces,
		wPawnAttackBB, bPawnAttackBB,
		kingRing,
		openFiles, wSemiOpenFiles, bSemiOpenFiles,
		wRookStackFiles, bRookStackFiles,
		&rookMovementBB,
		&kingAttackMobilityBB,
		&danger,
		false,
	)

	queenMG, queenEG := evaluateQueens(
		b,
		allPieces,
		wPawnAttackBB, bPawnAttackBB,
		kingRing,
		&queenMovementBB,
		&kingAttackMobilityBB,
		&danger,
		false,
	)

	kingCheckThreats(b, &danger, allPieces, wPawnAttackBB, bPawnAttackBB,
		knightMovementBB, bishopMovementBB, queenMovementBB)

	kingPsqtMG, kingPsqtEG := countPieceTables(&b.White.Kings, &b.Black.Kings, &PSQT_MG[gm.PieceTypeKing], &PSQT_EG[gm.PieceTypeKing])
	kingAttackPenaltyMG, kingAttackPenaltyEG := kingDangerScore(&danger, b)
	kingShelterMG, kingStormMG, kingStormEG := kingShelterStorm(b, wPawnAttackBB, bPawnAttackBB)
	kingMinorDefenseBonusMG := kingMinorPieceDefences(kingRing, knightMovementBB, bishopMovementBB)
	kingPasserProximityEG := kingPasserProximity(b, pawnEntry)

	wPieceCount := bits.OnesCount64(b.White.Bishops | b.White.Knights | b.White.Rooks | b.White.Queens)
	bPieceCount := bits.OnesCount64(b.Black.Bishops | b.Black.Knights | b.Black.Rooks | b.Black.Queens)
	wPawnCount := bits.OnesCount64(b.White.Pawns)
	bPawnCount := bits.OnesCount64(b.Black.Pawns)

	kingCentralManhattanPenalty := 0
	kingMopUpBonus := 0
	piecePhase := GetPiecePhase(b)

	if (piecePhase < 16 && bits.OnesCount64(b.White.Queens|b.Black.Queens) == 0) || piecePhase < 10 {
		noPawnsLeft := wPawnCount == 0 && bPawnCount == 0
		if wPieceCount > 0 && bPieceCount == 0 && noPawnsLeft {
			kingMopUpBonus = getKingMopUpBonus(b, true, b.White.Queens > 0, b.White.Rooks > 0)
		} else if wPieceCount == 0 && bPieceCount > 0 && noPawnsLeft {
			// getKingMopUpBonus already negates for the black-advantage case, so
			// this must not negate again -- it did, and reported the mop-up
			// backwards whenever black was the side mating.
			kingMopUpBonus = getKingMopUpBonus(b, false, b.Black.Queens > 0, b.Black.Rooks > 0)
		} else {
			kingCentralManhattanPenalty = kingEndGameCentralizationPenalty(b)
		}
	}

	spaceMG := spaceEvaluation(b, pawnEntry)

	wMaterialMG, wMaterialEG := countMaterial(&b.White)
	bMaterialMG, bMaterialEG := countMaterial(&b.Black)
	materialScoreMG := wMaterialMG - bMaterialMG
	materialScoreEG := wMaterialEG - bMaterialEG

	imbalanceMG, imbalanceEG := materialImbalance(b)

	pawnMG := pawnEntry.PawnScoreMG
	pawnEG := pawnEntry.PawnScoreEG

	passedMG, passedEG := passedPawnBonus(pawnEntry.WPassedBB, pawnEntry.BPassedBB)
	candMG, candEG, _, _ := CandidatePassedTerm(b, pawnEntry)
	// Candidate is no longer inside the cached PawnScore; add it to the pawn
	// total here so pawnStruct = pawn - passed keeps its meaning.
	pawnMG += candMG
	pawnEG += candEG
	passedMG += candMG
	passedEG += candEG

	pawnStructMG := pawnMG - passedMG
	pawnStructEG := pawnEG - passedEG

	weight := func(mg, eg int) int32 {
		mgWeight := piecePhase
		egWeight := TotalPhase - piecePhase
		return int32((mg*mgWeight + eg*egWeight) / TotalPhase)
	}

	breakdown.Material = weight(materialScoreMG, materialScoreEG)
	breakdown.Imbalance = weight(imbalanceMG, imbalanceEG)
	breakdown.PawnStructure = weight(pawnStructMG, pawnStructEG)
	breakdown.PassedPawns = weight(passedMG, passedEG)
	breakdown.KnightActivity = weight(knightMG, knightEG)
	breakdown.BishopActivity = weight(bishopMG, bishopEG)
	breakdown.RookActivity = weight(rookMG, rookEG)
	breakdown.QueenActivity = weight(queenMG, queenEG)
	breakdown.Activity = breakdown.KnightActivity + breakdown.BishopActivity + breakdown.RookActivity + breakdown.QueenActivity
	breakdown.Space = weight(spaceMG, 0)
	breakdown.KingAttackPressure = weight(kingAttackPenaltyMG, kingAttackPenaltyEG)
	// The storm belongs here rather than under pawn structure: it is scored on
	// the defending king's own three files and moves with the king, not with the
	// pawn chain.
	breakdown.KingPawnShield = weight(kingShelterMG+kingStormMG+kingMinorDefenseBonusMG+kingPsqtMG, kingStormEG+kingPsqtEG)
	breakdown.KingEndgame = weight(0, kingCentralManhattanPenalty+kingMopUpBonus+kingPasserProximityEG)
	breakdown.KingSafety = breakdown.KingAttackPressure + breakdown.KingPawnShield + breakdown.KingEndgame
	breakdown.WPawnAttackBB = wPawnAttackBB
	breakdown.BPawnAttackBB = bPawnAttackBB
	// danger index 0 = white's attackers bearing on the BLACK king; index 1 =
	// black's attackers on the WHITE king. Surface both the raw accumulator and
	// the centipawn value the engine actually applies.
	rawToBlack, _ := kingDangerRaw(&danger, 0, b.White.Queens != 0)
	rawToWhite, _ := kingDangerRaw(&danger, 1, b.Black.Queens != 0)
	cpToBlack, _ := kingDangerFor(&danger, 0, b.White.Queens != 0)
	cpToWhite, _ := kingDangerFor(&danger, 1, b.Black.Queens != 0)
	breakdown.AttackOnBlackKing = int32(rawToBlack)
	breakdown.AttackOnWhiteKing = int32(rawToWhite)
	breakdown.DangerToBlackKing = int32(cpToBlack)
	breakdown.DangerToWhiteKing = int32(cpToWhite)

	return totalCp, breakdown
}
