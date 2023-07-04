package engine

import (
	"fmt"
	"os"
	"time"

	"github.com/dylhunn/dragontoothmg"
)

var MinScore int16 = -32000
var MaxScore int16 = 32000

var killerMoveTable KillerStruct

//Quiescence variables
var quiescenceNodes = 0
var QuiescenceTime time.Duration

var SearchTime time.Duration
var searchShouldStop bool

var FutilityMargins = [10]int16{
	0,   // depth 0
	100, // depth 1
	140, // depth 2
	170, // depth 3
	190, // depth 4
	210, // depth 5
	220, // depth 6
	240, // depth 7
	260, // depth 8
	290, // depth 9
}

var RazoringMargins = [10]int{
	0,   // depth 0
	120, // depth 1
	150, // depth 2
	180, // depth 3
	210, // depth 4
	220, // depth 5
	240, // depth 6
	270, // depth 7
	300, // depth 8
	320, // depth 9
}

// Late move pruning
//var LateMovePruningMargins = [6]int{0, 16, 20, 24, 28, 36}

var LateMovePruningMargins = [6]int{0, 8, 16, 24, 28, 32}

//var LateMovePruningMargins = [6]int{14, 16, 18, 22, 24, 34}

var LMRLegalMovesLimit = 4
var LMRDepthLimit = 3

// Aspiration window variable
// TBD: Tinker until its "better"; 35 seems to give the best results, although I've
// not verified this at all other than testing out the values in 4-5 (wild) test-positions
var aspiratinoWindowSize int16 = 30

var lateMovePruningCounter = 0
var lateMoveReductionCounter = 0
var lateMoveReductionFullSearch = 0
var futilityPruningCounter = 0
var razoringCounter = 0
var staticNullMovePruneCount = 0
var nullMovePruneCount = 0
var IIDCounter = 0
var ttMoveCounter = 0

var HistoryMap map[uint64]int = make(map[uint64]int, 5000)
var History HistoryStruct

var TT TransTable
var prevSearchScore int16 = 0
var timeHandler TimeHandler
var GlobalStop = false

func StartSearch(board *dragontoothmg.Board, depth int, gameTime int, increment int, useCustomDepth bool, evalOnly bool) string {
	initVariables(board)

	/*
		Used to debug and test out static evaluation:
			- game phase
			- prints out all important evaluation variables, such as
				- both midgame & endgame values
				- the difference in score between mid & endgame
			- whether we consider this position moving towards a theoretical draw
	*/
	if evalOnly {
		Evaluation(board, true)
		println("Is this a theoretical draw: ", isTheoreticalDraw(board, true))
		os.Exit(0)
	}

	// Just some test stuff for SEE; Will remove later
	//var tempMove dragontoothmg.Move
	//tempMove.Setfrom(dragontoothmg.Square(17))
	//tempMove.Setto(dragontoothmg.Square(41))
	//see(b, tempMove, true)
	//os.Exit(0)

	GlobalStop = false
	if !timeHandler.isInitialized {
		timeHandler.initTimemanagement(gameTime, increment, int(board.Halfmoveclock), useCustomDepth)
		timeHandler.StartTime(int(board.Halfmoveclock))
	} else {
		timeHandler.initTimemanagement(gameTime, increment, int(board.Halfmoveclock), useCustomDepth)
		timeHandler.StartTime(int(board.Halfmoveclock))
	}

	var nilMove dragontoothmg.Move
	for i := 0; i < 64; i++ {
		for z := 0; z < 64; z++ {
			counterMove[0][i][z] = nilMove
			counterMove[1][i][z] = nilMove
		}
	}

	for i := 0; i < 64; i++ {
		for z := 0; z < 64; z++ {
			historyMove[0][i][z] = 0
			historyMove[1][i][z] = 0
		}
	}

	if !TT.isInitialized {
		TT.init()
	}

	var bestMove dragontoothmg.Move

	_, bestMove = rootsearch(board, depth, useCustomDepth)

	//NodesChecked = 0

	GlobalStop = false

	ttNodes = 0
	cutNodes = 0
	lateMoveReductionCounter = 0
	lateMoveReductionFullSearch = 0
	nullMovePruneCount = 0
	lateMovePruningCounter = 0
	razoringCounter = 0
	quiescenceNodes = 0
	nodesChecked = 0

	return bestMove.String()
}

