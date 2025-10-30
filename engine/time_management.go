package engine

import (
	"time"

	"github.com/dylhunn/dragontoothmg"
)

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

func (th *TimeHandler) StartTime(b *dragontoothmg.Board) {
	th.madeMoveCount = int(b.Fullmoveno)
	th.stopSearch = false

	// Estimate moves left from phase
	piecePhase := GetPiecePhase(b)
	movesLeft := estimateMovesRemaining(piecePhase) // 20..45

	// Engine-side safety knobs
	const overheadMs = 30      // reserve for UCI/IO jitter
	const minMoveMs = 5        // never less than this
	const maxFrac = 0.7        // never spend >70% of remaining time
	const panicThreshMs = 1000 // your existing threshold
	const panicFrac = 0.90     // use 90% of inc in panic

	rem := th.remainingTime
	inc := th.increment

	var moveTime int
	if inc > 0 {
		if rem < panicThreshMs {
			// Panic: try to “bank” a little time
			moveTime = int(float64(inc) * panicFrac)
		} else {
			// Normal: spend a fraction of remaining + take (most of) the inc
			moveTime = rem/movesLeft + inc
		}
	} else {
		moveTime = rem / 40
	}

	// Apply overhead and clamps
	if moveTime < minMoveMs {
		moveTime = minMoveMs
	}
	if moveTime > int(float64(rem)*maxFrac) {
		moveTime = int(float64(rem) * maxFrac)
	}
	if moveTime > rem-overheadMs {
		moveTime = rem - overheadMs
	}
	if moveTime < minMoveMs {
		moveTime = minMoveMs
	} // re-check after ceiling

	th.timeForMove = time.Now().Add(time.Duration(moveTime) * time.Millisecond)
}

func (th *TimeHandler) Update(extraTime int64) {
	th.timeForMove = time.Now().Add(time.Duration(extraTime) * time.Millisecond)
}

/*
	- True if we're out of time and we're not using a custom depth search
	- False if we still got time
*/
func (th *TimeHandler) TimeStatus() bool {
	if th.timeForMove.Before(time.Now()) && !th.usingCustomDepth {
		return true
	} else {
		return false
	}
}

func estimateMovesRemaining(phase int) int {
	// Linearly interpolate between 20 (endgame) and 45 (opening/midgame)
	// May consider even lower in endgame and even higher in opening/midgame
	return (phase*25)/24 + 20 // result ∈ [20, 45]
}
