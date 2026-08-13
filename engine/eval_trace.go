package engine

import (
	"encoding/json"
	"fmt"
	"io"
	"math/bits"

	gm "chess-engine/goosemg"
)

// EvalPair stores middlegame/endgame scores before tapered interpolation.
type EvalPair struct {
	MG int `json:"mg"`
	EG int `json:"eg"`
}

// EvalBitboard stores a bitboard in both machine-friendly and human-friendly forms.
type EvalBitboard struct {
	Hex     string   `json:"hex"`
	Count   int      `json:"count"`
	Squares []string `json:"squares"`
}

// EvalSideBitboards stores white/black bitboards for one feature.
type EvalSideBitboards struct {
	White EvalBitboard `json:"white"`
	Black EvalBitboard `json:"black"`
}

// EvalScoreTrace describes score orientation and draw adjustment.
type EvalScoreTrace struct {
	WhitePerspectiveBeforeDraw int32 `json:"whitePerspectiveBeforeDraw"`
	WhitePerspective           int32 `json:"whitePerspective"`
	SideToMove                 int32 `json:"sideToMove"`
}

// EvalPhaseTrace stores tapered-eval weights.
type EvalPhaseTrace struct {
	PiecePhase int `json:"piecePhase"`
	MGWeight   int `json:"mgWeight"`
	EGWeight   int `json:"egWeight"`
	TotalPhase int `json:"totalPhase"`
}

// EvalDrawTrace stores draw-scaling state.
type EvalDrawTrace struct {
	Theoretical bool  `json:"theoretical"`
	Divider     int32 `json:"divider"`
	Applied     bool  `json:"applied"`
}

// EvalTotalsTrace stores the final MG/EG buckets.
type EvalTotalsTrace struct {
	Material EvalPair `json:"material"`
	Variable EvalPair `json:"variable"`
	Score    EvalPair `json:"score"`
}

// EvalPawnTrace stores pawn-structure terms and pawn bitboards.
type EvalPawnTrace struct {
	Terms              map[string]EvalPair          `json:"terms"`
	Bitboards          map[string]EvalSideBitboards `json:"bitboards"`
	HashTotal          EvalPair                     `json:"hashTotal"`
	Total              EvalPair                     `json:"total"`
	CandidatePassedPct EvalPair                     `json:"candidatePassedPct"`
}

// EvalPieceTrace stores generic piece terms.
type EvalPieceTrace struct {
	Terms          map[string]EvalPair          `json:"terms"`
	Bitboards      map[string]EvalSideBitboards `json:"bitboards,omitempty"`
	MobilityCounts []int                        `json:"mobilityCounts,omitempty"`
	Total          EvalPair                     `json:"total"`
}

// EvalKingTrace stores king-specific terms and king attack diagnostics.
type EvalKingTrace struct {
	Terms                   map[string]EvalPair          `json:"terms"`
	Bitboards               map[string]EvalSideBitboards `json:"bitboards"`
	AttackersOnBlackKing    int                          `json:"attackersOnBlackKing"`
	AttackersOnWhiteKing    int                          `json:"attackersOnWhiteKing"`
	SafeChecksOnBlackKing   int                          `json:"safeChecksOnBlackKing"`
	SafeChecksOnWhiteKing   int                          `json:"safeChecksOnWhiteKing"`
	UnsafeChecksOnBlackKing int                          `json:"unsafeChecksOnBlackKing"`
	UnsafeChecksOnWhiteKing int                          `json:"unsafeChecksOnWhiteKing"`
	AttackUnitsOnBlackKing  int                          `json:"attackUnitsOnBlackKing"`
	AttackUnitsOnWhiteKing  int                          `json:"attackUnitsOnWhiteKing"`
	DangerToBlackKing       int                          `json:"dangerToBlackKing"`
	DangerToWhiteKing       int                          `json:"dangerToWhiteKing"`
	Total                   EvalPair                     `json:"total"`
}

// EvalTrace is a stable, parseable snapshot of one static evaluation.
type EvalTrace struct {
	FEN        string          `json:"fen"`
	SideToMove string          `json:"sideToMove"`
	Score      EvalScoreTrace  `json:"score"`
	Phase      EvalPhaseTrace  `json:"phase"`
	Draw       EvalDrawTrace   `json:"draw"`
	Totals     EvalTotalsTrace `json:"totals"`
	Material   EvalPair        `json:"material"`
	Imbalance  EvalPair        `json:"imbalance"`
	Tempo      int             `json:"tempo"`
	Space      EvalPair        `json:"space"`
	Center     EvalCenterTrace `json:"center"`
	Pawn       EvalPawnTrace   `json:"pawn"`
	Knight     EvalPieceTrace  `json:"knight"`
	Bishop     EvalPieceTrace  `json:"bishop"`
	Rook       EvalPieceTrace  `json:"rook"`
	Queen      EvalPieceTrace  `json:"queen"`
	King       EvalKingTrace   `json:"king"`
}

