// tuner/io_json.go
package tuner

import (
    "encoding/json"
    "os"
)

type pstJSON struct {
	MG [6][64]float64 `json:"mg"`
	EG [6][64]float64 `json:"eg"`
	K  float64        `json:"k"`
}

func SaveJSON(path string, pst *PST) error {
	payload := pstJSON{MG: pst.MG, EG: pst.EG, K: pst.K}
	tmp := path + ".tmp"
	b, _ := json.MarshalIndent(payload, "", "  ")
	if err := os.WriteFile(tmp, b, 0o644); err != nil { return err }
	return os.Rename(tmp, path)
}

func LoadJSON(path string, pst *PST) error {
	b, err := os.ReadFile(path)
	if err != nil { return err }
	var p pstJSON
	if err := json.Unmarshal(b, &p); err != nil { return err }
	pst.MG, pst.EG, pst.K = p.MG, p.EG, p.K
	return nil
}

// Extended model serialization: save/load k, PST and θ (featurizer params).
// This keeps PST groups for readability and includes a flat theta for full state.

type modelJSON struct {
    Layout string     `json:"layout"`           // descriptor of θ layout
    K      float64    `json:"k"`                // logistic scale
    PST    *pstJSON   `json:"pst,omitempty"`    // optional, for readability
    Theta  []float64  `json:"theta,omitempty"`  // full parameter vector θ
    // Grouped fields for readability (if available)
    MaterialMG []float64 `json:"material_mg,omitempty"`
    MaterialEG []float64 `json:"material_eg,omitempty"`
    PassersMG  []float64 `json:"passers_mg,omitempty"`
    PassersEG  []float64 `json:"passers_eg,omitempty"`
    // Phase 1 scalars (optional grouped fields)
    BishopPairMG        *float64 `json:"bishop_pair_mg,omitempty"`
    BishopPairEG        *float64 `json:"bishop_pair_eg,omitempty"`
    RookSemiOpenFileMG  *float64 `json:"rook_semi_open_mg,omitempty"`
    RookOpenFileMG      *float64 `json:"rook_open_mg,omitempty"`
    SeventhRankEG       *float64 `json:"seventh_rank_eg,omitempty"`
    QueenCentralizedEG  *float64 `json:"queen_centralized_eg,omitempty"`
    QueenInfiltrationMG *float64 `json:"queen_infiltration_mg,omitempty"`
    QueenInfiltrationEG *float64 `json:"queen_infiltration_eg,omitempty"`
    // Phase 2 pawn-structure scalars (optional grouped fields)
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
    PawnLeverMG *float64 `json:"pawnlever_mg,omitempty"`
    PawnLeverEG *float64 `json:"pawnlever_eg,omitempty"`
    BackwardMG  *float64 `json:"backward_mg,omitempty"`
    BackwardEG  *float64 `json:"backward_eg,omitempty"`
    // Phase 3 arrays
    MobilityMG    []float64 `json:"mobility_mg,omitempty"`
    MobilityEG    []float64 `json:"mobility_eg,omitempty"`
    KingSafety    []float64 `json:"king_safety_table,omitempty"`
}

// SaveModelJSON writes the featurizer parameters θ, current PST, and k.
func SaveModelJSON(path string, fe Featurizer, pst *PST) error {
    layout := "linear_v3_pst_first_square_passers_pawnstruct"
    payload := modelJSON{Layout: layout}
    if pst != nil {
        payload.K = pst.K
        payload.PST = &pstJSON{MG: pst.MG, EG: pst.EG, K: pst.K}
    }
    if fe != nil {
        payload.Theta = append([]float64(nil), fe.Params()...)
        // Best-effort grouped fields for readability if LinearEval
        if le, ok := fe.(*LinearEval); ok {
            // Material
            payload.MaterialMG = make([]float64, 6)
            payload.MaterialEG = make([]float64, 6)
            copy(payload.MaterialMG, le.MatMG[:])
            copy(payload.MaterialEG, le.MatEG[:])
            // Passers (square-based, 64)
            payload.PassersMG = make([]float64, 64)
            payload.PassersEG = make([]float64, 64)
            copy(payload.PassersMG, le.PasserMG[:])
            copy(payload.PassersEG, le.PasserEG[:])
            // Phase 1 scalars
            bpMG, bpEG := le.BishopPairMG, le.BishopPairEG
            rsMG, roMG := le.RookSemiOpenFileMG, le.RookOpenFileMG
            srEG := le.SeventhRankEG
            qcEG := le.QueenCentralizedEG
            qiMG, qiEG := le.QueenInfiltrationMG, le.QueenInfiltrationEG
            payload.BishopPairMG = &bpMG
            payload.BishopPairEG = &bpEG
            payload.RookSemiOpenFileMG = &rsMG
            payload.RookOpenFileMG = &roMG
            payload.SeventhRankEG = &srEG
            payload.QueenCentralizedEG = &qcEG
            payload.QueenInfiltrationMG = &qiMG
            payload.QueenInfiltrationEG = &qiEG
            // Phase 2 pawn-structure scalars
            dMG, dEG := le.DoubledMG, le.DoubledEG
            iMG, iEG := le.IsolatedMG, le.IsolatedEG
            cMG, cEG := le.ConnectedMG, le.ConnectedEG
            phMG, phEG := le.PhalanxMG, le.PhalanxEG
            bMG, bEG := le.BlockedMG, le.BlockedEG
            plMG, plEG := le.PawnLeverMG, le.PawnLeverEG
            bwMG, bwEG := le.BackwardMG, le.BackwardEG
            payload.DoubledMG = &dMG
            payload.DoubledEG = &dEG
            payload.IsolatedMG = &iMG
            payload.IsolatedEG = &iEG
            payload.ConnectedMG = &cMG
            payload.ConnectedEG = &cEG
            payload.PhalanxMG = &phMG
            payload.PhalanxEG = &phEG
            payload.BlockedMG = &bMG
            payload.BlockedEG = &bEG
            payload.PawnLeverMG = &plMG
            payload.PawnLeverEG = &plEG
            payload.BackwardMG = &bwMG
            payload.BackwardEG = &bwEG
            // Phase 3 arrays
            payload.MobilityMG = make([]float64, 7)
            payload.MobilityEG = make([]float64, 7)
            for i := 0; i < 7; i++ {
                payload.MobilityMG[i] = le.MobilityMG[i]
                payload.MobilityEG[i] = le.MobilityEG[i]
            }
            // Phase 4: King safety table
            payload.KingSafety = make([]float64, 100)
            for i := 0; i < 100; i++ { payload.KingSafety[i] = le.KingSafety[i] }
        }
    }
    tmp := path + ".tmp"
    b, _ := json.MarshalIndent(payload, "", "  ")
    if err := os.WriteFile(tmp, b, 0o644); err != nil { return err }
    return os.Rename(tmp, path)
}

