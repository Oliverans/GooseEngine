package tuner

import (
	"math"
	"testing"
)

func TestTexelLinkLossAndDerivatives(t *testing.T) {
	link, err := NewTexelLink(0.01)
	if err != nil {
		t.Fatal(err)
	}
	draw, err := link.Evaluate(0, OutcomeDraw)
	if err != nil {
		t.Fatal(err)
	}
	assertNear(t, draw.ExpectedWhiteScore, 0.5, 1e-15)
	assertNear(t, draw.Brier, 0, 1e-15)
	assertNear(t, draw.LogLoss, math.Log(2), 1e-15)
	assertNear(t, draw.BrierDerivativeEval, 0, 1e-15)

	const evaluation = 137.0
	sample, err := link.Evaluate(evaluation, OutcomeWhiteWin)
	if err != nil {
		t.Fatal(err)
	}
	const epsilon = 1e-5
	plusEval, _ := link.Evaluate(evaluation+epsilon, OutcomeWhiteWin)
	minusEval, _ := link.Evaluate(evaluation-epsilon, OutcomeWhiteWin)
	numericEval := (plusEval.Brier - minusEval.Brier) / (2 * epsilon)
	assertNear(t, sample.BrierDerivativeEval, numericEval, 1e-10)

	plusK, _ := NewTexelLink(link.K + epsilon)
	minusK, _ := NewTexelLink(link.K - epsilon)
	plusKLoss, _ := plusK.Evaluate(evaluation, OutcomeWhiteWin)
	minusKLoss, _ := minusK.Evaluate(evaluation, OutcomeWhiteWin)
	numericK := (plusKLoss.Brier - minusKLoss.Brier) / (2 * epsilon)
	assertNear(t, sample.BrierDerivativeK, numericK, 1e-5)

	extreme, err := link.Evaluate(1e9, OutcomeBlackWin)
	if err != nil {
		t.Fatal(err)
	}
	if !finite(extreme.LogLoss) || extreme.ExpectedWhiteScore != 1 {
		t.Fatalf("unstable extreme loss: %+v", extreme)
	}
}

func TestOutcomeLossAccumulatorUsesNormalizedWeights(t *testing.T) {
	link, _ := NewTexelLink(0.01)
	var accumulator OutcomeLossAccumulator
	black, err := accumulator.Add(link, -100, OutcomeBlackWin, 1)
	if err != nil {
		t.Fatal(err)
	}
	white, err := accumulator.Add(link, 50, OutcomeWhiteWin, 3)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := accumulator.Add(link, 0, OutcomeDraw, 0); err != nil {
		t.Fatal(err)
	}
	metrics, err := accumulator.Metrics()
	if err != nil {
		t.Fatal(err)
	}
	assertNear(t, metrics.Brier, (black.Brier+3*white.Brier)/4, 1e-15)
	assertNear(t, metrics.LogLoss, (black.LogLoss+3*white.LogLoss)/4, 1e-15)
	if metrics.Samples != 2 || metrics.Weight != 4 || metrics.ByOutcome[OutcomeDraw].Samples != 0 {
		t.Fatalf("unexpected weighted metrics: %+v", metrics)
	}
	if _, err := accumulator.Add(link, 0, OutcomeDraw, -1); err == nil {
		t.Fatal("negative sample weight was accepted")
	}
}

func TestTexelCalibrationFindsKnownScale(t *testing.T) {
	const trueK = 0.01
	var calibration TexelCalibrationSet
	for _, evaluation := range []float64{-200, -100, 100, 200} {
		prediction := stableSigmoid(trueK * evaluation)
		if err := calibration.Add(evaluation, OutcomeWhiteWin, prediction); err != nil {
			t.Fatal(err)
		}
		if err := calibration.Add(evaluation, OutcomeBlackWin, 1-prediction); err != nil {
			t.Fatal(err)
		}
	}
	fit, err := calibration.FitBrier(0.001, 0.02, 1e-12, 100)
	if err != nil {
		t.Fatal(err)
	}
	assertNear(t, fit.K, trueK, 1e-8)
	if fit.Samples != 8 || fit.UniqueEvals != 4 || fit.TotalWeight != 4 {
		t.Fatalf("unexpected fit metadata: %+v", fit)
	}
	link, err := NewTexelLink(fit.K)
	if err != nil {
		t.Fatal(err)
	}
	metrics, err := calibration.Metrics(link)
	if err != nil {
		t.Fatal(err)
	}
	assertNear(t, metrics.Brier, fit.Brier, 1e-15)
	if metrics.Samples != 8 || metrics.ByOutcome[OutcomeBlackWin].Samples != 4 || metrics.ByOutcome[OutcomeWhiteWin].Samples != 4 {
		t.Fatalf("unexpected calibration metrics: %+v", metrics)
	}
}

func assertNear(t *testing.T, got, want, tolerance float64) {
	t.Helper()
	if math.Abs(got-want) > tolerance {
		t.Fatalf("got %.16g, want %.16g (tolerance %.3g)", got, want, tolerance)
	}
}
