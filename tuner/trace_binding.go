package tuner

import (
	"fmt"
	"sort"

	eng "chess-engine/engine"
	gm "chess-engine/goosemg"
)

// PhaseHandles binds the middlegame and endgame halves of one semantic term.
type PhaseHandles struct {
	MG ParameterHandle
	EG ParameterHandle
}

type MobilityBindings struct {
	Knight PhaseHandles
	Bishop PhaseHandles
	Rook   PhaseHandles
	Queen  PhaseHandles
}

type PawnBindings struct {
	IsolatedOpposed   PhaseHandles
	IsolatedUnopposed PhaseHandles
	DoubledOpposed    PhaseHandles
	DoubledUnopposed  PhaseHandles
	BackwardOpposed   PhaseHandles
	BackwardUnopposed PhaseHandles
	WeakLever         PhaseHandles
	Blocked           PhaseHandles
	Connected         ParameterHandle
	Passed            PhaseHandles
	CandidatePct      PhaseHandles
}

type PieceBindings struct {
	KnightOutpost     PhaseHandles
	KnightTropism     PhaseHandles
	BishopOutpost     PhaseHandles
	BadBishop         PhaseHandles
	BishopPair        PhaseHandles
	RookSemiOpen      PhaseHandles
	RookOpen          PhaseHandles
	RookFileCountOpen PhaseHandles
	RookFileCountSemi PhaseHandles
	RookSeventh       PhaseHandles
	RookStackedMG     ParameterHandle
	QueenCentralEG    ParameterHandle
	KingMinorMG       ParameterHandle
}

type CenterBindings struct {
	KnightMobilityPct PhaseHandles
	BishopMobilityPct PhaseHandles
	BishopPairPct     PhaseHandles
}

type ShelterStormBindings struct {
	ShelterMG      ParameterHandle
	StormFreeMG    ParameterHandle
	StormBlockedMG ParameterHandle
	StormBlockedEG ParameterHandle
}

type DangerBindings struct {
	AttackerWeight [4]PhaseHandles
	AttackValue    PhaseHandles
	NoEnemyQueen   PhaseHandles
	SafeCheck      [4]PhaseHandles
	UnsafeCheck    PhaseHandles
	Adjustment     PhaseHandles
	Divisor        PhaseHandles
}

type KingPasserBindings struct {
	ProximityEG ParameterHandle
	Divisor     ParameterHandle
	EnemyWeight ParameterHandle
	OwnWeight   ParameterHandle
}

type SpaceBindings struct {
	SafeMG        ParameterHandle
	BehindPawnMG  ParameterHandle
	SemiOpenMG    ParameterHandle
	OpenMG        ParameterHandle
	WeightOffset  ParameterHandle
	BlockedCap    ParameterHandle
	WeightDivisor ParameterHandle
}

type ImbalanceBindings struct {
	ReferencePawnCount ParameterHandle
	KnightPerPawn      PhaseHandles
}

// TraceBinding is the initialization-time connection between trace semantics
// and direct registry-vector handles. Construction fails unless every registry
// spec and every emitted trace field has an explicit owner.
type TraceBinding struct {
	RegistryFingerprint string

	Material    [7]PhaseHandles
	PSQT        [7]PhaseHandles
	Mobility    MobilityBindings
	Pawn        PawnBindings
	Piece       PieceBindings
	Center      CenterBindings
	Shelter     ShelterStormBindings
	Danger      DangerBindings
	KingPasser  KingPasserBindings
	Space       SpaceBindings
	Imbalance   ImbalanceBindings
	Tempo       ParameterHandle
	DrawDivider ParameterHandle

	fieldUses []TraceFieldBinding
}

