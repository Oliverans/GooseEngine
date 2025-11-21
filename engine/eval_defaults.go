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
	// Phase 1
	baseBishopPairBonusMG        int
	baseBishopPairBonusEG        int
	baseRookSemiOpenFileBonusMG  int
	baseRookOpenFileBonusMG      int
	baseSeventhRankBonusEG       int
	baseCentralizedQueenBonusEG  int
	baseQueenInfiltrationBonusMG int
	baseQueenInfiltrationBonusEG int
	// Phase 2
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
	basePawnLeverMG           int
	basePawnLeverEG           int
	baseWeakLeverPenaltyMG    int
	baseWeakLeverPenaltyEG    int
	// King safety
	baseKingSafetyTable            [100]int
	baseKingSemiOpenFilePenalty    int
	baseKingOpenFilePenalty        int
	baseKingMinorPieceDefenseBonus int
	baseKingPawnDefenseMG          int
	baseWeakSquaresPenaltyMG       int
	baseWeakKingSquaresPenaltyMG   int
	baseTempoBonus                 int
	// Extras (phase 5)
	baseKnightOutpostMG        int
	baseKnightOutpostEG        int
	baseBishopOutpostMG        int
	baseKnightCanAttackPieceMG int
	baseKnightCanAttackPieceEG int
	baseStackedRooksMG         int
	baseRookXrayQueenMG        int
	baseConnectedRooksBonusMG  int
	// Bishop x-ray and pawn storm family
	baseBishopXrayKingMG        int
	baseBishopXrayRookMG        int
	baseBishopXrayQueenMG       int
	basePawnStormMG             int
	basePawnProximityPenaltyMG  int
	basePawnLeverStormPenaltyMG int
	// Imbalance scalars
	baseImbalanceKnightPerPawnMG    int
	baseImbalanceKnightPerPawnEG    int
	baseImbalanceBishopPerPawnMG    int
	baseImbalanceBishopPerPawnEG    int
	baseImbalanceMinorsForMajorMG   int
	baseImbalanceMinorsForMajorEG   int
	baseImbalanceRedundantRookMG    int
	baseImbalanceRedundantRookEG    int
	baseImbalanceRookQueenOverlapMG int
	baseImbalanceRookQueenOverlapEG int
	baseImbalanceQueenManyMinorsMG  int
	baseImbalanceQueenManyMinorsEG  int
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
	// Phase 1
	baseBishopPairBonusMG = BishopPairBonusMG
	baseBishopPairBonusEG = BishopPairBonusEG
	baseRookSemiOpenFileBonusMG = RookSemiOpenFileBonusMG
	baseRookOpenFileBonusMG = RookOpenFileBonusMG
	baseSeventhRankBonusEG = SeventhRankBonusEG
	baseCentralizedQueenBonusEG = CentralizedQueenBonusEG
	baseQueenInfiltrationBonusMG = QueenInfiltrationBonusMG
	baseQueenInfiltrationBonusEG = QueenInfiltrationBonusEG
	// Phase 2
	baseDoubledPawnPenaltyMG = DoubledPawnPenaltyMG
	baseDoubledPawnPenaltyEG = DoubledPawnPenaltyEG
	baseIsolatedPawnMG = IsolatedPawnMG
	baseIsolatedPawnEG = IsolatedPawnEG
	baseConnectedPawnsBonusMG = ConnectedPawnsBonusMG
	baseConnectedPawnsBonusEG = ConnectedPawnsBonusEG
	basePhalanxPawnsBonusMG = PhalanxPawnsBonusMG
	basePhalanxPawnsBonusEG = PhalanxPawnsBonusEG
	baseBlockedPawnBonusMG = BlockedPawnBonusMG
	baseBlockedPawnBonusEG = BlockedPawnBonusEG
	baseBackwardPawnMG = BackwardPawnMG
	baseBackwardPawnEG = BackwardPawnEG
	basePawnLeverMG = PawnLeverMG
	basePawnLeverEG = PawnLeverEG
	baseWeakLeverPenaltyMG = WeakLeverPenaltyMG
	baseWeakLeverPenaltyEG = WeakLeverPenaltyEG
	// King safety
	baseKingSafetyTable = KingSafetyTable
	baseKingSemiOpenFilePenalty = KingSemiOpenFilePenalty
	baseKingOpenFilePenalty = KingOpenFilePenalty
	baseKingMinorPieceDefenseBonus = KingMinorPieceDefenseBonus
	baseKingPawnDefenseMG = KingPawnDefenseMG
	baseWeakSquaresPenaltyMG = WeakSquaresPenaltyMG
	baseWeakKingSquaresPenaltyMG = WeakKingSquaresPenaltyMG
	baseTempoBonus = TempoBonus
	// Extras
	baseKnightOutpostMG = KnightOutpostMG
	baseKnightOutpostEG = KnightOutpostEG
	baseBishopOutpostMG = BishopOutpostMG
	baseKnightCanAttackPieceMG = KnightCanAttackPieceMG
	baseKnightCanAttackPieceEG = KnightCanAttackPieceEG
	baseStackedRooksMG = StackedRooksMG
	baseRookXrayQueenMG = RookXrayQueenMG
	baseConnectedRooksBonusMG = ConnectedRooksBonusMG
	baseBishopXrayKingMG = BishopXrayKingMG
	baseBishopXrayRookMG = BishopXrayRookMG
	baseBishopXrayQueenMG = BishopXrayQueenMG
	basePawnStormMG = PawnStormMG
	basePawnProximityPenaltyMG = PawnProximityPenaltyMG
	basePawnLeverStormPenaltyMG = PawnLeverStormPenaltyMG
	baseImbalanceKnightPerPawnMG = ImbalanceKnightPerPawnMG
	baseImbalanceKnightPerPawnEG = ImbalanceKnightPerPawnEG
	baseImbalanceBishopPerPawnMG = ImbalanceBishopPerPawnMG
	baseImbalanceBishopPerPawnEG = ImbalanceBishopPerPawnEG
	baseImbalanceMinorsForMajorMG = ImbalanceMinorsForMajorMG
	baseImbalanceMinorsForMajorEG = ImbalanceMinorsForMajorEG
	baseImbalanceRedundantRookMG = ImbalanceRedundantRookMG
	baseImbalanceRedundantRookEG = ImbalanceRedundantRookEG
	baseImbalanceRookQueenOverlapMG = ImbalanceRookQueenOverlapMG
	baseImbalanceRookQueenOverlapEG = ImbalanceRookQueenOverlapEG
	baseImbalanceQueenManyMinorsMG = ImbalanceQueenManyMinorsMG
	baseImbalanceQueenManyMinorsEG = ImbalanceQueenManyMinorsEG
}

