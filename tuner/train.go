// tuner/train.go
package tuner

import (
    "context"
    "fmt"
    "math/rand"
    "time"
)

// Train optimizes parameters of a Featurizer while retaining k in pst.
func Train(ctx context.Context, fe Featurizer, pst *PST, data []Sample, opt *AdaGrad, cfg TrainConfig, stmMode bool) error {
    params := fe.Params() // snapshot and ensure length
    grads := make([]float64, len(params))

    bs := cfg.Batch
    if bs <= 0 { bs = 32768 }

    rng := rand.New(rand.NewSource(42))
    order := make([]int, len(data))
    for i := range order { order[i] = i }

    // Define a fixed held-out split for k-refit (post-hoc) each epoch
    holdoutSize := cfg.KRefitCap
    if holdoutSize <= 0 { holdoutSize = 200000 }
    if holdoutSize > len(data) { holdoutSize = len(data) }
    trainSize := len(data) - holdoutSize
    if trainSize < 0 { trainSize = 0 }

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
            if end > trainSize { end = trainSize }

            loss, _, n := batchGradFeIdx(fe, pst, data, order, off, end, stmMode, cfg.L2, grads)
            totalLoss += loss
            totalN += n

            // Ensure grads slice matches current params length.
            params = fe.Params()
            if len(grads) != len(params) { grads = make([]float64, len(params)) }
            opt.Step(params, grads)
            fe.SetParams(params)
        }

        if cfg.AutoK {
            // Post-hoc k refit on held-out split only
            if holdoutSize > 0 {
                refitK(fe, pst, data[trainSize:len(data)], stmMode)
            }
        }

        fmt.Printf("epoch %d  loss=%.6f  k=%.6f  n=%d  time=%s\n",
            ep, totalLoss/float64(max(1,totalN)), pst.K, totalN, time.Since(t0))
    }
    return nil
}