// NewTraceBinding resolves all semantic IDs once and validates complete
// registry/schema coverage.
func NewTraceBinding(registry *Registry) (*TraceBinding, error) {
	if registry == nil {
		return nil, fmt.Errorf("trace binding requires a registry")
	}
	builder := traceBindingBuilder{
		registry: registry,
		consumed: make([]bool, len(registry.Specs)),
	}
	out := &TraceBinding{RegistryFingerprint: registry.Fingerprint}

	pieces := activePieces()
	for _, piece := range pieces[:5] {
		out.Material[piece.kind] = builder.phase("material."+piece.id, FormulaLinear)
	}
	for _, piece := range pieces {
		out.PSQT[piece.kind] = builder.phase("psqt."+piece.id, FormulaLinear)
	}
	out.Pawn.Passed = builder.phase("pawn.passed.psqt", FormulaLinear)

	out.Mobility.Knight = builder.phase("mobility.knight", FormulaCenterScale)
	out.Mobility.Bishop = builder.phase("mobility.bishop", FormulaCenterScale)
	out.Mobility.Rook = builder.phase("mobility.rook", FormulaLinear)
	out.Mobility.Queen = builder.phase("mobility.queen", FormulaLinear)

	out.Pawn.IsolatedOpposed = builder.phase("pawn.isolated.opposed", FormulaLinear)
	out.Pawn.IsolatedUnopposed = builder.phase("pawn.isolated.unopposed", FormulaLinear)
	out.Pawn.DoubledOpposed = builder.phase("pawn.doubled.opposed", FormulaLinear)
	out.Pawn.DoubledUnopposed = builder.phase("pawn.doubled.unopposed", FormulaLinear)
	out.Pawn.BackwardOpposed = builder.phase("pawn.backward.opposed", FormulaLinear)
	out.Pawn.BackwardUnopposed = builder.phase("pawn.backward.unopposed", FormulaLinear)
	out.Pawn.WeakLever = builder.phase("pawn.weak_lever", FormulaLinear)
	out.Pawn.Blocked = builder.phase("pawn.blocked", FormulaLinear)
	out.Pawn.Connected = builder.parameter("pawn.connected.mg", FormulaConnectedPawn)
	out.Pawn.CandidatePct = builder.phase("pawn.candidate_passed_pct", FormulaCandidatePasser)

	out.Center.KnightMobilityPct = builder.phase("center.knight_mobility_pct", FormulaCenterScale)
	out.Center.BishopMobilityPct = builder.phase("center.bishop_mobility_pct", FormulaCenterScale)
	out.Center.BishopPairPct = builder.phase("center.bishop_pair_pct", FormulaCenterScale)

	out.Piece.KnightOutpost = builder.phase("knight.outpost", FormulaLinear)
	out.Piece.KnightTropism = builder.phase("knight.tropism", FormulaLinear)
	out.Piece.BishopOutpost = builder.phase("bishop.outpost", FormulaLinear)
	out.Piece.BadBishop = builder.phase("bishop.bad", FormulaLinear)
	out.Piece.BishopPair = builder.phase("bishop.pair", FormulaCenterScale)
	out.Piece.RookSemiOpen = builder.phase("rook.semi_open", FormulaLinear)
	out.Piece.RookOpen = builder.phase("rook.open", FormulaLinear)
	out.Piece.RookFileCountOpen = builder.phase("rook.file_count.open", FormulaLinear)
	out.Piece.RookFileCountSemi = builder.phase("rook.file_count.semi_open", FormulaLinear)
	out.Piece.RookSeventh = builder.phase("rook.seventh_rank", FormulaLinear)
	out.Piece.RookStackedMG = builder.parameter("rook.stacked.mg", FormulaLinear)
	out.Piece.QueenCentralEG = builder.parameter("queen.centralization.eg", FormulaLinear)
	out.Piece.KingMinorMG = builder.parameter("king.minor_defense.mg", FormulaLinear)

	out.Shelter.ShelterMG = builder.parameter("king.shelter.mg", FormulaLinear)
	out.Shelter.StormFreeMG = builder.parameter("king.storm.unblocked.mg", FormulaLinear)
	out.Shelter.StormBlockedMG = builder.parameter("king.storm.blocked.mg", FormulaLinear)
	out.Shelter.StormBlockedEG = builder.parameter("king.storm.blocked.eg", FormulaLinear)

	dangerKinds := []string{"knight", "bishop", "rook", "queen"}
	for i, kind := range dangerKinds {
		out.Danger.AttackerWeight[i] = builder.phase("king.danger.attacker."+kind, FormulaKingDanger)
		out.Danger.SafeCheck[i] = builder.phase("king.danger.safe_check."+kind, FormulaKingDanger)
	}
	out.Danger.AttackValue = builder.phase("king.danger.ring_attack", FormulaKingDanger)
	out.Danger.NoEnemyQueen = builder.phase("king.danger.no_enemy_queen", FormulaKingDanger)
	out.Danger.UnsafeCheck = builder.phase("king.danger.unsafe_check", FormulaKingDanger)
	out.Danger.Adjustment = builder.phase("king.danger.adjustment", FormulaKingDanger)
	out.Danger.Divisor = PhaseHandles{
		MG: builder.parameter("king.danger.divisor.mg", FormulaKingDanger),
		EG: builder.parameter("king.danger.divisor.eg", FormulaKingDanger),
	}

	out.KingPasser.ProximityEG = builder.parameter("king.passer.proximity.eg", FormulaKingPasser)
	out.KingPasser.Divisor = builder.parameter("king.passer.divisor", FormulaKingPasser)
	out.KingPasser.EnemyWeight = builder.parameter("king.passer.enemy_weight", FormulaKingPasser)
	out.KingPasser.OwnWeight = builder.parameter("king.passer.own_weight", FormulaKingPasser)

	out.Space.SafeMG = builder.parameter("space.safe.mg", FormulaSpace)
	out.Space.BehindPawnMG = builder.parameter("space.behind_pawn.mg", FormulaSpace)
	out.Space.SemiOpenMG = builder.parameter("space.semi_open.mg", FormulaSpace)
	out.Space.OpenMG = builder.parameter("space.open.mg", FormulaSpace)
	out.Space.WeightOffset = builder.parameter("space.weight_offset", FormulaSpace)
	out.Space.BlockedCap = builder.parameter("space.blocked_cap", FormulaSpace)
	out.Space.WeightDivisor = builder.parameter("space.weight_divisor", FormulaSpace)

	out.Imbalance.ReferencePawnCount = builder.parameter("imbalance.reference_pawn_count", FormulaImbalance)
	out.Imbalance.KnightPerPawn = builder.phase("imbalance.knight_per_pawn", FormulaImbalance)
	out.Tempo = builder.parameter("tempo", FormulaLinear)
	out.DrawDivider = builder.parameter("final.draw_divider", FormulaFinalScale)

	if builder.err != nil {
		return nil, builder.err
	}
	var missing []string
	for i, consumed := range builder.consumed {
		if !consumed {
			missing = append(missing, string(registry.Specs[i].ID))
		}
	}
	if len(missing) != 0 {
		sort.Strings(missing)
		return nil, fmt.Errorf("registry parameters without trace bindings: %v", missing)
	}

	fieldUses, err := validatedTraceFieldBindings()
	if err != nil {
		return nil, err
	}
	out.fieldUses = fieldUses
	return out, nil
}

