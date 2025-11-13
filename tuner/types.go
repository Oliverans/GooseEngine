// tuner/types.go
package tuner

type Sample struct {
	Pieces [6][]int // white pieces (P,N,B,R,Q,K)
	BP     [6][]int // black pieces
	STM    int      // 1 if white to move, 0 if black
	Label  float64  // 0, 0.5, 1
	PiecePhase int  // cached phase
}

type PST struct {
	MG [6][64]float64
	EG [6][64]float64
	K  float64
}

type BatchGrad struct {
	MG [6][64]float64
	EG [6][64]float64
	Dk   float64
	Loss float64
	N    int
}

type AdaGrad struct {
    G         []float64
    LR, Eps   float64
    L2        float64
    LRScale   []float64
}

type TrainConfig struct {
	Epochs    int
	Batch     int
	LR        float64
	L2        float64
	AutoK     bool
	Shuffle   bool
	KRefitCap int
}
