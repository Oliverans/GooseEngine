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

const benchDepth = 10

// runBench runs a benchmark search on standard positions and reports total nodes
func runBench() {
	totalNodes := 0

	for _, fen := range benchPositions {
		board := gm.ParseFen(fen)
		engine.ResetForNewGame()

		// Reset node counter before search

		// Search with fixed depth, large time, no time-based cutoff
		engine.StartSearch(&board, uint8(benchDepth), 1000000, 0, true, false, false, false)

		// Accumulate nodes
		totalNodes += engine.GetNodeCount()
	}

	fmt.Printf("%d\n", totalNodes)
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

var uciOptionSetters = map[string]uciOption{
	"futilitymargindepth1": {70, 170, func(v int) { engine.FutilityMargins[1] = int32(v) }},
	"futilitymargindepth2": {170, 270, func(v int) { engine.FutilityMargins[2] = int32(v) }},
	"futilitymargindepth3": {270, 370, func(v int) { engine.FutilityMargins[3] = int32(v) }},
	"futilitymargindepth4": {370, 470, func(v int) { engine.FutilityMargins[4] = int32(v) }},
	"futilitymargindepth5": {470, 570, func(v int) { engine.FutilityMargins[5] = int32(v) }},
	"futilitymargindepth6": {570, 670, func(v int) { engine.FutilityMargins[6] = int32(v) }},
	"futilitymargindepth7": {670, 770, func(v int) { engine.FutilityMargins[7] = int32(v) }},

	"razormargindepth1": {100, 200, func(v int) { engine.RazoringMargins[1] = int32(v) }},
	"razormargindepth2": {250, 350, func(v int) { engine.RazoringMargins[2] = int32(v) }},
	"razormargindepth3": {400, 500, func(v int) { engine.RazoringMargins[3] = int32(v) }},

	"rfpmargindepth1": {50, 150, func(v int) { engine.RFPMargins[1] = int32(v) }},
	"rfpmargindepth2": {150, 250, func(v int) { engine.RFPMargins[2] = int32(v) }},
	"rfpmargindepth3": {250, 350, func(v int) { engine.RFPMargins[3] = int32(v) }},
	"rfpmargindepth4": {350, 450, func(v int) { engine.RFPMargins[4] = int32(v) }},
	"rfpmargindepth5": {450, 550, func(v int) { engine.RFPMargins[5] = int32(v) }},
	"rfpmargindepth6": {550, 650, func(v int) { engine.RFPMargins[6] = int32(v) }},
	"rfpmargindepth7": {650, 750, func(v int) { engine.RFPMargins[7] = int32(v) }},

	"lmpdepth2":     {2, 8, func(v int) { engine.LateMovePruningMargins[2] = v }},
	"lmpdepth3":     {6, 12, func(v int) { engine.LateMovePruningMargins[3] = v }},
	"lmpdepth4":     {11, 17, func(v int) { engine.LateMovePruningMargins[4] = v }},
	"lmpdepth5plus": {17, 23, func(v int) { engine.LateMovePruningMargins[5] = v }},
	"lmpdepth6":     {24, 30, func(v int) { engine.LateMovePruningMargins[6] = v }},
	"lmpdepth7":     {32, 38, func(v int) { engine.LateMovePruningMargins[7] = v }},
	"lmpdepth8":     {41, 47, func(v int) { engine.LateMovePruningMargins[8] = v }},

	"lmrdepthlimit":   {0, 20, func(v int) { engine.LMRDepthLimit = int8(v) }},
	"lmrmovelimit":    {1, 5, func(v int) { engine.LMRMoveLimit = v }},
	"lmrhistorybonus": {450, 550, func(v int) { engine.LMRHistoryBonus = v }},
	"lmrhistorymalus": {-150, -50, func(v int) { engine.LMRHistoryMalus = v }},

	"nullmovemindepth": {0, 10, func(v int) { engine.NullMoveMinDepth = int8(v) }},

	"seeprunedepth":       {6, 10, func(v int) { engine.SEEPruneDepth = int8(v) }},
	"seeprunemargin":      {-100, -10, func(v int) { engine.SEEPruneMargin = v }},
	"quiescenceseemargin": {100, 200, func(v int) { engine.QuiescenceSeeMargin = v }},
	"probcutseemargin":    {100, 200, func(v int) { engine.ProbCutSeeMargin = v }},

	"deltamargin":          {100, 300, func(v int) { engine.DeltaMargin = int32(v) }},
	"aspirationwindowsize": {10, 100, func(v int) { engine.SetAspirationWindowSize(int32(v)) }},
}

func main() {
	uciLoop()
}

func uciLoop() {
	scanner := bufio.NewScanner(os.Stdin)
	board := gm.ParseFen(gm.Startpos) // the game board
	engine.History.History = make([]uint64, 500)

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

			// --- Search / pruning parameters exposed as UCI options ---

			// Futility margins (node-level) - base ±50
			fmt.Printf("option name FutilityMarginDepth1 type spin default %d min 70 max 170\n", engine.FutilityMargins[1])
			fmt.Printf("option name FutilityMarginDepth2 type spin default %d min 170 max 270\n", engine.FutilityMargins[2])

			// Razoring margins - base ±50
			fmt.Printf("option name RazorMarginDepth1 type spin default %d min 100 max 200\n", engine.RazoringMargins[1])
			fmt.Printf("option name RazorMarginDepth2 type spin default %d min 250 max 350\n", engine.RazoringMargins[2])
			fmt.Printf("option name RazorMarginDepth3 type spin default %d min 400 max 500\n", engine.RazoringMargins[3])

			// Late Move Pruning (LMP) thresholds - base ±3
			fmt.Printf("option name LMPDepth2 type spin default %d min 2 max 8\n", engine.LateMovePruningMargins[2])
			fmt.Printf("option name LMPDepth3 type spin default %d min 6 max 12\n", engine.LateMovePruningMargins[3])
			fmt.Printf("option name LMPDepth4 type spin default %d min 11 max 17\n", engine.LateMovePruningMargins[4])
			fmt.Printf("option name LMPDepth5Plus type spin default %d min 17 max 23\n", engine.LateMovePruningMargins[5])

			// LMR (Late Move Reductions) knobs
			fmt.Printf("option name LMRDepthLimit type spin default %d min 0 max 20\n", engine.LMRDepthLimit)

			// Null-move pruning knobs
			fmt.Printf("option name NullMoveMinDepth type spin default %d min 0 max 10\n", engine.NullMoveMinDepth)

			// Reverse Futility Pruning (Static Null Move) margins - base ±50
			fmt.Printf("option name RFPMarginDepth1 type spin default %d min 50 max 150\n", engine.RFPMargins[1])
			fmt.Printf("option name RFPMarginDepth2 type spin default %d min 150 max 250\n", engine.RFPMargins[2])
			fmt.Printf("option name RFPMarginDepth3 type spin default %d min 250 max 350\n", engine.RFPMargins[3])
			fmt.Printf("option name RFPMarginDepth4 type spin default %d min 350 max 450\n", engine.RFPMargins[4])
			fmt.Printf("option name RFPMarginDepth5 type spin default %d min 450 max 550\n", engine.RFPMargins[5])
			fmt.Printf("option name RFPMarginDepth6 type spin default %d min 550 max 650\n", engine.RFPMargins[6])
			fmt.Printf("option name RFPMarginDepth7 type spin default %d min 650 max 750\n", engine.RFPMargins[7])

			// Additional Futility margins - base ±50
			fmt.Printf("option name FutilityMarginDepth3 type spin default %d min 270 max 370\n", engine.FutilityMargins[3])
			fmt.Printf("option name FutilityMarginDepth4 type spin default %d min 370 max 470\n", engine.FutilityMargins[4])
			fmt.Printf("option name FutilityMarginDepth5 type spin default %d min 470 max 570\n", engine.FutilityMargins[5])
			fmt.Printf("option name FutilityMarginDepth6 type spin default %d min 570 max 670\n", engine.FutilityMargins[6])
			fmt.Printf("option name FutilityMarginDepth7 type spin default %d min 670 max 770\n", engine.FutilityMargins[7])

			// Additional LMP margins - base ±3
			fmt.Printf("option name LMPDepth6 type spin default %d min 24 max 30\n", engine.LateMovePruningMargins[6])
			fmt.Printf("option name LMPDepth7 type spin default %d min 32 max 38\n", engine.LateMovePruningMargins[7])
			fmt.Printf("option name LMPDepth8 type spin default %d min 41 max 47\n", engine.LateMovePruningMargins[8])

			// LMR parameters - base ±50 for history values
			fmt.Printf("option name LMRMoveLimit type spin default %d min 1 max 5\n", engine.LMRMoveLimit)
			fmt.Printf("option name LMRHistoryBonus type spin default %d min 450 max 550\n", engine.LMRHistoryBonus)
			fmt.Printf("option name LMRHistoryMalus type spin default %d min -150 max -50\n", engine.LMRHistoryMalus)

			// SEE pruning parameters
			fmt.Printf("option name SEEPruneDepth type spin default %d min 6 max 10\n", engine.SEEPruneDepth)
			fmt.Printf("option name SEEPruneMargin type spin default %d min -100 max -10\n", engine.SEEPruneMargin)
			fmt.Printf("option name QuiescenceSeeMargin type spin default %d min 100 max 200\n", engine.QuiescenceSeeMargin)
			fmt.Printf("option name ProbCutSeeMargin type spin default %d min 100 max 200\n", engine.ProbCutSeeMargin)

			// Other search parameters
			fmt.Printf("option name DeltaMargin type spin default %d min 100 max 300\n", engine.DeltaMargin)
			fmt.Printf("option name AspirationWindowSize type spin default %d min 10 max 100\n", engine.GetAspirationWindowSize())

			fmt.Println("uciok")
		case "isready":
			fmt.Println("readyok")
		case "ucinewgame":
			board = gm.ParseFen(gm.Startpos)
			engine.ResetForNewGame()
		case "quit":
			return
		case "stop":
			engine.GlobalStop = true
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
			engine.ResetCutStats()
			engine.AgeHistory()
			engine.ClearKillers(&engine.KillerMoveTable)
			engine.TT.NewSearch()
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
				engine.ResetStateTracking(&board)
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
				engine.ResetStateTracking(&board)
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
				engine.RecordState(&board)
			}
			engine.History.HalfclockRepetition = int(board.HalfmoveClock())
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
				}
			}
		default:
			fmt.Println("info string Unknown command:", line)
		}
	}
}
