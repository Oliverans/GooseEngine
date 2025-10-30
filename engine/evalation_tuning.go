package engine

import (
	"math/bits"

	"github.com/dylhunn/dragontoothmg"
)

type EvaluationTerms struct {
	// PSQTs

	// Pieces
	PieceValuesMG [7]int
	PieceValuesEG [7]int
	// Passed pawns
	PassedPawnWBB     uint64
	PassedPawnBBB     uint64
	PassedPawnPSQT_MG [64]int
	PassedPawnPSQT_EG [64]int

	// Mobility
	BishopMobility [2]uint64
	KnightMobility [2]uint64
	RookMobility   [2]uint64
	QueenMobility  [2]uint64

	// PAWNS
	DoubledPawns   int
	IsolatedPawns  int
	PhalanxPawns   int
	ConnectedPawns int
	BlockedPawns   int

	// KNIGHTS
	KnightOutposts [2]uint64
	KnightThreats  uint64

	// BISHOPS
	BishopOutpost [2]uint64
	BishopPairs   [2]bool
	//BishopXrayAttackMG                     int
	//BishopColorSetupMG, BishopColorSetupEG int

	// ROOKS
	RookSemiOpenFile int
	RookOpenFile     int
	RookSeventhRank  int
	RookXrayAttack   int

	// QUEENS
	CentralizedQueen  int
	QueenInfiltration int

	// KINGS
	// King general
	KingAttackPenaltyMG, KingAttackPenaltyEG int
	KingPawnShieldPenaltyMG                  int
	KingCentralManhattanPenalty              [2]uint64
	KingDistancePenalty                      int
	KingMinorPieceDefenseBonusMG             int
	KingPawnDefenseMG                        int
	KingPawnDistance                         [2]int
	KingSafety                               int

	MG int
	EG int

	WhitePieceBB [6]uint64
	BlackPieceBB [6]uint64

	WPieceCount int
	BPieceCount int

	MidgamePhase float64
	EndgamePhase float64
}

func countMaterialTerms(bb *dragontoothmg.Bitboards) (
	pawnMG, knightMG, bishopMG, rookMG, queenMG int,
	pawnEG, knightEG, bishopEG, rookEG, queenEG int,
) {
	// PAWNS
	pawns := bits.OnesCount64(bb.Pawns)
	pawnMG = pawns //* PieceValueMG[dragontoothmg.Pawn]
	pawnEG = pawns //* PieceValueEG[dragontoothmg.Pawn]

	// KNIGHTS
	knights := bits.OnesCount64(bb.Knights)
	knightMG = knights //* PieceValueMG[dragontoothmg.Knight]
	knightEG = knights //* PieceValueEG[dragontoothmg.Knight]

	// BISHOPS
	bishops := bits.OnesCount64(bb.Bishops)
	bishopMG = bishops //* PieceValueMG[dragontoothmg.Bishop]
	bishopEG = bishops //* PieceValueEG[dragontoothmg.Bishop]

	// ROOKS
	rooks := bits.OnesCount64(bb.Rooks)
	rookMG = rooks //* PieceValueMG[dragontoothmg.Rook]
	rookEG = rooks //* PieceValueEG[dragontoothmg.Rook]

	// QUEENS
	queens := bits.OnesCount64(bb.Queens)
	queenMG = queens //* PieceValueMG[dragontoothmg.Queen]
	queenEG = queens //* PieceValueEG[dragontoothmg.Queen]

	return
}

