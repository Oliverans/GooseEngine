package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// Local copy of tuner/model JSON schema (subset)
type pstJSON struct {
	MG [6][64]float64 `json:"mg"`
	EG [6][64]float64 `json:"eg"`
	K  float64        `json:"k"`
}

type modelJSON struct {
	Layout string    `json:"layout"`
	K      float64   `json:"k"`
	PST    *pstJSON  `json:"pst,omitempty"`
	Theta  []float64 `json:"theta,omitempty"`
	// Grouped fields
	MaterialMG []float64 `json:"material_mg,omitempty"`
	MaterialEG []float64 `json:"material_eg,omitempty"`
	PassersMG  []float64 `json:"passers_mg,omitempty"`
	PassersEG  []float64 `json:"passers_eg,omitempty"`
	// Phase 1 scalars
	BishopPairMG        *float64 `json:"bishop_pair_mg,omitempty"`
	BishopPairEG        *float64 `json:"bishop_pair_eg,omitempty"`
	RookSemiOpenFileMG  *float64 `json:"rook_semi_open_mg,omitempty"`
	RookOpenFileMG      *float64 `json:"rook_open_mg,omitempty"`
	SeventhRankEG       *float64 `json:"seventh_rank_eg,omitempty"`
	QueenCentralizedEG  *float64 `json:"queen_centralized_eg,omitempty"`
	QueenInfiltrationMG *float64 `json:"queen_infiltration_mg,omitempty"`
	QueenInfiltrationEG *float64 `json:"queen_infiltration_eg,omitempty"`
	// Phase 2
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
	// Phase 3
	MobilityMG []float64 `json:"mobility_mg,omitempty"`
	MobilityEG []float64 `json:"mobility_eg,omitempty"`
	// Phase 4
	KingSafety []float64 `json:"king_safety_table,omitempty"`
	// Imbalance scalars (optional grouped fields)
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
}

// Theta layout (length 1084)
// 0..383:  PST MG (6x64)
// 384..767: PST EG (6x64)
// 768..773: Material MG (6)
// 774..779: Material EG (6)
// 780..843: Passers MG (64)
// 844..907: Passers EG (64)
// 908..915: Phase 1 scalars (8)
// 916..929: Phase 2 scalars (14)
// 930..936: Mobility MG (7)
// 937..943: Mobility EG (7)
// 944..1043: KingSafetyTable (100)
// 1044..1047: King-safety correlates (4)
// 1048..1049: King endgame terms (2) (not exported)
// 1050..1066: Extras (17)
// 1067..1078: Material imbalance scalars (12)
// 1079..1083: Weak/Protected/Tempo (5)

func rd(x float64) int { return int(math.Round(x)) }

func formatArray1x64(vals []int) string {
	var b strings.Builder
	for i := 0; i < 64; i++ {
		if i%8 == 0 {
			if i != 0 {
				b.WriteString("\n\t\t")
			} else {
				b.WriteString("\t\t")
			}
		}
		b.WriteString(fmt.Sprintf("%d,", vals[i]))
	}
	return b.String()
}

func formatArray6x64(vals [6][64]int) string {
	// keyed by gm piece types 1..6 (P..K) in engine
	var b strings.Builder
	pieceOrder := []string{
		"gm.PieceTypePawn",
		"gm.PieceTypeKnight",
		"gm.PieceTypeBishop",
		"gm.PieceTypeRook",
		"gm.PieceTypeQueen",
		"gm.PieceTypeKing",
	}
	for pi := 0; pi < 6; pi++ {
		b.WriteString("\t\t" + pieceOrder[pi] + ": {\n")
		for i := 0; i < 64; i++ {
			if i%8 == 0 {
				b.WriteString("\t\t\t")
			}
			b.WriteString(fmt.Sprintf("%d,", vals[pi][i]))
			if i%8 == 7 {
				b.WriteString("\n")
			}
		}
		b.WriteString("\t\t},\n")
	}
	return b.String()
}