func rootsearch(b *dragontoothmg.Board, depth int, useCustomDepth bool) (int, dragontoothmg.Move) {
	var repeatSearchCounter uint8
	var totalRepeatSearches uint8
	var timeSpent int64
	var alpha int16 = int16(prevSearchScore - aspiratinoWindowSize)
	var beta int16 = int16(prevSearchScore + aspiratinoWindowSize)
	var bestScore = MinScore

	if prevSearchScore != 0 {
		alpha = int16(prevSearchScore - aspiratinoWindowSize)
		beta = int16(prevSearchScore + aspiratinoWindowSize)
	}
	var nullMove dragontoothmg.Move
	var bestMove dragontoothmg.Move
	var pvLine PVLine
	var prevPVLine PVLine
	var mateFound bool

	for i := 1; i <= depth && !mateFound && !timeHandler.TimeStatus(); i++ {
		// Clear PV line for next search
		pvLine.Clear()

		// Search & and update search time
		var startTime = time.Now()
		var score = alphabeta(b, alpha, beta, int8(i), 0, &pvLine, nullMove, false)
		timeSpent += time.Since(startTime).Milliseconds()

		// Calculate some numbers for INFO & debugging
		if timeSpent == 0 {
			timeSpent = 1
		}
		nps := uint64(float64(nodesChecked*1000) / float64(timeSpent))

		// Get the PV-line string! Thx
		var theMoves = getPVLineString(pvLine)

		if (score > MaxScore-50 || score < MinScore+50) && len(pvLine.Moves) > 0 {
			mateFound = true
		}

		/*
			==========================================================================================================
			ASPIRATION WINDOW
			Setting a smaller bound on alpha & beta, means we will cut more nodes initially when searching.
			It happens since it'll be easier to be above beta (fail high) or below alpha (fail low).
			If we misjudged our position, and we reach a value better or worse than the window (assumption is we're
			roughly correct about how we evaluate the position), we will increase set alpha&beta to the full scope instead
			==========================================================================================================
		*/
		if int16(score) <= alpha || int16(score) >= beta {
			if repeatSearchCounter == 6 {
				alpha = MinScore
				beta = MaxScore
				repeatSearchCounter = 0
			} else {
				alpha = prevSearchScore - (aspiratinoWindowSize * int16(repeatSearchCounter+1))
				beta = prevSearchScore + (aspiratinoWindowSize * int16(repeatSearchCounter+1))
			}
			//alpha = MinScore
			//beta = MaxScore
			i--
			repeatSearchCounter++
			totalRepeatSearches++
			continue
		}

		// Apply the window size!
		alpha = score - aspiratinoWindowSize
		beta = score + aspiratinoWindowSize
		bestScore = int16(score)

		if timeHandler.TimeStatus() && len(prevPVLine.Moves) >= 1 && !useCustomDepth {
			break
		}
		prevSearchScore = bestScore
		prevPVLine = pvLine
		_ = nps
		_ = theMoves
		fmt.Println("info depth ", i, "\tscore ", getMateOrCPScore(int(score)), "\tnodes ", nodesChecked, "\ttime ", timeSpent, "\tnps ", nps, "\tpv", theMoves)
	}

	ttNodes = 0
	cutNodes = 0
	lateMovePruningCounter = 0
	lateMoveReductionCounter = 0
	lateMoveReductionFullSearch = 0
	futilityPruningCounter = 0
	nullMovePruneCount = 0
	razoringCounter = 0
	quiescenceNodes = 0
	nodesChecked = 0

	searchShouldStop = false

	bestMove = prevPVLine.GetPVMove()
	prevSearchScore = bestScore
	//TT.clearTT()
	return int(bestScore), bestMove
}

