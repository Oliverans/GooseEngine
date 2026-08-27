package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"

	"chess-engine/engine"
	gm "chess-engine/goosemg"
)

func BenchmarkMain(b *testing.B) {
	board := gm.ParseFen(gm.Startpos) // the game board
	var bestmove = engine.StartSearch(&board, engine.SearchLimits{Depth: 50}, false, false, false)
	engine.SearchState.ResetForNewGame()
	fmt.Println("bestmove ", bestmove)
}

func TestParseGoLimits(t *testing.T) {
	got, messages, valid := parseGoLimits("go wtime 10000 btime 9000 winc 100 binc 200 movestogo 12 depth 20 movetime 750 nodes 50000 infinite")
	want := engine.SearchLimits{
		Depth:      20,
		MoveTimeMs: 750,
		Nodes:      50000,
		Infinite:   true,
		WTime:      10000,
		BTime:      9000,
		WInc:       100,
		BInc:       200,
		MovesToGo:  12,
	}
	if !valid || len(messages) != 0 {
		t.Fatalf("parseGoLimits valid = %v, messages = %v", valid, messages)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseGoLimits() = %+v, want %+v", got, want)
	}
}

func TestParseGoLimitsRejectsMalformedValues(t *testing.T) {
	_, messages, valid := parseGoLimits("go movetime nope nodes 0 depth 101")
	if valid {
		t.Fatal("parseGoLimits accepted malformed limits")
	}
	if len(messages) != 3 {
		t.Fatalf("messages = %v, want three diagnostics", messages)
	}
}

func TestUCIAsyncStopAndBusyCommands(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestUCIHelperProcess$")
	cmd.Env = append(os.Environ(), "GOOSE_UCI_HELPER=1")
	cmd.Stdin = strings.NewReader("position startpos\ngo infinite\nisready\nposition startpos\nstop\nquit\n")
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("UCI helper timed out: %v\n%s", ctx.Err(), output)
	}
	if err != nil {
		t.Fatalf("UCI helper failed: %v\n%s", err, output)
	}

	text := string(output)
	for _, want := range []string{
		"readyok",
		"info string Command position rejected while searching",
		"bestmove ",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("UCI output missing %q:\n%s", want, text)
		}
	}
}

func TestUCIInfoIncludesSelectiveDepthAndHashfull(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestUCIHelperProcess$")
	cmd.Env = append(os.Environ(), "GOOSE_UCI_HELPER=1")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Fprintln(stdin, "position startpos\ngo depth 1"); err != nil {
		t.Fatal(err)
	}

	var lines []string
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		lines = append(lines, line)
		if strings.HasPrefix(line, "bestmove ") {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	_, _ = fmt.Fprintln(stdin, "quit")
	_ = stdin.Close()
	if err := cmd.Wait(); err != nil {
		t.Fatalf("UCI helper failed: %v\n%s", err, stderr.String())
	}

	output := strings.Join(lines, "\n")
	if !strings.Contains(output, "info depth 1 seldepth ") || !strings.Contains(output, " hashfull ") {
		t.Fatalf("UCI info missing seldepth or hashfull:\n%s", output)
	}
}

func TestUCIHelperProcess(t *testing.T) {
	if os.Getenv("GOOSE_UCI_HELPER") != "1" {
		return
	}
	uciLoop()
	os.Exit(0)
}
