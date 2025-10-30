package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
)

// run executes a command and prints its combined output. Returns exit code.
func run(name string, args ...string) int {
	cmd := exec.Command(name, args...)
	cmd.Env = os.Environ()
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	fmt.Print(out.String())
	if err == nil {
		return 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode()
	}
	fmt.Fprintf(os.Stderr, "error running %s: %v\n", name, err)
	return 1
}

func main() {
	// Run all benchmarks in bench/ with benchmem.
	// Usage: go run ./cmd/benchrun
	// Print a simple header explaining Go's benchmark columns
	// Format: BenchmarkName  Iterations  ns/op  B/op  allocs/op
	fmt.Println("Columns: BENCHMARK  N  ns/op  B/op  allocs/op")
	code := run("go", "test", "./bench", "-run", "^$", "-bench", ".", "-benchmem", "-benchtime=1s")
	if code != 0 {
		os.Exit(code)
	}

	// Also run perft performance tests (macro throughput) with one-line outputs
	fmt.Println("\nPerft Performance:")
	fmt.Println("TEST \t\tDepth \t\tNodes \t\tTime \tNPS")
	// Initial position depth 3
	run("go", "run", "./cmd/perft", "-depth", "3", "-label", "Initial")
	// Initial position depth 4
	run("go", "run", "./cmd/perft", "-depth", "4", "-label", "Initial")
	// Initial position depth 5
	run("go", "run", "./cmd/perft", "-depth", "5", "-label", "Initial")
	// Initial position depth 6
	run("go", "run", "./cmd/perft", "-depth", "6", "-label", "Initial")
	// Kiwipete-ish middlegame depth 3
	_ = run("go", "run", "./cmd/perft", "-fen",
		"r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1",
		"-depth", "3", "-label", "Kiwipete")
	os.Exit(0)
}
