package engine

import (
	"fmt"
	"math/bits"

	gm "chess-engine/goosemg"
)

// =============================================================================
// TYPES & CONSTANTS
// =============================================================================

// Precomputed reductions
const MaxDepth int8 = 100

var LMR = [MaxDepth + 1][100]int8{}
var historyMaxVal = 8000 // Cap to prevent overflow, triggers aging

// To keep track of 3-fold repetition and/or 50 move draw
// (legacy: kept for UCI position tracking helpers)
type HistoryStruct struct {
	History             []uint64
	HalfclockRepetition int
}

type KillerStruct struct {
	KillerMoves [MaxDepth + 1][2]gm.Move
}

var KillerMoveLength = 2
var KillerMoveScore = 10

// =============================================================================
// SEARCH STATE
// =============================================================================

// searchState centralizes lifecycle operations around search state.
// It is currently a thin wrapper around existing globals.
type searchState struct {
	nodesChecked    int
	totalTimeSpent  int64
	cutStats        CutStatistics
	stateStack      []State
	killer          KillerStruct
	counterMoves    [2][64][64]gm.Move
	historyMoves    [2][64][64]int
	prevSearchScore int32
	searchShouldStop bool
	GlobalStop       bool
	tt              TransTable
	timeHandler     TimeHandler
}

// SearchState is the package-level instance used by the engine.
var SearchState = &searchState{}

// =============================================================================
// LIFECYCLE & STOP CONTROL
// =============================================================================

// ResetForNewGame clears all game-long state (TT, history, killers, counters, etc.).
func (s *searchState) ResetForNewGame() {
	ResetForNewGame()
}

// SyncPositionState rebuilds position-tracking state for a new root position.
func (s *searchState) SyncPositionState(board *gm.Board) {
	SearchState.ResetStateTracking(board)
}

// ResetForSearch performs per-search initialization.
func (s *searchState) ResetForSearch(board *gm.Board) {
	SearchState.ensureStateStackSynced(board)
}

// RequestStop signals an external stop (e.g. UCI stop command).
func (s *searchState) RequestStop() {
	s.GlobalStop = true
}

// ClearStop clears any external stop request.
func (s *searchState) ClearStop() {
	s.GlobalStop = false
}

// ShouldStopRoot returns true when the current search should stop,
// including time-based termination checks.
func (s *searchState) ShouldStopRoot() bool {
	return s.searchShouldStop || s.GlobalStop || s.timeHandler.stopSearch || s.timeHandler.TimeStatus()
}

// ShouldStopNoClock returns true when the search should stop without polling the clock.
func (s *searchState) ShouldStopNoClock() bool {
	return s.searchShouldStop || s.GlobalStop || s.timeHandler.stopSearch
}

// UpdateBetweenSearches performs post-search maintenance/aging.
func (s *searchState) UpdateBetweenSearches() {
	UpdateBetweenSearches()
}

func UpdateBetweenSearches() {
	AgeHistory()        // Age history
	ResetNodesChecked() // Reset nodes checked
	ResetCutStats()     // Reset cut statistics
	//ClearKillers(&SearchState.killer)
	SearchState.tt.NewSearch() // Increment TT for aging
}

func ResetForNewGame() {
	SearchState.tt.clearTT()
	SearchState.tt.NewSearch()
	ClearPawnHash()
	ClearKillers(&SearchState.killer)
	ClearHistoryTable()
	SearchState.stateStack = SearchState.stateStack[:0]
	var nilMove gm.Move
	for i := 0; i < 64; i++ {
		for z := 0; z < 64; z++ {
			SearchState.counterMoves[0][i][z] = nilMove
			SearchState.counterMoves[1][i][z] = nilMove
		}
	}

	SearchState.prevSearchScore = 0
	SearchState.searchShouldStop = false
	SearchState.GlobalStop = false
	SearchState.nodesChecked = 0
	SearchState.totalTimeSpent = 0
}

// =============================================================================
// MOVE ORDERING STATE (KILLERS / HISTORY / COUNTERS)
// =============================================================================

func InsertKiller(move gm.Move, ply int8, k *KillerStruct) {
	index := int(ply)
	if index >= len(k.KillerMoves) {
		index = len(k.KillerMoves) - 1
	}
	if move != k.KillerMoves[index][0] {
		k.KillerMoves[index][1] = k.KillerMoves[index][0]
		k.KillerMoves[index][0] = move
	}
}

func ClearKillers(k *KillerStruct) {
	for i := range k.KillerMoves {
		k.KillerMoves[i][0] = 0
		k.KillerMoves[i][1] = 0
	}
}

/*
HISTORY/COUNTER MOVES
If a move was a cut-node (above beta), and not a capture, we keep track of two things:
The move that countered us (previous move made) - a counter move
A historical score of the move - since we know it was a good move to keep track of, we use this for move ordering
*/
func storeCounter(sideToMove bool, prevMove gm.Move, move gm.Move) {
	if prevMove == 0 {
		return
	}
	from := gm.Square(prevMove.From())
	to := gm.Square(prevMove.To())
	if sideToMove {
		SearchState.counterMoves[0][from][to] = move
	} else {
		SearchState.counterMoves[1][from][to] = move
	}
}

