package tuner

import (
	"chess-engine/engine"
	"math/bits"

	"github.com/dylhunn/dragontoothmg"
)

func get_traces(terms *engine.EvaluationTerms, params *[][2]float64, indexes *Indexes) []TraceTerm {
	trace := make([]TraceTerm, 0, len(*params))
	// --- PSQT per square ---
	get_psqt_traces(&trace, terms, indexes)

	// --- Piece Values ---
	get_piece_values_traces(&trace, terms, indexes)

	// --- Passed Pawn PSQT ---
	get_passed_pawn_traces(&trace, terms, indexes)

	// --- Mobility ---
	get_mobility_traces(&trace, terms, indexes)

	// --- Pawns ---
	get_pawn_traces(&trace, terms, indexes)

	// --- Knights ---
	get_knight_traces(&trace, terms, indexes)

	// --- Bishops ---
	get_bishop_traces(&trace, terms, indexes)

	// --- Rooks ---
	get_rook_traces(&trace, terms, indexes)

	// --- Queens ---
	get_queen_traces(&trace, terms, indexes)

	// --- Kings ---
	get_king_traces(&trace, terms, indexes)

	return trace
}

/* ============ Getters of all trace functions ============ */
func get_mobility_traces(trace *[]TraceTerm, terms *engine.EvaluationTerms, indexes *Indexes) {
	get_knight_mobility_trace(trace, terms, indexes)
	get_bishop_mobility_trace(trace, terms, indexes)
	get_rook_mobility_trace(trace, terms, indexes)
	get_queen_mobility_trace(trace, terms, indexes)
}
func get_pawn_traces(trace *[]TraceTerm, terms *engine.EvaluationTerms, indexes *Indexes) {
	get_doubled_pawn_trace(trace, terms, indexes)
	get_isolated_pawns_trace(trace, terms, indexes)
	get_phalanx_pawns_trace(trace, terms, indexes)
	get_connected_pawns_trace(trace, terms, indexes)
	get_blocked_pawns_trace(trace, terms, indexes)
}
func get_knight_traces(trace *[]TraceTerm, terms *engine.EvaluationTerms, indexes *Indexes) {
	get_knight_outpost_trace(trace, terms, indexes)
}
func get_bishop_traces(trace *[]TraceTerm, terms *engine.EvaluationTerms, indexes *Indexes) {
	get_bishop_outpost_trace(trace, terms, indexes)
	get_bishop_pair_trace(trace, terms, indexes)
}
func get_rook_traces(trace *[]TraceTerm, terms *engine.EvaluationTerms, indexes *Indexes) {
	get_rook_semiopen_file_trace(trace, terms, indexes)
	get_rook_open_file_trace(trace, terms, indexes)
	get_rook_seventh_rank_trace(trace, terms, indexes)
	get_rook_xray_trace(trace, terms, indexes)
}
func get_queen_traces(trace *[]TraceTerm, terms *engine.EvaluationTerms, indexes *Indexes) {
	get_queen_centralized_trace(trace, terms, indexes)
	get_queen_infiltration_trace(trace, terms, indexes)
}
func get_king_traces(trace *[]TraceTerm, terms *engine.EvaluationTerms, indexes *Indexes) {
	//get_kingcentral_manhattan_trace(trace, terms, indexes)
	//get_king_distance_trace(trace, terms, indexes)
	get_king_safety_trace(trace, terms, indexes)
}

/* ============ PSQT trace function ============ */
func get_psqt_traces(trace *[]TraceTerm, terms *engine.EvaluationTerms, indexes *Indexes) {
	for color := 0; color < 2; color++ {
		for piece := dragontoothmg.Pawn; piece <= dragontoothmg.King; piece++ {
			var bb uint64
			var sign float64 = 1.0
			if color == 0 {
				bb = terms.WhitePieceBB[piece-1]
			} else {
				bb = terms.BlackPieceBB[piece-1]
				sign = -1.0
			}

			for x := bb; x != 0; x &= x - 1 {
				sq := bits.TrailingZeros64(x)
				if color == 1 {
					sq = engine.FlipView[sq] // flip for black
				}
				idx := indexes.PSQT + uint16((piece-1)*64+sq)
				*trace = append(*trace, TraceTerm{
					Index: idx,
					MG:    sign,
					EG:    sign,
				})
			}
		}
	}
}

