package engine

import (
	"encoding/json"
	"fmt"
	"io"
	"math/bits"
	"sort"
	"strings"

	gm "chess-engine/goosemg"
)

// PositionTraceLevel controls additive metadata generation. Extended includes
// Basic; Moves includes both Basic and Extended.
type PositionTraceLevel string

const (
	PositionTraceBasic    PositionTraceLevel = "basic"
	PositionTraceExtended PositionTraceLevel = "extended"
	PositionTraceMoves    PositionTraceLevel = "moves"
	positionTraceSchema                      = 1
)

// ParsePositionTraceLevel validates the public trace-level spelling.
func ParsePositionTraceLevel(value string) (PositionTraceLevel, error) {
	switch PositionTraceLevel(strings.ToLower(strings.TrimSpace(value))) {
	case PositionTraceBasic:
		return PositionTraceBasic, nil
	case PositionTraceExtended:
		return PositionTraceExtended, nil
	case PositionTraceMoves:
		return PositionTraceMoves, nil
	default:
		return "", fmt.Errorf("unknown position trace level %q", value)
	}
}

// ExplainTrace combines exact evaluation accounting with neutral board
// metadata. Position fields never contribute to Evaluation.
type ExplainTrace struct {
	SchemaVersion int                `json:"schemaVersion"`
	Level         PositionTraceLevel `json:"level"`
	Eval          EvalTrace          `json:"eval"`
	Position      PositionTrace      `json:"position"`
}

type PositionTrace struct {
	FEN        string                 `json:"fen"`
	SideToMove string                 `json:"sideToMove"`
	Signature  PositionSignatureTrace `json:"signature"`
	Regions    map[string]RegionTrace `json:"regions"`
	Control    ControlTrace           `json:"control"`
	Pieces     []PositionPieceTrace   `json:"pieces"`
	Pawns      PositionPawnTrace      `json:"pawns"`
	Kings      PositionKingsTrace     `json:"kings"`
	Choices    PositionChoicesTrace   `json:"choices"`
	Extended   *PositionExtendedTrace `json:"extended,omitempty"`
	Moves      []PositionMoveDelta    `json:"moves,omitempty"`
}

type SidePieceCountsTrace struct {
	Pawns   int `json:"pawns"`
	Knights int `json:"knights"`
	Bishops int `json:"bishops"`
	Rooks   int `json:"rooks"`
	Queens  int `json:"queens"`
}

type PositionSignatureTrace struct {
	White          SidePieceCountsTrace `json:"white"`
	Black          SidePieceCountsTrace `json:"black"`
	Material       string               `json:"material"`
	Queenless      bool                 `json:"queenless"`
	PiecePhase     int                  `json:"piecePhase"`
	CastlingRights []string             `json:"castlingRights"`
	HalfmoveClock  int                  `json:"halfmoveClock"`
	FullmoveNumber int                  `json:"fullmoveNumber"`
}

type SideControlTrace struct {
	ByPieceType map[string]EvalBitboard `json:"byPieceType"`
	Union       EvalBitboard            `json:"union"`
	AttackCount []int                   `json:"attackCount"`
}

type ControlTrace struct {
	White     SideControlTrace `json:"white"`
	Black     SideControlTrace `json:"black"`
	Contested EvalBitboard     `json:"contested"`
	WhiteOnly EvalBitboard     `json:"whiteOnly"`
	BlackOnly EvalBitboard     `json:"blackOnly"`
}

type RegionTrace struct {
	Squares          EvalBitboard `json:"squares"`
	WhitePieces      int          `json:"whitePieces"`
	BlackPieces      int          `json:"blackPieces"`
	WhiteControlled  int          `json:"whiteControlled"`
	BlackControlled  int          `json:"blackControlled"`
	ContestedSquares int          `json:"contestedSquares"`
}

type PositionPieceRef struct {
	Side   string `json:"side"`
	Type   string `json:"type"`
	Square string `json:"square"`
}

type PositionPinTrace struct {
	King   string           `json:"king"`
	Pinner PositionPieceRef `json:"pinner"`
	Line   EvalBitboard     `json:"line"`
}

type PositionPieceTrace struct {
	PositionPieceRef
	GeometricAttacks   EvalBitboard       `json:"geometricAttacks"`
	PseudoDestinations EvalBitboard       `json:"pseudoDestinations"`
	LegalDestinations  *EvalBitboard      `json:"legalDestinations,omitempty"`
	EvalMobility       EvalBitboard       `json:"evalMobility"`
	SafeMobility       EvalBitboard       `json:"safeMobility"`
	AttackedBy         []PositionPieceRef `json:"attackedBy"`
	DefendedBy         []PositionPieceRef `json:"defendedBy"`
	EnemyKingRingHits  int                `json:"enemyKingRingHits"`
	AbsolutePin        *PositionPinTrace  `json:"absolutePin,omitempty"`
}

type PawnRelationEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type PawnLeverTrace struct {
	Side   string `json:"side"`
	Source string `json:"source"`
	Target string `json:"target"`
}

type PawnWingCountsTrace struct {
	QueenSide int `json:"queenSide"`
	Center    int `json:"center"`
	KingSide  int `json:"kingSide"`
}

type PositionPawnTrace struct {
	Features       map[string]EvalSideBitboards `json:"features"`
	WhiteIslands   [][]string                   `json:"whiteIslands"`
	BlackIslands   [][]string                   `json:"blackIslands"`
	WhiteChains    []PawnRelationEdge           `json:"whiteChains"`
	BlackChains    []PawnRelationEdge           `json:"blackChains"`
	Levers         []PawnLeverTrace             `json:"levers"`
	WhiteByWing    PawnWingCountsTrace          `json:"whiteByWing"`
	BlackByWing    PawnWingCountsTrace          `json:"blackByWing"`
	OutpostSquares EvalSideBitboards            `json:"outpostSquares"`
}

type KingAttackContributorTrace struct {
	PositionPieceRef
	RingHits int `json:"ringHits"`
	WeightMG int `json:"weightMg"`
	WeightEG int `json:"weightEg"`
}

