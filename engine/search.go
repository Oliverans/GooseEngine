package engine

import (
	"fmt"
	"os"
	"time"

	gm "chess-engine/goosemg"
)

var MaxScore int16 = 32500
var Checkmate int16 = 20000

var killerMoveTable KillerStruct

var SearchTime time.Duration
var searchShouldStop bool

var FutilityMargins = [3]int16{
	0,   // depth 0
	150, // depth 1
	270, // depth 2
	//170, // depth 3
	//190, // depth 4
	//210, // depth 5
	//220, // depth 6
	//240, // depth 7
	//260, // depth 8
	//290, // depth 9
}

var RazoringMargins = [10]int16{
	0,   // depth 0
	100, // depth 1
	125, // depth 2
	150, // depth 3
	175, // depth 3
	200, // depth 3
	225, // depth 3
	250, // depth 3
	275, // depth 3
}

// Late move pruning
var LateMovePruningMargins = [6]int{0, 0, 0, 16, 20, 24}

var LMRLegalMovesLimit = 4
var LMRDepthLimit = 3

// Aspiration window variable
// TBD: Tinker until its "better"; 35 seems to give the best results, although I've
// not verified this at all other than testing out the values in 4-5 (wild) test-positions
var aspirationWindowSize int16 = 35

var TT TransTable
var prevSearchScore int16 = 0
var timeHandler TimeHandler
var GlobalStop = false

func StartSearch(board *gm.Board, depth uint8, gameTime int, increment int, useCustomDepth bool, evalOnly bool) string {
	initVariables(board)

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
		os.Exit(0)
	}

	_, bestMove = rootsearch(board, depth, useCustomDepth)

	return bestMove.String()
}