// EvalCenterTrace stores center-state scaling inputs.
type EvalCenterTrace struct {
	Locked                bool    `json:"locked"`
	Openness              float64 `json:"openness"`
	KnightMobilityScale   int     `json:"knightMobilityScale"`
	KnightMobilityScaleEG int     `json:"knightMobilityScaleEg"`
	BishopMobilityScale   int     `json:"bishopMobilityScale"`
	BishopMobilityScaleEG int     `json:"bishopMobilityScaleEg"`
	BishopPairScaleMG     int     `json:"bishopPairScaleMg"`
	BishopPairScaleEG     int     `json:"bishopPairScaleEg"`
}

// EvalTraceForBoard returns the structured evaluation trace for b.
func EvalTraceForBoard(b *gm.Board) EvalTrace {
	_, trace := EvaluateWithTrace(b)
	return trace
}

// EvaluateWithTrace evaluates b and returns both the side-to-move score and
// a structured trace. It is intended for debug/tests, not the search hot path.
func EvaluateWithTrace(b *gm.Board) (score int32, trace EvalTrace) {
	initVariables(b)

	pawnEntry := GetPawnEntry(b, false)
	wPawnAttackBB := pawnEntry.WPawnAttackBB
	bPawnAttackBB := pawnEntry.BPawnAttackBB
	openFiles := pawnEntry.OpenFiles
	wSemiOpenFiles := pawnEntry.WSemiOpenFiles
	bSemiOpenFiles := pawnEntry.BSemiOpenFiles

	pawnMG := pawnEntry.PawnScoreMG
	pawnEG := pawnEntry.PawnScoreEG
	candMG, candEG, _, _ := CandidatePassedTerm(b, pawnEntry)
	pawnMG += candMG
	pawnEG += candEG

	outposts := getOutpostsBB(b, wPawnAttackBB, bPawnAttackBB)
	whiteOutposts := outposts[0]
	blackOutposts := outposts[1]

	wRookStackFiles, bRookStackFiles := getRookConnectedFiles(b)

	lockedCenter, openIdx := getCenterState(b, openFiles, wSemiOpenFiles, bSemiOpenFiles,
		pawnEntry.WLeverBB, pawnEntry.BLeverBB)
	scales := getCenterMobilityScales(lockedCenter, openIdx)

	var knightMovementBB [2]uint64
	var bishopMovementBB [2]uint64
	var rookMovementBB [2]uint64
	var queenMovementBB [2]uint64
	var kingAttackMobilityBB [2]uint64
	var danger kingDanger

	kingRing := getKingSafetyTable(b, true, 0, 0)
	allPieces := b.White.All | b.Black.All

	knightMG, knightEG := evaluateKnights(
		b, wPawnAttackBB, bPawnAttackBB,
		kingRing,
		scales, whiteOutposts, blackOutposts,
		&knightMovementBB, &kingAttackMobilityBB, &danger, false,
	)
	bishopMG, bishopEG := evaluateBishops(
		b, allPieces, wPawnAttackBB, bPawnAttackBB,
		kingRing,
		scales, whiteOutposts, blackOutposts,
		pawnEntry.WBlockedBB, pawnEntry.BBlockedBB,
		&bishopMovementBB, &kingAttackMobilityBB, &danger, false,
	)
	rookMG, rookEG := evaluateRooks(
		b, allPieces, wPawnAttackBB, bPawnAttackBB,
		kingRing,
		openFiles, wSemiOpenFiles, bSemiOpenFiles,
		wRookStackFiles, bRookStackFiles,
		&rookMovementBB, &kingAttackMobilityBB, &danger, false,
	)
	queenMG, queenEG := evaluateQueens(
		b, allPieces, wPawnAttackBB, bPawnAttackBB,
		kingRing,
		&queenMovementBB, &kingAttackMobilityBB, &danger, false,
	)

	kingCheckThreats(b, &danger, allPieces, wPawnAttackBB, bPawnAttackBB,
		knightMovementBB, bishopMovementBB, queenMovementBB)

	kingPsqtMG, kingPsqtEG := countPieceTables(&b.White.Kings, &b.Black.Kings, &PSQT_MG[gm.PieceTypeKing], &PSQT_EG[gm.PieceTypeKing])
	kingAttackPenaltyMG, kingAttackPenaltyEG := kingDangerScore(&danger, b)
	kingShelterMG, kingStormMG, kingStormEG := kingShelterStorm(b, wPawnAttackBB, bPawnAttackBB)
	kingMinorDefenseBonusMG := kingMinorPieceDefences(kingRing, knightMovementBB, bishopMovementBB)
	kingPasserProximityEG := kingPasserProximity(b, pawnEntry)

	wPieceCount := bits.OnesCount64(b.White.Bishops | b.White.Knights | b.White.Rooks | b.White.Queens)
	bPieceCount := bits.OnesCount64(b.Black.Bishops | b.Black.Knights | b.Black.Rooks | b.Black.Queens)
	wPawnCount := bits.OnesCount64(b.White.Pawns)
	bPawnCount := bits.OnesCount64(b.Black.Pawns)

	kingCentralManhattanPenalty := 0
	kingMopUpBonus := 0
	piecePhase := GetPiecePhase(b)
	if (piecePhase < 16 && bits.OnesCount64(b.White.Queens|b.Black.Queens) == 0) || piecePhase < 10 {
		noPawnsLeft := wPawnCount == 0 && bPawnCount == 0
		if wPieceCount > 0 && bPieceCount == 0 && noPawnsLeft {
			kingMopUpBonus = getKingMopUpBonus(b, true, b.White.Queens > 0, b.White.Rooks > 0)
		} else if wPieceCount == 0 && bPieceCount > 0 && noPawnsLeft {
			kingMopUpBonus = getKingMopUpBonus(b, false, b.Black.Queens > 0, b.Black.Rooks > 0)
		} else {
			kingCentralManhattanPenalty = kingEndGameCentralizationPenalty(b)
		}
	}

	kingMG := kingPsqtMG + kingAttackPenaltyMG + kingShelterMG + kingStormMG + kingMinorDefenseBonusMG
	kingEG := kingPsqtEG + kingAttackPenaltyEG + kingStormEG + kingCentralManhattanPenalty + kingMopUpBonus + kingPasserProximityEG

	spaceMG := spaceEvaluation(b, pawnEntry)

	wMaterialMG, wMaterialEG := countMaterial(&b.White)
	bMaterialMG, bMaterialEG := countMaterial(&b.Black)
	materialScoreMG := wMaterialMG - bMaterialMG
	materialScoreEG := wMaterialEG - bMaterialEG

	toMoveBonus := TempoBonus
	if !b.Wtomove {
		toMoveBonus = -TempoBonus
	}
	imbalanceMG, imbalanceEG := materialImbalance(b)

	mgWeight := piecePhase
	egWeight := TotalPhase - piecePhase

	variableScoreMG := pawnMG + knightMG + bishopMG + rookMG + queenMG + kingMG + toMoveBonus + imbalanceMG + spaceMG
	variableScoreEG := pawnEG + knightEG + bishopEG + rookEG + queenEG + kingEG + toMoveBonus + imbalanceEG
	mgScore := materialScoreMG + variableScoreMG
	egScore := materialScoreEG + variableScoreEG

	whiteBeforeDraw := int32((mgScore*mgWeight + egScore*egWeight) / TotalPhase)
	whiteScore := whiteBeforeDraw
	draw := isTheoreticalDraw(b, false)
	if draw {
		whiteScore = whiteScore / DrawDivider
	}
	score = whiteScore
	if !b.Wtomove {
		score = -score
	}

	trace = EvalTrace{
		FEN:        b.ToFen(),
		SideToMove: sideName(b.Wtomove),
		Score: EvalScoreTrace{
			WhitePerspectiveBeforeDraw: whiteBeforeDraw,
			WhitePerspective:           whiteScore,
			SideToMove:                 score,
		},
		Phase: EvalPhaseTrace{
			PiecePhase: piecePhase,
			MGWeight:   mgWeight,
			EGWeight:   egWeight,
			TotalPhase: TotalPhase,
		},
		Draw: EvalDrawTrace{
			Theoretical: draw,
			Divider:     DrawDivider,
			Applied:     draw,
		},
		Totals: EvalTotalsTrace{
			Material: EvalPair{MG: materialScoreMG, EG: materialScoreEG},
			Variable: EvalPair{MG: variableScoreMG, EG: variableScoreEG},
			Score:    EvalPair{MG: mgScore, EG: egScore},
		},
		Material:  EvalPair{MG: materialScoreMG, EG: materialScoreEG},
		Imbalance: EvalPair{MG: imbalanceMG, EG: imbalanceEG},
		Tempo:     toMoveBonus,
		Space:     EvalPair{MG: spaceMG, EG: 0},
		Center: EvalCenterTrace{
			Locked:                lockedCenter,
			Openness:              openIdx,
			KnightMobilityScale:   scales.knightMobilityMG,
			KnightMobilityScaleEG: scales.knightMobilityEG,
			BishopMobilityScale:   scales.bishopMobilityMG,
			BishopMobilityScaleEG: scales.bishopMobilityEG,
			BishopPairScaleMG:     scales.bishopPairMG,
			BishopPairScaleEG:     scales.bishopPairEG,
		},
		Pawn:   buildPawnTrace(b, pawnEntry),
		Knight: traceKnights(b, wPawnAttackBB, bPawnAttackBB, scales, whiteOutposts, blackOutposts, knightMG, knightEG),
		Bishop: traceBishops(b, allPieces, wPawnAttackBB, bPawnAttackBB, scales, whiteOutposts, blackOutposts, pawnEntry.WBlockedBB, pawnEntry.BBlockedBB, bishopMG, bishopEG),
		Rook:   traceRooks(b, allPieces, wPawnAttackBB, bPawnAttackBB, openFiles, wSemiOpenFiles, bSemiOpenFiles, wRookStackFiles, bRookStackFiles, rookMG, rookEG),
		Queen:  traceQueens(b, allPieces, wPawnAttackBB, bPawnAttackBB, queenMG, queenEG),
		King: buildKingTrace(
			b, kingRing, &danger,
			kingPsqtMG, kingPsqtEG,
			kingAttackPenaltyMG, kingAttackPenaltyEG,
			kingShelterMG, kingStormMG, kingStormEG, kingMinorDefenseBonusMG,
			kingCentralManhattanPenalty,
			kingMopUpBonus, kingPasserProximityEG,
			kingMG, kingEG,
		),
	}

	return score, trace
}

