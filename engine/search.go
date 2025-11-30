package engine

import (
	"fmt"
	"time"

	gm "chess-engine/goosemg"
)

var MaxScore int16 = 32500
var Checkmate int16 = 20000

const DrawScore int16 = 0

var killerMoveTable KillerStruct

var SearchTime time.Duration
var searchShouldStop bool

// OPTIMIZATION 1: Extended futility margins for depths 1-7
// These margins represent how much material deficit we're willing to accept
// before assuming we can't raise alpha with quiet moves
var FutilityMargins = [8]int16{
	0,   // depth 0 (not used)
	120, // depth 1 - ~minor piece
	220, // depth 2
	320, // depth 3
	420, // depth 4
	520, // depth 5
	620, // depth 6
	720, // depth 7
}

// OPTIMIZATION 2: Reverse futility pruning margins (static null move pruning)
// If static eval exceeds beta by this margin, we can prune
var RFPMargins = [8]int16{
	0,   // depth 0
	100, // depth 1
	200, // depth 2
	300, // depth 3
	400, // depth 4
	500, // depth 5
	600, // depth 6
	700, // depth 7
}

// Razoring margins (currently disabled, but tuned if you want to enable)
var RazoringMargins = [4]int16{
	0,   // depth 0
	125, // depth 1
	225, // depth 2
	325, // depth 3
}

// OPTIMIZATION 3: More aggressive late move pruning margins
// Format: at depth N, prune moves after index LateMovePruningMargins[N]
// These are Stockfish-inspired values
var LateMovePruningMargins = [9]int{
	0,  // depth 0 (not used)
	3,  // depth 1 - after 3 moves
	5,  // depth 2 - after 5 moves
	9,  // depth 3
	14, // depth 4
	20, // depth 5
	27, // depth 6
	35, // depth 7
	44, // depth 8
}

// LMR parameters - relaxed to allow more reductions
var LMRDepthLimit int8 = 2 // Start LMR from depth 2 (was 3)
var LMRMoveLimit = 2       // Start reducing from move 2 (was 4)
var LMRHistoryBonus = 500  // Good history reduces reduction
var LMRHistoryMalus = -100 // Bad history increases reduction

// Null move parameters
var NullMoveMinDepth int8 = 2 // Reduced from 3

// OPTIMIZATION 4: Delta pruning margin for quiescence
var DeltaMargin int16 = 200 // Approximately a pawn

// SEE pruning threshold for main search
var SEEPruneDepth int8 = 8
var SEEPruneMargin = -20 // Per depth unit

var QuiescenceSeeMargin int = 100

// Aspiration window
var aspirationWindowSize int16 = 35

var TT TransTable
var prevSearchScore int16 = 0
var timeHandler TimeHandler
var GlobalStop = false

func StartSearch(board *gm.Board, depth uint8, gameTime int, increment int, useCustomDepth bool, evalOnly bool, moveOrderingOnly bool) string {
	initVariables(board)
	resetCutStats()

	ensureStateStackSynced(board)
	TT.NewSearch()

	if !TT.isInitialized {
		TT.init()
	}

	GlobalStop = false
	timeHandler.initTimemanagement(gameTime, increment, board.FullmoveNumber(), useCustomDepth)
	timeHandler.StartTime(board.FullmoveNumber())

	var bestMove gm.Move

	if evalOnly {
		Evaluation(board, true, false)
		println("Is this a theoretical draw: ", isTheoreticalDraw(board, true))
		return ""
	}

	if moveOrderingOnly {
		dumpRootMoveOrdering(board)
		return ""
	}

	_, bestMove = rootsearch(board, depth, useCustomDepth)

	if PrintCutStats {
		dumpCutStats()
		PrintCutStats = false
	}

	return bestMove.String()
}

