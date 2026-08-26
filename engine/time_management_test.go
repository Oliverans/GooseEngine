package engine

import (
	"math"
	"testing"
	"time"
)

func TestMaxHardLimitReservesCurrentClockBuffer(t *testing.T) {
	tests := []struct {
		name          string
		remainingTime int
		increment     int
		movesToGo     int
		want          int64
	}{
		{"increment cannot be spent before move", 200, 300, 0, 150},
		{"normal clock keeps policy cap", 10000, 300, 0, 2300},
		{"moves to go reserves buffer", 10000, 0, 20, 9800},
		{"sub-buffer clock keeps one millisecond", 40, 300, 0, 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			th := TimeHandler{
				remainingTime: test.remainingTime,
				increment:     test.increment,
				movesToGo:     test.movesToGo,
			}

			if got := th.maxHardLimitMillis(); got != test.want {
				t.Fatalf("maxHardLimitMillis() = %d, want %d", got, test.want)
			}
		})
	}
}

func TestOpeningTimeScale(t *testing.T) {
	tests := []struct {
		ply  int
		want float64
	}{
		{0, 0.75},
		{10, 0.8440393518518518},
		{20, 0.9814814814814814},
		{24, 1},
		{80, 1},
	}

	for _, test := range tests {
		if got := openingTimeScale(test.ply); math.Abs(got-test.want) > 1e-9 {
			t.Fatalf("openingTimeScale(%d) = %f, want %f", test.ply, got, test.want)
		}
	}
}

func TestStartTimeBuildsOpeningAndHardBudgets(t *testing.T) {
	th := TimeHandler{}
	th.initTimemanagement(10000, 300, 1, 0, false)
	th.StartTime(1, true)

	if th.baseAllocationMillis != 394 {
		t.Fatalf("base allocation = %d, want 394", th.baseAllocationMillis)
	}
	if th.optimumMillis != 177 {
		t.Fatalf("optimum allocation = %d, want 177", th.optimumMillis)
	}
	if th.maximumMillis != 985 {
		t.Fatalf("maximum allocation = %d, want 985", th.maximumMillis)
	}

	lowClock := TimeHandler{}
	lowClock.initTimemanagement(200, 300, 50, 0, false)
	lowClock.StartTime(50, true)
	if lowClock.maximumMillis != 150 {
		t.Fatalf("low-clock maximum = %d, want 150", lowClock.maximumMillis)
	}
}

func TestUpdateIterationAdjustsDynamicTarget(t *testing.T) {
	th := TimeHandler{
		baseAllocationMillis: 1000,
		optimumMillis:        600,
		targetMillis:         600,
		maximumMillis:        2500,
	}

	th.UpdateIteration(100, 1)
	if th.bestMoveStability != 0 || th.targetMillis != 600 {
		t.Fatalf("first iteration changed stability or target: stability %d, target %d", th.bestMoveStability, th.targetMillis)
	}

	th.UpdateIteration(100, 1)
	if th.bestMoveStability != 1 || th.targetMillis != 780 {
		t.Fatalf("stable iteration: stability %d, target %d; want 1 and 780", th.bestMoveStability, th.targetMillis)
	}

	th.UpdateIteration(0, 2)
	if th.bestMoveStability != 0 || th.lastScoreDrop != 100 || th.targetMillis != 1305 {
		t.Fatalf("unstable falling iteration: stability %d, drop %d, target %d; want 0, 100, 1305",
			th.bestMoveStability, th.lastScoreDrop, th.targetMillis)
	}
}

func TestDynamicTargetRespectsMinimumAndMaximum(t *testing.T) {
	minimum := TimeHandler{
		baseAllocationMillis: 1000,
		optimumMillis:        300,
		targetMillis:         300,
		maximumMillis:        2500,
	}
	minimum.UpdateIteration(0, 1)
	for range 8 {
		minimum.UpdateIteration(0, 1)
	}
	if minimum.targetMillis != 300 {
		t.Fatalf("minimum target = %d, want 300", minimum.targetMillis)
	}

	maximum := TimeHandler{
		baseAllocationMillis: 1000,
		optimumMillis:        1000,
		targetMillis:         1000,
		maximumMillis:        1200,
	}
	maximum.UpdateIteration(100, 1)
	maximum.UpdateIteration(0, 2)
	if maximum.targetMillis != 1200 {
		t.Fatalf("maximum target = %d, want 1200", maximum.targetMillis)
	}
}

func TestShouldStartNextIterationUsesDynamicTarget(t *testing.T) {
	th := TimeHandler{
		startTime:    time.Now().Add(-101 * time.Millisecond),
		targetMillis: 100,
	}
	if th.ShouldStartNextIteration() {
		t.Fatal("iteration started after dynamic target")
	}
	if th.stopReason != "dynamic target" {
		t.Fatalf("stop reason = %q, want dynamic target", th.stopReason)
	}

	th.usingCustomDepth = true
	if !th.ShouldStartNextIteration() {
		t.Fatal("custom-depth search was stopped by dynamic target")
	}
}