// RenderEvalTraceJSON writes the canonical parseable trace format.
func RenderEvalTraceJSON(w io.Writer, trace EvalTrace) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(trace)
}

// RenderEvalTraceText writes a stable human-readable trace summary.
func RenderEvalTraceText(w io.Writer, trace EvalTrace) error {
	writePair := func(name string, p EvalPair) {
		fmt.Fprintf(w, "  %-18s MG %6d  EG %6d\n", name, p.MG, p.EG)
	}
	fmt.Fprintln(w, "Evaluation Trace")
	fmt.Fprintf(w, "FEN: %s\n", trace.FEN)
	fmt.Fprintf(w, "Side to move: %s\n", trace.SideToMove)
	fmt.Fprintf(w, "Score: white-before-draw %d, white %d, side-to-move %d\n",
		trace.Score.WhitePerspectiveBeforeDraw, trace.Score.WhitePerspective, trace.Score.SideToMove)
	fmt.Fprintf(w, "Phase: piece %d, MG weight %d, EG weight %d, total %d\n",
		trace.Phase.PiecePhase, trace.Phase.MGWeight, trace.Phase.EGWeight, trace.Phase.TotalPhase)
	fmt.Fprintf(w, "Draw: theoretical %t, applied %t, divider %d\n",
		trace.Draw.Theoretical, trace.Draw.Applied, trace.Draw.Divider)

	fmt.Fprintln(w, "\nTotals")
	writePair("material", trace.Totals.Material)
	writePair("variable", trace.Totals.Variable)
	writePair("score buckets", trace.Totals.Score)
	writePair("imbalance", trace.Imbalance)
	fmt.Fprintf(w, "  %-18s %6d\n", "tempo", trace.Tempo)

	renderTermGroup(w, "Pawn", trace.Pawn.Terms)
	renderTermGroup(w, "Knight", trace.Knight.Terms)
	renderTermGroup(w, "Bishop", trace.Bishop.Terms)
	renderTermGroup(w, "Rook", trace.Rook.Terms)
	renderTermGroup(w, "Queen", trace.Queen.Terms)
	renderTermGroup(w, "King", trace.King.Terms)

	fmt.Fprintln(w, "\nSpace")
	writePair("space", trace.Space)
	fmt.Fprintf(w, "\nCenter: locked %t, openness %.2f, knight scale %d/%d, bishop scale %d/%d, bishop-pair scale %d/%d (MG/EG)\n",
		trace.Center.Locked, trace.Center.Openness,
		trace.Center.KnightMobilityScale, trace.Center.KnightMobilityScaleEG,
		trace.Center.BishopMobilityScale, trace.Center.BishopMobilityScaleEG,
		trace.Center.BishopPairScaleMG, trace.Center.BishopPairScaleEG)

	fmt.Fprintln(w, "\nBitboards")
	renderSideBB(w, "passed", trace.Pawn.Bitboards["passed"])
	renderSideBB(w, "isolated", trace.Pawn.Bitboards["isolated"])
	renderSideBB(w, "backward", trace.Pawn.Bitboards["backward"])
	renderSideBB(w, "blocked", trace.Pawn.Bitboards["blocked"])
	renderSideBB(w, "lever", trace.Pawn.Bitboards["lever"])
	renderSideBB(w, "candidate", trace.Pawn.Bitboards["candidate"])
	renderSideBB(w, "outpost", trace.Knight.Bitboards["outpostSquares"])
	renderSideBB(w, "rook stacked files", trace.Rook.Bitboards["stackedFiles"])
	renderSideBB(w, "rook blocked stacks", trace.Rook.Bitboards["blockedStackedFiles"])
	return nil
}

