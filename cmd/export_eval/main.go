package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
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
	BishopPairMG       *float64 `json:"bishop_pair_mg,omitempty"`
	BishopPairEG       *float64 `json:"bishop_pair_eg,omitempty"`
	RookSemiOpenFileMG *float64 `json:"rook_semi_open_mg,omitempty"`
	RookOpenFileMG     *float64 `json:"rook_open_mg,omitempty"`
	SeventhRankEG      *float64 `json:"seventh_rank_eg,omitempty"`
	QueenCentralizedEG *float64 `json:"queen_centralized_eg,omitempty"`
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
	// Phase 3 (legacy mobility scalars)
	MobilityMG []float64 `json:"mobility_mg,omitempty"`
	MobilityEG []float64 `json:"mobility_eg,omitempty"`
	// Mobility tables
	KnightMobilityMG []float64 `json:"knight_mobility_mg,omitempty"`
	KnightMobilityEG []float64 `json:"knight_mobility_eg,omitempty"`
	BishopMobilityMG []float64 `json:"bishop_mobility_mg,omitempty"`
	BishopMobilityEG []float64 `json:"bishop_mobility_eg,omitempty"`
	RookMobilityMG   []float64 `json:"rook_mobility_mg,omitempty"`
	RookMobilityEG   []float64 `json:"rook_mobility_eg,omitempty"`
	QueenMobilityMG  []float64 `json:"queen_mobility_mg,omitempty"`
	QueenMobilityEG  []float64 `json:"queen_mobility_eg,omitempty"`
	// Phase 4
	KingSafety []float64 `json:"king_safety_table,omitempty"`
	// Fixed-feature scalars (not in theta)
	CandidatePassedPctMG *float64 `json:"candidate_passed_pct_mg,omitempty"`
	CandidatePassedPctEG *float64 `json:"candidate_passed_pct_eg,omitempty"`
	BadBishopMG          *float64 `json:"bad_bishop_mg,omitempty"`
	BadBishopEG          *float64 `json:"bad_bishop_eg,omitempty"`
	KingPasserProxEG     *float64 `json:"king_passer_proximity_eg,omitempty"`
	KingPasserProxDiv    *float64 `json:"king_passer_proximity_div,omitempty"`
	KingPasserEnemyW     *float64 `json:"king_passer_enemy_weight,omitempty"`
	KingPasserOwnW       *float64 `json:"king_passer_own_weight,omitempty"`
	// Imbalance scalars (optional grouped fields)
	ImbalanceKnightPerPawnMG *float64 `json:"imbalance_knight_per_pawn_mg,omitempty"`
	ImbalanceKnightPerPawnEG *float64 `json:"imbalance_knight_per_pawn_eg,omitempty"`
	ImbalanceBishopPerPawnMG *float64 `json:"imbalance_bishop_per_pawn_mg,omitempty"`
	ImbalanceBishopPerPawnEG *float64 `json:"imbalance_bishop_per_pawn_eg,omitempty"`
}

// Theta layout (length 1217)
// 0..383: PST MG (6x64)
// 384..767: PST EG (6x64)
// 768..773: Material MG (6)
// 774..779: Material EG (6)
// 780..839: Mobility MG tables (60)
// 840..899: Mobility EG tables (60)
// 900..903: Core scalars (4)
// 904..912: Tier1 extras (9)
// 913..976: Passers MG (64)
// 977..1040: Passers EG (64)
// 1041..1056: PawnStruct (16)
// 1057..1156: KingSafetyTable (100)
// 1157..1160: King-safety correlates (4)
// 1161..1162: King endgame terms (2) (not exported)
// 1163..1206: Tier3 extras (44)
// 1207..1207: WeakKingSquares (1)
// 1208..1209: BishopPair (2)
// 1210..1213: Material imbalance scalars (4)
// 1214..1216: Space/Tempo (3)

func rd(x float64) int { return int(math.Round(x)) }

// formatPSQT6x64 formats a 6x64 PSQT array matching evaluation.go style
func formatPSQT6x64(vals [6][64]int) string {
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
		b.WriteString("\t" + pieceOrder[pi] + ": {\n")
		for i := 0; i < 64; i++ {
			if i%8 == 0 {
				b.WriteString("\t\t")
			}
			b.WriteString(fmt.Sprintf("%d", vals[pi][i]))
			if i%8 == 7 {
				b.WriteString(",\n")
			} else {
				b.WriteString(", ")
			}
		}
		b.WriteString("\t},\n")
	}
	return b.String()
}

