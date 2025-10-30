package tuner

import (
	"chess-engine/engine"
	"fmt"

	"github.com/dylhunn/dragontoothmg"
)

func generateIndexes() Indexes {
	var idx Indexes
	next := uint16(0)

	// PSQT: 6 pieces × 64 squares = 384 indices
	idx.PSQT = next
	next += 64 * 6

	// Piece values (Pawn..Queen)
	idx.PieceValues = next
	next += 5

	// Passed pawns
	idx.PassedPawnPSQT = next
	next += 64

	idx.KnightMobility = next
	next++
	idx.BishopMobility = next
	next++
	idx.RookMobility = next
	next++
	idx.QueenMobility = next
	next++

	idx.DoubledPawns = next
	next++
	idx.IsolatedPawns = next
	next++
	idx.PhalanxPawns = next
	next++
	idx.ConnectedPawns = next
	next++
	idx.BlockedPawns = next
	next++
	idx.KnightOutpost = next
	next++
	idx.KnightThreatsBonus = next
	next++
	idx.BishopOutpost = next
	next++
	idx.BishopXrayAttack = next
	next++
	idx.BishopColorSetup = next
	next++
	idx.RookSemiOpenFile = next
	next++
	idx.RookOpenFile = next
	next++
	idx.RookSeventhRank = next
	next++
	idx.RookXrayAttack = next
	next++
	idx.CentralizedQueen = next
	next++
	idx.QueenInfiltration = next
	next++

	// Endgame-only
	idx.KingCentralManhattanPenalty = next
	next++
	idx.KingDistancePenalty = next
	next++
	idx.KingPawnDistance = next
	next++

	// Midgame-only
	idx.KingSafety = next
	next++

	return idx
}

func initParamsDefaults(params *[][2]float64, idx Indexes) {
	// PSQT
	for piece := dragontoothmg.Pawn; piece <= dragontoothmg.King; piece++ {
		for sq := 0; sq < 64; sq++ {
			next := idx.PSQT + uint16((piece-1)*64+sq)
			(*params)[next][0] = float64(engine.PSQT_MG[piece][sq])
			(*params)[next][1] = float64(engine.PSQT_EG[piece][sq])
		}
	}

	// Piece values
	for i := uint16(dragontoothmg.Pawn); i <= dragontoothmg.Queen; i++ {
		(*params)[idx.PieceValues+(i-1)][0] = float64(engine.PieceValueMG[i])
		(*params)[idx.PieceValues+(i-1)][1] = float64(engine.PieceValueEG[i])
	}

	// Passed pawn PSQT initialization
	for sq := uint16(0); sq < 64; sq++ {
		next := idx.PassedPawnPSQT + sq
		(*params)[next][0] = float64(engine.PassedPawnPSQT_MG[sq])
		(*params)[next][1] = float64(engine.PassedPawnPSQT_EG[sq])
	}

	// Mobility
	(*params)[idx.KnightMobility][0] = float64(engine.MobilityValueMG[dragontoothmg.Knight])
	(*params)[idx.KnightMobility][1] = float64(engine.MobilityValueEG[dragontoothmg.Knight])
	(*params)[idx.BishopMobility][0] = float64(engine.MobilityValueMG[dragontoothmg.Bishop])
	(*params)[idx.BishopMobility][1] = float64(engine.MobilityValueEG[dragontoothmg.Bishop])
	(*params)[idx.RookMobility][0] = float64(engine.MobilityValueMG[dragontoothmg.Rook])
	(*params)[idx.RookMobility][1] = float64(engine.MobilityValueEG[dragontoothmg.Rook])
	(*params)[idx.QueenMobility][0] = float64(engine.MobilityValueMG[dragontoothmg.Queen])
	(*params)[idx.QueenMobility][1] = float64(engine.MobilityValueEG[dragontoothmg.Queen])

	// Pawns
	(*params)[idx.DoubledPawns][0] = float64(engine.DoubledPawnPenaltyMG)
	(*params)[idx.DoubledPawns][1] = float64(engine.DoubledPawnPenaltyEG)
	(*params)[idx.IsolatedPawns][0] = float64(engine.IsolatedPawnMG)
	(*params)[idx.IsolatedPawns][1] = float64(engine.IsolatedPawnEG)
	(*params)[idx.PhalanxPawns][0] = float64(engine.PhalanxPawnsBonusMG)
	(*params)[idx.PhalanxPawns][1] = float64(engine.PhalanxPawnsBonusEG)
	(*params)[idx.ConnectedPawns][0] = float64(engine.ConnectedPawnsBonusMG)
	(*params)[idx.ConnectedPawns][1] = float64(engine.ConnectedPawnsBonusEG)
	(*params)[idx.BlockedPawns][0] = float64(engine.BlockedPawnBonusMG)
	(*params)[idx.BlockedPawns][1] = float64(engine.BlockedPawnBonusEG)

	// Knights
	(*params)[idx.KnightOutpost][0] = float64(engine.KnightOutpostMG)
	(*params)[idx.KnightOutpost][1] = float64(engine.KnightOutpostEG)

	// Bishops
	(*params)[idx.BishopOutpost][0] = float64(engine.BishopOutpostMG)
	(*params)[idx.BishopOutpost][1] = float64(engine.BishopOutpostEG)
	(*params)[idx.BishopPair][0] = float64(engine.BishopPairBonusMG)
	(*params)[idx.BishopPair][1] = float64(engine.BishopPairBonusEG)

	// Rooks
	(*params)[idx.RookOpenFile][0] = float64(engine.RookOpenFileBonusMG)
	(*params)[idx.RookOpenFile][1] = 0.0
	(*params)[idx.RookSemiOpenFile][0] = float64(engine.RookSemiOpenFileBonusMG)
	(*params)[idx.RookSemiOpenFile][1] = 0.0
	(*params)[idx.RookSeventhRank][0] = 0.0
	(*params)[idx.RookSeventhRank][1] = float64(engine.SeventhRankBonusEG)

	// Queens
	(*params)[idx.CentralizedQueen][0] = 0.0
	(*params)[idx.CentralizedQueen][1] = float64(engine.CentralizedQueenBonusEG)
	(*params)[idx.QueenInfiltration][0] = float64(engine.QueenInfiltrationBonusMG)
	(*params)[idx.QueenInfiltration][1] = float64(engine.QueenInfiltrationBonusEG)

	// Kings
	(*params)[idx.KingSafety][0] = 0.075
	(*params)[idx.KingSafety][1] = 0.0
}

