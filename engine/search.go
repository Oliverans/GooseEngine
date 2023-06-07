package engine

import (
	"fmt"
	"math"
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
	200, // depth 4
	220, // depth 5
	250, // depth 6
	270, // depth 7
	290, // depth 8
	320, // depth 9
}

var RazoringMargins = [10]int{
	0,   // depth 0
	140, // depth 1
	170, // depth 2
	200, // depth 3
	230, // depth 4
	250, // depth 5
	270, // depth 6
	290, // depth 7
	330, // depth 8
	350, // depth 9
}

var ReverseFutilityMargins = [10]int{
	0,
	200, // depth 0
	225, // depth 1
	225, // depth 2
	250, // depth 3
	275, // depth 4
	300, // depth 5
	350, // depth 6
	375, // depth 7
	400, // depth 8
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
var aspiratinoWindowSize int16 = 40

var lateMovePruningCounter = 0
var lateMoveReductionCounter = 0
var lateMoveReductionFullSearch = 0
var futilityPruningCounter = 0
var razoringCounter = 0
var nullMovePruneCount = 0
var IIDCounter = 0
var ttMoveCounter = 0

var HistoryMap map[uint64]int = make(map[uint64]int)

var TT TransTable
var prevSearchScore int16 = 0
var timeHandler TimeHandler
var GlobalStop = false

func StartSearch(b *dragontoothmg.Board, depth int, gameTime int, increment int, useCustomDepth bool, evalOnly bool) string {
	for i := 0; i <= 64; i++ {
		PositionBB[i] = uint64(math.Pow(float64(2), float64(i)))
		sqBB := PositionBB[i]

		// Generate king moves lookup table.

		top := sqBB >> 8
		topRight := (sqBB >> 8 >> 1) & ^bitboardFileH
		topLeft := (sqBB >> 8 << 1) & ^bitboardFileA

		right := (sqBB >> 1) & ^bitboardFileH
		left := (sqBB << 1) & ^bitboardFileA

		bottom := sqBB << 8
		bottomRight := (sqBB << 8 >> 1) & ^bitboardFileH
		bottomLeft := (sqBB << 8 << 1) & ^bitboardFileA

		kingMoves := top | topRight | topLeft | right | left | bottom | bottomRight | bottomLeft

		KingMoves[i] = kingMoves
	}

	if evalOnly {
		Evaluation(b, true)
		println(isTheoreticalDraw(b, true))
		os.Exit(0)
	}

	InitSearchTables()

	//var tempMove dragontoothmg.Move
	//tempMove.Setfrom(dragontoothmg.Square(17))
	//tempMove.Setto(dragontoothmg.Square(41))
	//see(b, tempMove, true)
	//os.Exit(0)

	// Set values of pieces based on UCI variable input
	for _, pieceType := range pieceList {
		switch pieceType {
		case dragontoothmg.Pawn:
			pieceValueMG[pieceType] = PawnValueMG
			pieceValueEG[pieceType] = PawnValueEG
		case dragontoothmg.Knight:
			pieceValueMG[pieceType] = KnightValueMG
			pieceValueEG[pieceType] = KnightValueEG
		case dragontoothmg.Bishop:
			pieceValueMG[pieceType] = BishopValueMG
			pieceValueEG[pieceType] = BishopValueMG
		case dragontoothmg.Rook:
			pieceValueMG[pieceType] = RookValueMG
			pieceValueEG[pieceType] = RookValueEG
		case dragontoothmg.Queen:
			pieceValueMG[pieceType] = QueenValueMG
			pieceValueEG[pieceType] = QueenValueEG
		}
	}

	GlobalStop = false
	if !timeHandler.isInitialized {
		timeHandler.initTimemanagement(gameTime, increment, int(b.Halfmoveclock), useCustomDepth)
		timeHandler.StartTime(int(b.Halfmoveclock))
	} else {
		timeHandler.initTimemanagement(gameTime, increment, int(b.Halfmoveclock), useCustomDepth)
		timeHandler.StartTime(int(b.Halfmoveclock))
	}

	//TT.Unitialize()
	//TT.Clear()

	//entries := 0
	//for _, entry := range TT.entries {
	//	if entry.Flag == AlphaFlag || entry.Flag == BetaFlag || entry.Flag == ExactFlag {
	//		entries += 1
	//	}
	//}

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

	killerMoveTable.KillerMoves = make(map[int8]map[int]dragontoothmg.Move)
	for i := 0; i <= depth*4; i++ {
		killerMoveTable.KillerMoves[int8(i)] = make(map[int]dragontoothmg.Move)
	}

	if !TT.isInitialized {
		TT.init()
	}

	var bestMove dragontoothmg.Move

	_, bestMove = rootsearch(b, depth, useCustomDepth)

	//NodesChecked = 0

	GlobalStop = false

	TTNodes = 0
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
			alpha = int16(MinScore)
			beta = MaxScore
			i--
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
		fmt.Println("info depth ", i, "\tscore ", getMateOrCPScore(int(score)), "\tnodes ", nodesChecked, "\ttime ", timeSpent, "\tnps ", nps, "\tpv", theMoves)
	}

	otherTime := (InitSearchTime.Milliseconds() + NullTime.Milliseconds() + QuiescenceTime.Milliseconds() + seeTime.Milliseconds() + SearchTime.Milliseconds() + TotalEvalTime.Milliseconds() + MoveSortingTime.Milliseconds()) / 10

	fmt.Println("Total search time: \t", timeSpent)
	fmt.Println("Total other search:\t", otherTime)
	fmt.Println("Init search time: \t", InitSearchTime.Milliseconds()/10)
	fmt.Println("Null search time: \t", NullTime.Milliseconds()/10)
	fmt.Println("QSearch time: \t\t", QuiescenceTime.Milliseconds()/10)
	fmt.Println("SeeTime: \t\t", seeTime.Milliseconds()/10)
	fmt.Println("Alpha-Beta time: \t", SearchTime.Milliseconds()/10)
	fmt.Println("Eval time: \t\t", TotalEvalTime.Milliseconds()/10)
	fmt.Println("Move sorting time: \t", MoveSortingTime.Milliseconds()/10)

	println("TT Moves:\t", ttMoveCounter)
	println("TT Nodes:\t", TTNodes)
	println("Cut:\t\t", cutNodes)
	println("LMP:\t\t", lateMovePruningCounter)
	println("LMR:\t\t", lateMoveReductionCounter)
	println("LMFS:\t\t", lateMoveReductionFullSearch)
	println("Futility:\t", futilityPruningCounter)
	println("Null:\t\t", nullMovePruneCount)
	println("Razoring:\t", razoringCounter)
	println("Quiescence:\t", quiescenceNodes)
	println("IID:\t\t", IIDCounter)
	println("TT Move:\t", ttMoveCounter)
	println("Total:\t\t", nodesChecked)

	TTNodes = 0
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
	TT.clearTT()
	return int(bestScore), bestMove
}

