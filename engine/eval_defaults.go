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
	baseMobilityValueMG   [7]int
	baseMobilityValueEG   [7]int
	baseAttackerInner     [7]int
	baseAttackerOuter     [7]int
	// Tier 1/4: P1 scalars (BishopPair → Tier 4, rest → Tier 1)
	baseBishopPairBonusMG        int
	baseBishopPairBonusEG        int
	baseRookSemiOpenFileBonusMG  int
	baseRookOpenFileBonusMG      int
	baseSeventhRankBonusEG       int
	baseCentralizedQueenBonusEG  int
	baseQueenInfiltrationBonusMG int
	baseQueenInfiltrationBonusEG int
	// Tier 2: Pawn structure
	baseDoubledPawnPenaltyMG  int
	baseDoubledPawnPenaltyEG  int
	baseIsolatedPawnMG        int
	baseIsolatedPawnEG        int
	baseConnectedPawnsBonusMG int
	baseConnectedPawnsBonusEG int
	basePhalanxPawnsBonusMG   int
	basePhalanxPawnsBonusEG   int
	baseBlockedPawnBonusMG    int
	baseBlockedPawnBonusEG    int
	baseBackwardPawnMG        int
	baseBackwardPawnEG        int
	baseWeakLeverPenaltyMG    int
	baseWeakLeverPenaltyEG    int
	// King safety
	baseKingSafetyTable            [100]int
	baseKingSemiOpenFilePenalty    int
	baseKingOpenFilePenalty        int
	baseKingMinorPieceDefenseBonus int
	baseKingPawnDefenseMG          int
	baseSpaceBonusMG               int
	baseSpaceBonusEG               int
	baseWeakKingSquarePenaltyMG    int
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
	baseImbalanceKnightPerPawnMG    int
	baseImbalanceKnightPerPawnEG    int
	baseImbalanceBishopPerPawnMG    int
	baseImbalanceBishopPerPawnEG    int
)

func init() {
	// Snapshot all current values as baselines (evaluation.go values) before generated init runs.
	basePSQT_MG = PSQT_MG
	basePSQT_EG = PSQT_EG
	basePieceValueMG = pieceValueMG
	basePieceValueEG = pieceValueEG
	basePassedPawnPSQT_MG = PassedPawnPSQT_MG
	basePassedPawnPSQT_EG = PassedPawnPSQT_EG
	baseMobilityValueMG = mobilityValueMG
	baseMobilityValueEG = mobilityValueEG
	baseAttackerInner = attackerInner
	baseAttackerOuter = attackerOuter
	// Tier 1/4: P1 scalars
	baseBishopPairBonusMG = BishopPairBonusMG
	baseBishopPairBonusEG = BishopPairBonusEG
	baseRookSemiOpenFileBonusMG = RookSemiOpenMG
	baseRookOpenFileBonusMG = RookOpenMG
	baseSeventhRankBonusEG = RookSeventhRankEG
	baseCentralizedQueenBonusEG = QueenCentralizationEG
	// Tier 2: Pawn structure
	baseDoubledPawnPenaltyMG = PawnDoubledMG
	baseDoubledPawnPenaltyEG = PawnDoubledEG
	baseIsolatedPawnMG = IsolatedPawnMG
	baseIsolatedPawnEG = IsolatedPawnEG
	baseConnectedPawnsBonusMG = PawnConnectedMG
	baseConnectedPawnsBonusEG = PawnConnectedEG
	basePhalanxPawnsBonusMG = PawnPhalanxMG
	basePhalanxPawnsBonusEG = PawnPhalanxEG
	baseBlockedPawnBonusMG = PawnBlockedMG
	baseBlockedPawnBonusEG = PawnBlockedEG
	baseBackwardPawnMG = BackwardPawnMG
	baseBackwardPawnEG = BackwardPawnEG
	baseWeakLeverPenaltyMG = PawnWeakLeverMG
	baseWeakLeverPenaltyEG = PawnWeakLeverEG
	// King safety
	baseKingSafetyTable = KingSafetyTable
	baseKingSemiOpenFilePenalty = KingSemiOpenFileMG
	baseKingOpenFilePenalty = KingOpenFileMG
	baseKingMinorPieceDefenseBonus = KingMinorDefenseBonusMG
	baseKingPawnDefenseMG = KingPawnDefenseBonusMG
	baseSpaceBonusMG = SpaceBonusMG
	baseSpaceBonusEG = SpaceBonusEG
	baseWeakKingSquarePenaltyMG = WeakKingSquarePenaltyMG
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
	baseImbalanceBishopPerPawnMG = ImbalanceBishopPerPawnMG
	baseImbalanceBishopPerPawnEG = ImbalanceBishopPerPawnEG
}

