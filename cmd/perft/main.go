package main

import (
    "flag"
    "fmt"
    eng "github.com/Oliverans/GooseEngineMG/goosemg"
    "os"
    "sort"
    "time"
    "runtime/pprof"
)

func main() {
	fen := flag.String("fen", eng.FENStartPos, "FEN string (defaults to initial position)")
	depth := flag.Int("depth", 0, "Perft depth (required)")
	divide := flag.Bool("divide", false, "Print per-move node counts at root")
	repeat := flag.Int("repeat", 1, "Repeat perft N times and report aggregate (for steadier timings)")
    label := flag.String("label", "", "Optional label prefix for one-line output")
    cpuProf := flag.String("cpuprofile", "", "Write CPU profile to file during run")
    memProf := flag.String("memprofile", "", "Write heap profile to file after run")
    flag.Parse()

	if *depth <= 0 {
		fmt.Fprintln(os.Stderr, "-depth must be > 0")
		os.Exit(2)
	}

	board, err := eng.ParseFEN(*fen)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ParseFEN error: %v\n", err)
		os.Exit(2)
	}

	// Optional divide output
	if *divide {
		div := eng.PerftDivide(board, *depth)
		// Sort moves for stable output
		type kv struct {
			m eng.Move
			n uint64
		}
		arr := make([]kv, 0, len(div))
		var sum uint64
		for m, n := range div {
			arr = append(arr, kv{m, n})
			sum += n
		}
		sort.Slice(arr, func(i, j int) bool { return arr[i].m.String() < arr[j].m.String() })
		for _, x := range arr {
			fmt.Printf("%s: %d\n", x.m.String(), x.n)
		}
		fmt.Printf("Total: %d\n", sum)
		return
	}

    // Optional CPU profiling
    if *cpuProf != "" {
        f, err := os.Create(*cpuProf)
        if err != nil {
            fmt.Fprintf(os.Stderr, "creating cpuprofile: %v\n", err)
            os.Exit(2)
        }
        if err := pprof.StartCPUProfile(f); err != nil {
            fmt.Fprintf(os.Stderr, "start cpu profile: %v\n", err)
            os.Exit(2)
        }
        defer func() {
            pprof.StopCPUProfile()
            _ = f.Close()
        }()
    }

    // Timing loop
    var totalNodes uint64
    start := time.Now()
    for i := 0; i < *repeat; i++ {
        totalNodes += eng.Perft(board, *depth)
    }
    elapsed := time.Since(start)
    secs := elapsed.Seconds()
    nps := float64(totalNodes) / secs

    // Single line: Depth Nodes Time NPS
    fmt.Printf("%s \t%d \t\t%d \t\t%s \t%.0f\n", *label, *depth, totalNodes, elapsed, nps)

    // Optional heap profile after run
    if *memProf != "" {
        f, err := os.Create(*memProf)
        if err != nil {
            fmt.Fprintf(os.Stderr, "creating memprofile: %v\n", err)
            os.Exit(2)
        }
        if err := pprof.WriteHeapProfile(f); err != nil {
            fmt.Fprintf(os.Stderr, "write heap profile: %v\n", err)
            os.Exit(2)
        }
        _ = f.Close()
    }
}