// Accessors for baselines (evaluation.go values)
func DefaultPSQT_MG() [7][64]int { return basePSQT_MG }
func DefaultPSQT_EG() [7][64]int { return basePSQT_EG }

func DefaultPieceValueMG() [7]int { return basePieceValueMG }
func DefaultPieceValueEG() [7]int { return basePieceValueEG }

func DefaultPassedPawnPSQT_MG() [64]int { return basePassedPawnPSQT_MG }
func DefaultPassedPawnPSQT_EG() [64]int { return basePassedPawnPSQT_EG }

// Phase 3 mobility/attacker defaults
func DefaultMobilityValueMG() [7]int { return baseMobilityValueMG }
func DefaultMobilityValueEG() [7]int { return baseMobilityValueEG }
func DefaultAttackerInner() [7]int   { return baseAttackerInner }
func DefaultAttackerOuter() [7]int   { return baseAttackerOuter }

// Phase 1 scalar defaults
func DefaultBishopPairBonusMG() int        { return baseBishopPairBonusMG }
func DefaultBishopPairBonusEG() int        { return baseBishopPairBonusEG }
func DefaultRookSemiOpenFileBonusMG() int  { return baseRookSemiOpenFileBonusMG }
func DefaultRookOpenFileBonusMG() int      { return baseRookOpenFileBonusMG }
func DefaultSeventhRankBonusEG() int       { return baseSeventhRankBonusEG }
func DefaultCentralizedQueenBonusEG() int  { return baseCentralizedQueenBonusEG }
func DefaultQueenInfiltrationBonusMG() int { return baseQueenInfiltrationBonusMG }
func DefaultQueenInfiltrationBonusEG() int { return baseQueenInfiltrationBonusEG }

