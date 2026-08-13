package tuner

// CoverageAccumulator reuses one generation-mark array across the entire
// conversion, avoiding a map allocation for every source position.
type CoverageAccumulator struct {
	binding    *TraceBinding
	counts     []uint64
	seen       []uint32
	generation uint32
}

func NewCoverageAccumulator(binding *TraceBinding, parameterCount int) *CoverageAccumulator {
	return &CoverageAccumulator{
		binding: binding,
		counts:  make([]uint64, parameterCount),
		seen:    make([]uint32, parameterCount),
	}
}

func (a *CoverageAccumulator) Add(trace BoundTrace) {
	if a == nil || a.binding == nil {
		return
	}
	a.generation++
	if a.generation == 0 {
		clear(a.seen)
		a.generation = 1
	}
	mark := func(index int) {
		if index >= 0 && index < len(a.counts) && a.seen[index] != a.generation {
			a.seen[index] = a.generation
			a.counts[index]++
		}
	}
	a.binding.accumulateCoverage(trace, mark)
}

func (a *CoverageAccumulator) Counts() []uint64 {
	if a == nil {
		return nil
	}
	return a.counts
}

// AccumulateCoverage is the convenience form for callers that do not maintain
// a long-lived accumulator.
func (b *TraceBinding) AccumulateCoverage(trace BoundTrace, counts []uint64) {
	accumulator := &CoverageAccumulator{binding: b, counts: counts, seen: make([]uint32, len(counts))}
	accumulator.Add(trace)
}

// accumulateCoverage reports parameter indexes with primitive observations.
// For nonlinear formulas this is input coverage, not a claim that the current
// subgradient is non-zero after max/clamp operations.
func (b *TraceBinding) accumulateCoverage(trace BoundTrace, mark func(int)) {
	markHandle := func(handle ParameterHandle) {
		for index := handle.Offset; index < handle.Offset+handle.Length; index++ {
			mark(index)
		}
	}
	markPhase := func(handles PhaseHandles) {
		markHandle(handles.MG)
		markHandle(handles.EG)
	}
	for _, term := range trace.LinearMG {
		mark(term.ParameterIndex)
	}
	for _, term := range trace.LinearEG {
		mark(term.ParameterIndex)
	}
	u := trace.Nonlinear
	for rank := 1; rank < 7; rank++ {
		if u.Connected.White[rank] != 0 || u.Connected.Black[rank] != 0 {
			mark(b.Pawn.Connected.MustIndex(rank - 1))
		}
	}
	for _, candidate := range u.CandidatePassers {
		if len(candidate.Targets) == 0 {
			continue
		}
		markPhase(b.Pawn.CandidatePct)
		for _, target := range candidate.Targets {
			mark(target.MGParameterIndex)
			mark(target.EGParameterIndex)
		}
	}
	knightActive := false
	for i, units := range u.KnightMobility {
		if units != 0 {
			knightActive = true
			mark(b.Mobility.Knight.MG.Offset + i)
			mark(b.Mobility.Knight.EG.Offset + i)
		}
	}
	if knightActive && u.CenterOpenness != 0 {
		markPhase(b.Center.KnightMobilityPct)
	}
	bishopActive := false
	for i, units := range u.BishopMobility {
		if units != 0 {
			bishopActive = true
			mark(b.Mobility.Bishop.MG.Offset + i)
			mark(b.Mobility.Bishop.EG.Offset + i)
		}
	}
	if bishopActive && u.CenterOpenness != 0 {
		markPhase(b.Center.BishopMobilityPct)
	}
	if u.BishopPair != 0 {
		markPhase(b.Piece.BishopPair)
		if u.CenterOpenness != 0 {
			markPhase(b.Center.BishopPairPct)
		}
	}
	for _, side := range []struct {
		attackers  [4]int
		ringHits   int
		safeChecks [4]int
		unsafe     int
		hasQueen   bool
	}{
		{u.Danger.White.Attackers, u.Danger.White.RingHits, u.Danger.White.SafeChecks, u.Danger.White.UnsafeChecks, u.Danger.White.HasQueen},
		{u.Danger.Black.Attackers, u.Danger.Black.RingHits, u.Danger.Black.SafeChecks, u.Danger.Black.UnsafeChecks, u.Danger.Black.HasQueen},
	} {
		// Adjustment and divisor participate in the formula even when every
		// board-derived danger count is zero.
		markPhase(b.Danger.Adjustment)
		markPhase(b.Danger.Divisor)
		if side.ringHits != 0 {
			markPhase(b.Danger.AttackValue)
		}
		if side.unsafe != 0 {
			markPhase(b.Danger.UnsafeCheck)
		}
		if !side.hasQueen {
			markPhase(b.Danger.NoEnemyQueen)
		}
		for kind := 0; kind < 4; kind++ {
			if side.attackers[kind] != 0 {
				markPhase(b.Danger.AttackerWeight[kind])
			}
			if side.safeChecks[kind] != 0 {
				markPhase(b.Danger.SafeCheck[kind])
			}
		}
	}
	if len(u.KingPassers) != 0 {
		markHandle(b.KingPasser.ProximityEG)
		markHandle(b.KingPasser.Divisor)
		markHandle(b.KingPasser.EnemyWeight)
		markHandle(b.KingPasser.OwnWeight)
	}
	spaceActive := func(sideSafe, behind, semi, open, pieces int) bool {
		return sideSafe != 0 || behind != 0 || semi != 0 || open != 0 || pieces != 0
	}
	if spaceActive(u.Space.White.Safe, u.Space.White.BehindPawn, u.Space.White.SemiOpen, u.Space.White.Open, u.Space.White.PieceCount) ||
		spaceActive(u.Space.Black.Safe, u.Space.Black.BehindPawn, u.Space.Black.SemiOpen, u.Space.Black.Open, u.Space.Black.PieceCount) {
		if u.Space.White.Safe != 0 || u.Space.Black.Safe != 0 {
			markHandle(b.Space.SafeMG)
		}
		if u.Space.White.BehindPawn != 0 || u.Space.Black.BehindPawn != 0 {
			markHandle(b.Space.BehindPawnMG)
		}
		if u.Space.White.SemiOpen != 0 || u.Space.Black.SemiOpen != 0 {
			markHandle(b.Space.SemiOpenMG)
		}
		if u.Space.White.Open != 0 || u.Space.Black.Open != 0 {
			markHandle(b.Space.OpenMG)
		}
		markHandle(b.Space.WeightOffset)
		markHandle(b.Space.WeightDivisor)
		if u.Space.BlockedPawns != 0 {
			markHandle(b.Space.BlockedCap)
		}
	}
	if u.Imbalance.KnightDiff != 0 {
		markHandle(b.Imbalance.ReferencePawnCount)
		markPhase(b.Imbalance.KnightPerPawn)
	}
	if trace.TheoreticalDraw {
		markHandle(b.DrawDivider)
	}
}
