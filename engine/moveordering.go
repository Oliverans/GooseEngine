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

// Nice helper to get what piece is at a square :)
func GetPieceTypeAtPosition(position uint8, bitboards *gm.Bitboards) (pieceType gm.PieceType, occupied bool) {
	if bitboards.Pawns&(1<<position) > 0 {
		return gm.PieceTypePawn, true
	} else if bitboards.Knights&(1<<position) > 0 {
		return gm.PieceTypeKnight, true
	} else if bitboards.Bishops&(1<<position) > 0 {
		return gm.PieceTypeBishop, true
	} else if bitboards.Rooks&(1<<position) > 0 {
		return gm.PieceTypeRook, true
	} else if bitboards.Queens&(1<<position) > 0 {
		return gm.PieceTypeQueen, true
	} else if bitboards.Kings&(1<<position) > 0 {
		return gm.PieceTypeKing, true
	}
	return 0, false
}

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

func scoreMovesList(board *gm.Board, moves []gm.Move, depth int8, pvMove gm.Move, prevMove gm.Move) (movesList moveList) {
	var bitboardsOwn gm.Bitboards
	var bitboardsOpponent gm.Bitboards
	if board.Wtomove {
		bitboardsOwn = board.White
		bitboardsOpponent = board.Black
	} else {
		bitboardsOwn = board.Black
		bitboardsOpponent = board.White
	}

	movesList.moves = make([]move, len(moves))
	for i := 0; i < len(moves); i++ {
		move := moves[i]
		var moveEval uint16 = 0
		isCapture := gm.IsCapture(move, board)
		promotePiece := move.PromotionPieceType()
		isPVMove := (move == pvMove)

		if isPVMove {
			moveEval = captureOffset + 256 // max above is scoreOffset + 256, highest from mvvlva is 54
		} else if promotePiece != 0 {
			moveEval = captureOffset + uint16(pieceValueEG[promotePiece])
		} else if isCapture {
			pieceTypeFrom, _ := GetPieceTypeAtPosition(uint8(move.From()), &bitboardsOwn)
			enemyPiece, _ := GetPieceTypeAtPosition(uint8(move.To()), &bitboardsOpponent)
			moveEval = captureOffset + mvvLva[enemyPiece][pieceTypeFrom]
		} else if killerMoveTable.KillerMoves[depth][0] == move {
			moveEval = killerOffset + 100
		} else if killerMoveTable.KillerMoves[depth][1] == move {
			moveEval = killerOffset
		} else {
			var side int
			if board.Wtomove {
				side = 0
			} else {
				side = 1
			}
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

func scoreMovesListCaptures(board *gm.Board, moves []gm.Move) (movesList moveList, anyCaptures bool) {
	var bitboardsOwn gm.Bitboards
	var bitboardsOpponent gm.Bitboards
	if board.Wtomove {
		bitboardsOwn = board.White
		bitboardsOpponent = board.Black
	} else {
		bitboardsOwn = board.Black
		bitboardsOpponent = board.White
	}

	movesList.moves = make([]move, len(moves)) // Could maybe do better, but whatever
	var capturedMovesIndex uint8

	for i := 0; i < len(moves); i++ {
		move := moves[i]
		ourPiece, _ := GetPieceTypeAtPosition(uint8(move.From()), &bitboardsOwn)
		enemyPiece, isCapture := GetPieceTypeAtPosition(uint8(move.To()), &bitboardsOpponent)
		var move_eval uint16 = 0

		if isCapture {
			move_eval += mvvLva[enemyPiece][ourPiece]
			movesList.moves[capturedMovesIndex].move = move
			movesList.moves[capturedMovesIndex].score = move_eval
			capturedMovesIndex++
		}
	}
	movesList.moves = movesList.moves[:capturedMovesIndex]

	return movesList, capturedMovesIndex > 0
}
