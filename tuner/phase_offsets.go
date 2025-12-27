package tuner

// Layout consolidates θ layout offsets for easier maintenance.
// Keep this consistent with exporter and SetParams/Params helpers.
//
// Tier mapping (toggles → parameters):
//   - Tier 1 (Core): PST, Material, Mobility, P1 (except BishopPair), Extras (Outposts, StackedRooks, MobCenter)
//   - Tier 2 (Pawns): Passers, PawnStruct
//   - Tier 3 (King Safety): KingTable, KingCorr, KingEndgame, Extras (Tropism, PawnStorm), WeakTempo (WeakKingSquares)
//   - Tier 4 (Misc): P1 (BishopPair), Imbalance, WeakTempo (Space, Tempo)
type Layout struct {
	PSTMGStart, PSTEGStart           int // 384, 384 — Tier 1
	MaterialMGStart, MaterialEGStart int // 6, 6 — Tier 1
	PasserMGStart, PasserEGStart     int // 64, 64 — Tier 2
	P1Start                          int // 6 — Tier 1 (RookFiles, etc.) + Tier 4 (BishopPair)
	PawnStructStart                  int // 16 — Tier 2
	MobilityMGStart, MobilityEGStart int // 60, 60 — Tier 1
	KingTableStart                   int // 100 — Tier 3
	KingCorrStart                    int // 4 — Tier 3
	KingEndgameStart                 int // 2 — Tier 3
	ExtrasStart                      int // 53 — Tier 1 (Outposts, BadBishop, StackedRooks, MobCenter) + Tier 3 (Tropism, Storm)
	ImbalanceStart                   int // 4 - Tier 4
	WeakTempoStart                   int // 4 - Tier 3 (WeakKingSquares) + Tier 4 (Space, Tempo)
	Total                            int // 1217
}

func computeLayout() Layout {
	var l Layout
	off := 0
	l.PSTMGStart = off
	off += 384
	l.PSTEGStart = off
	off += 384
	l.MaterialMGStart = off
	off += 6
	l.MaterialEGStart = off
	off += 6
	l.PasserMGStart = off
	off += 64
	l.PasserEGStart = off
	off += 64
	l.P1Start = off
	off += 6
	l.PawnStructStart = off
	off += 16
	l.MobilityMGStart = off
	off += 60
	l.MobilityEGStart = off
	off += 60
	l.KingTableStart = off
	off += 100
	l.KingCorrStart = off
	off += 4
	l.KingEndgameStart = off
	off += 2
	l.ExtrasStart = off
	off += 53
	l.ImbalanceStart = off
	off += 4
	l.WeakTempoStart = off
	off += 4
	l.Total = off
	return l
}
