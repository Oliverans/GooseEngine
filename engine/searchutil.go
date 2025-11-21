package engine

import (
	"fmt"
	"math/bits"

	gm "chess-engine/goosemg"
)

var nodesChecked = 0
var LMR = [MaxDepth + 1][100]int8{}
var counterMove [2][64][64]gm.Move
var historyMove [2][64][64]int
var historyMaxVal = 10000 // Ensure we stay below the captures, countermoves etc

// God damn it Golang, why do I need to write my own Clamp function :(
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
A historical score of the move - since we know it was a good move to keep track of, we make sure we can use this for move ordering later
*/
func storeCounter(sideToMove bool, prevMove gm.Move, move gm.Move) {
	from := gm.Square(prevMove.From())
	to := gm.Square(prevMove.To())
	if sideToMove {
		counterMove[0][from][to] = move
	} else {
		counterMove[1][from][to] = move
	}
}

// Increment the history score for the given move if it caused a beta-cutoff and is quiet.
func incrementHistoryScore(sideToMove bool, move gm.Move, depth int8) {
	sideIdx := 0
	if !sideToMove {
		sideIdx = 1
	}

	historyMove[sideIdx][move.From()][move.To()] += int(depth * depth)
	if historyMove[sideIdx][move.From()][move.To()] >= historyMaxVal {
		ageHistoryTable(sideToMove)
	}
}

// Decrement the history score for the given move if it didn't cause a beta-cutoff and is quiet.
func decrementHistoryScore(sideToMove bool, move gm.Move) {
	sideIdx := 0
	if !sideToMove {
		sideIdx = 1
	}

	if historyMove[sideIdx][move.From()][move.To()] > 0 {
		historyMove[sideIdx][move.From()][move.To()]--
	}
}

// Age the values in the history table by halving them.
func ageHistoryTable(sideToMove bool) {
	for sq1 := 0; sq1 < 64; sq1++ {
		for sq2 := 0; sq2 < 64; sq2++ {
			if sideToMove {
				historyMove[0][sq1][sq2] /= 2
			} else {
				historyMove[1][sq1][sq2] /= 2
			}
		}
	}
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
	wCount += bits.OnesCount64(b.White.Bishops | b.White.Knights | b.White.Rooks | b.White.Queens)
	bCount += bits.OnesCount64(b.Black.Bishops | b.Black.Knights | b.Black.Rooks | b.Black.Queens)
	return wCount, bCount
}

// Precomputed reductions
const MaxDepth = 100

func getPVLineString(pvLine PVLine) (theMoves string) {
	for _, move := range pvLine.Moves {
		theMoves += " "
		theMoves += move.String()
	}
	return theMoves
}

// Taken from Blunder chess engine and just slightly modified, since I'm very lazy; works great though :)
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
		pliesToMate := mateValue + score // score is negative here
		if pliesToMate < 0 {
			pliesToMate = 0
		}
		mateInN := (pliesToMate + 1) / 2
		return fmt.Sprintf("mate %d", -mateInN)
	}

	return fmt.Sprintf("cp %d", score)
}

func ResetForNewGame() {
	TT.clearTT()
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

func computeLMRReduction(
	depth int8,
	legalMoves int,
	moveIdx int,
	isPVNode bool,
	tactical bool,
	historyScore int,
) int8 {
	// No reduction in these cases
	if isPVNode || tactical || int(depth) < LMRDepthLimit || legalMoves <= 2 {
		return 0
	}

	// Clamp depth index into LMR table
	d := int(depth)
	if d >= len(LMR) {
		d = len(LMR) - 1
	}
	if d < 0 {
		d = 0
	}

	// Prefer using "moves searched" as column rather than raw index
	m := legalMoves - 1
	row := LMR[d]
	if m < 0 {
		m = 0
	}
	if m >= len(row) {
		m = len(row) - 1
	}

	r := row[m]

	// History bonus: good moves get less reduction
	if r > 0 && historyScore > 0 {
		bonus := int8(historyScore / LMRHistoryReductionScale)
		if bonus > 2 {
			bonus = 2
		}
		if bonus > r {
			bonus = r
		}
		r -= bonus
	}

	// Really bad late moves get a bit more reduction
	if historyScore <= LMRHistoryLowThreshold && legalMoves > LMRLegalMovesLimit {
		r++
	}

	if r < 0 {
		r = 0
	}
	return r
}
