// tuner/types.go
package tuner

type Sample struct {
	Pieces     [6][]int // white pieces (P,N,B,R,Q,K)
	BP         [6][]int // black pieces
	STM        int      // 1 if white to move, 0 if black
	Label      float64  // 0, 0.5, 1
	PiecePhase int      // cached phase
}

type PST struct {
	MG [6][64]float64
	EG [6][64]float64
	K  float64
}

type BatchGrad struct {
	MG   [6][64]float64
	EG   [6][64]float64
	Dk   float64
	Loss float64
	N    int
}

type AdaGrad struct {
	G       []float64
	LR, Eps float64
	LRScale []float64
}

type TrainConfig struct {
	Epochs     int
	Batch      int
	LR         float64
	AutoK      bool
	Shuffle    bool
	KRefitCap  int
	LRScaling  bool
	Anchoring  bool
	LRScaleCfg LRScaleConfig
	AnchorCfg  AnchorConfig
	StatePath  string // per-epoch state output (optional)

	// Validation split (optional)
	ValCap         int     // fixed-size validation cap (0 = unused)
	ValFrac        float64 // fraction of data for validation (0 = unused)
	UseKRefitAsVal bool    // reuse K-refit holdout as validation

	// Reduce-on-plateau + early stopping (optional)
	PlateauPatience   int
	PlateauMinDelta   float64
	LRReduceFactor    float64
	LRMin             float64
	LRDropCooldown    int
	MaxLRDrops        int
	EarlyStopPatience int
}

// AnchorConfig defines per-parameter L2 anchor strengths.
type AnchorConfig struct {
	// Tier 1: Strong anchor (high lambda)
	Tier1Lambda float64

	// Tier 2: Moderate anchor
	Tier2Lambda float64

	// Tier 3: Light anchor
	Tier3Lambda float64

	// Free parameters
	FreeLambda float64
}

// LRScaleConfig defines per-parameter learning-rate multipliers.
// Values are typically in [0,1], where lower numbers reduce update magnitude.
type LRScaleConfig struct {
	// Tier 1: Strong constraint (0.1x LR)
	PawnPhalanx           float64
	PawnWeakLever         float64
	KnightOutpost         float64
	BishopOutpost         float64
	KingSemiOpenFile      float64
	KingOpenFile          float64
	PawnStormLeverPct     float64
	PawnStormWeakLeverPct float64
	PawnStormBlockedPct   float64

	// Tier 2: Moderate constraint (0.3x LR)
	BishopPair            float64
	RookFiles             float64
	SeventhRank           float64
	BackwardPawn          float64
	IsolatedPawn          float64
	DoubledPawn           float64
	CandidatePassed       float64
	KingDefense           float64
	Tempo                 float64
	PawnStormBaseMG       float64
	PawnStormOppositeMult float64

	// Tier 3: Light constraint (0.5x LR)
	ConnectedPawn float64
	BlockedPawn   float64
	BadBishop     float64
	KnightTropism float64
	StackedRooks  float64
	Material      float64

	// Free parameters (1.0x LR)
	PST             float64
	PasserPSQT      float64
	Mobility        float64
	KingSafetyTable float64
	Imbalance       float64
	Space           float64
}

// DefaultAnchorConfig returns recommended anchor strengths per tier.
func DefaultAnchorConfig() AnchorConfig {
	return AnchorConfig{
		Tier1Lambda: 0.1,
		Tier2Lambda: 0.01,
		Tier3Lambda: 0.01,
		FreeLambda:  0.001, // minimal regularization for stability
	}
}
