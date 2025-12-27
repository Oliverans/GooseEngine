// tuner/train.go
package tuner

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"

	eng "chess-engine/engine"
)

// Train optimizes parameters of a Featurizer while retaining k in pst.
type Optimizer interface {
	Step(params []float64, grads []float64)
}

func Train(ctx context.Context, fe Featurizer, pst *PST, data []BinarySample, opt Optimizer, cfg TrainConfig, stmMode bool) error {
	eng.InitPositionBB()
	params := fe.Params() // snapshot and ensure length
	grads := make([]float64, len(params))

	// Optional LR scaling and anchor weights (only for LinearEval with known layout).
	var lrScales []float64
	var anchorWeights []float64
	var anchor []float64
	if le, ok := fe.(*LinearEval); ok {
		// Ensure layout populated
		le.ensureLayout()
		if cfg.LRScaling {
			lrScales = BuildLRScaleVector(le.layout, cfg.LRScaleCfg)
		}
		if cfg.Anchoring {
			anchorWeights = BuildAnchorWeights(le.layout, cfg.AnchorCfg)
			anchor = append(anchor, params...) // seed anchor with initial params
		}
	}
	if len(lrScales) == len(params) {
		if setter, ok := opt.(interface{ SetLRScale([]float64) }); ok {
			setter.SetLRScale(lrScales)
		}
	}

	bs := cfg.Batch
	if bs <= 0 {
		bs = 32768
	}

	// [DEBUG_TMP] basic config snapshot
	//fmt.Printf("[DEBUG_TMP] train init: len(data)=%d batch=%d lrScaling=%v anchoring=%v\n", len(data), bs, cfg.LRScaling, cfg.Anchoring)
	//if le, ok := fe.(*LinearEval); ok {
	//	fmt.Printf("[DEBUG_TMP] toggles: PSTTrain=%v MaterialTrain=%v PassersTrain=%v PawnStructTrain=%v MobilityTrain=%v Extras4Train=%v Extras6Train=%v Extras7Train=%v\n",
	//		le.Toggles.PSTTrain, le.Toggles.MaterialTrain, le.Toggles.PassersTrain, le.Toggles.PawnStructTrain, le.Toggles.MobilityTrain, le.Toggles.Extras4Train, le.Toggles.Extras6Train, le.Toggles.Extras7Train)
	//}

	rng := rand.New(rand.NewSource(42))
	order := make([]int, len(data))
	for i := range order {
		order[i] = i
	}

	type lrScheduler interface {
		SetLR(float64)
		GetLR() float64
	}
	lrOpt, hasLRScheduler := opt.(lrScheduler)

	// Define fixed held-out splits for validation and k-refit.
	holdoutSize := cfg.KRefitCap
	if holdoutSize <= 0 {
		holdoutSize = 200000
	}
	if holdoutSize > len(data) {
		holdoutSize = len(data)
	}

	totalSize := len(data)
	kRefitSize := holdoutSize
	valSize := 0
	if cfg.UseKRefitAsVal {
		valSize = kRefitSize
	} else {
		remaining := totalSize - kRefitSize
		if remaining < 0 {
			remaining = 0
		}
		if cfg.ValFrac > 0 {
			valSize = int(math.Round(float64(remaining) * cfg.ValFrac))
		} else if cfg.ValCap > 0 {
			valSize = cfg.ValCap
		}
		if valSize > remaining {
			valSize = remaining
		}
	}

	trainSize := totalSize - kRefitSize
	valStart := trainSize
	valEnd := totalSize
	kRefitStart := valStart
	if !cfg.UseKRefitAsVal {
		trainSize = totalSize - kRefitSize - valSize
		if trainSize < 0 {
			trainSize = 0
		}
		valStart = trainSize
		valEnd = trainSize + valSize
		kRefitStart = valEnd
	}

	if cfg.EarlyStopPatience > 0 && cfg.PlateauPatience > 0 && cfg.EarlyStopPatience <= cfg.PlateauPatience {
		cfg.EarlyStopPatience = cfg.PlateauPatience + 1
	}

	scheduleEnabled := valSize > 0 && cfg.PlateauPatience > 0 && cfg.LRReduceFactor > 0 &&
		cfg.LRReduceFactor < 1.0 && cfg.LRMin >= 0 && hasLRScheduler
	earlyStopEnabled := scheduleEnabled && cfg.MaxLRDrops > 0 && cfg.EarlyStopPatience > 0

	bestValLoss := math.Inf(1)
	epochsNoImprove := 0
	lrDrops := 0
	cooldown := 0

	for ep := 1; ep <= cfg.Epochs; ep++ {
		t0 := time.Now()
		if cfg.Shuffle {
			// Only shuffle the training portion; keep held-out contiguous for stability
			rng.Shuffle(trainSize, func(i, j int) { order[i], order[j] = order[j], order[i] })
		}
		totalLoss, totalN := 0.0, 0

		// Train on training split only
		for off := 0; off < trainSize; off += bs {
			end := off + bs
			if end > trainSize {
				end = trainSize
			}

			loss, _, n := batchGradFeIdx(fe, pst, data, order, off, end, stmMode, grads)

			// [DEBUG_TMP] grad norm and param delta for first batch each epoch
			if off == 0 {
				var gradNorm float64
				for _, v := range grads {
					gradNorm += math.Abs(v)
				}
				// Use a central knight PST MG slot for a meaningful delta (piece=N, square=d4 -> idx=1*64+27)
				debugIdx := 64 + 27
				if debugIdx >= len(params) {
					debugIdx = 0
				}
				before := params[debugIdx]
				opt.Step(params, grads)
				//after := params[debugIdx]
				//fmt.Printf("[DEBUG_TMP] ep=%d batch0 loss=%.6f gradNorm=%.6f param[%d]_delta=%.6e\n", ep, loss/float64(max(1, n)), gradNorm, debugIdx, after-before)
				// revert param change so normal step below still applies
				params[debugIdx] = before
			}

			// Apply anchored L2 (loss + gradient) if configured and sized correctly.
			if cfg.Anchoring && len(anchorWeights) == len(params) && len(anchor) == len(params) {
				loss = AnchoredL2Loss(params, anchor, anchorWeights, loss)
				AnchoredL2Grad(grads, params, anchor, anchorWeights)
			}

			totalLoss += loss
			totalN += n

			// Ensure grads slice matches current params length.
			params = fe.Params()
			if len(grads) != len(params) {
				grads = make([]float64, len(params))
			}
			opt.Step(params, grads)
			fe.SetParams(params)
		}

		if cfg.AutoK {
			// Post-hoc k refit on held-out split only
			if kRefitSize > 0 {
				refitK(fe, pst, data[kRefitStart:totalSize], stmMode)
			}
		}

		// Validation loss (optional)
		valLoss := 0.0
		valN := 0
		if valSize > 0 {
			for off := valStart; off < valEnd; off += bs {
				end := off + bs
				if end > valEnd {
					end = valEnd
				}
				loss, n := batchLossFeIdx(fe, pst, data, order, off, end, stmMode)
				valLoss += loss
				valN += n
			}
		}

		// Reduce on plateau + early stopping
		if scheduleEnabled && valN > 0 {
			avgVal := valLoss / float64(max(1, valN))
			if avgVal < bestValLoss-cfg.PlateauMinDelta {
				bestValLoss = avgVal
				epochsNoImprove = 0
			} else if cooldown > 0 {
				cooldown--
			} else {
				epochsNoImprove++
				canReduce := cfg.MaxLRDrops <= 0 || lrDrops < cfg.MaxLRDrops
				if canReduce && epochsNoImprove >= cfg.PlateauPatience {
					newLR := math.Max(cfg.LRMin, lrOpt.GetLR()*cfg.LRReduceFactor)
					if newLR < lrOpt.GetLR() {
						lrOpt.SetLR(newLR)
						lrDrops++
						epochsNoImprove = 0
						cooldown = cfg.LRDropCooldown
					}
				}
			}
		}

		avgTrain := totalLoss / float64(max(1, totalN))
		avgVal := 0.0
		if valN > 0 {
			avgVal = valLoss / float64(max(1, valN))
		}

		if valSize > 0 {
			if hasLRScheduler {
				fmt.Printf("epoch %d  loss=%.6f  val=%.6f  k=%.6f  n=%d  lr=%.6g  time=%s\n",
					ep, avgTrain, avgVal, pst.K, totalN, lrOpt.GetLR(), time.Since(t0))
			} else {
				fmt.Printf("epoch %d  loss=%.6f  val=%.6f  k=%.6f  n=%d  time=%s\n",
					ep, avgTrain, avgVal, pst.K, totalN, time.Since(t0))
			}
		} else if hasLRScheduler {
			fmt.Printf("epoch %d  loss=%.6f  k=%.6f  n=%d  lr=%.6g  time=%s\n",
				ep, avgTrain, pst.K, totalN, lrOpt.GetLR(), time.Since(t0))
		} else {
			fmt.Printf("epoch %d  loss=%.6f  k=%.6f  n=%d  time=%s\n",
				ep, avgTrain, pst.K, totalN, time.Since(t0))
		}

		if cfg.StatePath != "" {
			if err := SaveModelJSON(cfg.StatePath, fe, pst); err != nil {
				return err
			}
		}

		if earlyStopEnabled && valN > 0 && lrDrops >= cfg.MaxLRDrops && epochsNoImprove >= cfg.EarlyStopPatience {
			break
		}
	}
	return nil
}
