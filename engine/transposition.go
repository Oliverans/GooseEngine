package engine

import (
	"time"

	gm "chess-engine/goosemg"
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
	Move  gm.Move
	Score int16
	Flag  int8
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

func (TT *TransTable) useEntry(ttEntry *TTEntry, hash uint64, depth int8, alpha int16, beta int16, ply int8, excludedMove gm.Move) (usable bool, score int16) {
	score = UnusableScore
	usable = false
	if ttEntry != nil && ttEntry.Hash == hash {
		if excludedMove != 0 && ttEntry.Move == excludedMove {
			return false, score
		}
		score = ttEntry.Score
		if ttEntry.Depth >= depth {
			var ttScore = ttEntry.Score

			if score > Checkmate {
				score -= int16(ply) //MaxScore - int16(depth)
			}
			if score < -Checkmate {
				score += int16(ply) //= -MaxScore + int16(depth)
			}

			if ttEntry.Flag == ExactFlag {
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

func (TT *TransTable) getEntry(hash uint64) (entry *TTEntry) {
	return &TT.entries[hash%TT.size]
}

/*
If there's a spot to improve searching and data storing, here is where it'd happen!
This is an "always replace"-approach; I've fiddled with depth comparisons and gotten weird/buggy results
*/
func (TT *TransTable) storeEntry(hash uint64, depth int8, ply int8, move gm.Move, score int16, flag int8) {
	entrySlot := hash % TT.size

	var entry TTEntry
	entry.Hash = hash
	entry.Depth = depth
	entry.Move = move
	entry.Flag = flag

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
