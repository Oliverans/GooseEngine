package engine

import "testing"

func prepareLMRTest(t *testing.T) {
	t.Helper()
	oldCutnode := LMRCutnode
	oldTTPv := LMRTTPv
	oldNoisyOffset := LMRNoisyOffset
	oldCheckBonus := LMRCheckBonus
	oldDeeperBase := LMRDeeperBase
	oldCaptureHistoryDivisor := LMRCaptureHistoryDivisor
	oldDepthLimit := LMRDepthLimit
	oldMoveLimit := LMRMoveLimit
	oldTable := LMR[8][5]
	t.Cleanup(func() {
		LMRCutnode = oldCutnode
		LMRTTPv = oldTTPv
		LMRNoisyOffset = oldNoisyOffset
		LMRCheckBonus = oldCheckBonus
		LMRDeeperBase = oldDeeperBase
		LMRCaptureHistoryDivisor = oldCaptureHistoryDivisor
		LMRDepthLimit = oldDepthLimit
		LMRMoveLimit = oldMoveLimit
		LMR[8][5] = oldTable
	})
	LMRCutnode = 100
	LMRTTPv = 50
	LMRNoisyOffset = -100
	LMRCheckBonus = -100
	LMRDeeperBase = 40
	LMRCaptureHistoryDivisor = 50
	LMRDepthLimit = 2
	LMRMoveLimit = 2
	LMR[8][5] = 150
}

func TestLMRReductionSignals(t *testing.T) {
	prepareLMRTest(t)

	tests := []struct {
		name           string
		isPVNode       bool
		cutnode        bool
		ttPv           bool
		isCapture      bool
		badCapture     bool
		moveGivesCheck bool
		want           int8
	}{
		{"base", false, false, true, false, false, false, 1},
		{"PV", true, false, true, false, false, false, 0},
		{"cutnode", false, true, true, false, false, false, 2},
		{"non-ttPv", false, false, false, false, false, false, 2},
		{"capture", false, false, true, true, false, false, 0},
		{"bad capture", false, false, true, true, true, false, 1},
		{"checking", false, false, true, false, false, true, 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, base := computeLMRReduction(
				8, 5, test.isPVNode, test.cutnode, test.ttPv, test.isCapture, test.badCapture, test.moveGivesCheck,
				0, 0, false, false, false,
			)
			if got != test.want || base != test.want {
				t.Fatalf("computeLMRReduction() = (%d, %d), want (%d, %d)", got, base, test.want, test.want)
			}
		})
	}
}

func TestLMRCaptureHistoryReduction(t *testing.T) {
	prepareLMRTest(t)
	LMR[8][5] = 250

	tests := []struct {
		name           string
		captureHistory int
		badCapture     bool
		moveGivesCheck bool
		want           int8
		wantBase       int8
	}{
		{"zero", 0, false, false, 1, 1},
		{"positive", 3000, false, false, 0, 1},
		{"negative", -3000, false, false, 2, 1},
		{"positive saturation", captureHistoryMax, false, false, 0, 1},
		{"negative saturation", -captureHistoryMax, false, false, 3, 1},
		{"bad capture", 3000, true, false, 1, 2},
		{"checking capture", -3000, false, true, 1, 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, base := computeLMRReduction(
				8, 5, false, false, true, true, test.badCapture, test.moveGivesCheck,
				0, test.captureHistory, false, false, false,
			)
			if got != test.want || base != test.wantBase {
				t.Fatalf("computeLMRReduction() = (%d, %d), want (%d, %d)", got, base, test.want, test.wantBase)
			}
		})
	}

	quiet, quietBase := computeLMRReduction(8, 5, false, false, true, false, false, false, 0, -captureHistoryMax, false, false, false)
	if quiet != quietBase {
		t.Fatalf("quiet reduction changed from %d to %d", quietBase, quiet)
	}

	clamped, clampedBase := computeLMRReduction(2, 5, false, true, false, true, true, false, 0, -captureHistoryMax, false, false, false)
	if clamped != 0 || clampedBase != 0 {
		t.Fatalf("clamped reduction = (%d, %d), want (0, 0)", clamped, clampedBase)
	}
}

func TestLMRGuards(t *testing.T) {
	prepareLMRTest(t)

	if !lmrEligible(4, 2, false) {
		t.Fatal("ordinary move was not LMR eligible")
	}
	if lmrEligible(1, 2, false) || lmrEligible(4, 1, false) || lmrEligible(4, 2, true) {
		t.Fatal("LMR guard accepted shallow, early, or promotion move")
	}
	if !lmrBadCapture(true, -1) || lmrBadCapture(true, 0) || lmrBadCapture(false, -1) {
		t.Fatal("bad capture classification did not follow picker SEE metadata")
	}
}

func TestLMRResearchDepth(t *testing.T) {
	prepareLMRTest(t)

	tests := []struct {
		name       string
		depth      int8
		bestScore  int32
		score      int32
		wantDepth  int8
		wantAdjust int8
	}{
		{"deeper", 6, 100, 153, 6, lmrResearchDeeper},
		{"nominal", 6, 100, 106, 5, lmrResearchNominal},
		{"shallower", 6, 100, 105, 4, lmrResearchShallower},
		{"minimum", 2, 100, 101, 1, lmrResearchNominal},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotDepth, gotAdjust := lmrResearchDepth(test.depth, test.bestScore, test.score, false)
			if gotDepth != test.wantDepth || gotAdjust != test.wantAdjust {
				t.Fatalf("lmrResearchDepth() = (%d, %d), want (%d, %d)", gotDepth, gotAdjust, test.wantDepth, test.wantAdjust)
			}
		})
	}
}
