package engine

import (
	gm "chess-engine/goosemg"
)

const fiftyMoveLimit = 100

// State captures the information we need to reason about repetitions and draws.
type State struct {
	Hash   uint64
	Rule50 int
}

var stateStack []State

// ResetStateTracking rebuilds the state stack so that it only contains the current board.
func ResetStateTracking(board *gm.Board) {
	stateStack = stateStack[:0]
	pushState(board)
}

// RecordState appends the board's current state to the history stack.
func RecordState(board *gm.Board) {
	pushState(board)
}

// ensureStateStackSynced guarantees that the top of the stack reflects the board position.
func ensureStateStackSynced(board *gm.Board) {
	if len(stateStack) == 0 {
		pushState(board)
		return
	}
	last := &stateStack[len(stateStack)-1]
	if last.Hash != board.Hash() {
		ResetStateTracking(board)
		return
	}
	last.Rule50 = board.HalfmoveClock()
}

func pushState(board *gm.Board) {
	stateStack = append(stateStack, State{
		Hash:   board.Hash(),
		Rule50: board.HalfmoveClock(),
	})
}

func popState() {
	if len(stateStack) == 0 {
		return
	}
	stateStack = stateStack[:len(stateStack)-1]
}

func isDraw(ply int, rootIndex int) bool {
	if len(stateStack) == 0 {
		return false
	}
	curr := stateStack[len(stateStack)-1]
	if curr.Rule50 >= fiftyMoveLimit {
		return true
	}

	matchCount, firstIdx := repetitionInfo(curr.Hash, curr.Rule50)
	if matchCount >= 2 {
		return true
	}
	return matchCount >= 1 && firstIdx >= rootIndex && firstIdx != -1
}

func upcomingRepetition(ply int, rootIndex int) bool {
	if len(stateStack) <= 1 {
		return false
	}
	curr := stateStack[len(stateStack)-1]
	start := len(stateStack) - 1 - curr.Rule50
	if start < 0 {
		start = 0
	}
	for i := len(stateStack) - 2; i >= start; i-- {
		if stateStack[i].Hash == curr.Hash && i >= rootIndex {
			return true
		}
	}
	return false
}

func repetitionInfo(hash uint64, rule50 int) (count int, firstIdx int) {
	firstIdx = -1
	if len(stateStack) <= 1 {
		return 0, firstIdx
	}
	start := len(stateStack) - 1 - rule50
	if start < 0 {
		start = 0
	}
	end := len(stateStack) - 2
	for i := start; i <= end; i++ {
		if stateStack[i].Hash == hash {
			count++
			if firstIdx == -1 {
				firstIdx = i
			}
		}
	}
	return count, firstIdx
}