// formatArray64 formats a [64]int array matching evaluation.go style (8 per row)
func formatArray64(vals [64]int) string {
	var b strings.Builder
	for i := 0; i < 64; i++ {
		if i%8 == 0 {
			b.WriteString("\t")
		}
		b.WriteString(fmt.Sprintf("%d", vals[i]))
		if i%8 == 7 {
			b.WriteString(",\n")
		} else {
			b.WriteString(", ")
		}
	}
	return b.String()
}

// formatArray100 formats a [100]int array matching evaluation.go style (10 per row)
func formatArray100(vals [100]int) string {
	var b strings.Builder
	for i := 0; i < 100; i++ {
		if i%10 == 0 {
			b.WriteString("\t")
		}
		b.WriteString(fmt.Sprintf("%d", vals[i]))
		if i%10 == 9 {
			b.WriteString(",\n")
		} else {
			b.WriteString(", ")
		}
	}
	return b.String()
}

// formatArrayInline formats a slice of ints on a single line
func formatArrayInline(vals []int) string {
	parts := make([]string, len(vals))
	for i, v := range vals {
		parts[i] = fmt.Sprintf("%d", v)
	}
	return strings.Join(parts, ", ")
}

// formatArray8Inline formats a [8]int array on a single line
func formatArray8Inline(vals [8]int) string {
	var parts []string
	for i := 0; i < 8; i++ {
		parts = append(parts, fmt.Sprintf("%d", vals[i]))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func main() {
	inPath := flag.String("in", "model.json", "input model JSON path")
	outPath := flag.String("out", "engine/evaluation_gen.go", "output path for generated evaluation_gen.go")
	tierOnly := flag.Int("tier", 0, "optional tier filter (0=all; 1=Core; 2=Pawns; 3=KingSafety; 4=Misc)")
	flag.Parse()

	// Allow positional overrides after flags (flag.Parse stops at first non-flag).
	args := flag.Args()
	lastOut := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-tier":
			if i+1 < len(args) {
				if v, err := strconv.Atoi(args[i+1]); err == nil {
					*tierOnly = v
				}
				i++
			}
		default:
			if !strings.HasPrefix(args[i], "-") {
				lastOut = args[i]
			}
		}
	}
	if lastOut != "" {
		*outPath = lastOut
	}

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
	var bpMG, bpEG, rsMG, roMG, srEG, qcEG int
	// Phase 2
	var dMG, dEG, iMG, iEG, cMG, cEG, phMG, phEG, bMG, bEG, wlMG, wlEG, bwMG, bwEG int
	// Mobility tables
	var knMobMG [9]int
	var knMobEG [9]int
	var biMobMG [14]int
	var biMobEG [14]int
	var rMobMG [15]int
	var rMobEG [15]int
	var qMobMG [22]int
	var qMobEG [22]int
	// King safety table + correlates
	var ks [100]int
	var kSemi, kOpen, kMinor, kPawn int
	// Fixed-feature scalars
	candPctMG, candPctEG := 50, 65
	badBishopMG, badBishopEG := -4, -7
	kingPasserProxEG, kingPasserProxDiv := 1, 10
	kingPasserEnemyW, kingPasserOwnW := 5, 2
	// Extras
	var ex_knOutMG, ex_knOutEG int
	var ex_tropMG, ex_tropEG int
	var ex_stackMG, ex_biOutMG, ex_biOutEG int
	var ps_freePct, ps_leverPct, ps_weakLeverPct, ps_blockedPct [8]int
	var ps_baseMG [8]int
	var ps_oppositeMult int
	// Imbalance scalars
	var imbKnPawnMG, imbKnPawnEG, imbBiPawnMG, imbBiPawnEG int
	// Phase 7
	var spaceMG, spaceEG, weakKingMG, tempo int

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
		// Mobility MG tables (60)
		for i := 0; i < len(knMobMG) && idx < len(th); i++ {
			knMobMG[i] = rd(th[idx])
			idx++
		}
		for i := 0; i < len(biMobMG) && idx < len(th); i++ {
			biMobMG[i] = rd(th[idx])
			idx++
		}
		for i := 0; i < len(rMobMG) && idx < len(th); i++ {
			rMobMG[i] = rd(th[idx])
			idx++
		}
		for i := 0; i < len(qMobMG) && idx < len(th); i++ {
			qMobMG[i] = rd(th[idx])
			idx++
		}
		// Mobility EG tables (60)
		for i := 0; i < len(knMobEG) && idx < len(th); i++ {
			knMobEG[i] = rd(th[idx])
			idx++
		}
		for i := 0; i < len(biMobEG) && idx < len(th); i++ {
			biMobEG[i] = rd(th[idx])
			idx++
		}
		for i := 0; i < len(rMobEG) && idx < len(th); i++ {
			rMobEG[i] = rd(th[idx])
			idx++
		}
		for i := 0; i < len(qMobEG) && idx < len(th); i++ {
			qMobEG[i] = rd(th[idx])
			idx++
		}
		// Core scalars (4)
		if idx+4 <= len(th) {
			rsMG = rd(th[idx+0])
			roMG = rd(th[idx+1])
			srEG = rd(th[idx+2])
			qcEG = rd(th[idx+3])
		}
		idx += 4

		// Tier1 extras (9)
		if idx+9 <= len(th) {
			ex_knOutMG = rd(th[idx+0])
			ex_knOutEG = rd(th[idx+1])
			ex_biOutMG = rd(th[idx+2])
			ex_biOutEG = rd(th[idx+3])
			ex_stackMG = rd(th[idx+4])
			// idx+5/6: Knight/Bishop mob center (not exported)
			badBishopMG = rd(th[idx+7])
			badBishopEG = rd(th[idx+8])
		}
		idx += 9

		// Passers MG/EG
		for i := 0; i < 64 && idx < len(th); i++ {
			passMG[i] = rd(th[idx])
			idx++
		}
		for i := 0; i < 64 && idx < len(th); i++ {
			passEG[i] = rd(th[idx])
			idx++
		}

		// PawnStruct (16)
		if idx+16 <= len(th) {
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
			candPctMG = rd(th[idx+14])
			candPctEG = rd(th[idx+15])
		}
		idx += 16

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

		// Tier3 extras (44)
		if idx+44 <= len(th) {
			ex_tropMG = rd(th[idx+0])
			ex_tropEG = rd(th[idx+1])
			for i := 0; i < 8; i++ {
				ps_freePct[i] = rd(th[idx+2+i])
				ps_leverPct[i] = rd(th[idx+10+i])
				ps_weakLeverPct[i] = rd(th[idx+18+i])
				ps_blockedPct[i] = rd(th[idx+26+i])
			}
			ps_oppositeMult = rd(th[idx+34])
			for i := 0; i < 8; i++ {
				ps_baseMG[i] = rd(th[idx+36+i])
			}
		}
		idx += 44

		// Weak king (1)
		if idx+1 <= len(th) {
			weakKingMG = rd(th[idx])
		}
		idx += 1

		// Bishop pair (2)
		if idx+2 <= len(th) {
			bpMG = rd(th[idx+0])
			bpEG = rd(th[idx+1])
		}
		idx += 2

		// Imbalance (4)
		if idx+4 <= len(th) {
			imbKnPawnMG = rd(th[idx+0])
			imbKnPawnEG = rd(th[idx+1])
			imbBiPawnMG = rd(th[idx+2])
			imbBiPawnEG = rd(th[idx+3])
		}
		idx += 4

		// Space/Tempo (3)
		if idx+3 <= len(th) {
			spaceMG = rd(th[idx+0]) // SpaceMG
			spaceEG = rd(th[idx+1]) // SpaceEG
			tempo = rd(th[idx+2])   // Tempo
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
		if len(m.KnightMobilityMG) == len(knMobMG) {
			for i := 0; i < len(knMobMG); i++ {
				knMobMG[i] = rd(m.KnightMobilityMG[i])
			}
		}
		if len(m.KnightMobilityEG) == len(knMobEG) {
			for i := 0; i < len(knMobEG); i++ {
				knMobEG[i] = rd(m.KnightMobilityEG[i])
			}
		}
		if len(m.BishopMobilityMG) == len(biMobMG) {
			for i := 0; i < len(biMobMG); i++ {
				biMobMG[i] = rd(m.BishopMobilityMG[i])
			}
		}
		if len(m.BishopMobilityEG) == len(biMobEG) {
			for i := 0; i < len(biMobEG); i++ {
				biMobEG[i] = rd(m.BishopMobilityEG[i])
			}
		}
		if len(m.RookMobilityMG) == len(rMobMG) {
			for i := 0; i < len(rMobMG); i++ {
				rMobMG[i] = rd(m.RookMobilityMG[i])
			}
		}
		if len(m.RookMobilityEG) == len(rMobEG) {
			for i := 0; i < len(rMobEG); i++ {
				rMobEG[i] = rd(m.RookMobilityEG[i])
			}
		}
		if len(m.QueenMobilityMG) == len(qMobMG) {
			for i := 0; i < len(qMobMG); i++ {
				qMobMG[i] = rd(m.QueenMobilityMG[i])
			}
		}
		if len(m.QueenMobilityEG) == len(qMobEG) {
			for i := 0; i < len(qMobEG); i++ {
				qMobEG[i] = rd(m.QueenMobilityEG[i])
			}
		}
		if len(m.KingSafety) == 100 {
			for i := 0; i < 100; i++ {
				ks[i] = rd(m.KingSafety[i])
			}
		}
		if m.CandidatePassedPctMG != nil {
			candPctMG = rd(*m.CandidatePassedPctMG)
		}
		if m.CandidatePassedPctEG != nil {
			candPctEG = rd(*m.CandidatePassedPctEG)
		}
		if m.BadBishopMG != nil {
			badBishopMG = rd(*m.BadBishopMG)
		}
		if m.BadBishopEG != nil {
			badBishopEG = rd(*m.BadBishopEG)
		}
		if m.KingPasserProxEG != nil {
			kingPasserProxEG = rd(*m.KingPasserProxEG)
		}
		if m.KingPasserProxDiv != nil {
			kingPasserProxDiv = rd(*m.KingPasserProxDiv)
		}
		if m.KingPasserEnemyW != nil {
			kingPasserEnemyW = rd(*m.KingPasserEnemyW)
		}
		if m.KingPasserOwnW != nil {
			kingPasserOwnW = rd(*m.KingPasserOwnW)
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
	}

	// Generate output file content matching evaluation.go format exactly
	var out strings.Builder
	out.WriteString("package engine\n\n")
	out.WriteString("import gm \"chess-engine/goosemg\"\n\n")

	tierAll := *tierOnly == 0
	tier1 := tierAll || *tierOnly == 1
	tier2 := tierAll || *tierOnly == 2
	tier3 := tierAll || *tierOnly == 3
	tier4 := tierAll || *tierOnly == 4

	// PSQT_MG (Tier 1)
	if tier1 {
		out.WriteString("var PSQT_MG = [7][64]int{\n")
		out.WriteString(formatPSQT6x64(pstMG))
		out.WriteString("}\n")

		// PSQT_EG
		out.WriteString("var PSQT_EG = [7][64]int{\n")
		out.WriteString(formatPSQT6x64(pstEG))
		out.WriteString("}\n")

		// pieceValueMG/EG (single line format)
		out.WriteString(fmt.Sprintf("var pieceValueMG = [7]int{gm.PieceTypeKing: 0, gm.PieceTypePawn: %d, gm.PieceTypeKnight: %d, gm.PieceTypeBishop: %d, gm.PieceTypeRook: %d, gm.PieceTypeQueen: %d}\n",
			matMG[0], matMG[1], matMG[2], matMG[3], matMG[4]))
		out.WriteString(fmt.Sprintf("var pieceValueEG = [7]int{gm.PieceTypeKing: 0, gm.PieceTypePawn: %d, gm.PieceTypeKnight: %d, gm.PieceTypeBishop: %d, gm.PieceTypeRook: %d, gm.PieceTypeQueen: %d}\n",
			matEG[0], matEG[1], matEG[2], matEG[3], matEG[4]))

		// Mobility tables
		out.WriteString(fmt.Sprintf("var KnightMobilityMG = [9]int{%s}\n", formatArrayInline(knMobMG[:])))
		out.WriteString(fmt.Sprintf("var KnightMobilityEG = [9]int{%s}\n", formatArrayInline(knMobEG[:])))
		out.WriteString(fmt.Sprintf("var BishopMobilityMG = [14]int{%s}\n", formatArrayInline(biMobMG[:])))
		out.WriteString(fmt.Sprintf("var BishopMobilityEG = [14]int{%s}\n", formatArrayInline(biMobEG[:])))
		out.WriteString(fmt.Sprintf("var RookMobilityMG = [15]int{%s}\n", formatArrayInline(rMobMG[:])))
		out.WriteString(fmt.Sprintf("var RookMobilityEG = [15]int{%s}\n", formatArrayInline(rMobEG[:])))
		out.WriteString(fmt.Sprintf("var QueenMobilityMG = [22]int{%s}\n", formatArrayInline(qMobMG[:])))
		out.WriteString(fmt.Sprintf("var QueenMobilityEG = [22]int{%s}\n", formatArrayInline(qMobEG[:])))
	}

	// PassedPawnPSQT (Tier 2)
	if tier2 {
		out.WriteString("var PassedPawnPSQT_MG = [64]int{\n")
		out.WriteString(formatArray64(passMG))
		out.WriteString("}\n")

		out.WriteString("var PassedPawnPSQT_EG = [64]int{\n")
		out.WriteString(formatArray64(passEG))
		out.WriteString("}\n")
	}

	// Scalar parameters in var() block - matching evaluation.go order
	out.WriteString("\nvar (\n")

	// Pawn structure (Tier 2)
	if tier2 {
		out.WriteString(fmt.Sprintf("\tBackwardPawnMG  = %d\n", bwMG))
		out.WriteString(fmt.Sprintf("\tBackwardPawnEG  = %d\n", bwEG))
		out.WriteString(fmt.Sprintf("\tIsolatedPawnMG  = %d\n", iMG))
		out.WriteString(fmt.Sprintf("\tIsolatedPawnEG  = %d\n", iEG))
		out.WriteString(fmt.Sprintf("\tPawnDoubledMG   = %d\n", dMG))
		out.WriteString(fmt.Sprintf("\tPawnDoubledEG   = %d\n", dEG))
		out.WriteString(fmt.Sprintf("\tPawnConnectedMG = %d\n", cMG))
		out.WriteString(fmt.Sprintf("\tPawnConnectedEG = %d\n", cEG))
		out.WriteString(fmt.Sprintf("\tPawnPhalanxMG   = %d\n", phMG))
		out.WriteString(fmt.Sprintf("\tPawnPhalanxEG   = %d\n", phEG))
		out.WriteString(fmt.Sprintf("\tPawnWeakLeverMG = %d\n", wlMG))
		out.WriteString(fmt.Sprintf("\tPawnWeakLeverEG = %d\n", wlEG))
		out.WriteString(fmt.Sprintf("\tPawnBlockedMG   = %d\n", bMG))
		out.WriteString(fmt.Sprintf("\tPawnBlockedEG   = %d\n", bEG))
		out.WriteString(fmt.Sprintf("\tCandidatePassedPctMG = %d\n", candPctMG))
		out.WriteString(fmt.Sprintf("\tCandidatePassedPctEG = %d\n", candPctEG))
		out.WriteString("\n")
	}

	// Knight (Tier 1 for outposts, Tier 3 for tropism)
	if tier1 {
		out.WriteString(fmt.Sprintf("\tKnightOutpostMG = %d\n", ex_knOutMG))
		out.WriteString(fmt.Sprintf("\tKnightOutpostEG = %d\n", ex_knOutEG))
	}
	if tier3 {
		out.WriteString(fmt.Sprintf("\tKnightTropismMG = %d\n", ex_tropMG))
		out.WriteString(fmt.Sprintf("\tKnightTropismEG = %d\n", ex_tropEG))
	}
	if tier1 || tier3 {
		out.WriteString("\n")
	}

	// Bishop (Tier 1 for outposts, Tier 4 for pair)
	if tier1 {
		out.WriteString(fmt.Sprintf("\tBishopOutpostMG = %d\n", ex_biOutMG))
		out.WriteString(fmt.Sprintf("\tBishopOutpostEG = %d\n", ex_biOutEG))
		out.WriteString(fmt.Sprintf("\tBadBishopMG     = %d\n", badBishopMG))
		out.WriteString(fmt.Sprintf("\tBadBishopEG     = %d\n", badBishopEG))
		out.WriteString("\n")
	}
	if tier4 {
		out.WriteString(fmt.Sprintf("\tBishopPairBonusMG = %d\n", bpMG))
		out.WriteString(fmt.Sprintf("\tBishopPairBonusEG = %d\n", bpEG))
		out.WriteString("\n")
	}

	// Rook (Tier 1)
	if tier1 {
		out.WriteString(fmt.Sprintf("\tRookStackedMG     = %d\n", ex_stackMG))
		out.WriteString(fmt.Sprintf("\tRookSeventhRankEG = %d\n", srEG))
		out.WriteString(fmt.Sprintf("\tRookSemiOpenMG    = %d\n", rsMG))
		out.WriteString(fmt.Sprintf("\tRookOpenMG        = %d\n", roMG))
		out.WriteString("\n")
	}

	// Queen (Tier 1)
	if tier1 {
		out.WriteString(fmt.Sprintf("\tQueenCentralizationEG = %d\n", qcEG))
		out.WriteString("\n")
	}

	// King file/defense scalars (Tier 3)
	if tier3 {
		out.WriteString(fmt.Sprintf("\tKingOpenFileMG          = %d\n", kOpen))
		out.WriteString(fmt.Sprintf("\tKingSemiOpenFileMG      = %d\n", kSemi))
		out.WriteString(fmt.Sprintf("\tKingMinorDefenseBonusMG = %d\n", kMinor))
		out.WriteString(fmt.Sprintf("\tKingPawnDefenseBonusMG  = %d\n", kPawn))
		out.WriteString(fmt.Sprintf("\tKingPasserProximityEG   = %d\n", kingPasserProxEG))
		out.WriteString(fmt.Sprintf("\tKingPasserProximityDiv  = %d\n", kingPasserProxDiv))
		out.WriteString(fmt.Sprintf("\tKingPasserEnemyWeight   = %d\n", kingPasserEnemyW))
		out.WriteString(fmt.Sprintf("\tKingPasserOwnWeight     = %d\n", kingPasserOwnW))
		out.WriteString("\n")
	}

	// Space/weak-king (Tier 4 for space, Tier 3 for weak king)
	if tier4 {
		out.WriteString(fmt.Sprintf("\tSpaceBonusMG            = %d\n", spaceMG))
		out.WriteString(fmt.Sprintf("\tSpaceBonusEG            = %d\n", spaceEG))
	}
	if tier3 {
		out.WriteString(fmt.Sprintf("\tWeakKingSquarePenaltyMG = %d\n", weakKingMG))
	}
	if tier3 || tier4 {
		out.WriteString("\n")
	}

	// Pawn storm (Tier 3)
	if tier3 {
		out.WriteString(fmt.Sprintf("\tPawnStormBaseMG             = [8]int%s\n", formatArray8Inline(ps_baseMG)))
		out.WriteString(fmt.Sprintf("\tPawnStormFreePct            = [8]int%s\n", formatArray8Inline(ps_freePct)))
		out.WriteString(fmt.Sprintf("\tPawnStormLeverPct           = [8]int%s\n", formatArray8Inline(ps_leverPct)))
		out.WriteString(fmt.Sprintf("\tPawnStormWeakLeverPct       = [8]int%s\n", formatArray8Inline(ps_weakLeverPct)))
		out.WriteString(fmt.Sprintf("\tPawnStormBlockedPct         = [8]int%s\n", formatArray8Inline(ps_blockedPct)))
		out.WriteString(fmt.Sprintf("\tPawnStormOppositeMultiplier = %d\n", ps_oppositeMult))
		out.WriteString("\n")
	}

	// Tempo (Tier 4)
	var drawDvd = 8
	if tier4 {
		out.WriteString(fmt.Sprintf("\tTempoBonus = %d\n", tempo))
		out.WriteString(fmt.Sprintf("\tDrawDivider int32 = %d\n", drawDvd))
		out.WriteString("\n")
	}

	out.WriteString(")\n\n")

	// King safety table (Tier 3)
	if tier3 {
		out.WriteString("var KingSafetyTable = [100]int{\n")
		out.WriteString(formatArray100(ks))
		out.WriteString("}\n\n")
	}

	// Material imbalance (Tier 4)
	var imbPwnCnt int = 5
	if tier4 {
		out.WriteString(fmt.Sprintf("var ImbalanceRefPawnCount = %d\n", imbPwnCnt))
		out.WriteString(fmt.Sprintf("var ImbalanceKnightPerPawnMG = %d\n", imbKnPawnMG))
		out.WriteString(fmt.Sprintf("var ImbalanceKnightPerPawnEG = %d\n", imbKnPawnEG))
		out.WriteString(fmt.Sprintf("var ImbalanceBishopPerPawnMG = %d\n", imbBiPawnMG))
		out.WriteString(fmt.Sprintf("var ImbalanceBishopPerPawnEG = %d\n", imbBiPawnEG))
	}

	if err := os.MkdirAll(filepath.Dir(*outPath), 0o755); err != nil {
		panic(err)
	}
	if err := os.WriteFile(*outPath, []byte(out.String()), 0o644); err != nil {
		panic(err)
	}
}