func printParams(params *[][2]float64, idx Indexes) {
	fmt.Println("==== Tuned Parameters ====")

	pieceNames := []string{"Pawn", "Knight", "Bishop", "Rook", "Queen", "King"}

	// --- Piece Values ---
	fmt.Println("\n-- Piece Values --")
	for i, name := range pieceNames[:5] { // skip King
		mg := (*params)[idx.PieceValues+uint16(i)][0]
		eg := (*params)[idx.PieceValues+uint16(i)][1]
		fmt.Printf("  %-6s: MG = %8.2f | EG = %8.2f\n", name, mg, eg)
	}

	// --- PSQT Full ---
	fmt.Println("\n-- PSQT Tables (MG | EG) --")
	for p := 0; p <= 5; p++ {
		fmt.Printf("\n%s:\n", pieceNames[p])
		for rank := 0; rank <= 7; rank++ {
			for file := 0; file < 8; file++ {
				sq := rank*8 + file
				idxSq := idx.PSQT + uint16(p*64+sq)
				mg := (*params)[idxSq][0]
				eg := (*params)[idxSq][1]
				fmt.Printf("%6.2f/%-6.2f ", mg, eg)
			}
			fmt.Println()
		}
	}

	// --- Passed Pawn PSQT Full ---
	fmt.Println("\n-- Passed Pawn PSQT (MG | EG) --")
	for sq := 0; sq < 64; sq++ {
		idxSq := idx.PassedPawnPSQT + uint16(sq)
		mg := (*params)[idxSq][0]
		eg := (*params)[idxSq][1]
		fmt.Printf("%6.1f/%-6.1f ", mg, eg)

		if (sq+1)%8 == 0 {
			fmt.Println()
		}
	}

	// --- Pawn Structure Terms ---
	fmt.Println("\n-- Mobility Structure --")
	printTerm(params, idx.KnightMobility, "Knight Mobilty")
	printTerm(params, idx.BishopMobility, "Bishop Mobilty")
	printTerm(params, idx.RookMobility, "Rook Mobilty")
	printTerm(params, idx.QueenMobility, "Queen Mobilty")

	// --- Pawn Structure Terms ---
	fmt.Println("\n-- Pawn Structure --")
	printTerm(params, idx.DoubledPawns, "Doubled pawns")
	printTerm(params, idx.IsolatedPawns, "Isolated pawns")
	printTerm(params, idx.PhalanxPawns, "Phalanx pawns")
	printTerm(params, idx.ConnectedPawns, "Connected pawns")
	printTerm(params, idx.BlockedPawns, "Blocked pawns")

	// --- Knight terms ---
	fmt.Println("\n-- Knight Terms --")
	printTerm(params, idx.KnightOutpost, "Knight outpost")

	// --- Bishop terms ---
	fmt.Println("\n-- Bishop Terms --")
	printTerm(params, idx.BishopOutpost, "Bishop outpost")
	printTerm(params, idx.BishopPair, "Bishop pair")

	// --- Rook terms ---
	fmt.Println("\n-- Rook Terms --")
	printTerm(params, idx.RookOpenFile, "Rook open file")
	printTerm(params, idx.RookSemiOpenFile, "Rook semi-open file")
	printTerm(params, idx.RookSeventhRank, "Rook 7th rank")

	// --- Queen terms ---
	fmt.Println("\n-- Queen Terms --")
	printTerm(params, idx.CentralizedQueen, "Centralized queen")
	printTerm(params, idx.QueenInfiltration, "Queen infiltration")

	// --- King terms ---
	fmt.Println("\n-- King Terms --")
	printTerm(params, idx.KingSafety, "King safety")
}

// Helper to reduce repetition
func printTerm(params *[][2]float64, index uint16, name string) {
	mg := (*params)[index][0]
	eg := (*params)[index][1]
	fmt.Printf("  %-20s: MG = %8.2f | EG = %8.2f\n", name, mg, eg)
}