// FieldBindings returns the trace-schema coverage manifest.
func (b *TraceBinding) FieldBindings() []TraceFieldBinding {
	if b == nil {
		return nil
	}
	return append([]TraceFieldBinding(nil), b.fieldUses...)
}

type traceBindingBuilder struct {
	registry *Registry
	consumed []bool
	err      error
}

func (b *traceBindingBuilder) phase(base string, formula FormulaKind) PhaseHandles {
	return PhaseHandles{
		MG: b.parameter(base+".mg", formula),
		EG: b.parameter(base+".eg", formula),
	}
}

func (b *traceBindingBuilder) parameter(id string, formula FormulaKind) ParameterHandle {
	if b.err != nil {
		return ParameterHandle{}
	}
	handle, ok := b.registry.Resolve(ParameterID(id))
	if !ok {
		b.err = fmt.Errorf("trace binding parameter %q is not registered", id)
		return ParameterHandle{}
	}
	spec := b.registry.Specs[handle.SpecIndex]
	if spec.Formula != formula {
		b.err = fmt.Errorf("trace binding parameter %q has formula %q, want %q", id, spec.Formula, formula)
		return ParameterHandle{}
	}
	if b.consumed[handle.SpecIndex] {
		b.err = fmt.Errorf("trace binding parameter %q has more than one owner", id)
		return ParameterHandle{}
	}
	b.consumed[handle.SpecIndex] = true
	return handle
}

