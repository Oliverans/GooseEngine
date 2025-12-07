// tuner/io_json.go
package tuner

import (
	"encoding/json"
	"os"

	gm "chess-engine/goosemg"
)

const modelLayoutTag = "linear_v6_pst_first_square_passers_phase6_no_pawnlever"

type pstJSON struct {
	MG [6][64]float64 `json:"mg"`
	EG [6][64]float64 `json:"eg"`
	K  float64        `json:"k"`
}

func SaveJSON(path string, pst *PST) error {
	payload := pstJSON{MG: pst.MG, EG: pst.EG, K: pst.K}
	tmp := path + ".tmp"
	b, _ := json.MarshalIndent(payload, "", "  ")
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func LoadJSON(path string, pst *PST) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var p pstJSON
	if err := json.Unmarshal(b, &p); err != nil {
		return err
	}
	pst.MG, pst.EG, pst.K = p.MG, p.EG, p.K
	return nil
}

// Extended model serialization: save/load k, PST and θ (featurizer params).
// This keeps PST groups for readability and includes a flat theta for full state.

type modelJSON struct {
	Layout string    `json:"layout"`
	K      float64   `json:"k"`
	PST    *pstJSON  `json:"pst,omitempty"`
	Theta  []float64 `json:"theta,omitempty"`
	// Grouped arrays
	MaterialMG []float64 `json:"material_mg,omitempty"`
	MaterialEG []float64 `json:"material_eg,omitempty"`
	PassersMG  []float64 `json:"passers_mg,omitempty"`
	PassersEG  []float64 `json:"passers_eg,omitempty"`
	MobilityMG []float64 `json:"mobility_mg,omitempty"`
	MobilityEG []float64 `json:"mobility_eg,omitempty"`
	KingSafety []float64 `json:"king_safety_table,omitempty"`
	// Phase 1 scalars
	BishopPairMG       *float64 `json:"bishop_pair_mg,omitempty"`
	BishopPairEG       *float64 `json:"bishop_pair_eg,omitempty"`
	RookSemiOpenFileMG *float64 `json:"rook_semi_open_mg,omitempty"`
	RookOpenFileMG     *float64 `json:"rook_open_mg,omitempty"`
	SeventhRankEG      *float64 `json:"seventh_rank_eg,omitempty"`
	QueenCentralizedEG *float64 `json:"queen_centralized_eg,omitempty"`
	// Phase 2 pawn-structure scalars
	DoubledMG   *float64 `json:"doubled_mg,omitempty"`
	DoubledEG   *float64 `json:"doubled_eg,omitempty"`
	IsolatedMG  *float64 `json:"isolated_mg,omitempty"`
	IsolatedEG  *float64 `json:"isolated_eg,omitempty"`
	ConnectedMG *float64 `json:"connected_mg,omitempty"`
	ConnectedEG *float64 `json:"connected_eg,omitempty"`
	PhalanxMG   *float64 `json:"phalanx_mg,omitempty"`
	PhalanxEG   *float64 `json:"phalanx_eg,omitempty"`
	BlockedMG   *float64 `json:"blocked_mg,omitempty"`
	BlockedEG   *float64 `json:"blocked_eg,omitempty"`
	WeakLeverMG *float64 `json:"weaklever_mg,omitempty"`
	WeakLeverEG *float64 `json:"weaklever_eg,omitempty"`
	BackwardMG  *float64 `json:"backward_mg,omitempty"`
	BackwardEG  *float64 `json:"backward_eg,omitempty"`
	// King correlates
	KingSemiOpenFilePenalty *float64 `json:"king_semi_open_file_penalty,omitempty"`
	KingOpenFilePenalty     *float64 `json:"king_open_file_penalty,omitempty"`
	KingMinorPieceDefense   *float64 `json:"king_minor_piece_defense_bonus,omitempty"`
	KingPawnDefenseMG       *float64 `json:"king_pawn_defense_mg,omitempty"`
	// Extras
	KnightOutpostMG      *float64 `json:"knight_outpost_mg,omitempty"`
	KnightOutpostEG      *float64 `json:"knight_outpost_eg,omitempty"`
	BishopOutpostMG      *float64 `json:"bishop_outpost_mg,omitempty"`
	KnightThreatsMG      *float64 `json:"knight_can_attack_piece_mg,omitempty"`
	KnightThreatsEG      *float64 `json:"knight_can_attack_piece_eg,omitempty"`
	StackedRooksMG       *float64 `json:"stacked_rooks_mg,omitempty"`
	PawnStormMG          *float64 `json:"pawn_storm_mg,omitempty"`
	PawnProximityPenalty *float64 `json:"pawn_proximity_penalty_mg,omitempty"`
	KnightMobCenterMG    *float64 `json:"knight_mobility_center_mg,omitempty"`
	BishopMobCenterMG    *float64 `json:"bishop_mobility_center_mg,omitempty"`
	// Imbalance scalars
	ImbalanceKnightPerPawnMG    *float64 `json:"imbalance_knight_per_pawn_mg,omitempty"`
	ImbalanceKnightPerPawnEG    *float64 `json:"imbalance_knight_per_pawn_eg,omitempty"`
	ImbalanceBishopPerPawnMG    *float64 `json:"imbalance_bishop_per_pawn_mg,omitempty"`
	ImbalanceBishopPerPawnEG    *float64 `json:"imbalance_bishop_per_pawn_eg,omitempty"`
	ImbalanceMinorsForMajorMG   *float64 `json:"imbalance_minors_for_major_mg,omitempty"`
	ImbalanceMinorsForMajorEG   *float64 `json:"imbalance_minors_for_major_eg,omitempty"`
	ImbalanceRedundantRookMG    *float64 `json:"imbalance_redundant_rook_mg,omitempty"`
	ImbalanceRedundantRookEG    *float64 `json:"imbalance_redundant_rook_eg,omitempty"`
	ImbalanceRookQueenOverlapMG *float64 `json:"imbalance_rook_queen_overlap_mg,omitempty"`
	ImbalanceRookQueenOverlapEG *float64 `json:"imbalance_rook_queen_overlap_eg,omitempty"`
	ImbalanceQueenManyMinorsMG  *float64 `json:"imbalance_queen_many_minors_mg,omitempty"`
	ImbalanceQueenManyMinorsEG  *float64 `json:"imbalance_queen_many_minors_eg,omitempty"`
	// Space/weak-king + tempo
	SpaceMG           *float64 `json:"space_mg,omitempty"`
	SpaceEG           *float64 `json:"space_eg,omitempty"`
	WeakKingSquaresMG *float64 `json:"weak_king_squares_mg,omitempty"`
	WeakKingSquaresEG *float64 `json:"weak_king_squares_eg,omitempty"`
	Tempo             *float64 `json:"tempo_bonus,omitempty"`
}