type PositionKingTrace struct {
	Side               string                       `json:"side"`
	Square             string                       `json:"square"`
	Ring               EvalBitboard                 `json:"ring"`
	AttackContributors []KingAttackContributorTrace `json:"attackContributors"`
	Defenders          []PositionPieceRef           `json:"defenders"`
	ShieldPawns        EvalBitboard                 `json:"shieldPawns"`
	UnoccupiedRing     EvalBitboard                 `json:"unoccupiedRing"`
	WeakSquares        EvalBitboard                 `json:"weakSquares"`
	EscapeSquares      EvalBitboard                 `json:"escapeSquares"`
	OpenFiles          []string                     `json:"openFiles"`
	SemiOpenFiles      []string                     `json:"semiOpenFiles"`
	Attackers          int                          `json:"attackers"`
	SafeChecks         int                          `json:"safeChecks"`
	UnsafeChecks       int                          `json:"unsafeChecks"`
	AttackUnits        int                          `json:"attackUnits"`
	Danger             int                          `json:"danger"`
}

type PositionKingsTrace struct {
	White PositionKingTrace `json:"white"`
	Black PositionKingTrace `json:"black"`
}

type PositionMoveTrace struct {
	Move     string `json:"move"`
	Piece    string `json:"piece"`
	From     string `json:"from"`
	To       string `json:"to"`
	Capture  bool   `json:"capture"`
	Captured string `json:"captured,omitempty"`
	// CapturedSquare differs from To only for en passant.
	CapturedSquare string `json:"capturedSquare,omitempty"`
	Promotion      string `json:"promotion,omitempty"`
	Check          bool   `json:"check"`
	SEE            *int32 `json:"see,omitempty"`
}

type PositionChoicesTrace struct {
	Side          string              `json:"side"`
	InCheck       bool                `json:"inCheck"`
	LegalCount    int                 `json:"legalCount"`
	CaptureCount  int                 `json:"captureCount"`
	CheckCount    int                 `json:"checkCount"`
	QuietCount    int                 `json:"quietCount"`
	Moves         []PositionMoveTrace `json:"moves"`
	LoosePieces   []PositionPieceRef  `json:"loosePieces"`
	HangingPieces []PositionPieceRef  `json:"hangingPieces"`
}

type LatentLineTrace struct {
	Slider             PositionPieceRef  `json:"slider"`
	Direction          string            `json:"direction"`
	CurrentSquares     EvalBitboard      `json:"currentSquares"`
	Blocker            PositionPieceRef  `json:"blocker"`
	SquaresBehind      EvalBitboard      `json:"squaresBehind"`
	ScopeGainIfCleared int               `json:"scopeGainIfCleared"`
	NextPiece          *PositionPieceRef `json:"nextPiece,omitempty"`
}

type PieceRouteTrace struct {
	Piece        PositionPieceRef `json:"piece"`
	ReachableIn1 EvalBitboard     `json:"reachableIn1"`
	ReachableIn2 EvalBitboard     `json:"reachableIn2"`
	ReachableIn3 EvalBitboard     `json:"reachableIn3"`
	EnemyHalf    EvalBitboard     `json:"enemyHalf"`
}

type PieceCriticalityTrace struct {
	Piece                 PositionPieceRef   `json:"piece"`
	SolelyDefendedPieces  []PositionPieceRef `json:"solelyDefendedPieces"`
	UniqueControlSquares  EvalBitboard       `json:"uniqueControlSquares"`
	UniqueKingRingSquares EvalBitboard       `json:"uniqueKingRingSquares"`
	BlocksEnemySliders    []PositionPieceRef `json:"blocksEnemySliders"`
}

type PositionExtendedTrace struct {
	LatentLines []LatentLineTrace       `json:"latentLines"`
	Routes      []PieceRouteTrace       `json:"routes"`
	Criticality []PieceCriticalityTrace `json:"criticality"`
}

type SideIntDelta struct {
	White int `json:"white"`
	Black int `json:"black"`
}

type PawnCountDelta struct {
	Passed    SideIntDelta `json:"passed"`
	Candidate SideIntDelta `json:"candidate"`
	Isolated  SideIntDelta `json:"isolated"`
	Backward  SideIntDelta `json:"backward"`
	Blocked   SideIntDelta `json:"blocked"`
	WeakLever SideIntDelta `json:"weakLever"`
}

type PositionMoveDelta struct {
	PositionMoveTrace
	ControlSquaresDelta  SideIntDelta      `json:"controlSquaresDelta"`
	MobilityDelta        SideIntDelta      `json:"mobilityDelta"`
	ContestedDelta       int               `json:"contestedDelta"`
	KingAttackUnitsDelta SideIntDelta      `json:"kingAttackUnitsDelta"`
	PawnDelta            PawnCountDelta    `json:"pawnDelta"`
	OpenedFiles          []string          `json:"openedFiles"`
	ClosedFiles          []string          `json:"closedFiles"`
	ControlGained        EvalSideBitboards `json:"controlGained"`
	ControlLost          EvalSideBitboards `json:"controlLost"`
	OutpostsCreated      EvalSideBitboards `json:"outpostsCreated"`
	OutpostsLost         EvalSideBitboards `json:"outpostsLost"`
}

type positionPieceState struct {
	ref     PositionPieceRef
	color   gm.Color
	pt      gm.PieceType
	square  int
	attacks uint64
}

type positionMetrics struct {
	control    [2]uint64
	mobility   [2]int
	contested  int
	kingAttack [2]int // attacks on white king, attacks on black king
	pawnCounts [2][6]int
	openFiles  uint64
	outposts   [2]uint64
}

// ExplainTraceForBoard returns exact evaluation accounting and an additive
// read-only position trace.
func ExplainTraceForBoard(b *gm.Board, level PositionTraceLevel) ExplainTrace {
	initVariables(b)
	return ExplainTrace{
		SchemaVersion: positionTraceSchema,
		Level:         level,
		Eval:          EvalTraceForBoard(b),
		Position:      PositionTraceForBoard(b, level),
	}
}

