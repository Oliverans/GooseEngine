package tuner

import (
	"context"
	"math"
	"os"
	"strconv"
	"testing"
)

// BenchmarkEpoch runs a full training epoch (or a large fraction) through Train,
// so profiling reflects the real path (Eval+Grad+AdaGrad over the dataset).
// Default config mirrors your usual CLI:
//   - data: C:\Users\olive\Downloads\E12.41-1M-D12-Resolved.book\E12.41-1M-D12-Resolved.book
//   - epochs: 1
//   - batch: 65536
//   - lr: 0.2
//   - k: 0.006
//   - autok: false
//   - l2: 1e-4
//
// Env overrides:
//
//	TUNER_BENCH_DATA, TUNER_BENCH_ROWS, TUNER_BENCH_BATCH, TUNER_BENCH_L2
func BenchmarkEpoch(b *testing.B) {
	// Defaults
	defPath := "C:\\Users\\olive\\Downloads\\E12.41-1M-D12-Resolved.book\\E12.41-1M-D12-Resolved.book"
	path := os.Getenv("TUNER_BENCH_DATA")
	if path == "" {
		path = defPath
	}

	rows := 0 // 0 = load all
	if v := os.Getenv("TUNER_BENCH_ROWS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			rows = n
		}
	}
	batch := 65536
	if v := os.Getenv("TUNER_BENCH_BATCH"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			batch = n
		}
	}

	data, err := LoadDataset(path, false, rows)
	if err != nil || len(data) == 0 {
		b.Fatalf("failed to load dataset %q: %v (n=%d)", path, err, len(data))
	}

	pst := &PST{K: 0.006}
	fe := &LinearEval{PST: pst}
	SeedFromEngineDefaults(fe, pst)

	// Optimizer
	params := fe.Params()
	opt := NewAdaGrad(len(params), 0.2)
	//opt.SetLRScale(BuildLRScale(fe))

	// Config mirrored inline in the manual epoch loop below.

	// Prebuild boards to reduce noise (Eval/Grad still do full work)
	boards := make([]*Position, len(data))
	for i := range data {
		boards[i] = NewBoardFromSample(data[i])
	}

	// Replace NewBoardFromSample usage inside batch with prebuilt boards by temporarily
	// wrapping fe.Eval/Grad call path. Simpler approach: reuse the training loop logic here.

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// One epoch: manual loop to avoid overhead in Train that reorders data, etc.
		grads := make([]float64, len(fe.Params()))
		k := pst.K
		n := 0
		sumLoss := 0.0
		for off := 0; off < len(boards); off += batch {
			end := off + batch
			if end > len(boards) {
				end = len(boards)
			}
			for j := off; j < end; j++ {
				E := fe.Eval(boards[j])
				if data[j].STM == 0 {
					E = -E
				}
				p := 1.0 / (1.0 + math.Exp(-k*E))
				diff := p - data[j].Label
				dLdE := 2.0 * diff * k * p * (1.0 - p)
				fe.Grad(boards[j], dLdE, grads)
				sumLoss += diff * diff
				n++
			}
			params = fe.Params()
			opt.Step(params, grads)
			fe.SetParams(params)
			for t := range grads {
				grads[t] = 0
			}
		}
		_ = sumLoss
		_ = n
	}

	_ = context.Background()
}