var InitSearchTime time.Duration
var NullTime time.Duration

var pvNodesCount int
var nonPVNodesCount int

func alphabeta(b *dragontoothmg.Board, alpha int16, beta int16, depth int8, ply int8, pvLine *PVLine, prevMove dragontoothmg.Move, didNull bool) int16 {
	var tmpTime = time.Now()
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
	if HistoryMap[posHash] >= 3 {
		return 0 // Draw
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
		quieTime := time.Now()
		score := quiescence(b, alpha, beta, &childPVLine, 10)
		TT.storeEntry(posHash, depth, bestMove, score, ExactFlag)
		QuiescenceTime += time.Since(quieTime)
		return score
	}
	// Look for moves in the transposition table
	ttEntry := TT.getEntry(posHash)
	_ = isRoot
	usable, ttScore := TT.useEntry(ttEntry, posHash, depth, alpha, beta)
	if usable && !isRoot && ttScore != UnusableScore && !isPVNode {
		TTNodes += 1
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
	if depth < int8(len(FutilityMargins)) && !inCheck && !isPVNode && depth < 3 {
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
		quieTime := time.Now()
		staticScore := quiescence(b, alpha, beta, &childPVLine, 10)
		QuiescenceTime += time.Since(quieTime)
		var staticFutilityPruneScore int16 = 65 * int16(depth)
		if staticScore-staticFutilityPruneScore >= beta {
			razoringCounter += 1
			return staticScore - staticFutilityPruneScore
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

	var nullSearchTime = time.Now()
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
	NullTime += time.Since(nullSearchTime)

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

	sortTime := time.Now()
	var theMoveList PairList = SortStruct(b, SortMoves(allMoves, b, depth, alpha, beta, prevMove, *pvLine, bestMove))
	MoveSortingTime += time.Since(sortTime)
	//if futilityPruning {
	//	theMoveList = SortStruct(b, SortCapturesOnly(b.GenerateLegalMoves(), b))
	//} else {
	//	theMoveList = SortStruct(b, SortMoves(allMoves, b, depth, alpha, beta, prevMove, *pvLine, bestMove))
	//}

	var ttFlag int8 = AlphaFlag
	bestMove = 0000
	InitSearchTime += time.Since(tmpTime)

	for _, moveList := range theMoveList {
		movesChecked += 1

		var isCapture = dragontoothmg.IsCapture(moveList.Key, b) // Get capture before moving :)
		unapplyFunc := b.Apply(moveList.Key)
		inCheck := b.OurKingInCheck()

		ply++
		posHash := b.Hash()
		HistoryMap[posHash]++

		// Tactical moves - if we're capturing, checking or promoting a pawn
		tactical := (isCapture || inCheck || moveList.Key.Promote() > 0)

		/*
			LATE MOVE PRUNING:
			Assuming our move ordering is good, we're not interested in searching most moves at the bottom of the move ordering
			We're most likely interested in the full depth for the first 1 or 2 moves
		*/
		if depth <= 5 && !isPVNode && !tactical && movesChecked > LateMovePruningMargins[depth] {
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

		if movesChecked <= 1 { // Check the first move fully, no matter what, so we guarantee ourselves a PV-line
			score = -alphabeta(b, -beta, -alpha, (depth - 1), ply, &childPVLine, moveList.Key, didNull)
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
				score = -alphabeta(b, -(alpha + 1), -alpha, (depth - 1 - reduct), ply, &childPVLine, moveList.Key, didNull)
			} else {
				score = -alphabeta(b, -(alpha + 1), -alpha, (depth - 2), ply, &childPVLine, moveList.Key, didNull)
			}

			if score > alpha && reduct > 0 && !isPVNode {
				score = -alphabeta(b, -(alpha + 1), -alpha, (depth - 1), ply, &childPVLine, moveList.Key, didNull)
				if score > alpha {
					score = -alphabeta(b, -beta, -alpha, (depth - 1), ply, &childPVLine, moveList.Key, didNull)
				}
				lateMoveReductionFullSearch++
			} else if score > alpha && score <= beta {
				score = -alphabeta(b, -beta, -alpha, (depth - 1), ply, &childPVLine, moveList.Key, didNull)
				lateMoveReductionFullSearch++
			}
		}

		if score > bestScore { // Catches both >alpha and >beta, so we always get a move in the TT if this move was the cause of the drop
			bestScore = score
			bestMove = moveList.Key
		}

		if score >= beta {
			cutNodes += 1
			bestMove = moveList.Key
			bestScore = beta
			ttFlag = BetaFlag
			if !isCapture {
				InsertKiller(moveList.Key, depth, killerMoveTable)
				storeCounter(&counterMove, !b.Wtomove, prevMove, moveList.Key)
				incrementHistoryScore(b, moveList.Key, depth)
			}
			HistoryMap[posHash]--
			childPVLine.Clear()
			unapplyFunc()
			break
		} else {
			decrementHistoryScore(b, moveList.Key)
		}

		if score > alpha {
			alpha = score
			ttFlag = ExactFlag
			pvLine.Update(moveList.Key, childPVLine)
			if !isCapture {
				incrementHistoryScore(b, moveList.Key, depth)
			}
		} else {
			decrementHistoryScore(b, moveList.Key)
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

	tmpTime := time.Now()
	var movePair PairList = SortStruct(b, SortCapturesOnly(b.GenerateLegalMoves(), b))
	MoveSortingTime += time.Since(tmpTime)
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

	for _, moveList := range movePair {
		unapplyFunc := b.Apply(moveList.Key)

		tmpTime := time.Now()
		see := see(b, moveList.Key, false)
		seeTime += time.Since(tmpTime)
		if see < 0 {
			continue
		}

		eval := -quiescence(b, -beta, -alpha, &childPVLine, depth-1)
		unapplyFunc()

		if eval >= beta {
			cutNodes += 1
			return beta
		}

		if eval > alpha {
			alpha = eval
			bestScore = eval
			pvLine.Update(moveList.Key, childPVLine)
		}
		childPVLine.Clear()
	}
	return bestScore
}