/* ============ Piece Values trace function ============ */
func get_piece_values_traces(trace *[]TraceTerm, terms *engine.EvaluationTerms, indexes *Indexes) {
	// --- Piece Values ---
	for i := uint16(dragontoothmg.Pawn) - 1; i <= dragontoothmg.Queen; i++ {
		if terms.PieceValuesMG[i] != 0 { // We only want to tune if we actually got that piece ...
			*trace = append(*trace, TraceTerm{
				Index: indexes.PieceValues + i,
				MG:    float64(terms.PieceValuesMG[i]),
				EG:    float64(terms.PieceValuesEG[i]),
			})
		}
	}
}

/* ============ Passed Pawn trace functions ============ */
func get_passed_pawn_traces(trace *[]TraceTerm, terms *engine.EvaluationTerms, indexes *Indexes) {
	// Add terms for each passed pawn square.
	// White = +1.0, Black = -1.0, flip square for black.
	for color := 0; color < 2; color++ {
		var bb uint64
		var sign float64 = 1.0
		if color == 0 {
			bb = terms.PassedPawnWBB
		} else {
			bb = terms.PassedPawnBBB
			sign = -1.0
		}

		for x := bb; x != 0; x &= x - 1 {
			sq := bits.TrailingZeros64(x)
			if color == 1 {
				sq = engine.FlipView[sq]
			}
			idx := indexes.PassedPawnPSQT + uint16(sq)
			*trace = append(*trace, TraceTerm{
				Index: idx,
				MG:    sign,
				EG:    sign,
			})
		}
	}
}

/* ============ Mobility trace functions ============ */
func get_knight_mobility_trace(trace *[]TraceTerm, terms *engine.EvaluationTerms, indexes *Indexes) {
	var knightMobility int = bits.OnesCount64(terms.KnightMobility[0]) - bits.OnesCount64(terms.KnightMobility[1])
	if knightMobility != 0 { // We only want to tune if we actually got that piece ...
		*trace = append(*trace, TraceTerm{
			Index: indexes.KnightMobility,
			MG:    float64(knightMobility),
			EG:    float64(knightMobility),
		})
	}
}
func get_bishop_mobility_trace(trace *[]TraceTerm, terms *engine.EvaluationTerms, indexes *Indexes) {
	var bishopMobility int = bits.OnesCount64(terms.BishopMobility[0]) - bits.OnesCount64(terms.BishopMobility[1])
	if bishopMobility != 0 { // We only want to tune if we actually got that piece ...
		*trace = append(*trace, TraceTerm{
			Index: indexes.BishopMobility,
			MG:    float64(bishopMobility),
			EG:    float64(bishopMobility),
		})
	}
}
func get_rook_mobility_trace(trace *[]TraceTerm, terms *engine.EvaluationTerms, indexes *Indexes) {
	var rookMobility int = bits.OnesCount64(terms.RookMobility[0]) - bits.OnesCount64(terms.RookMobility[1])
	if rookMobility != 0 { // We only want to tune if we actually got that piece ...
		*trace = append(*trace, TraceTerm{
			Index: indexes.RookMobility,
			MG:    float64(rookMobility),
			EG:    float64(rookMobility),
		})
	}
}
func get_queen_mobility_trace(trace *[]TraceTerm, terms *engine.EvaluationTerms, indexes *Indexes) {
	var queenMobility int = bits.OnesCount64(terms.QueenMobility[0]) - bits.OnesCount64(terms.QueenMobility[1])
	if queenMobility != 0 { // We only want to tune if we actually got that piece ...
		*trace = append(*trace, TraceTerm{
			Index: indexes.QueenMobility,
			MG:    float64(queenMobility),
			EG:    float64(queenMobility),
		})
	}
}

