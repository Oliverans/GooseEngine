package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"runtime/pprof"
	"time"

	"chess-engine/engine"
	gm "chess-engine/goosemg"
)

func main() {
	// --- Flags ---
	depthFlag := flag.Int("depth", 10, "search depth in plies")
	repeatFlag := flag.Int("repeat", 1, "number of searches to run")
	fenFlag := flag.String("fen", "", "FEN to search (empty = startpos)")
	cpuProfile := flag.String("cpuprofile", "", "write CPU profile to file")
	memProfile := flag.String("memprofile", "", "write memory profile (heap) to file")
	flag.Parse()

	if *depthFlag <= 0 {
		log.Fatalf("depth must be positive, got %d", *depthFlag)
	}

	// --- Optional CPU profiling setup ---
	var cpuFile *os.File
	var err error
	if *cpuProfile != "" {
		cpuFile, err = os.Create(*cpuProfile)
		if err != nil {
			log.Fatalf("could not create CPU profile: %v", err)
		}
		if err := pprof.StartCPUProfile(cpuFile); err != nil {
			log.Fatalf("could not start CPU profile: %v", err)
		}
		defer func() {
			pprof.StopCPUProfile()
			cpuFile.Close()
		}()
	}

	// --- Engine / board setup (mirrors uci.go behavior) ---
	engine.History.History = make([]uint64, 500)

	// This mimics "go depth N" in your UCI:
	// timeToUse defaults to 250000 ms (no wtime/btime given).
	timeToUseMs := 250000
	incMs := 0
	useCustomDepth := true
	evalOnly := false
	moveOrderingOnly := false

	// FEN selection
	fen := gm.Startpos
	if *fenFlag != "" {
		fen = *fenFlag
	}

	depth := *depthFlag
	repeat := *repeatFlag

	fmt.Printf("searchbench: fen=%q depth=%d repeat=%d\n", fen, depth, repeat)

	startAll := time.Now()
	for i := 0; i < repeat; i++ {
		// Fresh position for each run
		board := gm.ParseFen(fen)

		// Match your UCI setup / new game handling
		engine.ResetForNewGame()
		engine.ResetStateTracking(&board)
		engine.GlobalStop = false

		iterStart := time.Now()
		bestMove := engine.StartSearch(
			&board,
			uint8(depth),
			timeToUseMs,
			incMs,
			useCustomDepth,
			evalOnly,
			moveOrderingOnly,
		)
		iterElapsed := time.Since(iterStart)

		fmt.Printf("iteration %d: bestmove %v  time=%v\n", i+1, bestMove, iterElapsed)
	}
	totalElapsed := time.Since(startAll)
	fmt.Printf("total time: %v\n", totalElapsed)

	// --- Optional heap profile at the end ---
	if *memProfile != "" {
		f, err := os.Create(*memProfile)
		if err != nil {
			log.Fatalf("could not create memory profile: %v", err)
		}
		defer f.Close()

		runtime.GC() // get up-to-date heap info
		if err := pprof.WriteHeapProfile(f); err != nil {
			log.Fatalf("could not write memory profile: %v", err)
		}
	}
}