// LoadModelJSON reads θ and k (and PST if present) and updates fe and pst.
func LoadModelJSON(path string, fe Featurizer, pst *PST) error {
    b, err := os.ReadFile(path)
    if err != nil { return err }
    var m modelJSON
    if err := json.Unmarshal(b, &m); err != nil { return err }
    // PST block (optional)
    if m.PST != nil && pst != nil {
        pst.MG, pst.EG, pst.K = m.PST.MG, m.PST.EG, m.PST.K
    }
    if pst != nil && m.K != 0 {
        pst.K = m.K
    }
    if fe != nil {
        if len(m.Theta) > 0 {
            fe.SetParams(m.Theta)
        } else if le, ok := fe.(*LinearEval); ok {
            // Populate from grouped fields if available
            if len(m.MaterialMG) == 6 { copy(le.MatMG[:], m.MaterialMG) }
            if len(m.MaterialEG) == 6 { copy(le.MatEG[:], m.MaterialEG) }
            if len(m.PassersMG) == 64 { copy(le.PasserMG[:], m.PassersMG) }
            if len(m.PassersEG) == 64 { copy(le.PasserEG[:], m.PassersEG) }
            // Phase 1 scalars if present
            if m.BishopPairMG != nil { le.BishopPairMG = *m.BishopPairMG }
            if m.BishopPairEG != nil { le.BishopPairEG = *m.BishopPairEG }
            if m.RookSemiOpenFileMG != nil { le.RookSemiOpenFileMG = *m.RookSemiOpenFileMG }
            if m.RookOpenFileMG != nil { le.RookOpenFileMG = *m.RookOpenFileMG }
            if m.SeventhRankEG != nil { le.SeventhRankEG = *m.SeventhRankEG }
            if m.QueenCentralizedEG != nil { le.QueenCentralizedEG = *m.QueenCentralizedEG }
            if m.QueenInfiltrationMG != nil { le.QueenInfiltrationMG = *m.QueenInfiltrationMG }
            if m.QueenInfiltrationEG != nil { le.QueenInfiltrationEG = *m.QueenInfiltrationEG }
            // Phase 2 pawn-structure scalars if present
            if m.DoubledMG != nil { le.DoubledMG = *m.DoubledMG }
            if m.DoubledEG != nil { le.DoubledEG = *m.DoubledEG }
            if m.IsolatedMG != nil { le.IsolatedMG = *m.IsolatedMG }
            if m.IsolatedEG != nil { le.IsolatedEG = *m.IsolatedEG }
            if m.ConnectedMG != nil { le.ConnectedMG = *m.ConnectedMG }
            if m.ConnectedEG != nil { le.ConnectedEG = *m.ConnectedEG }
            if m.PhalanxMG != nil { le.PhalanxMG = *m.PhalanxMG }
            if m.PhalanxEG != nil { le.PhalanxEG = *m.PhalanxEG }
            if m.BlockedMG != nil { le.BlockedMG = *m.BlockedMG }
            if m.BlockedEG != nil { le.BlockedEG = *m.BlockedEG }
            if m.PawnLeverMG != nil { le.PawnLeverMG = *m.PawnLeverMG }
            if m.PawnLeverEG != nil { le.PawnLeverEG = *m.PawnLeverEG }
            if m.BackwardMG  != nil { le.BackwardMG  = *m.BackwardMG }
            if m.BackwardEG  != nil { le.BackwardEG  = *m.BackwardEG }
            if len(m.MobilityMG) == 7 { copy(le.MobilityMG[:], m.MobilityMG) }
            if len(m.MobilityEG) == 7 { copy(le.MobilityEG[:], m.MobilityEG) }
            // Enforce invariant: no Pawn/King mobility weights
            le.MobilityMG[1], le.MobilityMG[6] = 0, 0
            le.MobilityEG[1], le.MobilityEG[6] = 0, 0
            // Phase 4: King safety table
            if len(m.KingSafety) == 100 { copy(le.KingSafety[:], m.KingSafety) }
            // Sync PST already handled above (if present). Rebuild theta from fields.
            _ = le.Params()
        }
    }
    return nil
}