// PositionTraceForBoard derives neutral metadata without changing evaluation
// parameters, search state, or the supplied board.
func PositionTraceForBoard(b *gm.Board, level PositionTraceLevel) PositionTrace {
	initVariables(b)
	states := buildPositionPieceStates(b)
	controlBB, controlCount, byType := buildControl(states)
	entry := GetPawnEntry(b, false)
	ring := getKingSafetyTable(b, true, 0, 0)
	kingDangerAcc := positionKingDanger(b, states, entry, ring)
	pins := findAbsolutePins(b)
	legalByFrom, choices := buildChoices(b)

	trace := PositionTrace{
		FEN:        b.ToFen(),
		SideToMove: sideName(b.Wtomove),
		Signature:  buildPositionSignature(b),
		Regions:    buildRegions(b, controlBB),
		Control: ControlTrace{
			White:     sideControl(byType[0], controlBB[0], controlCount[0]),
			Black:     sideControl(byType[1], controlBB[1], controlCount[1]),
			Contested: evalBB(controlBB[0] & controlBB[1]),
			WhiteOnly: evalBB(controlBB[0] &^ controlBB[1]),
			BlackOnly: evalBB(controlBB[1] &^ controlBB[0]),
		},
		Pawns:   buildPositionPawns(b, entry),
		Choices: choices,
	}

	trace.Pieces = buildPositionPieces(
		b, states, controlBB, entry, ring, pins, legalByFrom)
	trace.Kings = PositionKingsTrace{
		White: buildPositionKing(b, gm.White, states, controlBB, entry, ring, &kingDangerAcc),
		Black: buildPositionKing(b, gm.Black, states, controlBB, entry, ring, &kingDangerAcc),
	}
	trace.Choices.LoosePieces, trace.Choices.HangingPieces =
		buildLooseAndHanging(trace.Pieces, trace.Choices.Moves)

	if level == PositionTraceExtended || level == PositionTraceMoves {
		latent := buildLatentLines(b, states)
		trace.Extended = &PositionExtendedTrace{
			LatentLines: latent,
			Routes:      buildRoutes(b, states, controlBB),
			Criticality: buildCriticality(states, trace.Pieces, controlCount,
				ring, latent),
		}
	}
	if level == PositionTraceMoves {
		trace.Moves = buildMoveDeltas(b, trace.Choices.Moves)
	}
	return trace
}

func RenderExplainTraceJSON(w io.Writer, trace ExplainTrace) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(trace)
}

func buildPositionSignature(b *gm.Board) PositionSignatureTrace {
	w := sidePieceCounts(b.White)
	bl := sidePieceCounts(b.Black)
	rights := make([]string, 0, 4)
	cr := b.CastlingRights()
	if cr&gm.CastlingWhiteK != 0 {
		rights = append(rights, "K")
	}
	if cr&gm.CastlingWhiteQ != 0 {
		rights = append(rights, "Q")
	}
	if cr&gm.CastlingBlackK != 0 {
		rights = append(rights, "k")
	}
	if cr&gm.CastlingBlackQ != 0 {
		rights = append(rights, "q")
	}
	return PositionSignatureTrace{
		White: w, Black: bl,
		Material: fmt.Sprintf("%dP%dN%dB%dR%dQ-vs-%dP%dN%dB%dR%dQ",
			w.Pawns, w.Knights, w.Bishops, w.Rooks, w.Queens,
			bl.Pawns, bl.Knights, bl.Bishops, bl.Rooks, bl.Queens),
		Queenless:      w.Queens+bl.Queens == 0,
		PiecePhase:     GetPiecePhase(b),
		CastlingRights: rights,
		HalfmoveClock:  b.HalfmoveClock(),
		FullmoveNumber: b.FullmoveNumber(),
	}
}

func sidePieceCounts(bb gm.Bitboards) SidePieceCountsTrace {
	return SidePieceCountsTrace{
		Pawns: bits.OnesCount64(bb.Pawns), Knights: bits.OnesCount64(bb.Knights),
		Bishops: bits.OnesCount64(bb.Bishops), Rooks: bits.OnesCount64(bb.Rooks),
		Queens: bits.OnesCount64(bb.Queens),
	}
}

func buildPositionPieceStates(b *gm.Board) []positionPieceState {
	occ := b.AllOccupancy()
	states := make([]positionPieceState, 0, 32)
	for x := occ; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		piece := b.PieceAt(gm.Square(sq))
		color := piece.Color()
		states = append(states, positionPieceState{
			ref:   PositionPieceRef{Side: colorName(color), Type: pieceTypeName(piece.Type()), Square: squareName(sq)},
			color: color, pt: piece.Type(), square: sq,
			attacks: positionAttacks(piece.Type(), sq, occ, color),
		})
	}
	return states
}

func positionAttacks(pt gm.PieceType, sq int, occ uint64, color gm.Color) uint64 {
	occ &^= PositionBB[sq]
	switch pt {
	case gm.PieceTypePawn:
		e, w := PawnCaptureBitboards(PositionBB[sq], color == gm.White)
		return e | w
	case gm.PieceTypeKnight:
		return KnightMasks[sq]
	case gm.PieceTypeBishop:
		return gm.CalculateBishopMoveBitboard(uint8(sq), occ)
	case gm.PieceTypeRook:
		return gm.CalculateRookMoveBitboard(uint8(sq), occ)
	case gm.PieceTypeQueen:
		return gm.CalculateBishopMoveBitboard(uint8(sq), occ) |
			gm.CalculateRookMoveBitboard(uint8(sq), occ)
	case gm.PieceTypeKing:
		return KingMoves[sq]
	}
	return 0
}