func renderTermGroup(w io.Writer, title string, terms map[string]EvalPair) {
	order := []string{"psqt", "material", "mobility", "outpost", "tropism", "pair", "badBishop",
		"semiOpen", "open", "fileCount", "stacked", "seventh", "centralization", "attack",
		"shelter", "storm", "minorDefense", "centralizationPenalty", "mopUp",
		"passerProximity", "isolated", "doubled", "connected", "passed",
		"candidate", "blocked", "backward", "weakLever", "total"}
	fmt.Fprintf(w, "\n%s\n", title)
	for _, name := range order {
		if p, ok := terms[name]; ok {
			fmt.Fprintf(w, "  %-18s MG %6d  EG %6d\n", name, p.MG, p.EG)
		}
	}
}

func renderSideBB(w io.Writer, name string, bb EvalSideBitboards) {
	fmt.Fprintf(w, "  %-18s W %-16s %-24v | B %-16s %-24v\n",
		name, bb.White.Hex, bb.White.Squares, bb.Black.Hex, bb.Black.Squares)
}

func buildPawnTrace(b *gm.Board, entry *PawnHashEntry) EvalPawnTrace {
	pawnPsqtMG, pawnPsqtEG := countPieceTables(&b.White.Pawns, &b.Black.Pawns, &PSQT_MG[gm.PieceTypePawn], &PSQT_EG[gm.PieceTypePawn])
	isoMG, isoEG := isolatedPawnPenalty(entry)
	doubledMG, doubledEG := pawnDoublingPenalties(b, entry)
	connMG, connEG := connectedPawnBonus(b, entry)
	passedMG, passedEG := passedPawnBonus(entry.WPassedBB, entry.BPassedBB)
	candidateMG, candidateEG, wCandidate, bCandidate := CandidatePassedTerm(b, entry)
	wLeverPush, bLeverPush := LeverPushBitboards(b)
	blockedMG, blockedEG := blockedPawnPenalty(entry.WBlockedBB, entry.BBlockedBB)
	backMG, backEG := backwardPawnPenalty(entry)
	wDoubled, bDoubled := doubledPawnBitboards(b)
	weakLeverMG, weakLeverEG := pawnWeakLeverPenalty(entry.WWeakLeverBB, entry.BWeakLeverBB)
	totalMG := entry.PawnScoreMG + candidateMG
	totalEG := entry.PawnScoreEG + candidateEG

	return EvalPawnTrace{
		Terms: map[string]EvalPair{
			"psqt":      {MG: pawnPsqtMG, EG: pawnPsqtEG},
			"isolated":  {MG: isoMG, EG: isoEG},
			"doubled":   {MG: doubledMG, EG: doubledEG},
			"connected": {MG: connMG, EG: connEG},
			"passed":    {MG: passedMG, EG: passedEG},
			"candidate": {MG: candidateMG, EG: candidateEG},
			"blocked":   {MG: blockedMG, EG: blockedEG},
			"backward":  {MG: backMG, EG: backEG},
			"weakLever": {MG: weakLeverMG, EG: weakLeverEG},
			"total":     {MG: totalMG, EG: totalEG},
		},
		Bitboards: map[string]EvalSideBitboards{
			"attacks":  sideBB(entry.WPawnAttackBB, entry.BPawnAttackBB),
			"passed":   sideBB(entry.WPassedBB, entry.BPassedBB),
			"isolated": sideBB(entry.WIsolatedBB, entry.BIsolatedBB),
			"backward": sideBB(entry.WBackwardBB, entry.BBackwardBB),
			"blocked":  sideBB(entry.WBlockedBB, entry.BBlockedBB),
			// "opposed" and "doubled" are what split the doubled and backward
			// terms; the term values above are the totals, so intersect these to
			// read the split.
			"opposed": sideBB(entry.WOpposedBB, entry.BOpposedBB),
			"doubled": sideBB(wDoubled, bDoubled),
			// "phalanx" is no longer a term of its own -- it is the multiplier
			// inside "connected" -- but the set is still worth seeing.
			"phalanx":       sideBB(phalanxPawns(b.White.Pawns), phalanxPawns(b.Black.Pawns)),
			"lever":         sideBB(entry.WLeverBB, entry.BLeverBB),
			"leverPushed":   sideBB(wLeverPush, bLeverPush),
			"weakLever":     sideBB(entry.WWeakLeverBB, entry.BWeakLeverBB),
			"candidate":     sideBB(wCandidate, bCandidate),
			"semiOpenFiles": sideBB(entry.WSemiOpenFiles, entry.BSemiOpenFiles),
			"openFiles":     sideBB(entry.OpenFiles, entry.OpenFiles),
		},
		HashTotal:          EvalPair{MG: entry.PawnScoreMG, EG: entry.PawnScoreEG},
		Total:              EvalPair{MG: totalMG, EG: totalEG},
		CandidatePassedPct: EvalPair{MG: CandidatePassedPctMG, EG: CandidatePassedPctEG},
	}
}

