package engine

import (
	"time"
	"unsafe"

	gm "chess-engine/goosemg"
)

const (
	// Flags
	AlphaFlag = iota
	BetaFlag
	ExactFlag

	// In MB
	TTSize      = 256
	clusterSize = 4

	// Unusable score
	UnusableScore = -32750
)

type TransTable struct {
	isInitialized bool
	entries       []TTEntry
	clusterCount  uint64
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
	TT.clusterCount = 0
}

func (TT *TransTable) init() {
	// Set up transposition table
	entrySize := uint64(unsafe.Sizeof(TTEntry{}))
	if entrySize == 0 {
		entrySize = 1
	}
	totalBytes := uint64(TTSize) * 1024 * 1024
	clusterBytes := entrySize * clusterSize
	if clusterBytes == 0 {
		clusterBytes = entrySize
	}
	clusterCount := totalBytes / clusterBytes
	if clusterCount == 0 {
		clusterCount = 1
	}
	TT.clusterCount = clusterCount
	TT.entries = make([]TTEntry, TT.clusterCount*clusterSize)
	TT.isInitialized = true
}

func (TT *TransTable) useEntry(ttEntry *TTEntry, hash uint64, depth int8, alpha int16, beta int16, ply int8, excludedMove gm.Move) (usable bool, score int16) {
	score = UnusableScore
	usable = false
	if ttEntry != nil && ttEntry.Hash == hash {
		if excludedMove != 0 && ttEntry.Move == excludedMove {
			return false, score
		}
		if ttEntry.Depth >= depth {
			norm := ttEntry.Score
			if norm > Checkmate {
				norm -= int16(ply)
			} else if norm < -Checkmate {
				norm += int16(ply)
			}
			switch ttEntry.Flag {
			case ExactFlag:
				usable = true
				score = norm
			case AlphaFlag:
				if norm <= alpha {
					usable = true
					score = alpha
				}
			case BetaFlag:
				if norm >= beta {
					usable = true
					score = beta
				}
			}
		}
	}
	return usable, score
}

func (TT *TransTable) getEntry(hash uint64) (entry *TTEntry, found bool) {
	if TT.clusterCount == 0 {
		return nil, false
	}

	clusterIndex := hash % TT.clusterCount
	start := int(clusterIndex * clusterSize)
	for i := 0; i < clusterSize; i++ {
		next := &TT.entries[start+i]
		if next.Hash == hash {
			return next, true
		}
	}
	return nil, false
}

/*
If there's a spot to improve searching and data storing, here is where it'd happen!
This is an "always replace"-approach; I've fiddled with depth comparisons and gotten weird/buggy results
*/
func (TT *TransTable) storeEntry(hash uint64, depth int8, ply int8, move gm.Move, score int16, flag int8) {
	// Create entry
	if TT.clusterCount == 0 {
		return
	}

	clusterIndex := hash % TT.clusterCount
	base := int(clusterIndex * clusterSize)

	// If we have a mate score, we add the ply
	if score > Checkmate {
		score += int16(ply) //MaxScore - int16(depth)
	}
	if score < -Checkmate {
		score -= int16(ply) //= -MaxScore + int16(depth)
	}
	targetIdx := -1

	// Prefer updating existing entry
	for i := 0; i < clusterSize; i++ {
		idx := base + i
		if TT.entries[idx].Hash == hash {
			targetIdx = idx
			break
		}
	}

	// Next look for an empty slot
	if targetIdx == -1 {
		for i := 0; i < clusterSize; i++ {
			idx := base + i
			if TT.entries[idx].Hash == 0 {
				targetIdx = idx
				break
			}
		}
	}

	// Otherwise replace the shallowest entry in the cluster
	if targetIdx == -1 {
		targetIdx = base
		minDepth := TT.entries[base].Depth
		for i := 1; i < clusterSize; i++ {
			idx := base + i
			if TT.entries[idx].Depth < minDepth {
				minDepth = TT.entries[idx].Depth
				targetIdx = idx
			}
		}
	}

	entry := &TT.entries[targetIdx]
	entry.Hash = hash
	entry.Depth = depth
	entry.Move = move
	entry.Flag = flag
	entry.Score = score
}
