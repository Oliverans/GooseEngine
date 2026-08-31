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
		engine.StartSearch(&board, engine.SearchLimits{Depth: uint8(benchDepth)}, false, false, false)

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

func handleSetOption(line string) {
	name, value, ok := parseSetOption(line)
	if !ok {
		fmt.Println("info string Malformed setoption command")
		return
	}
	normalized := normalizeOptionName(name)
	if opt, ok := uciOptionSetters[normalized]; ok {
		val, err := strconv.Atoi(value)
		if err != nil {
			fmt.Printf("info string Malformed value for %s: %v\n", name, err)
			return
		}
		if val < opt.min || val > opt.max {
			fmt.Printf("info string Value %d out of range [%d, %d] for %s\n", val, opt.min, opt.max, name)
			return
		}
		opt.setter(val)
		return
	}
	if setter, ok := uciCheckOptionSetters[normalized]; ok {
		val, err := strconv.ParseBool(value)
		if err != nil {
			fmt.Printf("info string Malformed check value for %s: %v\n", name, err)
			return
		}
		setter(val)
		return
	}
	if setter, ok := uciStringOptionSetters[normalized]; ok {
		setter(value)
		return
	}
	if setter, ok := uciButtonOptionSetters[normalized]; ok {
		setter()
		return
	}
	fmt.Printf("info string Unknown option: %s\n", name)
}

func parseSetOption(line string) (name string, value string, ok bool) {
	tokens := strings.Fields(line)
	if len(tokens) < 3 || strings.ToLower(tokens[0]) != "setoption" {
		return "", "", false
	}
	nameStart := -1
	valueStart := -1
	for i := 1; i < len(tokens); i++ {
		switch strings.ToLower(tokens[i]) {
		case "name":
			nameStart = i + 1
		case "value":
			valueStart = i + 1
		}
	}
	if nameStart < 0 {
		return "", "", false
	}
	nameEnd := len(tokens)
	if valueStart >= 0 {
		nameEnd = valueStart - 1
	}
	if nameStart >= nameEnd {
		return "", "", false
	}
	name = strings.Join(tokens[nameStart:nameEnd], " ")
	if valueStart >= 0 && valueStart < len(tokens) {
		value = strings.Join(tokens[valueStart:], " ")
	} else {
		value = "true"
	}
	return name, value, true
}

func parseGoLimits(line string) (engine.SearchLimits, []string, bool) {
	var limits engine.SearchLimits
	var messages []string
	tokens := strings.Fields(line)
	valid := true

	parseInt := func(index *int, name string, positive bool) (int, bool) {
		if *index+1 >= len(tokens) {
			messages = append(messages, "Malformed go command option "+name)
			valid = false
			return 0, false
		}
		*index = *index + 1
		value, err := strconv.Atoi(tokens[*index])
		if err != nil || (positive && value <= 0) || (!positive && value < 0) {
			messages = append(messages, "Malformed go command option "+name)
			valid = false
			return 0, false
		}
		return value, true
	}

	for i := 1; i < len(tokens); i++ {
		switch strings.ToLower(tokens[i]) {
		case "infinite":
			limits.Infinite = true
		case "wtime":
			if value, ok := parseInt(&i, "wtime", false); ok {
				limits.WTime = value
			}
		case "btime":
			if value, ok := parseInt(&i, "btime", false); ok {
				limits.BTime = value
			}
		case "winc":
			if value, ok := parseInt(&i, "winc", false); ok {
				limits.WInc = value
			}
		case "binc":
			if value, ok := parseInt(&i, "binc", false); ok {
				limits.BInc = value
			}
		case "movestogo":
			if value, ok := parseInt(&i, "movestogo", true); ok {
				limits.MovesToGo = value
			}
		case "depth":
			if value, ok := parseInt(&i, "depth", true); ok {
				if value > int(engine.MaxDepth) {
					messages = append(messages, fmt.Sprintf("go depth %d exceeds maximum %d", value, engine.MaxDepth))
					valid = false
				} else {
					limits.Depth = uint8(value)
				}
			}
		case "movetime":
			if value, ok := parseInt(&i, "movetime", true); ok {
				limits.MoveTimeMs = value
			}
		case "nodes":
			if i+1 >= len(tokens) {
				messages = append(messages, "Malformed go command option nodes")
				valid = false
				continue
			}
			i++
			value, err := strconv.ParseUint(tokens[i], 10, 64)
			if err != nil || value == 0 {
				messages = append(messages, "Malformed go command option nodes")
				valid = false
				continue
			}
			limits.Nodes = value
		default:
			messages = append(messages, "Unknown go subcommand "+tokens[i])
		}
	}

	return limits, messages, valid
}

