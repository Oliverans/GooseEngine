package engine

import (
	"fmt"
	"time"

	gm "chess-engine/goosemg"
)

// =============================================================================
// SCORE CONSTANTS
// =============================================================================
const (
	MaxScore  int32 = 32500
	Checkmate int32 = 20000
	DrawScore int32 = 0
)

// =============================================================================
// MARGINS
// =============================================================================
var FutilityMargins = [8]int32{0, 120, 220, 320, 420, 520, 620, 720}
var RFPMargins = [8]int32{0, 100, 200, 300, 400, 500, 600, 700}
var RazoringMargins = [4]int32{0, 150, 300, 450}

var LateMovePruningMargins = [9]int{0, 3, 5, 9, 14, 20, 27, 35, 44}

// =============================================================================
// LMR/PRUNING PARAMETERS
// =============================================================================
var LMRDepthLimit int8 = 2
var LMRMoveLimit = 2
var LMRHistoryBonus = 500
var LMRHistoryMalus = -100
var NullMoveMinDepth int8 = 3
var QuiescenceSeeMargin int = 150
var ProbCutSeeMargin int = 150

var DeltaMargin int32 = 220
var aspirationWindowSize int32 = 40

// GetAspirationWindowSize returns the current aspiration window size
func GetAspirationWindowSize() int32 {
	return aspirationWindowSize
}

// SetAspirationWindowSize sets the aspiration window size
func SetAspirationWindowSize(val int32) {
	aspirationWindowSize = val
}

func StartSearch(board *gm.Board, depth uint8, gameTime int, increment int, useCustomDepth bool, evalOnly bool, moveOrderingOnly bool, printSearchInformation bool) string {
	initVariables(board)

	//Stat reset
	SearchState.ResetForSearch(board)

	if !SearchState.tt.isInitialized {
		SearchState.tt.init()
	}

	SearchState.GlobalStop = false
	SearchState.timeHandler.initTimemanagement(gameTime, increment, board.FullmoveNumber(), useCustomDepth)
	SearchState.timeHandler.StartTime(board.FullmoveNumber())

	var bestMove gm.Move

	if evalOnly {
		Evaluation(board, true)
		println("Is this a theoretical draw: ", isTheoreticalDraw(board, true))
		return ""
	}

	if moveOrderingOnly {
		dumpRootMoveOrdering(board)
		return ""
	}

	_, bestMove = rootsearch(board, depth, useCustomDepth, printSearchInformation)

	if PrintCutStats {
		dumpCutStats()
		PrintCutStats = false
	}

	return bestMove.String()
}

func rootsearch(b *gm.Board, depth uint8, useCustomDepth bool, printSearchInformation bool) (int, gm.Move) {
	var timeSpent int64
	var alpha int32 = -MaxScore
	var beta int32 = MaxScore
	var bestScore int32 = -MaxScore
	rootIndex := len(SearchState.stateStack) - 1
	var aspCounter = 0

	if SearchState.prevSearchScore != 0 {
		alpha = SearchState.prevSearchScore - aspirationWindowSize
		beta = SearchState.prevSearchScore + aspirationWindowSize
	}

	var nullMove gm.Move
	var bestMove gm.Move
	var pvLine PVLine
	var prevPVLine PVLine
	var mateFound bool

	for i := uint8(1); i <= depth; i++ {
		if !useCustomDepth && i > 1 {
			if SearchState.timeHandler.SoftTimeExceeded() && !SearchState.timeHandler.ShouldExtendTime() {
				break
			}
			if SearchState.timeHandler.ShouldStopEarly() {
				break
			}
		}

		pvLine.Clear()
		mateFound = false

		startTime := time.Now()
		score := alphabeta(b, alpha, beta, int8(i), 0, &pvLine, nullMove, false, false, 0, rootIndex)
		timeSpent += time.Since(startTime).Milliseconds()

		if SearchState.ShouldStopRoot() {
			if len(prevPVLine.Moves) == 0 && len(pvLine.Moves) > 0 {
				bestScore = score
				SearchState.prevSearchScore = bestScore
				prevPVLine = pvLine.Clone()
			}
			break
		}

		if timeSpent == 0 {
			timeSpent = 1
		}
		nps := uint64(float64(SearchState.nodesChecked*1000) / float64(timeSpent))

		theMoves := getPVLineString(pvLine)

		if score <= alpha || score >= beta {
			aspCounter++
			// Immediately open to full window and retry
			alpha = -MaxScore
			beta = MaxScore
			i--
			continue
		}

		if (score > Checkmate || score < -Checkmate) && len(pvLine.Moves) > 0 {
			mateFound = true
		}

		alpha = score - aspirationWindowSize
		beta = score + aspirationWindowSize
		bestScore = score

		if len(pvLine.Moves) > 0 {
			SearchState.timeHandler.UpdateStability(int16(score), uint32(pvLine.Moves[0]))
		}

		if SearchState.timeHandler.ShouldExtendTime() {
			SearchState.timeHandler.ExtendTime()
		}

		SearchState.prevSearchScore = bestScore
		prevPVLine = pvLine.Clone()

		if printSearchInformation {
			fmt.Println(
				"info depth", i,
				"score", getMateOrCPScore(int(score)),
				"nodes", SearchState.nodesChecked,
				"time", timeSpent,
				"nps", nps,
				"pv", theMoves,
			)
		}

		if mateFound {
			break
		}
	}

	// Reset globals
	//SearchState.nodesChecked = 0
	SearchState.searchShouldStop = false
	SearchState.timeHandler.stopSearch = false

	SearchState.totalTimeSpent += timeSpent
	bestMove = prevPVLine.GetPVMove()

	return int(bestScore), bestMove
}

