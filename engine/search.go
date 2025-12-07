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

var KillerMoveTable KillerStruct

var ttMoveAvailable uint64
var ttMoveNotAvailable uint64

var SearchTime time.Duration
var searchShouldStop bool

// =============================================================================
// MARGINS
// =============================================================================
var FutilityMargins = [8]int32{0, 120, 220, 320, 420, 520, 620, 720}
var RFPMargins = [8]int32{0, 100, 200, 300, 400, 500, 600, 700}
var RazoringMargins = [4]int32{0, 125, 225, 325}

var LateMovePruningMargins = [9]int{0, 3, 5, 9, 14, 20, 27, 35, 44}

// =============================================================================
// LMR/PRUNING PARAMETERS - int8 is fine for depth-related values
// =============================================================================
var LMRDepthLimit int8 = 2
var LMRMoveLimit = 2
var LMRHistoryBonus = 500
var LMRHistoryMalus = -100
var NullMoveMinDepth int8 = 2
var SEEPruneDepth int8 = 8
var SEEPruneMargin = -20
var QuiescenceSeeMargin int = 100

// Score-related - use int32
var DeltaMargin int32 = 200
var aspirationWindowSize int32 = 35
var prevSearchScore int32 = 0

var TT TransTable
var timeHandler TimeHandler
var GlobalStop = false

func StartSearch(board *gm.Board, depth uint8, gameTime int, increment int, useCustomDepth bool, evalOnly bool, moveOrderingOnly bool) string {
	initVariables(board)

	//Stat reset
	ensureStateStackSynced(board)

	if !TT.isInitialized {
		TT.init()
	}

	GlobalStop = false
	timeHandler.initTimemanagement(gameTime, increment, board.FullmoveNumber(), useCustomDepth)
	timeHandler.StartTime(board.FullmoveNumber())

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

	_, bestMove = rootsearch(board, depth, useCustomDepth)

	if PrintCutStats {
		dumpCutStats()
		PrintCutStats = false
	}

	return bestMove.String()
}

