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
	ageLimit = 10
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

func (TT *TransTable) useEntry(ttEntry *TTEntry, hash uint64, depth int8, ply int8, alpha int16, beta int16, age uint8) (bool, int16) {
	if ttEntry.Hash != hash || ttEntry.Depth < depth {
		return false, UnusableScore
	}

	// de-normalize
	s := ttEntry.Score
	if s > Checkmate {
		s -= int16(ply)
	} else if s < -Checkmate {
		s += int16(ply)
	}
	isMate := s > Checkmate || s < -Checkmate

	switch ttEntry.Flag {
	case ExactFlag:
		return true, s

	case AlphaFlag: // upper bound (score <= s)
		if s <= alpha {
			if isMate {
				return true, s // keep exact mate distance
			}
			return true, alpha // style: edge for non-mates
		}

	case BetaFlag: // lower bound (score >= s)
		if s >= beta {
			if isMate {
				return true, s // keep exact mate distance
			}
			return true, beta // style: edge for non-mates
		}
	}
	return false, UnusableScore
}

/*
	This returning method works for a "single-bucket" transposition table implementation, like Always-Replace or Depth-Preferred
*/
func (TT *TransTable) getEntry(hash uint64) (entry *TTEntry) {
	return &TT.entries[hash%TT.size]
}

/*
	If there's a spot to improve searching and data storing, here is where it'd happen!
	This is an "always replace"-approach; I've fiddled with depth comparisons and gotten weird/buggy results
	UPDATE: now should be depth-preferred

*/
func (TT *TransTable) storeEntry(hash uint64, depth int8, ply int8,
	move dragontoothmg.Move, score int16, flag int8, age uint8) {

	slot := hash % TT.size
	cur := TT.entries[slot]

	// normalize mate scores exactly like before
	if score > Checkmate {
		score += int16(ply)
	} else if score < -Checkmate {
		score -= int16(ply)
	}

	newEntry := TTEntry{
		Hash:  hash,
		Depth: depth,
		Move:  move, // keep storing moves for Alpha too, per your preference
		Score: score,
		Flag:  flag, // not used for replacement here
		Age:   age,  // not used for replacement here
	}

	replace := false
	if cur.Hash == 0 {
		// empty slot
		replace = true
	} else if cur.Hash == hash {
		// same position: prefer deeper, and allow equal-depth refresh
		if depth >= cur.Depth {
			replace = true
		}
	} else {
		// collision: only replace if strictly deeper
		if depth > cur.Depth {
			replace = true
		}
	}

	if replace {
		TT.entries[slot] = newEntry
	}
}

//func (TT *TransTable) storeEntry(hash uint64, depth int8, ply int8, move dragontoothmg.Move, score int16, flag int8, age uint8) {
//	// Create entry
//	entrySlot := hash % TT.size
//
//	var entry TTEntry
//	entry.Hash = hash
//	entry.Depth = depth
//	entry.Move = move
//	entry.Flag = flag
//	entry.Age = age
//
//	if score > Checkmate { // positive mate score
//		score += int16(ply)
//	}
//	if score < -Checkmate { // negative mate score
//		score -= int16(ply)
//	}
//
//	entry.Score = score
//
//	TT.entries[entrySlot] = entry
//}