func normalizeOptionName(name string) string {
	return strings.ReplaceAll(strings.ToLower(name), " ", "")
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
	"multipv": {1, 256, func(v int) { engine.MultiPV = v }},

	"futilitybase":     {10, 30, func(v int) { engine.FutilityBase = int32(v) }},
	"futilityscale":    {50, 150, func(v int) { engine.FutilityScale = int32(v) }},
	"futilitymaxdepth": {0, 12, func(v int) { engine.FutilityMaxDepth = int8(v) }},

	"rfpscale":         {50, 150, func(v int) { engine.RFPScale = int32(v) }},
	"rfpmaxdepth":      {0, 12, func(v int) { engine.RFPMaxDepth = int8(v) }},
	"razoringscale":    {100, 200, func(v int) { engine.RazoringScale = int32(v) }},
	"razoringmaxdepth": {0, 6, func(v int) { engine.RazoringMaxDepth = int8(v) }},

	"lmpoffset":                {1, 6, func(v int) { engine.LMPOffset = v }},
	"lmpmaxdepth":              {0, 12, func(v int) { engine.LMPMaxDepth = int8(v) }},
	"lmrdepthlimit":            {2, 20, func(v int) { engine.LMRDepthLimit = int8(v) }},
	"lmrmovelimit":             {2, 8, func(v int) { engine.LMRMoveLimit = v }},
	"lmrhistorybonus":          {450, 550, func(v int) { engine.LMRHistoryBonus = v }},
	"lmrhistorymalus":          {-150, -50, func(v int) { engine.LMRHistoryMalus = v }},
	"lmrcutnode":               {0, 200, func(v int) { engine.LMRCutnode = v }},
	"lmrttpv":                  {0, 100, func(v int) { engine.LMRTTPv = v }},
	"lmrnoisyoffset":           {-200, 0, func(v int) { engine.LMRNoisyOffset = v }},
	"lmrcheckbonus":            {-200, 0, func(v int) { engine.LMRCheckBonus = v }},
	"lmrdeeperbase":            {0, 120, func(v int) { engine.LMRDeeperBase = int32(v) }},
	"lmrcapturehistorydivisor": {20, 100, func(v int) { engine.LMRCaptureHistoryDivisor = v }},
	"iirmindepth":              {2, 20, func(v int) { engine.IIRMinDepth = int8(v) }},

	"nullmovemindepth":              {0, 10, func(v int) { engine.NullMoveMinDepth = int8(v) }},
	"nmmarginbase":                  {120, 250, func(v int) { engine.NMMarginBase = int32(v) }},
	"nmmargindepth":                 {10, 25, func(v int) { engine.NMMarginDepth = int32(v) }},
	"nullmovereductionbase":         {1, 6, func(v int) { engine.NullMoveReductionBase = int8(v) }},
	"nullmovereductiondepthdivisor": {2, 8, func(v int) { engine.NullMoveReductionDepthDivisor = int8(v) }},

	"singularmindepth":              {4, 16, func(v int) { engine.SingularMinDepth = int8(v) }},
	"singularttdepthslack":          {0, 8, func(v int) { engine.SingularTTDepthSlack = int8(v) }},
	"singularmarginbase":            {0, 200, func(v int) { engine.SingularMarginBase = int32(v) }},
	"singularmargindepth":           {0, 30, func(v int) { engine.SingularMarginDepth = int32(v) }},
	"singularreductionbase":         {1, 6, func(v int) { engine.SingularReductionBase = int8(v) }},
	"singularreductiondepthdivisor": {2, 8, func(v int) { engine.SingularReductionDepthDivisor = int8(v) }},

	"quiescenceseemargin":           {100, 200, func(v int) { engine.QuiescenceSeeMargin = v }},
	"seenoisyscale":                 {50, 200, func(v int) { engine.SEENoisyScale = v }},
	"seenoisyhistorydivisor":        {64, 256, func(v int) { engine.SEENoisyHistoryDivisor = v }},
	"seequietscale":                 {20, 80, func(v int) { engine.SEEQuietScale = v }},
	"seeprunemaxdepth":              {0, 12, func(v int) { engine.SEEPruneMaxDepth = int8(v) }},
	"capturefutilitybase":           {0, 400, func(v int) { engine.CaptureFutilityBase = int32(v) }},
	"capturefutilityscale":          {25, 300, func(v int) { engine.CaptureFutilityScale = int32(v) }},
	"capturefutilitymaxdepth":       {0, 8, func(v int) { engine.CaptureFutilityMaxDepth = int8(v) }},
	"capturefutilityhistorydivisor": {64, 256, func(v int) { engine.CaptureFutilityHistoryDivisor = v }},
	"probcutseemargin":              {100, 200, func(v int) { engine.ProbCutSeeMargin = v }},
	"probcutmindepth":               {3, 12, func(v int) { engine.ProbCutMinDepth = int8(v) }},
	"probcutbetamargin":             {50, 400, func(v int) { engine.ProbCutBetaMargin = int32(v) }},
	"probcutreduction":              {1, 8, func(v int) { engine.ProbCutReduction = int8(v) }},
	"probcutmaxcaptures":            {1, 32, func(v int) { engine.ProbCutMaxCaptures = v }},

	"deltamargin":          {100, 300, func(v int) { engine.DeltaMargin = int32(v) }},
	"aspirationwindowsize": {10, 100, func(v int) { engine.AspirationWindowSize = int32(v) }},
	"aspirationmaxfails":   {0, 8, func(v int) { engine.AspirationMaxFails = v }},

	"timesoftpercent":            {30, 100, func(v int) { engine.TimeSoftPercent = v }},
	"timehardpercent":            {100, 600, func(v int) { engine.TimeHardPercent = v }},
	"timeopeningminpercent":      {50, 100, func(v int) { engine.TimeOpeningMinPercent = v }},
	"timeopeningramply":          {4, 80, func(v int) { engine.TimeOpeningRampPly = v }},
	"timeminimumpercent":         {10, 100, func(v int) { engine.TimeMinimumPercent = v }},
	"timebestmovechangedpercent": {100, 250, func(v int) { engine.TimeBestMoveChangedPercent = v }},
	"timebestmovesteppercent":    {0, 30, func(v int) { engine.TimeBestMoveStepPercent = v }},
	"timebestmovestablepercent":  {30, 100, func(v int) { engine.TimeBestMoveStablePercent = v }},
	"timescoredropmaxcp":         {10, 300, func(v int) { engine.TimeScoreDropMaxCP = int32(v) }},
	"timescoredropmaxpercent":    {100, 300, func(v int) { engine.TimeScoreDropMaxPercent = v }},
}

