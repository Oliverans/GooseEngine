package tuner

// Layout consolidates θ layout offsets for easier maintenance.
// Keep this consistent with exporter and SetParams/Params helpers.
type Layout struct {
    PSTMGStart, PSTEGStart       int // 384, 384
    MaterialMGStart, MaterialEGStart int // 6, 6
    PasserMGStart, PasserEGStart int // 64, 64
    P1Start                      int // 8
    PawnStructStart              int // 14
    MobilityMGStart, MobilityEGStart int // 7, 7
    KingTableStart               int // 100
    KingCorrStart                int // 4
    ExtrasStart                  int // 16
    WeakTempoStart               int // 3
    Total                        int // 1059
}

func computeLayout() Layout {
    var l Layout
    off := 0
    l.PSTMGStart = off; off += 384
    l.PSTEGStart = off; off += 384
    l.MaterialMGStart = off; off += 6
    l.MaterialEGStart = off; off += 6
    l.PasserMGStart = off; off += 64
    l.PasserEGStart = off; off += 64
    l.P1Start = off; off += 8
    l.PawnStructStart = off; off += 14
    l.MobilityMGStart = off; off += 7
    l.MobilityEGStart = off; off += 7
    l.KingTableStart = off; off += 100
    l.KingCorrStart = off; off += 4
    l.ExtrasStart = off; off += 16
    l.WeakTempoStart = off; off += 3
    l.Total = off
    return l
}
