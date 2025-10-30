package tuner

import (
	"chess-engine/engine"
	"fmt"
	"math"
	"math/rand"

	"github.com/dylhunn/dragontoothmg"
)

// tuner.go is a texel tuning implementation for Goose Engine Alpha.
type TEntry struct {
	index  int
	fen    string
	result float64
}

// Tracing
type TraceTerm struct {
	Index uint16  // parameter slot
	MG    float64 // midgame contribution (e.g., +1 for a knight)
	EG    float64 // endgame contribution
}

type Indexes struct {
	// We train both MG/EG by setting both in params; so effectively we use half the index slots
	// There's no "good or bad", but it works so.

	// PSQT (6 pieces × 64 squares) 384 squares (0-383)
	PSQT uint16 // 0 -> 383

	// Piece Values (5 pieces * 2)
	PieceValues uint16 // 384 -> 394

	// Passed Pawn PSQT (64 squares)
	PassedPawnPSQT uint16 // 395 -> 458

	// Mobility
	KnightMobility uint16 // 459
	BishopMobility uint16 // 460
	RookMobility   uint16 // 461
	QueenMobility  uint16 // 462

	// Scalar terms (459+)
	// Pawns
	DoubledPawns   uint16 // 463
	IsolatedPawns  uint16 // 464
	PhalanxPawns   uint16 // 465
	ConnectedPawns uint16 // 466
	BlockedPawns   uint16 // 467

	// Knights
	KnightOutpost      uint16 // 468
	KnightThreatsBonus uint16 // 469

	// Bishops
	BishopOutpost    uint16 // 470
	BishopPair       uint16 // 471
	BishopXrayAttack uint16 // 472
	BishopColorSetup uint16 // 473

	// Rooks
	RookSemiOpenFile uint16 // 474
	RookOpenFile     uint16 // 475
	RookSeventhRank  uint16 // 476
	RookXrayAttack   uint16 // 477

	// Queens
	CentralizedQueen  uint16 // 478
	QueenInfiltration uint16 // 479

	// King
	KingCentralManhattanPenalty uint16 // 480
	KingDistancePenalty         uint16 // 481
	KingPawnDistance            uint16 // 482
	KingSafety                  uint16 // 483
}

// Initiate array of positions for tuner to use
func InitEntry(board *dragontoothmg.Board) {

	var entries = make([]TEntry, 9996883)

	parseNextEPD(&entries)

	var learningRate float64 = 1e-1 // <-- High
	//var learningRate float64 = 5e-6 // <-- Balanced? Possibly too low
	//var learningRate float64 = 1e-6 // <-- Medium
	//var learningRate float64 = 3e-7 // <-- Very low
	//var learningRate float64 = 0.0002 // Ugh
	runTuner(entries, learningRate, 100)
}

func runTuner(entries []TEntry, learningRate float64, epochs int) [][2]float64 {
	const numParams = 483
	const lambda = 0.0001
	const epsilon = 1e-15
	const batchSize = 100000 // you can tune this (5k–50k is typical)

	params := make([][2]float64, numParams)
	index := generateIndexes()
	initParamsDefaults(&params, index)

	n := len(entries)

	for epoch := 0; epoch < epochs; epoch++ {
		var totalLoss float64
		// Shuffle entries each epoch (optional, but recommended)
		rand.Shuffle(n, func(i, j int) { entries[i], entries[j] = entries[j], entries[i] })

		for start := 0; start < n; start += batchSize {
			end := start + batchSize
			if end > n {
				end = n
			}

			batch := entries[start:end]
			gradients := make([][2]float64, numParams)
			var batchLoss float64

			for _, entry := range batch {
				board := dragontoothmg.ParseFen(entry.fen)
				terms := engine.EvaluationTest(&board)

				eval, trace := linearEvalWithTrace(terms, params, index)
				sigmoid := 1.0 / (1.0 + math.Exp(-eval/400.0))

				p := sigmoid
				p = math.Max(epsilon, math.Min(1.0-epsilon, p))

				loss := -(entry.result*math.Log(p) + (1-entry.result)*math.Log(1-p))
				gradFactor := p - entry.result

				batchLoss += loss

				for _, t := range trace {
					gradients[t.Index][0] += gradFactor * t.MG
					gradients[t.Index][1] += gradFactor * t.EG
				}
			}

			// Apply L2 regularization and update
			for i := range params {
				gradients[i][0] = gradients[i][0]/float64(len(batch)) + lambda*params[i][0]
				gradients[i][1] = gradients[i][1]/float64(len(batch)) + lambda*params[i][1]

				params[i][0] -= learningRate * gradients[i][0]
				params[i][1] -= learningRate * gradients[i][1]
			}
			totalLoss += batchLoss
		}

		if epoch%25 == 0 {
			printParams(&params, index)
		}

		fmt.Printf("Epoch %d complete. Total loss: %.6f\n", epoch+1, totalLoss)
	}

	printParams(&params, index)
	return params
}

func linearEvalWithTrace(terms engine.EvaluationTerms, params [][2]float64, indexes Indexes) (float64, []TraceTerm) {
	var trace []TraceTerm = get_traces(&terms, &params, &indexes)

	var eval float64
	for _, t := range trace {
		eval += (t.MG * terms.MidgamePhase * params[t.Index][0])
		eval += (t.EG * terms.EndgamePhase * params[t.Index][1])
	}

	return eval, trace
}