func buildControl(states []positionPieceState) ([2]uint64, [2][]int, [2]map[string]uint64) {
	var union [2]uint64
	counts := [2][]int{make([]int, 64), make([]int, 64)}
	byType := [2]map[string]uint64{{}, {}}
	for _, piece := range states {
		side := int(piece.color)
		union[side] |= piece.attacks
		byType[side][piece.ref.Type] |= piece.attacks
		for x := piece.attacks; x != 0; x &= x - 1 {
			counts[side][bits.TrailingZeros64(x)]++
		}
	}
	return union, counts, byType
}

func sideControl(byType map[string]uint64, union uint64, counts []int) SideControlTrace {
	out := map[string]EvalBitboard{}
	for _, name := range []string{"pawn", "knight", "bishop", "rook", "queen", "king"} {
		out[name] = evalBB(byType[name])
	}
	return SideControlTrace{ByPieceType: out, Union: evalBB(union), AttackCount: counts}
}

func buildRegions(b *gm.Board, control [2]uint64) map[string]RegionTrace {
	regions := map[string]uint64{
		"queenside":   filesMask(0, 2),
		"centerFiles": filesMask(3, 4),
		"kingside":    filesMask(5, 7),
		"coreCenter":  squaresMask(2, 5, 2, 5),
		"whiteCamp":   squaresMask(0, 7, 0, 3),
		"blackCamp":   squaresMask(0, 7, 4, 7),
	}
	out := map[string]RegionTrace{}
	for name, mask := range regions {
		out[name] = RegionTrace{
			Squares:          evalBB(mask),
			WhitePieces:      bits.OnesCount64(mask & b.White.All),
			BlackPieces:      bits.OnesCount64(mask & b.Black.All),
			WhiteControlled:  bits.OnesCount64(mask & control[0]),
			BlackControlled:  bits.OnesCount64(mask & control[1]),
			ContestedSquares: bits.OnesCount64(mask & control[0] & control[1]),
		}
	}
	return out
}

func buildPositionPieces(b *gm.Board, states []positionPieceState, control [2]uint64,
	entry *PawnHashEntry, ring [2]uint64,
	pins map[int]PositionPinTrace, legalByFrom map[int]uint64) []PositionPieceTrace {
	pieces := make([]PositionPieceTrace, 0, len(states))
	for _, state := range states {
		enemy := 1 - int(state.color)
		own := b.ColorOccupancy(state.color)
		pseudo := state.attacks &^ own
		evalMob := pseudo
		if state.pt == gm.PieceTypePawn || state.pt == gm.PieceTypeKing {
			evalMob = 0
		} else if enemy == 0 {
			evalMob &^= entry.WPawnAttackBB
		} else {
			evalMob &^= entry.BPawnAttackBB
		}
		safe := pseudo &^ control[enemy]
		attacked, defended := []PositionPieceRef{}, []PositionPieceRef{}
		for _, other := range states {
			if other.square == state.square || other.attacks&PositionBB[state.square] == 0 {
				continue
			}
			if other.color == state.color {
				defended = append(defended, other.ref)
			} else {
				attacked = append(attacked, other.ref)
			}
		}
		sortPieceRefs(attacked)
		sortPieceRefs(defended)
		enemyKing := enemy
		piece := PositionPieceTrace{
			PositionPieceRef: state.ref,
			GeometricAttacks: evalBB(state.attacks), PseudoDestinations: evalBB(pseudo),
			EvalMobility: evalBB(evalMob), SafeMobility: evalBB(safe),
			AttackedBy: attacked, DefendedBy: defended,
			EnemyKingRingHits: bits.OnesCount64(state.attacks & ring[enemyKing]),
		}
		// Full legality is defined only for the side to move. Omitting this field
		// for the opponent avoids representing "not generated" as no mobility.
		if state.color == b.SideToMove() {
			legal := evalBB(legalByFrom[state.square])
			piece.LegalDestinations = &legal
		}
		if pin, ok := pins[state.square]; ok {
			p := pin
			piece.AbsolutePin = &p
		}
		pieces = append(pieces, piece)
	}
	return pieces
}

func buildPositionPawns(b *gm.Board, entry *PawnHashEntry) PositionPawnTrace {
	outposts := getOutpostsBB(b, entry.WPawnAttackBB, entry.BPawnAttackBB)
	_, _, wCandidate, bCandidate := CandidatePassedTerm(b, entry)
	wLeverPush, bLeverPush := LeverPushBitboards(b)
	features := map[string]EvalSideBitboards{
		"attacks":       sideBB(entry.WPawnAttackBB, entry.BPawnAttackBB),
		"passed":        sideBB(entry.WPassedBB, entry.BPassedBB),
		"isolated":      sideBB(entry.WIsolatedBB, entry.BIsolatedBB),
		"backward":      sideBB(entry.WBackwardBB, entry.BBackwardBB),
		"blocked":       sideBB(entry.WBlockedBB, entry.BBlockedBB),
		"lever":         sideBB(entry.WLeverBB, entry.BLeverBB),
		"leverPushed":   sideBB(wLeverPush, bLeverPush),
		"weakLever":     sideBB(entry.WWeakLeverBB, entry.BWeakLeverBB),
		"candidate":     sideBB(wCandidate, bCandidate),
		"openFiles":     sideBB(entry.OpenFiles, entry.OpenFiles),
		"semiOpenFiles": sideBB(entry.WSemiOpenFiles, entry.BSemiOpenFiles),
	}
	return PositionPawnTrace{
		Features:     features,
		WhiteIslands: pawnIslands(b.White.Pawns), BlackIslands: pawnIslands(b.Black.Pawns),
		WhiteChains: pawnChains(b.White.Pawns, gm.White), BlackChains: pawnChains(b.Black.Pawns, gm.Black),
		Levers: pawnLevers(b), WhiteByWing: pawnWingCounts(b.White.Pawns),
		BlackByWing: pawnWingCounts(b.Black.Pawns), OutpostSquares: sideBB(outposts[0], outposts[1]),
	}
}

