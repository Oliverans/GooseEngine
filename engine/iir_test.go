package engine

import (
	"testing"

	gm "chess-engine/goosemg"
)

func TestIIRDepth(t *testing.T) {
	oldDepth := IIRMinDepth
	IIRMinDepth = 4
	t.Cleanup(func() { IIRMinDepth = oldDepth })

	ttMove := gm.NewMove(1, 18, gm.WhiteKnight, gm.NoPiece, gm.NoPiece, gm.FlagNone)
	tests := []struct {
		name        string
		depth       int8
		ttMove      gm.Move
		inCheck     bool
		wantDepth   int8
		wantReduced bool
	}{
		{"below minimum", 3, 0, false, 3, false},
		{"at minimum", 4, 0, false, 3, true},
		{"above minimum", 7, 0, false, 6, true},
		{"TT move", 4, ttMove, false, 4, false},
		{"in check", 4, 0, true, 4, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			depth, reduced := iirDepth(test.depth, test.ttMove, test.inCheck)
			if depth != test.wantDepth || reduced != test.wantReduced {
				t.Fatalf("iirDepth() = (%d, %v), want (%d, %v)", depth, reduced, test.wantDepth, test.wantReduced)
			}
		})
	}
}
