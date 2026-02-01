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
// CONTINUATION HISTORY
// =============================================================================

// Continuation history tracks move sequences that work well together.
// contHist1Ply[side][prevPiece][prevTo][currPiece][currTo] - 1-ply continuation
// contHist2Ply[side][prevPiece][prevTo][currPiece][currTo] - 2-ply continuation
//
// Piece types are 0-5 (Pawn=0, Knight=1, Bishop=2, Rook=3, Queen=4, King=5)
// This captures patterns like "after Nf3, Bg5 is often good"

const contHistMax = 8000

// ContHistEntry holds the context needed to update continuation history
type ContHistEntry struct {
	Piece int8 // 0-5 piece type
	To    int8 // 0-63 destination square
	Valid bool // Whether this entry has valid data
}

// ContHistEntryFromMove extracts continuation history context from a move
func ContHistEntryFromMove(move gm.Move) ContHistEntry {
	if move == 0 {
		return ContHistEntry{Valid: false}
	}
	pieceType := int8(move.MovedPiece().Type() - 1) // Convert to 0-5
	if pieceType < 0 || pieceType > 5 {
		return ContHistEntry{Valid: false}
	}
	return ContHistEntry{
		Piece: pieceType,
		To:    int8(move.To()),
		Valid: true,
	}
}

// ContHistScore returns the continuation history score for a move
// given the previous moves (1-ply and 2-ply back)
func ContHistScore(side int, currMove gm.Move, prev1Ply, prev2Ply ContHistEntry) int {
	if currMove == 0 {
		return 0
	}

	currPiece := int(currMove.MovedPiece().Type() - 1)
	if currPiece < 0 || currPiece > 5 {
		return 0
	}
	currTo := int(currMove.To())

	score := 0

	// 1-ply continuation (opponent's last move -> our move)
	if prev1Ply.Valid {
		score += int(SearchState.contHist1Ply[side][prev1Ply.Piece][prev1Ply.To][currPiece][currTo])
	}

	// 2-ply continuation (our previous move -> our current move)
	if prev2Ply.Valid {
		score += int(SearchState.contHist2Ply[side][prev2Ply.Piece][prev2Ply.To][currPiece][currTo])
	}

	return score
}

// ContHistUpdateGood updates continuation history for a move that caused beta cutoff
func ContHistUpdateGood(side int, currMove gm.Move, prev1Ply, prev2Ply ContHistEntry, depth int8) {
	if currMove == 0 {
		return
	}

	currPiece := int(currMove.MovedPiece().Type() - 1)
	if currPiece < 0 || currPiece > 5 {
		return
	}
	currTo := int(currMove.To())

	bonus := int(depth) * int(depth)

	// Update 1-ply continuation
	if prev1Ply.Valid {
		current := int(SearchState.contHist1Ply[side][prev1Ply.Piece][prev1Ply.To][currPiece][currTo])
		// Gravity formula: bonus decreases as value approaches max
		adjustedBonus := bonus - current*bonus/contHistMax
		newVal := current + adjustedBonus
		if newVal > contHistMax {
			newVal = contHistMax
		}
		if newVal < -contHistMax {
			newVal = -contHistMax
		}
		SearchState.contHist1Ply[side][prev1Ply.Piece][prev1Ply.To][currPiece][currTo] = int16(newVal)
	}

	// Update 2-ply continuation
	if prev2Ply.Valid {
		current := int(SearchState.contHist2Ply[side][prev2Ply.Piece][prev2Ply.To][currPiece][currTo])
		adjustedBonus := bonus - current*bonus/contHistMax
		newVal := current + adjustedBonus
		if newVal > contHistMax {
			newVal = contHistMax
		}
		if newVal < -contHistMax {
			newVal = -contHistMax
		}
		SearchState.contHist2Ply[side][prev2Ply.Piece][prev2Ply.To][currPiece][currTo] = int16(newVal)
	}
}

