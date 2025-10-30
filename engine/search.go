package engine

import (
	"fmt"
	"os"
	"time"

	"github.com/dylhunn/dragontoothmg"
)

var EmptyMove dragontoothmg.Move

var MaxScore int16 = 20000
var Checkmate int16 = 9000

var killerMoveTable KillerStruct

// Quiescence variables
var quiescenceNodes = 0
var QuiescenceTime time.Duration
var FutilityPruneMargin int16 = int16(PieceValueEG[dragontoothmg.Queen])

// Alpha-Beta
var SearchTime time.Duration
var FutilityMargins = [7]int16{
	0,   // depth 0
	200, // depth 1
	325, // depth 2
	425, // depth 3
	500, // depth 4
	600, // depth 5
	725, // depth 6
}
var RazoringMargins = [4]int16{
	0,   // depth 0
	300, // depth 1
	325, // depth 2
	350, // depth 3
}
var staticNullMovePruningMargin int16 = 85
var extensionMargin int16 = 125

var LateMovePruningMargins = [5]int{0, 8, 12, 14, 16}
var LMRLegalMovesLimit = 4
var LMRDepthLimit = 3

// For IID
var aspirationWindowSize int16 = 40

var TT TransTable
var prevSearchScore int16 = 0
var timeHandler TimeHandler
var GlobalStop = false

func StartSearch(board *dragontoothmg.Board, depth uint8, gameTime int, increment int, useCustomDepth bool, evalOnly bool) string {
	initVariables()

	/*
		Used to debug and test out static evaluation:
			- game phase
			- prints out all important evaluation variables, such as
				- both midgame & endgame values
				- the difference in score between mid & endgame
					- all individual piece evaluation scores and their components
				- helper variables (outpost squares etc)
			- whether we consider this position moving towards a theoretical draw
	*/

	if !TT.isInitialized {
		TT.init()
	}

	GlobalStop = false
	timeHandler.initTimemanagement(gameTime, increment, int(board.Halfmoveclock), useCustomDepth)
	timeHandler.StartTime(board)

	var bestMove dragontoothmg.Move

	if evalOnly {
		Evaluation(board, true, false)
		println("Is this a theoretical draw: ", isTheoreticalDraw(board, true))
		os.Exit(0)
	}

	bestMove = rootsearch(board, depth)

	return bestMove.String()
}