func rootsearch(b *gm.Board, depth uint8, useCustomDepth bool) (int, gm.Move) {
	var timeSpent int64
	var alpha int16 = -MaxScore
	var beta int16 = MaxScore
	var bestScore = -MaxScore
	rootIndex := len(stateStack) - 1

	if prevSearchScore != 0 {
		alpha = prevSearchScore - aspirationWindowSize
		beta = prevSearchScore + aspirationWindowSize
	}

	var nullMove gm.Move
	var bestMove gm.Move
	var pvLine PVLine
	var prevPVLine PVLine
	var mateFound bool

	var currentWindow = aspirationWindowSize
	for i := uint8(1); i <= depth; i++ {
		pvLine.Clear()
		mateFound = false

		var startTime = time.Now()
		var score = alphabeta(b, alpha, beta, int8(i), 0, &pvLine, nullMove, false, false, 0, rootIndex)
		timeSpent += time.Since(startTime).Milliseconds()

		if searchShouldStop || timeHandler.TimeStatus() || timeHandler.stopSearch || GlobalStop {
			break
		}

		if timeSpent == 0 {
			timeSpent = 1
		}
		nps := uint64(float64(nodesChecked*1000) / float64(timeSpent))

		var theMoves = getPVLineString(pvLine)

		// Aspiration window re-search
		if score <= alpha || score >= beta {
			currentWindow *= 2
			if currentWindow >= MaxScore {
				currentWindow = MaxScore
			}
			alpha = score - currentWindow
			beta = score + currentWindow
			if alpha < -MaxScore {
				alpha = -MaxScore
			}
			if beta > MaxScore {
				beta = MaxScore
			}
			i--
			continue
		}

		if (score > Checkmate || score < -Checkmate) && len(pvLine.Moves) > 0 {
			mateFound = true
		}

		alpha = score - aspirationWindowSize
		beta = score + aspirationWindowSize
		bestScore = score

		currentWindow = aspirationWindowSize

		prevSearchScore = bestScore
		prevPVLine = pvLine.Clone()

		fmt.Println("info depth", i, "score", getMateOrCPScore(int(score)), "nodes", nodesChecked, "time", timeSpent, "nps", nps, "pv", theMoves)
		if mateFound {
			break
		}
	}

	nodesChecked = 0
	searchShouldStop = false
	timeHandler.stopSearch = false

	bestMove = prevPVLine.GetPVMove()

	return int(bestScore), bestMove
}

