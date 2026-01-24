package engine

import (
	"fmt"
	"math/bits"

	gm "chess-engine/goosemg"
)

var nodesChecked = 0
var totalTimeSpent int64 = 0
var LMR = [MaxDepth + 1][100]int8{}
var counterMove [2][64][64]gm.Move
var historyMove [2][64][64]int
var historyMaxVal = 8000 // Cap to prevent overflow, triggers aging

// Clamp helper function
func Clamp(f, low, high int8) int8 {
	if f < low {
		return low
	}
	if f > high {
		return high
	}
	return f
}

// To keep track of 3-fold repetition and/or 50 move draw
type HistoryStruct struct {
	History             []uint64
	HalfclockRepetition int
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
		counterMove[0][from][to] = move
	} else {
		counterMove[1][from][to] = move
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
	currentVal := historyMove[sideIdx][move.From()][move.To()]
	bonus = bonus - currentVal*bonus/historyMaxVal

	historyMove[sideIdx][move.From()][move.To()] += bonus

	if historyMove[sideIdx][move.From()][move.To()] >= historyMaxVal {
		AgeHistory()
	}
}

func decrementHistoryScoreBy(sideToMove bool, move gm.Move, depth int8) {
	sideIdx := 0
	if !sideToMove {
		sideIdx = 1
	}

	malus := int(depth) * int(depth)
	currentVal := historyMove[sideIdx][move.From()][move.To()]
	malus = malus + currentVal*malus/historyMaxVal

	historyMove[sideIdx][move.From()][move.To()] -= malus

	if historyMove[sideIdx][move.From()][move.To()] <= -historyMaxVal {
		historyMove[sideIdx][move.From()][move.To()] = -historyMaxVal
		AgeHistory()
	}
}

func AgeHistory() {
	for side := 0; side < 2; side++ {
		for from := 0; from < 64; from++ {
			for to := 0; to < 64; to++ {
				historyMove[side][from][to] /= 2
			}
		}
	}
	// Also age counter move history if you have it
}

// Clear the values in the history table.
func ClearHistoryTable() {
	for sq1 := 0; sq1 < 64; sq1++ {
		for sq2 := 0; sq2 < 64; sq2++ {
			historyMove[0][sq1][sq2] = 0
			historyMove[1][sq1][sq2] = 0
		}
	}
}

func Min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

func Max(x, y int) int {
	if x > y {
		return x
	}
	return y
}

func hasMinorOrMajorPiece(b *gm.Board) (wCount int, bCount int) {
	wCount = bits.OnesCount64(b.White.Bishops | b.White.Knights | b.White.Rooks | b.White.Queens)
	bCount = bits.OnesCount64(b.Black.Bishops | b.Black.Knights | b.Black.Rooks | b.Black.Queens)
	return wCount, bCount
}

// Precomputed reductions
const MaxDepth int8 = 100

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

func UpdateBetweenSearches() {
	AgeHistory()        // Age history
	ResetNodesChecked() // Reset nodes checked
	ResetCutStats()     // Reset cut statistics
	//ClearKillers(&KillerMoveTable)
	TT.NewSearch() // Increment TT for aging
}

func ResetForNewGame() {
	TT.clearTT()
	TT.NewSearch()
	ClearPawnHash()
	ClearKillers(&KillerMoveTable)
	ClearHistoryTable()
	stateStack = stateStack[:0]
	var nilMove gm.Move
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
	prevSearchScore = 0
	nodesChecked = 0
	totalTimeSpent = 0
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

func abs32(x int32) int32 {
	if x < 0 {
		return -x
	}
	return x
}

func GetNodeCount() int {
	return nodesChecked
}

func GetTimeSpent() int64 {
	return totalTimeSpent
}

func ResetNodesChecked() {
	nodesChecked = 0
	totalTimeSpent = 0
}