func buildChoices(b *gm.Board) (map[int]uint64, PositionChoicesTrace) {
	board := *b
	moves := board.GenerateLegalMoves()
	byFrom := map[int]uint64{}
	rows := make([]PositionMoveTrace, 0, len(moves))
	captures, checks := 0, 0
	for _, move := range moves {
		row := describeMove(&board, move)
		rows = append(rows, row)
		byFrom[int(move.From())] |= PositionBB[int(move.To())]
		if row.Capture {
			captures++
		}
		if row.Check {
			checks++
		}
	}
	return byFrom, PositionChoicesTrace{
		Side: colorName(board.SideToMove()), InCheck: board.OurKingInCheck(),
		LegalCount: len(rows), CaptureCount: captures, CheckCount: checks,
		QuietCount: len(rows) - captures, Moves: rows,
	}
}

func describeMove(b *gm.Board, move gm.Move) PositionMoveTrace {
	from, to := int(move.From()), int(move.To())
	piece := b.PieceAt(move.From())
	captured := b.PieceAt(move.To())
	capture := gm.IsCapture(move, b)
	row := PositionMoveTrace{
		Move: move.String(), Piece: pieceTypeName(piece.Type()), From: squareName(from),
		To: squareName(to), Capture: capture, Check: b.GivesCheck(move),
	}
	if capture {
		capturedSquare := to
		if captured == gm.NoPiece {
			row.Captured = "pawn"
			if piece.Color() == gm.White {
				capturedSquare -= 8
			} else {
				capturedSquare += 8
			}
		} else {
			row.Captured = pieceTypeName(captured.Type())
		}
		row.CapturedSquare = squareName(capturedSquare)
		value := SEE(b, move)
		row.SEE = &value
	}
	if promo := move.PromotionPieceType(); promo != gm.PieceTypeNone {
		row.Promotion = pieceTypeName(promo)
	}
	return row
}

// positionKingDanger rebuilds the evaluation's king-danger accumulator from the
// trace's piece states, using the evaluation's occupancy rules rather than the
// plain ones positionAttacks uses elsewhere. Rooks are the reason this exists:
// evaluateRooks counts ring pressure through friendly rooks and both queens, so
// reporting true-occupancy hits here made the two paths disagree on 3.3% of
// positions.
func positionKingDanger(b *gm.Board, states []positionPieceState,
	entry *PawnHashEntry, ring [2]uint64) kingDanger {

	var danger kingDanger
	var knightAtk, bishopAtk, queenAtk [2]uint64
	for _, s := range states {
		switch s.pt {
		case gm.PieceTypeKnight:
			knightAtk[int(s.color)] |= s.attacks
		case gm.PieceTypeBishop:
			bishopAtk[int(s.color)] |= s.attacks
		case gm.PieceTypeQueen:
			queenAtk[int(s.color)] |= s.attacks
		}
	}
	for _, s := range states {
		if s.pt == gm.PieceTypePawn || s.pt == gm.PieceTypeKing {
			continue
		}
		side := int(s.color)
		wMG, wEG := safetyWeight(s.pt)
		danger.addAttacker(side, bits.OnesCount64(positionRingAttacks(b, s)&ring[1-side]), wMG, wEG)
	}
	kingCheckThreats(b, &danger, b.White.All|b.Black.All,
		entry.WPawnAttackBB, entry.BPawnAttackBB, knightAtk, bishopAtk, queenAtk)
	return danger
}

// positionRingAttacks returns the attack set the evaluation uses when charging
// king-ring pressure for this piece.
func positionRingAttacks(b *gm.Board, s positionPieceState) uint64 {
	if s.pt == gm.PieceTypeRook {
		return gm.CalculateRookMoveBitboard(uint8(s.square), rookAttackOccupancy(b, s.color == gm.White))
	}
	return s.attacks
}

func buildPositionKing(b *gm.Board, side gm.Color, states []positionPieceState,
	control [2]uint64, entry *PawnHashEntry,
	ring [2]uint64, danger *kingDanger) PositionKingTrace {
	idx, enemy := int(side), 1-int(side)
	kingBB := b.Bitboards(side).Kings
	if kingBB == 0 {
		return PositionKingTrace{Side: colorName(side)}
	}
	kingSq := bits.TrailingZeros64(kingBB)
	contributors := []KingAttackContributorTrace{}
	defenders := []PositionPieceRef{}
	// The accumulator is built once for the whole board in positionKingDanger;
	// this loop only names the pieces behind it.
	for _, state := range states {
		if state.color == side {
			if state.pt != gm.PieceTypeKing && state.attacks&ring[idx] != 0 {
				defenders = append(defenders, state.ref)
			}
			continue
		}
		if state.pt == gm.PieceTypePawn || state.pt == gm.PieceTypeKing {
			continue
		}
		hits := bits.OnesCount64(positionRingAttacks(b, state) & ring[idx])
		if hits == 0 {
			continue
		}
		wMG, wEG := safetyWeight(state.pt)
		contributors = append(contributors, KingAttackContributorTrace{
			PositionPieceRef: state.ref, RingHits: hits, WeightMG: wMG, WeightEG: wEG,
		})
	}
	attackerHasQueen := b.Bitboards(gm.Color(enemy)).Queens != 0
	rawUnits, _ := kingDangerRaw(danger, enemy, attackerHasQueen)
	dangerCp, _ := kingDangerFor(danger, enemy, attackerHasQueen)
	sortPieceRefs(defenders)
	own := b.Bitboards(side)
	pawnAttacks := entry.WPawnAttackBB
	semi := entry.WSemiOpenFiles
	if side == gm.Black {
		pawnAttacks = entry.BPawnAttackBB
		semi = entry.BSemiOpenFiles
	}
	shield := ring[idx] & own.Pawns
	weak := ring[idx] &^ pawnAttacks &^ own.All
	escape := KingMoves[kingSq] &^ own.All &^ control[enemy]
	zoneFiles := getKingFileZone(kingSq % 8)
	return PositionKingTrace{
		Side: colorName(side), Square: squareName(kingSq), Ring: evalBB(ring[idx]),
		AttackContributors: contributors, Defenders: defenders, ShieldPawns: evalBB(shield),
		UnoccupiedRing: evalBB(ring[idx] &^ own.All), WeakSquares: evalBB(weak), EscapeSquares: evalBB(escape),
		OpenFiles: fileNames(entry.OpenFiles & zoneFiles), SemiOpenFiles: fileNames(semi & zoneFiles),
		Attackers: danger.attackers[enemy], AttackUnits: rawUnits, Danger: dangerCp,
		SafeChecks: danger.safeChk[enemy], UnsafeChecks: danger.unsafeChk[enemy],
	}
}

