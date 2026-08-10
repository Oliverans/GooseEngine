package engine

// Export read-only copies of baseline evaluation constants for tuner seeding.
// These snapshots are captured before any generated overrides (evaluation_gen.go) run.

// Baseline copies
var (
	basePSQT_MG           [7][64]int
	basePSQT_EG           [7][64]int
	basePieceValueMG      [7]int
	basePieceValueEG      [7]int
	basePassedPawnPSQT_MG [64]int
	basePassedPawnPSQT_EG [64]int
	baseAttackerInner     [7]int
	baseAttackerOuter     [7]int
	// Tier 1/4: P1 scalars (BishopPair → Tier 4, rest → Tier 1)
	baseBishopPairBonusMG        int
	baseBishopPairBonusEG        int
	baseRookSemiOpenFileBonusMG  int
	baseRookSemiOpenFileBonusEG  int
	baseRookOpenFileBonusMG      int
	baseRookOpenFileBonusEG      int
	baseSeventhRankBonusMG       int
	baseSeventhRankBonusEG       int
	baseCentralizedQueenBonusEG  int
	baseQueenInfiltrationBonusMG int
	baseQueenInfiltrationBonusEG int
	// Tier 2: Pawn structure
	basePawnConnectedMG       [7]int
	baseWeakLeverPenaltyMG    int
	baseWeakLeverPenaltyEG    int
	// King safety
	baseKingSafetyTable            [100]int
	baseKingShelterMG              [4][8]int
	baseKingStormUnblockedMG       [4][8]int
	baseKingStormBlockedMG         [8]int
	baseKingStormBlockedEG         [8]int
	baseKingMinorPieceDefenseBonus int
	baseTempoBonus                 int
	// Tier 1/3: Extras (Outposts/StackedRooks → Tier 1, Tropism/PawnStorm → Tier 3)
	baseKnightOutpostMG        int
	baseKnightOutpostEG        int
	baseBishopOutpostMG        int
	baseBishopOutpostEG        int
	baseKnightCanAttackPieceMG int
	baseKnightCanAttackPieceEG int
	baseStackedRooksMG         int
	// Bishop x-ray and pawn storm family
	baseBishopXrayKingMG       int
	baseBishopXrayRookMG       int
	baseBishopXrayQueenMG      int
	basePawnStormMG            int
	basePawnProximityPenaltyMG int
	basePawnStormBaseMG        [8]int
	basePawnStormFreePct       [8]int
	basePawnStormLeverPct      [8]int
	basePawnStormWeakLeverPct  [8]int
	basePawnStormBlockedPct    [8]int
	basePawnStormOppositeMult  int
	// Imbalance scalars
	baseImbalanceKnightPerPawnMG int
	baseImbalanceKnightPerPawnEG int
)

func init() {
	// Snapshot all current values as baselines (evaluation.go values) before generated init runs.
	basePSQT_MG = PSQT_MG
	basePSQT_EG = PSQT_EG
	basePieceValueMG = pieceValueMG
	basePieceValueEG = pieceValueEG
	basePassedPawnPSQT_MG = PassedPawnPSQT_MG
	basePassedPawnPSQT_EG = PassedPawnPSQT_EG
	baseAttackerInner = attackerInner
	baseAttackerOuter = attackerOuter
	// Tier 1/4: P1 scalars
	baseBishopPairBonusMG = BishopPairBonusMG
	baseBishopPairBonusEG = BishopPairBonusEG
	baseRookSemiOpenFileBonusMG = RookSemiOpenMG
	baseRookSemiOpenFileBonusEG = RookSemiOpenEG
	baseRookOpenFileBonusMG = RookOpenMG
	baseRookOpenFileBonusEG = RookOpenEG
	baseSeventhRankBonusMG = RookSeventhRankMG
	baseSeventhRankBonusEG = RookSeventhRankEG
	baseCentralizedQueenBonusEG = QueenCentralizationEG
	// Tier 2: Pawn structure
	basePawnConnectedMG = PawnConnectedMG
	baseWeakLeverPenaltyMG = PawnWeakLeverMG
	baseWeakLeverPenaltyEG = PawnWeakLeverEG
	// King safety
	baseKingSafetyTable = KingSafetyTable
	baseKingShelterMG = KingShelterMG
	baseKingStormUnblockedMG = KingStormUnblockedMG
	baseKingStormBlockedMG = KingStormBlockedMG
	baseKingStormBlockedEG = KingStormBlockedEG
	baseKingMinorPieceDefenseBonus = KingMinorDefenseBonusMG
	baseTempoBonus = TempoBonus
	// Extras
	baseKnightOutpostMG = KnightOutpostMG
	baseKnightOutpostEG = KnightOutpostEG
	baseBishopOutpostMG = BishopOutpostMG
	baseBishopOutpostEG = BishopOutpostEG
	baseStackedRooksMG = RookStackedMG
	basePawnStormFreePct = PawnStormFreePct
	basePawnStormLeverPct = PawnStormLeverPct
	basePawnStormWeakLeverPct = PawnStormWeakLeverPct
	basePawnStormBlockedPct = PawnStormBlockedPct
	basePawnStormBaseMG = PawnStormBaseMG
	basePawnStormOppositeMult = PawnStormOppositeMultiplier
	baseImbalanceKnightPerPawnMG = ImbalanceKnightPerPawnMG
	baseImbalanceKnightPerPawnEG = ImbalanceKnightPerPawnEG
}

