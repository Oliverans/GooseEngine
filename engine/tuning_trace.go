package engine

import (
	"encoding/json"
	"io"
	"math/bits"

	gm "chess-engine/goosemg"
)

// tuningTraceSchema changes whenever the meaning or shape of a tuning record
// changes. It is deliberately independent of the broader position-trace schema.
const TuningTraceSchemaVersion = 1

const tuningTraceSchema = TuningTraceSchemaVersion

// TuningIndexedUnit is one non-zero coefficient in a one-dimensional table.
type TuningIndexedUnit struct {
	Index int `json:"i"`
	Units int `json:"u"`
}

// TuningPSQTUnit is one non-zero coefficient in a piece-square table. Piece is
// the goosemg PieceType value and Square is always from White's point of view.
type TuningPSQTUnit struct {
	Piece  int `json:"p"`
	Square int `json:"s"`
	Units  int `json:"u"`
}

type TuningMobilityUnits struct {
	Knight [9]int  `json:"knight"`
	Bishop [14]int `json:"bishop"`
	Rook   [15]int `json:"rook"`
	Queen  [22]int `json:"queen"`
}

// Connected keeps the two colours separate because the derived EG ramp divides
// each side before subtracting. Combining the signed units first can round a
// different way for negative values.
type TuningConnectedUnits struct {
	White [7]int `json:"white"`
	Black [7]int `json:"black"`
}

type TuningPawnUnits struct {
	IsolatedOpposed   int                     `json:"isolatedOpposed"`
	IsolatedUnopposed int                     `json:"isolatedUnopposed"`
	DoubledOpposed    int                     `json:"doubledOpposed"`
	DoubledUnopposed  int                     `json:"doubledUnopposed"`
	BackwardOpposed   int                     `json:"backwardOpposed"`
	BackwardUnopposed int                     `json:"backwardUnopposed"`
	WeakLever         int                     `json:"weakLever"`
	Blocked           [2]int                  `json:"blocked"`
	Connected         TuningConnectedUnits    `json:"connected"`
	Passed            []TuningIndexedUnit     `json:"passed,omitempty"`
	CandidatePassers  []TuningCandidatePasser `json:"candidatePassers,omitempty"`
}

// Side is +1 for White and -1 for Black. Targets are relative passed-pawn PSQT
// indices which remain structurally possible after the candidate captures.
type TuningCandidatePasser struct {
	Side    int   `json:"side"`
	Source  int   `json:"source"`
	Targets []int `json:"targets"`
}

type TuningPieceUnits struct {
	KnightOutpost      int `json:"knightOutpost"`
	KnightTropism      int `json:"knightTropism"`
	BishopOutpost      int `json:"bishopOutpost"`
	BadBishop          int `json:"badBishop"`
	BishopPair         int `json:"bishopPair"`
	RookSemiOpen       int `json:"rookSemiOpen"`
	RookOpen           int `json:"rookOpen"`
	RookFileCountOpen  int `json:"rookFileCountOpen"`
	RookFileCountSemi  int `json:"rookFileCountSemi"`
	RookStacked        int `json:"rookStacked"`
	RookSeventh        int `json:"rookSeventh"`
	QueenCentralized   int `json:"queenCentralized"`
	KingMinorDefenders int `json:"kingMinorDefenders"`
}

type TuningCenterUnits struct {
	Locked   bool `json:"locked"`
	Openness int  `json:"openness"` // -4 (closed) through +4 (open)
}

type TuningSpaceSide struct {
	Safe       int `json:"safe"`
	BehindPawn int `json:"behindPawn"`
	SemiOpen   int `json:"semiOpen"`
	Open       int `json:"open"`
	PieceCount int `json:"pieceCount"`
}

type TuningSpaceUnits struct {
	White        TuningSpaceSide `json:"white"`
	Black        TuningSpaceSide `json:"black"`
	BlockedPawns int             `json:"blockedPawns"`
}

// Table counts already carry the sign used by the white-minus-black evaluator.
type TuningShelterStormUnits struct {
	Shelter      [4][8]int `json:"shelter"`
	StormFree    [4][8]int `json:"stormFree"`
	StormBlocked [8]int    `json:"stormBlocked"`
}

// Attacker and SafeCheck use the fixed order knight, bishop, rook, queen.
type TuningDangerSide struct {
	Attackers    [4]int `json:"attackers"`
	RingHits     int    `json:"ringHits"`
	SafeChecks   [4]int `json:"safeChecks"`
	UnsafeChecks int    `json:"unsafeChecks"`
	HasQueen     bool   `json:"hasQueen"`
}

type TuningDangerUnits struct {
	White TuningDangerSide `json:"white"` // White attacks the black king
	Black TuningDangerSide `json:"black"` // Black attacks the white king
}

type TuningKingPasser struct {
	Side          int `json:"side"`
	RelativeRank  int `json:"relativeRank"`
	EnemyDistance int `json:"enemyDistance"`
	OwnDistance   int `json:"ownDistance"`
}