func formatArray100(vals []int) string {
	var b strings.Builder
	for i := 0; i < 100; i++ {
		if i%10 == 0 {
			b.WriteString("\t\t")
		}
		b.WriteString(fmt.Sprintf("%d,", vals[i]))
		if i%10 == 9 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func main() {
	inPath := flag.String("in", "model.json", "input model JSON path")
	outPath := flag.String("out", "engine/evaluation_gen.go", "output path for generated evaluation_gen.go")
	flag.Parse()

	raw, err := os.ReadFile(*inPath)
	if err != nil {
		panic(err)
	}
	var m modelJSON
	if err := json.Unmarshal(raw, &m); err != nil {
		panic(err)
	}

	// Storage
	var pstMG, pstEG [6][64]int
	var matMG, matEG [6]int
	var passMG, passEG [64]int
	// Phase 1
	var bpMG, bpEG, rsMG, roMG, srEG, qcEG, qiMG, qiEG int
	// Phase 2
	var dMG, dEG, iMG, iEG, cMG, cEG, phMG, phEG, bMG, bEG, wlMG, wlEG, bwMG, bwEG int
	// Mobility
	var mobMG, mobEG [7]int
	// King safety table + correlates
	var ks [100]int
	var kSemi, kOpen, kMinor, kPawn int
	// Extras
	var ex_knOutMG, ex_knOutEG, ex_knThrMG, ex_knThrEG int
	var ex_tropMG, ex_tropEG int
	var ex_stackMG, ex_xrayQMG, ex_xrayKMG, ex_biOutMG int
	var ex_bxKMG, ex_bxRMG, ex_bxQMG, ex_stormMG, ex_proxMG int
	var ex_knMobCenterMG, ex_biMobCenterMG int
	// Imbalance scalars
	var imbKnPawnMG, imbKnPawnEG, imbBiPawnMG, imbBiPawnEG, imbMinorMajorMG, imbMinorMajorEG int
	var imbRedRookMG, imbRedRookEG, imbRookQueenMG, imbRookQueenEG, imbQueenMinorsMG, imbQueenMinorsEG int
	// Phase 6
	var weakMG, weakKingMG, protMG, protKingMG, tempo int

	// Avoid unused warnings for tuner-only extras we don't export
	_ = ex_knMobCenterMG
	_ = ex_biMobCenterMG

	if len(m.Theta) > 0 {
		th := m.Theta
		idx := 0
		// PST MG
		for pt := 0; pt < 6; pt++ {
			for i := 0; i < 64; i++ {
				if idx < len(th) {
					pstMG[pt][i] = rd(th[idx])
					idx++
				}
			}
		}
		// PST EG
		for pt := 0; pt < 6; pt++ {
			for i := 0; i < 64; i++ {
				if idx < len(th) {
					pstEG[pt][i] = rd(th[idx])
					idx++
				}
			}
		}
		// Material MG/EG
		for i := 0; i < 6 && idx < len(th); i++ {
			matMG[i] = rd(th[idx])
			idx++
		}
		for i := 0; i < 6 && idx < len(th); i++ {
			matEG[i] = rd(th[idx])
			idx++
		}
		// Passers MG/EG
		for i := 0; i < 64 && idx < len(th); i++ {
			passMG[i] = rd(th[idx])
			idx++
		}
		for i := 0; i < 64 && idx < len(th); i++ {
			passEG[i] = rd(th[idx])
			idx++
		}
		// Phase 1 (8)
		if idx+8 <= len(th) {
			bpMG = rd(th[idx+0])
			bpEG = rd(th[idx+1])
			rsMG = rd(th[idx+2])
			roMG = rd(th[idx+3])
			srEG = rd(th[idx+4])
			qcEG = rd(th[idx+5])
			qiMG = rd(th[idx+6])
			qiEG = rd(th[idx+7])
		}
		idx += 8
		// Phase 2 (14)
		if idx+14 <= len(th) {
			dMG = rd(th[idx+0])
			dEG = rd(th[idx+1])
			iMG = rd(th[idx+2])
			iEG = rd(th[idx+3])
			cMG = rd(th[idx+4])
			cEG = rd(th[idx+5])
			phMG = rd(th[idx+6])
			phEG = rd(th[idx+7])
			bMG = rd(th[idx+8])
			bEG = rd(th[idx+9])
			wlMG = rd(th[idx+10])
			wlEG = rd(th[idx+11])
			bwMG = rd(th[idx+12])
			bwEG = rd(th[idx+13])
		}
		idx += 14
		// Mobility MG/EG (7 + 7)
		for i := 0; i < 7 && idx < len(th); i++ {
			mobMG[i] = rd(th[idx])
			idx++
		}
		for i := 0; i < 7 && idx < len(th); i++ {
			mobEG[i] = rd(th[idx])
			idx++
		}
		// King safety table (100)
		for i := 0; i < 100 && idx < len(th); i++ {
			ks[i] = rd(th[idx])
			idx++
		}
		// King correlates (4)
		if idx+4 <= len(th) {
			kSemi = rd(th[idx+0])
			kOpen = rd(th[idx+1])
			kMinor = rd(th[idx+2])
			kPawn = rd(th[idx+3])
		}
		idx += 4
		// King endgame (2) - EG-only weights (currently unused in engine export)
		if idx+2 <= len(th) {
			idx += 2
		} else {
			idx = len(th)
		}
		// Extras (17)
		if idx+17 <= len(th) {
			ex_knOutMG = rd(th[idx+0])
			ex_knOutEG = rd(th[idx+1])
			ex_knThrMG = rd(th[idx+2])
			ex_knThrEG = rd(th[idx+3])
			ex_tropMG = rd(th[idx+4])
			ex_tropEG = rd(th[idx+5])
			ex_stackMG = rd(th[idx+6])
			ex_xrayQMG = rd(th[idx+7])
			ex_xrayKMG = rd(th[idx+8])
			ex_biOutMG = rd(th[idx+9])
			ex_bxKMG = rd(th[idx+10])
			ex_bxRMG = rd(th[idx+11])
			ex_bxQMG = rd(th[idx+12])
			ex_stormMG = rd(th[idx+13])
			ex_proxMG = rd(th[idx+14])
			ex_knMobCenterMG = rd(th[idx+15])
			ex_biMobCenterMG = rd(th[idx+16])
		}
		idx += 17
		// Imbalance (12)
		if idx+12 <= len(th) {
			imbKnPawnMG = rd(th[idx+0])
			imbKnPawnEG = rd(th[idx+1])
			imbBiPawnMG = rd(th[idx+2])
			imbBiPawnEG = rd(th[idx+3])
			imbMinorMajorMG = rd(th[idx+4])
			imbMinorMajorEG = rd(th[idx+5])
			imbRedRookMG = rd(th[idx+6])
			imbRedRookEG = rd(th[idx+7])
			imbRookQueenMG = rd(th[idx+8])
			imbRookQueenEG = rd(th[idx+9])
			imbQueenMinorsMG = rd(th[idx+10])
			imbQueenMinorsEG = rd(th[idx+11])
		}
		idx += 12

		// Phase 6 (5)
		if idx+5 <= len(th) {
			weakMG = rd(th[idx+0])
			weakKingMG = rd(th[idx+1])
			protMG = rd(th[idx+2])
			protKingMG = rd(th[idx+3])
			tempo = rd(th[idx+4])
		}
	} else {
		// Grouped fields path (fallback)
		if m.PST != nil {
			for pt := 0; pt < 6; pt++ {
				for i := 0; i < 64; i++ {
					pstMG[pt][i] = rd(m.PST.MG[pt][i])
					pstEG[pt][i] = rd(m.PST.EG[pt][i])
				}
			}
		}
		if len(m.MaterialMG) == 6 {
			for i := 0; i < 6; i++ {
				matMG[i] = rd(m.MaterialMG[i])
			}
		}
		if len(m.MaterialEG) == 6 {
			for i := 0; i < 6; i++ {
				matEG[i] = rd(m.MaterialEG[i])
			}
		}
		if len(m.PassersMG) == 64 {
			for i := 0; i < 64; i++ {
				passMG[i] = rd(m.PassersMG[i])
			}
		}
		if len(m.PassersEG) == 64 {
			for i := 0; i < 64; i++ {
				passEG[i] = rd(m.PassersEG[i])
			}
		}
		if m.BishopPairMG != nil {
			bpMG = rd(*m.BishopPairMG)
		}
		if m.BishopPairEG != nil {
			bpEG = rd(*m.BishopPairEG)
		}
		if m.RookSemiOpenFileMG != nil {
			rsMG = rd(*m.RookSemiOpenFileMG)
		}
		if m.RookOpenFileMG != nil {
			roMG = rd(*m.RookOpenFileMG)
		}
		if m.SeventhRankEG != nil {
			srEG = rd(*m.SeventhRankEG)
		}
		if m.QueenCentralizedEG != nil {
			qcEG = rd(*m.QueenCentralizedEG)
		}
		if m.QueenInfiltrationMG != nil {
			qiMG = rd(*m.QueenInfiltrationMG)
		}
		if m.QueenInfiltrationEG != nil {
			qiEG = rd(*m.QueenInfiltrationEG)
		}
		if m.DoubledMG != nil {
			dMG = rd(*m.DoubledMG)
		}
		if m.DoubledEG != nil {
			dEG = rd(*m.DoubledEG)
		}
		if m.IsolatedMG != nil {
			iMG = rd(*m.IsolatedMG)
		}
		if m.IsolatedEG != nil {
			iEG = rd(*m.IsolatedEG)
		}
		if m.ConnectedMG != nil {
			cMG = rd(*m.ConnectedMG)
		}
		if m.ConnectedEG != nil {
			cEG = rd(*m.ConnectedEG)
		}
		if m.PhalanxMG != nil {
			phMG = rd(*m.PhalanxMG)
		}
		if m.PhalanxEG != nil {
			phEG = rd(*m.PhalanxEG)
		}
		if m.BlockedMG != nil {
			bMG = rd(*m.BlockedMG)
		}
		if m.BlockedEG != nil {
			bEG = rd(*m.BlockedEG)
		}
		if m.WeakLeverMG != nil {
			wlMG = rd(*m.WeakLeverMG)
		}
		if m.WeakLeverEG != nil {
			wlEG = rd(*m.WeakLeverEG)
		}
		if m.BackwardMG != nil {
			bwMG = rd(*m.BackwardMG)
		}
		if m.BackwardEG != nil {
			bwEG = rd(*m.BackwardEG)
		}
		if len(m.MobilityMG) == 7 {
			for i := 0; i < 7; i++ {
				mobMG[i] = rd(m.MobilityMG[i])
			}
		}
		if len(m.MobilityEG) == 7 {
			for i := 0; i < 7; i++ {
				mobEG[i] = rd(m.MobilityEG[i])
			}
		}
		if len(m.KingSafety) == 100 {
			for i := 0; i < 100; i++ {
				ks[i] = rd(m.KingSafety[i])
			}
		}
		if m.ImbalanceKnightPerPawnMG != nil {
			imbKnPawnMG = rd(*m.ImbalanceKnightPerPawnMG)
		}
		if m.ImbalanceKnightPerPawnEG != nil {
			imbKnPawnEG = rd(*m.ImbalanceKnightPerPawnEG)
		}
		if m.ImbalanceBishopPerPawnMG != nil {
			imbBiPawnMG = rd(*m.ImbalanceBishopPerPawnMG)
		}
		if m.ImbalanceBishopPerPawnEG != nil {
			imbBiPawnEG = rd(*m.ImbalanceBishopPerPawnEG)
		}
		if m.ImbalanceMinorsForMajorMG != nil {
			imbMinorMajorMG = rd(*m.ImbalanceMinorsForMajorMG)
		}
		if m.ImbalanceMinorsForMajorEG != nil {
			imbMinorMajorEG = rd(*m.ImbalanceMinorsForMajorEG)
		}
		if m.ImbalanceRedundantRookMG != nil {
			imbRedRookMG = rd(*m.ImbalanceRedundantRookMG)
		}
		if m.ImbalanceRedundantRookEG != nil {
			imbRedRookEG = rd(*m.ImbalanceRedundantRookEG)
		}
		if m.ImbalanceRookQueenOverlapMG != nil {
			imbRookQueenMG = rd(*m.ImbalanceRookQueenOverlapMG)
		}
		if m.ImbalanceRookQueenOverlapEG != nil {
			imbRookQueenEG = rd(*m.ImbalanceRookQueenOverlapEG)
		}
		if m.ImbalanceQueenManyMinorsMG != nil {
			imbQueenMinorsMG = rd(*m.ImbalanceQueenManyMinorsMG)
		}
		if m.ImbalanceQueenManyMinorsEG != nil {
			imbQueenMinorsEG = rd(*m.ImbalanceQueenManyMinorsEG)
		}
	}

	// Generate output file content in engine order
	var out strings.Builder
	out.WriteString("package engine\n\n")
	out.WriteString("import gm \"chess-engine/goosemg\"\n\n")
	out.WriteString("func init() {\n")
	// PSQT
	out.WriteString("\tPSQT_MG = [7][64]int{\n")
	out.WriteString(formatArray6x64(pstMG))
	out.WriteString("\t}\n")
	out.WriteString("\tPSQT_EG = [7][64]int{\n")
	out.WriteString(formatArray6x64(pstEG))
	out.WriteString("\t}\n")
	// Piece values
	out.WriteString(fmt.Sprintf("\tpieceValueMG = [7]int{gm.PieceTypeKing: 0, gm.PieceTypePawn: %d, gm.PieceTypeKnight: %d, gm.PieceTypeBishop: %d, gm.PieceTypeRook: %d, gm.PieceTypeQueen: %d}\n",
		matMG[0], matMG[1], matMG[2], matMG[3], matMG[4]))
	out.WriteString(fmt.Sprintf("\tpieceValueEG = [7]int{gm.PieceTypeKing: 0, gm.PieceTypePawn: %d, gm.PieceTypeKnight: %d, gm.PieceTypeBishop: %d, gm.PieceTypeRook: %d, gm.PieceTypeQueen: %d}\n",
		matEG[0], matEG[1], matEG[2], matEG[3], matEG[4]))
	// Mobility values
	out.WriteString(fmt.Sprintf("\tmobilityValueMG = [7]int{gm.PieceTypeKing: 0, gm.PieceTypePawn: 0, gm.PieceTypeKnight: %d, gm.PieceTypeBishop: %d, gm.PieceTypeRook: %d, gm.PieceTypeQueen: %d}\n",
		mobMG[2], mobMG[3], mobMG[4], mobMG[5]))
	out.WriteString(fmt.Sprintf("\tmobilityValueEG = [7]int{gm.PieceTypeKing: 0, gm.PieceTypePawn: 0, gm.PieceTypeKnight: %d, gm.PieceTypeBishop: %d, gm.PieceTypeRook: %d, gm.PieceTypeQueen: %d}\n",
		mobEG[2], mobEG[3], mobEG[4], mobEG[5]))
	// Passed pawns
	out.WriteString("\tPassedPawnPSQT_MG = [64]int{\n")
	out.WriteString(formatArray1x64(passMG[:]))
	out.WriteString("\n\t}\n")
	out.WriteString("\tPassedPawnPSQT_EG = [64]int{\n")
	out.WriteString(formatArray1x64(passEG[:]))
	out.WriteString("\n\t}\n")
	// Scalars in evaluation.go order
	out.WriteString(fmt.Sprintf("\tQueenCentralizationEG = %d\n", qcEG))
	out.WriteString(fmt.Sprintf("\tQueenInfiltrationMG = %d\n", qiMG))
	out.WriteString(fmt.Sprintf("\tQueenInfiltrationEG = %d\n", qiEG))
	out.WriteString(fmt.Sprintf("\tRookXrayQueenMG = %d\n", ex_xrayQMG))
	out.WriteString(fmt.Sprintf("\tRookXrayKingMG = %d\n", ex_xrayKMG))
	out.WriteString(fmt.Sprintf("\tRookStackedMG = %d\n", ex_stackMG))
	out.WriteString(fmt.Sprintf("\tRookSeventhRankEG = %d\n", srEG))
	out.WriteString(fmt.Sprintf("\tRookSemiOpenMG = %d\n", rsMG))
	out.WriteString(fmt.Sprintf("\tRookOpenMG = %d\n", roMG))
	out.WriteString(fmt.Sprintf("\tKnightOutpostMG = %d\n", ex_knOutMG))
	out.WriteString(fmt.Sprintf("\tKnightOutpostEG = %d\n", ex_knOutEG))
	out.WriteString(fmt.Sprintf("\tKnightThreatMG = %d\n", ex_knThrMG))
	out.WriteString(fmt.Sprintf("\tKnightThreatEG = %d\n", ex_knThrEG))
	out.WriteString(fmt.Sprintf("\tBishopXrayRookMG = %d\n", ex_bxRMG))
	out.WriteString(fmt.Sprintf("\tBishopXrayQueenMG = %d\n", ex_bxQMG))
	out.WriteString(fmt.Sprintf("\tBishopXrayKingMG = %d\n", ex_bxKMG))
	out.WriteString(fmt.Sprintf("\tBishopOutpostMG = %d\n", ex_biOutMG))
	out.WriteString(fmt.Sprintf("\tBishopPairBonusMG = %d\n", bpMG))
	out.WriteString(fmt.Sprintf("\tBishopPairBonusEG = %d\n", bpEG))
	out.WriteString(fmt.Sprintf("\tBackwardPawnMG = %d\n", bwMG))
	out.WriteString(fmt.Sprintf("\tBackwardPawnEG = %d\n", bwEG))
	out.WriteString(fmt.Sprintf("\tIsolatedPawnMG = %d\n", iMG))
	out.WriteString(fmt.Sprintf("\tIsolatedPawnEG = %d\n", iEG))
	out.WriteString(fmt.Sprintf("\tPawnDoubledMG = %d\n", dMG))
	out.WriteString(fmt.Sprintf("\tPawnDoubledEG = %d\n", dEG))
	out.WriteString(fmt.Sprintf("\tPawnStormMG = %d\n", ex_stormMG))
	out.WriteString(fmt.Sprintf("\tPawnFrontProximityMG = %d\n", ex_proxMG))
	out.WriteString(fmt.Sprintf("\tPawnConnectedMG = %d\n", cMG))
	out.WriteString(fmt.Sprintf("\tPawnConnectedEG = %d\n", cEG))
	out.WriteString(fmt.Sprintf("\tPawnPhalanxMG = %d\n", phMG))
	out.WriteString(fmt.Sprintf("\tPawnPhalanxEG = %d\n", phEG))
	out.WriteString(fmt.Sprintf("\tPawnWeakLeverMG = %d\n", wlMG))
	out.WriteString(fmt.Sprintf("\tPawnWeakLeverEG = %d\n", wlEG))
	out.WriteString(fmt.Sprintf("\tPawnBlockedMG = %d\n", bMG))
	out.WriteString(fmt.Sprintf("\tPawnBlockedEG = %d\n", bEG))
	out.WriteString(fmt.Sprintf("\tKingOpenFileMG = %d\n", kOpen))
	out.WriteString(fmt.Sprintf("\tKingSemiOpenFileMG = %d\n", kSemi))
	out.WriteString(fmt.Sprintf("\tKingMinorDefenseBonusMG = %d\n", kMinor))
	out.WriteString(fmt.Sprintf("\tKingPawnDefenseBonusMG = %d\n", kPawn))
	out.WriteString(fmt.Sprintf("\tKnightTropismMG = %d\n", ex_tropMG))
	out.WriteString(fmt.Sprintf("\tKnightTropismEG = %d\n", ex_tropEG))
	out.WriteString(fmt.Sprintf("\tWeakSquarePenaltyMG = %d\n", weakMG))
	out.WriteString(fmt.Sprintf("\tWeakSquarePenaltyEG = %d\n", weakKingMG))
	out.WriteString(fmt.Sprintf("\tProtectedSquareBonusMG = %d\n", protMG))
	out.WriteString(fmt.Sprintf("\tProtectedSquareBonusEG = %d\n", protKingMG))
	out.WriteString(fmt.Sprintf("\tTempoBonus = %d\n", tempo))
	// King safety table
	out.WriteString("\tKingSafetyTable = [100]int{\n")
	out.WriteString(formatArray100(ks[:]))
	out.WriteString("\t}\n")
	// Material imbalance scalars
	out.WriteString(fmt.Sprintf("\tImbalanceKnightPerPawnMG = %d\n", imbKnPawnMG))
	out.WriteString(fmt.Sprintf("\tImbalanceKnightPerPawnEG = %d\n", imbKnPawnEG))
	out.WriteString(fmt.Sprintf("\tImbalanceBishopPerPawnMG = %d\n", imbBiPawnMG))
	out.WriteString(fmt.Sprintf("\tImbalanceBishopPerPawnEG = %d\n", imbBiPawnEG))
	out.WriteString(fmt.Sprintf("\tImbalanceMinorsForMajorMG = %d\n", imbMinorMajorMG))
	out.WriteString(fmt.Sprintf("\tImbalanceMinorsForMajorEG = %d\n", imbMinorMajorEG))
	out.WriteString(fmt.Sprintf("\tImbalanceRedundantRookMG = %d\n", imbRedRookMG))
	out.WriteString(fmt.Sprintf("\tImbalanceRedundantRookEG = %d\n", imbRedRookEG))
	out.WriteString(fmt.Sprintf("\tImbalanceRookQueenOverlapMG = %d\n", imbRookQueenMG))
	out.WriteString(fmt.Sprintf("\tImbalanceRookQueenOverlapEG = %d\n", imbRookQueenEG))
	out.WriteString(fmt.Sprintf("\tImbalanceQueenManyMinorsMG = %d\n", imbQueenMinorsMG))
	out.WriteString(fmt.Sprintf("\tImbalanceQueenManyMinorsEG = %d\n", imbQueenMinorsEG))
	out.WriteString("}\n")

	if err := os.MkdirAll(filepath.Dir(*outPath), 0o755); err != nil {
		panic(err)
	}
	if err := os.WriteFile(*outPath, []byte(out.String()), 0o644); err != nil {
		panic(err)
	}
}