var pvNodesCount int
var nonPVNodesCount int

func alphabeta(b *dragontoothmg.Board, alpha int16, beta int16, depth int8, ply int8, pvLine *PVLine, prevMove dragontoothmg.Move, didNull bool) int16 {
	nodesChecked++

	if nodesChecked&2048 == 0 {
		if timeHandler.TimeStatus() {
			searchShouldStop = true
		}
	}

	if GlobalStop || searchShouldStop {
		return 0
	}

	/* INIT KEY VARIABLES */
	var bestMove dragontoothmg.Move
	var childPVLine = PVLine{}
	var isPVNode = (beta - alpha) > 1
	var isRoot = depth == 0

	if isPVNode {
		pvNodesCount++
	} else {
		nonPVNodesCount++
	}
	posHash := b.Hash()
	// Draw check
	if HistoryMap[posHash] == 3 {
		return 0
	}

	inCheck := b.OurKingInCheck()

	// Generate legal moves
	allMoves := b.GenerateLegalMoves()

	// CHECKMATE
	if len(allMoves) == 0 {
		if inCheck {
			return MinScore + int16(ply) // Checkmate
		} else {
			return 0 // Draw ...
		}
	}

	// Make sure we extend our search by 1, if we're in check (so we don't get stuck / end search while in mate and miss something)
	if inCheck {
		depth++
	}

	// If we reach our target depth, do a quiescene search and return the score
	if depth <= 0 {
		score := quiescence(b, alpha, beta, &childPVLine, 10)
		TT.storeEntry(posHash, depth, bestMove, score, ExactFlag)
		return score
	}
	// Look for moves in the transposition table
	ttEntry := TT.getEntry(posHash)
	_ = isRoot
	usable, ttScore := TT.useEntry(&ttEntry, posHash, depth, alpha, beta)
	if usable && !isRoot && ttScore != UnusableScore && !isPVNode {
		ttNodes++
		return int16(ttScore)
	} else if ttScore != UnusableScore {
		bestMove = ttEntry.Move
		ttMoveCounter++
	}

	staticScore := Evaluation(b, false)
	/*
		RAZORING:
		If we're in a pre-pre-frontier node (3 steps away from the horizon - our maxDepth - and our current evaluation
		is bad as-is, we do a quick quiescence search - and if we don't beat alpha (i.e. fail low), we can return early
		and not search any more moves in this branch
	*/
	if depth < int8(len(RazoringMargins)) && !inCheck && !isPVNode && depth <= 2 {
		staticFutilityPruneScore := RazoringMargins[depth] * 3
		if staticScore+staticFutilityPruneScore < int(alpha) {
			razoringCounter += 1
			return alpha
		}
	}

	/*
		STATIC NULL MOVE PRUNING:
		If we raise beta even after giving a large penalty to our score, we're most likely going to fail high regardless
		So we return
	*/

	if !inCheck && !isPVNode && beta < MaxScore-50 {
		//staticScore := quiescence(b, alpha, beta, &childPVLine, 10)
		var staticFutilityPruneScore int16 = 15 * int16(depth)
		if int16(staticScore)-staticFutilityPruneScore >= beta {
			staticNullMovePruneCount++
			return beta //staticScore - staticFutilityPruneScore
		}
	}

	/*
		FUTILITY PRUNING:
		If we're 1 step away from our max depth (a 'frontier node'), we make a static evaluation.
		If (evaluation + minor piece margin) does not beat alpha, we'll most likely fail low.
		Basically, we're assuming we're down so much material, or that our pieces are in such awful positions its not worth doing
		a quiet move here.

		We will instead only check tactical moves (i.e. capture, check or promotion)
		in order to make sure we don't miss anything important.
	*/

	var futilityPruning = false
	if !inCheck && !isPVNode && int(depth) < len(FutilityMargins) {
		futilityPruneScore := FutilityMargins[depth]
		if int16(staticScore)+futilityPruneScore <= alpha {
			futilityPruning = true
		}
	}

	/*
		NULL MOVE PRUNING:
		If we're in a position where, even if the opponent gets a free move (we do "null move"),
		and we're still above beta, we're most likely in a fail-high node; so we prune.
	*/

	var wCount, bCount = hasMinorOrMajorPiece(b)
	var anyMinorsOrMajors = (wCount > 0) || (bCount > 0)
	if !inCheck && !didNull && anyMinorsOrMajors && !isPVNode && depth > 2 {
		unApplyfunc := b.ApplyNullMove()
		R := 3 + (depth / 6) // Roughly, "you-me-you" order; I think... Maybe I should create a pre-defined index instead?
		score := -alphabeta(b, -beta, -beta+1, (depth - 1 - R), ply, &childPVLine, bestMove, true)
		unApplyfunc()
		if score >= beta {
			nullMovePruneCount += 1
			return beta
		}
	}

	var movesChecked = 0
	var score = MinScore

	/*
		INTERNAL ITERATIVE DEEPENING:
		If we didn't find any move in the transposition table, for the sake of move ordering, it's faster to make a shallow search
		and get the PV move from that
	*/

	if ttEntry.Move == 0000 && depth >= 4 && isPVNode { //&& (isPVNode || ttEntry.Flag == BetaFlag) && depth >= 4 {
		IIDCounter++
		alphabeta(b, -beta, -alpha, (depth - 3), ply, &childPVLine, prevMove, didNull)
		if len(childPVLine.Moves) > 0 {
			bestMove = childPVLine.GetPVMove()
			childPVLine.Clear()
		}
	}
	var bestScore = MinScore

	//var theMoveList = scoreMoves(allMoves, b, depth, alpha, beta, prevMove, *pvLine, bestMove)
	//var theMoveList PairList = SortStruct(b, scoreMoves(allMoves, b, depth, alpha, beta, prevMove, *pvLine, bestMove))

	var moveList moveList = scoreMovesList(b, allMoves, depth, bestMove, prevMove)

	var ttFlag int8 = AlphaFlag
	bestMove = 0000

	for index := uint8(0); index < uint8(len(moveList.moves)); index++ {
		movesChecked += 1

		// Get the next move
		orderNextMove(index, &moveList)
		move := moveList.moves[index].move

		var isCapture bool = dragontoothmg.IsCapture(move, b) // Get capture before moving :)
		unapplyFunc := b.Apply(move)

		inCheck := b.OurKingInCheck()

		ply++
		HistoryMap[posHash]++

		// Tactical moves - if we're capturing, checking or promoting a pawn
		tactical := (isCapture || inCheck || move.Promote() > 0)

		/*
			LATE MOVE PRUNING:
			Assuming our move ordering is good, we're not interested in searching most moves at the bottom of the move ordering
			We're most likely interested in the full depth for the first 1 or 2 moves
		*/
		if depth <= 3 && !isPVNode && !tactical && movesChecked > LateMovePruningMargins[depth] {
			lateMovePruningCounter++
			HistoryMap[posHash]--
			unapplyFunc()
			continue
		}

		if futilityPruning && !tactical && !isPVNode {
			futilityPruningCounter++
			HistoryMap[posHash]--
			unapplyFunc()
			continue
		}

		if movesChecked == 1 { // Check the first move fully, no matter what, so we guarantee ourselves a PV-line
			score = -alphabeta(b, -beta, -alpha, (depth - 1), ply, &childPVLine, move, didNull)
		} else {
			/*
				LATE MOVE REDUCTION:
				Assuming good move ordering, the first move is <most likely> the best move.
				We will therefor spend less time searching moves further down in the move ordering.

				If we didn't already get a beta cut-off (Cut-node), most likely we're in an All-node
				So we want to avoid spending time here, so we do a reduced search hoping it will fail low.
				However, if we manage to raise alpha after doing a reduced search, we will do a full search of that node.
				We validate by doing a reduced search on the rest of the nodes

				Great blog-post that helped me wrap my head around this:
				http://macechess.blogspot.com/2010/08/implementing-late-move-reductions.html
			*/

			var reduct int8 = 0
			if !isPVNode && int(depth) >= LMRDepthLimit && !tactical {
				reduct = int8(LMR[depth][movesChecked])
				lateMoveReductionCounter++
			}

			if reduct > 0 {
				score = -alphabeta(b, -(alpha + 1), -alpha, (depth - 1 - reduct), ply, &childPVLine, move, didNull)
			} else {
				score = -alphabeta(b, -(alpha + 1), -alpha, (depth - 3), ply, &childPVLine, move, didNull)
			}

			if score > alpha && reduct > 0 && !isPVNode {
				score = -alphabeta(b, -(alpha + 1), -alpha, (depth - 1), ply, &childPVLine, move, didNull)
				if score > alpha {
					score = -alphabeta(b, -beta, -alpha, (depth - 1), ply, &childPVLine, move, didNull)
				}
				lateMoveReductionFullSearch++
			} else if score > alpha && score <= beta {
				score = -alphabeta(b, -beta, -alpha, (depth - 1), ply, &childPVLine, move, didNull)
				lateMoveReductionFullSearch++
			}
		}

		if score > bestScore { // Catches both >alpha and >beta, so we always get a move in the TT if this move was the cause of the drop
			bestScore = score
			bestMove = move
		}

		if score >= beta {
			cutNodes += 1
			bestMove = move
			bestScore = beta
			ttFlag = BetaFlag
			if !isCapture {
				InsertKiller(move, depth, &killerMoveTable)
				storeCounter(&counterMove, !b.Wtomove, prevMove, move)
				incrementHistoryScore(b, move, depth)
			}
			HistoryMap[posHash]--
			childPVLine.Clear()
			unapplyFunc()
			break
		} else {
			decrementHistoryScore(b, move)
		}

		if score > alpha {
			alpha = score
			ttFlag = ExactFlag
			pvLine.Update(move, childPVLine)
			if !isCapture {
				incrementHistoryScore(b, move, depth)
			}
		} else {
			decrementHistoryScore(b, move)
		}

		HistoryMap[posHash]--
		unapplyFunc()
		ply--
		childPVLine.Clear()
	}

	TT.storeEntry(posHash, depth, bestMove, bestScore, ttFlag)
	return bestScore
}

