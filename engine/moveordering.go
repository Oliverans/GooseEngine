package engine

import (
	gm "chess-engine/goosemg"
)

type move struct {
	move     gm.Move
	score    int32
	seeScore int32
}

type moveList struct {
	moves []move
}

type movePickerStage uint8

const (
	movePickerStageTT movePickerStage = iota
	movePickerStageGenerateTacticals
	movePickerStageGoodCaptures
	movePickerStageGenerateQuiets
	movePickerStageQuiets
	movePickerStageBadCaptures
	movePickerStageDone
)

type movePickerCursor struct {
	list  moveList
	index uint8
}

type movePicker struct {
	board    *gm.Board
	depth    int8
	ply      int8
	ttMove   gm.Move
	prevMove gm.Move

	stage      movePickerStage
	skipQuiets bool
	moveIndex  uint8
	hasMoves   bool

	captures    movePickerCursor
	quiets      movePickerCursor
	badCaptures movePickerCursor
}

// Most Valuable Victim - Least Valuable Aggressor; used to score & sort captures
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
// 8. Quiet moves:      500,000 + history + cont_history (can go negative but still above losing captures)
// 9. Losing captures:  100,000 + MVV-LVA (still tried, but last)
// 10. Under-promos:     50,000 + piece value

const (
	scorePVMove           int32 = 2_000_000_000
	scoreQueenPromo       int32 = 1_000_000
	scoreWinningCapture   int32 = 900_000
	scoreEqualCapture     int32 = 800_000
	scoreKiller1          int32 = 700_000
	scoreKiller2          int32 = 690_000
	scoreCounterMove      int32 = 600_000
	scoreQuietBase        int32 = 500_000
	scoreSEEPruningCutoff int32 = (scoreCounterMove + scoreQuietBase) / 2
	scoreLosingCapture    int32 = 100_000
	scoreUnderPromo       int32 = 50_000
)

const MaxPlyMoveList = 128
const MaxMovesPerPosition = 256

// Pre-allocated move lists per ply depth for *all* moves.
var moveListPool [MaxPlyMoveList][MaxMovesPerPosition]move
var moveListLengths [MaxPlyMoveList]int

var movePickerRawPool [MaxPlyMoveList][MaxMovesPerPosition]gm.Move
var movePickerCapturePool [MaxPlyMoveList][MaxMovesPerPosition]move
var movePickerQuietPool [MaxPlyMoveList][MaxMovesPerPosition]move
var movePickerBadCapturePool [MaxPlyMoveList][MaxMovesPerPosition]move
var captureMovesTriedPool [MaxPlyMoveList][MaxMovesPerPosition]gm.Move

var qMoveRawPool [MaxPlyMoveList][MaxMovesPerPosition]gm.Move
var qMoveListPool [MaxPlyMoveList][MaxMovesPerPosition]move

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

func newMovePicker(board *gm.Board, depth int8, ply int8, ttMove gm.Move, prevMove gm.Move) movePicker {
	return movePicker{
		board:    board,
		depth:    depth,
		ply:      ply,
		ttMove:   ttMove,
		prevMove: prevMove,
		stage:    movePickerStageTT,
	}
}

