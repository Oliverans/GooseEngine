package engine

import (
	gm "chess-engine/goosemg"
)

const (
	// Flags
	AlphaFlag = iota
	BetaFlag
	ExactFlag

	// TT Size in MB
	TTSize = 256

	// Unusable score
	UnusableScore int32 = -32500

	// Number of entries per bucket
	BucketSize = 4
)

// TTEntry represents a single transposition table entry
// Optimized for size: 16 bytes with this layout
type TTEntry struct {
	Hash       uint32  // Upper 32 bits of hash (lower bits are implicit from index)
	Move       gm.Move // Move that caused this position
	Score      int32   // Score from search
	Depth      int8    // Search depth
	Flag       int8    // Alpha/Beta/Exact flag
	Generation uint8   // Which search this entry is from
}

// TTBucket holds multiple entries for the same hash index
// This improves hit rates and reduces destructive collisions
type TTBucket struct {
	Entries [BucketSize]TTEntry
}

// TransTable is the main transposition table structure
type TransTable struct {
	isInitialized bool
	buckets       []TTBucket
	size          uint64 // Number of buckets
	generation    uint8  // Current search generation (incremented each new search)
}

// clearTT resets the transposition table
func (TT *TransTable) clearTT() {
	TT.buckets = nil
	TT.isInitialized = false
	TT.generation = 0
}

// init initializes the transposition table with the configured size
func (TT *TransTable) init() {
	// Calculate number of buckets based on memory size
	// Each bucket is BucketSize * 16 bytes = 32 bytes for BucketSize=2
	bucketBytes := uint64(BucketSize * 16)
	TT.size = (TTSize * 1024 * 1024) / bucketBytes
	TT.buckets = make([]TTBucket, TT.size)
	TT.generation = 0
	TT.isInitialized = true
}

// NewSearch should be called at the start of each new search
// Increments the generation counter to age old entries
func (TT *TransTable) NewSearch() {
	TT.generation++
	// Handle wraparound (unlikely but safe)
	if TT.generation == 0 {
		TT.generation = 1
	}
}

// getEntry looks up an entry in the transposition table
// Returns a pointer to the matching entry, or an empty entry if no match found
// IMPORTANT: Caller should verify the hash matches before using the move
func (TT *TransTable) getEntry(hash uint64) *TTEntry {
	if !TT.isInitialized {
		return &emptyEntry
	}

	bucketIdx := hash % TT.size
	bucket := &TT.buckets[bucketIdx]
	hashHigh := uint32(hash >> 32)

	// Check all entries in the bucket for a match
	for i := 0; i < BucketSize; i++ {
		if bucket.Entries[i].Hash == hashHigh {
			return &bucket.Entries[i]
		}
	}

	// No match found - return empty entry (not a random entry!)
	return &emptyEntry
}

// Empty entry returned when TT is not initialized or no match found
var emptyEntry TTEntry

// ProbeEntry looks up an entry and returns both the entry and whether it matched
// This is the preferred method when you need to know if the entry is valid
func (TT *TransTable) ProbeEntry(hash uint64) (entry *TTEntry, found bool) {
	if !TT.isInitialized {
		return &emptyEntry, false
	}

	bucketIdx := hash % TT.size
	bucket := &TT.buckets[bucketIdx]
	hashHigh := uint32(hash >> 32)

	// Check all entries in the bucket for a match
	for i := 0; i < BucketSize; i++ {
		if bucket.Entries[i].Hash == hashHigh {
			return &bucket.Entries[i], true
		}
	}

	return &emptyEntry, false
}

// useEntry determines if a TT entry can be used to cutoff search
// Returns (usable, score) where usable indicates if we can use this entry
// Note: This assumes the entry was obtained via ProbeEntry and matched
func (TT *TransTable) useEntry(ttEntry *TTEntry, hash uint64, depth int8, alpha int32, beta int32, ply int8, excludedMove gm.Move) (usable bool, score int32) {
	score = UnusableScore
	usable = false

	// Empty entry check
	if ttEntry == nil || ttEntry.Hash == 0 {
		return false, score
	}

	// Verify hash matches (upper 32 bits) - defensive check
	hashHigh := uint32(hash >> 32)
	if ttEntry.Hash != hashHigh {
		return false, score
	}

	// Check if this is the excluded move (for singular extension)
	if excludedMove != 0 && ttEntry.Move == excludedMove {
		return false, score
	}

	// Only use entry if depth is sufficient
	if ttEntry.Depth >= depth {
		score = ttEntry.Score

		// Adjust mate scores relative to current ply
		if score > Checkmate {
			score -= int32(ply)
		}
		if score < -Checkmate {
			score += int32(ply)
		}

		// Check if we can use this entry based on flag
		switch ttEntry.Flag {
		case ExactFlag:
			usable = true
		case AlphaFlag:
			if ttEntry.Score <= alpha {
				score = alpha
				usable = true
			}
		case BetaFlag:
			if ttEntry.Score >= beta {
				score = beta
				usable = true
			}
		}
	}

	return usable, score
}

