package engine

// Base alpha-beta written during vacation in Japan 2020

import (
	"fmt"
	"os"
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
var FutilityBase int32 = 21
var FutilityScale int32 = 114
var FutilityMaxDepth int8 = 7
var RFPScale int32 = 83
var RFPMaxDepth int8 = 7
var RazoringScale int32 = 155
var RazoringMaxDepth int8 = 3

var AspirationWindowSize int32 = 20

// AspirationMaxFails caps how many times a root search widens its aspiration
// window before giving up and searching full width. Window sizes go
// AspirationWindowSize << 1, << 2, ... so with 40 and a cap of 4 the schedule is
// 40, 80, 160, 320, 640, then full width.
var AspirationMaxFails = 4

// MultiPV controls the number of principal variations searched at the root.
// 1 = standard single-PV search (default, no behavior change).
var MultiPV int = 1

// EvalOutputMode controls eval-only rendering from the search entrypoint.
type EvalOutputMode int

const (
	EvalOutputNone EvalOutputMode = iota
	EvalOutputText
	EvalOutputJSON
)

// SearchLimits describes the independent caps accepted by the UCI go command.
// A zero value means that the corresponding limit is not active.
type SearchLimits struct {
	Depth      uint8
	MoveTimeMs int
	Nodes      uint64
	Infinite   bool
	WTime      int
	BTime      int
	WInc       int
	BInc       int
	MovesToGo  int
}

// =============================================================================
// LMR PARAMETERS
// =============================================================================
var LMRDepthLimit int8 = 2
var LMRMoveLimit = 2
var LMRHistoryBonus = 515
var LMRHistoryMalus = -100
var LMPOffset int = 3
var LMPMaxDepth int8 = 8

var NullMoveMinDepth int8 = 4
var NMMarginBase int32 = 210
var NMMarginDepth int32 = 16
var NullMoveReductionBase int8 = 3
var NullMoveReductionDepthDivisor int8 = 4

var SingularMinDepth int8 = 8
var SingularTTDepthSlack int8 = 3
var SingularMarginBase int32 = 50
var SingularMarginDepth int32 = 10
var SingularReductionBase int8 = 3
var SingularReductionDepthDivisor int8 = 4

// =============================================================================
// QSEARCH PARAMETERS
// =============================================================================
var ProbCutSeeMargin int = 140
var ProbCutMinDepth int8 = 5
var ProbCutBetaMargin int32 = 200
var ProbCutReduction int8 = 4
var ProbCutMaxCaptures = 10
var DeltaMargin int32 = 210
var QuiescenceSeeMargin int = 150

func StartSearch(board *gm.Board, limits SearchLimits, evalOnly bool, moveOrderingOnly bool, printSearchInformation bool) string {
	evalMode := EvalOutputNone
	if evalOnly {
		evalMode = EvalOutputText
	}
	return StartSearchWithEvalMode(board, limits, evalMode, moveOrderingOnly, printSearchInformation)
}

func StartSearchWithEvalMode(board *gm.Board, limits SearchLimits, evalMode EvalOutputMode, moveOrderingOnly bool, printSearchInformation bool) string {
	initVariables(board)
	defer SearchState.ClearStop()

	//Stat reset
	SearchState.ResetForSearch(board)

	if !SearchState.tt.isInitialized {
		SearchState.tt.init()
	}

	SearchState.timeHandler.initTimemanagement(limits, board.FullmoveNumber(), board.Wtomove)
	SearchState.timeHandler.StartTime(board.FullmoveNumber(), board.Wtomove)

	depth := limits.Depth
	depthLimited := depth > 0
	if depth == 0 || depth > uint8(MaxDepth) {
		depth = uint8(MaxDepth)
	}

	var bestMove gm.Move

	if evalMode != EvalOutputNone {
		_, trace := EvaluateWithTrace(board)
		if evalMode == EvalOutputJSON {
			_ = RenderEvalTraceJSON(os.Stdout, trace)
		} else {
			_ = RenderEvalTraceText(os.Stdout, trace)
		}
		return ""
	}

	if moveOrderingOnly {
		dumpRootMoveOrdering(board)
		return ""
	}

	_, bestMove = rootsearch(board, depth, depthLimited, printSearchInformation)

	if PrintCutStats {
		dumpCutStats()
		PrintCutStats = false
	}

	if bestMove == 0 {
		return "0000"
	}
	return bestMove.String()
}

func rootsearch(b *gm.Board, depth uint8, depthLimited bool, printSearchInformation bool) (int, gm.Move) {
	var timeSpent int64
	rootIndex := len(SearchState.stateStack) - 1
	var nullMove gm.Move

	// Determine active multi-PV count: clamp by configured value and number
	// of legal root moves. Note: rootsearch is invoked on positions with at
	// least one legal move (caller responsibility), but we still defend.
	activeMultiPV := MultiPV
	if activeMultiPV < 1 {
		activeMultiPV = 1
	}
	rootMoves := b.GenerateLegalMoves()
	legalCount := len(rootMoves)
	if legalCount > 0 && activeMultiPV > legalCount {
		activeMultiPV = legalCount
	}
	if activeMultiPV < 1 {
		activeMultiPV = 1
	}

	// Per-slot state, indexed 0..activeMultiPV-1.
	prevPVLines := make([]PVLine, activeMultiPV)
	prevScores := make([]int32, activeMultiPV)
	completed := make([]bool, activeMultiPV)
	for k := range prevScores {
		prevScores[k] = -MaxScore
	}

	for i := uint8(1); i <= depth; i++ {
		if i > 1 && !SearchState.timeHandler.ShouldStartNextIteration(SearchState.nodesChecked) {
			break
		}
		SearchState.selDepth = 0

		// Per-depth scratch: only commit to prev* once all slots at this
		// depth complete (atomic-per-depth multi-PV update).
		depthPVLines := make([]PVLine, activeMultiPV)
		depthScores := make([]int32, activeMultiPV)
		depthCompleted := make([]bool, activeMultiPV)

		// Reset root exclusion list at the start of each depth.
		SearchState.rootExcludedMoves = SearchState.rootExcludedMoves[:0]

		stoppedMidDepth := false

		for pvIdx := 0; pvIdx < activeMultiPV; pvIdx++ {
			var alpha, beta int32

			// Aspiration window. Start narrow around the previous iteration's
			// score; each failure widens by a further doubling of the tuned base
			// (AspirationWindowSize << failures) and re-searches. Only the bound
			// that actually failed moves: on a fail high alpha is still a valid
			// lower bound, so widening it would cost nodes for nothing.
			delta := AspirationWindowSize
			failures := 0
			fullWidth := !(i >= 5 && completed[pvIdx] && prevScores[pvIdx] > -MaxScore)

			if fullWidth {
				alpha = -MaxScore
				beta = MaxScore
			} else {
				alpha = prevScores[pvIdx] - delta
				beta = prevScores[pvIdx] + delta
			}

			for {
				var pvLine PVLine

				startTime := time.Now()
				score := alphabeta(b, alpha, beta, int8(i), 0, &pvLine, nullMove, false, false, 0, rootIndex)
				timeSpent += time.Since(startTime).Milliseconds()

				if SearchState.ShouldStopRoot() {
					stoppedMidDepth = true
					// Preserve a partial PV only if this slot has no prior
					// completed result (matches single-PV legacy behavior).
					if len(prevPVLines[pvIdx].Moves) == 0 && len(pvLine.Moves) > 0 {
						prevScores[pvIdx] = score
						prevPVLines[pvIdx] = pvLine.Clone()
						completed[pvIdx] = true
					}
					break
				}

				// A full-width search cannot fail, so it is always accepted.
				if !fullWidth && (score <= alpha || score >= beta) {
					failures++
					// Counted here rather than at the widening below, so the
					// mate-score and max-failures escapes are still tallied.
					if score <= alpha {
						SearchState.cutStats.AspirationFailLow++
					} else {
						SearchState.cutStats.AspirationFailHigh++
					}
					// Mate scores gain nothing from a narrow window, and after a
					// few failures the guess is not worth refining any further.
					if abs32(score) >= Checkmate || failures > AspirationMaxFails {
						alpha = -MaxScore
						beta = MaxScore
						fullWidth = true
						continue
					}

					// Only the bound that failed moves. Stockfish additionally
					// pulls beta to (alpha+beta)/2 on a fail low; measured here
					// that lost ~4% overall and roughly half of it again on
					// score-volatile positions, because the lowered beta then
					// provokes a fail high. Left out deliberately.
					delta = AspirationWindowSize << failures
					if score <= alpha {
						alpha = Max32(score-delta, -MaxScore)
					} else {
						beta = Min32(score+delta, MaxScore)
					}
					continue
				}

				depthPVLines[pvIdx] = pvLine.Clone()
				depthScores[pvIdx] = score
				depthCompleted[pvIdx] = true

				// Lock this slot's best move out of subsequent slots.
				if len(pvLine.Moves) > 0 {
					SearchState.rootExcludedMoves = append(SearchState.rootExcludedMoves, pvLine.Moves[0])
				}
				break
			}

			if stoppedMidDepth {
				break
			}
		}

		if stoppedMidDepth {
			break
		}

		// All slots at this depth completed: commit atomically.
		for pvIdx := 0; pvIdx < activeMultiPV; pvIdx++ {
			prevPVLines[pvIdx] = depthPVLines[pvIdx]
			prevScores[pvIdx] = depthScores[pvIdx]
			completed[pvIdx] = depthCompleted[pvIdx]
		}

		if timeSpent == 0 {
			timeSpent = 1
		}
		nps := uint64(float64(SearchState.nodesChecked*1000) / float64(timeSpent))

		// Time-management decisions are driven by PV1 only.
		score1 := prevScores[0]
		pvLine1 := prevPVLines[0]
		if len(pvLine1.Moves) > 0 {
			SearchState.timeHandler.UpdateIteration(score1, uint32(pvLine1.Moves[0]))
		}

		if printSearchInformation {
			emitMultiPV := MultiPV > 1
			hashfull := SearchState.tt.GetHashfull()
			for pvIdx := 0; pvIdx < activeMultiPV; pvIdx++ {
				if !completed[pvIdx] {
					continue
				}
				theMoves := getPVLineString(prevPVLines[pvIdx])
				if emitMultiPV {
					fmt.Println(
						"info depth", i,
						"seldepth", SearchState.selDepth,
						"multipv", pvIdx+1,
						"score", getMateOrCPScore(int(prevScores[pvIdx])),
						"nodes", SearchState.nodesChecked,
						"time", timeSpent,
						"nps", nps,
						"hashfull", hashfull,
						"pv", theMoves,
					)
				} else {
					fmt.Println(
						"info depth", i,
						"seldepth", SearchState.selDepth,
						"score", getMateOrCPScore(int(prevScores[pvIdx])),
						"nodes", SearchState.nodesChecked,
						"time", timeSpent,
						"nps", nps,
						"hashfull", hashfull,
						"pv", theMoves,
					)
				}
			}
		}

		// Stop iterative deepening once PV1 is a forced mate.
		if (score1 > Checkmate || score1 < -Checkmate) && len(pvLine1.Moves) > 0 {
			SearchState.timeHandler.stopReason = "mate"
			break
		}
	}
	if SearchState.timeHandler.stopReason == "" {
		if SearchState.StopRequested() {
			SearchState.timeHandler.stopReason = "external stop"
		} else if depthLimited {
			SearchState.timeHandler.stopReason = "depth"
		} else {
			SearchState.timeHandler.stopReason = "completed depth"
		}
	}

	// Reset globals
	SearchState.searchShouldStop = false
	SearchState.timeHandler.stopSearch = false
	SearchState.rootExcludedMoves = SearchState.rootExcludedMoves[:0]

	SearchState.totalTimeSpent += timeSpent
	var bestMove gm.Move
	if len(prevPVLines[0].Moves) > 0 {
		bestMove = prevPVLines[0].GetPVMove()
	} else if len(rootMoves) > 0 {
		// An asynchronous stop can arrive before depth one has produced a PV.
		// UCI still requires a legal bestmove response in that case.
		bestMove = rootMoves[0]
	}

	return int(prevScores[0]), bestMove
}

func alphabeta(b *gm.Board, alpha int32, beta int32, depth int8, ply int8, pvLine *PVLine, prevMove gm.Move, didNull bool, isExtended bool, excludedMove gm.Move, rootIndex int) int32 {
	SearchState.nodesChecked++
	if ply > SearchState.selDepth {
		SearchState.selDepth = ply
	}

	if SearchState.nodesChecked&4095 == 0 {
		if SearchState.timeHandler.TimeStatus() || SearchState.timeHandler.NodeHardLimitReached(SearchState.nodesChecked) {
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

	// Check extension
	if inCheck {
		depth++
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

	SearchState.cutStats.TTProbes++
	if ttHit {
		SearchState.cutStats.TTHits++
		if usable {
			SearchState.cutStats.TTUsable++
		}
	}

	if usable && !isRoot && !isPVNode {
		SearchState.cutStats.TTCutoffs++
		return ttScore
	}

	if depth <= 0 {
		return quiescence(b, alpha, beta, pvLine, 30, ply, rootIndex)
	}

	var ttMove gm.Move
	if ttHit {
		ttMove = ttEntry.Move
	}

	if ttMove != 0 {
		bestMove = ttMove
	}

	var rawStaticEval int32
	var hasStaticEval bool
	rawStaticEval, hasStaticEval = ttStaticEval(ttEntry, ttHit)
	if !hasStaticEval {
		rawStaticEval = Evaluation(b, false)
	}
	staticScore := rawStaticEval
	storedStaticEval := rawStaticEval

	if inCheck {
		SearchState.evalStack[ply] = -MaxScore // We never aggressively prune checks
		storedStaticEval = -MaxScore
	} else {
		SearchState.evalStack[ply] = staticScore
	}

	// Calculate improving
	improving := true // Default to true (conservative)
	if ply >= 2 && !inCheck {
		if SearchState.evalStack[ply-2] != -MaxScore {
			improving = staticScore > SearchState.evalStack[ply-2]
		}
		// If ply-2 was in check, keep improving = true (conservative)
	}
	/*
		====== REVERSE FUTILITY PRUNING ======
		If the static evaluation minus a safety margin still beats beta,
		we can assume our position is so far above beta that we can prune this branch
	*/
	var wCount, bCount = hasMinorOrMajorPiece(b)
	var sideHasPieces = (b.Wtomove && wCount > 0) || (!b.Wtomove && bCount > 0)
	if !inCheck && !isPVNode && depth <= RFPMaxDepth && depth >= 1 && abs32(beta) < Checkmate && !isRoot {
		SearchState.cutStats.RFPEligible++
		rfpMargin := RFPScale * int32(depth)
		if improving {
			rfpMargin -= 50 // More lenient when improving
		}
		if staticScore-rfpMargin >= beta {
			//SearchState.tt.storeEntry(posHash, depth, ply, ttMove, staticScore-rfpMargin, BetaFlag)
			SearchState.cutStats.RFPCutoffs++
			return staticScore - rfpMargin
		}
	}

	/*
		====== NULL-MOVE PRUNING ======
		If we give the opponent a free move, and we still raise beta even after
		giving our opponent the free move, we can prune this branch
	*/
	var margin int32 = Max32(0, NMMarginBase-NMMarginDepth*int32(depth)) // Margin to only look at positions already risking being beta nodes
	if !inCheck && !isPVNode && !didNull && sideHasPieces && depth >= NullMoveMinDepth && !isRoot && staticScore >= beta-margin {
		SearchState.cutStats.NullMoveAttempts++
		nullState := b.MakeNullMove()
		SearchState.pushState(b)

		var R = NullMoveReductionBase + depth/NullMoveReductionDepthDivisor
		score := -alphabeta(b, -beta, -beta+1, depth-1-R, ply+1, &childPVLine, bestMove, true, isExtended, 0, rootIndex)

		b.UnmakeNullMove(nullState)
		SearchState.popState()

		if score >= beta && score < Checkmate {
			//SearchState.tt.storeEntry(posHash, depth, ply, 0, score, BetaFlag)
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
	if depth <= RazoringMaxDepth && !isPVNode && !inCheck && !isRoot {
		razorMargin := RazoringScale * int32(depth)
		if staticScore+razorMargin < alpha {
			SearchState.cutStats.RazoringAttempts++
			score := quiescence(b, alpha, beta, &childPVLine, 30, ply, rootIndex)
			if score < alpha {
				SearchState.cutStats.RazoringCutoffs++
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
	if !isPVNode && !isRoot && !inCheck && !didNull && !isExtended && depth >= SingularMinDepth && ttMove != 0 && ttBound(ttEntry.Flag) != AlphaFlag && ttEntry.Depth >= depth-SingularTTDepthSlack {
		ttValue := int32(ttEntry.Score)
		if ttValue < Checkmate && ttValue > -Checkmate {
			// Counted here, not at the outer guard: only this branch runs the
			// verification search that the extension is paying for.
			SearchState.cutStats.SingularAttempts++
			margin := SingularMarginBase + SingularMarginDepth*int32(depth)
			scoreToBeat := ttValue - margin
			R := SingularReductionBase + depth/SingularReductionDepthDivisor
			if R > depth-1 {
				R = depth - 1
			}
			var verificationPV PVLine
			scoreSingular := alphabeta(b, scoreToBeat-1, scoreToBeat, depth-1-R, ply, &verificationPV, prevMove, didNull, true, ttMove, rootIndex)
			if scoreSingular < scoreToBeat {
				SearchState.cutStats.SingularHits++
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
	if !inCheck && !isPVNode && depth >= ProbCutMinDepth && abs32(alpha) < Checkmate {
		probCutBeta := beta + ProbCutBetaMargin

		poolIndex := movePickerPoolIndex(ply)
		captures := b.GenerateCapturesInto(qMoveRawPool[poolIndex][:0])
		scoredCaptures, hasCaptures := scoreMovesListTacticals(captures, ply, 0)
		if hasCaptures {
			SearchState.cutStats.ProbCutAttempts++
			maxProbCutCaptures := Min(ProbCutMaxCaptures, len(scoredCaptures.moves)) // TEST; most likely we're

			for i := uint8(0); i < uint8(maxProbCutCaptures); i++ {
				orderNextMove(i, &scoredCaptures)
				move := scoredCaptures.moves[i].move

				if see(b, move, false) < -ProbCutSeeMargin {
					SearchState.cutStats.ProbCutSeeSkips++
					continue
				}

				ok, moveState := b.MakeMove(move)
				if !ok {
					continue
				}
				SearchState.pushState(b)
				SearchState.ContHistPushMove(ply, move)
				SearchState.cutStats.ProbCutMovesSearched++
				qScore := -quiescence(b, -probCutBeta, -probCutBeta+1, &childPVLine, 10, ply+1, rootIndex)

				if qScore >= probCutBeta {
					score := -alphabeta(b, -probCutBeta, -probCutBeta+1, depth-ProbCutReduction, ply+1, &childPVLine, prevMove, didNull, isExtended, excludedMove, rootIndex)
					if score >= probCutBeta {
						b.UnmakeMove(move, moveState)
						SearchState.popState()
						//SearchState.tt.storeEntry(posHash, depth, ply, move, score, BetaFlag)
						SearchState.cutStats.ProbCutCutoffs++
						return score
					}
					// The taken branch returns, so reaching here means qsearch
					// cleared the raised beta but the reduced search did not.
					SearchState.cutStats.ProbCutVerifyFails++
				}
				b.UnmakeMove(move, moveState)
				SearchState.popState()
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

		SearchState.cutStats.IIDCalls++
		var iidPV PVLine
		alphabeta(b, alpha, beta, reducedDepth, ply, &iidPV, prevMove, false, true, 0, rootIndex)

		iidEntry, _ := SearchState.tt.ProbeEntry(posHash)
		if iidEntry.Move != 0 {
			ttMove = iidEntry.Move
			bestMove = ttMove
		}
	}

	var score int32 = -MaxScore
	var bestScore int32 = -MaxScore
	picker := newMovePicker(b, depth, ply, bestMove, prevMove)
	var nodeBound int8 = AlphaFlag
	legalMoves := 0

	quietMovesTried := make([]gm.Move, 0, 16)

	for {
		move, index, hasMove := picker.Next()
		if !hasMove {
			break
		}

		if move == excludedMove {
			continue
		}

		if isRoot && len(SearchState.rootExcludedMoves) > 0 {
			skip := false
			for _, ex := range SearchState.rootExcludedMoves {
				if ex == move {
					skip = true
					break
				}
			}
			if skip {
				continue
			}
		}

		sideIdx := 0
		if !b.Wtomove {
			sideIdx = 1
		}

		isCapture := gm.IsCapture(move, b)
		isPromotion := move.PromotionPieceType() != gm.PieceTypeNone
		moveGivesCheck := false
		if !isCapture && !isPromotion {
			moveGivesCheck = b.GivesCheck(move)
		}

		// Tactical = capture, check, or promotion
		tactical := isCapture || moveGivesCheck || isPromotion
		isQuiet := !isCapture && !isPromotion && !moveGivesCheck

		/*
			====== LATE MOVE PRUNING ======
			Skip quiet moves late in the move list at low depths.
		*/
		if depth <= LMPMaxDepth && !isPVNode && !tactical && !isRoot && legalMoves > 1 {
			lmpMargin := int(depth) * (int(depth) + LMPOffset) / 2
			if !improving {
				lmpMargin = lmpMargin * 2 / 3
			}
			if lmpMargin > 0 && legalMoves > lmpMargin {
				SearchState.cutStats.LateMovePrunes++
				continue
			}
		}

		/*
			====== FUTILITY PRUNING ======
			If we're near the horizon and the static evaluation plus a margin can't beat alpha,
			quiet moves are unlikely to raise the score.
			We skip all quiet moves, assuming only a capture could help
		*/
		if depth <= FutilityMaxDepth && depth >= 1 && !moveGivesCheck && !isPVNode && !isRoot && !tactical && abs32(alpha) < Checkmate && legalMoves >= 1 {
			futilityMargin := FutilityBase + FutilityScale*int32(depth)
			if !improving {
				futilityMargin -= 50
			}
			if staticScore+futilityMargin <= alpha {
				SearchState.cutStats.FutilityPrunes++
				continue
			}
		}

		if isQuiet {
			quietMovesTried = append(quietMovesTried, move)
		}

		ok, moveState := b.MakeMove(move)
		if !ok {
			SearchState.cutStats.MakeMoveRejects++
			continue
		}
		SearchState.pushState(b)
		SearchState.ContHistPushMove(ply, move)

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
			moveHistoryScore := HistoryCombinedScore(sideIdx, move, ply)

			var reduct int8 = 0
			if depth >= LMRDepthLimit && legalMoves >= LMRMoveLimit && !moveGivesCheck && !tactical {
				reduct = computeLMRReduction(
					depth, legalMoves, int(index), isPVNode, tactical,
					moveHistoryScore, improving,
					IsKiller(move, ply, &SearchState.killer), extendMove,
				)
				// Nested inside the eligibility test so the extra compare only
				// runs for LMR candidates, and counts actual reductions rather
				// than candidacy: computeLMRReduction may still return 0.
				if reduct > 0 {
					SearchState.cutStats.LMRReduced++
				}
			}

			// Stage 1: Search with (possibly reduced) depth using null window
			nextDepth := calculateSearchDepth(depth-1, reduct, extendMove)
			score = -alphabeta(b, -(alpha + 1), -alpha, nextDepth, ply+1, &childPVLine, move, false, nextExtended, 0, rootIndex)

			// Stage 2: If we had a reduction and score beats alpha, re-search at full depth with null window
			if score > alpha && reduct > 0 {
				SearchState.cutStats.LMRResearched++
				nextDepth = calculateSearchDepth(depth-1, 0, extendMove)
				score = -alphabeta(b, -(alpha + 1), -alpha, nextDepth, ply+1, &childPVLine, move, false, nextExtended, 0, rootIndex)
			}

			// Stage 3: If score is within window (alpha, beta), do full window search
			if score > alpha && score < beta {
				nextDepth = calculateSearchDepth(depth-1, 0, extendMove)
				score = -alphabeta(b, -beta, -alpha, nextDepth, ply+1, &childPVLine, move, false, nextExtended, 0, rootIndex)
			}
		}

		b.UnmakeMove(move, moveState)
		SearchState.popState()

		if score > bestScore {
			bestScore = score
			bestMove = move
		}

		if score >= beta {
			SearchState.cutStats.BetaCutoffs++
			// legalMoves is at least 1 here, having been incremented before the
			// search that produced this score. Bucket 3 absorbs move 4 onward.
			SearchState.cutStats.BetaCutoffByMove[Min(legalMoves, 4)-1]++
			nodeBound = BetaFlag
			if isQuiet {
				InsertKiller(move, ply, &SearchState.killer)
				HistoryUpdateAllGood(b.Wtomove, move, prevMove, ply, depth)

				for _, failedMove := range quietMovesTried {
					if failedMove != move {
						HistoryUpdateAllBad(b.Wtomove, failedMove, ply, depth)
					}
				}
			}
			break
		}

		if score > alpha {
			alpha = score
			nodeBound = ExactFlag
			pvLine.Update(move, childPVLine)

			if isQuiet {
				HistoryUpdateGood(b.Wtomove, move, depth)
			}
		}
		childPVLine.Clear()
	}

	// The loop leaves only by exhaustion or by break, so legalMoves is the final
	// count on every path.
	SearchState.cutStats.MovesSearched += uint64(legalMoves)
	if legalMoves == 0 && !picker.hasMoves {
		if inCheck {
			return -MaxScore + int32(ply)
		}
		return DrawScore
	}

	if !SearchState.ShouldStopNoClock() {
		SearchState.tt.storeEntry(posHash, depth, ply, bestMove, bestScore, storedStaticEval, nodeBound, false)
	}

	return bestScore
}

func quiescence(b *gm.Board, alpha int32, beta int32, pvLine *PVLine, depth int8, ply int8, rootIndex int) int32 {
	pvLine.Clear()
	SearchState.nodesChecked++
	SearchState.cutStats.QNodes++
	if ply > SearchState.selDepth {
		SearchState.selDepth = ply
	}

	if SearchState.nodesChecked&2047 == 0 {
		if SearchState.timeHandler.TimeStatus() || SearchState.timeHandler.NodeHardLimitReached(SearchState.nodesChecked) {
			SearchState.searchShouldStop = true
		}
	}

	if SearchState.ShouldStopNoClock() {
		return 0
	}

	inCheck := b.OurKingInCheck()
	if ply >= MaxDepth {
		if inCheck {
			return DrawScore
		}
		return Evaluation(b, false)
	}

	isPVNode := beta-alpha > 1
	posHash := b.Hash()
	ttEntry, ttHit := SearchState.tt.ProbeEntry(posHash)
	usable, ttScore := SearchState.tt.useEntry(ttEntry, posHash, 0, alpha, beta, ply, 0)
	SearchState.cutStats.QTTProbes++
	if ttHit {
		SearchState.cutStats.QTTHits++
	}
	if usable && !isPVNode {
		SearchState.cutStats.QTTCutoffs++
		return ttScore
	}

	var ttMove gm.Move
	if ttHit {
		ttMove = ttEntry.Move
	}
	originalAlpha := alpha
	var childPVLine = PVLine{}

	bestScore := -MaxScore + int32(ply)
	rawStaticEval := int32(-MaxScore)
	standpat := rawStaticEval
	if !inCheck {
		var hasStaticEval bool
		rawStaticEval, hasStaticEval = ttStaticEval(ttEntry, ttHit)
		if !hasStaticEval {
			rawStaticEval = Evaluation(b, false)
		}
		standpat = rawStaticEval
		if ttHit {
			switch ttBound(ttEntry.Flag) {
			case ExactFlag:
				standpat = ttScore
			case BetaFlag:
				if ttScore > standpat {
					standpat = ttScore
				}
			case AlphaFlag:
				if ttScore < standpat {
					standpat = ttScore
				}
			}
		}
		bestScore = standpat
		if standpat >= beta {
			SearchState.cutStats.QStandPatCutoffs++
			if !SearchState.ShouldStopNoClock() {
				SearchState.tt.storeEntry(posHash, 0, ply, 0, standpat, rawStaticEval, BetaFlag, false)
			}
			return standpat
		}
		if standpat > alpha {
			alpha = standpat
		}
	}

	poolIndex := movePickerPoolIndex(ply)
	var moveList moveList
	if inCheck {
		moves := b.GenerateMovesInto(qMoveRawPool[poolIndex][:0])
		moveList = scoreMovesListInto(b, moves, 0, ply, ttMove, 0, qMoveListPool[poolIndex][:len(moves)])
	} else {
		moves := b.GenerateTacticalsInto(qMoveRawPool[poolIndex][:0])
		moveList, _ = scoreMovesListTacticals(moves, ply, ttMove)
	}
	var bestMove gm.Move

	for index := uint8(0); index < uint8(len(moveList.moves)); index++ {
		orderNextMove(index, &moveList)
		move := moveList.moves[index].move

		/*
			====== DELTA PRUNING ======
			If the capture + a margin still can't beat alpha, skip it.
			Only apply when not in check.
		*/
		if !inCheck && abs32(alpha) < Checkmate {
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

		ok, moveState := b.MakeMove(move)
		if !ok {
			continue
		}
		SearchState.pushState(b)
		SearchState.ContHistPushMove(ply, move)

		score := -quiescence(b, -beta, -alpha, &childPVLine, depth-1, ply+1, rootIndex)

		b.UnmakeMove(move, moveState)
		SearchState.popState()

		if score > bestScore {
			bestScore = score
		}

		if score >= beta {
			SearchState.cutStats.QBetaCutoffs++
			bestMove = move
			if !SearchState.ShouldStopNoClock() {
				SearchState.tt.storeEntry(posHash, 0, ply, bestMove, score, rawStaticEval, BetaFlag, false)
			}
			return score // Return score, not beta (more accurate)
		}

		if score > alpha {
			alpha = score
			bestMove = move
			pvLine.Update(move, childPVLine)
		}
		childPVLine.Clear()
	}

	nodeBound := AlphaFlag
	if bestScore > originalAlpha {
		nodeBound = ExactFlag
	}
	if !SearchState.ShouldStopNoClock() {
		SearchState.tt.storeEntry(posHash, 0, ply, bestMove, bestScore, rawStaticEval, nodeBound, false)
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
