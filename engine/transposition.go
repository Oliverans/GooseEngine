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
	UnusableScore int16 = -32500
)

type TransTable struct {
	isInitialized bool
	entries       map[uint64]TTEntry
	size          uint64
	GlobalAge     uint8
}

type TTEntry struct {
	Hash  uint64
	Depth int8
	Move  dragontoothmg.Move
	Score int16
	Flag  int8
	Age   uint8
}

var TranspositionTime time.Duration

func (TT *TransTable) useEntry(ttEntry *TTEntry, hash uint64, depth int8, alpha int16, beta int16) (usable bool, score int16) {
	score = 0
	usable = false
	if ttEntry.Hash == hash {
		var ttScore = ttEntry.Score
		if ttEntry.Depth >= depth {

			if ttScore > MaxScore-50 {
				ttScore = MaxScore - int16(ttEntry.Depth)
			} else if ttScore < MinScore+50 {
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
	index := hash % TT.size
	firstEntry := TT.entries[index]

	if index+1 == TT.size {
		return firstEntry
	}
	if firstEntry.Hash == hash {
		return firstEntry
	}
	return TT.entries[index+1]
}

func (TT *TransTable) storeEntry(hash uint64, depth int8, move dragontoothmg.Move, score int16, flag int8) {
	// Create entry
	entrySlot := hash % TT.size
	prevEntry := TT.entries[entrySlot]

	/*
		TWO BUCKET; the "extra bucket" is just allowing for multiple storings of the same hash, so we don't necessarily
		have to replace another entry.
		The storing of "multiple entries" for a hash, is simply just entry-slot increments; +1 for each. Or in our case,
		we'll just use two, so we use a "two-bucket". It could increase and become more complex.
	*/

	// If we hit the length of the map, we can't store another entry.
	var entry TTEntry
	entry.Hash = hash
	entry.Depth = depth
	entry.Move = move
	entry.Score = score
	entry.Flag = flag

	// Then we just store it.
	if entrySlot+1 == TT.size {
		TT.entries[entrySlot] = entry
	}

	// Check whether we should store the new one
	if prevEntry.Depth < depth {
		TT.entries[entrySlot] = entry
	} else { // Otherwise we store it in our "other bucket" ##### TWO BUCKET OMG
		TT.entries[entrySlot+1] = entry
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
