package engine

import "time"

const (
	defaultMovesPerTimeControl = 40
	timeBufferDivisor          = 20
	minTimeBufferMillis        = 100
)

type TimeHandler struct {
	remainingTime               int
	fullmoveNumber              int
	increment                   int
	timeForMove                 time.Time
	stopSearch                  bool
	isInitialized               bool
	usingCustomDepth            bool
	currentMoveAllocationMillis int64
}

func (th *TimeHandler) initTimemanagement(remaniningTime int, increment int, fullmoveNumber int, useCustomDepth bool) {
	th.remainingTime = remaniningTime
	th.increment = increment
	th.fullmoveNumber = fullmoveNumber
	th.stopSearch = false
	th.isInitialized = true
	th.usingCustomDepth = useCustomDepth
	th.currentMoveAllocationMillis = 0
}

func (th *TimeHandler) StartTime(fullmoveNumber int) {
	th.fullmoveNumber = fullmoveNumber
	th.stopSearch = false

	movesCompleted := fullmoveNumber - 1
	if movesCompleted < 0 {
		movesCompleted = 0
	}
	movesRemaining := defaultMovesPerTimeControl - movesCompleted
	if movesRemaining < 1 {
		movesRemaining = 1
	}

	moveTime := th.remainingTime / movesRemaining
	if th.increment > 0 {
		moveTime += th.increment
	}

	if th.remainingTime > 0 {
		buffer := th.remainingTime / timeBufferDivisor
		if buffer < minTimeBufferMillis {
			buffer = minTimeBufferMillis
		}
		if buffer >= th.remainingTime {
			buffer = th.remainingTime / 2
			if buffer < 1 {
				buffer = 1
			}
		}
		maxAllocation := th.remainingTime - buffer
		if maxAllocation < 1 {
			maxAllocation = 1
		}
		if moveTime > maxAllocation {
			moveTime = maxAllocation
		}
	}
	if moveTime < 1 {
		moveTime = 1
	}

	if th.remainingTime == 0 {
		th.currentMoveAllocationMillis = 5000
		th.timeForMove = time.Now().Add(time.Duration(th.currentMoveAllocationMillis) * time.Millisecond) // 5 second searches for testing ...
	} else {
		th.currentMoveAllocationMillis = int64(moveTime)
		th.timeForMove = time.Now().Add(time.Duration(moveTime) * time.Millisecond)
	}
}

func (th *TimeHandler) Update(extraTime int64) {
	if extraTime == 0 {
		return
	}

	adjustment := time.Duration(extraTime) * time.Millisecond
	if th.timeForMove.IsZero() {
		th.timeForMove = time.Now().Add(adjustment)
		return
	}
	th.timeForMove = th.timeForMove.Add(adjustment)
}

func (th *TimeHandler) TimeStatus() bool {
	return !th.usingCustomDepth && !th.timeForMove.IsZero() && th.timeForMove.Before(time.Now())
}

func (th *TimeHandler) ExtendForAspiration() {
	if th.usingCustomDepth {
		return
	}
	extra := th.currentMoveAllocationMillis
	if extra <= 0 {
		extra = 100
	}
	th.Update(extra)
}