type TuningImbalanceUnits struct {
	TotalPawns int `json:"totalPawns"`
	KnightDiff int `json:"knightDiff"`
}

type TuningUnits struct {
	Material     [7]int                  `json:"material"`
	PSQT         []TuningPSQTUnit        `json:"psqt,omitempty"`
	Mobility     TuningMobilityUnits     `json:"mobility"`
	Pawn         TuningPawnUnits         `json:"pawn"`
	Piece        TuningPieceUnits        `json:"piece"`
	Center       TuningCenterUnits       `json:"center"`
	Space        TuningSpaceUnits        `json:"space"`
	ShelterStorm TuningShelterStormUnits `json:"shelterStorm"`
	Danger       TuningDangerUnits       `json:"danger"`
	KingPassers  []TuningKingPasser      `json:"kingPassers,omitempty"`
	Imbalance    TuningImbalanceUnits    `json:"imbalance"`
	Tempo        int                     `json:"tempo"`
}

type TuningReferenceTrace struct {
	Buckets          EvalPair `json:"buckets"`
	WhitePerspective int32    `json:"whitePerspective"`
	SideToMove       int32    `json:"sideToMove"`
}

// TuningTrace contains parameter-independent observations plus the small fixed
// (currently untuned) endgame contribution. Reference is a checksum only: a
// tuner forward pass must use Units rather than the current engine contribution.
type TuningTrace struct {
	SchemaVersion   int                  `json:"schemaVersion"`
	FEN             string               `json:"fen"`
	SideToMove      int                  `json:"sideToMove"` // +1 White, -1 Black
	PiecePhase      int                  `json:"piecePhase"`
	TotalPhase      int                  `json:"totalPhase"`
	TheoreticalDraw bool                 `json:"theoreticalDraw"`
	Fixed           EvalPair             `json:"fixed"`
	Units           TuningUnits          `json:"units"`
	Reference       TuningReferenceTrace `json:"reference"`
}

// TuningTraceForBoard builds the offline training record without changing the
// production Evaluation path or the pawn-hash layout.
func TuningTraceForBoard(b *gm.Board) TuningTrace {
	initVariables(b)
	_, eval := EvaluateWithTrace(b)
	entry := GetPawnEntry(b, false)

	units := TuningUnits{}
	units.Material, units.PSQT = tuningMaterialAndPSQT(b)
	units.Mobility = tuningMobility(eval)
	units.Pawn = tuningPawnUnits(b, entry)
	units.Piece = tuningPieceUnits(b, entry)
	units.Center = tuningCenterUnits(b, entry)
	units.Space = tuningSpaceUnits(b, entry)
	units.ShelterStorm = tuningShelterStormUnits(b, entry)
	units.Danger, units.Piece.KingMinorDefenders = tuningDangerUnits(b, entry)
	units.KingPassers = tuningKingPassers(b, entry)
	units.Imbalance = TuningImbalanceUnits{
		TotalPawns: bits.OnesCount64(b.White.Pawns | b.Black.Pawns),
		KnightDiff: bits.OnesCount64(b.White.Knights) - bits.OnesCount64(b.Black.Knights),
	}
	if b.Wtomove {
		units.Tempo = 1
	} else {
		units.Tempo = -1
	}

	fixed := EvalPair{
		EG: eval.King.Terms["centralizationPenalty"].EG + eval.King.Terms["mopUp"].EG,
	}
	side := 1
	if !b.Wtomove {
		side = -1
	}
	return TuningTrace{
		SchemaVersion: tuningTraceSchema,
		FEN:           b.ToFen(), SideToMove: side,
		PiecePhase: eval.Phase.PiecePhase, TotalPhase: eval.Phase.TotalPhase,
		TheoreticalDraw: eval.Draw.Theoretical,
		Fixed:           fixed, Units: units,
		Reference: TuningReferenceTrace{
			Buckets:          eval.Totals.Score,
			WhitePerspective: eval.Score.WhitePerspective,
			SideToMove:       eval.Score.SideToMove,
		},
	}
}

// RenderTuningTraceJSON writes one compact record suitable for JSONL datasets.
func RenderTuningTraceJSON(w io.Writer, trace TuningTrace) error {
	return json.NewEncoder(w).Encode(trace)
}

func tuningMaterialAndPSQT(b *gm.Board) ([7]int, []TuningPSQTUnit) {
	var material [7]int
	var byPieceSquare [7][64]int
	for pt := gm.PieceTypePawn; pt <= gm.PieceTypeKing; pt++ {
		white, black := tuningPieceBitboards(b, pt)
		if pt != gm.PieceTypeKing {
			material[pt] = bits.OnesCount64(white) - bits.OnesCount64(black)
		}
		for x := white; x != 0; x &= x - 1 {
			byPieceSquare[pt][bits.TrailingZeros64(x)]++
		}
		for x := black; x != 0; x &= x - 1 {
			byPieceSquare[pt][FlipView[bits.TrailingZeros64(x)]]--
		}
	}
	units := make([]TuningPSQTUnit, 0, 32)
	for pt := gm.PieceTypePawn; pt <= gm.PieceTypeKing; pt++ {
		for sq, n := range byPieceSquare[pt] {
			if n != 0 {
				units = append(units, TuningPSQTUnit{Piece: int(pt), Square: sq, Units: n})
			}
		}
	}
	return material, units
}

