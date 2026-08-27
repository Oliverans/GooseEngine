package engine

import (
	"time"
)

const (
	expectedGameLength   = 60   // Expect games to last ~60 moves
	minMovesRemaining    = 10   // Always assume at least 10 moves left
	maxTimeUsageFraction = 0.20 // Never use more than 20% of remaining time
	movesToGoBufferDiv   = 50   // 2% buffer when movestogo is known
	minBufferMillis      = 50
	moveTimeOverheadMs   = 10
	nodeHardLimitFactor  = 4
)

var TimeSoftPercent = 60
var TimeHardPercent = 250
var TimeOpeningMinPercent = 75
var TimeOpeningRampPly = 24
var TimeMinimumPercent = 30
var TimeBestMoveChangedPercent = 145
var TimeBestMoveStepPercent = 15
var TimeBestMoveStablePercent = 70
var TimeScoreDropMaxCP int32 = 100
var TimeScoreDropMaxPercent = 150

type TimeHandler struct {
	remainingTime        int
	fullmoveNumber       int
	increment            int
	startTime            time.Time
	hardTimeLimit        time.Time
	stopSearch           bool
	isInitialized        bool
	usingCustomDepth     bool
	clockEnabled         bool
	infinite             bool
	moveTimeMillis       int64
	nodeLimit            uint64
	baseAllocationMillis int64
	optimumMillis        int64
	targetMillis         int64
	maximumMillis        int64
	movesToGo            int
	openingScale         float64
	stopReason           string

	lastScore         int32
	lastScoreDrop     int32
	lastBestMove      uint32
	hasLastIteration  bool
	bestMoveStability int
}

func (th *TimeHandler) initTimemanagement(limits SearchLimits, fullmoveNumber int, whiteToMove bool) {
	if whiteToMove {
		th.remainingTime = limits.WTime
		th.increment = limits.WInc
	} else {
		th.remainingTime = limits.BTime
		th.increment = limits.BInc
	}
	th.fullmoveNumber = fullmoveNumber
	th.stopSearch = false
	th.isInitialized = true
	th.infinite = limits.Infinite
	th.moveTimeMillis = int64(limits.MoveTimeMs)
	th.nodeLimit = limits.Nodes
	th.clockEnabled = limits.MoveTimeMs > 0 || th.remainingTime > 0 || th.increment > 0
	th.usingCustomDepth = limits.Depth > 0 && !th.clockEnabled && limits.Nodes == 0 && !limits.Infinite
	th.baseAllocationMillis = 0
	th.optimumMillis = 0
	th.targetMillis = 0
	th.maximumMillis = 0
	th.movesToGo = limits.MovesToGo
	th.openingScale = 1
	th.stopReason = ""
	th.lastScore = 0
	th.lastScoreDrop = 0
	th.lastBestMove = 0
	th.hasLastIteration = false
	th.bestMoveStability = 0
}

func (th *TimeHandler) StartTime(fullmoveNumber int, whiteToMove bool) {
	th.fullmoveNumber = fullmoveNumber
	th.stopSearch = false
	th.startTime = time.Now()
	th.hardTimeLimit = time.Time{}

	if th.infinite || !th.clockEnabled {
		return
	}

	if th.moveTimeMillis > 0 {
		budget := th.moveTimeMillis - moveTimeOverheadMs
		if budget < 1 {
			budget = 1
		}
		th.baseAllocationMillis = budget
		th.optimumMillis = budget
		th.targetMillis = budget
		th.maximumMillis = budget
		th.openingScale = 1
		th.hardTimeLimit = th.startTime.Add(time.Duration(budget) * time.Millisecond)
		return
	}

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
	gamePly := 2 * (fullmoveNumber - 1)
	if !whiteToMove {
		gamePly++
	}
	if gamePly < 0 {
		gamePly = 0
	}
	th.openingScale = openingTimeScale(gamePly)

	softMillis := int64(baseTime) * int64(TimeSoftPercent) / 100
	th.optimumMillis = int64(float64(softMillis) * th.openingScale)
	if th.optimumMillis < 1 {
		th.optimumMillis = 1
	}
	th.targetMillis = th.optimumMillis

	hardMillis := int64(baseTime) * int64(TimeHardPercent) / 100

	maxHard := th.maxHardLimitMillis()
	if hardMillis > maxHard {
		hardMillis = maxHard
	}
	if hardMillis < th.optimumMillis {
		hardMillis = th.optimumMillis
	}
	th.maximumMillis = hardMillis

	th.hardTimeLimit = th.startTime.Add(time.Duration(hardMillis) * time.Millisecond)
}

