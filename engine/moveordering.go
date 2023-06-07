package engine

import (
	"time"

	"github.com/dylhunn/dragontoothmg"
)

var MoveOrderingTime time.Duration
var MoveSortingTime time.Duration

/*
Stockfish piece values - might be worth to tinker with.
Middlegame: Knight = 6.198; Bishop = 6.548; Rook = 10.127; Queen = 20.143
Endgame: Knight = 4.106; Bishop = 4.399; Rook = 6.635; Queen = 12.894
*/

var MvvLva [7][7]int = [7][7]int{
	{0, 0, 0, 0, 0, 0, 0},
	{0, 14, 13, 12, 11, 10, 0}, // victim Pawn
	{0, 24, 23, 22, 21, 20, 0}, // victim Knight
	{0, 34, 33, 32, 31, 30, 0}, // victim Bishop
	{0, 44, 43, 42, 41, 40, 0}, // victim Rook
	{0, 54, 53, 52, 51, 50, 0}, // victim Queen
	{0, 0, 0, 0, 0, 0, 0},      // victim King
}
var MvvLvaOffset = 20000

var PositionBB [65]uint64

func GetPieceTypeAtPosition(position uint8, bitboards dragontoothmg.Bitboards) (pieceType dragontoothmg.Piece, occupied bool) {
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
	return
}

func SortMoves(moves []dragontoothmg.Move, board *dragontoothmg.Board, depth int8, alpha int16, beta int16, prevMove dragontoothmg.Move, pvLine PVLine, pvMove dragontoothmg.Move) (move_dict map[dragontoothmg.Move]int) {
	move_dict = make(map[dragontoothmg.Move]int)

	/*
		We get closer to the endgame as the material count lessens
		As we transition, the modifiers affect how we evaluate the board
		TBD:
			A "guessing algorithm" that guesses how much closer we are to the endgame
			based on piece positions
	*/

	var bitboardsOwn dragontoothmg.Bitboards
	var bitboardsOpponent dragontoothmg.Bitboards
	if board.Wtomove {
		bitboardsOwn = board.White
		bitboardsOpponent = board.Black
	} else {
		bitboardsOwn = board.Black
		bitboardsOpponent = board.White
	}

	for _, move := range moves {

		move_eval := 0
		isCapture := dragontoothmg.IsCapture(move, board)
		promotePiece := move.Promote()
		isPVMove := (move == pvMove) && pvMove != 0000

		/*
			For later optimization, I will start using lower variable scopes (uint8 etc);
			Setting an offset
		*/

		if isPVMove {
			move_eval += MvvLvaOffset + 100 // max above is mvvlvaoffset + 256, highest from mvvlva is 54!
		} else if promotePiece != 0 {
			move_eval += MvvLvaOffset + pieceValueEG[promotePiece] // EndGame, since that's where 99.9999999999 of promotions happens; shouldn't be bad even if it happens in midgame anyway!
		} else if isCapture {
			pieceTypeFrom, _ := GetPieceTypeAtPosition(move.From(), bitboardsOwn)
			enemyPiece, _ := GetPieceTypeAtPosition(move.To(), bitboardsOpponent)
			move_eval += (MvvLvaOffset + MvvLva[enemyPiece][pieceTypeFrom])
		} else if killerMoveTable.KillerMoves[depth][0] == move || killerMoveTable.KillerMoves[depth][1] == move {
			move_eval += MvvLvaOffset
		} else {
			var side int
			if board.Wtomove {
				side = 0
			} else {
				side = 1
			}
			move_eval += historyMove[side][move.From()][move.To()]
			if counterMove[side][prevMove.From()][prevMove.To()] == move {
				move_eval += move_eval * move_eval
			}
		}
		move_dict[move] = move_eval
	}
	return move_dict
}

func SortCapturesOnly(moves []dragontoothmg.Move, board *dragontoothmg.Board) map[dragontoothmg.Move]int {
	tmpDict := make(map[dragontoothmg.Move]int)
	var bitboardsOwn dragontoothmg.Bitboards
	var bitboardsOpponent dragontoothmg.Bitboards
	if board.Wtomove {
		bitboardsOwn = board.White
		bitboardsOpponent = board.Black
	} else {
		bitboardsOwn = board.Black
		bitboardsOpponent = board.White
	}

	for _, move := range moves {
		pieceTypeFrom, _ := GetPieceTypeAtPosition(move.From(), bitboardsOwn)
		enemyPiece, isCapture := GetPieceTypeAtPosition(move.To(), bitboardsOpponent)
		var move_eval = 0
		if isCapture {
			/* SEE is now in the qsearch instead of in the move ordering */
			//see := see(board, move, false)
			//if see < 0 {
			//	continue
			//}
			move_eval += MvvLvaOffset + MvvLva[enemyPiece][pieceTypeFrom]
			tmpDict[move] += move_eval
		}
	}
	return tmpDict
}
