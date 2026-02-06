package engine

import (
	"time"
)

const (
	// Base assumptions
	expectedGameLength = 60 // Expect games to last ~60 moves
	minMovesRemaining  = 10 // Always assume at least 10 moves left

	// Time allocation factors
	softTimeFactor       = 0.6  // Use 60% of allocated time as soft limit
	hardTimeFactor       = 2.5  // Hard limit is 2.5x the base allocation
	maxTimeUsageFraction = 0.20 // Never use more than 20% of remaining time
	movesToGoBufferDiv   = 50   // 2% buffer when movestogo is known

	// Safety buffer
	minBufferMillis = 50
)

type TimeHandler struct {
	remainingTime        int
	fullmoveNumber       int
	increment            int
	startTime            time.Time
	softTimeLimit        time.Time
	hardTimeLimit        time.Time
	stopSearch           bool
	isInitialized        bool
	usingCustomDepth     bool
	baseAllocationMillis int64
	movesToGo            int

	// For dynamic adjustments
	lastScore         int16
	lastBestMove      uint32
	scoreStability    int
	bestMoveStability int
}

func (th *TimeHandler) initTimemanagement(remainingTime int, increment int, fullmoveNumber int, movesToGo int, useCustomDepth bool) {
	th.remainingTime = remainingTime
	th.increment = increment
	th.fullmoveNumber = fullmoveNumber
	th.stopSearch = false
	th.isInitialized = true
	th.usingCustomDepth = useCustomDepth
	th.baseAllocationMillis = 0
	th.movesToGo = movesToGo
	th.scoreStability = 0
	th.bestMoveStability = 0
}

func (th *TimeHandler) StartTime(fullmoveNumber int) {
	th.fullmoveNumber = fullmoveNumber
	th.stopSearch = false
	th.startTime = time.Now()

	// Estimate moves remaining based on game phase
	movesRemaining := th.estimateMovesRemaining(fullmoveNumber)
	if th.movesToGo > 0 {
		movesRemaining = th.movesToGo
	}

	// Calculate base time allocation
	baseTime := th.calculateBaseTime(movesRemaining)

	// Apply safety limits
	baseTime = th.applySafetyLimits(baseTime)

	th.baseAllocationMillis = int64(baseTime)

	// Set soft and hard limits
	softMillis := int64(float64(baseTime) * softTimeFactor)
	hardMillis := int64(float64(baseTime) * hardTimeFactor)

	// Hard limit can't exceed safety threshold
	maxHard := th.maxHardLimitMillis()
	if hardMillis > maxHard {
		hardMillis = maxHard
	}

	// Ensure minimums
	if softMillis < 1 {
		softMillis = 1
	}
	if hardMillis < softMillis {
		hardMillis = softMillis
	}

	th.softTimeLimit = th.startTime.Add(time.Duration(softMillis) * time.Millisecond)
	th.hardTimeLimit = th.startTime.Add(time.Duration(hardMillis) * time.Millisecond)
}

func (th *TimeHandler) estimateMovesRemaining(fullmoveNumber int) int {
	// Simple model: expect game to last expectedGameLength moves
	// But always assume at least minMovesRemaining

	remaining := expectedGameLength - fullmoveNumber
	if remaining < minMovesRemaining {
		remaining = minMovesRemaining
	}

	return remaining
}

func (th *TimeHandler) calculateBaseTime(movesRemaining int) int {
	if th.remainingTime <= 0 {
		// No base time, just use increment
		if th.increment > 0 {
			return th.increment * 3 / 4 // Use 75% of increment
		}
		return 1000 // Fallback: 1 second
	}

	// Base allocation: remaining time / moves remaining
	baseTime := th.remainingTime / movesRemaining

	// Add a portion of the increment
	if th.increment > 0 {
		baseTime += th.increment * 3 / 4
	}

	return baseTime
}