func openingTimeScale(gamePly int) float64 {
	if gamePly <= 0 {
		return float64(TimeOpeningMinPercent) / 100
	}
	if gamePly >= TimeOpeningRampPly {
		return 1
	}
	x := float64(gamePly) / float64(TimeOpeningRampPly)
	smooth := x * x * (3 - 2*x)
	minimum := float64(TimeOpeningMinPercent) / 100
	return minimum + (1-minimum)*smooth
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

	available := th.remainingTime - th.bufferMillis()
	if available < 1 {
		available = 1
	}
	if th.movesToGo > 0 {
		return int64(available)
	}

	maxHard := int64(float64(th.remainingTime) * maxTimeUsageFraction)
	if th.increment > 0 {
		maxHard += int64(th.increment)
	}
	if maxHard > int64(available) {
		maxHard = int64(available)
	}
	if maxHard < 1 {
		maxHard = 1
	}
	return maxHard
}

// TimeStatus returns true if we should stop searching
// This checks the HARD limit - we must stop
func (th *TimeHandler) TimeStatus() bool {
	if th.infinite || !th.clockEnabled || th.usingCustomDepth {
		return false
	}
	exceeded := !th.hardTimeLimit.IsZero() && time.Now().After(th.hardTimeLimit)
	if exceeded {
		th.stopReason = "hard maximum"
	}
	return exceeded
}

func (th *TimeHandler) UpdateIteration(score int32, bestMove uint32) {
	if th.infinite || th.moveTimeMillis > 0 || !th.clockEnabled || th.usingCustomDepth {
		return
	}
	if !th.hasLastIteration {
		th.lastScore = score
		th.lastBestMove = bestMove
		th.hasLastIteration = true
		return
	}

	if bestMove == th.lastBestMove {
		th.bestMoveStability++
	} else {
		th.bestMoveStability = 0
	}

	th.lastScoreDrop = th.lastScore - score
	if th.lastScoreDrop < 0 {
		th.lastScoreDrop = 0
	}

	movePercent := TimeBestMoveChangedPercent - TimeBestMoveStepPercent*th.bestMoveStability
	if movePercent < TimeBestMoveStablePercent {
		movePercent = TimeBestMoveStablePercent
	}

	scorePercent := 100
	if TimeScoreDropMaxCP > 0 && th.lastScoreDrop > 0 {
		drop := th.lastScoreDrop
		if drop > TimeScoreDropMaxCP {
			drop = TimeScoreDropMaxCP
		}
		scorePercent += int(drop) * (TimeScoreDropMaxPercent - 100) / int(TimeScoreDropMaxCP)
	}

	target := th.optimumMillis * int64(movePercent) / 100
	target = target * int64(scorePercent) / 100
	minimum := th.baseAllocationMillis * int64(TimeMinimumPercent) / 100
	if minimum < 1 {
		minimum = 1
	}
	if target < minimum {
		target = minimum
	}
	if target > th.maximumMillis {
		target = th.maximumMillis
	}
	th.targetMillis = target
	th.lastScore = score
	th.lastBestMove = bestMove
}

func (th *TimeHandler) ShouldStartNextIteration(nodesChecked int) bool {
	if th.nodeLimit > 0 && uint64(nodesChecked) >= th.nodeLimit {
		th.stopReason = "node soft limit"
		return false
	}
	if th.infinite || !th.clockEnabled || th.usingCustomDepth {
		return true
	}
	if time.Since(th.startTime).Milliseconds() >= th.targetMillis {
		th.stopReason = "dynamic target"
		return false
	}
	return true
}

// NodeHardLimitReached is the coarse in-iteration backstop for fixed-node
// searches. The soft limit is checked between completed iterations.
func (th *TimeHandler) NodeHardLimitReached(nodesChecked int) bool {
	if th.nodeLimit == 0 || nodesChecked < 0 {
		return false
	}
	threshold := ^uint64(0)
	if th.nodeLimit <= ^uint64(0)/nodeHardLimitFactor {
		threshold = th.nodeLimit * nodeHardLimitFactor
	}
	if uint64(nodesChecked) < threshold {
		return false
	}
	th.stopReason = "node hard limit"
	return true
}