func tuningPieceBitboards(b *gm.Board, pt gm.PieceType) (uint64, uint64) {
	switch pt {
	case gm.PieceTypePawn:
		return b.White.Pawns, b.Black.Pawns
	case gm.PieceTypeKnight:
		return b.White.Knights, b.Black.Knights
	case gm.PieceTypeBishop:
		return b.White.Bishops, b.Black.Bishops
	case gm.PieceTypeRook:
		return b.White.Rooks, b.Black.Rooks
	case gm.PieceTypeQueen:
		return b.White.Queens, b.Black.Queens
	case gm.PieceTypeKing:
		return b.White.Kings, b.Black.Kings
	}
	return 0, 0
}

func tuningMobility(eval EvalTrace) TuningMobilityUnits {
	var out TuningMobilityUnits
	copy(out.Knight[:], eval.Knight.MobilityCounts)
	copy(out.Bishop[:], eval.Bishop.MobilityCounts)
	copy(out.Rook[:], eval.Rook.MobilityCounts)
	copy(out.Queen[:], eval.Queen.MobilityCounts)
	return out
}

func tuningPawnUnits(b *gm.Board, entry *PawnHashEntry) TuningPawnUnits {
	wDoubled, bDoubled := doubledPawnBitboards(b)
	out := TuningPawnUnits{
		IsolatedOpposed:   bits.OnesCount64(entry.BIsolatedBB&entry.BOpposedBB) - bits.OnesCount64(entry.WIsolatedBB&entry.WOpposedBB),
		IsolatedUnopposed: bits.OnesCount64(entry.BIsolatedBB&^entry.BOpposedBB) - bits.OnesCount64(entry.WIsolatedBB&^entry.WOpposedBB),
		DoubledOpposed:    bits.OnesCount64(bDoubled&entry.BOpposedBB) - bits.OnesCount64(wDoubled&entry.WOpposedBB),
		DoubledUnopposed:  bits.OnesCount64(bDoubled&^entry.BOpposedBB) - bits.OnesCount64(wDoubled&^entry.WOpposedBB),
		BackwardOpposed:   bits.OnesCount64(entry.BBackwardBB&entry.BOpposedBB) - bits.OnesCount64(entry.WBackwardBB&entry.WOpposedBB),
		BackwardUnopposed: bits.OnesCount64(entry.BBackwardBB&^entry.BOpposedBB) - bits.OnesCount64(entry.WBackwardBB&^entry.WOpposedBB),
		WeakLever:         bits.OnesCount64(entry.BWeakLeverBB) - bits.OnesCount64(entry.WWeakLeverBB),
	}
	for i := 0; i < 2; i++ {
		out.Blocked[i] = bits.OnesCount64(entry.WBlockedBB&onlyRank[4+i]) - bits.OnesCount64(entry.BBlockedBB&onlyRank[3-i])
	}
	out.Connected.White = tuningConnectedFor(b.White.Pawns, entry.WPawnAttackBB, entry.WOpposedBB, true)
	out.Connected.Black = tuningConnectedFor(b.Black.Pawns, entry.BPawnAttackBB, entry.BOpposedBB, false)
	out.Passed = tuningIndexedBitboard(entry.WPassedBB, entry.BPassedBB)
	out.CandidatePassers = tuningCandidatePassers(b, entry)
	return out
}

func tuningConnectedFor(own, ownAttacks, opposed uint64, white bool) (out [7]int) {
	phalanx := phalanxPawns(own)
	qualifying := phalanx | (own & ownAttacks)
	for r := 1; r < 7; r++ {
		mask := onlyRank[r]
		if !white {
			mask = onlyRank[7-r]
		}
		n := bits.OnesCount64(qualifying & mask)
		out[r] = 2*n + bits.OnesCount64(phalanx&mask) - bits.OnesCount64(qualifying&opposed&mask)
	}
	return out
}

