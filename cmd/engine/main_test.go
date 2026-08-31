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

func TestSetIIRMinDepth(t *testing.T) {
	oldDepth := engine.IIRMinDepth
	t.Cleanup(func() { engine.IIRMinDepth = oldDepth })

	handleSetOption("setoption name IIRMinDepth value 6")
	if engine.IIRMinDepth != 6 {
		t.Fatalf("IIRMinDepth = %d", engine.IIRMinDepth)
	}
}

func TestSetLMROptions(t *testing.T) {
	oldCutnode := engine.LMRCutnode
	oldTTPv := engine.LMRTTPv
	oldNoisyOffset := engine.LMRNoisyOffset
	oldCheckBonus := engine.LMRCheckBonus
	oldDeeperBase := engine.LMRDeeperBase
	oldCaptureHistoryDivisor := engine.LMRCaptureHistoryDivisor
	t.Cleanup(func() {
		engine.LMRCutnode = oldCutnode
		engine.LMRTTPv = oldTTPv
		engine.LMRNoisyOffset = oldNoisyOffset
		engine.LMRCheckBonus = oldCheckBonus
		engine.LMRDeeperBase = oldDeeperBase
		engine.LMRCaptureHistoryDivisor = oldCaptureHistoryDivisor
	})

	handleSetOption("setoption name LMRCutnode value 120")
	handleSetOption("setoption name LMRTTPv value 60")
	handleSetOption("setoption name LMRNoisyOffset value -90")
	handleSetOption("setoption name LMRCheckBonus value -110")
	handleSetOption("setoption name LMRDeeperBase value 55")
	handleSetOption("setoption name LMRCaptureHistoryDivisor value 60")

	if engine.LMRCutnode != 120 || engine.LMRTTPv != 60 || engine.LMRNoisyOffset != -90 || engine.LMRCheckBonus != -110 || engine.LMRDeeperBase != 55 || engine.LMRCaptureHistoryDivisor != 60 {
		t.Fatalf("LMR options = %d/%d/%d/%d/%d/%d", engine.LMRCutnode, engine.LMRTTPv, engine.LMRNoisyOffset, engine.LMRCheckBonus, engine.LMRDeeperBase, engine.LMRCaptureHistoryDivisor)
	}
}

func TestSetLMRCaptureHistoryDivisorBounds(t *testing.T) {
	oldDivisor := engine.LMRCaptureHistoryDivisor
	t.Cleanup(func() { engine.LMRCaptureHistoryDivisor = oldDivisor })

	handleSetOption("setoption name LMRCaptureHistoryDivisor value 20")
	if engine.LMRCaptureHistoryDivisor != 20 {
		t.Fatalf("LMRCaptureHistoryDivisor = %d, want 20", engine.LMRCaptureHistoryDivisor)
	}
	handleSetOption("setoption name LMRCaptureHistoryDivisor value 19")
	handleSetOption("setoption name LMRCaptureHistoryDivisor value 101")
	if engine.LMRCaptureHistoryDivisor != 20 {
		t.Fatalf("out-of-range divisor changed value to %d", engine.LMRCaptureHistoryDivisor)
	}
	handleSetOption("setoption name LMRCaptureHistoryDivisor value 100")
	if engine.LMRCaptureHistoryDivisor != 100 {
		t.Fatalf("LMRCaptureHistoryDivisor = %d, want 100", engine.LMRCaptureHistoryDivisor)
	}
}

func TestSetSEENoisyHistoryDivisorBounds(t *testing.T) {
	oldDivisor := engine.SEENoisyHistoryDivisor
	t.Cleanup(func() { engine.SEENoisyHistoryDivisor = oldDivisor })

	handleSetOption("setoption name SEENoisyHistoryDivisor value 64")
	if engine.SEENoisyHistoryDivisor != 64 {
		t.Fatalf("SEENoisyHistoryDivisor = %d, want 64", engine.SEENoisyHistoryDivisor)
	}
	handleSetOption("setoption name SEENoisyHistoryDivisor value 63")
	handleSetOption("setoption name SEENoisyHistoryDivisor value 257")
	if engine.SEENoisyHistoryDivisor != 64 {
		t.Fatalf("out-of-range divisor changed value to %d", engine.SEENoisyHistoryDivisor)
	}
	handleSetOption("setoption name SEENoisyHistoryDivisor value 256")
	if engine.SEENoisyHistoryDivisor != 256 {
		t.Fatalf("SEENoisyHistoryDivisor = %d, want 256", engine.SEENoisyHistoryDivisor)
	}
}

func TestSetCaptureFutilityOptions(t *testing.T) {
	oldBase := engine.CaptureFutilityBase
	oldScale := engine.CaptureFutilityScale
	oldMaxDepth := engine.CaptureFutilityMaxDepth
	oldDivisor := engine.CaptureFutilityHistoryDivisor
	t.Cleanup(func() {
		engine.CaptureFutilityBase = oldBase
		engine.CaptureFutilityScale = oldScale
		engine.CaptureFutilityMaxDepth = oldMaxDepth
		engine.CaptureFutilityHistoryDivisor = oldDivisor
	})

	handleSetOption("setoption name CaptureFutilityBase value 120")
	handleSetOption("setoption name CaptureFutilityScale value 90")
	handleSetOption("setoption name CaptureFutilityMaxDepth value 3")
	handleSetOption("setoption name CaptureFutilityHistoryDivisor value 100")
	if engine.CaptureFutilityBase != 120 || engine.CaptureFutilityScale != 90 || engine.CaptureFutilityMaxDepth != 3 || engine.CaptureFutilityHistoryDivisor != 100 {
		t.Fatalf("capture futility options = %d/%d/%d/%d", engine.CaptureFutilityBase, engine.CaptureFutilityScale, engine.CaptureFutilityMaxDepth, engine.CaptureFutilityHistoryDivisor)
	}

	handleSetOption("setoption name CaptureFutilityBase value 401")
	handleSetOption("setoption name CaptureFutilityScale value 24")
	handleSetOption("setoption name CaptureFutilityMaxDepth value 9")
	handleSetOption("setoption name CaptureFutilityHistoryDivisor value 63")
	if engine.CaptureFutilityBase != 120 || engine.CaptureFutilityScale != 90 || engine.CaptureFutilityMaxDepth != 3 || engine.CaptureFutilityHistoryDivisor != 100 {
		t.Fatalf("out-of-range values changed capture futility options")
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