// ContHistUpdateBad updates continuation history for moves that didn't cause cutoff
func ContHistUpdateBad(side int, currMove gm.Move, prev1Ply, prev2Ply ContHistEntry, depth int8) {
	if currMove == 0 {
		return
	}

	currPiece := int(currMove.MovedPiece().Type() - 1)
	if currPiece < 0 || currPiece > 5 {
		return
	}
	currTo := int(currMove.To())

	malus := int(depth) * int(depth)

	// Update 1-ply continuation with penalty
	if prev1Ply.Valid {
		current := int(SearchState.contHist1Ply[side][prev1Ply.Piece][prev1Ply.To][currPiece][currTo])
		adjustedMalus := malus + current*malus/contHistMax
		newVal := current - adjustedMalus
		if newVal < -contHistMax {
			newVal = -contHistMax
		}
		SearchState.contHist1Ply[side][prev1Ply.Piece][prev1Ply.To][currPiece][currTo] = int16(newVal)
	}

	// Update 2-ply continuation with penalty
	if prev2Ply.Valid {
		current := int(SearchState.contHist2Ply[side][prev2Ply.Piece][prev2Ply.To][currPiece][currTo])
		adjustedMalus := malus + current*malus/contHistMax
		newVal := current - adjustedMalus
		if newVal < -contHistMax {
			newVal = -contHistMax
		}
		SearchState.contHist2Ply[side][prev2Ply.Piece][prev2Ply.To][currPiece][currTo] = int16(newVal)
	}
}

// ContHistClear resets all continuation history tables
func ContHistClear() {
	for side := 0; side < 2; side++ {
		for p1 := 0; p1 < 6; p1++ {
			for t1 := 0; t1 < 64; t1++ {
				for p2 := 0; p2 < 6; p2++ {
					for t2 := 0; t2 < 64; t2++ {
						SearchState.contHist1Ply[side][p1][t1][p2][t2] = 0
						SearchState.contHist2Ply[side][p1][t1][p2][t2] = 0
					}
				}
			}
		}
	}
}

// ContHistAge halves all continuation history values
func ContHistAge() {
	for side := 0; side < 2; side++ {
		for p1 := 0; p1 < 6; p1++ {
			for t1 := 0; t1 < 64; t1++ {
				for p2 := 0; p2 < 6; p2++ {
					for t2 := 0; t2 < 64; t2++ {
						SearchState.contHist1Ply[side][p1][t1][p2][t2] /= 2
						SearchState.contHist2Ply[side][p1][t1][p2][t2] /= 2
					}
				}
			}
		}
	}
}

// =============================================================================
// SEARCH STATE
// =============================================================================

// searchState centralizes lifecycle operations around search state.
type searchState struct {
	nodesChecked     int
	totalTimeSpent   int64
	cutStats         CutStatistics
	stateStack       []State
	killer           KillerStruct
	counterMoves     [2][64][64]gm.Move
	historyMoves     [2][64][64]int
	evalStack        [MaxDepth]int32
	prevSearchScore  int32
	searchShouldStop bool
	GlobalStop       bool
	tt               TransTable
	timeHandler      TimeHandler

	// Move stack for continuation history lookups
	moveStack [MaxDepth + 4]gm.Move

	// Continuation history tables (1-ply and 2-ply)
	contHist1Ply [2][6][64][6][64]int16
	contHist2Ply [2][6][64][6][64]int16
}

// SearchState is the package-level instance used by the engine.
var SearchState = &searchState{}

// ContHistPushMove records a move on the move stack for continuation history
func (s *searchState) ContHistPushMove(ply int8, move gm.Move) {
	if ply >= 0 && ply < MaxDepth {
		s.moveStack[ply+2] = move // +2 offset so we can look back safely
	}
}

// contHistPrevMove returns the move made N plies ago (1 = opponent's last, 2 = our last)
func (s *searchState) contHistPrevMove(ply int8, pliesBack int) gm.Move {
	idx := int(ply) + 2 - pliesBack
	if idx >= 0 && idx < len(s.moveStack) {
		return s.moveStack[idx]
	}
	return 0
}

// ContHistContext returns the continuation history context for move scoring
func (s *searchState) ContHistContext(ply int8) (prev1Ply, prev2Ply ContHistEntry) {
	prev1Ply = ContHistEntryFromMove(s.contHistPrevMove(ply, 1))
	prev2Ply = ContHistEntryFromMove(s.contHistPrevMove(ply, 2))
	return
}

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
	// Clear move stack for new search
	for i := range s.moveStack {
		s.moveStack[i] = 0
	}
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
	HistoryAge()        // Age history
	ContHistAge()       // Age continuation history
	ResetNodesChecked() // Reset nodes checked
	ResetCutStats()     // Reset cut statistics
	SearchState.tt.NewSearch()
}