func traceKnights(b *gm.Board, wPawnAttackBB, bPawnAttackBB uint64, scales centerScales, whiteOutposts, blackOutposts uint64, totalMG, totalEG int) EvalPieceTrace {
	psqtMG, psqtEG := countPieceTables(&b.White.Knights, &b.Black.Knights, &PSQT_MG[gm.PieceTypeKnight], &PSQT_EG[gm.PieceTypeKnight])
	mobMG, mobEG, counts := knightMobilityTrace(b, wPawnAttackBB, bPawnAttackBB)
	mobMG = (mobMG * scales.knightMobilityMG) / 100
	mobEG = (mobEG * scales.knightMobilityEG) / 100
	outMG := KnightOutpostMG*bits.OnesCount64(b.White.Knights&whiteOutposts) - KnightOutpostMG*bits.OnesCount64(b.Black.Knights&blackOutposts)
	outEG := KnightOutpostEG*bits.OnesCount64(b.White.Knights&whiteOutposts) - KnightOutpostEG*bits.OnesCount64(b.Black.Knights&blackOutposts)
	tropMG, tropEG := knightKingTropism(b)
	return EvalPieceTrace{
		Terms: map[string]EvalPair{
			"psqt":     {MG: psqtMG, EG: psqtEG},
			"mobility": {MG: mobMG, EG: mobEG},
			"outpost":  {MG: outMG, EG: outEG},
			"tropism":  {MG: tropMG, EG: tropEG},
			"total":    {MG: totalMG, EG: totalEG},
		},
		Bitboards: map[string]EvalSideBitboards{
			"outpostSquares": sideBB(whiteOutposts, blackOutposts),
		},
		MobilityCounts: counts,
		Total:          EvalPair{MG: totalMG, EG: totalEG},
	}
}

