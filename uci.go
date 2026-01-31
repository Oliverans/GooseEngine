package main

import (
	"bufio"
	"chess-engine/engine"
	"fmt"
	"os"
	"strconv"
	"strings"

	gm "chess-engine/goosemg"
)

// Standard bench positions used by many chess engines
var benchPositions = []string{
	"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
	"r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1",
	"8/2p5/3p4/KP5r/1R3p1k/8/4P1P1/8 w - - 0 1",
	"r3k2r/Pppp1ppp/1b3nbN/nP6/BBP1P3/q4N2/Pp1P2PP/R2Q1RK1 w kq - 0 1",
	"rnbq1k1r/pp1Pbppp/2p5/8/2B5/8/PPP1NnPP/RNBQK2R w KQ - 1 8",
	"r4rk1/1pp1qppp/p1np1n2/2b1p1B1/2B1P1b1/P1NP1N2/1PP1QPPP/R4RK1 w - - 0 10",
	"r3k2r/1bp1qpb1/p1np1np1/4p2p/2P1P3/1PN2N1P/PB1PQPB1/R3K2R w KQkq - 0 1",
	"2kr3r/pbpn1pq1/1p2pn1p/3p2p1/2PP4/P1N1P1P1/1PQ1NPBP/R4RK1 w - - 0 1",
	"r2qk2r/ppp1bppp/2n1bn2/3pp3/8/2NPBNP1/PPP1PPBP/R2QK2R w KQkq - 0 1",
	"r1bq1rk1/ppp2ppp/2nb1n2/3pp3/2B1P3/2NP1N2/PPP2PPP/R1BQ1RK1 w - - 0 1",
}

const benchDepth = 11

// runBench runs a benchmark search on standard positions and reports total nodes
func runBench() {
	totalNodes := 0
	var totalTimeSpent int64 = 0

	for _, fen := range benchPositions {
		board := gm.ParseFen(fen)
		engine.SearchState.ResetForNewGame()

		// Search with fixed depth, large time, no time-based cutoff
		engine.StartSearch(&board, uint8(benchDepth), 1000000, 0, true, false, false, false)

		// Accumulate nodes
		totalNodes += engine.GetNodeCount()
		totalTimeSpent += engine.GetTimeSpent()
	}

	nps := uint64(float64(totalNodes*1000) / float64(totalTimeSpent))

	fmt.Printf("%d nodes %d nps\n", totalNodes, nps)
}

// parseIntOption parses an integer value from "setoption name X value Y" commands
func parseIntOption(scanner *bufio.Scanner, optionName string) (int, bool) {
	if !scanner.Scan() {
		fmt.Printf("info string Malformed setoption for %s\n", optionName)
		return 0, false
	}
	scanner.Scan()
	val, err := strconv.Atoi(scanner.Text())
	if err != nil {
		fmt.Printf("info string Malformed value for %s: %v\n", optionName, err)
		return 0, false
	}
	return val, true
}

// UCI options with bounds and setter
type uciOption struct {
	min, max int
	setter   func(int)
}

var uciThreads = 1

var uciOptionSetters = map[string]uciOption{
	"hash":    {1, 4096, func(v int) { engine.TTSize = v }},
	"threads": {1, 1, func(v int) { uciThreads = v }},

	"futilitybase":  {10, 30, func(v int) { engine.FutilityBase = int32(v) }},
	"futilityscale": {50, 150, func(v int) { engine.FutilityScale = int32(v) }},

	"rfpscale":      {50, 150, func(v int) { engine.RFPScale = int32(v) }},
	"razoringscale": {100, 200, func(v int) { engine.RazoringScale = int32(v) }},

	"lmpoffset":       {1, 6, func(v int) { engine.LMPOffset = v }},
	"lmrdepthlimit":   {2, 20, func(v int) { engine.LMRDepthLimit = int8(v) }},
	"lmrmovelimit":    {2, 8, func(v int) { engine.LMRMoveLimit = v }},
	"lmrhistorybonus": {450, 550, func(v int) { engine.LMRHistoryBonus = v }},
	"lmrhistorymalus": {-150, -50, func(v int) { engine.LMRHistoryMalus = v }},

	"nullmovemindepth":    {0, 10, func(v int) { engine.NullMoveMinDepth = int8(v) }},
	"nmmarginbase":        {120, 250, func(v int) { engine.NullMoveMinDepth = int8(v) }},
	"nmmargindepth":       {10, 25, func(v int) { engine.NullMoveMinDepth = int8(v) }},
	"quiescenceseemargin": {100, 200, func(v int) { engine.QuiescenceSeeMargin = v }},
	"probcutseemargin":    {100, 200, func(v int) { engine.ProbCutSeeMargin = v }},

	"deltamargin":          {100, 300, func(v int) { engine.DeltaMargin = int32(v) }},
	"aspirationwindowsize": {10, 100, func(v int) { engine.AspirationWindowSize = int32(v) }},
}

