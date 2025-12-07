package tuner

// Layout consolidates θ layout offsets for easier maintenance.
// Keep this consistent with exporter and SetParams/Params helpers.
type Layout struct {
	PSTMGStart, PSTEGStart           int // 384, 384
	MaterialMGStart, MaterialEGStart int // 6, 6
	PasserMGStart, PasserEGStart     int // 64, 64
	P1Start                          int // 8
	PawnStructStart                  int // 14
	MobilityMGStart, MobilityEGStart int // 7, 7
	KingTableStart                   int // 100
	KingCorrStart                    int // 4
	KingEndgameStart                 int // 2
	ExtrasStart                      int // 17
	ImbalanceStart                   int // 12
	WeakTempoStart                   int // 5
	Total                            int // 1084
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
	off += 8
	l.PawnStructStart = off
	off += 14
	l.MobilityMGStart = off
	off += 7
	l.MobilityEGStart = off
	off += 7
	l.KingTableStart = off
	off += 100
	l.KingCorrStart = off
	off += 4
	l.KingEndgameStart = off
	off += 2
	l.ExtrasStart = off
	off += 17
	l.ImbalanceStart = off
	off += 12
	l.WeakTempoStart = off
	off += 5
	l.Total = off
	return l
}