func traceBishops(b *gm.Board, allPieces, wPawnAttackBB, bPawnAttackBB uint64, scales centerScales, whiteOutposts, blackOutposts, wBlocked, bBlocked uint64, totalMG, totalEG int) EvalPieceTrace {
	psqtMG, psqtEG := countPieceTables(&b.White.Bishops, &b.Black.Bishops, &PSQT_MG[gm.PieceTypeBishop], &PSQT_EG[gm.PieceTypeBishop])
	mobMG, mobEG, counts := bishopMobilityTrace(b, allPieces, wPawnAttackBB, bPawnAttackBB)
	mobMG = (mobMG * scales.bishopMobilityMG) / 100
	mobEG = (mobEG * scales.bishopMobilityEG) / 100
	outMG := BishopOutpostMG*bits.OnesCount64(b.White.Bishops&whiteOutposts) - BishopOutpostMG*bits.OnesCount64(b.Black.Bishops&blackOutposts)
	outEG := BishopOutpostEG*bits.OnesCount64(b.White.Bishops&whiteOutposts) - BishopOutpostEG*bits.OnesCount64(b.Black.Bishops&blackOutposts)
	pairMG, pairEG := bishopPairBonuses(b)
	pairMG = (pairMG * scales.bishopPairMG) / 100
	pairEG = (pairEG * scales.bishopPairEG) / 100
	badMG, badEG := badBishopTrace(b, wBlocked, bBlocked)
	return EvalPieceTrace{
		Terms: map[string]EvalPair{
			"psqt":      {MG: psqtMG, EG: psqtEG},
			"mobility":  {MG: mobMG, EG: mobEG},
			"outpost":   {MG: outMG, EG: outEG},
			"pair":      {MG: pairMG, EG: pairEG},
			"badBishop": {MG: badMG, EG: badEG},
			"total":     {MG: totalMG, EG: totalEG},
		},
		Bitboards: map[string]EvalSideBitboards{
			"outpostSquares": sideBB(whiteOutposts, blackOutposts),
			"blockedPawns":   sideBB(wBlocked, bBlocked),
		},
		MobilityCounts: counts,
		Total:          EvalPair{MG: totalMG, EG: totalEG},
	}
}

func traceRooks(b *gm.Board, allPieces, wPawnAttackBB, bPawnAttackBB, openFiles, wSemiOpenFiles, bSemiOpenFiles, wStackFiles, bStackFiles uint64, totalMG, totalEG int) EvalPieceTrace {
	psqtMG, psqtEG := countPieceTables(&b.White.Rooks, &b.Black.Rooks, &PSQT_MG[gm.PieceTypeRook], &PSQT_EG[gm.PieceTypeRook])
	mobMG, mobEG, counts := rookMobilityTrace(b, allPieces, wPawnAttackBB, bPawnAttackBB)
	semiMG, semiEG, openMG, openEG := rookFilesBonus(b, openFiles, wSemiOpenFiles, bSemiOpenFiles)
	fileCountMG, fileCountEG := rookFileCountBonus(b, openFiles, wSemiOpenFiles, bSemiOpenFiles)
	stackMG := rookStackBonusMG(wStackFiles, bStackFiles)
	seventhMG, seventhEG := rookSeventhRankBonus(b)
	wBlockedStackFiles, bBlockedStackFiles := getRookBlockedStackFiles(b)
	return EvalPieceTrace{
		Terms: map[string]EvalPair{
			"psqt":      {MG: psqtMG, EG: psqtEG},
			"mobility":  {MG: mobMG, EG: mobEG},
			"semiOpen":  {MG: semiMG, EG: semiEG},
			"open":      {MG: openMG, EG: openEG},
			"fileCount": {MG: fileCountMG, EG: fileCountEG},
			"stacked":   {MG: stackMG, EG: 0},
			"seventh":   {MG: seventhMG, EG: seventhEG},
			"total":     {MG: totalMG, EG: totalEG},
		},
		Bitboards: map[string]EvalSideBitboards{
			"openFiles":           sideBB(openFiles, openFiles),
			"semiOpenFiles":       sideBB(wSemiOpenFiles, bSemiOpenFiles),
			"stackedFiles":        sideBB(wStackFiles, bStackFiles),
			"blockedStackedFiles": sideBB(wBlockedStackFiles, bBlockedStackFiles),
			"seventhRank":         sideBB(b.White.Rooks&seventhRankMask, b.Black.Rooks&secondRankMask),
		},
		MobilityCounts: counts,
		Total:          EvalPair{MG: totalMG, EG: totalEG},
	}
}

func getRookBlockedStackFiles(b *gm.Board) (wFiles uint64, bFiles uint64) {
	allPieces := b.White.All | b.Black.All
	evalSide := func(rooks uint64) uint64 {
		var files uint64
		for file := 0; file < 8; file++ {
			fileMask := onlyFile[file]
			rOnFile := rooks & fileMask
			if bits.OnesCount64(rOnFile) < 2 {
				continue
			}

			minRank := 8
			maxRank := -1
			for x := rOnFile; x != 0; x &= x - 1 {
				rank := bits.TrailingZeros64(x) / 8
				if rank < minRank {
					minRank = rank
				}
				if rank > maxRank {
					maxRank = rank
				}
			}

			var between uint64
			for rank := minRank + 1; rank <= maxRank-1; rank++ {
				between |= PositionBB[file+8*rank]
			}
			if between&allPieces != 0 {
				files |= fileMask
			}
		}
		return files
	}
	return evalSide(b.White.Rooks), evalSide(b.Black.Rooks)
}