/* ============ Pawn trace functions ============ */
func get_doubled_pawn_trace(trace *[]TraceTerm, terms *engine.EvaluationTerms, indexes *Indexes) {
	if terms.DoubledPawns != 0 { // We only want to tune if we actually got that piece ...
		*trace = append(*trace, TraceTerm{
			Index: indexes.DoubledPawns,
			MG:    float64(terms.DoubledPawns),
			EG:    float64(terms.DoubledPawns),
		})
	}
}
func get_isolated_pawns_trace(trace *[]TraceTerm, terms *engine.EvaluationTerms, indexes *Indexes) {
	if terms.IsolatedPawns != 0 { // We only want to tune if we actually got that piece ...
		*trace = append(*trace, TraceTerm{
			Index: indexes.IsolatedPawns,
			MG:    float64(terms.IsolatedPawns),
			EG:    float64(terms.IsolatedPawns),
		})
	}
}
func get_phalanx_pawns_trace(trace *[]TraceTerm, terms *engine.EvaluationTerms, indexes *Indexes) {
	if terms.PhalanxPawns != 0 { // We only want to tune if we actually got that piece ...
		*trace = append(*trace, TraceTerm{
			Index: indexes.PhalanxPawns,
			MG:    float64(terms.PhalanxPawns),
			EG:    float64(terms.PhalanxPawns),
		})
	}
}
func get_connected_pawns_trace(trace *[]TraceTerm, terms *engine.EvaluationTerms, indexes *Indexes) {
	if terms.ConnectedPawns != 0 { // We only want to tune if we actually got that piece ...
		*trace = append(*trace, TraceTerm{
			Index: indexes.ConnectedPawns,
			MG:    float64(terms.ConnectedPawns),
			EG:    float64(terms.ConnectedPawns),
		})
	}
}
func get_blocked_pawns_trace(trace *[]TraceTerm, terms *engine.EvaluationTerms, indexes *Indexes) {
	if terms.BlockedPawns != 0 { // We only want to tune if we actually got that piece ...
		*trace = append(*trace, TraceTerm{
			Index: indexes.BlockedPawns,
			MG:    float64(terms.BlockedPawns),
			EG:    float64(terms.BlockedPawns),
		})
	}
}

/* ============ Knight trace function ============ */
func get_knight_outpost_trace(trace *[]TraceTerm, terms *engine.EvaluationTerms, indexes *Indexes) {
	var knightOutpostCount int = bits.OnesCount64(terms.KnightOutposts[0]) - bits.OnesCount64(terms.KnightOutposts[1])
	if knightOutpostCount != 0 { // We only want to tune if we actually got that piece ...
		*trace = append(*trace, TraceTerm{
			Index: indexes.KnightOutpost,
			MG:    float64(knightOutpostCount),
			EG:    float64(knightOutpostCount),
		})
	}
}

/* ============ Bishop trace function ============ */
func get_bishop_outpost_trace(trace *[]TraceTerm, terms *engine.EvaluationTerms, indexes *Indexes) {
	var bishopOutpostCount int = bits.OnesCount64(terms.BishopOutpost[0]) - bits.OnesCount64(terms.BishopOutpost[1])
	if bishopOutpostCount != 0 { // We only want to tune if we actually got that piece ...
		*trace = append(*trace, TraceTerm{
			Index: indexes.BishopOutpost,
			MG:    float64(bishopOutpostCount),
			EG:    float64(bishopOutpostCount),
		})
	}
}

/* ============ Bishop trace function ============ */
func get_bishop_pair_trace(trace *[]TraceTerm, terms *engine.EvaluationTerms, indexes *Indexes) {
	var bishopPairCount int
	if terms.BishopPairs[0] {
		bishopPairCount++
	}
	if terms.BishopPairs[1] {
		bishopPairCount--
	}
	if bishopPairCount != 0 { // We only want to tune if we actually got that piece ...
		*trace = append(*trace, TraceTerm{
			Index: indexes.BishopPair,
			MG:    float64(bishopPairCount),
			EG:    float64(bishopPairCount),
		})
	}
}