// storeEntry stores a position in the transposition table
// Uses a scoring system to determine which entry to replace
func (TT *TransTable) storeEntry(hash uint64, depth int8, ply int8, move gm.Move, score int32, flag int8) {
	if !TT.isInitialized {
		return
	}

	bucketIdx := hash % TT.size
	bucket := &TT.buckets[bucketIdx]
	hashHigh := uint32(hash >> 32)

	// Adjust mate scores for storage (make them relative to root)
	if score > Checkmate {
		score += int32(ply)
	}
	if score < -Checkmate {
		score -= int32(ply)
	}

	// First pass: check if position already exists in bucket
	for i := 0; i < BucketSize; i++ {
		if bucket.Entries[i].Hash == hashHigh {
			// Position exists - update it if new info is better or same depth
			// Always update if: same/deeper depth, or entry is from old search
			existing := &bucket.Entries[i]
			if depth >= existing.Depth || existing.Generation != TT.generation {
				existing.Hash = hashHigh
				existing.Move = move
				existing.Score = score
				existing.Depth = depth
				existing.Flag = flag
				existing.Generation = TT.generation
			} else if move != 0 && existing.Move == 0 {
				// At minimum, store the move if we didn't have one
				existing.Move = move
			}
			return
		}
	}

	// Second pass: find the best entry to replace
	replaceIdx := 0
	worstScore := TT.scoreEntryForReplacement(&bucket.Entries[0], depth)

	for i := 1; i < BucketSize; i++ {
		entryScore := TT.scoreEntryForReplacement(&bucket.Entries[i], depth)
		if entryScore < worstScore {
			worstScore = entryScore
			replaceIdx = i
		}
	}

	// Replace the selected entry
	entry := &bucket.Entries[replaceIdx]
	entry.Hash = hashHigh
	entry.Move = move
	entry.Score = score
	entry.Depth = depth
	entry.Flag = flag
	entry.Generation = TT.generation
}

// scoreEntryForReplacement calculates a priority score for an entry
// Lower score = more likely to be replaced
// Parameters:
//   - entry: the existing entry to evaluate
//   - newDepth: depth of the new entry we want to store
func (TT *TransTable) scoreEntryForReplacement(entry *TTEntry, newDepth int8) int {
	// Empty entry - definitely replace it
	if entry.Hash == 0 {
		return -10000
	}

	score := 0

	// Depth is the most important factor
	// Each depth level is worth 8 points
	score += int(entry.Depth) * 8

	// Age penalty: old entries are less valuable
	// Each generation of age costs 4 points
	age := int(TT.generation) - int(entry.Generation)
	if age < 0 {
		age += 256 // Handle wraparound
	}
	score -= age * 4

	// Exact entries are more valuable than bound entries
	if entry.Flag == ExactFlag {
		score += 4
	}

	// If the new entry has significantly higher depth, prefer replacing
	// This helps when we're doing a deeper search
	depthDiff := int(newDepth) - int(entry.Depth)
	if depthDiff > 2 {
		score -= depthDiff * 2
	}

	return score
}

// Prefetch hints to the CPU that we'll need this bucket soon
// Call this a few moves before you actually need the entry
// Note: Go doesn't have direct prefetch intrinsics, but this pattern
// helps the compiler/runtime with memory access patterns
func (TT *TransTable) Prefetch(hash uint64) {
	if TT.isInitialized {
		_ = TT.buckets[hash%TT.size]
	}
}

// GetHashfull returns the approximate fill rate of the TT (per mille)
// This is useful for UCI "info hashfull" output
func (TT *TransTable) GetHashfull() int {
	if !TT.isInitialized || TT.size == 0 {
		return 0
	}

	// Sample first 1000 buckets for performance
	sampleSize := uint64(1000)
	if sampleSize > TT.size {
		sampleSize = TT.size
	}

	used := 0
	for i := uint64(0); i < sampleSize; i++ {
		bucket := &TT.buckets[i]
		for j := 0; j < BucketSize; j++ {
			if bucket.Entries[j].Hash != 0 && bucket.Entries[j].Generation == TT.generation {
				used++
			}
		}
	}

	// Return per mille (parts per thousand)
	return (used * 1000) / (int(sampleSize) * BucketSize)
}

// GetTTMove retrieves just the move from a TT entry without full lookup
// Useful for move ordering when you don't need the full entry
func (TT *TransTable) GetTTMove(hash uint64) gm.Move {
	if !TT.isInitialized {
		return 0
	}

	bucketIdx := hash % TT.size
	bucket := &TT.buckets[bucketIdx]
	hashHigh := uint32(hash >> 32)

	// Check all entries in the bucket for a match
	for i := 0; i < BucketSize; i++ {
		if bucket.Entries[i].Hash == hashHigh {
			return bucket.Entries[i].Move
		}
	}

	return 0
}

// Stats returns statistics about the TT for debugging
type TTStats struct {
	Size           uint64
	BucketCount    uint64
	EntriesPerSlot int
	Generation     uint8
	Hashfull       int
}

func (TT *TransTable) Stats() TTStats {
	return TTStats{
		Size:           TT.size * uint64(BucketSize),
		BucketCount:    TT.size,
		EntriesPerSlot: BucketSize,
		Generation:     TT.generation,
		Hashfull:       TT.GetHashfull(),
	}
}