func traceQueens(b *gm.Board, allPieces, wPawnAttackBB, bPawnAttackBB uint64, totalMG, totalEG int) EvalPieceTrace {
	psqtMG, psqtEG := countPieceTables(&b.White.Queens, &b.Black.Queens, &PSQT_MG[gm.PieceTypeQueen], &PSQT_EG[gm.PieceTypeQueen])
	mobMG, mobEG, counts := queenMobilityTrace(b, allPieces, wPawnAttackBB, bPawnAttackBB)
	central := centralizedQueen(b)
	return EvalPieceTrace{
		Terms: map[string]EvalPair{
			"psqt":           {MG: psqtMG, EG: psqtEG},
			"mobility":       {MG: mobMG, EG: mobEG},
			"centralization": {MG: 0, EG: central},
			"total":          {MG: totalMG, EG: totalEG},
		},
		Bitboards: map[string]EvalSideBitboards{
			"centralized": sideBB(b.White.Queens&centralizedQueenSquares, b.Black.Queens&centralizedQueenSquares),
		},
		MobilityCounts: counts,
		Total:          EvalPair{MG: totalMG, EG: totalEG},
	}
}

func buildKingTrace(
	b *gm.Board,
	ring [2]uint64,
	danger *kingDanger,
	psqtMG, psqtEG int,
	attackMG, attackEG int,
	shelterMG, stormMG, stormEG, minorDefenseMG int,
	centralizationEG, mopUpEG, passerProximityEG int,
	totalMG, totalEG int,
) EvalKingTrace {
	// AttackUnits* is the clamped danger accumulator; Danger* is what it becomes
	// after squaring. Attackers* is the piece count that drives the weight sum.
	rawToBlack, _ := kingDangerRaw(danger, 0, b.White.Queens != 0)
	rawToWhite, _ := kingDangerRaw(danger, 1, b.Black.Queens != 0)
	cpToBlack, _ := kingDangerFor(danger, 0, b.White.Queens != 0)
	cpToWhite, _ := kingDangerFor(danger, 1, b.Black.Queens != 0)
	return EvalKingTrace{
		Terms: map[string]EvalPair{
			"psqt":   {MG: psqtMG, EG: psqtEG},
			"attack": {MG: attackMG, EG: attackEG},
			// "shelter" replaces the old "file" and "pawnDefense" entries: the
			// per-file table folds the missing-pawn penalty and the pawn-cover
			// bonus into one rank-aware number.
			"shelter": {MG: shelterMG, EG: 0},
			// "storm" moved here from the pawn group when it stopped being an
			// offensive bonus for pushing pawns and became part of the defending
			// king's own file walk. Its endgame half comes only from blocked
			// storm pawns.
			"storm":                 {MG: stormMG, EG: stormEG},
			"minorDefense":          {MG: minorDefenseMG, EG: 0},
			"centralizationPenalty": {MG: 0, EG: centralizationEG},
			"mopUp":                 {MG: 0, EG: mopUpEG},
			"passerProximity":       {MG: 0, EG: passerProximityEG},
			"total":                 {MG: totalMG, EG: totalEG},
		},
		Bitboards: map[string]EvalSideBitboards{
			"ring": sideBB(ring[0], ring[1]),
		},
		AttackersOnBlackKing:    danger.attackers[0],
		AttackersOnWhiteKing:    danger.attackers[1],
		SafeChecksOnBlackKing:   danger.safeChk[0],
		SafeChecksOnWhiteKing:   danger.safeChk[1],
		UnsafeChecksOnBlackKing: danger.unsafeChk[0],
		UnsafeChecksOnWhiteKing: danger.unsafeChk[1],
		AttackUnitsOnBlackKing:  rawToBlack,
		AttackUnitsOnWhiteKing:  rawToWhite,
		DangerToBlackKing:       cpToBlack,
		DangerToWhiteKing:       cpToWhite,
		Total:                   EvalPair{MG: totalMG, EG: totalEG},
	}
}

func knightMobilityTrace(b *gm.Board, wPawnAttackBB, bPawnAttackBB uint64) (mg, eg int, counts []int) {
	counts = make([]int, len(KnightMobilityMG))
	for x := b.White.Knights; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		idx := mobilityIndex(bits.OnesCount64(KnightMasks[sq]&^bPawnAttackBB&^b.White.All), len(KnightMobilityMG)-1)
		counts[idx]++
		mg += KnightMobilityMG[idx]
		eg += KnightMobilityEG[idx]
	}
	for x := b.Black.Knights; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		idx := mobilityIndex(bits.OnesCount64(KnightMasks[sq]&^wPawnAttackBB&^b.Black.All), len(KnightMobilityMG)-1)
		counts[idx]--
		mg -= KnightMobilityMG[idx]
		eg -= KnightMobilityEG[idx]
	}
	return
}

