package engine

import (
	"math/bits"

	"github.com/dylhunn/dragontoothmg"
)

type Eval struct {
	Phase          uint8
	Outposts       [2]uint64
	PawnAttackBB   [2]uint64
	KingSafetyZone [2]uint64
	KingAttacks    [2][64]uint8
}

func initEvalVars(board *dragontoothmg.Board, eval *Eval) {
	var piecePhase = getPiecePhase(board)
	var currPhase = TotalPhase - piecePhase
	var phase = uint8((currPhase*256 + (TotalPhase / 2)) / TotalPhase)

	// Pawn attack bitboards
	var wPawnAttackBBEast, wPawnAttackBBWest = PawnCaptureBitboards(board.White.Pawns, true)
	var bPawnAttackBBEast, bPawnAttackBBWest = PawnCaptureBitboards(board.Black.Pawns, false)
	var wPawnAttackBB = wPawnAttackBBEast | wPawnAttackBBWest
	var bPawnAttackBB = bPawnAttackBBEast | bPawnAttackBBWest

	// Potential knight & bishop outpost bitboards
	var wPotentialOutpostsBB = wPawnAttackBB & wAllowedOutpostMask
	var bPotentialOutpostsBB = bPawnAttackBB & bAllowedOutpostMask
	var wOutpostBB uint64
	var bOutpostBB uint64
	var wKingSafetyZone = board.White.Kings
	var bKingSafetyZone = board.Black.Kings

	for sq := 0; sq < 64; sq++ {
		sqBB := PositionBB[sq]
		//file := onlyFile[sq%8]

		// Generate allowed outposts!
		if sqBB&wPotentialOutpostsBB > 0 {
			var filesToCheck = (getFileOfSquare(sq-1) &^ bitboardFileH) | (getFileOfSquare(sq+1) &^ bitboardFileA)
			if bits.OnesCount64(ranksAbove[(sq/8)+1]&filesToCheck&board.Black.Pawns) > 0 {
				wOutpostBB = wOutpostBB | sqBB
			}
		} else if sqBB&bPotentialOutpostsBB > 0 {
			var filesToCheck = (getFileOfSquare(sq-1) &^ bitboardFileH) | (getFileOfSquare(sq+1) &^ bitboardFileA)
			if bits.OnesCount64(ranksBelow[(sq/8)-1]&filesToCheck&board.Black.Pawns) > 0 {
				bOutpostBB = bOutpostBB | sqBB
			}
		}

		if sqBB&board.White.Kings > 0 {
			wKingSafetyZone = wKingSafetyZone | (wKingSafetyZone << 8) | (wKingSafetyZone >> 8)
			wKingSafetyZone = wKingSafetyZone | ((wKingSafetyZone & ^bitboardFileA) >> 1) | ((wKingSafetyZone & ^bitboardFileH) << 1)
		} else if sqBB&board.Black.Kings > 0 {
			bKingSafetyZone = bKingSafetyZone | (bKingSafetyZone << 8) | (bKingSafetyZone >> 8)
			bKingSafetyZone = bKingSafetyZone | ((bKingSafetyZone & ^bitboardFileA) >> 1) | ((bKingSafetyZone & ^bitboardFileH) << 1)
		}

	}

	// Save all values we gathered to our eval struct
	eval.Outposts[0] = wOutpostBB
	eval.Outposts[1] = bOutpostBB
	eval.PawnAttackBB[0] = wPawnAttackBB
	eval.PawnAttackBB[1] = bPawnAttackBB
	eval.Phase = phase

}

func TempEval(board *dragontoothmg.Board) (score int) {
	var eval Eval
	initEvalVars(board, &eval)

	//var mgScore = 0
	//var egScore = 0
	//var piecePhase = getPiecePhase(board)
	//var currPhase = TotalPhase - piecePhase
	//var phase = (currPhase*256 + (TotalPhase / 2)) / TotalPhase
	//
	//// Knight scores
	//var wKnightMovementScore uint16
	//var bKnightMovementScore uint16
	//var wKnightOutpost uint16
	//var bKnightOutpost uint16
	//
	//// Bishop scores
	//var wBishopMovementScore uint16
	//var bBishopMovementScore uint16
	//var wBishopOutpost uint16
	//var bBishopOutpost uint16
	//
	//// Rook scores
	//var wRookMovementScoreMG uint8
	//var wRookMovementScoreEG uint8
	//var bRookMovementScore uint8
	//var wRookFileBonus uint8
	//var bRookFileBonus uint8
	//
	//var wQueenpMovementScore uint16
	//var wKingMovementScore uint16
	//
	//var bQueenpMovementScore uint16
	//var bKingMovementScore uint16
	//
	//allPieces := board.White.All | board.Black.All
	//
	//// Iterate through all pieces on the board
	//for x := allPieces; x != 0; x &= x - 1 {
	//	sq := uint8(bits.TrailingZeros64(x))
	//	sqBB := PositionBB[sq]
	//
	//	// Check whether we're a white or black piece
	//	piece, isWhite := GetPieceTypeAtPosition(sq, &board.White)
	//	if !isWhite { // Swap ...
	//		piece, _ = GetPieceTypeAtPosition(sq, &board.Black)
	//	}
	//
	//	// All eval calculations is put in here
	//	switch piece {
	//	case dragontoothmg.Pawn:
	//	case dragontoothmg.Knight:
	//		if isWhite {
	//			wKnightMovementScore += uint16(bits.OnesCount64((KnightMasks[sq] &^ board.White.All) &^ eval.PawnAttackBB[1]))
	//			wKnightOutpost += uint16(bits.OnesCount64(eval.Outposts[0] & sqBB))
	//		} else {
	//			bKnightMovementScore += uint16(bits.OnesCount64((KnightMasks[sq] &^ board.Black.All) &^ eval.PawnAttackBB[0]))
	//			bKnightOutpost += uint16(bits.OnesCount64(eval.Outposts[1] & sqBB))
	//		}
	//	case dragontoothmg.Bishop:
	//		if isWhite {
	//			wBishopMovementScore += uint16(bits.OnesCount64(dragontoothmg.CalculateBishopMoveBitboard(sq, (board.White.All|board.Black.All))&^board.White.All) &^ int(eval.PawnAttackBB[1]))
	//			wBishopOutpost += uint16(bits.OnesCount64(eval.Outposts[0] & sqBB))
	//		} else {
	//			bBishopMovementScore += uint16(bits.OnesCount64(dragontoothmg.CalculateBishopMoveBitboard(sq, (board.White.All|board.Black.All))&^board.Black.All) &^ int(eval.PawnAttackBB[0]))
	//			bBishopOutpost += uint16(bits.OnesCount64(eval.Outposts[1] & sqBB))
	//		}
	//	case dragontoothmg.Rook:
	//		if isWhite {
	//			wRookMovementScore += uint8(dragontoothmg.CalculateRookMoveBitboard(uint8(sq), (board.White.All|board.Black.All)) & ^board.White.All)
	//		} else {
	//			bRookMovementScore += uint8(dragontoothmg.CalculateRookMoveBitboard(uint8(sq), (board.White.All|board.Black.All)) & ^board.Black.All)
	//		}
	//	case dragontoothmg.Queen:
	//	case dragontoothmg.King:
	//	}
	//
	//}
	//score = int(((float64(mgScore) * (float64(256) - float64(phase))) + (float64(egScore) * float64(phase))) / float64(256))
	return score
}