func main() {
	if len(os.Args) >= 2 && os.Args[1] == "bench" {
		runBench()
		os.Exit(0)
	}
	uciLoop()
}

func uciLoop() {
	scanner := bufio.NewScanner(os.Stdin)
	board := gm.ParseFen(gm.Startpos) // the game board

	var evalOnly = false
	var moveOrderingOnly = false
	var printSearchInformation = true

	for scanner.Scan() {
		line := scanner.Text()
		tokens := strings.Fields(line)
		if len(tokens) == 0 { // ignore blank lines
			continue
		}
		switch strings.ToLower(tokens[0]) {
		case "bench":
			runBench()
		case "eval":
			evalOnly = true
		case "HideSearchInfo":
			printSearchInformation = !printSearchInformation
		case "moveordering":
			moveOrderingOnly = true
		case "cutstats":
			engine.PrintCutStats = true
		case "uci":
			fmt.Println("id name GooseEngine Alpha version 0.2")
			fmt.Println("id author Goose")

			fmt.Printf("option name Hash type spin default %d min 1 max 4096\n", engine.TTSize)
			fmt.Printf("option name Threads type spin default %d min 1 max 1\n", uciThreads)

			// --- Search / pruning parameters exposed as UCI options ---

			// Futility margins (node-level) - base ±50
			fmt.Printf("option name FutilityBase type spin default %d min 10 max 30\n", engine.FutilityBase)
			fmt.Printf("option name FutilityScale type spin default %d min 50 max 150\n", engine.FutilityScale)

			// Reverse Futility Pruning (Static Null Move) margins - base ±50
			fmt.Printf("option name RFPScale type spin default %d min 50 max 150\n", engine.RFPScale)

			// Razoring margins - base ±50
			fmt.Printf("option name RazoringScale type spin default %d min 100 max 200\n", engine.RazoringScale)

			// LMR (Late Move Reductions) knobs
			fmt.Printf("option name LMRDepthLimit type spin default %d min 0 max 20\n", engine.LMRDepthLimit)

			// Null-move pruning knobs
			fmt.Printf("option name NullMoveMinDepth type spin default %d min 2 max 10\n", engine.NullMoveMinDepth)
			fmt.Printf("option name NMMarginBase type spin default %d min 120 max 250\n", engine.NMMarginBase)
			fmt.Printf("option name NMMarginDepth type spin default %d min 10 max 25\n", engine.NMMarginDepth)

			// Additional LMP margins - base ±3
			fmt.Printf("option name LMPOffset type spin default %d min 1 max 6\n", engine.LMPOffset)

			// LMR parameters - base ±50 for history values
			fmt.Printf("option name LMRMoveLimit type spin default %d min 1 max 5\n", engine.LMRMoveLimit)
			fmt.Printf("option name LMRHistoryBonus type spin default %d min 450 max 550\n", engine.LMRHistoryBonus)
			fmt.Printf("option name LMRHistoryMalus type spin default %d min -150 max -50\n", engine.LMRHistoryMalus)

			// SEE pruning parameters
			fmt.Printf("option name QuiescenceSeeMargin type spin default %d min 100 max 200\n", engine.QuiescenceSeeMargin)
			fmt.Printf("option name ProbCutSeeMargin type spin default %d min 100 max 200\n", engine.ProbCutSeeMargin)

			// Other search parameters
			fmt.Printf("option name DeltaMargin type spin default %d min 100 max 300\n", engine.DeltaMargin)
			fmt.Printf("option name AspirationWindowSize type spin default %d min 10 max 100\n", engine.AspirationWindowSize)

			fmt.Println("uciok")
		case "isready":
			fmt.Println("readyok")
		case "ucinewgame":
			board = gm.ParseFen(gm.Startpos)
			engine.SearchState.ResetForNewGame()
		case "quit":
			return
		case "stop":
			engine.SearchState.RequestStop()
		case "go":
			goScanner := bufio.NewScanner(strings.NewReader(line))
			goScanner.Split(bufio.ScanWords)
			goScanner.Scan() // skip the first token
			var timeToUse = 0
			var incToUse = 0
			var err error
			var wTime = 0
			var bTime = 0
			var wInc = 0
			var bInc = 0
			var depthToUse = 0
			for goScanner.Scan() {
				nextToken := strings.ToLower(goScanner.Text())
				switch nextToken {
				case "infinite":
					continue
				case "wtime":
					if !goScanner.Scan() {
						fmt.Println("info string Malformed go command option wtime")
						continue
					}
					if err != nil {
						fmt.Println("info string Malformed go command option; could not convert wtime")
						continue
					}
					wTime, err = strconv.Atoi(goScanner.Text())
				case "btime":
					if !goScanner.Scan() {
						fmt.Println("info string Malformed go command option btime")
						continue
					}
					if err != nil {
						fmt.Println("info string Malformed go command option; could not convert btime")
						continue
					}
					bTime, err = strconv.Atoi(goScanner.Text())
				case "winc":
					if !goScanner.Scan() {
						fmt.Println("info string Malformed go command option winc")
						continue
					}
					if err != nil {
						fmt.Println("info string Malformed go command option; could not convert winc")
						continue
					}
					wInc, err = strconv.Atoi(goScanner.Text())
				case "binc":
					if !goScanner.Scan() {
						fmt.Println("info string Malformed go command option binc")
						continue
					}
					if err != nil {
						fmt.Println("info string Malformed go command option; could not convert binc")
						continue
					}
					bInc, err = strconv.Atoi(goScanner.Text())
				case "depth":
					if !goScanner.Scan() {
						fmt.Println("info string Malformed go command option depth")
						continue
					}
					if err != nil {
						fmt.Println("info string Malformed go command option; could not convert depth")
						continue
					}
					depthToUse, err = strconv.Atoi(goScanner.Text())
				default:
					fmt.Println("info string Unknown go subcommand", nextToken)
					continue
				}
			}

			if board.Wtomove {
				if wTime > 0 {
					timeToUse = wTime
				} else {
					timeToUse = 250000
				}
				incToUse = wInc
			} else {
				if bTime > 0 {
					timeToUse = bTime
				} else {
					timeToUse = 250000
				}
				incToUse = bInc
			}
			var useCustomDepth = false
			if depthToUse > 0 {
				useCustomDepth = true
			} else {
				depthToUse = 50
			}

			bestMove := engine.StartSearch(&board, uint8(depthToUse), timeToUse, incToUse, useCustomDepth, evalOnly, moveOrderingOnly, printSearchInformation)
			fmt.Println("bestmove ", bestMove)

			// Reset after search (while not incrementing time ...)
			engine.SearchState.UpdateBetweenSearches()
		case "position":
			posScanner := bufio.NewScanner(strings.NewReader(line))
			posScanner.Split(bufio.ScanWords)
			posScanner.Scan() // skip the first token
			if !posScanner.Scan() {
				fmt.Println("info string Malformed position command")
				continue
			}
			if strings.ToLower(posScanner.Text()) == "startpos" {
				board = gm.ParseFen(gm.Startpos)
				posScanner.Scan() // advance the scanner to leave it in a consistent state
				engine.SearchState.SyncPositionState(&board)
			} else if strings.ToLower(posScanner.Text()) == "fen" {
				fenstr := ""
				for posScanner.Scan() && strings.ToLower(posScanner.Text()) != "moves" {
					fenstr += posScanner.Text() + " "
				}
				if fenstr == "" {
					fmt.Println("info string Invalid fen position")
					continue
				}
				board = gm.ParseFen(fenstr)
				engine.SearchState.SyncPositionState(&board)
			} else {
				fmt.Println("info string Invalid position subcommand")
				continue
			}
			if strings.ToLower(posScanner.Text()) != "moves" {
				continue
			}
			for posScanner.Scan() { // for each move
				moveStr := strings.ToLower(posScanner.Text())
				legalMoves := board.GenerateLegalMoves()
				var nextMove gm.Move
				found := false
				for _, mv := range legalMoves {
					if mv.String() == moveStr {
						nextMove = mv
						found = true
						break
					}
				}
				if !found {
					parsed, err := gm.ParseMove(moveStr)
					if err != nil {
						fmt.Println("info string Contingency move parsing failed")
						continue
					}
					for _, mv := range legalMoves {
						if mv.From() == parsed.From() && mv.To() == parsed.To() && mv.PromotionPieceType() == parsed.PromotionPieceType() {
							nextMove = mv
							found = true
							break
						}
					}
					if !found {
						fmt.Println("info string Move", moveStr, "not found for position", board.ToFen())
						continue
					}
				}
				board.Apply(nextMove)
				engine.SearchState.RecordState(&board)
			}
		case "setoption":
			goScanner := bufio.NewScanner(strings.NewReader(line))
			goScanner.Split(bufio.ScanWords)
			goScanner.Scan() // skip "setoption", we use our uciOptionSetters instead ...

			for goScanner.Scan() {
				token := strings.ToLower(goScanner.Text())
				if token == "name" {
					continue
				}
				if opt, ok := uciOptionSetters[token]; ok {
					if val, ok := parseIntOption(goScanner, token); ok {
						if val < opt.min || val > opt.max {
							fmt.Printf("info string Value %d out of range [%d, %d] for %s\n", val, opt.min, opt.max, token)
							continue
						}
						opt.setter(val)
					}
				} else {
					if token == "value" {
						continue
					}
					// Unknown option - consume "value" and the actual value to stay in sync
					fmt.Printf("info string Unknown option: %s\n", token)
					if goScanner.Scan() && strings.ToLower(goScanner.Text()) == "value" {
						goScanner.Scan() // consume the value itself
					}
				}
			}
		default:
			fmt.Println("info string Unknown command:", line)
		}
	}
}