func rootsearch(b *gm.Board, depth uint8, useCustomDepth bool) (int, gm.Move) {
	var timeSpent int64
	var alpha int32 = -MaxScore
	var beta int32 = MaxScore
	var bestScore int32 = -MaxScore
	rootIndex := len(stateStack) - 1

	// Use previous search score as center of aspiration window if available
	if prevSearchScore != 0 {
		alpha = prevSearchScore - aspirationWindowSize
		beta = prevSearchScore + aspirationWindowSize
	}

	var nullMove gm.Move
	var bestMove gm.Move
	var pvLine PVLine
	var prevPVLine PVLine
	var mateFound bool

	currentWindow := aspirationWindowSize

	for i := uint8(1); i <= depth; i++ {
		if !useCustomDepth && i > 1 {
			if timeHandler.SoftTimeExceeded() && !timeHandler.ShouldExtendTime() {
				break
			}
			if timeHandler.ShouldStopEarly() {
				break
			}
		}

		pvLine.Clear()
		mateFound = false

		startTime := time.Now()
		score := alphabeta(b, alpha, beta, int8(i), 0, &pvLine, nullMove, false, false, 0, rootIndex)
		timeSpent += time.Since(startTime).Milliseconds()

		if searchShouldStop || timeHandler.TimeStatus() || timeHandler.stopSearch || GlobalStop {
			if len(prevPVLine.Moves) == 0 && len(pvLine.Moves) > 0 {
				bestScore = score
				prevSearchScore = bestScore
				prevPVLine = pvLine.Clone()
			}
			break
		}

		if timeSpent == 0 {
			timeSpent = 1
		}
		nps := uint64(float64(nodesChecked*1000) / float64(timeSpent))

		theMoves := getPVLineString(pvLine)

		// Aspiration window re-search
		if score <= alpha || score >= beta {
			if alpha <= -MaxScore && beta >= MaxScore {
				currentWindow *= 2
			} else {
				if currentWindow >= int32(MaxScore) {
					currentWindow = int32(MaxScore)
				} else {
					currentWindow *= 2
				}
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

		if (score > Checkmate || score < -Checkmate) && len(pvLine.Moves) > 0 { // If we found checkmate...
			mateFound = true
		}

		alpha = score - aspirationWindowSize
		beta = score + aspirationWindowSize
		bestScore = score

		// Update score tracker
		if len(pvLine.Moves) > 0 {
			timeHandler.UpdateStability(int16(score), uint32(pvLine.Moves[0]))
		}

		// UNstable score requires more time usage
		if timeHandler.ShouldExtendTime() {
			timeHandler.ExtendTime()
		}

		currentWindow = int32(aspirationWindowSize)

		prevSearchScore = bestScore
		prevPVLine = pvLine.Clone()

		fmt.Println(
			"info depth", i,
			"score", getMateOrCPScore(int(score)),
			"nodes", nodesChecked,
			"time", timeSpent,
			"nps", nps,
			"pv", theMoves,
		)

		if mateFound {
			break
		}
	}

	// Reset per-search globals
	nodesChecked = 0
	searchShouldStop = false
	timeHandler.stopSearch = false

	// Get the best move from the last stable PV
	bestMove = prevPVLine.GetPVMove()

	// Emergency fallback: never return an empty move
	//if bestMove == 0 {
	//	moves := b.GenerateLegalMoves()
	//	if len(moves) > 0 {
	//		println("OH MY GOD, EMERGENCY FALLBACK")
	//		bestMove = moves[0]
	//	}
	//}

	return int(bestScore), bestMove
}

func alphabeta(b *gm.Board, alpha int32, beta int32, depth int8, ply int8, pvLine *PVLine, prevMove gm.Move, didNull bool, isExtended bool, excludedMove gm.Move, rootIndex int) int32 {
	nodesChecked++

	if nodesChecked&4095 == 0 {
		if timeHandler.TimeStatus() {
			searchShouldStop = true
		}
	}

	if ply >= MaxDepth {
		return Evaluation(b, false)
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
	if ttEntry.Move != 0 {
		ttMoveAvailable++
	} else {
		ttMoveNotAvailable++
	}
	usable, ttScore := TT.useEntry(ttEntry, posHash, depth, alpha, beta, ply, excludedMove)

	if usable && !isRoot && !isPVNode {
		cutStats.TTCutoffs++
		return ttScore
	}

	var staticScore int32
	// Only use TT move if we actually found a matching entry
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

	improving := false
	if ply >= 2 && !inCheck {
		improving = staticScore > alpha
	}

	var wCount, bCount = hasMinorOrMajorPiece(b)
	var sideHasPieces = (b.Wtomove && wCount > 0) || (!b.Wtomove && bCount > 0)

	/*
		If our position is so good that even after giving a margin to the opponent,
		we still beat beta, we can safely prune.
		Applied at depths 1-7, NOT in PV nodes or when in check.
	*/
	if !inCheck && !isPVNode && depth <= 7 && depth >= 1 && abs32(beta) < Checkmate && !isRoot {
		rfpMargin := RFPMargins[depth]
		if !improving {
			rfpMargin -= 50 // More aggressive when not improving
		}
		if staticScore-rfpMargin >= beta {
			cutStats.StaticNullCutoffs++
			TT.storeEntry(posHash, depth, ply, ttMove, staticScore-rfpMargin, BetaFlag)
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
			TT.storeEntry(posHash, depth, ply, ttMove, score, BetaFlag)
			if depth > 10 {
				verifyScore := alphabeta(b, beta-1, beta, depth-1-R, ply, &childPVLine, prevMove, true, isExtended, 0, rootIndex)
				if verifyScore >= beta {
					return verifyScore
				}
			} else {
				return score
			}
		}
	}

	/*
		SINGULAR EXTENSION
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
		INTERNAL ITERATIVE REDUCTIONS
		Reduce depth when we have no TT move
	*/
	//if ttMove == 0 && depth >= 4 {
	//	depth--
	//	if !isPVNode {
	//		depth--
	//	}
	//}

	/*
	   INTERNAL ITERATIVE DEEPENING
	   When we have no TT move at sufficient depth, do a reduced search to find one.
	   This is much better than searching blind.
	*/
	if ttMove == 0 && depth >= 5 && !didNull && !isExtended {
		// Do a reduced-depth search
		reducedDepth := depth - 2
		if depth >= 8 {
			reducedDepth = depth - depth/4
		}

		var iidPV PVLine
		alphabeta(b, alpha, beta, reducedDepth, ply, &iidPV, prevMove, false, true, 0, rootIndex)

		// The IID search should have stored a TT entry - retrieve it
		iidEntry, _ := TT.ProbeEntry(posHash)
		if iidEntry.Move != 0 {
			ttMove = iidEntry.Move
			bestMove = ttMove
		}
	}

	// Generate and score moves
	allMoves := b.GenerateLegalMoves()

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
	//bestMove = 0

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

		isCapture := gm.IsCapture(move, b)
		moveGivesCheck := b.GivesCheck(move) // Assuming this method exists; if not, check after apply
		isPromotion := move.PromotionPieceType() != gm.PieceTypeNone

		// Tactical = capture, check, or promotion
		tactical := isCapture || moveGivesCheck || isPromotion
		legalMoves++

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

		// Check whether the move would give a check

		// Update tactical flag with actual check detection
		if moveGivesCheck {
			tactical = true
		}

		/*
			At depths 1-7, if static eval + margin can't beat alpha, prune quiet moves.
		*/
		if depth <= 7 && depth >= 1 && !moveGivesCheck && !isPVNode && !isRoot && !tactical && abs32(alpha) < Checkmate {
			futilityMargin := FutilityMargins[depth]
			if !improving {
				futilityMargin -= 50 // More aggressive when not improving
			}
			if staticScore+futilityMargin <= alpha {
				cutStats.FutilityPrunes++
				continue
			}
		}

		// Track quiet moves for history malus
		if !isCapture {
			quietMovesTried = append(quietMovesTried, move)
		}

		// Apply the move
		var unapplyFunc = applyMoveWithState(b, move)

		/*
			LATE MOVE REDUCTIONS
		*/
		extendMove := !isExtended && move == ttMove && singularExtension
		nextExtended := isExtended || extendMove

		if legalMoves == 1 {
			// First move: full-depth, full-window search
			nextDepth := calculateSearchDepth(depth-1, 0, extendMove)
			score = -alphabeta(b, -beta, -alpha, nextDepth, ply+1, &childPVLine, move, false, nextExtended, 0, rootIndex)
		} else {
			// Get move history for reduction calculation
			moveHistoryScore := historyMove[sideIdx][move.From()][move.To()]

			// Calculate reduction using all heuristics
			var reduct int8 = 0
			if depth >= LMRDepthLimit && legalMoves >= LMRMoveLimit && !moveGivesCheck && !tactical {
				reduct = computeLMRReduction(
					depth, legalMoves, int(index), isPVNode, tactical,
					moveHistoryScore, improving,
					IsKiller(move, ply, &KillerMoveTable), extendMove,
				)
			}

			// Perform Principal Variation Search with the calculated reduction
			score = searchMoveWithPVS(b, move, depth-1, reduct, alpha, beta, ply, extendMove, nextExtended, rootIndex, &childPVLine)
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
			//moveString := move.String()
			//if moveString != "g5h5" && moveString != "f4g5" && moveString != "g5f6" {
			//println("BETA CUTOFF -- move:", move.String(), " -- Score: ", score, " -- Alpha:Beta:", alpha, ":", beta, "-- depth:", depth)
			//}
			if !isCapture {
				// Store killer and counter moves
				InsertKiller(move, ply, &KillerMoveTable)
				storeCounter(b.Wtomove, prevMove, move)

				// History bonus for the good move
				incrementHistoryScore(b.Wtomove, move, depth)

				// History malus for all quiet moves that didn't work
				for _, failedMove := range quietMovesTried {
					if failedMove != move {
						decrementHistoryScoreBy(b.Wtomove, failedMove, depth)
					}
				}
			}
			break
		}

		// Alpha improvement
		if score > alpha {
			//if move.String() != "g5h5" || move.String() != "f4g5" {
			//println("ALPHA INCREASE -- move:", move.String(), " -- Score: ", score, " -- Alpha:Beta:", alpha, ":", beta, "-- depth:", depth)
			//}
			alpha = score
			ttFlag = ExactFlag
			pvLine.Update(move, childPVLine)

			if !isCapture {
				incrementHistoryScore(b.Wtomove, move, depth)
			}
		}
	}

	childPVLine.Clear()

	// Store in transposition table
	if !timeHandler.stopSearch && !GlobalStop && !searchShouldStop { //&& bestMove != 0 {
		TT.storeEntry(posHash, depth, ply, bestMove, bestScore, ttFlag)
	}

	return bestScore
}

func quiescence(b *gm.Board, alpha int32, beta int32, pvLine *PVLine, depth int8, ply int8, rootIndex int) int32 {
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

	var standpat int32 = Evaluation(b, false)

	// Check extension in qsearch
	//if inCheck {
	//	depth++
	//}

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

	var bestScore int32
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

// calculateSearchDepth computes the search depth for a move, accounting for reductions and extensions
func calculateSearchDepth(baseDepth int8, reduction int8, extendMove bool) int8 {
	depth := baseDepth - reduction
	if extendMove && reduction == 0 {
		depth++
	}
	return depth
}

// searchMoveWithPVS performs a Principal Variation Search for a move
// This implements the standard PVS 3-stage pattern:
// 1. Search with reduced depth using null window
// 2. If reduction was applied and score > alpha, re-search at full depth with null window
// 3. If score is between alpha and beta, do a full window search
func searchMoveWithPVS(b *gm.Board, move gm.Move, baseDepth int8, reduction int8,
	alpha int32, beta int32, ply int8, extendMove bool, nextExtended bool,
	rootIndex int, childPVLine *PVLine) int32 {

	// Stage 1: Reduced depth null-window search
	nextDepth := calculateSearchDepth(baseDepth, reduction, extendMove)
	score := -alphabeta(b, -(alpha + 1), -alpha, nextDepth, ply+1, childPVLine, move, false, nextExtended, 0, rootIndex)

	// Stage 2: Re-search at full depth if we had a reduction and score > alpha
	if score > alpha && reduction > 0 {
		nextDepth = calculateSearchDepth(baseDepth, 0, extendMove)
		score = -alphabeta(b, -(alpha + 1), -alpha, nextDepth, ply+1, childPVLine, move, false, nextExtended, 0, rootIndex)
	}

	// Stage 3: Full window search if score is in (alpha, beta) window
	if score > alpha && score < beta {
		nextDepth = calculateSearchDepth(baseDepth, 0, extendMove)
		score = -alphabeta(b, -beta, -alpha, nextDepth, ply+1, childPVLine, move, false, nextExtended, 0, rootIndex)
	}

	return score
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
