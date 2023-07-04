package engine

import (
	"github.com/dylhunn/dragontoothmg"
)

type move struct {
	move  dragontoothmg.Move
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

// Score offset, used so moves scored via our history map doesn't reach above our mvv-lva moves
var scoreOffset uint16 = 20000

func GetPieceTypeAtPosition(position uint8, bitboards *dragontoothmg.Bitboards) (pieceType dragontoothmg.Piece, occupied bool) {
	if bitboards.Pawns&(1<<position) > 0 {
		return dragontoothmg.Pawn, true
	} else if bitboards.Knights&(1<<position) > 0 {
		return dragontoothmg.Knight, true
	} else if bitboards.Bishops&(1<<position) > 0 {
		return dragontoothmg.Bishop, true
	} else if bitboards.Rooks&(1<<position) > 0 {
		return dragontoothmg.Rook, true
	} else if bitboards.Queens&(1<<position) > 0 {
		return dragontoothmg.Queen, true
	} else if bitboards.Kings&(1<<position) > 0 {
		return dragontoothmg.King, true
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

func scoreMovesList(board *dragontoothmg.Board, moves []dragontoothmg.Move, depth int8, pvMove dragontoothmg.Move, prevMove dragontoothmg.Move) (movesList moveList) {
	var bitboardsOwn dragontoothmg.Bitboards
	var bitboardsOpponent dragontoothmg.Bitboards
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
		isCapture := dragontoothmg.IsCapture(move, board)
		promotePiece := move.Promote()
		isPVMove := (move == pvMove) && pvMove != 0000

		if isPVMove {
			moveEval += scoreOffset + 100 // max above is scoreOffset + 256, highest from mvvlva is 54
		} else if promotePiece != 0 {
			moveEval += scoreOffset + uint16(pieceValueEG[promotePiece])
		} else if isCapture {
			pieceTypeFrom, _ := GetPieceTypeAtPosition(move.From(), &bitboardsOwn)
			enemyPiece, _ := GetPieceTypeAtPosition(move.To(), &bitboardsOpponent)
			moveEval += scoreOffset + mvvLva[enemyPiece][pieceTypeFrom]
		} else if killerMoveTable.KillerMoves[depth][0] == move || killerMoveTable.KillerMoves[depth][1] == move {
			moveEval += scoreOffset
		} else {
			var side int
			if board.Wtomove {
				side = 0
			} else {
				side = 1
			}
			moveEval += uint16(historyMove[side][move.From()][move.To()])
			if counterMove[side][prevMove.From()][prevMove.To()] == move {
				moveEval += moveEval * moveEval
			}
		}
		movesList.moves[i].move = move
		movesList.moves[i].score = moveEval
	}

	return movesList
}

func scoreMovesListCaptures(board *dragontoothmg.Board, moves []dragontoothmg.Move) (movesList moveList, anyCaptures bool) {
	var bitboardsOwn dragontoothmg.Bitboards
	var bitboardsOpponent dragontoothmg.Bitboards
	if board.Wtomove {
		bitboardsOwn = board.White
		bitboardsOpponent = board.Black
	} else {
		bitboardsOwn = board.Black
		bitboardsOpponent = board.White
	}

	movesList.moves = make([]move, len(moves)) // Could maybe do better, but whatever
	for i := 0; i < len(moves); i++ {
		move := moves[i]
		pieceTypeFrom, _ := GetPieceTypeAtPosition(move.From(), &bitboardsOwn)
		enemyPiece, isCapture := GetPieceTypeAtPosition(move.To(), &bitboardsOpponent)
		var move_eval uint16 = 0

		if isCapture {
			move_eval += scoreOffset + mvvLva[enemyPiece][pieceTypeFrom]
			movesList.moves[i].move = move
			movesList.moves[i].score = move_eval
		}
	}
	return movesList, (len(movesList.moves) > 0)
}