func rootsearch(b *gm.Board, depth uint8, useCustomDepth bool) (int, gm.Move) {
	var timeSpent int64
	var alpha int16 = int16(prevSearchScore - aspirationWindowSize)
	var beta int16 = int16(prevSearchScore + aspirationWindowSize)
	var bestScore = -MaxScore

	if prevSearchScore != 0 {
		alpha = int16(prevSearchScore - aspirationWindowSize)
		beta = int16(prevSearchScore + aspirationWindowSize)
	}
	var nullMove gm.Move
	var bestMove gm.Move
	var pvLine PVLine
	var prevPVLine PVLine
	var mateFound bool

	for i := uint8(1); i <= depth && !timeHandler.TimeStatus(); i++ {
		// Clear PV line for next search
		pvLine.Clear()

		// Search & and update search time
		var startTime = time.Now()
		var score = alphabeta(b, alpha, beta, int8(i), 0, &pvLine, nullMove, false, false, 0)
		timeSpent += time.Since(startTime).Milliseconds()

		// Calculate some numbers for INFO & debugging
		if timeSpent == 0 {
			timeSpent = 1
		}
		nps := uint64(float64(nodesChecked*1000) / float64(timeSpent))

		// Get the PV-line string! Thx
		var theMoves = getPVLineString(pvLine)
		if (score > Checkmate || score < -Checkmate) && len(pvLine.Moves) > 0 {
			mateFound = true
		}

		/*
			#################################################################################
			ASPIRATION WINDOW
			Setting a smaller bound on alpha & beta, means we will cut more nodes initially when searching.
			It happens since it'll be easier to be above beta (fail high) or below alpha (fail low).
			If we misjudged our position, and we reach a value better or worse than the window (assumption is we're
			roughly correct about how we evaluate the position), we will increase set alpha&beta to the full scope instead
			#################################################################################
		*/
		if (score <= alpha || score >= beta) && !timeHandler.TimeStatus() {
			alpha = -MaxScore
			beta = MaxScore
			i--
			continue
		}

		// Apply the window size!
		alpha = score - aspirationWindowSize
		beta = score + aspirationWindowSize
		bestScore = score

		if timeHandler.TimeStatus() && len(prevPVLine.Moves) >= 1 && !useCustomDepth {
			break
		}

		prevSearchScore = bestScore
		prevPVLine = pvLine

		_ = nps
		_ = theMoves
		fmt.Println("info depth ", i, "\tscore ", getMateOrCPScore(int(score)), "\tnodes ", nodesChecked, "\ttime ", timeSpent, "\tnps ", nps, "\tpv", theMoves)
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

func alphabeta(b *gm.Board, alpha int16, beta int16, depth int8, ply int8, pvLine *PVLine, prevMove gm.Move, didNull bool, isExtended bool, excludedMove gm.Move) int16 {
	nodesChecked++

	if nodesChecked&2047 == 0 {
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
	var futilityPruning = false

	inCheck := b.OurKingInCheck()

	// Generate legal moves
	allMoves := b.GenerateLegalMoves()

	posHash := b.Hash()
	posRepeats := HistoryMap[posHash]

	// 3-fold repition draw
	if posRepeats >= 2 && !isRoot { // Draw
		return 0
	}

	// Make sure we extend our search by 1 so we don't end our search while in check ...
	if inCheck {
		depth++
	}

	// If we reach our target depth, do a quiescene search and return the score
	if depth <= 0 {
		score := quiescence(b, alpha, beta, &childPVLine, 30)
		return score
	}

	/*
		#################################################################################
		TRANSPOSITION TABLE:
		We save the results from our previous searches, and check whether we can use it
		to determine if it was a good or bad search result.

		if we found a previously searched position that was searched at a higher depth, it is "usable"
		Then we don't have to re-search the same position, and can return the previous search's result
		#################################################################################
	*/
	ttEntryPtr, ttFound := TT.getEntry(posHash)
	var ttEntry TTEntry
	if ttEntryPtr != nil {
		ttEntry = *ttEntryPtr
	}
	usable, ttScore := TT.useEntry(ttEntryPtr, posHash, depth, alpha, beta, ply, excludedMove)
	if usable && !isRoot {
		return ttScore
	}
	if ttFound {
		bestMove = ttEntry.Move
	}

	var staticScore = Evaluation(b, false, false)

	/*
		#################################################################################
		NULL MOVE PRUNING:
		If we're in a position where, even if the opponent gets a free move (we do "null move"),
		and we're still above beta, we're most likely in a fail-high node; so we prune.
		#################################################################################
	*/

	var wCount, bCount = hasMinorOrMajorPiece(b)
	var sideHasPieces bool = (b.Wtomove && wCount > 0) || (!b.Wtomove && bCount > 0)
	if !inCheck && !isPVNode && !didNull && sideHasPieces && depth >= 2 {
		unApplyfunc := b.ApplyNullMove()
		nullHash := b.Hash()
		HistoryMap[nullHash]++
		var R int8 = 2 + (depth / 6)
		score := -alphabeta(b, -beta, -beta+1, (depth - 1 - R), ply+1, &childPVLine, bestMove, true, isExtended, 0)
		unApplyfunc()
		HistoryMap[nullHash]--
		if score >= beta && score < Checkmate {
			return beta
		}
	}

	/*
		#################################################################################
		STATIC NULL MOVE PRUNING:
		If we raise beta even after giving a large penalty to our score, we're most likely in a FAIL HIGH node
		So we return
		#################################################################################
	*/
	if !inCheck && !isPVNode && sideHasPieces && absInt16(beta) < Checkmate {
		var staticFutilityPruneScore int16 = (85 * int16(depth))
		if (int16(staticScore) - staticFutilityPruneScore) >= beta {
			return beta
		}
	}

	var singularExtension bool
	if !isPVNode && !isRoot && !inCheck && !didNull && !isExtended && depth >= 6 && ttEntry.Move != 0 && (ttEntry.Flag == ExactFlag || ttEntry.Flag == BetaFlag) && ttEntry.Depth >= depth-1 {
		ttValue := ttEntry.Score
		if ttValue > Checkmate {
			ttValue -= int16(ply)
		}
		if ttValue < -Checkmate {
			ttValue += int16(ply)
		}
		if ttValue < Checkmate && ttValue > -Checkmate {
			margin := int16(75 + 5*depth)
			// Base beta
			scoreToBeat := ttValue - margin

			R := 3 + depth/6

			var verificationPV PVLine
			scoreSingular := alphabeta(b, scoreToBeat-1, scoreToBeat, depth-1-R, ply, &verificationPV, prevMove, didNull, true, ttEntry.Move)
			if scoreSingular < scoreToBeat {
				singularExtension = true
			}
		}
	}

	/*
		#################################################################################
		RAZORING:
		If we're in a pre-pre-frontier node (3 steps away from the horizon - our maxDepth - and our current evaluation
		is bad as-is, we do a quick quiescence search - and if we don't beat alpha (i.e. fail low), we can return early
		and not search any more moves in this branch
		If we extend this logic, the same thing applies at other nodes than pre-pre-frontier nodes; so we make it a bit more dynamic.
		#################################################################################
	*/
	if !inCheck && !isPVNode && int(depth) < len(RazoringMargins) {
		staticFutilityPruneScore := RazoringMargins[depth] * 3
		if int16(staticScore)+staticFutilityPruneScore < alpha {
			score := quiescence(b, alpha, beta, &childPVLine, 30)
			if score < alpha {
				return alpha
			}
		}
	}

	/*
		#################################################################################
		FUTILITY PRUNING:
		If we're 1 step away from our max depth (a 'frontier node'), we make a static evaluation.
		If (evaluation + minor piece margin) does not beat alpha, we'll most likely fail low.
		Basically, we're assuming we're down so much material, or that our pieces are in such awful positions its not worth doing
		a quiet move here.

		We will instead only check tactical moves (i.e. capture, check or promotion)
		in order to make sure we don't miss anything important.
		#################################################################################
	*/

	if !inCheck && !isPVNode && !isRoot && int(depth) < len(FutilityMargins) && absInt16(alpha) < Checkmate {
		futilityPruneScore := FutilityMargins[depth]
		if int16(staticScore)+futilityPruneScore <= alpha {
			futilityPruning = true
		}
	}

	var score = -MaxScore

	/*
		#################################################################################
		INTERNAL ITERATIVE DEEPENING:
		If we didn't find any move in the transposition table, for the sake of move ordering, it's faster to make a shallow search
		and get the PV move from that
		#################################################################################
	*/

	if ttEntry.Move == 0 && depth >= 4 && !didNull {
		score := -alphabeta(b, -beta, -alpha, (depth - 2), ply+1, &childPVLine, prevMove, didNull, isExtended, 0)
		_ = score
		if len(childPVLine.Moves) > 0 {
			bestMove = childPVLine.GetPVMove()
			childPVLine.Clear()
		}
	}

	var bestScore = -MaxScore
	var moveList moveList = scoreMovesList(b, allMoves, depth, ply, bestMove, prevMove)

	var ttFlag int8 = AlphaFlag
	var searchedAny bool
	bestMove = 0

	for index := uint8(0); index < uint8(len(moveList.moves)); index++ {
		// Get the next move
		orderNextMove(index, &moveList)
		move := moveList.moves[index].move

		if excludedMove != 0 && move == excludedMove {
			continue
		}

		// Prepare variables for move search
		var isCapture bool = gm.IsCapture(move, b) // Get whether move is a capture, before moving

		var unapplyFunc = b.Apply(move)
		var inCheck = b.OurKingInCheck()
		var posHash = b.Hash()

		// Tactical moves - if we're capturing, checking or promoting a pawn
		tactical := (isCapture || inCheck || move.PromotionPieceType() != gm.PieceTypeNone)

		/*
			#################################################################################
			LATE MOVE PRUNING:
			Assuming our move ordering is good, we're not interested in searching most moves at the bottom of the move ordering
			We're most likely interested in the full depth for the first 1 or 2 moves
			#################################################################################
		*/
		if depth < int8(len(LateMovePruningMargins)) && !isPVNode && !tactical && int(index) > LateMovePruningMargins[depth] {
			unapplyFunc()
			continue
		}

		// Futility pruning
		if futilityPruning && !tactical && !isPVNode {
			unapplyFunc()
			continue
		}

		HistoryMap[posHash]++

		/*
			#################################################################################
			LATE MOVE REDUCTION:
			Assuming good move ordering, the first move is <most likely> the best move.
			We will therefor spend less time searching moves further down in the move ordering.

			If we didn't already get a beta cut-off (Cut-node), most likely we're in an All-node (where we
			have actually search and not cut ...)! Meaning we want to avoid spending time here.

			We do a reduced search hoping it will fail low. However, if we manage to raise alpha after
			doing a reduced search, we will do a full search of that node as that note has the potential to be
			an interesting path to take in the tree.

			Great blog-post that helped me wrap my head around this:
			http://macechess.blogspot.com/2010/08/implementing-late-move-reductions.html
			#################################################################################
		*/

		extendMove := !isExtended && singularExtension && move == ttEntry.Move
		nextExtended := isExtended || extendMove

		// Do the PV node full search - we should get one valid PVline even if we miss a bunch of search optimization
		if index <= 2 {
			nextDepth := depth - 1
			if extendMove {
				nextDepth++
			}
			score = -alphabeta(b, -beta, -alpha, nextDepth, ply+1, &childPVLine, move, didNull, nextExtended, 0)
			searchedAny = true
		} else {
			var reduct int8 = 0
			if !isPVNode && !tactical && int(depth) >= LMRDepthLimit {
				d := int(depth)
				if d < 0 {
					d = 0
				} else if d > MaxDepth {
					d = MaxDepth
				}
				moveIdx := int(index)
				row := LMR[d]
				if len(row) == 0 {
					reduct = 0
				} else {
					if moveIdx >= len(row) {
						moveIdx = len(row) - 1
					}
					reduct = row[moveIdx]
				}
			}

			nextDepth := depth - 1 - reduct
			if nextDepth < 0 {
				nextDepth = 0
			}
			if extendMove && reduct == 0 {
				nextDepth++
			}

			score = -alphabeta(b, -(alpha + 1), -alpha, nextDepth, ply+1, &childPVLine, move, didNull, nextExtended, 0)
			searchedAny = true

			if score > alpha && reduct > 0 { // If our search was good at a reduced search
				nextDepth = depth - 1
				if nextDepth < 0 {
					nextDepth = 0
				}
				if extendMove {
					nextDepth++
				}
				score = -alphabeta(b, -(alpha + 1), -alpha, nextDepth, ply+1, &childPVLine, move, didNull, nextExtended, 0)
				if score > alpha {
					score = -alphabeta(b, -beta, -alpha, nextDepth, ply+1, &childPVLine, move, didNull, nextExtended, 0)
				}
			} else if score > alpha && score < beta { // If our search was in range
				nextDepth = depth - 1
				if nextDepth < 0 {
					nextDepth = 0
				}
				if extendMove {
					nextDepth++
				}
				score = -alphabeta(b, -beta, -alpha, nextDepth, ply+1, &childPVLine, move, didNull, nextExtended, 0)
			}
		}

		// Catches both the first <alpha and >beta, so we always get a move in the
		// transposition table if this move was the cause of the cut or raising of alpha
		if score > bestScore {
			bestScore = score
			bestMove = move
		}

		HistoryMap[posHash]--

		if score >= beta {
			ttFlag = BetaFlag
			if !isCapture {
				InsertKiller(move, ply, &killerMoveTable)
				storeCounter(!b.Wtomove, prevMove, move)
				incrementHistoryScore(!b.Wtomove, move, depth)
			}
			childPVLine.Clear()
			unapplyFunc()
			break
		}

		if score > alpha {
			alpha = score
			ttFlag = ExactFlag
			pvLine.Update(move, childPVLine)
			if !isCapture {
				incrementHistoryScore(!b.Wtomove, move, depth)
			}
		} else {
			decrementHistoryScore(!b.Wtomove, move)
		}

		unapplyFunc()
		childPVLine.Clear()
	}

	if len(allMoves) > 0 && !searchedAny {
		return max(alpha, int16(staticScore))
	}

	// Checkmate or stalemate
	if len(allMoves) == 0 {
		if inCheck {
			return -MaxScore + int16(ply) // Checkmate
		}
		return 0 // ... Draw
	}

	if !timeHandler.stopSearch && !GlobalStop && bestMove != 0 {
		TT.storeEntry(posHash, depth, ply, bestMove, bestScore, ttFlag)
	}

	return bestScore
}

func quiescence(b *gm.Board, alpha int16, beta int16, pvLine *PVLine, depth int8) int16 {
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

	var standpat int16 = int16(Evaluation(b, false, false))

	if inCheck {
		depth++
	}

	alpha = max(alpha, standpat)

	if alpha >= beta && !inCheck {
		return alpha
	}

	if depth <= 0 {
		return standpat
	}

	var moves = b.GenerateLegalMoves()
	if inCheck {
		if len(moves) == 0 {
			return -MaxScore + 1
		}
		var bestScore int16 = -MaxScore
		for _, move := range moves {
			unapplyFunc := b.Apply(move)
			score := -quiescence(b, -beta, -alpha, &childPVLine, depth-1)
			unapplyFunc()

			if score > bestScore {
				bestScore = score
			}

			if score >= beta {
				return beta
			}

			if score > alpha {
				alpha = score
				pvLine.Update(move, childPVLine)
			}
			childPVLine.Clear()
		}
		return bestScore
	}

	var bestScore = alpha
	var moveList, hasAnyCapture = scoreMovesListCaptures(b, moves)

	if hasAnyCapture {
		for index := uint8(0); index < uint8(len(moveList.moves)); index++ {

			orderNextMove(index, &moveList)
			move := moveList.moves[index].move
			see := see(b, move, false)
			if see < 0 {
				continue
			}

			unapplyFunc := b.Apply(move)

			score := -quiescence(b, -beta, -alpha, &childPVLine, depth-1)
			unapplyFunc()

			if score > bestScore {
				bestScore = score
			}

			if score >= beta {
				return beta
			}

			if score > alpha {
				alpha = score
				pvLine.Update(move, childPVLine)
			}
			childPVLine.Clear()
		}
	}

	return bestScore
}