// BoundLinearTerm is a direct coefficient in the complete registry vector.
type BoundLinearTerm struct {
	ParameterIndex int
	Units          int
}

type BoundCandidateTarget struct {
	MGParameterIndex int
	EGParameterIndex int
}

type BoundCandidatePasser struct {
	Side    int
	Source  int
	Targets []BoundCandidateTarget
}

// BoundNonlinearUnits retains only the primitives needed by nonlinear formula
// groups. Parameter handles live in TraceBinding and are not repeated per row.
type BoundNonlinearUnits struct {
	Connected        eng.TuningConnectedUnits
	CandidatePassers []BoundCandidatePasser
	CenterOpenness   int
	KnightMobility   [9]int
	BishopMobility   [14]int
	BishopPair       int
	Danger           eng.TuningDangerUnits
	KingPassers      []eng.TuningKingPasser
	Space            eng.TuningSpaceUnits
	Imbalance        eng.TuningImbalanceUnits
}

// BoundTrace is the registry-indexed intermediate record consumed by the two
// Phase 3 forward models.
type BoundTrace struct {
	SchemaVersion   int
	FEN             string
	SideToMove      int
	PiecePhase      int
	TotalPhase      int
	TheoreticalDraw bool
	Fixed           eng.EvalPair
	Reference       eng.TuningReferenceTrace
	LinearMG        []BoundLinearTerm
	LinearEG        []BoundLinearTerm
	Nonlinear       BoundNonlinearUnits
}