func (th *TimeHandler) applySafetyLimits(baseTime int) int {
	if th.remainingTime <= 0 {
		return baseTime
	}

	if th.movesToGo <= 0 {
		// Never use more than maxTimeUsageFraction of remaining time
		maxAllowed := int(float64(th.remainingTime) * maxTimeUsageFraction)
		if th.increment > 0 {
			maxAllowed += th.increment
		}
		if baseTime > maxAllowed {
			baseTime = maxAllowed
		}
	}

	// Keep a minimum buffer
	buffer := th.bufferMillis()

	maxWithBuffer := th.remainingTime - buffer
	if maxWithBuffer < 1 {
		maxWithBuffer = 1
	}

	if baseTime > maxWithBuffer {
		baseTime = maxWithBuffer
	}

	return baseTime
}

func (th *TimeHandler) bufferMillis() int {
	if th.remainingTime <= 0 {
		return minBufferMillis
	}
	divisor := 20
	if th.movesToGo > 0 {
		divisor = movesToGoBufferDiv
	}
	buffer := th.remainingTime / divisor
	if buffer < minBufferMillis {
		buffer = minBufferMillis
	}
	return buffer
}

func (th *TimeHandler) maxHardLimitMillis() int64 {
	if th.remainingTime <= 0 {
		return 1
	}
	if th.movesToGo > 0 {
		maxHard := th.remainingTime - th.bufferMillis()
		if maxHard < 1 {
			maxHard = 1
		}
		return int64(maxHard)
	}
	maxHard := int64(float64(th.remainingTime) * maxTimeUsageFraction)
	if th.increment > 0 {
		maxHard += int64(th.increment)
	}
	if maxHard < 1 {
		maxHard = 1
	}
	return maxHard
}

// TimeStatus returns true if we should stop searching
// This checks the HARD limit - we must stop
func (th *TimeHandler) TimeStatus() bool {
	if th.usingCustomDepth {
		return false
	}
	return !th.hardTimeLimit.IsZero() && time.Now().After(th.hardTimeLimit)
}

// SoftTimeExceeded returns true if we've passed the soft limit
// Use this to decide whether to start a new iteration
func (th *TimeHandler) SoftTimeExceeded() bool {
	if th.usingCustomDepth {
		return false
	}
	return !th.softTimeLimit.IsZero() && time.Now().After(th.softTimeLimit)
}

// UpdateStability should be called after each depth completion
// It tracks whether the best move and score are stable
func (th *TimeHandler) UpdateStability(score int16, bestMove uint32) {
	if bestMove == th.lastBestMove {
		th.bestMoveStability++
	} else {
		th.bestMoveStability = 0
		th.lastBestMove = bestMove
	}

	scoreDiff := score - th.lastScore
	if scoreDiff < 0 {
		scoreDiff = -scoreDiff
	}

	if scoreDiff < 10 { // Score within 10cp
		th.scoreStability++
	} else {
		th.scoreStability = 0
	}
	th.lastScore = score
}

// ShouldStopEarly returns true if we can stop before soft limit
// due to very stable position
func (th *TimeHandler) ShouldStopEarly() bool {
	if th.usingCustomDepth {
		return false
	}

	// If best move has been stable for 4+ depths and score is stable,
	// we can stop after using 40% of soft time
	if th.bestMoveStability >= 4 && th.scoreStability >= 3 {
		elapsed := time.Since(th.startTime).Milliseconds()
		earlyStop := int64(float64(th.baseAllocationMillis) * 0.4)
		return elapsed >= earlyStop
	}

	return false
}

// ShouldExtendTime returns true if we should think longer
// due to unstable position or score drop
func (th *TimeHandler) ShouldExtendTime() bool {
	// Extend if best move keeps changing
	if th.bestMoveStability == 0 && th.scoreStability < 2 {
		return true
	}
	return false
}

// ExtendTime adds additional time when position is complex
func (th *TimeHandler) ExtendTime() {
	if th.usingCustomDepth {
		return
	}

	// Extend hard limit by 50% of base allocation
	extension := time.Duration(th.baseAllocationMillis/2) * time.Millisecond
	th.hardTimeLimit = th.hardTimeLimit.Add(extension)

	// But never exceed the safety maximum
	maxTime := th.startTime.Add(time.Duration(th.maxHardLimitMillis()) * time.Millisecond)
	if th.hardTimeLimit.After(maxTime) {
		th.hardTimeLimit = maxTime
	}
}