func ResetForNewGame() {
	SearchState.tt.clearTT()
	SearchState.tt.NewSearch()
	ClearPawnHash()
	ClearKillers(&SearchState.killer)
	HistoryClear()
	ContHistClear()
	SearchState.stateStack = SearchState.stateStack[:0]
	var nilMove gm.Move
	for i := 0; i < 64; i++ {
		for z := 0; z < 64; z++ {
			SearchState.counterMoves[0][i][z] = nilMove
			SearchState.counterMoves[1][i][z] = nilMove
		}
	}
	for i := range SearchState.moveStack {
		SearchState.moveStack[i] = 0
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

func HistoryUpdateGood(sideToMove bool, move gm.Move, depth int8) {
	sideIdx := 0
	if !sideToMove {
		sideIdx = 1
	}

	bonus := int(depth) * int(depth)
	currentVal := SearchState.historyMoves[sideIdx][move.From()][move.To()]
	bonus = bonus - currentVal*bonus/historyMaxVal

	SearchState.historyMoves[sideIdx][move.From()][move.To()] += bonus

	if SearchState.historyMoves[sideIdx][move.From()][move.To()] >= historyMaxVal {
		HistoryAge()
	}
}

func HistoryUpdateBad(sideToMove bool, move gm.Move, depth int8) {
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
		HistoryAge()
	}
}

func HistoryAge() {
	for side := 0; side < 2; side++ {
		for from := 0; from < 64; from++ {
			for to := 0; to < 64; to++ {
				SearchState.historyMoves[side][from][to] /= 2
			}
		}
	}
}

func HistoryClear() {
	for sq1 := 0; sq1 < 64; sq1++ {
		for sq2 := 0; sq2 < 64; sq2++ {
			SearchState.historyMoves[0][sq1][sq2] = 0
			SearchState.historyMoves[1][sq1][sq2] = 0
		}
	}
}

// =============================================================================
// COMBINED HISTORY UPDATE (call on beta cutoff for quiet moves)
// =============================================================================

// HistoryUpdateAllGood updates all history tables when a quiet move causes beta cutoff
func HistoryUpdateAllGood(sideToMove bool, move gm.Move, prevMove gm.Move, ply int8, depth int8) {
	sideIdx := 0
	if !sideToMove {
		sideIdx = 1
	}

	// Update main history
	HistoryUpdateGood(sideToMove, move, depth)

	// Update counter move
	storeCounter(sideToMove, prevMove, move)

	// Update continuation history
	prev1Ply, prev2Ply := SearchState.ContHistContext(ply)
	ContHistUpdateGood(sideIdx, move, prev1Ply, prev2Ply, depth)
}

// HistoryUpdateAllBad updates all history tables for quiet moves that didn't cause cutoff
func HistoryUpdateAllBad(sideToMove bool, move gm.Move, ply int8, depth int8) {
	sideIdx := 0
	if !sideToMove {
		sideIdx = 1
	}

	// Update main history with penalty
	HistoryUpdateBad(sideToMove, move, depth)

	// Update continuation history with penalty
	prev1Ply, prev2Ply := SearchState.ContHistContext(ply)
	ContHistUpdateBad(sideIdx, move, prev1Ply, prev2Ply, depth)
}

// =============================================================================
// LMR REDUCTIONS
// =============================================================================

func computeLMRReduction(depth int8, legalMoves int, moveIdx int, isPVNode bool, tactical bool,
	historyScore int, improving bool, isKiller bool, extendMove bool) int8 {
	if tactical || depth < 2 {
		return 0
	}

	d := Max(1, Min(int(depth), int(MaxDepth)))
	m := Max(1, Min(legalMoves, 99))

	r := LMR[d][m]

	if isPVNode {
		r--
	}

	if historyScore > LMRHistoryBonus {
		r--
	}
	if historyScore > LMRHistoryBonus*2 {
		r--
	}

	if historyScore < LMRHistoryMalus {
		r++
	}

	if legalMoves > 10 && !isPVNode {
		r++
	}

	if isPVNode && r > 0 {
		r--
	}

	if improving && r > 1 {
		r--
	}

	if isKiller && r > 0 {
		r--
	}

	if extendMove && r > 0 {
		r--
	}

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