// BindTrace validates and compiles one engine trace into registry-vector
// indexes. It performs no string-map lookup.
func (b *TraceBinding) BindTrace(trace eng.TuningTrace) (BoundTrace, error) {
	if b == nil {
		return BoundTrace{}, fmt.Errorf("cannot bind a trace with a nil binding")
	}
	if trace.SchemaVersion != eng.TuningTraceSchemaVersion {
		return BoundTrace{}, fmt.Errorf("tuning trace schema %d, want %d", trace.SchemaVersion, eng.TuningTraceSchemaVersion)
	}
	if trace.SideToMove != 1 && trace.SideToMove != -1 {
		return BoundTrace{}, fmt.Errorf("tuning trace sideToMove %d is not +1 or -1", trace.SideToMove)
	}
	// Promotions can raise the engine's raw phase above TotalPhase. Production
	// evaluation currently extrapolates with a negative EG weight in that case,
	// so the tuner must preserve it rather than clamp or reject it.
	if trace.TotalPhase <= 0 || trace.PiecePhase < 0 {
		return BoundTrace{}, fmt.Errorf("invalid tuning phase %d/%d", trace.PiecePhase, trace.TotalPhase)
	}

	out := BoundTrace{
		SchemaVersion:   trace.SchemaVersion,
		FEN:             trace.FEN,
		SideToMove:      trace.SideToMove,
		PiecePhase:      trace.PiecePhase,
		TotalPhase:      trace.TotalPhase,
		TheoreticalDraw: trace.TheoreticalDraw,
		Fixed:           trace.Fixed,
		Reference:       trace.Reference,
		LinearMG:        make([]BoundLinearTerm, 0, 64),
		LinearEG:        make([]BoundLinearTerm, 0, 64),
		Nonlinear: BoundNonlinearUnits{
			Connected:      trace.Units.Pawn.Connected,
			CenterOpenness: trace.Units.Center.Openness,
			KnightMobility: trace.Units.Mobility.Knight,
			BishopMobility: trace.Units.Mobility.Bishop,
			BishopPair:     trace.Units.Piece.BishopPair,
			Danger:         trace.Units.Danger,
			Space:          trace.Units.Space,
			Imbalance:      trace.Units.Imbalance,
		},
	}
	if out.Nonlinear.CenterOpenness < -4 || out.Nonlinear.CenterOpenness > 4 {
		return BoundTrace{}, fmt.Errorf("center openness %d is outside [-4,4]", out.Nonlinear.CenterOpenness)
	}

	u := trace.Units
	for pt := gm.PieceTypePawn; pt <= gm.PieceTypeQueen; pt++ {
		b.addPhase(&out, b.Material[pt], u.Material[pt])
	}
	if u.Material[gm.PieceTypeKing] != 0 {
		return BoundTrace{}, fmt.Errorf("king material units must be zero, got %d", u.Material[gm.PieceTypeKing])
	}
	for _, unit := range u.PSQT {
		if unit.Piece < int(gm.PieceTypePawn) || unit.Piece > int(gm.PieceTypeKing) {
			return BoundTrace{}, fmt.Errorf("PSQT piece index %d is invalid", unit.Piece)
		}
		if unit.Square < 0 || unit.Square >= 64 {
			return BoundTrace{}, fmt.Errorf("PSQT square %d is invalid", unit.Square)
		}
		handle := b.PSQT[unit.Piece]
		var index int
		if unit.Piece == int(gm.PieceTypePawn) {
			if unit.Square < 8 || unit.Square >= 56 {
				return BoundTrace{}, fmt.Errorf("pawn PSQT square %d is an unreachable storage sentinel", unit.Square)
			}
			index = handle.MG.MustIndex(unit.Square/8-1, unit.Square%8)
			b.addLinear(&out.LinearMG, index, unit.Units)
			index = handle.EG.MustIndex(unit.Square/8-1, unit.Square%8)
			b.addLinear(&out.LinearEG, index, unit.Units)
		} else {
			b.addLinear(&out.LinearMG, handle.MG.MustIndex(unit.Square), unit.Units)
			b.addLinear(&out.LinearEG, handle.EG.MustIndex(unit.Square), unit.Units)
		}
	}

	for _, unit := range u.Pawn.Passed {
		if unit.Index < 8 || unit.Index >= 56 {
			return BoundTrace{}, fmt.Errorf("passed-pawn PSQT index %d is an unreachable storage sentinel", unit.Index)
		}
		rank, file := unit.Index/8-1, unit.Index%8
		b.addLinear(&out.LinearMG, b.Pawn.Passed.MG.MustIndex(rank, file), unit.Units)
		b.addLinear(&out.LinearEG, b.Pawn.Passed.EG.MustIndex(rank, file), unit.Units)
	}

	b.addPhase(&out, b.Pawn.IsolatedOpposed, u.Pawn.IsolatedOpposed)
	b.addPhase(&out, b.Pawn.IsolatedUnopposed, u.Pawn.IsolatedUnopposed)
	b.addPhase(&out, b.Pawn.DoubledOpposed, u.Pawn.DoubledOpposed)
	b.addPhase(&out, b.Pawn.DoubledUnopposed, u.Pawn.DoubledUnopposed)
	b.addPhase(&out, b.Pawn.BackwardOpposed, u.Pawn.BackwardOpposed)
	b.addPhase(&out, b.Pawn.BackwardUnopposed, u.Pawn.BackwardUnopposed)
	b.addPhase(&out, b.Pawn.WeakLever, u.Pawn.WeakLever)
	for i, units := range u.Pawn.Blocked {
		b.addLinear(&out.LinearMG, b.Pawn.Blocked.MG.MustIndex(i), units)
		b.addLinear(&out.LinearEG, b.Pawn.Blocked.EG.MustIndex(i), units)
	}
	if u.Pawn.Connected.White[0] != 0 || u.Pawn.Connected.Black[0] != 0 {
		return BoundTrace{}, fmt.Errorf("connected-pawn rank zero must be empty")
	}
	out.Nonlinear.CandidatePassers = make([]BoundCandidatePasser, len(u.Pawn.CandidatePassers))
	for i, candidate := range u.Pawn.CandidatePassers {
		if candidate.Side != 1 && candidate.Side != -1 {
			return BoundTrace{}, fmt.Errorf("candidate passer side %d is not +1 or -1", candidate.Side)
		}
		boundCandidate := BoundCandidatePasser{
			Side:    candidate.Side,
			Source:  candidate.Source,
			Targets: make([]BoundCandidateTarget, len(candidate.Targets)),
		}
		for j, target := range candidate.Targets {
			if target < 8 || target >= 56 {
				return BoundTrace{}, fmt.Errorf("candidate passer target %d is an unreachable passed-pawn cell", target)
			}
			rank, file := target/8-1, target%8
			boundCandidate.Targets[j] = BoundCandidateTarget{
				MGParameterIndex: b.Pawn.Passed.MG.MustIndex(rank, file),
				EGParameterIndex: b.Pawn.Passed.EG.MustIndex(rank, file),
			}
		}
		out.Nonlinear.CandidatePassers[i] = boundCandidate
	}

	for i, units := range u.Mobility.Rook {
		b.addLinear(&out.LinearMG, b.Mobility.Rook.MG.MustIndex(i), units)
		b.addLinear(&out.LinearEG, b.Mobility.Rook.EG.MustIndex(i), units)
	}
	for i, units := range u.Mobility.Queen {
		b.addLinear(&out.LinearMG, b.Mobility.Queen.MG.MustIndex(i), units)
		b.addLinear(&out.LinearEG, b.Mobility.Queen.EG.MustIndex(i), units)
	}

	b.addPhase(&out, b.Piece.KnightOutpost, u.Piece.KnightOutpost)
	b.addPhase(&out, b.Piece.KnightTropism, u.Piece.KnightTropism)
	b.addPhase(&out, b.Piece.BishopOutpost, u.Piece.BishopOutpost)
	b.addPhase(&out, b.Piece.BadBishop, u.Piece.BadBishop)
	b.addPhase(&out, b.Piece.RookSemiOpen, u.Piece.RookSemiOpen)
	b.addPhase(&out, b.Piece.RookOpen, u.Piece.RookOpen)
	b.addPhase(&out, b.Piece.RookFileCountOpen, u.Piece.RookFileCountOpen)
	b.addPhase(&out, b.Piece.RookFileCountSemi, u.Piece.RookFileCountSemi)
	b.addPhase(&out, b.Piece.RookSeventh, u.Piece.RookSeventh)
	b.addLinear(&out.LinearMG, b.Piece.RookStackedMG.Offset, u.Piece.RookStacked)
	b.addLinear(&out.LinearEG, b.Piece.QueenCentralEG.Offset, u.Piece.QueenCentralized)
	b.addLinear(&out.LinearMG, b.Piece.KingMinorMG.Offset, u.Piece.KingMinorDefenders)

	for edge := range u.ShelterStorm.Shelter {
		for rank, units := range u.ShelterStorm.Shelter[edge] {
			if rank == 7 {
				if units != 0 {
					return BoundTrace{}, fmt.Errorf("king shelter rank seven is an unreachable storage sentinel")
				}
				continue
			}
			b.addLinear(&out.LinearMG, b.Shelter.ShelterMG.MustIndex(edge, rank), units)
		}
		for rank, units := range u.ShelterStorm.StormFree[edge] {
			b.addLinear(&out.LinearMG, b.Shelter.StormFreeMG.MustIndex(edge, rank), units)
		}
	}
	for rank, units := range u.ShelterStorm.StormBlocked {
		if rank < 2 {
			if units != 0 {
				return BoundTrace{}, fmt.Errorf("blocked-storm rank %d is an unreachable storage sentinel", rank)
			}
			continue
		}
		b.addLinear(&out.LinearMG, b.Shelter.StormBlockedMG.MustIndex(rank-2), units)
		b.addLinear(&out.LinearEG, b.Shelter.StormBlockedEG.MustIndex(rank-2), units)
	}
	b.addLinear(&out.LinearMG, b.Tempo.Offset, u.Tempo)
	b.addLinear(&out.LinearEG, b.Tempo.Offset, u.Tempo)

	out.Nonlinear.KingPassers = append([]eng.TuningKingPasser(nil), u.KingPassers...)
	for _, passer := range out.Nonlinear.KingPassers {
		if passer.Side != 1 && passer.Side != -1 {
			return BoundTrace{}, fmt.Errorf("king passer side %d is not +1 or -1", passer.Side)
		}
	}
	return out, nil
}

func (b *TraceBinding) addPhase(out *BoundTrace, handles PhaseHandles, units int) {
	b.addLinear(&out.LinearMG, handles.MG.Offset, units)
	b.addLinear(&out.LinearEG, handles.EG.Offset, units)
}

func (b *TraceBinding) addLinear(out *[]BoundLinearTerm, index, units int) {
	if units == 0 {
		return
	}
	*out = append(*out, BoundLinearTerm{ParameterIndex: index, Units: units})
}