func findAbsolutePins(b *gm.Board) map[int]PositionPinTrace {
	pins := map[int]PositionPinTrace{}
	dirs := []struct {
		df, dr   int
		name     string
		diagonal bool
	}{
		{0, 1, "north", false}, {0, -1, "south", false}, {1, 0, "east", false}, {-1, 0, "west", false},
		{1, 1, "northEast", true}, {-1, 1, "northWest", true}, {1, -1, "southEast", true}, {-1, -1, "southWest", true},
	}
	for _, side := range []gm.Color{gm.White, gm.Black} {
		kingBB := b.Bitboards(side).Kings
		if kingBB == 0 {
			continue
		}
		ksq := bits.TrailingZeros64(kingBB)
		kf, kr := ksq%8, ksq/8
		for _, dir := range dirs {
			candidate := -1
			line := uint64(0)
			for f, r := kf+dir.df, kr+dir.dr; f >= 0 && f < 8 && r >= 0 && r < 8; f, r = f+dir.df, r+dir.dr {
				sq := r*8 + f
				pc := b.PieceAt(gm.Square(sq))
				line |= PositionBB[sq]
				if pc == gm.NoPiece {
					continue
				}
				if candidate < 0 {
					if pc.Color() != side {
						break
					}
					candidate = sq
					continue
				}
				if pc.Color() == side {
					break
				}
				pt := pc.Type()
				valid := (dir.diagonal && (pt == gm.PieceTypeBishop || pt == gm.PieceTypeQueen)) ||
					(!dir.diagonal && (pt == gm.PieceTypeRook || pt == gm.PieceTypeQueen))
				if valid {
					pins[candidate] = PositionPinTrace{King: squareName(ksq), Pinner: pieceRef(pc, sq), Line: evalBB(line)}
				}
				break
			}
		}
	}
	return pins
}

func buildLooseAndHanging(pieces []PositionPieceTrace, moves []PositionMoveTrace) ([]PositionPieceRef, []PositionPieceRef) {
	loose := []PositionPieceRef{}
	hangingSet := map[string]bool{}
	for _, move := range moves {
		if move.Capture && move.SEE != nil && *move.SEE > 0 {
			hangingSet[move.CapturedSquare] = true
		}
	}
	for _, piece := range pieces {
		if len(piece.AttackedBy) > 0 && len(piece.DefendedBy) == 0 {
			loose = append(loose, piece.PositionPieceRef)
		}
	}
	hanging := []PositionPieceRef{}
	for _, piece := range pieces {
		if hangingSet[piece.Square] {
			hanging = append(hanging, piece.PositionPieceRef)
		}
	}
	sortPieceRefs(loose)
	sortPieceRefs(hanging)
	return loose, hanging
}

func buildLatentLines(b *gm.Board, states []positionPieceState) []LatentLineTrace {
	stateBySquare := map[int]positionPieceState{}
	for _, state := range states {
		stateBySquare[state.square] = state
	}
	lines := []LatentLineTrace{}
	for _, state := range states {
		if state.pt != gm.PieceTypeBishop && state.pt != gm.PieceTypeRook && state.pt != gm.PieceTypeQueen {
			continue
		}
		for _, dir := range sliderDirections(state.pt) {
			f, r := state.square%8, state.square/8
			current := uint64(0)
			blockerSq := -1
			behind := uint64(0)
			nextSq := -1
			for nf, nr := f+dir.df, r+dir.dr; nf >= 0 && nf < 8 && nr >= 0 && nr < 8; nf, nr = nf+dir.df, nr+dir.dr {
				sq := nr*8 + nf
				if blockerSq < 0 {
					if _, occupied := stateBySquare[sq]; occupied {
						blockerSq = sq
					} else {
						current |= PositionBB[sq]
					}
					continue
				}
				if _, occupied := stateBySquare[sq]; occupied {
					behind |= PositionBB[sq]
					nextSq = sq
					break
				}
				behind |= PositionBB[sq]
			}
			if blockerSq < 0 || behind == 0 {
				continue
			}
			blocker := stateBySquare[blockerSq]
			line := LatentLineTrace{Slider: state.ref, Direction: dir.name, CurrentSquares: evalBB(current),
				Blocker: blocker.ref, SquaresBehind: evalBB(behind), ScopeGainIfCleared: bits.OnesCount64(behind)}
			if nextSq >= 0 {
				ref := stateBySquare[nextSq].ref
				line.NextPiece = &ref
			}
			lines = append(lines, line)
		}
	}
	return lines
}

type traceDirection struct {
	df, dr int
	name   string
}

func sliderDirections(pt gm.PieceType) []traceDirection {
	orth := []traceDirection{{0, 1, "north"}, {0, -1, "south"}, {1, 0, "east"}, {-1, 0, "west"}}
	diag := []traceDirection{{1, 1, "northEast"}, {-1, 1, "northWest"}, {1, -1, "southEast"}, {-1, -1, "southWest"}}
	if pt == gm.PieceTypeBishop {
		return diag
	}
	if pt == gm.PieceTypeRook {
		return orth
	}
	return append(orth, diag...)
}

func buildRoutes(b *gm.Board, states []positionPieceState, control [2]uint64) []PieceRouteTrace {
	routes := []PieceRouteTrace{}
	for _, state := range states {
		if state.pt == gm.PieceTypePawn {
			continue
		}
		levels := routeLevels(b, state, control[1-int(state.color)])
		enemyHalfMask := ranksAbove[4]
		if state.color == gm.Black {
			enemyHalfMask = ranksBelow[3]
		}
		routes = append(routes, PieceRouteTrace{Piece: state.ref, ReachableIn1: evalBB(levels[0]),
			ReachableIn2: evalBB(levels[1]), ReachableIn3: evalBB(levels[2]),
			EnemyHalf: evalBB((levels[0] | levels[1] | levels[2]) & enemyHalfMask)})
	}
	return routes
}