func quiescence(b *dragontoothmg.Board, alpha int16, beta int16, pvLine *PVLine, depth int8) int16 {
	nodesChecked++
	quiescenceNodes++

	if nodesChecked&2048 == 0 {
		if timeHandler.TimeStatus() {
			searchShouldStop = true
		}
	}

	if GlobalStop || searchShouldStop {
		return 0
	}

	inCheck := b.OurKingInCheck()
	var childPVLine = PVLine{}

	var standpat int16 = int16(Evaluation(b, false))

	if inCheck {
		depth++
	}

	alpha = int16(Max(int(alpha), int(standpat)))

	if depth <= 0 {
		return standpat
	}

	if alpha >= beta {
		return beta
	}

	var bestScore = alpha

	var moves = b.GenerateLegalMoves()

	var moveList, anyCaptures = scoreMovesListCaptures(b, moves)

	if anyCaptures {
		for index := uint8(0); index < uint8(len(moveList.moves)); index++ {
			orderNextMove(index, &moveList)
			move := moveList.moves[index].move
			if move == 0000 {
				continue
			}

			see := see(b, move, false)
			if see < 0 {
				continue
			}

			unapplyFunc := b.Apply(move)

			eval := -quiescence(b, -beta, -alpha, &childPVLine, depth-1)
			unapplyFunc()

			if eval >= beta {
				cutNodes += 1
				return beta
			}

			if eval > alpha {
				alpha = eval
				bestScore = eval
				pvLine.Update(move, childPVLine)
			}
			childPVLine.Clear()
		}
	} else {
		return bestScore
	}
	return bestScore
}
