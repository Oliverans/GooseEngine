package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
	"strconv"

	"github.com/Oliverans/GooseEngineMG/engine"
	"github.com/Oliverans/GooseEngineMG/engine/nnue"
	"github.com/Oliverans/GooseEngineMG/engine/search"
)

func atoi(s string) int { v, _ := strconv.Atoi(s); return v }

func main() {
	reader := bufio.NewReader(os.Stdin)
	var board engine.Board
	var searcher *search.Searcher
	var network *nnue.Network

	if net, err := nnue.LoadNetwork("default.nnue"); err == nil { network = net } else { network = &nnue.Network{QA:255, QB:64, SCALE:400} }
	searcher = search.NewSearcher(network, 1<<20)

	fmt.Println("id name GooseEngineMG-NNUE")
	fmt.Println("id author ChatGPT-Scaffold")
	fmt.Println("uciok")

	for {
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "" { continue }
		parts := strings.Split(line, " ")
		switch parts[0] {
		case "quit": return
		case "uci": fmt.Println("uciok")
		case "isready": fmt.Println("readyok")
		case "ucinewgame":
			board = engine.NewGame()
			searcher = search.NewSearcher(network, 1<<20)
		case "position":
			if len(parts) < 2 { continue }
			sub := parts[1]
			if sub == "startpos" {
				board = engine.NewGame()
				movesIndex := 2
				if movesIndex < len(parts) && parts[movesIndex] == "moves" {
					movesIndex++
					for i := movesIndex; i < len(parts); i++ {
						m := engine.MoveFromUCI(parts[i])
						st := board.MakeMove(m)
						if searcher.Accumulator == nil { searcher.Accumulator = nnue.NewAccumulatorFromBoard(&board, network) } else { searcher.Accumulator.ApplyMove(m, st, &board, network) }
					}
				} else {
					if searcher.Accumulator == nil { searcher.Accumulator = nnue.NewAccumulatorFromBoard(&board, network) }
				}
			} else if sub == "fen" {
				fen := strings.Join(parts[2:], " ")
				idx := strings.Index(fen, " moves ")
				var movesList []string
				if idx != -1 { movesList = strings.Split(strings.TrimSpace(fen[idx+7:]), " "); fen = fen[:idx] }
				board = engine.BoardFromFEN(fen)
				searcher.Accumulator = nnue.NewAccumulatorFromBoard(&board, network)
				for _, mv := range movesList {
					m := engine.MoveFromUCI(mv)
					st := board.MakeMove(m)
					searcher.Accumulator.ApplyMove(m, st, &board, network)
				}
			}
		case "go":
			var depth int
			var moveTime time.Duration
			for i := 1; i < len(parts); i++ {
				switch parts[i] {
				case "depth": if i+1 < len(parts) { depth = atoi(parts[i+1]) }
				case "movetime": if i+1 < len(parts) { ms := atoi(parts[i+1]); moveTime = time.Duration(ms)*time.Millisecond }
				}
			}
			if searcher.Accumulator == nil { searcher.Accumulator = nnue.NewAccumulatorFromBoard(&board, network) }
			searcher.SetTimeManager(&search.TimeManager{})
			searcher.TimeManager().Start(moveTime)
			searcher.SetStop(false); searcher.SetPly(0); searcher.SetNodes(0)

			bestMove := engine.NullMove
			if depth > 0 {
				for d := 1; d <= depth && !searcher.Stop(); d++ {
					score := searcher.Search(&board, searcher.Accumulator, d, -search.InfinityScore, search.InfinityScore, true)
					if entry, ok := searcher.TT().Probe(board.ZobristKey(), d); ok { bestMove = entry.BestMove }
					fmt.Printf("info depth %d score cp %d nodes %d time %d pv %s\n", d, score, searcher.Nodes(), searcher.TimeManager().Elapsed().Milliseconds(), bestMove.UCI())
					if searcher.TimeManager().CheckTimeout() { break }
				}
			} else if moveTime > 0 {
				for d := 1; !searcher.Stop(); d++ {
					score := searcher.Search(&board, searcher.Accumulator, d, -search.InfinityScore, search.InfinityScore, true)
					if entry, ok := searcher.TT().Probe(board.ZobristKey(), d); ok { bestMove = entry.BestMove }
					fmt.Printf("info depth %d score cp %d nodes %d time %d pv %s\n", d, score, searcher.Nodes(), searcher.TimeManager().Elapsed().Milliseconds(), bestMove.UCI())
					if searcher.TimeManager().CheckTimeout() { break }
				}
			}
			if bestMove == engine.NullMove { fmt.Println("bestmove (none)") } else { fmt.Printf("bestmove %s\n", bestMove.UCI()) }
		}
	}
}
