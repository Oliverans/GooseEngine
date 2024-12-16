package engine

import "time"

type TimeHandler struct {
	remainingTime    int
	madeMoveCount    int
	increment        int
	timeForMove      time.Time
	stopSearch       bool
	isInitialized    bool
	usingCustomDepth bool
}

func (th *TimeHandler) initTimemanagement(remaniningTime int, increment int, madeMoveCount int, useCustomDepth bool) {
	th.remainingTime = remaniningTime
	th.increment = increment
	th.madeMoveCount = madeMoveCount
	th.stopSearch = false
	th.isInitialized = true
	th.usingCustomDepth = useCustomDepth
}

func (th *TimeHandler) StartTime(madeMoveCount int) {
	th.madeMoveCount = madeMoveCount
	th.stopSearch = false

	var moveTime = 0
	if th.increment > 0 {
		moveTime = th.remainingTime/max(60, 40-madeMoveCount) + th.increment
	} else {
		moveTime = (th.remainingTime / 40)
	}

	if th.remainingTime == 0 {
		th.timeForMove = time.Now().Add(time.Duration(5000) * time.Millisecond) // 5 second searches for testing ...
	} else {
		th.timeForMove = time.Now().Add(time.Duration(moveTime) * time.Millisecond)
	}
}

func (th *TimeHandler) Update(extraTime int64) {

	// Set the new time for the current search.
	th.timeForMove = time.Now().Add(time.Duration(extraTime) * time.Millisecond)
}

func (th *TimeHandler) TimeStatus() bool {
	if timeHandler.timeForMove.Before(time.Now()) && !th.usingCustomDepth {
		return true
	} else {
		return false
	}
}
