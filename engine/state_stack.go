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

// ResetStateTracking rebuilds the state stack so that it only contains the current board.
func (s *searchState) ResetStateTracking(board *gm.Board) {
	s.stateStack = s.stateStack[:0]
	s.pushState(board)
}

// RecordState appends the board's current state to the history stack.
func (s *searchState) RecordState(board *gm.Board) {
	s.pushState(board)
}

// ensureStateStackSynced guarantees that the top of the stack reflects the board position.
func (s *searchState) ensureStateStackSynced(board *gm.Board) {
	if len(s.stateStack) == 0 {
		s.pushState(board)
		return
	}
	last := &s.stateStack[len(s.stateStack)-1]
	if last.Hash != board.Hash() {
		s.ResetStateTracking(board)
		return
	}
	last.Rule50 = board.HalfmoveClock()
}

func (s *searchState) pushState(board *gm.Board) {
	s.stateStack = append(s.stateStack, State{
		Hash:   board.Hash(),
		Rule50: board.HalfmoveClock(),
	})
}

func (s *searchState) popState() {
	if len(s.stateStack) == 0 {
		return
	}
	s.stateStack = s.stateStack[:len(s.stateStack)-1]
}

func (s *searchState) isDraw(ply int, rootIndex int) bool {
	if len(s.stateStack) == 0 {
		return false
	}
	curr := s.stateStack[len(s.stateStack)-1]
	if curr.Rule50 >= fiftyMoveLimit {
		return true
	}

	matchCount, firstIdx := s.repetitionInfo(curr.Hash, curr.Rule50)
	if matchCount >= 2 {
		return true
	}
	return matchCount >= 1 && firstIdx >= rootIndex && firstIdx != -1
}

func (s *searchState) upcomingRepetition(ply int, rootIndex int) bool {
	if len(s.stateStack) <= 1 {
		return false
	}
	curr := s.stateStack[len(s.stateStack)-1]
	start := len(s.stateStack) - 1 - curr.Rule50
	if start < 0 {
		start = 0
	}
	for i := len(s.stateStack) - 2; i >= start; i-- {
		if s.stateStack[i].Hash == curr.Hash && i >= rootIndex {
			return true
		}
	}
	return false
}

func (s *searchState) repetitionInfo(hash uint64, rule50 int) (count int, firstIdx int) {
	firstIdx = -1
	if len(s.stateStack) <= 1 {
		return 0, firstIdx
	}
	start := len(s.stateStack) - 1 - rule50
	if start < 0 {
		start = 0
	}
	end := len(s.stateStack) - 2
	for i := start; i <= end; i++ {
		if s.stateStack[i].Hash == hash {
			count++
			if firstIdx == -1 {
				firstIdx = i
			}
		}
	}
	return count, firstIdx
}