func rootsearch(b *dragontoothmg.Board, depth uint8) dragontoothmg.Move {
	var timeSpent int64
	var alpha int16 = int16(prevSearchScore - aspirationWindowSize)
	var beta int16 = int16(prevSearchScore + aspirationWindowSize)
	//var bestScore = -MaxScore

	if prevSearchScore != 0 {
		alpha = int16(prevSearchScore - aspirationWindowSize)
		beta = int16(prevSearchScore + aspirationWindowSize)
	}

	var nullMove dragontoothmg.Move
	var bestMove dragontoothmg.Move
	var pvLine PVLine
	var aspirationLoopCount int = 0

	// Some extra stuff for debugging
	//var searchedDepth uint8

	for i := uint8(1); i <= depth && !timeHandler.TimeStatus(); i++ {

		// Clear PV line for next depth search
		pvLine.Clear()

		// Search & and update search time
		var startTime = time.Now()
		var score = alphabeta(b, alpha, beta, int8(i), 0, &pvLine, nullMove, false, false)
		timeSpent += time.Since(startTime).Milliseconds()

		// If we're out of time, break and we'll just try to grab whatever there is from the PV line
		if timeHandler.TimeStatus() {
			if len(pvLine.Moves) == 0 {
				continue // Search!
			}
			//fmt.Printf("Failed best move: %v \t Actual best move: %v", bestMove, TT.getEntry(b.Hash()).Move)
			break
		}

		// Make sure we don't divide with 0
		if timeSpent == 0 {
			timeSpent = 1
		}
		nps := uint64(float64(nodesChecked*1000) / float64(timeSpent))

		// Get the PV-line string
		var theMoves = getPVLineString(pvLine)

		/*
			#################################################################################
			ASPIRATION WINDOW
			Setting a smaller bound on alpha & beta, means we will cut more nodes initially when searching.
			It happens since it'll be easier to be above beta (fail high) or below alpha (fail low).
			If we misjudged our position, and we reach a value better or worse than the window (assumption is we're
			roughly correct about how we evaluate the position), we will increase set alpha&beta to the full scope instead
			#################################################################################
		*/
		if score <= alpha || score >= beta {
			switch aspirationLoopCount {
			case 0:
				alpha = -(aspirationWindowSize * 2)
				beta = aspirationWindowSize * 2
				aspirationLoopCount++
			case 1:
				alpha = -(aspirationWindowSize * 4)
				beta = aspirationWindowSize * 4
				aspirationLoopCount++
			case 2:
				alpha = -(aspirationWindowSize * 8)
				beta = aspirationWindowSize * 8
				aspirationLoopCount++
			default:
				alpha = -MaxScore
				beta = MaxScore
			}
			i--
			continue
		}

		// Reset the aspiration window size for the next search
		alpha = score - aspirationWindowSize
		beta = score + aspirationWindowSize

		// We save the bestMove so if we have to exit the search at the next depth early, we have something to return
		// We save the current best score so we have smth to work with for the next search
		bestMove = pvLine.GetPVMove(b)
		prevSearchScore = score

		//if bestMove == EmptyMove || bestMove == 0 || bestMove.String() == "0000" {
		//	fmt.Printf("1 - Failed best move: %v \t Actual best move: %v\n\n", bestMove, TT.getEntry(b.Hash()).Move)
		//}

		aspirationLoopCount = 0

		// Print out current search results
		fmt.Println("info depth ", i, "\tscore ", getMateOrCPScore(int(score)), "\tnodes ", nodesChecked, "\ttime ", timeSpent, "\tnps ", nps, "\tpv", theMoves)

		//searchedDepth = i
		if score >= (MaxScore-100) || score <= -(MaxScore+100) {
			break
		}
	}

	//var theMoves = getPVLineString(pvLine)
	//fmt.Printf("info depth %v score %v nodes %v time %v pv%v\n", searchedDepth, getMateOrCPScore(int(prevSearchScore)), nodesChecked, timeSpent, theMoves)

	// Reset search variables
	nodesChecked = 0
	timeHandler.stopSearch = false

	//fmt.Printf("3g - Failed best move: %v \t Actual best move: %v \t Best move string: %v \n", bestMove, TT.getEntry(b.Hash()).Move, TT.getEntry(b.Hash()).Move.String())

	// Prepare for next search ...
	//prevSearchScore = bestScore

	return bestMove
}