func alphabeta(b *gm.Board, alpha int16, beta int16, depth int8, ply int8, pvLine *PVLine, prevMove gm.Move, didNull bool, isExtended bool, excludedMove gm.Move, rootIndex int) int16 {
	nodesChecked++

	if nodesChecked&4095 == 0 {
		if timeHandler.TimeStatus() {
			searchShouldStop = true
		}
	}

	if GlobalStop || searchShouldStop {
		return 0
	}

	/* INIT KEY VARIABLES */
	var bestMove gm.Move
	var childPVLine = PVLine{}
	var isPVNode = (beta - alpha) > 1
	var isRoot = ply == 0

	// Draw detection
	if !isRoot {
		if isDraw(int(ply), rootIndex) {
			return DrawScore
		}
		if alpha < DrawScore && upcomingRepetition(int(ply), rootIndex) {
			alpha = DrawScore
		}
	}

	inCheck := b.OurKingInCheck()

	// Check extension
	if inCheck {
		depth++
	}

	if !inCheck && !b.HasLegalMoves() {
		return DrawScore
	}

	// Quiescence at leaf nodes
	if depth <= 0 {
		return quiescence(b, alpha, beta, &childPVLine, 30, ply, rootIndex)
	}

	posHash := b.Hash()

	/*
		TRANSPOSITION TABLE LOOKUP
	*/
	ttEntry, ttHit := TT.ProbeEntry(posHash)
	usable, ttScore := TT.useEntry(ttEntry, posHash, depth, alpha, beta, ply, excludedMove)
	if usable && !isRoot && !isPVNode {
		cutStats.TTCutoffs++
		return ttScore
	}

	// Only use TT move if we actually found a matching entry
	var ttMove gm.Move
	if ttHit {
		ttMove = ttEntry.Move
	}
	if usable {
		bestMove = ttMove
	}

	// Static evaluation - needed for various pruning techniques
	var staticScore int16 = Evaluation(b, false, false)

	// Improving flag: are we doing better than 2 plies ago?
	// This is used to be more aggressive with pruning when not improving
	improving := false
	if ply >= 2 && !inCheck {
		// Simple heuristic: compare to previous static eval if available
		// For now, assume improving if static score > alpha
		improving = staticScore > alpha
	}

	var wCount, bCount = hasMinorOrMajorPiece(b)
	var sideHasPieces = (b.Wtomove && wCount > 0) || (!b.Wtomove && bCount > 0)

	/*
		If our position is so good that even after giving a margin to the opponent,
		we still beat beta, we can safely prune.
		Applied at depths 1-7, NOT in PV nodes or when in check.
	*/
	if !inCheck && !isPVNode && depth <= 7 && depth >= 1 && absInt16(beta) < Checkmate && !isRoot {
		rfpMargin := RFPMargins[depth]
		if !improving {
			rfpMargin -= 50 // More aggressive when not improving
		}
		if staticScore-rfpMargin >= beta {
			cutStats.StaticNullCutoffs++
			return staticScore - rfpMargin
		}
	}

	/*
		NULL MOVE PRUNING
	*/
	if !inCheck && !isPVNode && !didNull && sideHasPieces && depth >= NullMoveMinDepth && !isRoot {
		unApplyfunc := applyNullMoveWithState(b)

		// More aggressive reduction: R = 3 + depth/3, with bonus for high depth
		var R int8 = 3 + depth/3
		if depth > 6 {
			R++
		}
		// Ensure we don't reduce below depth 1
		if R > depth-1 {
			R = depth - 1
		}

		score := -alphabeta(b, -beta, -beta+1, depth-1-R, ply+1, &childPVLine, bestMove, true, isExtended, 0, rootIndex)
		unApplyfunc()

		if score >= beta && score < Checkmate {
			cutStats.NullMoveCutoffs++
			// Verification search at high depths (optional, adds safety)
			if depth > 10 {
				verifyScore := alphabeta(b, beta-1, beta, depth-1-R, ply, &childPVLine, prevMove, true, isExtended, 0, rootIndex)
				if verifyScore >= beta {
					return beta
				}
			} else {
				return beta
			}
		}
	}

	/*
		SINGULAR EXTENSION
		If we have a TT move that appears singular (no other move comes close),
		extend its search depth.
	*/
	var singularExtension bool
	if !isPVNode && !isRoot && !inCheck && !didNull && !isExtended && depth >= 6 && ttMove != 0 && ttEntry.Flag == ExactFlag && ttEntry.Depth >= depth-3 {
		ttValue := ttEntry.Score
		if ttValue < Checkmate && ttValue > -Checkmate {
			margin := int16(50 + 10*depth)
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
		INTERNAL ITERATIVE REDUCTIONS
		Reduce depth when we have no TT move
	*/
	if ttMove == 0 && depth >= 4 && isPVNode {
		depth--
	}

	// Generate and score moves
	allMoves := b.GenerateLegalMoves()

	// Checkmate/stalemate check
	if len(allMoves) == 0 {
		if inCheck {
			return -MaxScore + int16(ply) // Checkmate
		}
		return DrawScore // Stalemate
	}

	var score int16 = -MaxScore
	var bestScore int16 = -MaxScore
	var moveList = scoreMovesList(b, allMoves, depth, ply, bestMove, prevMove)
	var ttFlag int8 = AlphaFlag
	legalMoves := 0
	bestMove = 0

	// Track quiet moves tried for history malus
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

		legalMoves++

		isCapture := gm.IsCapture(move, b)
		givesCheck := b.GivesCheck(move) // Assuming this method exists; if not, check after apply
		isPromotion := move.PromotionPieceType() != gm.PieceTypeNone

		// Tactical = capture, check, or promotion
		tactical := isCapture || givesCheck || isPromotion

		/*
			########################################################
			LATE MOVE PRUNING:
			Skip quiet moves late in the move list at low depths.
			########################################################
		*/
		if depth <= 8 && !isPVNode && !tactical && !isRoot && legalMoves > 1 {
			lmpMargin := LateMovePruningMargins[Min(int(depth), len(LateMovePruningMargins)-1)]
			// Be more aggressive when not improving
			if !improving {
				lmpMargin = lmpMargin * 2 / 3
			}
			if lmpMargin > 0 && legalMoves > lmpMargin {
				cutStats.LateMovePrunes++
				continue
			}
		}

		// Apply the move
		var unapplyFunc = applyMoveWithState(b, move)
		var afterMoveInCheck = b.OurKingInCheck()

		// Update tactical flag with actual check detection
		if afterMoveInCheck {
			tactical = true
		}

		/*
			At depths 1-7, if static eval + margin can't beat alpha, prune quiet moves.
		*/
		if depth <= 7 && depth >= 1 && !afterMoveInCheck && !isPVNode && !isRoot && !tactical && absInt16(alpha) < Checkmate {
			futilityMargin := FutilityMargins[depth]
			if !improving {
				futilityMargin -= 50 // More aggressive when not improving
			}
			if staticScore+futilityMargin <= alpha {
				cutStats.FutilityPrunes++
				unapplyFunc()
				continue
			}
		}

		// Track quiet moves for history malus
		if !isCapture {
			quietMovesTried = append(quietMovesTried, move)
		}

		/*
			LATE MOVE REDUCTIONS
		*/
		extendMove := !isExtended && move == ttMove && singularExtension
		nextExtended := isExtended || extendMove

		if legalMoves == 1 {
			// First move: full-depth, full-window search
			nextDepth := depth - 1
			if extendMove {
				nextDepth++
			}
			score = -alphabeta(b, -beta, -alpha, nextDepth, ply+1, &childPVLine, move, false, nextExtended, 0, rootIndex)
		} else {
			moveHistoryScore := historyMove[sideIdx][move.From()][move.To()]

			// OPTIMIZATION: Relaxed LMR conditions
			useLMR := depth >= LMRDepthLimit && legalMoves >= LMRMoveLimit && !afterMoveInCheck && !tactical

			var reduct int8 = 0
			if useLMR {
				// Base reduction from LMR table
				reduct = computeLMRReduction(depth, legalMoves, int(index), isPVNode, tactical, moveHistoryScore)

				// History adjustments
				if moveHistoryScore > LMRHistoryBonus {
					reduct--
				}
				if moveHistoryScore > LMRHistoryBonus*2 {
					reduct--
				}
				if moveHistoryScore < LMRHistoryMalus {
					reduct++
				}

				// PV nodes get less reduction
				if isPVNode && reduct > 0 {
					reduct--
				}

				// Don't reduce improving positions as much
				if improving && reduct > 1 {
					reduct--
				}

				// Killers get less reduction
				if IsKiller(move, ply, &killerMoveTable) && reduct > 0 {
					reduct--
				}

				// Clamp reduction
				if reduct < 0 {
					reduct = 0
				}
				if reduct > depth-2 {
					reduct = depth - 2
				}

				// Singular extension moves shouldn't be reduced as much
				if extendMove && reduct > 0 {
					reduct--
				}
			}

			nextDepth := depth - 1 - reduct
			if extendMove && reduct == 0 {
				nextDepth++
			}

			// Null-window search (possibly with reduction)
			score = -alphabeta(b, -(alpha + 1), -alpha, nextDepth, ply+1, &childPVLine, move, false, nextExtended, 0, rootIndex)

			// Re-search if we beat alpha
			if score > alpha {
				if reduct > 0 {
					// Full-depth null-window re-search
					nextDepth = depth - 1
					if extendMove {
						nextDepth++
					}
					score = -alphabeta(b, -(alpha + 1), -alpha, nextDepth, ply+1, &childPVLine, move, false, nextExtended, 0, rootIndex)
				}

				// Full-window re-search if still within bounds
				if score > alpha && score < beta {
					nextDepth = depth - 1
					if extendMove {
						nextDepth++
					}
					score = -alphabeta(b, -beta, -alpha, nextDepth, ply+1, &childPVLine, move, false, nextExtended, 0, rootIndex)
				}
			}
		}

		unapplyFunc()

		// Update best score and move
		if score > bestScore {
			bestScore = score
			bestMove = move
		}

		// Beta cutoff
		if score >= beta {
			cutStats.BetaCutoffs++
			ttFlag = BetaFlag

			if !isCapture {
				// Store killer and counter moves
				InsertKiller(move, ply, &killerMoveTable)
				storeCounter(!b.Wtomove, prevMove, move)

				// History bonus for the good move
				incrementHistoryScore(!b.Wtomove, move, depth)

				// History malus for all quiet moves that didn't work
				for _, failedMove := range quietMovesTried {
					if failedMove != move {
						decrementHistoryScoreBy(!b.Wtomove, failedMove, depth)
					}
				}
			}
			break
		}

		// Alpha improvement
		if score > alpha {
			alpha = score
			ttFlag = ExactFlag
			pvLine.Update(move, childPVLine)

			if !isCapture {
				incrementHistoryScore(!b.Wtomove, move, depth)
			}
		}
	}

	childPVLine.Clear()

	// Store in transposition table
	if !timeHandler.stopSearch && !GlobalStop && !searchShouldStop && bestMove != 0 {
		TT.storeEntry(posHash, depth, ply, bestMove, bestScore, ttFlag)
	}

	return bestScore
}

func quiescence(b *gm.Board, alpha int16, beta int16, pvLine *PVLine, depth int8, ply int8, rootIndex int) int16 {
	nodesChecked++

	if nodesChecked&2047 == 0 {
		if timeHandler.TimeStatus() {
			searchShouldStop = true
		}
	}

	if GlobalStop || searchShouldStop {
		return 0
	}

	inCheck := b.OurKingInCheck()
	var childPVLine = PVLine{}

	var standpat int16 = Evaluation(b, false, false)

	// Check extension in qsearch
	if inCheck {
		depth++
	}

	// Stand-pat pruning (not when in check)
	if !inCheck {
		if standpat >= beta {
			cutStats.QStandPatCutoffs++
			return standpat
		}
		if standpat > alpha {
			alpha = standpat
		}
	}

	var bestScore int16
	if inCheck {
		bestScore = -MaxScore // Must escape check
	} else {
		bestScore = standpat
	}

	// Generate moves: all moves when in check, only captures otherwise
	var moveList moveList
	if inCheck {
		moveList = scoreMovesList(b, b.GenerateLegalMoves(), 0, ply, gm.Move(0), gm.Move(0))
	} else {
		moveList, _ = scoreMovesListCaptures(b, b.GenerateCaptures(), ply)
	}

	movesSearched := 0

	for index := uint8(0); index < uint8(len(moveList.moves)); index++ {
		orderNextMove(index, &moveList)
		move := moveList.moves[index].move

		/*
			OPTIMIZATION 4: DELTA PRUNING
			If the capture + a margin still can't beat alpha, skip it.
			Only apply when not in check.
		*/
		if !inCheck {
			// SEE pruning first
			seeScore := see(b, move, false)
			if seeScore < -QuiescenceSeeMargin {
				continue
			}

			// Delta pruning: estimate maximum gain from this capture
			capturedPiece := move.CapturedPiece()
			moveGain := int16(0)
			if capturedPiece != gm.NoPiece {
				moveGain = int16(pieceValueMG[capturedPiece.Type()])
			}

			// Add promotion value if applicable
			if move.PromotionPieceType() != gm.PieceTypeNone {
				moveGain += int16(pieceValueMG[move.PromotionPieceType()] - pieceValueMG[gm.PieceTypePawn])
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
			cutStats.QBetaCutoffs++
			return score // Return score, not beta (more accurate)
		}

		if score > alpha {
			alpha = score
			pvLine.Update(move, childPVLine)
		}
		childPVLine.Clear()
	}

	// If in check and no moves, it's checkmate
	//if inCheck && movesSearched == 0 {
	//	return -MaxScore + int16(ply)
	//}

	return bestScore
}

func applyMoveWithState(b *gm.Board, move gm.Move) func() {
	unapply := b.Apply(move)
	pushState(b)
	return func() {
		unapply()
		popState()
	}
}

func applyNullMoveWithState(b *gm.Board) func() {
	unapply := b.ApplyNullMove()
	pushState(b)
	return func() {
		unapply()
		popState()
	}
}