// Accessors for baselines (evaluation.go values)
func DefaultPSQT_MG() [7][64]int { return basePSQT_MG }
func DefaultPSQT_EG() [7][64]int { return basePSQT_EG }

func DefaultPieceValueMG() [7]int { return basePieceValueMG }
func DefaultPieceValueEG() [7]int { return basePieceValueEG }

func DefaultPassedPawnPSQT_MG() [64]int { return basePassedPawnPSQT_MG }
func DefaultPassedPawnPSQT_EG() [64]int { return basePassedPawnPSQT_EG }

// Tier 1: Mobility/attacker defaults
func DefaultAttackerInner() [7]int { return baseAttackerInner }
func DefaultAttackerOuter() [7]int { return baseAttackerOuter }

// Tier 1/4: P1 scalar defaults (BishopPair → Tier 4, rest → Tier 1)
func DefaultBishopPairBonusMG() int        { return baseBishopPairBonusMG }
func DefaultBishopPairBonusEG() int        { return baseBishopPairBonusEG }
func DefaultRookSemiOpenFileBonusMG() int  { return baseRookSemiOpenFileBonusMG }
func DefaultRookSemiOpenFileBonusEG() int  { return baseRookSemiOpenFileBonusEG }
func DefaultRookOpenFileBonusMG() int      { return baseRookOpenFileBonusMG }
func DefaultRookOpenFileBonusEG() int      { return baseRookOpenFileBonusEG }
func DefaultSeventhRankBonusMG() int       { return baseSeventhRankBonusMG }
func DefaultSeventhRankBonusEG() int       { return baseSeventhRankBonusEG }
func DefaultCentralizedQueenBonusEG() int  { return baseCentralizedQueenBonusEG }
func DefaultQueenInfiltrationBonusMG() int { return baseQueenInfiltrationBonusMG }
func DefaultQueenInfiltrationBonusEG() int { return baseQueenInfiltrationBonusEG }

// Tier 2: Pawn structure defaults
// Retired pawn scalars. Doubled and backward now split by `opposed` into two
// constants each, and blocked became a rank-indexed pair with the opposite sign.
// The frozen tuner still fits one number for each, so these accessors return the
// last value the single constant held, purely so tuner/seed.go keeps compiling.
// Nothing in the engine reads them. Delete with their tuner call sites.
func DefaultDoubledPawnPenaltyMG() int  { return 9 }
func DefaultDoubledPawnPenaltyEG() int  { return 20 }
// Retired. Isolated now splits by `opposed` into two constants per phase.
// Returns the last values the flat pair held so tuner/seed.go keeps compiling.
func DefaultIsolatedPawnMG() int        { return 9 }
func DefaultIsolatedPawnEG() int        { return 14 }
func DefaultPawnConnectedMG() [7]int { return basePawnConnectedMG }

// Retired. Connected and phalanx merged into the rank-indexed PawnConnectedMG,
// where phalanx became a multiplier rather than an addend. These return the last
// values the two flat constants held, so tuner/seed.go keeps compiling; nothing
// in the engine reads them.
func DefaultConnectedPawnsBonusMG() int { return 11 }
func DefaultConnectedPawnsBonusEG() int { return 5 }
func DefaultPhalanxPawnsBonusMG() int   { return 7 }
func DefaultPhalanxPawnsBonusEG() int   { return 10 }
func DefaultBlockedPawnBonusMG() int    { return 0 }
func DefaultBlockedPawnBonusEG() int    { return 1 }
func DefaultBackwardPawnMG() int        { return 6 }
func DefaultBackwardPawnEG() int        { return 11 }
func DefaultWeakLeverPenaltyMG() int    { return baseWeakLeverPenaltyMG }
func DefaultWeakLeverPenaltyEG() int    { return baseWeakLeverPenaltyEG }