func (p *movePicker) Next() (move, uint8, bool) {
	for {
		switch p.stage {
		case movePickerStageTT:
			p.stage = movePickerStageGenerateTacticals
			if p.ttMove != 0 {
				index := p.moveIndex
				p.moveIndex++
				return move{move: p.ttMove}, index, true
			}

		case movePickerStageGenerateTacticals:
			poolIndex := movePickerPoolIndex(p.ply)
			rawMoves := p.board.GenerateTacticalsInto(movePickerRawPool[poolIndex][:0])
			SearchState.cutStats.MovesGenerated += uint64(len(rawMoves))
			p.hasMoves = p.hasMoves || len(rawMoves) != 0

			scored := scoreMovesListInto(
				p.board, rawMoves, p.depth, p.ply, p.ttMove, p.prevMove,
				movePickerCapturePool[poolIndex][:len(rawMoves)],
			)
			goodCount := 0
			badCount := 0
			for _, entry := range scored.moves {
				if entry.move == p.ttMove {
					continue
				}
				if isGoodTactical(entry) {
					movePickerCapturePool[poolIndex][goodCount] = entry
					goodCount++
				} else {
					movePickerBadCapturePool[poolIndex][badCount] = entry
					badCount++
				}
			}
			p.captures.list.moves = movePickerCapturePool[poolIndex][:goodCount]
			p.badCaptures.list.moves = movePickerBadCapturePool[poolIndex][:badCount]
			p.stage = movePickerStageGoodCaptures

		case movePickerStageGoodCaptures:
			cursor := &p.captures
			if int(cursor.index) >= len(cursor.list.moves) {
				p.stage = movePickerStageGenerateQuiets
				continue
			}

			orderNextMove(cursor.index, &cursor.list)
			entry := cursor.list.moves[cursor.index]
			cursor.index++
			index := p.moveIndex
			p.moveIndex++
			return entry, index, true

		case movePickerStageGenerateQuiets:
			if p.skipQuiets {
				p.stage = movePickerStageBadCaptures
				continue
			}

			poolIndex := movePickerPoolIndex(p.ply)
			rawMoves := p.board.GenerateQuietsInto(movePickerRawPool[poolIndex][:0])
			SearchState.cutStats.MovesGenerated += uint64(len(rawMoves))
			p.hasMoves = p.hasMoves || len(rawMoves) != 0

			scored := scoreMovesListInto(
				p.board, rawMoves, p.depth, p.ply, p.ttMove, p.prevMove,
				movePickerQuietPool[poolIndex][:len(rawMoves)],
			)
			quietCount := 0
			badCount := len(p.badCaptures.list.moves)
			for _, entry := range scored.moves {
				if entry.move == p.ttMove {
					continue
				}
				promotion := entry.move.PromotionPieceType()
				if promotion == gm.PieceTypeQueen {
					continue
				}
				if promotion != gm.PieceTypeNone {
					movePickerBadCapturePool[poolIndex][badCount] = entry
					badCount++
					continue
				}
				movePickerQuietPool[poolIndex][quietCount] = entry
				quietCount++
			}
			p.quiets.list.moves = movePickerQuietPool[poolIndex][:quietCount]
			p.badCaptures.list.moves = movePickerBadCapturePool[poolIndex][:badCount]
			p.stage = movePickerStageQuiets

		case movePickerStageQuiets:
			cursor := &p.quiets
			if int(cursor.index) >= len(cursor.list.moves) {
				p.stage = movePickerStageBadCaptures
				continue
			}

			orderNextMove(cursor.index, &cursor.list)
			entry := cursor.list.moves[cursor.index]
			cursor.index++
			index := p.moveIndex
			p.moveIndex++
			return entry, index, true

		case movePickerStageBadCaptures:
			cursor := &p.badCaptures
			if int(cursor.index) >= len(cursor.list.moves) {
				p.stage = movePickerStageDone
				continue
			}

			orderNextMove(cursor.index, &cursor.list)
			entry := cursor.list.moves[cursor.index]
			cursor.index++
			index := p.moveIndex
			p.moveIndex++
			return entry, index, true

		default:
			return move{}, 0, false
		}
	}
}

func movePickerPoolIndex(ply int8) int {
	index := int(ply)
	if index < 0 {
		return 0
	}
	if index >= MaxPlyMoveList {
		return MaxPlyMoveList - 1
	}
	return index
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

	moves.moves[currIndex], moves.moves[bestIndex] = moves.moves[bestIndex], moves.moves[currIndex]
}

func isGoodTactical(entry move) bool {
	promotion := entry.move.PromotionPieceType()
	return promotion == gm.PieceTypeQueen || (promotion == gm.PieceTypeNone && entry.seeScore >= 0)
}

func scoreMovesList(board *gm.Board, moves []gm.Move, depth int8, ply int8, pvMove gm.Move, prevMove gm.Move) (movesList moveList) {
	return scoreMovesListInto(board, moves, depth, ply, pvMove, prevMove, GetMoveListForPly(ply, len(moves)))
}

