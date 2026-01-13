package tuner

// Layout consolidates theta layout offsets for easier maintenance.
// Keep this consistent with exporter and SetParams/Params helpers.
//
// Strict tier block order (theta):
//   - Tier 1: PST MG/EG, Material MG/EG, Mobility MG/EG, Core scalars, Tier1 extras
//   - Tier 2: Passers MG/EG, PawnStruct
//   - Tier 3: King table, King correlates, King endgame, Tier3 extras, WeakKing
//   - Tier 4: BishopPair, Imbalance, Space/Tempo
type Layout struct {
	// Tier 1
	PSTMGStart, PSTEGStart           int // 384, 384
	MaterialMGStart, MaterialEGStart int // 6, 6
	MobilityMGStart, MobilityEGStart int // 60, 60
	CoreScalarStart                  int // 4 (Rook files, SeventhRank, QueenCentralized)
	Tier1ExtrasStart                 int // 9 (Outposts, StackedRooks, MobCenter, BadBishop)

	// Tier 2
	PasserMGStart, PasserEGStart int // 64, 64
	PawnStructStart              int // 16

	// Tier 3
	KingTableStart   int // 100
	KingCorrStart    int // 4
	KingEndgameStart int // 2
	Tier3ExtrasStart int // 44 (Tropism + PawnStorm + PawnProximity)
	WeakKingStart    int // 1

	// Tier 4
	BishopPairStart int // 2
	ImbalanceStart  int // 4
	SpaceTempoStart int // 3 (SpaceMG, SpaceEG, Tempo)

	Total int // 1217
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
	l.MobilityMGStart = off
	off += 60
	l.MobilityEGStart = off
	off += 60
	l.CoreScalarStart = off
	off += 4
	l.Tier1ExtrasStart = off
	off += 9
	l.PasserMGStart = off
	off += 64
	l.PasserEGStart = off
	off += 64
	l.PawnStructStart = off
	off += 16
	l.KingTableStart = off
	off += 100
	l.KingCorrStart = off
	off += 4
	l.KingEndgameStart = off
	off += 2
	l.Tier3ExtrasStart = off
	off += 44
	l.WeakKingStart = off
	off += 1
	l.BishopPairStart = off
	off += 2
	l.ImbalanceStart = off
	off += 4
	l.SpaceTempoStart = off
	off += 3
	l.Total = off
	return l
}
