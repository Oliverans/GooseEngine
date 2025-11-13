// tuner/consts.go
package tuner

// Piece order
const (
	P = 0
	N = 1
	B = 2
	R = 3
	Q = 4
	K = 5
)

var pieceIndex = map[byte]int{
	'P': P, 'N': N, 'B': B, 'R': R, 'Q': Q, 'K': K,
	'p': P, 'n': N, 'b': B, 'r': R, 'q': Q, 'k': K,
}

// Flip-view for black pieces (mirror vertically)
var flipView = [64]int{
	56,57,58,59,60,61,62,63, 48,49,50,51,52,53,54,55,
	40,41,42,43,44,45,46,47, 32,33,34,35,36,37,38,39,
	24,25,26,27,28,29,30,31, 16,17,18,19,20,21,22,23,
	8,9,10,11,12,13,14,15,   0,1,2,3,4,5,6,7,
}

// Phase constants (match your engine if needed)
const (
	PawnPhase   = 0
	KnightPhase = 1
	BishopPhase = 1
	RookPhase   = 2
	QueenPhase  = 4
	TotalPhase  = PawnPhase*16 + KnightPhase*4 + BishopPhase*4 + RookPhase*4 + QueenPhase*2
)