// SaveModelJSON writes the featurizer parameters θ, current PST, and k.
func SaveModelJSON(path string, fe Featurizer, pst *PST) error {
	payload := modelJSON{Layout: modelLayoutTag}
	if pst != nil {
		payload.K = pst.K
		payload.PST = &pstJSON{MG: pst.MG, EG: pst.EG, K: pst.K}
	}
	if fe != nil {
		payload.Theta = append([]float64(nil), fe.Params()...)
		if le, ok := fe.(*LinearEval); ok {
			payload.MaterialMG = append(payload.MaterialMG, le.MatMG[:]...)
			payload.MaterialEG = append(payload.MaterialEG, le.MatEG[:]...)
			payload.PassersMG = append(payload.PassersMG, le.PasserMG[:]...)
			payload.PassersEG = append(payload.PassersEG, le.PasserEG[:]...)
			payload.MobilityMG = append(payload.MobilityMG, le.MobilityMG[:]...)
			payload.MobilityEG = append(payload.MobilityEG, le.MobilityEG[:]...)
			payload.KingSafety = append(payload.KingSafety, le.KingSafety[:]...)
			// Phase 1
			payload.BishopPairMG = floatPtr(le.BishopPairMG)
			payload.BishopPairEG = floatPtr(le.BishopPairEG)
			payload.RookSemiOpenFileMG = floatPtr(le.RookSemiOpenFileMG)
			payload.RookOpenFileMG = floatPtr(le.RookOpenFileMG)
			payload.SeventhRankEG = floatPtr(le.SeventhRankEG)
			payload.QueenCentralizedEG = floatPtr(le.QueenCentralizedEG)
			// Phase 2
			payload.DoubledMG = floatPtr(le.DoubledMG)
			payload.DoubledEG = floatPtr(le.DoubledEG)
			payload.IsolatedMG = floatPtr(le.IsolatedMG)
			payload.IsolatedEG = floatPtr(le.IsolatedEG)
			payload.ConnectedMG = floatPtr(le.ConnectedMG)
			payload.ConnectedEG = floatPtr(le.ConnectedEG)
			payload.PhalanxMG = floatPtr(le.PhalanxMG)
			payload.PhalanxEG = floatPtr(le.PhalanxEG)
			payload.BlockedMG = floatPtr(le.BlockedMG)
			payload.BlockedEG = floatPtr(le.BlockedEG)
			payload.WeakLeverMG = floatPtr(le.WeakLeverMG)
			payload.WeakLeverEG = floatPtr(le.WeakLeverEG)
			payload.BackwardMG = floatPtr(le.BackwardMG)
			payload.BackwardEG = floatPtr(le.BackwardEG)
			// King correlates
			payload.KingSemiOpenFilePenalty = floatPtr(le.KingSemiOpenFilePenalty)
			payload.KingOpenFilePenalty = floatPtr(le.KingOpenFilePenalty)
			payload.KingMinorPieceDefense = floatPtr(le.KingMinorPieceDefense)
			payload.KingPawnDefenseMG = floatPtr(le.KingPawnDefenseMG)
			// Extras
			payload.KnightOutpostMG = floatPtr(le.KnightOutpostMG)
			payload.KnightOutpostEG = floatPtr(le.KnightOutpostEG)
			payload.BishopOutpostMG = floatPtr(le.BishopOutpostMG)
			payload.KnightThreatsMG = floatPtr(le.KnightThreatsMG)
			payload.KnightThreatsEG = floatPtr(le.KnightThreatsEG)
			payload.StackedRooksMG = floatPtr(le.StackedRooksMG)
			payload.PawnStormMG = floatPtr(le.PawnStormMG)
			payload.PawnProximityPenalty = floatPtr(le.PawnProximityMG)
			payload.KnightMobCenterMG = floatPtr(le.KnightMobCenterMG)
			payload.BishopMobCenterMG = floatPtr(le.BishopMobCenterMG)
			// Imbalance
			payload.ImbalanceKnightPerPawnMG = floatPtr(le.ImbalanceKnightPerPawnMG)
			payload.ImbalanceKnightPerPawnEG = floatPtr(le.ImbalanceKnightPerPawnEG)
			payload.ImbalanceBishopPerPawnMG = floatPtr(le.ImbalanceBishopPerPawnMG)
			payload.ImbalanceBishopPerPawnEG = floatPtr(le.ImbalanceBishopPerPawnEG)
			payload.ImbalanceMinorsForMajorMG = floatPtr(le.ImbalanceMinorsForMajorMG)
			payload.ImbalanceMinorsForMajorEG = floatPtr(le.ImbalanceMinorsForMajorEG)
			payload.ImbalanceRedundantRookMG = floatPtr(le.ImbalanceRedundantRookMG)
			payload.ImbalanceRedundantRookEG = floatPtr(le.ImbalanceRedundantRookEG)
			payload.ImbalanceRookQueenOverlapMG = floatPtr(le.ImbalanceRookQueenOverlapMG)
			payload.ImbalanceRookQueenOverlapEG = floatPtr(le.ImbalanceRookQueenOverlapEG)
			payload.ImbalanceQueenManyMinorsMG = floatPtr(le.ImbalanceQueenManyMinorsMG)
			payload.ImbalanceQueenManyMinorsEG = floatPtr(le.ImbalanceQueenManyMinorsEG)
			// Space/weak-king + tempo
			payload.SpaceMG = floatPtr(le.SpaceMG)
			payload.SpaceEG = floatPtr(le.SpaceEG)
			payload.WeakKingSquaresMG = floatPtr(le.WeakKingSquaresMG)
			payload.WeakKingSquaresEG = floatPtr(le.WeakKingSquaresEG)
			payload.Tempo = floatPtr(le.Tempo)
		}
	}
	tmp := path + ".tmp"
	b, _ := json.MarshalIndent(payload, "", "  ")
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// LoadModelJSON reads θ and k (and PST if present) and updates fe and pst.
func LoadModelJSON(path string, fe Featurizer, pst *PST) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var m modelJSON
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	if m.PST != nil && pst != nil {
		pst.MG, pst.EG, pst.K = m.PST.MG, m.PST.EG, m.PST.K
	}
	if pst != nil && m.K != 0 {
		pst.K = m.K
	}
	if fe == nil {
		return nil
	}
	if len(m.Theta) > 0 {
		fe.SetParams(m.Theta)
		return nil
	}
	le, ok := fe.(*LinearEval)
	if !ok {
		return nil
	}
	if len(m.MaterialMG) == 6 {
		copy(le.MatMG[:], m.MaterialMG)
	}
	if len(m.MaterialEG) == 6 {
		copy(le.MatEG[:], m.MaterialEG)
	}
	if len(m.PassersMG) == 64 {
		copy(le.PasserMG[:], m.PassersMG)
	}
	if len(m.PassersEG) == 64 {
		copy(le.PasserEG[:], m.PassersEG)
	}
	if len(m.MobilityMG) == 7 {
		copy(le.MobilityMG[:], m.MobilityMG)
	}
	if len(m.MobilityEG) == 7 {
		copy(le.MobilityEG[:], m.MobilityEG)
	}
	if len(m.KingSafety) == 100 {
		copy(le.KingSafety[:], m.KingSafety)
	}
	// Enforce invariant: Pawn/King mobility not trained.
	le.MobilityMG[gm.PieceTypePawn], le.MobilityMG[gm.PieceTypeKing] = 0, 0
	le.MobilityEG[gm.PieceTypePawn], le.MobilityEG[gm.PieceTypeKing] = 0, 0
	// Phase 1
	if m.BishopPairMG != nil {
		le.BishopPairMG = *m.BishopPairMG
	}
	if m.BishopPairEG != nil {
		le.BishopPairEG = *m.BishopPairEG
	}
	if m.RookSemiOpenFileMG != nil {
		le.RookSemiOpenFileMG = *m.RookSemiOpenFileMG
	}
	if m.RookOpenFileMG != nil {
		le.RookOpenFileMG = *m.RookOpenFileMG
	}
	if m.SeventhRankEG != nil {
		le.SeventhRankEG = *m.SeventhRankEG
	}
	if m.QueenCentralizedEG != nil {
		le.QueenCentralizedEG = *m.QueenCentralizedEG
	}
	// Phase 2
	if m.DoubledMG != nil {
		le.DoubledMG = *m.DoubledMG
	}
	if m.DoubledEG != nil {
		le.DoubledEG = *m.DoubledEG
	}
	if m.IsolatedMG != nil {
		le.IsolatedMG = *m.IsolatedMG
	}
	if m.IsolatedEG != nil {
		le.IsolatedEG = *m.IsolatedEG
	}
	if m.ConnectedMG != nil {
		le.ConnectedMG = *m.ConnectedMG
	}
	if m.ConnectedEG != nil {
		le.ConnectedEG = *m.ConnectedEG
	}
	if m.PhalanxMG != nil {
		le.PhalanxMG = *m.PhalanxMG
	}
	if m.PhalanxEG != nil {
		le.PhalanxEG = *m.PhalanxEG
	}
	if m.BlockedMG != nil {
		le.BlockedMG = *m.BlockedMG
	}
	if m.BlockedEG != nil {
		le.BlockedEG = *m.BlockedEG
	}
	if m.WeakLeverMG != nil {
		le.WeakLeverMG = *m.WeakLeverMG
	}
	if m.WeakLeverEG != nil {
		le.WeakLeverEG = *m.WeakLeverEG
	}
	if m.BackwardMG != nil {
		le.BackwardMG = *m.BackwardMG
	}
	if m.BackwardEG != nil {
		le.BackwardEG = *m.BackwardEG
	}
	// King correlates
	if m.KingSemiOpenFilePenalty != nil {
		le.KingSemiOpenFilePenalty = *m.KingSemiOpenFilePenalty
	}
	if m.KingOpenFilePenalty != nil {
		le.KingOpenFilePenalty = *m.KingOpenFilePenalty
	}
	if m.KingMinorPieceDefense != nil {
		le.KingMinorPieceDefense = *m.KingMinorPieceDefense
	}
	if m.KingPawnDefenseMG != nil {
		le.KingPawnDefenseMG = *m.KingPawnDefenseMG
	}
	// Extras
	if m.KnightOutpostMG != nil {
		le.KnightOutpostMG = *m.KnightOutpostMG
	}
	if m.KnightOutpostEG != nil {
		le.KnightOutpostEG = *m.KnightOutpostEG
	}
	if m.BishopOutpostMG != nil {
		le.BishopOutpostMG = *m.BishopOutpostMG
	}
	if m.KnightThreatsMG != nil {
		le.KnightThreatsMG = *m.KnightThreatsMG
	}
	if m.KnightThreatsEG != nil {
		le.KnightThreatsEG = *m.KnightThreatsEG
	}
	if m.StackedRooksMG != nil {
		le.StackedRooksMG = *m.StackedRooksMG
	}
	if m.PawnStormMG != nil {
		le.PawnStormMG = *m.PawnStormMG
	}
	if m.PawnProximityPenalty != nil {
		le.PawnProximityMG = *m.PawnProximityPenalty
	}
	if m.KnightMobCenterMG != nil {
		le.KnightMobCenterMG = *m.KnightMobCenterMG
	}
	if m.BishopMobCenterMG != nil {
		le.BishopMobCenterMG = *m.BishopMobCenterMG
	}
	// Imbalance
	if m.ImbalanceKnightPerPawnMG != nil {
		le.ImbalanceKnightPerPawnMG = *m.ImbalanceKnightPerPawnMG
	}
	if m.ImbalanceKnightPerPawnEG != nil {
		le.ImbalanceKnightPerPawnEG = *m.ImbalanceKnightPerPawnEG
	}
	if m.ImbalanceBishopPerPawnMG != nil {
		le.ImbalanceBishopPerPawnMG = *m.ImbalanceBishopPerPawnMG
	}
	if m.ImbalanceBishopPerPawnEG != nil {
		le.ImbalanceBishopPerPawnEG = *m.ImbalanceBishopPerPawnEG
	}
	if m.ImbalanceMinorsForMajorMG != nil {
		le.ImbalanceMinorsForMajorMG = *m.ImbalanceMinorsForMajorMG
	}
	if m.ImbalanceMinorsForMajorEG != nil {
		le.ImbalanceMinorsForMajorEG = *m.ImbalanceMinorsForMajorEG
	}
	if m.ImbalanceRedundantRookMG != nil {
		le.ImbalanceRedundantRookMG = *m.ImbalanceRedundantRookMG
	}
	if m.ImbalanceRedundantRookEG != nil {
		le.ImbalanceRedundantRookEG = *m.ImbalanceRedundantRookEG
	}
	if m.ImbalanceRookQueenOverlapMG != nil {
		le.ImbalanceRookQueenOverlapMG = *m.ImbalanceRookQueenOverlapMG
	}
	if m.ImbalanceRookQueenOverlapEG != nil {
		le.ImbalanceRookQueenOverlapEG = *m.ImbalanceRookQueenOverlapEG
	}
	if m.ImbalanceQueenManyMinorsMG != nil {
		le.ImbalanceQueenManyMinorsMG = *m.ImbalanceQueenManyMinorsMG
	}
	if m.ImbalanceQueenManyMinorsEG != nil {
		le.ImbalanceQueenManyMinorsEG = *m.ImbalanceQueenManyMinorsEG
	}
	// Space/weak-king + tempo
	if m.SpaceMG != nil {
		le.SpaceMG = *m.SpaceMG
	}
	if m.SpaceEG != nil {
		le.SpaceEG = *m.SpaceEG
	}
	if m.WeakKingSquaresMG != nil {
		le.WeakKingSquaresMG = *m.WeakKingSquaresMG
	}
	if m.WeakKingSquaresEG != nil {
		le.WeakKingSquaresEG = *m.WeakKingSquaresEG
	}
	if m.Tempo != nil {
		le.Tempo = *m.Tempo
	}
	// rebuild theta buffer from struct fields
	_ = le.Params()
	return nil
}

func floatPtr(v float64) *float64 {
	val := v
	return &val
}
