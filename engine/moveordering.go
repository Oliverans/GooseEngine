package engine

import (
	gm "chess-engine/goosemg"
)

// CHANGE 1: Use int32 for score to handle negative history values correctly
type move struct {
	move  gm.Move
	score int32
}

type moveList struct {
	moves []move
}

// Most Valuable Victim - Least Valuable Aggressor; used to score & sort captures
// CHANGE 2: Slightly wider spread for better differentiation
var mvvLva [7][7]int32 = [7][7]int32{
	{0, 0, 0, 0, 0, 0, 0},
	{0, 105, 104, 103, 102, 101, 100}, // victim Pawn
	{0, 205, 204, 203, 202, 201, 200}, // victim Knight
	{0, 305, 304, 303, 302, 301, 300}, // victim Bishop
	{0, 405, 404, 403, 402, 401, 400}, // victim Rook
	{0, 505, 504, 503, 502, 501, 500}, // victim Queen
	{0, 0, 0, 0, 0, 0, 0},             // victim King
}

var SortingCaptures int
var SortingNormal int

// Score tiers (from highest to lowest priority):
// 1. PV/TT move:      2,000,000,000 (MaxInt32 essentially)
// 2. Queen promo:     1,000,000 + piece value
// 3. Winning captures: 900,000 + MVV-LVA + SEE bonus
// 4. Equal captures:   800,000 + MVV-LVA
// 5. Killer 1:         700,000
// 6. Killer 2:         690,000
// 7. Counter move:     600,000 + history
// 8. Quiet moves:      500,000 + history (can go negative but still above losing captures)
// 9. Losing captures:  100,000 + MVV-LVA (still tried, but last)
// 10. Under-promos:     50,000 + piece value

const (
	scorePVMove         int32 = 2_000_000_000
	scoreQueenPromo     int32 = 1_000_000
	scoreWinningCapture int32 = 900_000
	scoreEqualCapture   int32 = 800_000
	scoreKiller1        int32 = 700_000
	scoreKiller2        int32 = 690_000
	scoreCounterMove    int32 = 600_000
	scoreQuietBase      int32 = 500_000
	scoreLosingCapture  int32 = 100_000
	scoreUnderPromo     int32 = 50_000
)

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
	moveListLengths[ply] = count
	return moveListPool[ply][:count]
}

// Ordering the moves one at a time, at index given.
// CHANGE 3: Updated to use int32 comparison
func orderNextMove(currIndex uint8, moves *moveList) {
	bestIndex := currIndex
	bestScore := moves.moves[bestIndex].score

	for index := bestIndex + 1; index < uint8(len(moves.moves)); index++ {
		if moves.moves[index].score > bestScore {
			bestIndex = index
			bestScore = moves.moves[index].score
		}
	}

	moves.moves[currIndex], moves.moves[bestIndex] = moves.moves[bestIndex], moves.moves[currIndex]
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
	} else if killerIdx >= len(SearchState.killer.KillerMoves) {
		killerIdx = len(SearchState.killer.KillerMoves) - 1
	}

	movesList.moves = GetMoveListForPly(ply, len(moves))

	for i := range moves {
		mv := moves[i]
		var moveEval int32

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
			// PV/TT move: always searched first
			moveEval = scorePVMove

		} else if promotePiece != gm.PieceTypeNone {
			// Promotions: queen promos high, under-promos lower
			if promotePiece == gm.PieceTypeQueen {
				moveEval = scoreQueenPromo + int32(pieceValueEG[promotePiece])
				// If it's also a capture, add MVV bonus
				if isCapture {
					moveEval += mvvLva[capturedType][gm.PieceTypePawn]
				}
			} else {
				// Under-promotions (knight, rook, bishop) - rare but sometimes needed
				moveEval = scoreUnderPromo + int32(pieceValueEG[promotePiece])
				if isCapture {
					moveEval += mvvLva[capturedType][gm.PieceTypePawn]
				}
			}

		} else if isCapture {
			pieceTypeFrom := mv.MovedPiece().Type()
			captureScore := mvvLva[capturedType][pieceTypeFrom]

			victimValue := int(SeePieceValue[capturedType])
			attackerValue := int(SeePieceValue[pieceTypeFrom])

			if victimValue >= attackerValue {
				diff := int32(victimValue - attackerValue)
				moveEval = scoreWinningCapture + captureScore + diff

			} else {
				// Potentially losing capture - need full SEE
				seeScore := see(board, mv, false)
				if seeScore > 0 {
					// Winning (e.g., protected piece takes unprotected higher piece)
					moveEval = scoreWinningCapture + captureScore + int32(seeScore)
				} else if seeScore == 0 {
					moveEval = scoreEqualCapture + captureScore
				} else {
					moveEval = scoreLosingCapture + captureScore
				}
			}

		} else if SearchState.killer.KillerMoves[killerIdx][0] == mv {
			// First killer - high priority quiet move
			moveEval = scoreKiller1

		} else if SearchState.killer.KillerMoves[killerIdx][1] == mv {
			// Second killer
			moveEval = scoreKiller2

		} else {
			histScore := int32(SearchState.historyMoves[side][mv.From()][mv.To()])
			moveEval = scoreQuietBase + histScore

			// Counter move bonus
			if prevMove != 0 && SearchState.counterMoves[side][prevMove.From()][prevMove.To()] == mv {
				moveEval = scoreCounterMove + histScore
			}
		}

		movesList.moves[i].move = mv
		movesList.moves[i].score = moveEval
	}

	return movesList
}

func scoreMovesListCaptures(moves []gm.Move, ply int8) (movesList moveList, anyCaptures bool) {
	if ply < 0 {
		ply = 0
	}
	if int(ply) >= MaxPlyMoveList {
		ply = MaxPlyMoveList - 1
	}

	pool := qMoveListPool[ply][:]
	var capturedMovesIndex uint8

	for i := range moves {
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

// IsKiller checks if a move is a killer move at the given ply
func IsKiller(move gm.Move, ply int8, k *KillerStruct) bool {
	index := int(ply)
	if index >= len(k.KillerMoves) {
		index = len(k.KillerMoves) - 1
	}
	return move == k.KillerMoves[index][0] || move == k.KillerMoves[index][1]
}