var uciCheckOptionSetters = map[string]func(bool){}

var uciStringOptionSetters = map[string]func(string){}

var uciButtonOptionSetters = map[string]func(){}

func main() {
	if len(os.Args) >= 2 && os.Args[1] == "bench" {
		runBench()
		os.Exit(0)
	}
	uciLoop()
}

func uciLoop() {
	scanner := bufio.NewScanner(os.Stdin)
	commands := make(chan string)
	go func() {
		defer close(commands)
		for scanner.Scan() {
			commands <- scanner.Text()
		}
	}()

	board := gm.ParseFen(gm.Startpos) // the game board

	evalMode := engine.EvalOutputNone
	var moveOrderingOnly = false
	var printSearchInformation = true
	var searchDone chan struct{}
	searching := false

	for {
		var line string
		var ok bool
		select {
		case <-searchDone:
			searching = false
			searchDone = nil
			continue
		case line, ok = <-commands:
		}
		if !ok {
			if searching {
				engine.SearchState.RequestStop()
				<-searchDone
			}
			return
		}

		tokens := strings.Fields(line)
		if len(tokens) == 0 { // ignore blank lines
			continue
		}
		command := strings.ToLower(tokens[0])
		if searching {
			select {
			case <-searchDone:
				searching = false
				searchDone = nil
			default:
			}
		}
		if searching {
			switch command {
			case "stop":
				engine.SearchState.RequestStop()
			case "quit":
				engine.SearchState.RequestStop()
				<-searchDone
				return
			case "isready":
				fmt.Println("readyok")
			default:
				fmt.Printf("info string Command %s rejected while searching\n", tokens[0])
			}
			continue
		}

		switch command {
		case "bench":
			runBench()
		case "eval":
			evalMode = engine.EvalOutputText
		case "evaljson":
			evalMode = engine.EvalOutputJSON
		case "explainjson", "positiontrace":
			if len(tokens) > 1 && strings.EqualFold(tokens[1], "tuner") {
				if err := engine.RenderTuningTraceJSON(os.Stdout, engine.TuningTraceForBoard(&board)); err != nil {
					fmt.Println("info string tuning trace error:", err)
				}
				continue
			}
			level := engine.PositionTraceBasic
			if len(tokens) > 1 {
				parsed, err := engine.ParsePositionTraceLevel(tokens[1])
				if err != nil {
					fmt.Println("info string", err)
					continue
				}
				level = parsed
			}
			if err := engine.RenderExplainTraceJSON(os.Stdout, engine.ExplainTraceForBoard(&board, level)); err != nil {
				fmt.Println("info string position trace error:", err)
			}
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
			fmt.Printf("option name MultiPV type spin default %d min 1 max 256\n", engine.MultiPV)

			// --- Search / pruning parameters exposed as UCI options ---

			// Futility margins (node-level) - base ±50
			fmt.Printf("option name FutilityBase type spin default %d min 10 max 30\n", engine.FutilityBase)
			fmt.Printf("option name FutilityScale type spin default %d min 50 max 150\n", engine.FutilityScale)
			fmt.Printf("option name FutilityMaxDepth type spin default %d min 0 max 12\n", engine.FutilityMaxDepth)

			// Reverse Futility Pruning (Static Null Move) margins - base ±50
			fmt.Printf("option name RFPScale type spin default %d min 50 max 150\n", engine.RFPScale)
			fmt.Printf("option name RFPMaxDepth type spin default %d min 0 max 12\n", engine.RFPMaxDepth)

			// Razoring margins - base ±50
			fmt.Printf("option name RazoringScale type spin default %d min 100 max 200\n", engine.RazoringScale)
			fmt.Printf("option name RazoringMaxDepth type spin default %d min 0 max 6\n", engine.RazoringMaxDepth)

			// LMR (Late Move Reductions) knobs
			fmt.Printf("option name LMRDepthLimit type spin default %d min 2 max 20\n", engine.LMRDepthLimit)

			// Null-move pruning knobs
			fmt.Printf("option name NullMoveMinDepth type spin default %d min 0 max 10\n", engine.NullMoveMinDepth)
			fmt.Printf("option name NMMarginBase type spin default %d min 120 max 250\n", engine.NMMarginBase)
			fmt.Printf("option name NMMarginDepth type spin default %d min 10 max 25\n", engine.NMMarginDepth)
			fmt.Printf("option name NullMoveReductionBase type spin default %d min 1 max 6\n", engine.NullMoveReductionBase)
			fmt.Printf("option name NullMoveReductionDepthDivisor type spin default %d min 2 max 8\n", engine.NullMoveReductionDepthDivisor)

			// Additional LMP margins - base ±3
			fmt.Printf("option name LMPOffset type spin default %d min 1 max 6\n", engine.LMPOffset)
			fmt.Printf("option name LMPMaxDepth type spin default %d min 0 max 12\n", engine.LMPMaxDepth)

			// LMR parameters - base ±50 for history values
			fmt.Printf("option name LMRMoveLimit type spin default %d min 2 max 8\n", engine.LMRMoveLimit)
			fmt.Printf("option name LMRHistoryBonus type spin default %d min 450 max 550\n", engine.LMRHistoryBonus)
			fmt.Printf("option name LMRHistoryMalus type spin default %d min -150 max -50\n", engine.LMRHistoryMalus)
			fmt.Printf("option name LMRCutnode type spin default %d min 0 max 200\n", engine.LMRCutnode)
			fmt.Printf("option name LMRTTPv type spin default %d min 0 max 100\n", engine.LMRTTPv)
			fmt.Printf("option name LMRNoisyOffset type spin default %d min -200 max 0\n", engine.LMRNoisyOffset)
			fmt.Printf("option name LMRCheckBonus type spin default %d min -200 max 0\n", engine.LMRCheckBonus)
			fmt.Printf("option name LMRDeeperBase type spin default %d min 0 max 120\n", engine.LMRDeeperBase)
			fmt.Printf("option name LMRCaptureHistoryDivisor type spin default %d min 20 max 100\n", engine.LMRCaptureHistoryDivisor)
			fmt.Printf("option name IIRMinDepth type spin default %d min 2 max 20\n", engine.IIRMinDepth)

			// SEE pruning parameters
			fmt.Printf("option name QuiescenceSeeMargin type spin default %d min 100 max 200\n", engine.QuiescenceSeeMargin)
			fmt.Printf("option name SEENoisyScale type spin default %d min 50 max 200\n", engine.SEENoisyScale)
			fmt.Printf("option name SEENoisyHistoryDivisor type spin default %d min 64 max 256\n", engine.SEENoisyHistoryDivisor)
			fmt.Printf("option name SEEQuietScale type spin default %d min 20 max 80\n", engine.SEEQuietScale)
			fmt.Printf("option name SEEPruneMaxDepth type spin default %d min 0 max 12\n", engine.SEEPruneMaxDepth)
			fmt.Printf("option name CaptureFutilityBase type spin default %d min 0 max 400\n", engine.CaptureFutilityBase)
			fmt.Printf("option name CaptureFutilityScale type spin default %d min 25 max 300\n", engine.CaptureFutilityScale)
			fmt.Printf("option name CaptureFutilityMaxDepth type spin default %d min 0 max 8\n", engine.CaptureFutilityMaxDepth)
			fmt.Printf("option name CaptureFutilityHistoryDivisor type spin default %d min 64 max 256\n", engine.CaptureFutilityHistoryDivisor)
			fmt.Printf("option name ProbCutSeeMargin type spin default %d min 100 max 200\n", engine.ProbCutSeeMargin)
			fmt.Printf("option name ProbCutMinDepth type spin default %d min 3 max 12\n", engine.ProbCutMinDepth)
			fmt.Printf("option name ProbCutBetaMargin type spin default %d min 50 max 400\n", engine.ProbCutBetaMargin)
			fmt.Printf("option name ProbCutReduction type spin default %d min 1 max 8\n", engine.ProbCutReduction)
			fmt.Printf("option name ProbCutMaxCaptures type spin default %d min 1 max 32\n", engine.ProbCutMaxCaptures)

			fmt.Printf("option name SingularMinDepth type spin default %d min 4 max 16\n", engine.SingularMinDepth)
			fmt.Printf("option name SingularTTDepthSlack type spin default %d min 0 max 8\n", engine.SingularTTDepthSlack)
			fmt.Printf("option name SingularMarginBase type spin default %d min 0 max 200\n", engine.SingularMarginBase)
			fmt.Printf("option name SingularMarginDepth type spin default %d min 0 max 30\n", engine.SingularMarginDepth)
			fmt.Printf("option name SingularReductionBase type spin default %d min 1 max 6\n", engine.SingularReductionBase)
			fmt.Printf("option name SingularReductionDepthDivisor type spin default %d min 2 max 8\n", engine.SingularReductionDepthDivisor)

			// Other search parameters
			fmt.Printf("option name DeltaMargin type spin default %d min 100 max 300\n", engine.DeltaMargin)
			fmt.Printf("option name AspirationWindowSize type spin default %d min 10 max 100\n", engine.AspirationWindowSize)
			fmt.Printf("option name AspirationMaxFails type spin default %d min 0 max 8\n", engine.AspirationMaxFails)

			fmt.Printf("option name TimeSoftPercent type spin default %d min 30 max 100\n", engine.TimeSoftPercent)
			fmt.Printf("option name TimeHardPercent type spin default %d min 100 max 600\n", engine.TimeHardPercent)
			fmt.Printf("option name TimeOpeningMinPercent type spin default %d min 50 max 100\n", engine.TimeOpeningMinPercent)
			fmt.Printf("option name TimeOpeningRampPly type spin default %d min 4 max 80\n", engine.TimeOpeningRampPly)
			fmt.Printf("option name TimeMinimumPercent type spin default %d min 10 max 100\n", engine.TimeMinimumPercent)
			fmt.Printf("option name TimeBestMoveChangedPercent type spin default %d min 100 max 250\n", engine.TimeBestMoveChangedPercent)
			fmt.Printf("option name TimeBestMoveStepPercent type spin default %d min 0 max 30\n", engine.TimeBestMoveStepPercent)
			fmt.Printf("option name TimeBestMoveStablePercent type spin default %d min 30 max 100\n", engine.TimeBestMoveStablePercent)
			fmt.Printf("option name TimeScoreDropMaxCP type spin default %d min 10 max 300\n", engine.TimeScoreDropMaxCP)
			fmt.Printf("option name TimeScoreDropMaxPercent type spin default %d min 100 max 300\n", engine.TimeScoreDropMaxPercent)

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
			limits, messages, valid := parseGoLimits(line)
			for _, message := range messages {
				fmt.Println("info string", message)
			}
			if !valid {
				continue
			}
			currentEvalMode := evalMode
			if currentEvalMode != engine.EvalOutputNone {
				evalMode = engine.EvalOutputNone
			}
			searchBoard := board
			done := make(chan struct{})
			searchDone = done
			searching = true
			engine.SearchState.ClearStop()
			go func() {
				bestMove := engine.StartSearchWithEvalMode(&searchBoard, limits, currentEvalMode, moveOrderingOnly, printSearchInformation)
				engine.SearchState.UpdateBetweenSearches()
				if currentEvalMode == engine.EvalOutputNone {
					fmt.Println("bestmove", bestMove)
				}
				close(done)
			}()
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
			handleSetOption(line)
		default:
			fmt.Println("info string Unknown command:", line)
		}
	}
}