// Phase 2 pawn structure defaults
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
func DefaultPawnLeverMG() int           { return basePawnLeverMG }
func DefaultPawnLeverEG() int           { return basePawnLeverEG }
func DefaultWeakLeverPenaltyMG() int    { return baseWeakLeverPenaltyMG }
func DefaultWeakLeverPenaltyEG() int    { return baseWeakLeverPenaltyEG }

// King safety table
func DefaultKingSafetyTable() [100]int { return baseKingSafetyTable }

// King safety correlated defaults
func DefaultKingSemiOpenFilePenalty() int    { return baseKingSemiOpenFilePenalty }
func DefaultKingOpenFilePenalty() int        { return baseKingOpenFilePenalty }
func DefaultKingMinorPieceDefenseBonus() int { return baseKingMinorPieceDefenseBonus }
func DefaultKingPawnDefenseMG() int          { return baseKingPawnDefenseMG }

// Weak squares + tempo
func DefaultWeakSquaresPenaltyMG() int     { return baseWeakSquaresPenaltyMG }
func DefaultWeakKingSquaresPenaltyMG() int { return baseWeakKingSquaresPenaltyMG }
func DefaultTempoBonus() int               { return baseTempoBonus }

// Phase 5 extras defaults
func DefaultKnightOutpostMG() int        { return baseKnightOutpostMG }
func DefaultKnightOutpostEG() int        { return baseKnightOutpostEG }
func DefaultBishopOutpostMG() int        { return baseBishopOutpostMG }
func DefaultKnightCanAttackPieceMG() int { return baseKnightCanAttackPieceMG }
func DefaultKnightCanAttackPieceEG() int { return baseKnightCanAttackPieceEG }
func DefaultStackedRooksMG() int         { return baseStackedRooksMG }
func DefaultRookXrayQueenMG() int        { return baseRookXrayQueenMG }
func DefaultConnectedRooksBonusMG() int  { return baseConnectedRooksBonusMG }

// New accessors for bishop xray and pawn storm family
func DefaultBishopXrayKingMG() int        { return baseBishopXrayKingMG }
func DefaultBishopXrayRookMG() int        { return baseBishopXrayRookMG }
func DefaultBishopXrayQueenMG() int       { return baseBishopXrayQueenMG }
func DefaultPawnStormMG() int             { return basePawnStormMG }
func DefaultPawnProximityPenaltyMG() int  { return basePawnProximityPenaltyMG }
func DefaultPawnLeverStormPenaltyMG() int { return basePawnLeverStormPenaltyMG }

// Imbalance defaults
func DefaultImbalanceKnightPerPawnMG() int    { return baseImbalanceKnightPerPawnMG }
func DefaultImbalanceKnightPerPawnEG() int    { return baseImbalanceKnightPerPawnEG }
func DefaultImbalanceBishopPerPawnMG() int    { return baseImbalanceBishopPerPawnMG }
func DefaultImbalanceBishopPerPawnEG() int    { return baseImbalanceBishopPerPawnEG }
func DefaultImbalanceMinorsForMajorMG() int   { return baseImbalanceMinorsForMajorMG }
func DefaultImbalanceMinorsForMajorEG() int   { return baseImbalanceMinorsForMajorEG }
func DefaultImbalanceRedundantRookMG() int    { return baseImbalanceRedundantRookMG }
func DefaultImbalanceRedundantRookEG() int    { return baseImbalanceRedundantRookEG }
func DefaultImbalanceRookQueenOverlapMG() int { return baseImbalanceRookQueenOverlapMG }
func DefaultImbalanceRookQueenOverlapEG() int { return baseImbalanceRookQueenOverlapEG }
func DefaultImbalanceQueenManyMinorsMG() int  { return baseImbalanceQueenManyMinorsMG }
func DefaultImbalanceQueenManyMinorsEG() int  { return baseImbalanceQueenManyMinorsEG }