// Increment the history score for the given move if it caused a beta-cutoff and is quiet.
// OPTIMIZATION: Use depth^2 bonus, common in modern engines
func incrementHistoryScore(sideToMove bool, move gm.Move, depth int8) {
	sideIdx := 0
	if !sideToMove {
		sideIdx = 1
	}

	bonus := int(depth) * int(depth)
	currentVal := SearchState.historyMoves[sideIdx][move.From()][move.To()]
	bonus = bonus - currentVal*bonus/historyMaxVal

	SearchState.historyMoves[sideIdx][move.From()][move.To()] += bonus

	if SearchState.historyMoves[sideIdx][move.From()][move.To()] >= historyMaxVal {
		AgeHistory()
	}
}

func decrementHistoryScoreBy(sideToMove bool, move gm.Move, depth int8) {
	sideIdx := 0
	if !sideToMove {
		sideIdx = 1
	}

	malus := int(depth) * int(depth)
	currentVal := SearchState.historyMoves[sideIdx][move.From()][move.To()]
	malus = malus + currentVal*malus/historyMaxVal

	SearchState.historyMoves[sideIdx][move.From()][move.To()] -= malus

	if SearchState.historyMoves[sideIdx][move.From()][move.To()] <= -historyMaxVal {
		SearchState.historyMoves[sideIdx][move.From()][move.To()] = -historyMaxVal
		AgeHistory()
	}
}

func AgeHistory() {
	for side := 0; side < 2; side++ {
		for from := 0; from < 64; from++ {
			for to := 0; to < 64; to++ {
				SearchState.historyMoves[side][from][to] /= 2
			}
		}
	}
	// Also age counter move history if you have it
}

// Clear the values in the history table.
func ClearHistoryTable() {
	for sq1 := 0; sq1 < 64; sq1++ {
		for sq2 := 0; sq2 < 64; sq2++ {
			SearchState.historyMoves[0][sq1][sq2] = 0
			SearchState.historyMoves[1][sq1][sq2] = 0
		}
	}
}

// =============================================================================
// LMR REDUCTIONS
// =============================================================================

// OPTIMIZATION: Improved LMR reduction computation
// Uses the precomputed LMR table with dynamic adjustments
// Consolidates all reduction heuristics in one place for maintainability
func computeLMRReduction(depth int8, legalMoves int, moveIdx int, isPVNode bool, tactical bool,
	historyScore int, improving bool, isKiller bool, extendMove bool) int8 {
	// No reduction for tactical moves or very shallow depths
	if tactical || depth < 2 {
		return 0
	}

	// Clamp indices into LMR table bounds
	d := Max(1, Min(int(depth), int(MaxDepth)))
	m := Max(1, Min(legalMoves, 99))

	// Get base reduction from precomputed table
	r := LMR[d][m]

	// PV nodes get less reduction
	if isPVNode {
		r--
	}

	// History-based adjustments
	// Good history: reduce less (this move has been good before)
	if historyScore > LMRHistoryBonus {
		r--
	}
	if historyScore > LMRHistoryBonus*2 {
		r--
	}

	// Bad history: reduce more (this move has failed before)
	if historyScore < LMRHistoryMalus {
		r++
	}

	// Very late moves can be reduced more aggressively
	if legalMoves > 10 && !isPVNode {
		r++
	}

	// Additional heuristics from search.go consolidated here:

	// PV nodes get additional reduction bonus
	if isPVNode && r > 0 {
		r--
	}

	// Improving positions deserve less reduction
	if improving && r > 1 {
		r--
	}

	// Killer moves are promising, reduce less
	if isKiller && r > 0 {
		r--
	}

	// If we're extending this move, reduce the reduction
	if extendMove && r > 0 {
		r--
	}

	// Ensure bounds
	if r < 0 {
		r = 0
	}
	if r > depth-2 {
		r = depth - 2
	}

	return r
}

// =============================================================================
// SEARCH STATS
// =============================================================================

func GetNodeCount() int {
	return SearchState.nodesChecked
}

func GetTimeSpent() int64 {
	return SearchState.totalTimeSpent
}

func ResetNodesChecked() {
	SearchState.nodesChecked = 0
	SearchState.totalTimeSpent = 0
}

// =============================================================================
// SUPPORT HELPERS
// =============================================================================

func hasMinorOrMajorPiece(b *gm.Board) (wCount int, bCount int) {
	wCount = bits.OnesCount64(b.White.Bishops | b.White.Knights | b.White.Rooks | b.White.Queens)
	bCount = bits.OnesCount64(b.Black.Bishops | b.Black.Knights | b.Black.Rooks | b.Black.Queens)
	return wCount, bCount
}

func getPVLineString(pvLine PVLine) (theMoves string) {
	for _, move := range pvLine.Moves {
		theMoves += " "
		theMoves += move.String()
	}
	return theMoves
}

// Taken from Blunder chess engine and slightly modified
func getMateOrCPScore(score int) string {
	mateValue := int(MaxScore)
	mateThreshold := int(Checkmate)

	if score >= mateThreshold {
		pliesToMate := mateValue - score
		if pliesToMate < 0 {
			pliesToMate = 0
		}
		mateInN := (pliesToMate + 1) / 2
		return fmt.Sprintf("mate %d", mateInN)
	} else if score <= -mateThreshold {
		pliesToMate := mateValue + score
		if pliesToMate < 0 {
			pliesToMate = 0
		}
		mateInN := (pliesToMate + 1) / 2
		return fmt.Sprintf("mate %d", -mateInN)
	}

	return fmt.Sprintf("cp %d", score)
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