func routeLevels(b *gm.Board, state positionPieceState, enemyControl uint64) [3]uint64 {
	var levels [3]uint64
	frontier := map[int]bool{state.square: true}
	visited := PositionBB[state.square]
	baseOcc := b.AllOccupancy() &^ PositionBB[state.square]
	baseOwn := b.ColorOccupancy(state.color) &^ PositionBB[state.square]
	enemyOcc := b.ColorOccupancy(oppositeColor(state.color))
	for depth := 0; depth < 3; depth++ {
		next := map[int]bool{}
		for sq := range frontier {
			occ := baseOcc | PositionBB[sq]
			own := baseOwn | PositionBB[sq]
			dest := positionAttacks(state.pt, sq, occ, state.color) &^ own
			if state.pt == gm.PieceTypeKing {
				dest &^= enemyControl
			}
			levels[depth] |= dest &^ visited
			for x := dest &^ enemyOcc; x != 0; x &= x - 1 {
				next[bits.TrailingZeros64(x)] = true
			}
		}
		visited |= levels[depth]
		frontier = next
	}
	return levels
}

func buildCriticality(states []positionPieceState, pieces []PositionPieceTrace,
	controlCount [2][]int, kingRing [2]uint64, latent []LatentLineTrace) []PieceCriticalityTrace {
	blockedBy := map[string][]PositionPieceRef{}
	for _, line := range latent {
		if line.Slider.Side != line.Blocker.Side {
			blockedBy[refKey(line.Blocker)] = append(blockedBy[refKey(line.Blocker)], line.Slider)
		}
	}
	out := []PieceCriticalityTrace{}
	for _, state := range states {
		sole := []PositionPieceRef{}
		for _, candidate := range pieces {
			if candidate.Side != state.ref.Side || candidate.Square == state.ref.Square {
				continue
			}
			if len(candidate.DefendedBy) == 1 && refKey(candidate.DefendedBy[0]) == refKey(state.ref) {
				sole = append(sole, candidate.PositionPieceRef)
			}
		}
		unique := uint64(0)
		ring := uint64(0)
		side := int(state.color)
		for x := state.attacks; x != 0; x &= x - 1 {
			sq := bits.TrailingZeros64(x)
			if controlCount[side][sq] == 1 {
				unique |= PositionBB[sq]
			}
		}
		ring = unique & kingRing[side]
		blockers := blockedBy[refKey(state.ref)]
		sortPieceRefs(sole)
		sortPieceRefs(blockers)
		out = append(out, PieceCriticalityTrace{Piece: state.ref, SolelyDefendedPieces: sole,
			UniqueControlSquares: evalBB(unique), UniqueKingRingSquares: evalBB(ring),
			BlocksEnemySliders: blockers})
	}
	return out
}

func buildMoveDeltas(b *gm.Board, moves []PositionMoveTrace) []PositionMoveDelta {
	before := collectPositionMetrics(b)
	legal := b.GenerateLegalMoves()
	described := map[string]PositionMoveTrace{}
	for _, row := range moves {
		described[row.Move] = row
	}
	out := make([]PositionMoveDelta, 0, len(legal))
	for _, move := range legal {
		after := *b
		ok, _ := after.MakeMove(move)
		if !ok {
			continue
		}
		post := collectPositionMetrics(&after)
		row := described[move.String()]
		out = append(out, PositionMoveDelta{
			PositionMoveTrace:    row,
			ControlSquaresDelta:  SideIntDelta{White: bits.OnesCount64(post.control[0]) - bits.OnesCount64(before.control[0]), Black: bits.OnesCount64(post.control[1]) - bits.OnesCount64(before.control[1])},
			MobilityDelta:        SideIntDelta{White: post.mobility[0] - before.mobility[0], Black: post.mobility[1] - before.mobility[1]},
			ContestedDelta:       post.contested - before.contested,
			KingAttackUnitsDelta: SideIntDelta{White: post.kingAttack[0] - before.kingAttack[0], Black: post.kingAttack[1] - before.kingAttack[1]},
			PawnDelta:            pawnCountDelta(before.pawnCounts, post.pawnCounts),
			OpenedFiles:          fileNames(post.openFiles &^ before.openFiles), ClosedFiles: fileNames(before.openFiles &^ post.openFiles),
			ControlGained:   sideBB(post.control[0]&^before.control[0], post.control[1]&^before.control[1]),
			ControlLost:     sideBB(before.control[0]&^post.control[0], before.control[1]&^post.control[1]),
			OutpostsCreated: sideBB(post.outposts[0]&^before.outposts[0], post.outposts[1]&^before.outposts[1]),
			OutpostsLost:    sideBB(before.outposts[0]&^post.outposts[0], before.outposts[1]&^post.outposts[1]),
		})
	}
	return out
}

