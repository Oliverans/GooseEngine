package engine

import (
	gm "chess-engine/goosemg"
)

// SEE exposes static exchange evaluation for external callers.
func SEE(b *gm.Board, m gm.Move) int32 {
	return int32(see(b, m, false))
}
