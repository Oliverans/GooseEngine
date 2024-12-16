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
	TTSize = 256

	// Unusable score
	UnusableScore = -32500

	// Max moves ago before we simply replace an entry
	ageLimit = 15
)

type TransTable struct {
	isInitialized bool
	entries       []TTEntry
	size          uint64
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

func (TT *TransTable) clearTT() {
	TT.entries = nil
	TT.isInitialized = false
	//TT.init()
}

func (TT *TransTable) init() {
	// Set up transposition table
	TT.size = (TTSize * 1024 * 1024) / 16
	TT.entries = make([]TTEntry, TT.size)
	TT.isInitialized = true
}

func (TT *TransTable) useEntry(ttEntry *TTEntry, hash uint64, depth int8, alpha int16, beta int16) (usable bool, score int16) {
	score = UnusableScore
	usable = false
	if ttEntry.Hash == hash {
		score = ttEntry.Score
		if ttEntry.Depth > depth {
			var ttScore = ttEntry.Score

			if ttEntry.Flag == ExactFlag {
				score = ttScore
				usable = true
			}

			if ttEntry.Flag == AlphaFlag && ttScore < alpha {
				score = alpha
				usable = true
			}

			if ttEntry.Flag == BetaFlag && ttScore > beta {
				score = beta
				usable = true
			}

		}
	}
	return usable, score
}

func (TT *TransTable) getEntry(hash uint64) (entry *TTEntry) {
	return &TT.entries[hash%TT.size]
}

/*
	If there's a spot to improve searching and data storing, here is where it'd happen!
	This is an "always replace"-approach; I've fiddled with depth comparisons and gotten weird/buggy results

*/
func (TT *TransTable) storeEntry(hash uint64, depth int8, ply int8, move dragontoothmg.Move, score int16, flag int8, age uint8) {
	// Create entry
	entrySlot := hash % TT.size

	// Check if we got an entry already
	//prevEntry := TT.entries[entrySlot]
	//shouldReplace := prevEntry.Depth < depth // || prevEntry.Age+ageLimit < age

	// Replace
	//if shouldReplace && move != 0000 && flag == ExactFlag {
	//if depth < prevEntry.Depth { // || move == 0 {
	//	return
	//}

	if score == 0 {
		return
	}

	var entry TTEntry
	entry.Hash = hash
	entry.Depth = depth
	entry.Move = move
	entry.Flag = flag
	entry.Age = age

	// If we have a mate score, we add the ply
	if score > Checkmate {
		score += int16(ply) //MaxScore - int16(depth)
	}
	if score < -Checkmate {
		score -= int16(ply) //= -MaxScore + int16(depth)
	}
	entry.Score = score
	TT.entries[entrySlot] = entry
	//}
}
