package engine

import (
	"time"

	"github.com/dylhunn/dragontoothmg"
)

const (
	// Flags
	AlphaFlag = iota
	BetaFlag
	ExactFlag

	// In MB
	TTSize = 64

	// Unusable score
	UnusableScore = -32500
)

type TransTable struct {
	isInitialized bool
	entries       map[uint64]TTEntry
	size          uint64
}

type TTEntry struct {
	Hash  uint64
	Depth int8
	Move  dragontoothmg.Move
	Score int16
	Flag  int8
}

var TranspositionTime time.Duration

func (TT *TransTable) useEntry(ttEntry TTEntry, hash uint64, depth int8, alpha int16, beta int16) (usable bool, score int16) {
	score = 0
	usable = false
	if ttEntry.Hash == hash {
		var ttScore = ttEntry.Score
		if ttEntry.Depth >= depth {

			if int16(ttScore) > MaxScore-50 {
				ttScore = MaxScore - int16(ttEntry.Depth)
			} else if int16(ttScore) < MinScore+50 {
				ttScore = MinScore + int16(ttEntry.Depth)
			}

			if ttEntry.Flag == ExactFlag {
				score = ttScore
				usable = true
			}

			if ttEntry.Flag == AlphaFlag && ttScore <= alpha {
				score = alpha
				usable = true
			}

			if ttEntry.Flag == BetaFlag && ttScore >= beta {
				score = beta
				usable = true
			}

		}
	}
	return usable, score
}

func (TT *TransTable) getEntry(hash uint64) (entry TTEntry) {
	return TT.entries[hash%TT.size]
}

func (TT *TransTable) storeEntry(hash uint64, depth int8, move dragontoothmg.Move, score int16, flag int8) {
	// Create entry
	entrySlot := hash % TT.size
	prevEntry := TT.entries[entrySlot]

	// Replace
	if prevEntry.Depth <= depth || prevEntry.Hash != hash {
		var entry TTEntry
		entry.Hash = hash
		entry.Depth = depth
		entry.Move = move
		entry.Score = score
		entry.Flag = flag
		TT.entries[entrySlot] = entry
	}
}

func (TT *TransTable) clearTT() {
	TT.entries = nil
	TT.isInitialized = false
}

func (TT *TransTable) init() {
	// Set up transposition table
	TT.entries = make(map[uint64]TTEntry, ((TTSize * 1024 * 1024) / TTSize))
	TT.size = ((TTSize * 1024 * 1024) / TTSize)
	TT.isInitialized = true
}
