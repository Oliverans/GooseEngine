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
var LateMovePruningMargins = [7]int{
	999,
	999,
	999,
	12,
	16,
	24,
	32,
}

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

func StartSearch(board *gm.Board, depth uint8, gameTime int, increment int, useCustomDepth bool, evalOnly bool, moveOrderingOnly bool) string {
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

	if moveOrderingOnly {
		dumpRootMoveOrdering(board)
		os.Exit(0)
	}

	_, bestMove = rootsearch(board, depth, useCustomDepth)

	// Debug: probe TT at root and print stored best move (if any)
	entry := TT.getEntry(board.Hash())
	if entry != nil && entry.Move != 0 {
		fmt.Println("DEBUG ---> BEST MOVE FROM TT IS:", entry.Move.String())
	} else {
		fmt.Println("DEBUG ---> BEST MOVE FROM TT IS:", "<none>")
	}

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
		// Store a deep copy of the PV from this iteration to avoid it being
		// mutated by the next iteration's reuse of the PV slice backing array.
		prevPVLine = pvLine.Clone()

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

func dumpRootMoveOrdering(board *gm.Board) {
	legalMoves := board.GenerateLegalMoves()
	var nullMove gm.Move
	scoredMoves := scoreMovesList(board, legalMoves, 0, 0, nullMove, nullMove)
	for i := uint8(0); i < uint8(len(scoredMoves.moves)); i++ {
		orderNextMove(i, &scoredMoves)
	}

	fmt.Println("info string move ordering (start position)")
	for idx, entry := range scoredMoves.moves {
		fmt.Printf("info string #%d %s score=%d\n", idx+1, entry.move.String(), entry.score)
	}
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
	inHistory := posRepeats > 0

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
		DRAW DETECTION:
		Require a count of at least 2 repetitions of the same position declare a draw.
		#################################################################################

	*/
	// Repetition draw detection: HistoryMap at this node includes the
	// current occurrence (parent increments before recursing). Therefore, require
	// a count of at least 2 repetitions of the same position declare a draw.
	// This should also help catch any
	if (posRepeats >= 2 || b.IsDrawBy50()) && !isRoot { // Draw
		return 0
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
	ttEntry := TT.getEntry(posHash)
	usable, ttScore := TT.useEntry(ttEntry, posHash, depth, alpha, beta, ply, excludedMove)
	if usable && !isRoot && !inHistory {
		return ttScore
	}
	if usable {
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
	legalMoves := 0
	bestMove = 0

	for index := uint8(0); index < uint8(len(moveList.moves)); index++ {
		// Get the next move
		orderNextMove(index, &moveList)
		move := moveList.moves[index].move

		if move == excludedMove {
			continue
		}

		legalMoves++

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
		if depth <= 2 && !isPVNode && !tactical && legalMoves > LateMovePruningMargins[depth] {
			unapplyFunc()
			continue
		}

		// Futility pruning
		if futilityPruning && !tactical && !isPVNode && legalMoves > 1 {
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

		extendMove := !isExtended && move == ttEntry.Move && singularExtension
		nextExtended := isExtended || extendMove

		// Do the PV node full search - we should get one valid PVline even if we miss a bunch of search optimization
		if legalMoves <= 2 {
			nextDepth := depth - 1
			if extendMove {
				nextDepth++
			}
			score = -alphabeta(b, -beta, -alpha, nextDepth, ply+1, &childPVLine, move, didNull, nextExtended, 0)
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

			if score > alpha && reduct > 0 && legalMoves > LMRLegalMovesLimit { // If our search was good at a reduced search
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

	// Checkmate or stalemate
	if len(allMoves) == 0 {
		if inCheck {
			return -MaxScore + int16(ply) // Checkmate
		}
		return 0 // ... Draw
	}

	// Avoid polluting TT with potentially incomplete results when search is aborted
	if !timeHandler.stopSearch && !GlobalStop && !searchShouldStop && bestMove != 0 {
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

	var bestScore = alpha

	var moveList, _ = scoreMovesListCaptures(b, b.GenerateCaptures())

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

	return bestScore
}