func alphabeta(b *gm.Board, alpha int32, beta int32, depth int8, ply int8, pvLine *PVLine, prevMove gm.Move, didNull bool, isExtended bool, excludedMove gm.Move, rootIndex int) int32 {
	SearchState.nodesChecked++

	if SearchState.nodesChecked&4095 == 0 {
		if SearchState.timeHandler.TimeStatus() {
			SearchState.searchShouldStop = true
		}
	}

	if ply >= MaxDepth {
		return Evaluation(b, false)
	}

	if SearchState.ShouldStopNoClock() {
		return 0
	}

	/* INIT KEY VARIABLES */
	var bestMove gm.Move
	var childPVLine = PVLine{}
	var isPVNode = (beta - alpha) > 1
	var isRoot = ply == 0

	if !isRoot {
		if SearchState.isDraw(int(ply), rootIndex) {
			return DrawScore
		}
		if alpha < DrawScore && SearchState.upcomingRepetition(int(ply), rootIndex) {
			alpha = DrawScore
		}
	}

	inCheck := b.OurKingInCheck()
	allMoves := b.GenerateLegalMoves()
	hasNoLegalMoves := len(allMoves) == 0

	// Draw or checkmate ...
	if !inCheck && hasNoLegalMoves {
		return DrawScore
	} else if inCheck && hasNoLegalMoves {
		return -MaxScore + int32(ply)
	}

	// Check extension
	if inCheck {
		depth++
	}

	if depth <= 0 {
		return quiescence(b, alpha, beta, pvLine, 30, ply, rootIndex)
	}

	posHash := b.Hash()

	/*
		====== TRANSPOSITION TABLE ======
		If we've searched this position before at equal or greater depth,
		we can use the stored score to either return immediately, or to
		improve move ordering by trying the previously best move first.
	*/
	ttEntry, ttHit := SearchState.tt.ProbeEntry(posHash)
	usable, ttScore := SearchState.tt.useEntry(ttEntry, posHash, depth, alpha, beta, ply, excludedMove)

	if usable && !isRoot && !isPVNode {
		SearchState.cutStats.TTCutoffs++
		return ttScore
	}

	var staticScore int32
	var ttMove gm.Move
	if ttHit {
		ttMove = ttEntry.Move
	}

	if usable {
		staticScore = ttEntry.Score
		bestMove = ttMove
	} else {
		staticScore = Evaluation(b, false)
	}

	// If a static evaluation shows us potentially improving alpha, we can prune more aggressively
	improving := false
	if ply >= 2 && !inCheck {
		improving = staticScore > alpha
	}

	/*
		====== STATIC NULL-MOVE PRUNING ======
		If the static evaluation minus a safety margin still beats beta,
		we can assume our position is so far above beta that we can prune this branch
	*/
	var wCount, bCount = hasMinorOrMajorPiece(b)
	var sideHasPieces = (b.Wtomove && wCount > 0) || (!b.Wtomove && bCount > 0)
	if !inCheck && !isPVNode && depth <= 7 && depth >= 1 && abs32(beta) < Checkmate && !isRoot {
		rfpMargin := RFPMargins[depth]
		if !improving {
			rfpMargin -= 50 // More aggressive when not improving
		}
		if staticScore-rfpMargin >= beta {
			SearchState.cutStats.StaticNullCutoffs++
			SearchState.tt.storeEntry(posHash, depth, ply, ttMove, staticScore-rfpMargin, BetaFlag)
			return staticScore - rfpMargin
		}
	}

	/*
		====== NULL-MOVE PRUNING ======
		If we give the opponent a free move, and we still raise beta even after
		giving our opponent the free move, we can prune this branch
	*/
	var margin int32 = Max32(0, 200-15*int32(depth)) // Margin to only look at positions already risking being beta nodes
	if !inCheck && !isPVNode && !didNull && sideHasPieces && depth >= NullMoveMinDepth && !isRoot && staticScore >= beta-margin {
		unApplyfunc := applyNullMoveWithState(b)

		var R = 3 + depth/4
		score := -alphabeta(b, -beta, -beta+1, depth-1-R, ply+1, &childPVLine, bestMove, true, isExtended, 0, rootIndex)
		unApplyfunc()

		if score >= beta && score < Checkmate {
			SearchState.tt.storeEntry(posHash, depth, ply, 0, score, BetaFlag)
			SearchState.cutStats.NullMoveCutoffs++
			return score
		}

	}
	/*
		====== Razoring ======
		If we're near the horizon and the static evaluation is far below alpha,
		the position is likely too bad for quiet moves to save it.
		We drop into qsearch to confirm, and if it still fails low, we return early.
	*/
	if depth <= 3 && !isPVNode && !inCheck && !isRoot {
		if staticScore+RazoringMargins[depth] < alpha {
			score := quiescence(b, alpha, beta, &childPVLine, 30, ply, rootIndex)
			if score < alpha {
				return score
			}
		}
	}

	/*
		====== SINGULAR EXTENSION ======
		If we have a TT move that appears singular (no other move comes close),
		extend its search depth.
	*/
	var singularExtension bool
	if !isPVNode && !isRoot && !inCheck && !didNull && !isExtended && depth >= 8 && ttMove != 0 && ttEntry.Flag == ExactFlag && ttEntry.Depth >= depth-3 {
		ttValue := ttEntry.Score
		if ttValue < Checkmate && ttValue > -Checkmate {
			margin := int32(50 + 10*depth)
			scoreToBeat := ttValue - margin
			R := int8(3) + depth/4
			if R > depth-1 {
				R = depth - 1
			}
			var verificationPV PVLine
			scoreSingular := alphabeta(b, scoreToBeat-1, scoreToBeat, depth-1-R, ply, &verificationPV, prevMove, didNull, true, ttMove, rootIndex)
			if scoreSingular < scoreToBeat {
				singularExtension = true
			}
		}
	}

	/*
		====== PROBCUT ======
		We test if a shallow search at an elevated beta (beta + margin) still fails high.
		We test with qsearch, then confirm with a reduced search that it still elevates beta.
		If both searches beat the elevated beta, the position is likely to fail high and we cut early.
	*/
	if !inCheck && !isPVNode && depth >= 5 && abs32(alpha) < Checkmate {
		probCutBeta := beta + 200

		captures := b.GenerateCaptures()
		scoredCaptures, hasCaptures := scoreMovesListCaptures(captures, ply)
		if hasCaptures {
			maxProbCutCaptures := Min(10, len(scoredCaptures.moves)) // TEST; most likely we're

			for i := uint8(0); i < uint8(maxProbCutCaptures); i++ {
				orderNextMove(i, &scoredCaptures)
				move := scoredCaptures.moves[i].move

				if see(b, move, false) < -ProbCutSeeMargin {
					continue
				}

				unapplyFunc := applyMoveWithState(b, move)

				qScore := -quiescence(b, -probCutBeta, -probCutBeta+1, &childPVLine, 10, ply+1, rootIndex)

				if qScore >= probCutBeta {
					score := -alphabeta(b, -probCutBeta, -probCutBeta+1, depth-4, ply+1, &childPVLine, prevMove, didNull, isExtended, excludedMove, rootIndex)

					if score >= probCutBeta {
						unapplyFunc()
						SearchState.tt.storeEntry(posHash, depth, ply, move, score, BetaFlag)
						return score
					}
				}
				unapplyFunc()
			}
		}
	}

	/*
	   ====== INTERNAL ITERATIVE DEEPENING ======
	   When we have no TT move at sufficient depth, do a reduced search to find one.
	   This is much better than searching blind.
	*/
	if ttMove == 0 && depth >= 5 && !didNull && !isExtended {
		reducedDepth := depth - 2
		if depth >= 8 {
			reducedDepth = depth - depth/4
		}

		var iidPV PVLine
		alphabeta(b, alpha, beta, reducedDepth, ply, &iidPV, prevMove, false, true, 0, rootIndex)

		iidEntry, _ := SearchState.tt.ProbeEntry(posHash)
		if iidEntry.Move != 0 {
			ttMove = iidEntry.Move
			bestMove = ttMove
		}
	}

	// Checkmate/stalemate check
	if len(allMoves) == 0 {
		if inCheck {
			return -MaxScore + int32(ply) // Checkmate
		}
		return DrawScore // Stalemate
	}

	var score int32 = -MaxScore
	var bestScore int32 = -MaxScore
	var moveList = scoreMovesList(b, allMoves, depth, ply, bestMove, prevMove)
	var ttFlag int8 = AlphaFlag
	legalMoves := 0

	quietMovesTried := make([]gm.Move, 0, 16)

	for index := uint8(0); index < uint8(len(moveList.moves)); index++ {
		orderNextMove(index, &moveList)
		move := moveList.moves[index].move

		if move == excludedMove {
			continue
		}

		sideIdx := 0
		if !b.Wtomove {
			sideIdx = 1
		}

		isCapture := gm.IsCapture(move, b)
		moveGivesCheck := b.GivesCheck(move)
		isPromotion := move.PromotionPieceType() != gm.PieceTypeNone

		// Tactical = capture, check, or promotion
		tactical := isCapture || moveGivesCheck || isPromotion

		/*
			====== LATE MOVE PRUNING ======
			Skip quiet moves late in the move list at low depths.
		*/
		if depth <= 8 && !isPVNode && !tactical && !isRoot && legalMoves > 1 {
			lmpMargin := LateMovePruningMargins[Min(int(depth), len(LateMovePruningMargins)-1)]
			if !improving {
				lmpMargin = lmpMargin * 2 / 3
			}
			if lmpMargin > 0 && legalMoves > lmpMargin {
				SearchState.cutStats.LateMovePrunes++
				continue
			}
		}
		if moveGivesCheck {
			tactical = true
		}

		/*
			====== FUTILITY PRUNING ======
			If we're near the horizon and the static evaluation plus a margin can't beat alpha,
			quiet moves are unlikely to raise the score.
			We skip all quiet moves, assuming only a capture could help
		*/
		if depth <= 7 && depth >= 1 && !moveGivesCheck && !isPVNode && !isRoot && !tactical && abs32(alpha) < Checkmate {
			futilityMargin := FutilityMargins[depth]
			if !improving {
				futilityMargin -= 50
			}
			if staticScore+futilityMargin <= alpha {
				SearchState.cutStats.FutilityPrunes++
				continue
			}
		}

		if !isCapture {
			quietMovesTried = append(quietMovesTried, move)
		}

		var unapplyFunc = applyMoveWithState(b, move)

		/*
			====== LATE MOVE REDUCTION ======
			Moves searched later in the move list are less likely to be good; we try searching these moves
			at reduced depth, and only if they beat alpha, we re-search at full depth to verify
		*/
		extendMove := !isExtended && move == ttMove && singularExtension
		nextExtended := isExtended || extendMove

		legalMoves++
		if legalMoves == 1 {
			// First move: search with full window, no reduction
			nextDepth := calculateSearchDepth(depth-1, 0, extendMove)
			score = -alphabeta(b, -beta, -alpha, nextDepth, ply+1, &childPVLine, move, false, nextExtended, 0, rootIndex)
		} else {
			moveHistoryScore := SearchState.historyMoves[sideIdx][move.From()][move.To()]

			var reduct int8 = 0
			if depth >= LMRDepthLimit && legalMoves >= LMRMoveLimit && !moveGivesCheck && !tactical {
				reduct = computeLMRReduction(
					depth, legalMoves, int(index), isPVNode, tactical,
					moveHistoryScore, improving,
					IsKiller(move, ply, &SearchState.killer), extendMove,
				)
			}

			// Stage 1: Search with (possibly reduced) depth using null window
			nextDepth := calculateSearchDepth(depth-1, reduct, extendMove)
			score = -alphabeta(b, -(alpha + 1), -alpha, nextDepth, ply+1, &childPVLine, move, false, nextExtended, 0, rootIndex)

			// Stage 2: If we had a reduction and score beats alpha, re-search at full depth with null window
			if score > alpha && reduct > 0 {
				nextDepth = calculateSearchDepth(depth-1, 0, extendMove)
				score = -alphabeta(b, -(alpha + 1), -alpha, nextDepth, ply+1, &childPVLine, move, false, nextExtended, 0, rootIndex)
			}

			// Stage 3: If score is within window (alpha, beta), do full window search
			if score > alpha && score < beta {
				nextDepth = calculateSearchDepth(depth-1, 0, extendMove)
				score = -alphabeta(b, -beta, -alpha, nextDepth, ply+1, &childPVLine, move, false, nextExtended, 0, rootIndex)
			}
		}

		unapplyFunc()

		if score > bestScore {
			bestScore = score
			bestMove = move
		}

		if score >= beta {
			SearchState.cutStats.BetaCutoffs++
			ttFlag = BetaFlag
			if !isCapture {
				InsertKiller(move, ply, &SearchState.killer)
				storeCounter(b.Wtomove, prevMove, move)

				incrementHistoryScore(b.Wtomove, move, depth)

				for _, failedMove := range quietMovesTried {
					if failedMove != move {
						decrementHistoryScoreBy(b.Wtomove, failedMove, depth)
					}
				}
			}
			break
		}

		if score > alpha {
			alpha = score
			ttFlag = ExactFlag
			pvLine.Update(move, childPVLine)

			if !isCapture {
				incrementHistoryScore(b.Wtomove, move, depth)
			}
		}
		childPVLine.Clear()
	}

	if !SearchState.ShouldStopNoClock() {
		SearchState.tt.storeEntry(posHash, depth, ply, bestMove, bestScore, ttFlag)
	}

	return bestScore
}