func bishopMobilityTrace(b *gm.Board, allPieces, wPawnAttackBB, bPawnAttackBB uint64) (mg, eg int, counts []int) {
	counts = make([]int, len(BishopMobilityMG))
	for x := b.White.Bishops; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		attacks := gm.CalculateBishopMoveBitboard(uint8(sq), allPieces&^PositionBB[sq])
		idx := mobilityIndex(bits.OnesCount64(attacks&^bPawnAttackBB&^b.White.All), len(BishopMobilityMG)-1)
		counts[idx]++
		mg += BishopMobilityMG[idx]
		eg += BishopMobilityEG[idx]
	}
	for x := b.Black.Bishops; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		attacks := gm.CalculateBishopMoveBitboard(uint8(sq), allPieces&^PositionBB[sq])
		idx := mobilityIndex(bits.OnesCount64(attacks&^wPawnAttackBB&^b.Black.All), len(BishopMobilityMG)-1)
		counts[idx]--
		mg -= BishopMobilityMG[idx]
		eg -= BishopMobilityEG[idx]
	}
	return
}

func rookMobilityTrace(b *gm.Board, allPieces, wPawnAttackBB, bPawnAttackBB uint64) (mg, eg int, counts []int) {
	_ = allPieces // kept in the signature alongside the other sliding-piece trace helpers
	counts = make([]int, len(RookMobilityMG))
	for x := b.White.Rooks; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		attacks := gm.CalculateRookMoveBitboard(uint8(sq), rookAttackOccupancy(b, true))
		idx := mobilityIndex(bits.OnesCount64(attacks&^bPawnAttackBB&^b.White.All), len(RookMobilityMG)-1)
		counts[idx]++
		mg += RookMobilityMG[idx]
		eg += RookMobilityEG[idx]
	}
	for x := b.Black.Rooks; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		attacks := gm.CalculateRookMoveBitboard(uint8(sq), rookAttackOccupancy(b, false))
		idx := mobilityIndex(bits.OnesCount64(attacks&^wPawnAttackBB&^b.Black.All), len(RookMobilityMG)-1)
		counts[idx]--
		mg -= RookMobilityMG[idx]
		eg -= RookMobilityEG[idx]
	}
	return
}

func queenMobilityTrace(b *gm.Board, allPieces, wPawnAttackBB, bPawnAttackBB uint64) (mg, eg int, counts []int) {
	counts = make([]int, len(QueenMobilityMG))
	for x := b.White.Queens; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		attacks := gm.CalculateRookMoveBitboard(uint8(sq), allPieces&^PositionBB[sq]) |
			gm.CalculateBishopMoveBitboard(uint8(sq), allPieces&^PositionBB[sq])
		idx := mobilityIndex(bits.OnesCount64(attacks&^bPawnAttackBB&^b.White.All), len(QueenMobilityMG)-1)
		counts[idx]++
		mg += QueenMobilityMG[idx]
		eg += QueenMobilityEG[idx]
	}
	for x := b.Black.Queens; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		attacks := gm.CalculateRookMoveBitboard(uint8(sq), allPieces&^PositionBB[sq]) |
			gm.CalculateBishopMoveBitboard(uint8(sq), allPieces&^PositionBB[sq])
		idx := mobilityIndex(bits.OnesCount64(attacks&^wPawnAttackBB&^b.Black.All), len(QueenMobilityMG)-1)
		counts[idx]--
		mg -= QueenMobilityMG[idx]
		eg -= QueenMobilityEG[idx]
	}
	return
}

func badBishopTrace(b *gm.Board, wBlocked, bBlocked uint64) (mg, eg int) {
	wLightFixed := bits.OnesCount64(wBlocked & lightSquares)
	wDarkFixed := bits.OnesCount64(wBlocked & darkSquares)
	bLightFixed := bits.OnesCount64(bBlocked & lightSquares)
	bDarkFixed := bits.OnesCount64(bBlocked & darkSquares)
	for x := b.White.Bishops; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		bm, be := badBishopPenalty(sq, wDarkFixed, wLightFixed)
		mg += bm
		eg += be
	}
	for x := b.Black.Bishops; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		bm, be := badBishopPenalty(sq, bDarkFixed, bLightFixed)
		mg -= bm
		eg -= be
	}
	return
}

func sideBB(w, b uint64) EvalSideBitboards {
	return EvalSideBitboards{White: evalBB(w), Black: evalBB(b)}
}

func evalBB(bb uint64) EvalBitboard {
	return EvalBitboard{
		Hex:     fmt.Sprintf("%016x", bb),
		Count:   bits.OnesCount64(bb),
		Squares: bitboardSquares(bb),
	}
}

func bitboardSquares(bb uint64) []string {
	squares := make([]string, 0, bits.OnesCount64(bb))
	for x := bb; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		squares = append(squares, squareName(sq))
	}
	return squares
}

func squareName(sq int) string {
	if sq < 0 || sq >= 64 {
		return ""
	}
	return string([]byte{byte('a' + sq%8), byte('1' + sq/8)})
}

func sideName(wtomove bool) string {
	if wtomove {
		return "white"
	}
	return "black"
}
