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
	//board := gm.ParseFen(gm.Startpos) // the game board
	//tuner.InitEntry(&board)
	uciLoop()
}

func uciLoop() {
	scanner := bufio.NewScanner(os.Stdin)
	board := gm.ParseFen(gm.Startpos) // the game board
	engine.History.History = make([]uint64, 500)
	//engine.InitVariables(&board)
	// used for communicating with search routine

	//haltchannel := make(chan bool)
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
		case "uci":
			fmt.Println("id name GooseEngine Alpha version 0.2")
			fmt.Println("id author Goose")
			//fmt.Println("option name IsolatedPawnMG type spin default ", engine.IsolatedPawnMG, " min ", engine.Max(0, engine.IsolatedPawnMG-5), " max ", engine.Min(engine.IsolatedPawnMG+5, 100))
			//fmt.Println("option name IsolatedPawnEG type spin default ", engine.IsolatedPawnEG, " min ", engine.Max(0, engine.IsolatedPawnEG-10), " max ", engine.Min(engine.IsolatedPawnEG+10, 100))
			//fmt.Println("option name DoubledPawnPenaltyMG type spin default ", engine.DoubledPawnPenaltyMG, " min ", engine.Max(0, engine.DoubledPawnPenaltyMG-5), " max ", engine.Min(engine.DoubledPawnPenaltyMG+5, 15))
			//fmt.Println("option name DoubledPawnPenaltyEG type spin default ", engine.DoubledPawnPenaltyEG, " min ", engine.Max(5, engine.DoubledPawnPenaltyEG-5), " max ", engine.Min(engine.DoubledPawnPenaltyEG+5, 20))
			//fmt.Println("option name KnightOutpostMG type spin default ", engine.KnightOutpostMG, " min ", engine.Max(5, engine.KnightOutpostMG-20), " max ", engine.Min(engine.KnightOutpostMG+20, 40))
			//fmt.Println("option name KnightOutpostEG type spin default ", engine.KnightOutpostEG, " min ", engine.Max(5, engine.KnightOutpostEG-20), " max ", engine.Min(engine.KnightOutpostEG+20, 40))
			//fmt.Println("option name BishopOutpostMG type spin default ", engine.BishopOutpostMG, " min ", engine.Max(0, engine.BishopOutpostMG-20), " max ", engine.Min(engine.BishopOutpostMG+20, 40))
			//fmt.Println("option name BishopPairBonusMG type spin default ", engine.BishopPairBonusMG, " min ", engine.Max(5, engine.BishopPairBonusMG-10), " max ", engine.Min(engine.BishopPairBonusMG+10, 20))
			//fmt.Println("option name BishopPairBonusEG type spin default ", engine.BishopPairBonusEG, " min ", engine.Max(0, engine.BishopPairBonusEG-20), " max ", engine.Min(engine.BishopPairBonusEG+20, 60))
			//fmt.Println("option name RookSemiOpenFileBonusMG type spin default ", engine.RookSemiOpenFileBonusMG, " min ", engine.Max(0, engine.RookSemiOpenFileBonusMG-20), " max ", engine.Min(engine.RookSemiOpenFileBonusMG+20, 30))
			//fmt.Println("option name RookOpenFileBonusMG type spin default ", engine.RookOpenFileBonusMG, " min ", engine.Max(0, engine.RookOpenFileBonusMG-20), " max ", engine.Min(engine.RookOpenFileBonusMG+20, 30))
			//fmt.Println("option name KingSemiOpenFilePenalty type spin default ", engine.KingSemiOpenFilePenalty, " min ", engine.Max(0, engine.KingSemiOpenFilePenalty-5), " max ", engine.Min(engine.KingSemiOpenFilePenalty+5, 20))
			//fmt.Println("option name KingOpenFilePenalty type spin default ", engine.KingOpenFilePenalty, " min ", engine.Max(0, engine.KingOpenFilePenalty-5), " max ", engine.Min(engine.KingOpenFilePenalty+5, 15))
			//fmt.Println("option name PawnValueMG type spin default ", engine.PawnValueMG, " min ", engine.PawnValueMG-30, " max ", engine.PawnValueMG+30)
			//fmt.Println("option name PawnValueEG type spin default ", engine.PawnValueEG, " min ", engine.PawnValueEG-30, " max ", engine.PawnValueEG+30)
			//fmt.Println("option name KnightValueMG type spin default ", engine.KnightValueMG, " min ", engine.KnightValueMG-150, " max ", engine.KnightValueMG+150)
			//fmt.Println("option name KnightValueEG type spin default ", engine.KnightValueEG, " min ", engine.KnightValueEG-150, " max ", engine.KnightValueEG+150)
			//fmt.Println("option name BishopValueMG type spin default ", engine.BishopValueMG, " min ", engine.BishopValueMG-150, " max ", engine.BishopValueMG+150)
			//fmt.Println("option name BishopValueEG type spin default ", engine.BishopValueEG, " min ", engine.BishopValueEG-150, " max ", engine.BishopValueEG+150)
			//fmt.Println("option name RookValueMG type spin default ", engine.RookValueMG, " min ", engine.RookValueMG-250, " max ", engine.RookValueMG+250)
			//fmt.Println("option name RookValueEG type spin default ", engine.RookValueEG, " min ", engine.RookValueEG-250, " max ", engine.RookValueEG+250)
			//fmt.Println("option name QueenValueMG type spin default ", engine.QueenValueMG, " min ", engine.QueenValueMG-350, " max ", engine.QueenValueMG+350)
			//fmt.Println("option name QueenValueEG type spin default ", engine.QueenValueEG, " min ", engine.QueenValueEG-350, " max ", engine.QueenValueEG+350)
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
						fmt.Println("info string Malformed go command option binc")
						continue
					}
					if err != nil {
						fmt.Println("info string Malformed go command option; could not convert binc")
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
					timeToUse = 300000
				}
				incToUse = wInc
			} else {
				if bTime > 0 {
					timeToUse = bTime
				} else {
					timeToUse = 300000
				}
				incToUse = bInc
			}
			var useCustomDepth = false
			if depthToUse > 0 {
				useCustomDepth = true
			} else {
				depthToUse = 50
			}

			var best_move = engine.StartSearch(&board, uint8(depthToUse), timeToUse, incToUse, useCustomDepth, evalOnly, moveOrderingOnly)
			fmt.Println("bestmove ", best_move)
		case "position":
			engine.HistoryMap = nil
			engine.HistoryMap = make(map[uint64]int, 5000)
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
				engine.HistoryMap[board.Hash()]++
			}
			//engine.History.History = make([]uint64, (int(board.HalfmoveClock()) + 50))
			engine.History.HalfclockRepetition = int(board.HalfmoveClock())
		case "setoption":
			goScanner := bufio.NewScanner(strings.NewReader(line))
			goScanner.Split(bufio.ScanWords)
			goScanner.Scan() // skip the first token
			var err error
			for goScanner.Scan() {
				nextToken := strings.ToLower(goScanner.Text())
				switch nextToken {
				case "isolatedpawnmg":
					if !goScanner.Scan() {
						fmt.Println("info string Malformed go command option")
						continue
					}
					goScanner.Scan()
					engine.IsolatedPawnMG, err = strconv.Atoi(goScanner.Text())
				case "isolatedpawneg":
					if !goScanner.Scan() {
						fmt.Println("info string Malformed go command option")
						continue
					}
					goScanner.Scan()
					engine.IsolatedPawnEG, err = strconv.Atoi(goScanner.Text())
				case "doubledpawnpenaltymg":
					if !goScanner.Scan() {
						fmt.Println("info string Malformed go command option")
						continue
					}
					goScanner.Scan()
					engine.DoubledPawnPenaltyMG, err = strconv.Atoi(goScanner.Text())
				case "DoubledPawnPenaltyEG":
					if !goScanner.Scan() {
						fmt.Println("info string Malformed go command option")
						continue
					}
					goScanner.Scan()
					engine.DoubledPawnPenaltyEG, err = strconv.Atoi(goScanner.Text())
				case "knightoutpostmg":
					if !goScanner.Scan() {
						fmt.Println("info string Malformed go command option")
						continue
					}
					goScanner.Scan()
					engine.KnightOutpostMG, err = strconv.Atoi(goScanner.Text())
				case "knightoutposteg":
					if !goScanner.Scan() {
						fmt.Println("info string Malformed go command option")
						continue
					}
					goScanner.Scan()
					engine.KnightOutpostEG, err = strconv.Atoi(goScanner.Text())
				case "bishopoutpostmg":
					if !goScanner.Scan() {
						fmt.Println("info string Malformed go command option")
						continue
					}
					goScanner.Scan()
					engine.BishopOutpostMG, err = strconv.Atoi(goScanner.Text())
				case "bishoppairbonusmg":
					if !goScanner.Scan() {
						fmt.Println("info string Malformed go command option")
						continue
					}
					goScanner.Scan()
					engine.BishopPairBonusMG, err = strconv.Atoi(goScanner.Text())
				case "bishoppairbonuseg":
					if !goScanner.Scan() {
						fmt.Println("info string Malformed go command option")
						continue
					}
					goScanner.Scan()
					engine.BishopPairBonusEG, err = strconv.Atoi(goScanner.Text())
				case "rooksemiopenfilebonusmg":
					if !goScanner.Scan() {
						fmt.Println("info string Malformed go command option")
						continue
					}
					goScanner.Scan()
					engine.RookSemiOpenFileBonusMG, err = strconv.Atoi(goScanner.Text())
				case "rookopenfilebonusmg":
					if !goScanner.Scan() {
						fmt.Println("info string Malformed go command option")
						continue
					}
					goScanner.Scan()
					engine.RookOpenFileBonusMG, err = strconv.Atoi(goScanner.Text())
				case "kingsemiopenfilepenalty":
					if !goScanner.Scan() {
						fmt.Println("info string Malformed go command option")
						continue
					}
					goScanner.Scan()
					engine.KingSemiOpenFilePenalty, err = strconv.Atoi(goScanner.Text())
				case "kingopenfilepenalty":
					if !goScanner.Scan() {
						fmt.Println("info string Malformed go command option")
						continue
					}
					goScanner.Scan()
					engine.KingOpenFilePenalty, err = strconv.Atoi(goScanner.Text())
				case "pawnvaluemg":
					if !goScanner.Scan() {
						fmt.Println("info string Malformed go command option", err)
						continue
					}
					goScanner.Scan()
					engine.PawnValueMG, err = strconv.Atoi(goScanner.Text())
				case "pawnvalueeg":
					if !goScanner.Scan() {
						fmt.Println("info string Malformed go command option", err)
						continue
					}
					goScanner.Scan()
					engine.PawnValueEG, err = strconv.Atoi(goScanner.Text())
				case "knightvaluemg":
					if !goScanner.Scan() {
						fmt.Println("info string Malformed go command option", err)
						continue
					}
					goScanner.Scan()
					engine.KnightValueMG, err = strconv.Atoi(goScanner.Text())
				case "knightvalueeg":
					if !goScanner.Scan() {
						fmt.Println("info string Malformed go command option", err)
						continue
					}
					goScanner.Scan()
					engine.KnightValueEG, err = strconv.Atoi(goScanner.Text())
				case "bishopvaluemg":
					if !goScanner.Scan() {
						fmt.Println("info string Malformed go command option", err)
						continue
					}
					goScanner.Scan()
					engine.BishopValueMG, err = strconv.Atoi(goScanner.Text())
				case "bishopvalueeg":
					if !goScanner.Scan() {
						fmt.Println("info string Malformed go command option", err)
						continue
					}
					goScanner.Scan()
					engine.BishopValueEG, err = strconv.Atoi(goScanner.Text())
				case "rookvaluemg":
					if !goScanner.Scan() {
						fmt.Println("info string Malformed go command option", err)
						continue
					}
					goScanner.Scan()
					engine.RookValueMG, err = strconv.Atoi(goScanner.Text())
				case "rookvalueeg":
					if !goScanner.Scan() {
						fmt.Println("info string Malformed go command option", err)
						continue
					}
					goScanner.Scan()
					engine.RookValueEG, err = strconv.Atoi(goScanner.Text())
				case "queenvaluemg":
					if !goScanner.Scan() {
						fmt.Println("info string Malformed go command option", err)
						continue
					}
					goScanner.Scan()
					engine.QueenValueMG, err = strconv.Atoi(goScanner.Text())
				case "queenvalueeg":
					if !goScanner.Scan() {
						fmt.Println("info string Malformed go command option", err)
						continue
					}
					goScanner.Scan()
					engine.QueenValueEG, err = strconv.Atoi(goScanner.Text())
				}
			}
		default:
			fmt.Println("info string Unknown command:", line)
		}
	}
}