func alphabeta(b *dragontoothmg.Board, alpha int16, beta int16, depth int8, ply int8, pvLine *PVLine, prevMove dragontoothmg.Move, didNull bool, isExtended bool) int16 {
	nodesChecked++

	if nodesChecked&2048 == 0 && depth != 1 {
		if timeHandler.TimeStatus() || GlobalStop {
			return 0
		}
	}

	/* INIT KEY VARIABLES */
	var bestMove dragontoothmg.Move = EmptyMove
	var childPVLine = PVLine{}
	var isPVNode = (beta - alpha) != 1
	var futilityPruning = false
	var inCheck bool = b.OurKingInCheck()

	// 3-fold repetition draw
	var posHash uint64 = b.Hash()
	if isThreefoldRepetition(posHash) {
		return 0
	}

	// Make sure we extend our search by 1 so we don't end our search while in check ...
	if inCheck {
		depth++
	}

	// If we reach our target depth, do a quiescene search and return the score
	if depth <= 0 {
		score := quiescence(b, alpha, beta, &childPVLine, 0, 0)
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
	var isRoot = ply == 0
	ttEntry := TT.getEntry(posHash)
	usable, ttScore := TT.useEntry(ttEntry, posHash, depth, ply, alpha, beta, uint8(b.Fullmoveno))
	if usable && !isRoot {
		return ttScore
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
	var anyMinorsOrMajors = (wCount > 0 || bCount > 0)
	if !inCheck && !isPVNode && !didNull && anyMinorsOrMajors && depth >= 3 && beta < Checkmate {
		unApplyfunc := b.ApplyNullMove()
		//var R int8 = 3 + (6 / depth)
		var R int8 = 2
		score := -alphabeta(b, -beta, -beta+1, (depth - 1 - R), ply+1, &childPVLine, ttEntry.Move, true, isExtended)
		unApplyfunc()
		if score >= beta {
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
	if !inCheck && !isPVNode && beta < Checkmate {
		if (int16(staticScore) - (staticNullMovePruningMargin * int16(depth))) >= beta {
			return beta
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
	if !inCheck && !isPVNode && int(depth) < len(RazoringMargins) && beta < Checkmate {
		if int16(staticScore)+(RazoringMargins[depth]) < alpha {
			score := quiescence(b, alpha-1, alpha, &childPVLine, 0, 0)
			if score < alpha {
				return score
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
	if !inCheck && !isPVNode && int(depth) < len(FutilityMargins) && alpha < Checkmate && beta < Checkmate {
		if int16(staticScore)+FutilityMargins[depth] < alpha {
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
	//if ttEntry.Move == EmptyMove && depth >= 5 && !didNull && !inCheck {
	//	//var R int8 = 3 + (6 / depth)
	//	_ = -alphabeta(b, -alpha+1, -alpha, (depth - 1 - (1 + depth/8)), ply+1, &childPVLine, prevMove, didNull, isExtended)
	//	if len(childPVLine.Moves) > 0 {
	//		bestMove = childPVLine.GetPVMove(b)
	//		childPVLine.Clear()
	//	}
	//}

	var bestScore = -MaxScore

	// Generate legal moves
	allMoves := b.GenerateLegalMoves()

	var moveList moveList
	if bestMove == EmptyMove && ttEntry.Flag != AlphaFlag {
		moveList = scoreMovesList(b, allMoves, depth, ttEntry.Move, prevMove)
	} else {
		moveList = scoreMovesList(b, allMoves, depth, bestMove, prevMove)
	}

	var ttFlag int8 = AlphaFlag
	bestMove = EmptyMove

	for index := uint8(0); index < uint8(len(moveList.moves)); index++ {
		// Get the next move
		orderNextMove(index, &moveList)
		var move dragontoothmg.Move = moveList.moves[index].move

		// Prepare logic for filtering
		var isCapture bool = moveList.moves[index].capturedPiece > 0
		var unapplyFunc = b.Apply(move)
		var inCheck = b.OurKingInCheck()

		// Tactical moves - if we're capturing, checking or promoting a pawn
		var tactical bool = (isCapture || inCheck || move.Promote() > 0)

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
		if futilityPruning && !tactical && !isPVNode && index > 2 {
			unapplyFunc()
			continue
		}

		var posHash = b.Hash()
		History.History = append(History.History, posHash)

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

		// Do the PV node full search - we should get one valid PVline even if we miss a bunch of search optimization
		if index <= 2 {
			score = -alphabeta(b, -beta, -alpha, (depth - 1), ply+1, &childPVLine, move, didNull, isExtended)
			isExtended = false
		} else {
			//var reduct int8 = 0

			var reduct = int8(LMR[depth][index])
			//if depth-1-reduct < 1 {
			//	reduct = depth - 2
			//}

			score = -alphabeta(b, -(alpha + 1), -alpha, (depth - 1 - reduct), ply+1, &childPVLine, move, didNull, isExtended)

			if score > alpha && reduct > 0 { // If our search was good at a reduced search
				score = -alphabeta(b, -(alpha + 1), -alpha, (depth - 1), ply+1, &childPVLine, move, didNull, isExtended)
				if score > alpha && score < beta {
					score = -alphabeta(b, -beta, -alpha, (depth - 1), ply+1, &childPVLine, move, didNull, isExtended)
				}
			} else if score > alpha && score < beta { // Full search ....
				score = -alphabeta(b, -beta, -alpha, (depth - 1), ply+1, &childPVLine, move, didNull, isExtended)
			}
		}

		if score > bestScore {
			bestScore = score
			bestMove = move
		}

		History.History = History.History[:len(History.History)-1]
		unapplyFunc()

		if score >= beta {
			ttFlag = BetaFlag
			if !isCapture {
				killerMoveTable.InsertKiller(move, depth)
				incrementHistoryScore(b, move, depth)
				storeCounter(!b.Wtomove, prevMove, move)
			}
			break
		}

		if score > alpha {
			alpha = score
			ttFlag = ExactFlag
			pvLine.Update(move, childPVLine)
			if !isCapture {
				incrementHistoryScore(b, move, depth)
				storeCounter(!b.Wtomove, prevMove, move)
			}
		} else {
			if !isCapture {
				decrementHistoryScore(b, move)
			}
		}

		childPVLine.Clear()
	}

	// Checkmate or stalemate
	if len(allMoves) == 0 {
		if inCheck {
			return -MaxScore + int16(ply) // Checkmate
		}
		return 0 // ... Draw
	}

	if !timeHandler.stopSearch && bestMove != EmptyMove {
		TT.storeEntry(posHash, depth, ply, bestMove, bestScore, ttFlag, uint8(b.Fullmoveno))
	}

	return bestScore
}

func quiescence(b *dragontoothmg.Board, alpha int16, beta int16, pvLine *PVLine, depth int8, ply int8) int16 {
	nodesChecked++
	quiescenceNodes++

	//var isRoot = ply == 0

	if nodesChecked&2048 == 0 {
		if timeHandler.TimeStatus() || GlobalStop {
			return 0
		}
	}

	var inCheck bool = b.OurKingInCheck()

	if inCheck {
		// If no legal evasion exists, it's checkmate.
		if len(b.GenerateLegalMoves()) == 0 {
			return -MaxScore + int16(ply)
		}
	}

	var childPVLine = PVLine{}

	var standpat int16 = int16(Evaluation(b, false, false))

	if inCheck {
		depth++
	}

	if !inCheck && standpat >= beta {
		return standpat
	}

	//if depth <= -10 {
	//	return alpha
	//}

	/*
		#################################################################################
		1) BIG DELTA PRUNING:
		We check whether a move's capture plus a
		can beat alpha. If it does not, we don't want to continue on this branch
		#################################################################################
	*/
	if standpat < (alpha-FutilityPruneMargin) && !inCheck {
		return alpha
	}

	alpha = max(alpha, standpat)

	var bestScore = alpha
	var moves = b.GenerateLegalMoves()
	var moveList, hasAnyCapture = scoreMovesListCaptures(b, moves, EmptyMove)

	if hasAnyCapture {
		for index := uint8(0); index < uint8(len(moveList.moves)); index++ {

			orderNextMove(index, &moveList)
			move := moveList.moves[index].move

			/*
				#################################################################################
				2) STATIC EXCHANGE EVALUATION:
				We attempt to calculate the potential material win or loss of trading pieces on a given square
				We most likely don't want to lose too much material .... So if it seems we are, we skip this move

				TODO:
				This need to take into account pins at some point!
				#################################################################################
			*/
			see := see(b, move, false)
			if see < 0 {
				continue
			}

			unapplyFunc := b.Apply(move)

			score := -quiescence(b, -beta, -alpha, &childPVLine, depth-1, ply+1)

			unapplyFunc()

			if score > bestScore {
				bestScore = score
			}

			if score >= beta {
				break
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
