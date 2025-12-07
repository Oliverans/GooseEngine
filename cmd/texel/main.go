// cmd/texel/main.go
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"chess-engine/tuner"
)

var (
	dataPath  = flag.String("data", "", "Path to TSV/CSV with FEN and label")
	outJSON   = flag.String("out", "pst_out.json", "Where to write tuned PST as JSON")
	inJSON    = flag.String("init", "", "Optional JSON with initial PST and k")
	isCSV     = flag.Bool("csv", false, "Input is CSV (default TSV)")
	labelMode = flag.String("label", "white", `Label meaning: "white" (P(White wins)) or "side" (P(STM wins))`)
	epochs    = flag.Int("epochs", 3, "Training epochs")
	batchSize = flag.Int("batch", 32768, "Mini-batch size")
	lr        = flag.Float64("lr", 0.2, "AdaGrad base learning rate")
	l2        = flag.Float64("l2", 0.0, "L2 regularization (optional)")
	kScale    = flag.Float64("k", 0.004, "Logistic scale k for centipawns (try 0.003..0.006)")
	autoK     = flag.Bool("autok", false, "Re-fit k by 1D search + light gradient updates")
	shuffle   = flag.Bool("shuffle", true, "Shuffle each epoch")
	threads   = flag.Int("threads", runtime.NumCPU(), "GOMAXPROCS")
	maxRows   = flag.Int("max_rows", 0, "Optional cap on rows loaded (0=all)")
	summary   = flag.Bool("summary", false, "Print summary of tuned parameters")
)

func main() {
	flag.Parse()
	if *dataPath == "" {
		fmt.Println("Usage:")
		flag.PrintDefaults()
		os.Exit(2)
	}
	runtime.GOMAXPROCS(*threads)

	fmt.Printf("Loading dataset: %s\n", *dataPath)
	samps, err := tuner.LoadDataset(*dataPath, *isCSV, *maxRows)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Loaded %d samples\n", len(samps))

	var pst tuner.PST
	// Build featurizer (initially PST-only)
	fe := &tuner.LinearEval{PST: &pst}
	if *inJSON != "" {
		if err := tuner.LoadModelJSON(*inJSON, fe, &pst); err != nil {
			// Back-compat: try old PST-only JSON if model JSON fails
			if err2 := tuner.LoadJSON(*inJSON, &pst); err2 != nil {
				panic(err)
			}
		}
		fmt.Printf("Loaded init weights from %s\n", *inJSON)
	} else {
		pst.K = *kScale
		// Seed default θ from engine evaluation constants
		tuner.SeedFromEngineDefaults(fe, &pst)
	}

	// Ensure eval and train toggles are initialized
	fe.Toggles = tuner.DefaultEvalToggles()

	// Size optimizer to full I, length
	opt := tuner.NewAdam(len(fe.Params()), *lr, *l2)
	//lrScale := tuner.BuildLRScale(fe)
	//opt.SetLRScale(lrScale)

	ctx := context.Background()
	stmMode := strings.EqualFold(*labelMode, "side")

	cfg := tuner.TrainConfig{
		Epochs:    *epochs,
		Batch:     *batchSize,
		LR:        *lr,
		L2:        *l2,
		AutoK:     *autoK,
		Shuffle:   *shuffle,
		KRefitCap: 200000,
	}

	//fmt.Printf("Num params: %d\n", len(fe.Params()))
	//fmt.Printf("Toggles: %+v\n", fe.Toggles)

	//fmt.Println("=== BEFORE TRAIN ===")
	printSummary(fe, &pst)

	if err := tuner.Train(ctx, fe, &pst, samps, opt, cfg, stmMode); err != nil {
		panic(err)
	}

	//fmt.Println("=== AFTER TRAIN ===")
	printSummary(fe, &pst)

	if err := os.MkdirAll(filepath.Dir(*outJSON), 0o755); err != nil && !os.IsExist(err) {
		panic(err)
	}
	if *summary {
		printSummary(fe, &pst)
	}
	if err := tuner.SaveModelJSON(*outJSON, fe, &pst); err != nil {
		panic(err)
	}
	fmt.Printf("Saved tuned PST to %s\n", *outJSON)
}

func printSummary(fe tuner.Featurizer, pst *tuner.PST) {
	//fmt.Println("=== Training Summary ===")
	//fmt.Printf("k = %.6f\n", pst.K)
	// PST ranges
	minMG, maxMG := 1e18, -1e18
	minEG, maxEG := 1e18, -1e18
	for pt := 0; pt < 6; pt++ {
		for sq := 0; sq < 64; sq++ {
			v := pst.MG[pt][sq]
			if v < minMG {
				minMG = v
			}
			if v > maxMG {
				maxMG = v
			}
			w := pst.EG[pt][sq]
			if w < minEG {
				minEG = w
			}
			if w > maxEG {
				maxEG = w
			}
		}
	}
	//fmt.Printf("PST MG range: [%.1f, %.1f]\n", minMG, maxMG)
	//fmt.Printf("PST EG range: [%.1f, %.1f]\n", minEG, maxEG)

	if le, ok := fe.(*tuner.LinearEval); ok {
		// Material
		//fmt.Printf("Material MG (P,N,B,R,Q,K): %.1f %.1f %.1f %.1f %.1f %.1f\n",
		//le.MatMG[0], le.MatMG[1], le.MatMG[2], le.MatMG[3], le.MatMG[4], le.MatMG[5])
		//fmt.Printf("Material EG (P,N,B,R,Q,K): %.1f %.1f %.1f %.1f %.1f %.1f\n",
		//le.MatEG[0], le.MatEG[1], le.MatEG[2], le.MatEG[3], le.MatEG[4], le.MatEG[5])
		// Passers (aggregate by rank for readability)
		var rankMG [8]float64
		var rankEG [8]float64
		for sq := 0; sq < 64; sq++ {
			r := sq / 8
			rankMG[r] += le.PasserMG[sq]
			rankEG[r] += le.PasserEG[sq]
		}
	}
}
