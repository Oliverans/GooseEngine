package engine

import (
	gm "chess-engine/goosemg"
)

type move struct {
	move  gm.Move
	score uint16
}

type moveList struct {
	moves []move
}

// Most Valuable Victim - Least Valuable Aggressor; used to score & sort captures
var mvvLva [7][7]uint16 = [7][7]uint16{
	{0, 0, 0, 0, 0, 0, 0},
	{0, 14, 13, 12, 11, 10, 0}, // victim Pawn
	{0, 24, 23, 22, 21, 20, 0}, // victim Knight
	{0, 34, 33, 32, 31, 30, 0}, // victim Bishop
	{0, 44, 43, 42, 41, 40, 0}, // victim Rook
	{0, 54, 53, 52, 51, 50, 0}, // victim Queen
	{0, 0, 0, 0, 0, 0, 0},      // victim King
}

var SortingCaptures int
var SortingNormal int

// Capture offset, also used for promotions; used for captures & PV moves.
// Should always be above quiet move heuristics.
var captureOffset uint16 = 20000

// Offset values for prioritizing quiet move heuristics.
var killerOffset uint16 = 2000
var counterOffset uint16 = 1000

// Quiet moves need a base offset so they always score above losing captures.
var quietOffset uint16 = 5000

const MaxPlyMoveList = 128
const MaxMovesPerPosition = 256

// Pre-allocated move lists per ply depth for *all* moves.
var moveListPool [MaxPlyMoveList][MaxMovesPerPosition]move
var moveListLengths [MaxPlyMoveList]int

// For quiescence, we have a separate pre-allocated pool (captures only).
var qMoveListPool [MaxPlyMoveList][64]move

// GetMoveListForPly returns a pre-allocated slice for the given ply.
func GetMoveListForPly(ply int8, count int) []move {
	if ply < 0 {
		ply = 0
	}
	if int(ply) >= MaxPlyMoveList {
		ply = MaxPlyMoveList - 1
	}
	// NOTE: If you ever see panics here, either increase MaxMovesPerPosition
	// or clamp count before slicing.
	moveListLengths[ply] = count
	return moveListPool[ply][:count]
}

// QuickSEEWinning provides a fast upper bound on SEE value.
func QuickSEEWinning(_ *gm.Board, move gm.Move) bool {
	capturedPiece := move.CapturedPiece()
	if capturedPiece == gm.NoPiece {
		return false
	}

	victimValue := int(SeePieceValue[capturedPiece.Type()])
	attackerValue := int(SeePieceValue[move.MovedPiece().Type()])

	// If we capture something worth more than (or equal to) our piece, it's
	// winning even if we then lose our piece back.
	if victimValue >= attackerValue {
		return true
	}

	// If we capture with a pawn, it's often winning; this is a heuristic.
	if move.MovedPiece().Type() == gm.PieceTypePawn {
		return true
	}

	return false
}

// Ordering the moves one at a time, at index given.
func orderNextMove(currIndex uint8, moves *moveList) {
	bestIndex := currIndex
	bestScore := moves.moves[bestIndex].score

	for index := bestIndex + 1; index < uint8(len(moves.moves)); index++ {
		if moves.moves[index].score > bestScore {
			bestIndex = index
			bestScore = moves.moves[index].score
		}
	}

	tempMove := moves.moves[currIndex]
	moves.moves[currIndex] = moves.moves[bestIndex]
	moves.moves[bestIndex] = tempMove
}

func scoreMovesList(board *gm.Board, moves []gm.Move, _ int8, ply int8, pvMove gm.Move, prevMove gm.Move) (movesList moveList) {
	side := 0
	if !board.Wtomove {
		side = 1
	}

	// Clamp killer index to table bounds.
	killerIdx := int(ply)
	if killerIdx < 0 {
		killerIdx = 0
	} else if killerIdx >= len(killerMoveTable.KillerMoves) {
		killerIdx = len(killerMoveTable.KillerMoves) - 1
	}

	movesList.moves = GetMoveListForPly(ply, len(moves))

	for i := 0; i < len(moves); i++ {
		mv := moves[i]
		var moveEval uint16

		capturedPiece := mv.CapturedPiece()
		capturedType := capturedPiece.Type()

		isEnPassant := mv.Flags() == gm.FlagEnPassant
		if isEnPassant {
			capturedType = gm.PieceTypePawn
		}
		isCapture := capturedPiece != gm.NoPiece || isEnPassant

		promotePiece := mv.PromotionPieceType()
		isPVMove := (mv == pvMove)

		if isPVMove {
			// PV move: always highest score.
			moveEval = ^uint16(0)
		} else if promotePiece != gm.PieceTypeNone {
			// Promotions get captureOffset plus promoted piece value.
			moveEval = captureOffset + uint16(pieceValueEG[promotePiece])
		} else if isCapture {
			// Capture scoring with lazy SEE.
			pieceTypeFrom := mv.MovedPiece().Type()
			captureScore := mvvLva[capturedType][pieceTypeFrom]

			// Lazy SEE: Only call SEE for potentially losing captures.
			// A capture is clearly winning if victim >= attacker.
			victimValue := int(SeePieceValue[capturedType])
			attackerValue := int(SeePieceValue[pieceTypeFrom])

			if victimValue >= attackerValue {
				// Clearly winning capture (e.g., QxP, RxB, BxN, NxP, PxAnything).
				// No need for SEE - assume winning and add a small material diff.
				diff := victimValue - attackerValue
				moveEval = captureOffset + captureScore + uint16(diff)
			} else {
				// Potentially losing capture – need full SEE.
				seeScore := see(board, mv, false)
				if seeScore >= 0 {
					moveEval = captureOffset + captureScore + uint16(seeScore)
				} else {
					// Losing captures go to the back of the line but retain MVV/LVA ordering.
					moveEval = captureScore
				}
			}
		} else if killerMoveTable.KillerMoves[killerIdx][0] == mv {
			// First killer.
			moveEval = quietOffset + killerOffset + 100
		} else if killerMoveTable.KillerMoves[killerIdx][1] == mv {
			// Second killer.
			moveEval = quietOffset + killerOffset
		} else {
			// History + counter move.
			moveEval = quietOffset + uint16(historyMove[side][mv.From()][mv.To()])
			if counterMove[side][prevMove.From()][prevMove.To()] == mv {
				moveEval += counterOffset
			}
		}

		movesList.moves[i].move = mv
		movesList.moves[i].score = moveEval
	}

	return movesList
}

func scoreMovesListCaptures(_ *gm.Board, moves []gm.Move, ply int8) (movesList moveList, anyCaptures bool) {
	if ply < 0 {
		ply = 0
	}
	if int(ply) >= MaxPlyMoveList {
		ply = MaxPlyMoveList - 1
	}

	// Work on the per-ply capture pool.
	pool := qMoveListPool[ply][:]
	var capturedMovesIndex uint8

	for i := 0; i < len(moves); i++ {
		mv := moves[i]
		capturedPiece := mv.CapturedPiece()
		capturedType := capturedPiece.Type()

		if mv.Flags() == gm.FlagEnPassant {
			capturedType = gm.PieceTypePawn
		}

		if capturedPiece != gm.NoPiece || mv.Flags() == gm.FlagEnPassant {
			moverType := mv.MovedPiece().Type()
			score := mvvLva[capturedType][moverType]

			pool[capturedMovesIndex].move = mv
			pool[capturedMovesIndex].score = score
			capturedMovesIndex++
		}
	}

	movesList.moves = pool[:capturedMovesIndex]
	return movesList, capturedMovesIndex > 0
}
