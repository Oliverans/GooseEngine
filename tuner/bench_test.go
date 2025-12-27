package tuner

import (
	"math"
	"os"
	"strconv"
	"testing"
)

// getBenchData tries to load a dataset from env var TUNER_BENCH_DATA.
// If missing, it returns a small synthetic set built from a few fixed FENs.
func getBenchData(max int) []Sample {
	// Default dataset path (Windows): matches your usual training path
	defPath := "C:\\Users\\olive\\Downloads\\E12.41-1M-D12-Resolved.book\\E12.41-1M-D12-Resolved.book"
	path := os.Getenv("TUNER_BENCH_DATA")
	if path == "" {
		path = defPath
	}
	if path != "" {
		samples, err := LoadDataset(path, false, max)
		if err == nil && len(samples) > 0 {
			return samples
		}
	}
	// Synthetic fallback: a few diverse positions (white/black to move) with labels
	fens := []struct{ fen, lab string }{
		{"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w - - 0 1", "0.5"},
		{"r1bqkbnr/pppp1ppp/2n5/4p3/1b1P4/5NP1/PPPNPPBP/R1BQK2R w KQkq - 4 6", "0.6"},
		{"r2q1rk1/pp1nbppp/2p1bn2/3p2B1/3P4/2N1PN2/PPQ2PPP/R3KB1R w KQ - 2 10", "0.55"},
		{"r1bq1rk1/pp2bppp/2n1pn2/2pp4/3P1B2/2P1PN2/PP1NBPPP/R2Q1RK1 w - - 6 8", "0.52"},
		{"r4rk1/1bqnbppp/p1n1p3/1pppP3/3P1P2/2PBBN2/PP1QN1PP/2KR3R w - - 0 14", "0.55"},
		{"r1bq1rk1/ppp2ppp/2n2n2/3pp3/1b1P4/2P1PN2/PP1N1PPP/R1BQKB1R w KQ - 2 7", "0.48"},
	}
	out := make([]Sample, 0, max)
	for len(out) < max {
		for _, it := range fens {
			if len(out) >= max {
				break
			}
			y, _ := parseLabel(it.lab)
			s, err := fenToSample(it.fen, y)
			if err == nil {
				out = append(out, s)
			}
		}
	}
	return out
}

// BenchmarkMain exercises a representative inner-loop: computing grads over batches
// and applying AdaGrad steps. Use env vars to control size:
//   - TUNER_BENCH_DATA: path to dataset (optional)
//   - TUNER_BENCH_ROWS: max rows to load (default 200000)
//   - TUNER_BENCH_BATCH: batch size (default 32768)
func BenchmarkMain(b *testing.B) {
	// Controls
	rows := 200000
	if v := os.Getenv("TUNER_BENCH_ROWS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			rows = n
		}
	}
	// Default batch 65536 to mirror your CLI runs
	batch := 65536
	if v := os.Getenv("TUNER_BENCH_BATCH"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			batch = n
		}
	}

	// Init model
	pst := &PST{K: 0.006}
	fe := &LinearEval{PST: pst}
	SeedFromEngineDefaults(fe, pst)

	// Optimizer with per-parameter LR scale
	params := fe.Params()
	// Defaults: lr=0.2, l2=1e-4
	opt := NewAdaGrad(len(params), 0.2)
	//opt.SetLRScale(BuildLRScale(fe))

	// Data
	data := getBenchData(rows)
	if len(data) == 0 {
		b.Fatalf("no data for benchmark")
	}
	// Prebuild boards to avoid parsing in the timed section
	boards := make([]*Position, len(data))
	for i := range data {
		boards[i] = NewBoardFromSample(data[i])
	}

	b.ReportAllocs()
	b.ResetTimer()

	grads := make([]float64, len(params))
	for i := 0; i < b.N; i++ {
		// One pass over data in batches
		sumLoss := 0.0
		n := 0
		for off := 0; off < len(boards); off += batch {
			end := off + batch
			if end > len(boards) {
				end = len(boards)
			}
			// Compute grads for this batch
			for j := off; j < end; j++ {
				E := fe.Eval(boards[j])
				// k defaults to 0.006; use sample label for realism
				p := 1.0 / (1.0 + math.Exp(-pst.K*E))
				diff := p - data[j].Label
				dLdE := 2.0 * diff * pst.K * p * (1.0 - p)
				fe.Grad(boards[j], dLdE, grads)
				sumLoss += diff * diff
				n++
			}
			// Apply step
			params = fe.Params()
			opt.Step(params, grads)
			fe.SetParams(params)
			// Zero grads
			for k := range grads {
				grads[k] = 0
			}
		}
		_ = sumLoss
		_ = n
	}
}
