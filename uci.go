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

func main() {
	uciLoop()
}

func uciLoop() {
	scanner := bufio.NewScanner(os.Stdin)
	board := gm.ParseFen(gm.Startpos) // the game board
	engine.History.History = make([]uint64, 500)

	var evalOnly = false
	var moveOrderingOnly = false
	for scanner.Scan() {
		line := scanner.Text()
		tokens := strings.Fields(line)
		if len(tokens) == 0 { // ignore blank lines
			continue
		}
		switch strings.ToLower(tokens[0]) {
		case "eval":
			evalOnly = true
		case "moveordering":
			moveOrderingOnly = true
		case "cutstats":
			engine.PrintCutStats = true
		case "uci":
			fmt.Println("id name GooseEngine Alpha version 0.2")
			fmt.Println("id author Goose")

			// --- Search / pruning parameters exposed as UCI options ---

			// Futility margins (node-level)
			fmt.Printf("option name FutilityMarginDepth1 type spin default %d min 0 max 1000\n", engine.FutilityMargins[1])
			fmt.Printf("option name FutilityMarginDepth2 type spin default %d min 0 max 1000\n", engine.FutilityMargins[2])

			// Razoring margins
			fmt.Printf("option name RazorMarginDepth1 type spin default %d min 0 max 1000\n", engine.RazoringMargins[1])
			fmt.Printf("option name RazorMarginDepth2 type spin default %d min 0 max 1000\n", engine.RazoringMargins[2])
			fmt.Printf("option name RazorMarginDepth3 type spin default %d min 0 max 1000\n", engine.RazoringMargins[3])

			// Late Move Pruning (LMP) thresholds
			// depth = 2,3,4, >=5 (5+ uses the last entry in your table)
			fmt.Printf("option name LMPDepth2 type spin default %d min 0 max 64\n", engine.LateMovePruningMargins[2])
			fmt.Printf("option name LMPDepth3 type spin default %d min 0 max 64\n", engine.LateMovePruningMargins[3])
			fmt.Printf("option name LMPDepth4 type spin default %d min 0 max 64\n", engine.LateMovePruningMargins[4])
			fmt.Printf("option name LMPDepth5Plus type spin default %d min 0 max 64\n", engine.LateMovePruningMargins[5])

			// LMR (Late Move Reductions) knobs
			fmt.Printf("option name LMRDepthLimit type spin default %d min 0 max 20\n", engine.LMRDepthLimit)

			// Null-move pruning knobs
			fmt.Printf("option name NullMoveMinDepth type spin default %d min 0 max 10\n", engine.NullMoveMinDepth)

			// (You can later add aspiration window / static-null / qsearch params here
			//  once you expose them as engine-level variables.)

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

			bestMove := engine.StartSearch(&board, uint8(depthToUse), timeToUse, incToUse, useCustomDepth, evalOnly, moveOrderingOnly)
			fmt.Println("bestmove ", bestMove)
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
			goScanner.Scan() // skip the first token ("setoption")
			for goScanner.Scan() {
				nextToken := strings.ToLower(goScanner.Text())
				switch nextToken {

				// --- Search / pruning options ---

				case "futilitymargindepth1":
					if !goScanner.Scan() {
						fmt.Println("info string Malformed setoption for FutilityMarginDepth1")
						continue
					}
					goScanner.Scan()
					val, err := strconv.Atoi(goScanner.Text())
					if err != nil {
						fmt.Println("info string Malformed value for FutilityMarginDepth1", err)
						continue
					}
					engine.FutilityMargins[1] = int16(val)

				case "futilitymargindepth2":
					if !goScanner.Scan() {
						fmt.Println("info string Malformed setoption for FutilityMarginDepth2")
						continue
					}
					goScanner.Scan()
					val, err := strconv.Atoi(goScanner.Text())
					if err != nil {
						fmt.Println("info string Malformed value for FutilityMarginDepth2", err)
						continue
					}
					engine.FutilityMargins[2] = int16(val)

				case "razormargindepth1":
					if !goScanner.Scan() {
						fmt.Println("info string Malformed setoption for RazorMarginDepth1")
						continue
					}
					goScanner.Scan()
					val, err := strconv.Atoi(goScanner.Text())
					if err != nil {
						fmt.Println("info string Malformed value for RazorMarginDepth1", err)
						continue
					}
					engine.RazoringMargins[1] = int16(val)

				case "razormargindepth2":
					if !goScanner.Scan() {
						fmt.Println("info string Malformed setoption for RazorMarginDepth2")
						continue
					}
					goScanner.Scan()
					val, err := strconv.Atoi(goScanner.Text())
					if err != nil {
						fmt.Println("info string Malformed value for RazorMarginDepth2", err)
						continue
					}
					engine.RazoringMargins[2] = int16(val)

				case "razormargindepth3":
					if !goScanner.Scan() {
						fmt.Println("info string Malformed setoption for RazorMarginDepth3")
						continue
					}
					goScanner.Scan()
					val, err := strconv.Atoi(goScanner.Text())
					if err != nil {
						fmt.Println("info string Malformed value for RazorMarginDepth3", err)
						continue
					}
					engine.RazoringMargins[3] = int16(val)

				case "lmpdepth2":
					if !goScanner.Scan() {
						fmt.Println("info string Malformed setoption for LMPDepth2")
						continue
					}
					goScanner.Scan()
					val, err := strconv.Atoi(goScanner.Text())
					if err != nil {
						fmt.Println("info string Malformed value for LMPDepth2", err)
						continue
					}
					engine.LateMovePruningMargins[2] = val

				case "lmpdepth3":
					if !goScanner.Scan() {
						fmt.Println("info string Malformed setoption for LMPDepth3")
						continue
					}
					goScanner.Scan()
					val, err := strconv.Atoi(goScanner.Text())
					if err != nil {
						fmt.Println("info string Malformed value for LMPDepth3", err)
						continue
					}
					engine.LateMovePruningMargins[3] = val

				case "lmpdepth4":
					if !goScanner.Scan() {
						fmt.Println("info string Malformed setoption for LMPDepth4")
						continue
					}
					goScanner.Scan()
					val, err := strconv.Atoi(goScanner.Text())
					if err != nil {
						fmt.Println("info string Malformed value for LMPDepth4", err)
						continue
					}
					engine.LateMovePruningMargins[4] = val

				case "lmpdepth5plus":
					if !goScanner.Scan() {
						fmt.Println("info string Malformed setoption for LMPDepth5Plus")
						continue
					}
					goScanner.Scan()
					val, err := strconv.Atoi(goScanner.Text())
					if err != nil {
						fmt.Println("info string Malformed value for LMPDepth5Plus", err)
						continue
					}
					engine.LateMovePruningMargins[5] = val

				case "lmrdepthlimit":
					if !goScanner.Scan() {
						fmt.Println("info string Malformed setoption for LMRDepthLimit")
						continue
					}
					goScanner.Scan()
					val, err := strconv.Atoi(goScanner.Text())
					if err != nil {
						fmt.Println("info string Malformed value for LMRDepthLimit", err)
						continue
					}
					engine.LMRDepthLimit = int8(val)

				case "nullmovemindepth":
					if !goScanner.Scan() {
						fmt.Println("info string Malformed setoption for NullMoveMinDepth")
						continue
					}
					goScanner.Scan()
					val, err := strconv.Atoi(goScanner.Text())
					if err != nil {
						fmt.Println("info string Malformed value for NullMoveMinDepth", err)
						continue
					}
					engine.NullMoveMinDepth = int8(val)

					// --- (Existing eval tuning options below; left as-is, but note the casing fixes) ---
				}
			}
		default:
			fmt.Println("info string Unknown command:", line)
		}
	}
}