/* ============ Rook trace functions ============ */
func get_rook_semiopen_file_trace(trace *[]TraceTerm, terms *engine.EvaluationTerms, indexes *Indexes) {
	if terms.RookSemiOpenFile != 0 { // We only want to tune if we actually got that piece ...
		*trace = append(*trace, TraceTerm{
			Index: indexes.RookSemiOpenFile,
			MG:    float64(terms.RookSemiOpenFile),
			EG:    0.0,
		})
	}
}
func get_rook_open_file_trace(trace *[]TraceTerm, terms *engine.EvaluationTerms, indexes *Indexes) {
	if terms.RookOpenFile != 0 { // We only want to tune if we actually got that piece ...
		*trace = append(*trace, TraceTerm{
			Index: indexes.RookOpenFile,
			MG:    float64(terms.RookOpenFile),
			EG:    0.0,
		})
	}
}
func get_rook_seventh_rank_trace(trace *[]TraceTerm, terms *engine.EvaluationTerms, indexes *Indexes) {
	if terms.RookSeventhRank != 0 { // We only want to tune if we actually got that piece ...
		*trace = append(*trace, TraceTerm{
			Index: indexes.RookSeventhRank,
			MG:    0.0,
			EG:    float64(terms.RookSeventhRank),
		})
	}
}
func get_rook_xray_trace(trace *[]TraceTerm, terms *engine.EvaluationTerms, indexes *Indexes) {
	if terms.RookXrayAttack != 0 { // We only want to tune if we actually got that piece ...
		*trace = append(*trace, TraceTerm{
			Index: indexes.RookXrayAttack,
			MG:    float64(terms.RookXrayAttack),
			EG:    0,
		})
	}
}

/* ============ Queen trace functions ============ */
func get_queen_centralized_trace(trace *[]TraceTerm, terms *engine.EvaluationTerms, indexes *Indexes) {
	if terms.CentralizedQueen != 0 { // We only want to tune if we actually got that piece ...
		*trace = append(*trace, TraceTerm{
			Index: indexes.CentralizedQueen,
			MG:    0,
			EG:    float64(terms.CentralizedQueen),
		})
	}
}
func get_queen_infiltration_trace(trace *[]TraceTerm, terms *engine.EvaluationTerms, indexes *Indexes) {
	if terms.QueenInfiltration != 0 { // We only want to tune if we actually got that piece ...
		*trace = append(*trace, TraceTerm{
			Index: indexes.QueenInfiltration,
			MG:    float64(terms.QueenInfiltration),
			EG:    float64(terms.QueenInfiltration),
		})
	}
}

/* ============ King trace functions ============ */
//func get_kingcentral_manhattan_trace(trace *[]TraceTerm, terms *engine.EvaluationTerms, indexes *Indexes) {
//	if terms.WPieceCount > terms.BPieceCount && (terms.WPieceCount - terms.BPieceCount) > 2 {
//
//	}
//	if terms.KingCentralManhattanPenalty[] != 0 { // We only want to tune if we actually got that piece ...
//		*trace = append(*trace, TraceTerm{
//			Index: indexes.KingCentralManhattanPenalty,
//			MG:    float64(terms.QueenInfiltration),
//			EG:    float64(terms.QueenInfiltration),
//		})
//	}
//}

//func get_king_distance_trace(trace *[]TraceTerm, terms *engine.EvaluationTerms, indexes *Indexes) {
//	if terms.KingDistancePenalty != 0 { // We only want to tune if we actually got that piece ...
//		*trace = append(*trace, TraceTerm{
//			Index: indexes.KingDistancePenalty,
//			MG:    float64(terms.KingDistancePenalty),
//			EG:    float64(terms.KingDistancePenalty),
//		})
//	}
//}
//func get_king_pawn_distance_trace(trace *[]TraceTerm, terms *engine.EvaluationTerms, indexes *Indexes) {
//	if terms.KingPawnDistance != 0 { // We only want to tune if we actually got that piece ...
//		*trace = append(*trace, TraceTerm{
//			Index: indexes.KingPawnDistance,
//			MG:    float64(terms.KingPawnDistance),
//			EG:    float64(terms.KingPawnDistance),
//		})
//	}
//}
func get_king_safety_trace(trace *[]TraceTerm, terms *engine.EvaluationTerms, indexes *Indexes) {
	if terms.KingSafety != 0 { // We only want to tune if we actually got a value to tune ...
		*trace = append(*trace, TraceTerm{
			Index: indexes.KingSafety,
			MG:    float64(terms.KingSafety),
			EG:    0.0,
		})
	}
}