func scoreMovesListInto(board *gm.Board, moves []gm.Move, _ int8, ply int8, pvMove gm.Move, prevMove gm.Move, scored []move) (movesList moveList) {
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

	movesList.moves = scored[:len(moves)]

	// Get continuation history context once for all moves
	prev1Ply, prev2Ply := SearchState.ContHistContext(ply)

	for i := range moves {
		mv := moves[i]
		var moveEval int32
		var negativeSEE int32

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
					negativeSEE = int32(seeScore)
				}
			}
			moveEval += int32(captureHistoryScore(mv))

		} else if SearchState.killer.KillerMoves[killerIdx][0] == mv {
			// First killer - high priority quiet move
			moveEval = scoreKiller1

		} else if SearchState.killer.KillerMoves[killerIdx][1] == mv {
			// Second killer
			moveEval = scoreKiller2

		} else {
			// Regular quiet move: combine main history + continuation history
			histScore := int32(SearchState.historyMoves[side][mv.From()][mv.To()])

			// NEW: Add continuation history score (weighted at 50%)
			contScore := int32(ContHistScore(side, mv, prev1Ply, prev2Ply))
			combinedHist := histScore + contScore/2

			moveEval = scoreQuietBase + combinedHist

			// Counter move bonus (still uses combined history for tie-breaking)
			if prevMove != 0 && SearchState.counterMoves[side][prevMove.From()][prevMove.To()] == mv {
				moveEval = scoreCounterMove + combinedHist
			}
		}

		movesList.moves[i].move = mv
		movesList.moves[i].score = moveEval
		movesList.moves[i].seeScore = negativeSEE
	}

	return movesList
}

func scoreMovesListTacticals(moves []gm.Move, ply int8, ttMove gm.Move) (movesList moveList, anyTacticals bool) {
	if ply < 0 {
		ply = 0
	}
	if int(ply) >= MaxPlyMoveList {
		ply = MaxPlyMoveList - 1
	}

	pool := qMoveListPool[ply][:]
	moveIndex := 0

	for i := range moves {
		mv := moves[i]
		capturedPiece := mv.CapturedPiece()
		capturedType := capturedPiece.Type()
		promotion := mv.PromotionPieceType()

		if mv.Flags() == gm.FlagEnPassant {
			capturedType = gm.PieceTypePawn
		}

		isCapture := capturedPiece != gm.NoPiece || mv.Flags() == gm.FlagEnPassant
		if isCapture || promotion == gm.PieceTypeQueen {
			var score int32
			if mv == ttMove {
				score = scorePVMove
			} else {
				moverType := mv.MovedPiece().Type()
				score = mvvLva[capturedType][moverType]
				if promotion == gm.PieceTypeQueen {
					score = scoreQueenPromo + int32(pieceValueEG[promotion])
					if isCapture {
						score += mvvLva[capturedType][gm.PieceTypePawn]
					}
				} else if isCapture {
					score += int32(captureHistoryScore(mv))
				}
			}

			pool[moveIndex].move = mv
			pool[moveIndex].score = score
			pool[moveIndex].seeScore = 0
			moveIndex++
		}
	}

	movesList.moves = pool[:moveIndex]
	return movesList, moveIndex > 0
}

// IsKiller checks if a move is a killer move at the given ply
func IsKiller(move gm.Move, ply int8, k *KillerStruct) bool {
	index := int(ply)
	if index >= len(k.KillerMoves) {
		index = len(k.KillerMoves) - 1
	}
	return move == k.KillerMoves[index][0] || move == k.KillerMoves[index][1]
}

// HistoryCombinedScore returns the combined history + continuation history score
// Useful for LMR decisions in search
func HistoryCombinedScore(side int, move gm.Move, ply int8) int {
	mainHist := SearchState.historyMoves[side][move.From()][move.To()]
	prev1Ply, prev2Ply := SearchState.ContHistContext(ply)
	contHist := ContHistScore(side, move, prev1Ply, prev2Ply)
	return mainHist + contHist/2
}