// Accessors for baselines (evaluation.go values)
func DefaultPSQT_MG() [7][64]int { return basePSQT_MG }
func DefaultPSQT_EG() [7][64]int { return basePSQT_EG }

func DefaultPieceValueMG() [7]int { return basePieceValueMG }
func DefaultPieceValueEG() [7]int { return basePieceValueEG }

func DefaultPassedPawnPSQT_MG() [64]int { return basePassedPawnPSQT_MG }
func DefaultPassedPawnPSQT_EG() [64]int { return basePassedPawnPSQT_EG }

// Tier 1: Mobility/attacker defaults
func DefaultMobilityValueMG() [7]int { return baseMobilityValueMG }
func DefaultMobilityValueEG() [7]int { return baseMobilityValueEG }
func DefaultAttackerInner() [7]int   { return baseAttackerInner }
func DefaultAttackerOuter() [7]int   { return baseAttackerOuter }

// Tier 1/4: P1 scalar defaults (BishopPair → Tier 4, rest → Tier 1)
func DefaultBishopPairBonusMG() int        { return baseBishopPairBonusMG }
func DefaultBishopPairBonusEG() int        { return baseBishopPairBonusEG }
func DefaultRookSemiOpenFileBonusMG() int  { return baseRookSemiOpenFileBonusMG }
func DefaultRookOpenFileBonusMG() int      { return baseRookOpenFileBonusMG }
func DefaultSeventhRankBonusEG() int       { return baseSeventhRankBonusEG }
func DefaultCentralizedQueenBonusEG() int  { return baseCentralizedQueenBonusEG }
func DefaultQueenInfiltrationBonusMG() int { return baseQueenInfiltrationBonusMG }
func DefaultQueenInfiltrationBonusEG() int { return baseQueenInfiltrationBonusEG }

// Tier 2: Pawn structure defaults
func DefaultDoubledPawnPenaltyMG() int  { return baseDoubledPawnPenaltyMG }
func DefaultDoubledPawnPenaltyEG() int  { return baseDoubledPawnPenaltyEG }
func DefaultIsolatedPawnMG() int        { return baseIsolatedPawnMG }
func DefaultIsolatedPawnEG() int        { return baseIsolatedPawnEG }
func DefaultConnectedPawnsBonusMG() int { return baseConnectedPawnsBonusMG }
func DefaultConnectedPawnsBonusEG() int { return baseConnectedPawnsBonusEG }
func DefaultPhalanxPawnsBonusMG() int   { return basePhalanxPawnsBonusMG }
func DefaultPhalanxPawnsBonusEG() int   { return basePhalanxPawnsBonusEG }
func DefaultBlockedPawnBonusMG() int    { return baseBlockedPawnBonusMG }
func DefaultBlockedPawnBonusEG() int    { return baseBlockedPawnBonusEG }
func DefaultBackwardPawnMG() int        { return baseBackwardPawnMG }
func DefaultBackwardPawnEG() int        { return baseBackwardPawnEG }
func DefaultWeakLeverPenaltyMG() int    { return baseWeakLeverPenaltyMG }
func DefaultWeakLeverPenaltyEG() int    { return baseWeakLeverPenaltyEG }

// King safety table
func DefaultKingSafetyTable() [100]int { return baseKingSafetyTable }

// King safety correlated defaults
func DefaultKingSemiOpenFilePenalty() int    { return baseKingSemiOpenFilePenalty }
func DefaultKingOpenFilePenalty() int        { return baseKingOpenFilePenalty }
func DefaultKingMinorPieceDefenseBonus() int { return baseKingMinorPieceDefenseBonus }
func DefaultKingPawnDefenseMG() int          { return baseKingPawnDefenseMG }

// Space/weak-king + tempo
func DefaultSpaceBonusMG() int            { return baseSpaceBonusMG }
func DefaultSpaceBonusEG() int            { return baseSpaceBonusEG }
func DefaultWeakKingSquarePenaltyMG() int { return baseWeakKingSquarePenaltyMG }
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
func DefaultImbalanceKnightPerPawnMG() int    { return baseImbalanceKnightPerPawnMG }
func DefaultImbalanceKnightPerPawnEG() int    { return baseImbalanceKnightPerPawnEG }
func DefaultImbalanceBishopPerPawnMG() int    { return baseImbalanceBishopPerPawnMG }
func DefaultImbalanceBishopPerPawnEG() int    { return baseImbalanceBishopPerPawnEG }