func EvaluationTest(b *dragontoothmg.Board) (terms EvaluationTerms) {
	// UPDATE & INIT VARIABLES FOR EVAL
	// Prepare pawn attacks and pawn attack spans

	var wPawnAttackBBEast, wPawnAttackBBWest = PawnCaptureBitboards(b.White.Pawns, true)
	var bPawnAttackBBEast, bPawnAttackBBWest = PawnCaptureBitboards(b.Black.Pawns, false)
	var wPawnAttackBB = wPawnAttackBBEast | wPawnAttackBBWest
	var bPawnAttackBB = bPawnAttackBBEast | bPawnAttackBBWest

	var wPawnAttackSpan, bPawnAttackSpan = pawnAttackSpan(wPawnAttackBB, bPawnAttackBB)

	// Pawn bitboards
	var wPhalanxsPawnsBB, bPhalanxsPawnsBB,
		wBlockedPawnsBB, bBlockedPawnsBB,
		wConnectedPawnsBB, bConnectedPawnsBB,
		wPassedPawnsBB, bPassedPawnsBB,
		wDoubledPawnsBB, bDoubledPawnsBB,
		wIsolatedPawnsBB, bIsolatedPawnsBB,
		wClosestPawn, bClosestPawn = getPawnBBs(b, wPawnAttackBB, bPawnAttackBB) // Last two are integers are not bitboards!

	var openFiles, wSemiOpenFiles, bSemiOpenFiles uint64 = getOpenFiles(b)
	var wQueenInfiltrationBB, bQueenInfiltrationBB = getQueenInfiltrationBB(wPawnAttackSpan, bPawnAttackSpan)

	// Prepare movement bitboard slots
	// For space control calculations
	var knightMovementBB = [2]uint64{}
	var bishopMovementBB = [2]uint64{}
	var rookMovementBB = [2]uint64{}
	var queenMovementBB = [2]uint64{}
	var kingMovementBB = [2]uint64{}

	// Get outpost bitboards
	var outposts = getOutpostsBB(wPawnAttackBB, bPawnAttackBB, wPawnAttackSpan, bPawnAttackSpan)
	whiteOutposts = outposts[0]
	blackOutposts = outposts[1]

	// Get game phase
	var piecePhase = GetPiecePhase(b)
	var currPhase = TotalPhase - piecePhase

	/*
		======================================================================================

		INDIVIDIAL PIECE TUNING VARIABLES! SHOULD MATCH WITH THE EvaluationTerms STRUCT!

		======================================================================================

	*/

	/* KINGS */
	var kingAttackPenaltyMG, _ int
	var kingOpenFilePenaltyMG int
	var KingMinorPieceDefenseBonusMG int
	var kingPawnDefenseMG int

	// For king safety ...
	var attackUnitCounts = [2]int{
		0: 0,
		1: 0,
	}

	var innerKingSafetyZones = getInnerKingSafetyTable(b)
	var outerKingSafetyZones = getOuterKingSafetyTable(innerKingSafetyZones)

	wPieceCount := bits.OnesCount64(b.White.Bishops | b.White.Knights | b.White.Rooks | b.White.Queens)
	bPieceCount := bits.OnesCount64(b.Black.Bishops | b.Black.Knights | b.Black.Rooks | b.Black.Queens)

	// Only used for mobility & king attack count in tuning eval
	for _, piece := range pieceList {
		switch piece {
		case dragontoothmg.Knight:
			for x := b.White.Knights; x != 0; x &= x - 1 {
				square := bits.TrailingZeros64(x)
				movementBB := (KnightMasks[square] &^ b.White.All) &^ bPawnAttackBB
				knightMovementBB[0] |= movementBB
				attackUnitCounts[0] += (bits.OnesCount64(movementBB&innerKingSafetyZones[1]) * attackerInner[dragontoothmg.Knight])
				attackUnitCounts[0] += (bits.OnesCount64(movementBB&outerKingSafetyZones[1]) * attackerOuter[dragontoothmg.Knight])
			}
			for x := b.Black.Knights; x != 0; x &= x - 1 {
				square := bits.TrailingZeros64(x)
				movementBB := (KnightMasks[square] &^ b.Black.All) &^ wPawnAttackBB
				knightMovementBB[1] |= movementBB
				attackUnitCounts[1] += (bits.OnesCount64(movementBB&innerKingSafetyZones[0]) * attackerInner[dragontoothmg.Knight])
				attackUnitCounts[1] += (bits.OnesCount64(movementBB&outerKingSafetyZones[0]) * attackerOuter[dragontoothmg.Knight])
			}
		case dragontoothmg.Bishop:
			for x := b.White.Bishops; x != 0; x &= x - 1 {
				square := bits.TrailingZeros64(x)
				movementBB := dragontoothmg.CalculateBishopMoveBitboard(uint8(square), (b.White.All|b.Black.All)) &^ (b.White.All | bPawnAttackBB)
				bishopMovementBB[0] |= movementBB
				attackUnitCounts[0] += (bits.OnesCount64(movementBB&innerKingSafetyZones[1]) * attackerInner[dragontoothmg.Bishop])
				attackUnitCounts[0] += (bits.OnesCount64(movementBB&outerKingSafetyZones[1]) * attackerOuter[dragontoothmg.Bishop])
			}
			for x := b.Black.Bishops; x != 0; x &= x - 1 {
				square := bits.TrailingZeros64(x)
				movementBB := dragontoothmg.CalculateBishopMoveBitboard(uint8(square), (b.White.All|b.Black.All)) &^ (b.Black.All | wPawnAttackBB)
				bishopMovementBB[1] |= movementBB
				attackUnitCounts[1] += (bits.OnesCount64(movementBB&innerKingSafetyZones[0]) * attackerInner[dragontoothmg.Bishop])
				attackUnitCounts[1] += (bits.OnesCount64(movementBB&outerKingSafetyZones[0]) * attackerOuter[dragontoothmg.Bishop])
			}
		case dragontoothmg.Rook:
			for x := b.White.Rooks; x != 0; x &= x - 1 {
				square := bits.TrailingZeros64(x)
				movementBB := (dragontoothmg.CalculateRookMoveBitboard(uint8(square), ((b.White.All&^b.White.Rooks)|b.Black.All)) & ^b.White.All) &^ bPawnAttackBB
				rookMovementBB[0] |= movementBB
				attackUnitCounts[0] += (bits.OnesCount64(movementBB&innerKingSafetyZones[1]) * attackerInner[dragontoothmg.Rook])
				attackUnitCounts[0] += (bits.OnesCount64(movementBB&outerKingSafetyZones[1]) * attackerOuter[dragontoothmg.Rook])
			}
			for x := b.Black.Rooks; x != 0; x &= x - 1 {
				square := bits.TrailingZeros64(x)
				movementBB := (dragontoothmg.CalculateRookMoveBitboard(uint8(square), (b.White.All|b.Black.All)) & ^b.Black.All) &^ wPawnAttackBB
				rookMovementBB[1] |= movementBB
				attackUnitCounts[1] += (bits.OnesCount64(movementBB&innerKingSafetyZones[0]) * attackerInner[dragontoothmg.Rook])
				attackUnitCounts[1] += (bits.OnesCount64(movementBB&outerKingSafetyZones[0]) * attackerOuter[dragontoothmg.Rook])
			}
		case dragontoothmg.Queen:
			for x := b.White.Queens; x != 0; x &= x - 1 {
				square := bits.TrailingZeros64(x)
				movementBB := dragontoothmg.CalculateBishopMoveBitboard(uint8(square), (b.White.All|b.Black.All)) & ^b.White.All
				movementBB = (movementBB | ((dragontoothmg.CalculateRookMoveBitboard(uint8(square), (b.White.All | b.Black.All))) & ^b.White.All)) &^ bPawnAttackBB
				queenMovementBB[0] |= movementBB
				attackUnitCounts[0] += (bits.OnesCount64(movementBB&innerKingSafetyZones[1]) * attackerInner[dragontoothmg.Queen])
				attackUnitCounts[0] += (bits.OnesCount64(movementBB&outerKingSafetyZones[1]) * attackerOuter[dragontoothmg.Queen])
			}
			for x := b.Black.Queens; x != 0; x &= x - 1 {
				square := bits.TrailingZeros64(x)
				movementBB := dragontoothmg.CalculateBishopMoveBitboard(uint8(square), (b.White.All|b.Black.All)) & ^b.Black.All
				movementBB = (movementBB | ((dragontoothmg.CalculateRookMoveBitboard(uint8(square), (b.White.All | b.Black.All))) & ^b.Black.All)) &^ wPawnAttackBB
				queenMovementBB[1] |= movementBB
				attackUnitCounts[1] += (bits.OnesCount64(movementBB&innerKingSafetyZones[0]) * attackerInner[dragontoothmg.Queen])
				attackUnitCounts[1] += (bits.OnesCount64(movementBB&outerKingSafetyZones[0]) * attackerOuter[dragontoothmg.Queen])
			}
		case dragontoothmg.King:
			kingAttackPenaltyMG = kingAttackCountPenalty(&attackUnitCounts)
			kingOpenFilePenaltyMG = kingFilesPenalty(b, openFiles, wSemiOpenFiles, bSemiOpenFiles)
			kingPawnDefenseMG = kingPawnDefense(b)
			KingMinorPieceDefenseBonusMG = kingMinorPieceDefences(innerKingSafetyZones, knightMovementBB, bishopMovementBB)

			kingMovementBB[0] = (innerKingSafetyZones[0] &^ b.White.All) &^ (knightMovementBB[1] | bishopMovementBB[1] | rookMovementBB[1] | queenMovementBB[1])
			kingMovementBB[1] = (innerKingSafetyZones[1] &^ b.Black.All) &^ (knightMovementBB[0] | bishopMovementBB[0] | rookMovementBB[0] | queenMovementBB[0])
		}
	}

	var kingSafety int = kingAttackPenaltyMG + kingOpenFilePenaltyMG + kingPawnDefenseMG + KingMinorPieceDefenseBonusMG

	/*
		#################################################################################
		TERMS
		We set terms used for the tuning.
		The terms here is simply either the direct bitboards, or we give a count of pieces matching a certain evaluation method.
		This is so we can tune the actual variable values of the engine.

		Later I may simplify the evaluation function to work the same way the tuner works in grabbing the counts.
		So any changes almost only requires a different return value & function name. For eZ copy-pasting.

		#################################################################################
	*/

	var mgPhase = 1 - (float64(currPhase) / 24.0)
	var egPhase = float64(currPhase) / 24.0

	// Assign values from the variables to the struct
	// Assign to terms
	terms.WhitePieceBB = [6]uint64{b.White.Pawns, b.White.Knights, b.White.Bishops, b.White.Rooks, b.White.Queens, b.White.Kings}
	terms.BlackPieceBB = [6]uint64{b.Black.Pawns, b.Black.Knights, b.Black.Bishops, b.Black.Rooks, b.Black.Queens, b.Black.Kings}

	// Pieces
	// Get material contributions for each side
	wPawnMG, wKnightMG, wBishopMG, wRookMG, wQueenMG,
		wPawnEG, wKnightEG, wBishopEG, wRookEG, wQueenEG := countMaterialTerms(&b.White)

	bPawnMG, bKnightMG, bBishopMG, bRookMG, bQueenMG,
		bPawnEG, bKnightEG, bBishopEG, bRookEG, bQueenEG := countMaterialTerms(&b.Black)

	// Store per-piece material values for each side
	wPieceMG := [6]int{wPawnMG, wKnightMG, wBishopMG, wRookMG, wQueenMG, 0}
	bPieceMG := [6]int{bPawnMG, bKnightMG, bBishopMG, bRookMG, bQueenMG, 0}

	wPieceEG := [6]int{wPawnEG, wKnightEG, wBishopEG, wRookEG, wQueenEG, 0}
	bPieceEG := [6]int{bPawnEG, bKnightEG, bBishopEG, bRookEG, bQueenEG, 0}
	// Assign the final difference into terms
	for i := 0; i < 6; i++ {
		terms.PieceValuesMG[i] = wPieceMG[i] - bPieceMG[i]
		terms.PieceValuesEG[i] = wPieceEG[i] - bPieceEG[i]
	}

	/* ============ Mobility ============ */
	terms.KnightMobility = knightMovementBB
	terms.BishopMobility = bishopMovementBB
	terms.RookMobility = rookMovementBB
	terms.QueenMobility = queenMovementBB

	/* ============ Passed Pawns ============ */
	terms.PassedPawnWBB = wPassedPawnsBB
	terms.PassedPawnBBB = bPassedPawnsBB
	terms.PassedPawnPSQT_MG = PassedPawnPSQT_MG
	terms.PassedPawnPSQT_EG = PassedPawnPSQT_EG

	/* ============ Pawns ============ */
	terms.DoubledPawns = (bits.OnesCount64(wDoubledPawnsBB) / 2) - (bits.OnesCount64(bDoubledPawnsBB) / 2)
	terms.IsolatedPawns = bits.OnesCount64(wIsolatedPawnsBB) - bits.OnesCount64(bIsolatedPawnsBB)
	terms.PhalanxPawns = bits.OnesCount64(wPhalanxsPawnsBB) - bits.OnesCount64(bPhalanxsPawnsBB)
	terms.ConnectedPawns = bits.OnesCount64(wConnectedPawnsBB) - bits.OnesCount64(bConnectedPawnsBB)
	terms.BlockedPawns = bits.OnesCount64(wBlockedPawnsBB) - bits.OnesCount64(bBlockedPawnsBB)

	/* ============ Knights ============ */
	terms.KnightOutposts = [2]uint64{(b.White.Knights & whiteOutposts), (b.Black.Knights & blackOutposts)}

	/* ============ Bishops ============ */
	terms.BishopOutpost = [2]uint64{(b.White.Bishops & whiteOutposts), (b.Black.Bishops & blackOutposts)}
	terms.BishopPairs = [2]bool{(bits.OnesCount64(b.White.Bishops) > 1), (bits.OnesCount64(b.Black.Bishops) > 1)}
	//terms.BishopXrayAttackMG = bishopXrayAttackMG
	//terms.BishopColorSetupMG = bishopColorSetupMG
	//terms.BishopColorSetupEG = bishopColorSetupEG

	/* ============ Rooks ============ */
	terms.RookSemiOpenFile = bits.OnesCount64(wSemiOpenFiles&b.White.Rooks) - bits.OnesCount64(bSemiOpenFiles&b.Black.Rooks)
	terms.RookOpenFile = bits.OnesCount64(openFiles&b.White.Rooks) - bits.OnesCount64(openFiles&b.Black.Rooks)
	terms.RookSeventhRank = bits.OnesCount64(b.White.Rooks&seventhRankMask) - bits.OnesCount64(b.Black.Rooks&secondRankMask)
	//terms.RookXrayAttack = rookXrayAttackMG

	/* ============ Queens ============ */
	terms.CentralizedQueen = bits.OnesCount64(b.White.Queens&centralizedQueenSquares) - bits.OnesCount64(b.Black.Queens&centralizedQueenSquares)
	terms.QueenInfiltration = bits.OnesCount64(b.White.Queens&wQueenInfiltrationBB) - bits.OnesCount64(b.Black.Queens&bQueenInfiltrationBB)

	/* ============ Kings ============ */
	//if (piecePhase < 16 && bits.OnesCount64(b.White.Queens|b.Black.Queens) == 0) || piecePhase < 10 {
	//	terms.KingCentralManhattanPenalty = [2]uint64{b.White.Kings, b.Black.Kings}
	//}
	//terms.KingDistancePenalty = kingDistancePenalty
	terms.KingPawnDistance = [2]int{wClosestPawn, bClosestPawn}
	terms.KingSafety = kingSafety

	/* ============ Phase & Piece count ============ */
	terms.WPieceCount = wPieceCount
	terms.BPieceCount = bPieceCount
	terms.MidgamePhase = mgPhase
	terms.EndgamePhase = egPhase

	return terms
}