// King safety table
func DefaultKingSafetyTable() [100]int { return baseKingSafetyTable }

// King safety correlated defaults
func DefaultKingShelterMG() [4][8]int        { return baseKingShelterMG }
func DefaultKingStormUnblockedMG() [4][8]int { return baseKingStormUnblockedMG }
func DefaultKingStormBlockedMG() [8]int      { return baseKingStormBlockedMG }
func DefaultKingStormBlockedEG() [8]int      { return baseKingStormBlockedEG }
func DefaultKingMinorPieceDefenseBonus() int { return baseKingMinorPieceDefenseBonus }

// Retired king-shelter scalars. KingSemiOpenFileMG, KingOpenFileMG and
// KingPawnDefenseBonusMG no longer exist in the evaluation -- the per-file
// KingShelterMG table absorbed all three. These accessors return the last
// values those constants held, purely so the frozen tuner in tuner/seed.go
// keeps compiling; nothing in the engine reads them. Delete them along with
// their tuner call sites when the tuner is rewritten.
func DefaultKingSemiOpenFilePenalty() int { return 12 }
func DefaultKingOpenFilePenalty() int     { return 20 }
func DefaultKingPawnDefenseMG() int       { return 2 }

// Space/weak-king + tempo
// Retired. Space is no longer a flat per-square bonus: it is four tier
// constants over a corrected zone, scaled by a material weight, middlegame only.
// These return the last values the two scalars held so tuner/seed.go keeps
// compiling; nothing in the engine reads them.
func DefaultSpaceBonusMG() int            { return 2 }
func DefaultSpaceBonusEG() int            { return 1 }
// Retired with the term itself; see the note in evaluation.go. Returns the last
// value it held so tuner/seed.go keeps compiling.
func DefaultWeakKingSquarePenaltyMG() int { return 4 }
func DefaultTempoBonus() int              { return baseTempoBonus }

// Tier 1/3: Extras defaults (Outposts/StackedRooks → Tier 1, Tropism/Storm → Tier 3)
func DefaultKnightOutpostMG() int        { return baseKnightOutpostMG }
func DefaultKnightOutpostEG() int        { return baseKnightOutpostEG }
func DefaultBishopOutpostMG() int        { return baseBishopOutpostMG }
func DefaultBishopOutpostEG() int        { return baseBishopOutpostEG }
func DefaultKnightCanAttackPieceMG() int { return baseKnightCanAttackPieceMG }
func DefaultKnightCanAttackPieceEG() int { return baseKnightCanAttackPieceEG }
func DefaultStackedRooksMG() int         { return baseStackedRooksMG }

// New accessors for bishop xray and pawn storm family
func DefaultBishopXrayKingMG() int       { return baseBishopXrayKingMG }
func DefaultBishopXrayRookMG() int       { return baseBishopXrayRookMG }
func DefaultBishopXrayQueenMG() int      { return baseBishopXrayQueenMG }
func DefaultPawnStormMG() int            { return basePawnStormMG }
func DefaultPawnProximityPenaltyMG() int { return basePawnProximityPenaltyMG }
func DefaultPawnStormBaseMG() [8]int     { return basePawnStormBaseMG }
func DefaultPawnStormFreePct() [8]int    { return basePawnStormFreePct }
func DefaultPawnStormLeverPct() [8]int   { return basePawnStormLeverPct }
func DefaultPawnStormWeakLeverPct() [8]int {
	return basePawnStormWeakLeverPct
}
func DefaultPawnStormBlockedPct() [8]int { return basePawnStormBlockedPct }
func DefaultPawnStormOppositeMult() int  { return basePawnStormOppositeMult }

// Imbalance defaults
func DefaultImbalanceKnightPerPawnMG() int { return baseImbalanceKnightPerPawnMG }
func DefaultImbalanceKnightPerPawnEG() int { return baseImbalanceKnightPerPawnEG }

// The per-bishop tilt is retired from the evaluation; Kaufman adjusts the bishop
// PAIR, not the single bishop, and mobility/BadBishop/centre-openness already
// carry it. These return the last values the two scalars held so tuner/seed.go
// keeps compiling; nothing in the engine reads them.
func DefaultImbalanceBishopPerPawnMG() int { return -6 }
func DefaultImbalanceBishopPerPawnEG() int { return -2 }
