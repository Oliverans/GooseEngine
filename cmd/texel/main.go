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
	dataPath        = flag.String("data", "", "Path to TSV/CSV with FEN and label")
	outJSON         = flag.String("out", "pst_out.json", "Where to write tuned PST as JSON")
	inJSON          = flag.String("init", "", "Optional JSON with initial PST and k")
	isCSV           = flag.Bool("csv", false, "Input is CSV (default TSV)")
	binary          = flag.Bool("binary", false, "Input is binary format (default: TSV/CSV)")
	enableLRScaling = flag.Bool("lr-scaling", true, "Enable per-parameter LR scaling")
	enableAnchoring = flag.Bool("anchoring", true, "Enable anchored L2 regularization")
	tier1LR         = flag.Float64("tier1-lr", 0.3, "LR multiplier for Tier 1 params")
	tier1Anchor     = flag.Float64("tier1-anchor", 0.1, "Anchor weight for Tier 1 params")
	labelMode       = flag.String("label", "white", `Label meaning: "white" (P(White wins)) or "side" (P(STM wins))`)
	epochs          = flag.Int("epochs", 3, "Training epochs")
	batchSize       = flag.Int("batch", 32768, "Mini-batch size")
	lr              = flag.Float64("lr", 0.2, "AdaGrad base learning rate")
	kScale          = flag.Float64("k", 0.004, "Logistic scale k for centipawns (try 0.003..0.006)")
	autoK           = flag.Bool("autok", false, "Re-fit k by 1D search + light gradient updates")
	shuffle         = flag.Bool("shuffle", true, "Shuffle each epoch")
	threads         = flag.Int("threads", runtime.NumCPU(), "GOMAXPROCS")
	maxRows         = flag.Int("max_rows", 0, "Optional cap on rows loaded (0=all)")
	summary         = flag.Bool("summary", false, "Print summary of tuned parameters")
	valCap          = flag.Int("val_cap", 0, "Validation set size (0=unused)")
	valFrac         = flag.Float64("val_frac", 0, "Validation set fraction of remaining data (0=unused)")
	valFromKRefit   = flag.Bool("val_from_krefit", false, "Reuse K-refit holdout as validation set")
	plateauPatience = flag.Int("plateau_patience", 0, "Epochs without val improvement before LR drop (0=off)")
	plateauMinDelta = flag.Float64("plateau_min_delta", 0, "Minimum val loss improvement to reset plateau")
	lrDropFactor    = flag.Float64("lr_drop_factor", 0.5, "LR multiplier on plateau (e.g. 0.5)")
	lrMin           = flag.Float64("lr_min", 0, "Minimum LR when reducing on plateau")
	lrDropCooldown  = flag.Int("lr_drop_cooldown", 0, "Cooldown epochs after LR drop")
	maxLRDrops      = flag.Int("max_lr_drops", 0, "Maximum LR drops before early stop (0=unlimited)")
	earlyStopPat    = flag.Int("early_stop_patience", 0, "Epochs without val improvement after max drops (0=off)")
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
	var samps []tuner.BinarySample
	var err error
	if *binary {
		samps, err = tuner.LoadBinaryDataset(*dataPath, *maxRows)
	} else {
		// Load text format and convert to binary format
		textSamps, loadErr := tuner.LoadDataset(*dataPath, *isCSV, *maxRows)
		if loadErr != nil {
			panic(loadErr)
		}
		// Convert to binary format for memory efficiency
		samps = make([]tuner.BinarySample, len(textSamps))
		for i := range textSamps {
			samps[i] = textSamps[i].ToBinary()
		}
	}
	if err != nil {
		panic(err)
	}
	fmt.Printf("Loaded %d samples\n", len(samps))

	statePath := makeStatePath(*outJSON)
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil && !os.IsExist(err) {
		panic(err)
	}

	var pst tuner.PST
	fe := &tuner.LinearEval{PST: &pst}
	if *inJSON != "" {
		if err := tuner.LoadModelJSON(*inJSON, fe, &pst); err != nil {
			if err2 := tuner.LoadJSON(*inJSON, &pst); err2 != nil {
				panic(err)
			}
		}
		fmt.Printf("Loaded init weights from %s\n", *inJSON)
	} else {
		pst.K = *kScale
		tuner.SeedFromEngineDefaults(fe, &pst)
	}

	fe.Toggles = tuner.DefaultEvalToggles()

	opt := tuner.NewAdam(len(fe.Params()), *lr)

	ctx := context.Background()
	stmMode := strings.EqualFold(*labelMode, "side")

	// Build LR scale and anchor configs with optional Tier 1 overrides.
	lrCfg := tuner.DefaultLRScales()
	lrCfg.PawnPhalanx = *tier1LR
	lrCfg.PawnWeakLever = *tier1LR
	lrCfg.KnightOutpost = *tier1LR
	lrCfg.BishopOutpost = *tier1LR
	lrCfg.KingSemiOpenFile = *tier1LR
	lrCfg.KingOpenFile = *tier1LR

	anchorCfg := tuner.DefaultAnchorConfig()
	anchorCfg.Tier1Lambda = *tier1Anchor

	cfg := tuner.TrainConfig{
		Epochs:            *epochs,
		Batch:             *batchSize,
		LR:                *lr,
		AutoK:             *autoK,
		Shuffle:           *shuffle,
		KRefitCap:         200000,
		LRScaling:         *enableLRScaling,
		Anchoring:         *enableAnchoring,
		LRScaleCfg:        lrCfg,
		AnchorCfg:         anchorCfg,
		StatePath:         statePath,
		ValCap:            *valCap,
		ValFrac:           *valFrac,
		UseKRefitAsVal:    *valFromKRefit,
		PlateauPatience:   *plateauPatience,
		PlateauMinDelta:   *plateauMinDelta,
		LRReduceFactor:    *lrDropFactor,
		LRMin:             *lrMin,
		LRDropCooldown:    *lrDropCooldown,
		MaxLRDrops:        *maxLRDrops,
		EarlyStopPatience: *earlyStopPat,
	}
	if cfg.EarlyStopPatience > 0 && cfg.PlateauPatience > 0 && cfg.EarlyStopPatience <= cfg.PlateauPatience {
		cfg.EarlyStopPatience = cfg.PlateauPatience + 1
	}
	printSummary(fe, &pst)

	if err := tuner.Train(ctx, fe, &pst, samps, opt, cfg, stmMode); err != nil {
		panic(err)
	}

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

func makeStatePath(out string) string {
	if out == "" {
		return "pst_out_state.json"
	}
	dir := filepath.Dir(out)
	base := filepath.Base(out)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	if ext == "" {
		return filepath.Join(dir, name+"_state.json")
	}
	if strings.EqualFold(ext, ".json") {
		return filepath.Join(dir, name+"_state"+ext)
	}
	return filepath.Join(dir, name+"_state.json")
}

func printSummary(fe tuner.Featurizer, pst *tuner.PST) {
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

	if le, ok := fe.(*tuner.LinearEval); ok {
		var rankMG [8]float64
		var rankEG [8]float64
		for sq := 0; sq < 64; sq++ {
			r := sq / 8
			rankMG[r] += le.PasserMG[sq]
			rankEG[r] += le.PasserEG[sq]
		}
	}
}