func tuningIndexedBitboard(white, black uint64) []TuningIndexedUnit {
	byIndex := [64]int{}
	for x := white; x != 0; x &= x - 1 {
		byIndex[bits.TrailingZeros64(x)]++
	}
	for x := black; x != 0; x &= x - 1 {
		byIndex[FlipView[bits.TrailingZeros64(x)]]--
	}
	out := make([]TuningIndexedUnit, 0, bits.OnesCount64(white|black))
	for i, n := range byIndex {
		if n != 0 {
			out = append(out, TuningIndexedUnit{Index: i, Units: n})
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func tuningCandidatePassers(b *gm.Board, entry *PawnHashEntry) []TuningCandidatePasser {
	wPush, bPush := LeverPushBitboards(b)
	occ := b.White.All | b.Black.All
	out := make([]TuningCandidatePasser, 0, 8)
	appendSide := func(sources, lever, push, passed, enemyPawns uint64, white bool) {
		for x := sources &^ passed; x != 0; x &= x - 1 {
			sq := bits.TrailingZeros64(x)
			pawnBB := PositionBB[sq]
			origins := pawnBB & lever
			if pawnBB&push != 0 {
				frontSq := sq - 8
				if white {
					frontSq = sq + 8
				}
				if frontSq >= 0 && frontSq < 64 && PositionBB[frontSq]&occ == 0 {
					origins |= PositionBB[frontSq]
				}
			}
			var targetMask uint64
			for y := origins; y != 0; y &= y - 1 {
				from := bits.TrailingZeros64(y)
				e, w := PawnCaptureBitboards(PositionBB[from], white)
				for targets := (e | w) & enemyPawns; targets != 0; targets &= targets - 1 {
					capSq := bits.TrailingZeros64(targets)
					remaining := enemyPawns &^ PositionBB[capSq]
					valid := remaining&PassedMaskBlack[capSq] == 0
					rel := FlipView[capSq]
					if white {
						valid = remaining&PassedMaskWhite[capSq] == 0
						rel = capSq
					}
					if valid {
						targetMask |= PositionBB[rel]
					}
				}
			}
			if targetMask == 0 {
				continue
			}
			targets := make([]int, 0, bits.OnesCount64(targetMask))
			for y := targetMask; y != 0; y &= y - 1 {
				targets = append(targets, bits.TrailingZeros64(y))
			}
			side, source := -1, FlipView[sq]
			if white {
				side, source = 1, sq
			}
			out = append(out, TuningCandidatePasser{Side: side, Source: source, Targets: targets})
		}
	}
	appendSide(entry.WLeverBB|wPush, entry.WLeverBB, wPush, entry.WPassedBB, b.Black.Pawns, true)
	appendSide(entry.BLeverBB|bPush, entry.BLeverBB, bPush, entry.BPassedBB, b.White.Pawns, false)
	return out
}

func tuningPieceUnits(b *gm.Board, entry *PawnHashEntry) TuningPieceUnits {
	outposts := getOutpostsBB(b, entry.WPawnAttackBB, entry.BPawnAttackBB)
	wStack, bStack := getRookConnectedFiles(b)
	openCount := bits.OnesCount64(entry.OpenFiles) / 8
	wSemiCount := bits.OnesCount64(entry.WSemiOpenFiles) / 8
	bSemiCount := bits.OnesCount64(entry.BSemiOpenFiles) / 8
	wRooks := bits.OnesCount64(b.White.Rooks)
	bRooks := bits.OnesCount64(b.Black.Rooks)
	wSeventh := bits.OnesCount64(b.White.Rooks & seventhRankMask)
	bSecond := bits.OnesCount64(b.Black.Rooks & secondRankMask)
	seventh := wSeventh - bSecond
	if wSeventh >= 2 {
		seventh += 2
	}
	if bSecond >= 2 {
		seventh -= 2
	}
	return TuningPieceUnits{
		KnightOutpost:     bits.OnesCount64(b.White.Knights&outposts[0]) - bits.OnesCount64(b.Black.Knights&outposts[1]),
		KnightTropism:     tuningKnightTropismUnits(b),
		BishopOutpost:     bits.OnesCount64(b.White.Bishops&outposts[0]) - bits.OnesCount64(b.Black.Bishops&outposts[1]),
		BadBishop:         tuningBadBishopUnits(b, entry),
		BishopPair:        tuningBishopPairUnit(b),
		RookSemiOpen:      bits.OnesCount64(entry.WSemiOpenFiles&b.White.Rooks) - bits.OnesCount64(entry.BSemiOpenFiles&b.Black.Rooks),
		RookOpen:          bits.OnesCount64(entry.OpenFiles&b.White.Rooks) - bits.OnesCount64(entry.OpenFiles&b.Black.Rooks),
		RookFileCountOpen: openCount * (wRooks - bRooks),
		RookFileCountSemi: wRooks*wSemiCount - bRooks*bSemiCount,
		RookStacked:       bits.OnesCount64(wStack)/8 - bits.OnesCount64(bStack)/8,
		RookSeventh:       seventh,
		QueenCentralized:  boolUnit(b.White.Queens&centralizedQueenSquares != 0) - boolUnit(b.Black.Queens&centralizedQueenSquares != 0),
	}
}

func tuningKnightTropismUnits(b *gm.Board) int {
	wKing := bits.TrailingZeros64(b.White.Kings)
	bKing := bits.TrailingZeros64(b.Black.Kings)
	units := 0
	for x := b.White.Knights; x != 0; x &= x - 1 {
		if d := chebyshevDistance(bits.TrailingZeros64(x), bKing); d <= 6 {
			units += 7 - d
		}
	}
	for x := b.Black.Knights; x != 0; x &= x - 1 {
		if d := chebyshevDistance(bits.TrailingZeros64(x), wKing); d <= 6 {
			units -= 7 - d
		}
	}
	return units
}

func tuningBadBishopUnits(b *gm.Board, entry *PawnHashEntry) int {
	wLight := bits.OnesCount64(entry.WBlockedBB & lightSquares)
	wDark := bits.OnesCount64(entry.WBlockedBB & darkSquares)
	bLight := bits.OnesCount64(entry.BBlockedBB & lightSquares)
	bDark := bits.OnesCount64(entry.BBlockedBB & darkSquares)
	units := 0
	for x := b.White.Bishops; x != 0; x &= x - 1 {
		if isDarkSquare(bits.TrailingZeros64(x)) {
			units += wDark
		} else {
			units += wLight
		}
	}
	for x := b.Black.Bishops; x != 0; x &= x - 1 {
		if isDarkSquare(bits.TrailingZeros64(x)) {
			units -= bDark
		} else {
			units -= bLight
		}
	}
	return units
}

func tuningBishopPairUnit(b *gm.Board) int {
	w := bits.OnesCount64(b.White.Bishops) > 1
	bl := bits.OnesCount64(b.Black.Bishops) > 1
	if w && !bl {
		return 1
	}
	if bl && !w {
		return -1
	}
	return 0
}

func boolUnit(v bool) int {
	if v {
		return 1
	}
	return 0
}

func tuningCenterUnits(b *gm.Board, entry *PawnHashEntry) TuningCenterUnits {
	locked, idx := getCenterState(b, entry.OpenFiles, entry.WSemiOpenFiles, entry.BSemiOpenFiles, entry.WLeverBB, entry.BLeverBB)
	openness := int(idx*8+0.5) - 4
	if locked {
		openness = -4
	}
	return TuningCenterUnits{Locked: locked, Openness: openness}
}

func tuningSpaceUnits(b *gm.Board, entry *PawnHashEntry) TuningSpaceUnits {
	forSide := func(ownPawns, enemyAttacks, zone, semi, open uint64, white bool, pieceCount int) TuningSpaceSide {
		safe := zone &^ ownPawns &^ enemyAttacks
		behind := ownPawns
		if white {
			behind |= behind >> 8
			behind |= behind >> 16
		} else {
			behind |= behind << 8
			behind |= behind << 16
		}
		return TuningSpaceSide{
			Safe: bits.OnesCount64(safe), BehindPawn: bits.OnesCount64(safe & behind),
			SemiOpen: bits.OnesCount64(safe & semi), Open: bits.OnesCount64(safe & open), PieceCount: pieceCount,
		}
	}
	return TuningSpaceUnits{
		White:        forSide(b.White.Pawns, entry.BPawnAttackBB, wSpaceZoneMask, entry.WSemiOpenFiles, entry.OpenFiles, true, bits.OnesCount64(b.White.All)),
		Black:        forSide(b.Black.Pawns, entry.WPawnAttackBB, bSpaceZoneMask, entry.BSemiOpenFiles, entry.OpenFiles, false, bits.OnesCount64(b.Black.All)),
		BlockedPawns: bits.OnesCount64(entry.WBlockedBB) + bits.OnesCount64(entry.BBlockedBB),
	}
}

func tuningShelterStormUnits(b *gm.Board, entry *PawnHashEntry) TuningShelterStormUnits {
	var out TuningShelterStormUnits
	addSide := func(white bool, enemyAttacks uint64) {
		var kings, ours, theirs uint64
		shelterSign, stormSign := -1, 1
		if white {
			kings, ours, theirs = b.White.Kings, b.White.Pawns, b.Black.Pawns
			shelterSign, stormSign = 1, -1
		} else {
			kings, ours, theirs = b.Black.Kings, b.Black.Pawns, b.White.Pawns
		}
		kingSq := bits.TrailingZeros64(kings)
		kingRank, kingFile := kingSq/8, kingSq%8
		shelter := ours &^ enemyAttacks
		if white {
			shelter &= ranksAbove[kingRank]
			theirs &= ranksAbove[kingRank]
		} else {
			shelter &= ranksBelow[kingRank]
			theirs &= ranksBelow[kingRank]
		}
		center := min(max(kingFile, 1), 6)
		for f := center - 1; f <= center+1; f++ {
			edge := min(f, 7-f)
			ourRank := frontmostRelRank(shelter&onlyFile[f], white)
			theirRank := frontmostRelRank(theirs&onlyFile[f], white)
			out.Shelter[edge][ourRank] += shelterSign
			if ourRank != 0 && ourRank == theirRank-1 {
				out.StormBlocked[theirRank] += stormSign
			} else {
				out.StormFree[edge][theirRank] += stormSign
			}
		}
	}
	addSide(true, entry.BPawnAttackBB)
	addSide(false, entry.WPawnAttackBB)
	return out
}

func tuningDangerUnits(b *gm.Board, entry *PawnHashEntry) (TuningDangerUnits, int) {
	ring := getKingSafetyTable(b, true, 0, 0)
	all := b.White.All | b.Black.All
	var attacks [2][4]uint64
	var rookTrue [2]uint64
	var out TuningDangerUnits
	sides := [2]*TuningDangerSide{&out.White, &out.Black}
	for side := 0; side < 2; side++ {
		white := side == 0
		knights, bishops, rooks, queens := b.White.Knights, b.White.Bishops, b.White.Rooks, b.White.Queens
		if !white {
			knights, bishops, rooks, queens = b.Black.Knights, b.Black.Bishops, b.Black.Rooks, b.Black.Queens
		}
		sides[side].HasQueen = queens != 0
		for x := knights; x != 0; x &= x - 1 {
			a := KnightMasks[bits.TrailingZeros64(x)]
			attacks[side][0] |= a
			tuningAddDangerAttacker(sides[side], 0, a, ring[1-side])
		}
		for x := bishops; x != 0; x &= x - 1 {
			sq := bits.TrailingZeros64(x)
			a := gm.CalculateBishopMoveBitboard(uint8(sq), all&^PositionBB[sq])
			attacks[side][1] |= a
			tuningAddDangerAttacker(sides[side], 1, a, ring[1-side])
		}
		for x := rooks; x != 0; x &= x - 1 {
			sq := bits.TrailingZeros64(x)
			pressure := gm.CalculateRookMoveBitboard(uint8(sq), rookAttackOccupancy(b, white))
			tuningAddDangerAttacker(sides[side], 2, pressure, ring[1-side])
			rookTrue[side] |= gm.CalculateRookMoveBitboard(uint8(sq), all&^PositionBB[sq])
		}
		for x := queens; x != 0; x &= x - 1 {
			sq := bits.TrailingZeros64(x)
			occ := all &^ PositionBB[sq]
			a := gm.CalculateBishopMoveBitboard(uint8(sq), occ) | gm.CalculateRookMoveBitboard(uint8(sq), occ)
			attacks[side][3] |= a
			tuningAddDangerAttacker(sides[side], 3, a, ring[1-side])
		}
	}
	pawnAttacks := [2]uint64{entry.WPawnAttackBB, entry.BPawnAttackBB}
	allBySide := [2]uint64{b.White.All, b.Black.All}
	kings := [2]uint64{b.White.Kings, b.Black.Kings}
	for def := 0; def < 2; def++ {
		atk := 1 - def
		kingSq := bits.TrailingZeros64(kings[def])
		knightCk := KnightMasks[kingSq]
		bishopCk := gm.CalculateBishopMoveBitboard(uint8(kingSq), all)
		rookCk := gm.CalculateRookMoveBitboard(uint8(kingSq), all)
		defended := pawnAttacks[def] | attacks[def][0] | attacks[def][1] | rookTrue[def]
		var unsafe uint64
		fold := func(kind int, reach, mask uint64) {
			landing := reach & mask &^ allBySide[atk]
			safe := landing &^ defended
			sides[atk].SafeChecks[kind] += bits.OnesCount64(safe)
			unsafe |= landing & defended
		}
		fold(0, attacks[atk][0], knightCk)
		fold(1, attacks[atk][1], bishopCk)
		fold(2, rookTrue[atk], rookCk)
		fold(3, attacks[atk][3], bishopCk|rookCk)
		sides[atk].UnsafeChecks = bits.OnesCount64(unsafe)
	}
	minor := bits.OnesCount64(ring[0]&(attacks[0][0]|attacks[0][1])) - bits.OnesCount64(ring[1]&(attacks[1][0]|attacks[1][1]))
	return out, minor
}

func tuningAddDangerAttacker(side *TuningDangerSide, kind int, attacks, ring uint64) {
	hits := bits.OnesCount64(attacks & ring)
	if hits == 0 {
		return
	}
	side.Attackers[kind]++
	side.RingHits += hits
}

func tuningKingPassers(b *gm.Board, entry *PawnHashEntry) []TuningKingPasser {
	wKing := bits.TrailingZeros64(b.White.Kings)
	bKing := bits.TrailingZeros64(b.Black.Kings)
	out := make([]TuningKingPasser, 0, bits.OnesCount64(entry.WPassedBB|entry.BPassedBB))
	for x := entry.WPassedBB; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		rank := sq / 8
		if rank >= 3 {
			block := sq + 8
			out = append(out, TuningKingPasser{Side: 1, RelativeRank: rank, EnemyDistance: chebyshevDistance(block, bKing), OwnDistance: chebyshevDistance(block, wKing)})
		}
	}
	for x := entry.BPassedBB; x != 0; x &= x - 1 {
		sq := bits.TrailingZeros64(x)
		rank := 7 - sq/8
		if rank >= 3 {
			block := sq - 8
			out = append(out, TuningKingPasser{Side: -1, RelativeRank: rank, EnemyDistance: chebyshevDistance(block, wKing), OwnDistance: chebyshevDistance(block, bKing)})
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// CurrentTuningPieceValues exposes the live material arrays to the tuner
// registry. The arrays are otherwise package-private because search and move
// ordering also consume them.
func CurrentTuningPieceValues() (mg, eg [7]int) {
	return pieceValueMG, pieceValueEG
}

// ScoreTuningTraceCurrent applies the engine's current integer parameters to a
// tuning record. It is primarily a parity oracle for the future floating-point
// tuner and for detecting extractor drift after evaluation changes.
func ScoreTuningTraceCurrent(trace TuningTrace) (int32, EvalPair) {
	u := trace.Units
	mg, eg := trace.Fixed.MG, trace.Fixed.EG
	for pt, n := range u.Material {
		mg += n * pieceValueMG[pt]
		eg += n * pieceValueEG[pt]
	}
	for _, x := range u.PSQT {
		mg += x.Units * PSQT_MG[x.Piece][x.Square]
		eg += x.Units * PSQT_EG[x.Piece][x.Square]
	}
	for _, x := range u.Pawn.Passed {
		mg += x.Units * PassedPawnPSQT_MG[x.Index]
		eg += x.Units * PassedPawnPSQT_EG[x.Index]
	}
	mg += u.Pawn.IsolatedOpposed*IsolatedOpposedMG + u.Pawn.IsolatedUnopposed*IsolatedUnopposedMG
	eg += u.Pawn.IsolatedOpposed*IsolatedOpposedEG + u.Pawn.IsolatedUnopposed*IsolatedUnopposedEG
	mg += u.Pawn.DoubledOpposed*PawnDoubledOpposedMG + u.Pawn.DoubledUnopposed*PawnDoubledUnopposedMG
	eg += u.Pawn.DoubledOpposed*PawnDoubledOpposedEG + u.Pawn.DoubledUnopposed*PawnDoubledUnopposedEG
	mg += u.Pawn.BackwardOpposed*BackwardOpposedMG + u.Pawn.BackwardUnopposed*BackwardUnopposedMG
	eg += u.Pawn.BackwardOpposed*BackwardOpposedEG + u.Pawn.BackwardUnopposed*BackwardUnopposedEG
	mg += u.Pawn.WeakLever * PawnWeakLeverMG
	eg += u.Pawn.WeakLever * PawnWeakLeverEG
	for i, n := range u.Pawn.Blocked {
		mg += n * PawnBlockedMG[i]
		eg += n * PawnBlockedEG[i]
	}
	for r := 1; r < 7; r++ {
		w := PawnConnectedMG[r] * u.Pawn.Connected.White[r]
		b := PawnConnectedMG[r] * u.Pawn.Connected.Black[r]
		mg += w - b
		eg += w*(r-2)/4 - b*(r-2)/4
	}
	for _, c := range u.Pawn.CandidatePassers {
		bestMG, bestEG := 0, 0
		for _, sq := range c.Targets {
			bestMG = max(bestMG, PassedPawnPSQT_MG[sq]*CandidatePassedPctMG/100)
			bestEG = max(bestEG, PassedPawnPSQT_EG[sq]*CandidatePassedPctEG/100)
		}
		mg += c.Side * bestMG
		eg += c.Side * bestEG
	}
	center := u.Center.Openness
	nScaleMG := 100 - center*CenterKnightMobilityMG/4
	nScaleEG := 100 - center*CenterKnightMobilityEG/4
	bScaleMG := 100 + center*CenterBishopMobilityMG/4
	bScaleEG := 100 + center*CenterBishopMobilityEG/4
	pScaleMG := 100 + center*CenterBishopPairMG/4
	pScaleEG := 100 + center*CenterBishopPairEG/4
	nMG, nEG := dot(u.Mobility.Knight[:], KnightMobilityMG[:]), dot(u.Mobility.Knight[:], KnightMobilityEG[:])
	mg += nMG * nScaleMG / 100
	eg += nEG * nScaleEG / 100
	bMG, bEG := dot(u.Mobility.Bishop[:], BishopMobilityMG[:]), dot(u.Mobility.Bishop[:], BishopMobilityEG[:])
	mg += bMG * bScaleMG / 100
	eg += bEG * bScaleEG / 100
	rMG, rEG := dot(u.Mobility.Rook[:], RookMobilityMG[:]), dot(u.Mobility.Rook[:], RookMobilityEG[:])
	mg += rMG
	eg += rEG
	qMG, qEG := dot(u.Mobility.Queen[:], QueenMobilityMG[:]), dot(u.Mobility.Queen[:], QueenMobilityEG[:])
	mg += qMG
	eg += qEG
	mg += u.Piece.KnightOutpost*KnightOutpostMG + u.Piece.KnightTropism*KnightTropismMG
	eg += u.Piece.KnightOutpost*KnightOutpostEG + u.Piece.KnightTropism*KnightTropismEG
	mg += u.Piece.BishopOutpost*BishopOutpostMG + u.Piece.BadBishop*BadBishopMG
	eg += u.Piece.BishopOutpost*BishopOutpostEG + u.Piece.BadBishop*BadBishopEG
	mg += (u.Piece.BishopPair * BishopPairBonusMG) * pScaleMG / 100
	eg += (u.Piece.BishopPair * BishopPairBonusEG) * pScaleEG / 100
	mg += u.Piece.RookSemiOpen*RookSemiOpenMG + u.Piece.RookOpen*RookOpenMG + u.Piece.RookFileCountOpen*RookFileCountOpenMG + u.Piece.RookFileCountSemi*RookFileCountSemiMG + u.Piece.RookStacked*RookStackedMG + u.Piece.RookSeventh*RookSeventhRankMG
	eg += u.Piece.RookSemiOpen*RookSemiOpenEG + u.Piece.RookOpen*RookOpenEG + u.Piece.RookFileCountOpen*RookFileCountOpenEG + u.Piece.RookFileCountSemi*RookFileCountSemiEG + u.Piece.RookSeventh*RookSeventhRankEG
	eg += u.Piece.QueenCentralized * QueenCentralizationEG
	mg += u.Piece.KingMinorDefenders * KingMinorDefenseBonusMG
	for i := range u.ShelterStorm.Shelter {
		for j, n := range u.ShelterStorm.Shelter[i] {
			mg += n * KingShelterMG[i][j]
		}
	}
	for i := range u.ShelterStorm.StormFree {
		for j, n := range u.ShelterStorm.StormFree[i] {
			mg += n * KingStormUnblockedMG[i][j]
		}
	}
	for i, n := range u.ShelterStorm.StormBlocked {
		mg += n * KingStormBlockedMG[i]
		eg += n * KingStormBlockedEG[i]
	}
	dwMG, dwEG := tuningDangerScore(u.Danger.White)
	dbMG, dbEG := tuningDangerScore(u.Danger.Black)
	mg += dwMG - dbMG
	eg += dwEG - dbEG
	for _, p := range u.KingPassers {
		delta := p.EnemyDistance*KingPasserEnemyWeight - p.OwnDistance*KingPasserOwnWeight
		eg += p.Side * (delta * p.RelativeRank * p.RelativeRank * KingPasserProximityEG) / KingPasserProximityDiv
	}
	blocked := u.Space.BlockedPawns
	if blocked > SpaceBlockedCap {
		blocked = SpaceBlockedCap
	}
	wRaw := tuningSpaceRaw(u.Space.White)
	bRaw := tuningSpaceRaw(u.Space.Black)
	ww := max(0, u.Space.White.PieceCount-SpaceWeightOffset+blocked)
	bw := max(0, u.Space.Black.PieceCount-SpaceWeightOffset+blocked)
	mg += (wRaw*ww*ww - bRaw*bw*bw) / SpaceWeightDiv
	imbUnits := (u.Imbalance.TotalPawns - ImbalanceRefPawnCount) * u.Imbalance.KnightDiff
	mg += imbUnits * ImbalanceKnightPerPawnMG
	eg += imbUnits * ImbalanceKnightPerPawnEG
	mg += u.Tempo * TempoBonus
	eg += u.Tempo * TempoBonus
	buckets := EvalPair{MG: mg, EG: eg}
	score := int32((mg*trace.PiecePhase + eg*(trace.TotalPhase-trace.PiecePhase)) / trace.TotalPhase)
	if trace.TheoreticalDraw {
		score /= DrawDivider
	}
	score *= int32(trace.SideToMove)
	return score, buckets
}

func dot(units, values []int) int {
	total := 0
	for i, n := range units {
		total += n * values[i]
	}
	return total
}

func tuningDangerScore(s TuningDangerSide) (int, int) {
	weightsMG := [4]int{SafetyKnightWeightMG, SafetyBishopWeightMG, SafetyRookWeightMG, SafetyQueenWeightMG}
	weightsEG := [4]int{SafetyKnightWeightEG, SafetyBishopWeightEG, SafetyRookWeightEG, SafetyQueenWeightEG}
	checksMG := [4]int{SafetySafeKnightCheckMG, SafetySafeBishopCheckMG, SafetySafeRookCheckMG, SafetySafeQueenCheckMG}
	checksEG := [4]int{SafetySafeKnightCheckEG, SafetySafeBishopCheckEG, SafetySafeRookCheckEG, SafetySafeQueenCheckEG}
	rawMG, rawEG := SafetyAdjustmentMG+s.RingHits*SafetyAttackValueMG, SafetyAdjustmentEG+s.RingHits*SafetyAttackValueEG
	for i := 0; i < 4; i++ {
		rawMG += s.Attackers[i]*weightsMG[i] + s.SafeChecks[i]*checksMG[i]
		rawEG += s.Attackers[i]*weightsEG[i] + s.SafeChecks[i]*checksEG[i]
	}
	rawMG += s.UnsafeChecks * SafetyUnsafeCheckMG
	rawEG += s.UnsafeChecks * SafetyUnsafeCheckEG
	if !s.HasQueen {
		rawMG += SafetyNoEnemyQueensMG
		rawEG += SafetyNoEnemyQueensEG
	}
	rawMG = max(0, rawMG)
	rawEG = max(0, rawEG)
	return rawMG * rawMG / (SafetyMGDivisor * SafetyMGDivisor), rawEG / SafetyEGDivisor
}

func tuningSpaceRaw(s TuningSpaceSide) int {
	return s.Safe*SpaceSafeMG + s.BehindPawn*SpaceBehindPawnMG + s.SemiOpen*SpaceSemiOpenMG + s.Open*SpaceOpenMG
}
