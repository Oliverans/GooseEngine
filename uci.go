package main

import (
	"bufio"
	"chess-engine/engine"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/dylhunn/dragontoothmg"
)

func main() {
	uciLoop()
}

func uciLoop() {
	scanner := bufio.NewScanner(os.Stdin)
	board := dragontoothmg.ParseFen(dragontoothmg.Startpos) // the game board
	// used for communicating with search routine

	//haltchannel := make(chan bool)
	var evalOnly = false
	for scanner.Scan() {
		line := scanner.Text()
		tokens := strings.Fields(line)
		if len(tokens) == 0 { // ignore blank lines
			continue
		}
		switch strings.ToLower(tokens[0]) {
		case "eval":
			evalOnly = true
		case "uci":
			/*
				var PawnValueMG = 70
				var PawnValueEG = 120
				var KnightValueMG = 390
				var KnightValueEG = 350
				var BishopValueMG = 420
				var BishopValueEG = 410
				var RookValueMG = 540
				var RookValueEG = 580
				var QueenValueMG = 1020
				var QueenValueEG = 950
			*/
			fmt.Println("id name GooseEngine Alpha version 0.1")
			fmt.Println("id author Goose")
			fmt.Println("option name IsolatedPawnMG type spin default 17 min 2 max 35")
			fmt.Println("option name IsolatedPawnEG type spin default 5 min 1 max 15")
			fmt.Println("option name DoubledPawnPenaltyMG type spin default 3 min 1 max 30")
			fmt.Println("option name DoubledPawnPenaltyEG type spin default 7 min 1 max 50")
			fmt.Println("option name ConnectedPawnsBonusMG type spin default 7 min 1 max 30")
			fmt.Println("option name ConnectedPawnsBonusEG type spin default 3 min 1 max 30")
			fmt.Println("option name PhalanxPawnsBonusMG type spin default 5 min 1 max 50")
			fmt.Println("option name PhalanxPawnsBonusEG type spin default 3 min 1 max 30")
			fmt.Println("option name PawnValueMG type spin default ", engine.PawnValueMG, " min ", engine.PawnValueMG-30, " max ", engine.PawnValueMG+30)
			fmt.Println("option name PawnValueEG type spin default ", engine.PawnValueEG, " min ", engine.PawnValueEG-30, " max ", engine.PawnValueEG+30)
			fmt.Println("option name KnightValueMG type spin default ", engine.KnightValueMG, " min ", engine.KnightValueMG-150, " max ", engine.KnightValueMG+150)
			fmt.Println("option name KnightValueEG type spin default ", engine.KnightValueEG, " min ", engine.KnightValueEG-150, " max ", engine.KnightValueEG+150)
			fmt.Println("option name BishopValueMG type spin default ", engine.BishopValueMG, " min ", engine.BishopValueMG-150, " max ", engine.BishopValueMG+150)
			fmt.Println("option name BishopValueEG type spin default ", engine.BishopValueEG, " min ", engine.BishopValueEG-150, " max ", engine.BishopValueEG+150)
			fmt.Println("option name RookValueMG type spin default ", engine.RookValueMG, " min ", engine.RookValueMG-250, " max ", engine.RookValueMG+250)
			fmt.Println("option name RookValueEG type spin default ", engine.RookValueEG, " min ", engine.RookValueEG-250, " max ", engine.RookValueEG+250)
			fmt.Println("option name QueenValueMG type spin default ", engine.QueenValueMG, " min ", engine.QueenValueMG-350, " max ", engine.QueenValueMG+350)
			fmt.Println("option name QueenValueEG type spin default ", engine.QueenValueEG, " min ", engine.QueenValueEG-350, " max ", engine.QueenValueEG+350)
			fmt.Println("uciok")
		case "isready":
			fmt.Println("readyok")
		case "ucinewgame":
			board = dragontoothmg.ParseFen(dragontoothmg.Startpos)
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

			var best_move = engine.StartSearch(&board, depthToUse, timeToUse, incToUse, useCustomDepth, evalOnly)
			fmt.Println("bestmove ", best_move)
		case "position":
			posScanner := bufio.NewScanner(strings.NewReader(line))
			posScanner.Split(bufio.ScanWords)
			posScanner.Scan() // skip the first token
			if !posScanner.Scan() {
				fmt.Println("info string Malformed position command")
				continue
			}
			engine.HistoryMap = make(map[uint64]int) // reset the history map
			if strings.ToLower(posScanner.Text()) == "startpos" {
				board = dragontoothmg.ParseFen(dragontoothmg.Startpos)
				engine.HistoryMap[board.Hash()]++ // record that this state has occurred
				posScanner.Scan()                 // advance the scanner to leave it in a consistent state
			} else if strings.ToLower(posScanner.Text()) == "fen" {
				fenstr := ""
				for posScanner.Scan() && strings.ToLower(posScanner.Text()) != "moves" {
					fenstr += posScanner.Text() + " "
				}
				if fenstr == "" {
					fmt.Println("info string Invalid fen position")
					continue
				}
				board = dragontoothmg.ParseFen(fenstr)
				engine.HistoryMap[board.Hash()]++ // record that this state has occurred
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
				var nextMove dragontoothmg.Move
				found := false
				for _, mv := range legalMoves {
					if mv.String() == moveStr {
						nextMove = mv
						found = true
						break
					}
				}
				if !found { // we didn't find the move, but we will try to apply it anyway
					fmt.Println("info string Move", moveStr, "not found for position", board.ToFen())
					var err error
					nextMove, err = dragontoothmg.ParseMove(moveStr)
					if err != nil {
						fmt.Println("info string Contingency move parsing failed")
						continue
					}
				}
				board.Apply(nextMove)
				engine.HistoryMap[board.Hash()]++
			}
		case "setoption":
			goScanner := bufio.NewScanner(strings.NewReader(line))
			goScanner.Split(bufio.ScanWords)
			goScanner.Scan() // skip the first token
			var err error
			for goScanner.Scan() {
				nextToken := strings.ToLower(goScanner.Text())
				switch nextToken {
				case "doubledpawnpenaltymg":
					if !goScanner.Scan() {
						fmt.Println("info string Malformed go command option")
						continue
					}
					goScanner.Scan()
					engine.DoubledPawnPenaltyMG, err = strconv.Atoi(goScanner.Text())
				case "doubledpawnpenaltyeg":
					if !goScanner.Scan() {
						fmt.Println("info string Malformed go command option")
						continue
					}
					goScanner.Scan()
					engine.DoubledPawnPenaltyEG, err = strconv.Atoi(goScanner.Text())
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
					engine.IsolatedPawnMG, err = strconv.Atoi(goScanner.Text())
				case "connectedpawnsbonusmg":
					if !goScanner.Scan() {
						fmt.Println("info string Malformed go command option")
						continue
					}
					goScanner.Scan()
					engine.ConnectedPawnsBonusMG, err = strconv.Atoi(goScanner.Text())
				case "connectedpawnsbonuseg":
					if !goScanner.Scan() {
						fmt.Println("info string Malformed go command option")
						continue
					}
					goScanner.Scan()
					engine.ConnectedPawnsBonusEG, err = strconv.Atoi(goScanner.Text())
				case "phalanxpawnsbonusmg":
					if !goScanner.Scan() {
						fmt.Println("info string Malformed go command option", err)
						continue
					}
					goScanner.Scan()
					engine.PhalanxPawnsBonusMG, err = strconv.Atoi(goScanner.Text())
				case "phalanxpawnsbonuseg":
					if !goScanner.Scan() {
						fmt.Println("info string Malformed go command option", err)
						continue
					}
					goScanner.Scan()
					engine.PhalanxPawnsBonusEG, err = strconv.Atoi(goScanner.Text())
				case "blockedpawn5thmg":
					if !goScanner.Scan() {
						fmt.Println("info string Malformed go command option", err)
						continue
					}
					goScanner.Scan()
					engine.BlockedPawn5thMG, err = strconv.Atoi(goScanner.Text())
				case "blockedpawn5theg":
					if !goScanner.Scan() {
						fmt.Println("info string Malformed go command option", err)
						continue
					}
					goScanner.Scan()
					engine.BlockedPawn5thEG, err = strconv.Atoi(goScanner.Text())
				case "blockedpawn6thmg":
					if !goScanner.Scan() {
						fmt.Println("info string Malformed go command option", err)
						continue
					}
					goScanner.Scan()
					engine.BlockedPawn6thEG, err = strconv.Atoi(goScanner.Text())
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
