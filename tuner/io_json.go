// tuner/io_json.go
package tuner

import (
	"encoding/json"
	"os"
)

const modelLayoutTag = "linear_v11_tiered_layout"

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
	MaterialMG       []float64 `json:"material_mg,omitempty"`
	MaterialEG       []float64 `json:"material_eg,omitempty"`
	PassersMG        []float64 `json:"passers_mg,omitempty"`
	PassersEG        []float64 `json:"passers_eg,omitempty"`
	KnightMobilityMG []float64 `json:"knight_mobility_mg,omitempty"`
	KnightMobilityEG []float64 `json:"knight_mobility_eg,omitempty"`
	BishopMobilityMG []float64 `json:"bishop_mobility_mg,omitempty"`
	BishopMobilityEG []float64 `json:"bishop_mobility_eg,omitempty"`
	RookMobilityMG   []float64 `json:"rook_mobility_mg,omitempty"`
	RookMobilityEG   []float64 `json:"rook_mobility_eg,omitempty"`
	QueenMobilityMG  []float64 `json:"queen_mobility_mg,omitempty"`
	QueenMobilityEG  []float64 `json:"queen_mobility_eg,omitempty"`
	KingSafety       []float64 `json:"king_safety_table,omitempty"`
	// Phase 1 scalars
	BishopPairMG       *float64 `json:"bishop_pair_mg,omitempty"`
	BishopPairEG       *float64 `json:"bishop_pair_eg,omitempty"`
	RookSemiOpenFileMG *float64 `json:"rook_semi_open_mg,omitempty"`
	RookOpenFileMG     *float64 `json:"rook_open_mg,omitempty"`
	SeventhRankEG      *float64 `json:"seventh_rank_eg,omitempty"`
	QueenCentralizedEG *float64 `json:"queen_centralized_eg,omitempty"`
	// Phase 2 pawn-structure scalars
	DoubledMG            *float64 `json:"doubled_mg,omitempty"`
	DoubledEG            *float64 `json:"doubled_eg,omitempty"`
	IsolatedMG           *float64 `json:"isolated_mg,omitempty"`
	IsolatedEG           *float64 `json:"isolated_eg,omitempty"`
	ConnectedMG          *float64 `json:"connected_mg,omitempty"`
	ConnectedEG          *float64 `json:"connected_eg,omitempty"`
	PhalanxMG            *float64 `json:"phalanx_mg,omitempty"`
	PhalanxEG            *float64 `json:"phalanx_eg,omitempty"`
	BlockedMG            *float64 `json:"blocked_mg,omitempty"`
	BlockedEG            *float64 `json:"blocked_eg,omitempty"`
	WeakLeverMG          *float64 `json:"weaklever_mg,omitempty"`
	WeakLeverEG          *float64 `json:"weaklever_eg,omitempty"`
	BackwardMG           *float64 `json:"backward_mg,omitempty"`
	BackwardEG           *float64 `json:"backward_eg,omitempty"`
	CandidatePassedPctMG *float64 `json:"candidate_passed_pct_mg,omitempty"`
	CandidatePassedPctEG *float64 `json:"candidate_passed_pct_eg,omitempty"`
	// King correlates
	KingSemiOpenFilePenalty *float64 `json:"king_semi_open_file_penalty,omitempty"`
	KingOpenFilePenalty     *float64 `json:"king_open_file_penalty,omitempty"`
	KingMinorPieceDefense   *float64 `json:"king_minor_piece_defense_bonus,omitempty"`
	KingPawnDefenseMG       *float64 `json:"king_pawn_defense_mg,omitempty"`
	// Extras
	KnightOutpostMG      *float64 `json:"knight_outpost_mg,omitempty"`
	KnightOutpostEG      *float64 `json:"knight_outpost_eg,omitempty"`
	BishopOutpostMG      *float64 `json:"bishop_outpost_mg,omitempty"`
	BishopOutpostEG      *float64 `json:"bishop_outpost_eg,omitempty"`
	BadBishopMG          *float64 `json:"bad_bishop_mg,omitempty"`
	BadBishopEG          *float64 `json:"bad_bishop_eg,omitempty"`
	StackedRooksMG       *float64 `json:"stacked_rooks_mg,omitempty"`
	PawnStormMG          *float64 `json:"pawn_storm_mg,omitempty"`
	PawnProximityPenalty *float64 `json:"pawn_proximity_penalty_mg,omitempty"`
	KnightMobCenterMG    *float64 `json:"knight_mobility_center_mg,omitempty"`
	BishopMobCenterMG    *float64 `json:"bishop_mobility_center_mg,omitempty"`
	// Imbalance scalars
	ImbalanceKnightPerPawnMG *float64 `json:"imbalance_knight_per_pawn_mg,omitempty"`
	ImbalanceKnightPerPawnEG *float64 `json:"imbalance_knight_per_pawn_eg,omitempty"`
	ImbalanceBishopPerPawnMG *float64 `json:"imbalance_bishop_per_pawn_mg,omitempty"`
	ImbalanceBishopPerPawnEG *float64 `json:"imbalance_bishop_per_pawn_eg,omitempty"`
	// Space/weak-king + tempo
	SpaceMG           *float64 `json:"space_mg,omitempty"`
	SpaceEG           *float64 `json:"space_eg,omitempty"`
	WeakKingSquaresMG *float64 `json:"weak_king_squares_mg,omitempty"`
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
			payload.KnightMobilityMG = append(payload.KnightMobilityMG, le.KnightMobilityMG[:]...)
			payload.KnightMobilityEG = append(payload.KnightMobilityEG, le.KnightMobilityEG[:]...)
			payload.BishopMobilityMG = append(payload.BishopMobilityMG, le.BishopMobilityMG[:]...)
			payload.BishopMobilityEG = append(payload.BishopMobilityEG, le.BishopMobilityEG[:]...)
			payload.RookMobilityMG = append(payload.RookMobilityMG, le.RookMobilityMG[:]...)
			payload.RookMobilityEG = append(payload.RookMobilityEG, le.RookMobilityEG[:]...)
			payload.QueenMobilityMG = append(payload.QueenMobilityMG, le.QueenMobilityMG[:]...)
			payload.QueenMobilityEG = append(payload.QueenMobilityEG, le.QueenMobilityEG[:]...)
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
			payload.CandidatePassedPctMG = floatPtr(le.CandidatePassedPctMG)
			payload.CandidatePassedPctEG = floatPtr(le.CandidatePassedPctEG)
			// King correlates
			payload.KingSemiOpenFilePenalty = floatPtr(le.KingSemiOpenFilePenalty)
			payload.KingOpenFilePenalty = floatPtr(le.KingOpenFilePenalty)
			payload.KingMinorPieceDefense = floatPtr(le.KingMinorPieceDefense)
			payload.KingPawnDefenseMG = floatPtr(le.KingPawnDefenseMG)
			// Extras
			payload.KnightOutpostMG = floatPtr(le.KnightOutpostMG)
			payload.KnightOutpostEG = floatPtr(le.KnightOutpostEG)
			payload.BishopOutpostMG = floatPtr(le.BishopOutpostMG)
			payload.BishopOutpostEG = floatPtr(le.BishopOutpostEG)
			payload.BadBishopMG = floatPtr(le.BadBishopMG)
			payload.BadBishopEG = floatPtr(le.BadBishopEG)
			payload.StackedRooksMG = floatPtr(le.StackedRooksMG)
			payload.PawnProximityPenalty = floatPtr(le.PawnProximityMG)
			payload.KnightMobCenterMG = floatPtr(le.KnightMobCenterMG)
			payload.BishopMobCenterMG = floatPtr(le.BishopMobCenterMG)
			// Imbalance
			payload.ImbalanceKnightPerPawnMG = floatPtr(le.ImbalanceKnightPerPawnMG)
			payload.ImbalanceKnightPerPawnEG = floatPtr(le.ImbalanceKnightPerPawnEG)
			payload.ImbalanceBishopPerPawnMG = floatPtr(le.ImbalanceBishopPerPawnMG)
			payload.ImbalanceBishopPerPawnEG = floatPtr(le.ImbalanceBishopPerPawnEG)
			// Space/weak-king + tempo
			payload.SpaceMG = floatPtr(le.SpaceMG)
			payload.SpaceEG = floatPtr(le.SpaceEG)
			payload.WeakKingSquaresMG = floatPtr(le.WeakKingSquaresMG)
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
	if len(m.KnightMobilityMG) == len(le.KnightMobilityMG) {
		copy(le.KnightMobilityMG[:], m.KnightMobilityMG)
	}
	if len(m.KnightMobilityEG) == len(le.KnightMobilityEG) {
		copy(le.KnightMobilityEG[:], m.KnightMobilityEG)
	}
	if len(m.BishopMobilityMG) == len(le.BishopMobilityMG) {
		copy(le.BishopMobilityMG[:], m.BishopMobilityMG)
	}
	if len(m.BishopMobilityEG) == len(le.BishopMobilityEG) {
		copy(le.BishopMobilityEG[:], m.BishopMobilityEG)
	}
	if len(m.RookMobilityMG) == len(le.RookMobilityMG) {
		copy(le.RookMobilityMG[:], m.RookMobilityMG)
	}
	if len(m.RookMobilityEG) == len(le.RookMobilityEG) {
		copy(le.RookMobilityEG[:], m.RookMobilityEG)
	}
	if len(m.QueenMobilityMG) == len(le.QueenMobilityMG) {
		copy(le.QueenMobilityMG[:], m.QueenMobilityMG)
	}
	if len(m.QueenMobilityEG) == len(le.QueenMobilityEG) {
		copy(le.QueenMobilityEG[:], m.QueenMobilityEG)
	}
	if len(m.KingSafety) == 100 {
		copy(le.KingSafety[:], m.KingSafety)
	}
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
	if m.CandidatePassedPctMG != nil {
		le.CandidatePassedPctMG = *m.CandidatePassedPctMG
	}
	if m.CandidatePassedPctEG != nil {
		le.CandidatePassedPctEG = *m.CandidatePassedPctEG
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
	if m.BishopOutpostEG != nil {
		le.BishopOutpostEG = *m.BishopOutpostEG
	}
	if m.BadBishopMG != nil {
		le.BadBishopMG = *m.BadBishopMG
	}
	if m.BadBishopEG != nil {
		le.BadBishopEG = *m.BadBishopEG
	}
	if m.StackedRooksMG != nil {
		le.StackedRooksMG = *m.StackedRooksMG
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