func collectPositionMetrics(b *gm.Board) positionMetrics {
	states := buildPositionPieceStates(b)
	control, _, _ := buildControl(states)
	entry := GetPawnEntry(b, false)
	ring := getKingSafetyTable(b, true, 0, 0)
	metric := positionMetrics{control: control, contested: bits.OnesCount64(control[0] & control[1]), openFiles: entry.OpenFiles}
	metric.outposts = getOutpostsBB(b, entry.WPawnAttackBB, entry.BPawnAttackBB)
	for _, state := range states {
		if state.pt != gm.PieceTypePawn && state.pt != gm.PieceTypeKing {
			enemyPawn := entry.BPawnAttackBB
			if state.color == gm.Black {
				enemyPawn = entry.WPawnAttackBB
			}
			metric.mobility[int(state.color)] += bits.OnesCount64(state.attacks &^ b.ColorOccupancy(state.color) &^ enemyPawn)
		}
		if state.color == gm.White && state.pt != gm.PieceTypePawn && state.pt != gm.PieceTypeKing {
			mg, _ := safetyWeight(state.pt)
			if hits := bits.OnesCount64(state.attacks & ring[1]); hits > 0 {
				metric.kingAttack[1] += mg + SafetyAttackValueMG*hits
			}
		}
		if state.color == gm.Black && state.pt != gm.PieceTypePawn && state.pt != gm.PieceTypeKing {
			mg, _ := safetyWeight(state.pt)
			if hits := bits.OnesCount64(state.attacks & ring[0]); hits > 0 {
				metric.kingAttack[0] += mg + SafetyAttackValueMG*hits
			}
		}
	}
	_, _, wCandidate, bCandidate := CandidatePassedTerm(b, entry)
	for side := 0; side < 2; side++ {
		metric.pawnCounts[side] = [6]int{bits.OnesCount64(mapBB(side, entry.WPassedBB, entry.BPassedBB)), bits.OnesCount64(mapBB(side, wCandidate, bCandidate)), bits.OnesCount64(mapBB(side, entry.WIsolatedBB, entry.BIsolatedBB)), bits.OnesCount64(mapBB(side, entry.WBackwardBB, entry.BBackwardBB)), bits.OnesCount64(mapBB(side, entry.WBlockedBB, entry.BBlockedBB)), bits.OnesCount64(mapBB(side, entry.WWeakLeverBB, entry.BWeakLeverBB))}
	}
	return metric
}

func pawnCountDelta(before, after [2][6]int) PawnCountDelta {
	d := func(i int) SideIntDelta {
		return SideIntDelta{White: after[0][i] - before[0][i], Black: after[1][i] - before[1][i]}
	}
	return PawnCountDelta{Passed: d(0), Candidate: d(1), Isolated: d(2), Backward: d(3), Blocked: d(4), WeakLever: d(5)}
}

func mapBB(side int, white, black uint64) uint64 {
	if side == 0 {
		return white
	}
	return black
}

func pawnIslands(pawns uint64) [][]string {
	files := [8]uint64{}
	for file := 0; file < 8; file++ {
		files[file] = pawns & onlyFile[file]
	}
	out := [][]string{}
	for file := 0; file < 8; {
		if files[file] == 0 {
			file++
			continue
		}
		island := uint64(0)
		for file < 8 && files[file] != 0 {
			island |= files[file]
			file++
		}
		out = append(out, bitboardSquares(island))
	}
	return out
}

func pawnChains(pawns uint64, side gm.Color) []PawnRelationEdge {
	edges := []PawnRelationEdge{}
	for x := pawns; x != 0; x &= x - 1 {
		from := bits.TrailingZeros64(x)
		e, w := PawnCaptureBitboards(PositionBB[from], side == gm.White)
		for y := (e | w) & pawns; y != 0; y &= y - 1 {
			edges = append(edges, PawnRelationEdge{From: squareName(from), To: squareName(bits.TrailingZeros64(y))})
		}
	}
	return edges
}

func pawnLevers(b *gm.Board) []PawnLeverTrace {
	out := []PawnLeverTrace{}
	for _, side := range []gm.Color{gm.White, gm.Black} {
		own, enemy := b.Bitboards(side).Pawns, b.Bitboards(oppositeColor(side)).Pawns
		for x := own; x != 0; x &= x - 1 {
			from := bits.TrailingZeros64(x)
			e, w := PawnCaptureBitboards(PositionBB[from], side == gm.White)
			for y := (e | w) & enemy; y != 0; y &= y - 1 {
				out = append(out, PawnLeverTrace{Side: colorName(side), Source: squareName(from), Target: squareName(bits.TrailingZeros64(y))})
			}
		}
	}
	return out
}

func pawnWingCounts(pawns uint64) PawnWingCountsTrace {
	return PawnWingCountsTrace{QueenSide: bits.OnesCount64(pawns & filesMask(0, 2)), Center: bits.OnesCount64(pawns & filesMask(3, 4)), KingSide: bits.OnesCount64(pawns & filesMask(5, 7))}
}

func fileNames(mask uint64) []string {
	out := []string{}
	for file := 0; file < 8; file++ {
		if mask&onlyFile[file] != 0 {
			out = append(out, string(rune('a'+file)))
		}
	}
	return out
}

func filesMask(minFile, maxFile int) uint64 {
	var bb uint64
	for f := minFile; f <= maxFile; f++ {
		bb |= onlyFile[f]
	}
	return bb
}
func squaresMask(minFile, maxFile, minRank, maxRank int) uint64 {
	var bb uint64
	for r := minRank; r <= maxRank; r++ {
		for f := minFile; f <= maxFile; f++ {
			bb |= PositionBB[r*8+f]
		}
	}
	return bb
}
func oppositeColor(side gm.Color) gm.Color {
	if side == gm.White {
		return gm.Black
	}
	return gm.White
}
func colorName(side gm.Color) string {
	if side == gm.White {
		return "white"
	}
	return "black"
}

func pieceTypeName(pt gm.PieceType) string {
	switch pt {
	case gm.PieceTypePawn:
		return "pawn"
	case gm.PieceTypeKnight:
		return "knight"
	case gm.PieceTypeBishop:
		return "bishop"
	case gm.PieceTypeRook:
		return "rook"
	case gm.PieceTypeQueen:
		return "queen"
	case gm.PieceTypeKing:
		return "king"
	}
	return "none"
}

func pieceRef(piece gm.Piece, sq int) PositionPieceRef {
	return PositionPieceRef{Side: colorName(piece.Color()), Type: pieceTypeName(piece.Type()), Square: squareName(sq)}
}
func refKey(ref PositionPieceRef) string { return ref.Side + ":" + ref.Type + ":" + ref.Square }
func sortPieceRefs(refs []PositionPieceRef) {
	sort.Slice(refs, func(i, j int) bool { return refKey(refs[i]) < refKey(refs[j]) })
}
