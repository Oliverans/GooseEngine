package engine

import (
	"github.com/dylhunn/dragontoothmg"
)

type move struct {
	move          dragontoothmg.Move
	score         uint16
	capturedPiece dragontoothmg.Piece
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

/*
	Move ordering offsets!
	- PV moves should be considered first, as it will most likely guide us to the best path in IID; or the failed path in some beta-cutoffs so we can quit as early as possible.
	- Promotions feels like it should be super important the few times it can occur; while this logic might not be 100% solid I've just put this high up :)
	- Captures are important so we never miss any tactical shots, which most likely would mean immediately losing the game
		- Whether we should have ALL captures first or not is difficult to say; perhaps "winning" MvvLva-captures should be above non-winning ones
	- History has the most weight out of all other moves, and we prefer killers over counters
	- The rest... Good luck :)
*/
// Should always be above quiet move heuristics
var pvOffset uint16 = 25000
var promotionOffset uint16 = 20000
var captureOffset uint16 = 15000

// Offset values for prioritizing quiet move heuristics
var killerOffset uint16 = 2000
var counterOffset uint16 = 1000

// Nice helper to get what piece is at a square :)
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
		var move dragontoothmg.Move = moves[i]
		var moveEval uint16 = 0
		var capturedPiece, isCapture = GetPieceTypeAtPosition(move.To(), &bitboardsOpponent)
		var promotePiece dragontoothmg.Piece = move.Promote()
		var isPVMove bool = (move == pvMove)

		if isPVMove {
			moveEval = pvOffset + 1500 // max above is scoreOffset + 1500, highest from mvvlva is 54
		} else if promotePiece > 0 {
			moveEval = promotionOffset + uint16(PieceValueEG[promotePiece])
		} else if isCapture {
			pieceTypeFrom, _ := GetPieceTypeAtPosition(move.From(), &bitboardsOwn)
			moveEval = captureOffset + mvvLva[capturedPiece][pieceTypeFrom]
		} else if killerMoveTable.KillerMoves[depth][0] == move {
			moveEval = killerOffset + 200
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
		movesList.moves[i].capturedPiece = capturedPiece
	}
	return movesList
}

func scoreMovesListCaptures(board *dragontoothmg.Board, moves []dragontoothmg.Move, pvMove dragontoothmg.Move) (movesList moveList, anyCaptures bool) {
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
	var capturedMovesIndex uint8

	for i := 0; i < len(moves); i++ {
		move := moves[i]

		var isPromotion bool = move.Promote() > 0
		var ourPiece, _ = GetPieceTypeAtPosition(move.From(), &bitboardsOwn)
		var enemyPiece, isCapture = GetPieceTypeAtPosition(move.To(), &bitboardsOpponent)

		if isCapture || isPromotion {
			var move_eval uint16 = 0
			if move == pvMove { // If we have a TT entry or PV-move; let's use this first
				move_eval = captureOffset + 256
			} else if isPromotion { // else we score promotions above any capture
				move_eval = captureOffset + 75
			} else { // else mvvLva for the rest
				move_eval = mvvLva[enemyPiece][ourPiece]
			}

			movesList.moves[capturedMovesIndex].move = move
			movesList.moves[capturedMovesIndex].score = move_eval
			movesList.moves[capturedMovesIndex].capturedPiece = enemyPiece
			capturedMovesIndex++
		}
	}
	movesList.moves = movesList.moves[:capturedMovesIndex]

	return movesList, capturedMovesIndex > 0
}
