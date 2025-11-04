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

// Capture offset, also used for promotions; used to captures & PV moves
// Should always be above quiet move heuristics
var captureOffset uint16 = 20000

// Offset values for prioritizing quiet move heuristics
var killerOffset uint16 = 2000
var counterOffset uint16 = 1000

// Ordering the moves one at a time, at index given
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
	killerIdx := int(ply)
	if killerIdx < 0 {
		killerIdx = 0
	} else if killerIdx >= len(killerMoveTable.KillerMoves) {
		killerIdx = len(killerMoveTable.KillerMoves) - 1
	}

	movesList.moves = make([]move, len(moves))
	for i := 0; i < len(moves); i++ {
		move := moves[i]
		var moveEval uint16 = 0
		capturedPiece := move.CapturedPiece()
		capturedType := capturedPiece.Type()
		isEnPassant := move.Flags() == gm.FlagEnPassant
		if isEnPassant {
			capturedType = gm.PieceTypePawn
		}
		isCapture := capturedPiece != gm.NoPiece || isEnPassant
		promotePiece := move.PromotionPieceType()
		isPVMove := (move == pvMove)

		if isPVMove {
			moveEval = captureOffset + 256 // max above is scoreOffset + 256, highest from mvvlva is 54
		} else if promotePiece != 0 {
			moveEval = captureOffset + uint16(pieceValueEG[promotePiece])
		} else if isCapture {
			pieceTypeFrom := move.MovedPiece().Type()
			moveEval = captureOffset + mvvLva[capturedType][pieceTypeFrom]
		} else if killerMoveTable.KillerMoves[killerIdx][0] == move {
			moveEval = killerOffset + 100
		} else if killerMoveTable.KillerMoves[killerIdx][1] == move {
			moveEval = killerOffset
		} else {
			moveEval = uint16(historyMove[side][move.From()][move.To()])
			if counterMove[side][prevMove.From()][prevMove.To()] == move {
				moveEval += counterOffset
			}
		}

		movesList.moves[i].move = move
		movesList.moves[i].score = moveEval
	}
	return movesList
}

func scoreMovesListCaptures(_ *gm.Board, moves []gm.Move) (movesList moveList, anyCaptures bool) {
	movesList.moves = make([]move, len(moves)) // Could maybe do better, but whatever
	var capturedMovesIndex uint8

	for i := 0; i < len(moves); i++ {
		move := moves[i]
		capturedPiece := move.CapturedPiece()
		capturedType := capturedPiece.Type()
		if move.Flags() == gm.FlagEnPassant {
			capturedType = gm.PieceTypePawn
		}
		if capturedPiece != gm.NoPiece || move.Flags() == gm.FlagEnPassant {
			moverType := move.MovedPiece().Type()
			score := mvvLva[capturedType][moverType]
			movesList.moves[capturedMovesIndex].move = move
			movesList.moves[capturedMovesIndex].score = score
			capturedMovesIndex++
		}
	}
	movesList.moves = movesList.moves[:capturedMovesIndex]

	return movesList, capturedMovesIndex > 0
}