func quiescence(b *gm.Board, alpha int32, beta int32, pvLine *PVLine, depth int8, ply int8, rootIndex int) int32 {
	pvLine.Clear()
	SearchState.nodesChecked++

	if SearchState.nodesChecked&2047 == 0 {
		if SearchState.timeHandler.TimeStatus() {
			SearchState.searchShouldStop = true
		}
	}

	if SearchState.ShouldStopNoClock() {
		return 0
	}

	inCheck := b.OurKingInCheck()
	var childPVLine = PVLine{}

	var standpat int32 = Evaluation(b, false)

	// Stand-pat pruning (not when in check)
	if !inCheck {
		if standpat >= beta {
			SearchState.cutStats.QStandPatCutoffs++
			return standpat
		}
		if standpat > alpha {
			alpha = standpat
		}
	}

	var bestScore int32 = standpat

	// Generate moves: all moves when in check, only captures otherwise
	var moveList moveList
	if inCheck {
		moveList = scoreMovesList(b, b.GenerateLegalMoves(), 0, ply, gm.Move(0), gm.Move(0))
	} else {
		moveList, _ = scoreMovesListCaptures(b.GenerateCaptures(), ply)
	}

	movesSearched := 0

	for index := uint8(0); index < uint8(len(moveList.moves)); index++ {
		orderNextMove(index, &moveList)
		move := moveList.moves[index].move

		/*
			====== DELTA PRUNING ======
			If the capture + a margin still can't beat alpha, skip it.
			Only apply when not in check.
		*/
		if !inCheck {
			// SEE pruning first
			seeScore := see(b, move, false)
			if seeScore < -QuiescenceSeeMargin {
				continue
			}

			capturedPiece := move.CapturedPiece()
			moveGain := int32(0)
			if capturedPiece != gm.NoPiece {
				moveGain = int32(pieceValueMG[capturedPiece.Type()])
			}

			// Add promotion value if applicable
			if move.PromotionPieceType() != gm.PieceTypeNone {
				moveGain += int32(pieceValueMG[move.PromotionPieceType()] - pieceValueMG[gm.PieceTypePawn])
			}

			// If even with the capture we can't beat alpha, skip
			if standpat+moveGain+DeltaMargin < alpha {
				continue
			}
		}

		unapplyFunc := applyMoveWithState(b, move)
		movesSearched++

		score := -quiescence(b, -beta, -alpha, &childPVLine, depth-1, ply+1, rootIndex)
		unapplyFunc()

		if score > bestScore {
			bestScore = score
		}

		if score >= beta {
			SearchState.cutStats.QBetaCutoffs++
			return score // Return score, not beta (more accurate)
		}

		if score > alpha {
			alpha = score
			pvLine.Update(move, childPVLine)
		}
		childPVLine.Clear()
	}

	return bestScore
}

// calculateSearchDepth computes the search depth for a move, accounting for reductions and extensions
func calculateSearchDepth(baseDepth int8, reduction int8, extendMove bool) int8 {
	depth := baseDepth - reduction
	if extendMove && reduction == 0 {
		depth++
	}
	return depth
}

func applyMoveWithState(b *gm.Board, move gm.Move) func() {
	unapply := b.Apply(move)
	SearchState.pushState(b)
	return func() {
		unapply()
		SearchState.popState()
	}
}

func applyNullMoveWithState(b *gm.Board) func() {
	unapply := b.ApplyNullMove()
	SearchState.pushState(b)
	return func() {
		unapply()
		SearchState.popState()
	}
}
