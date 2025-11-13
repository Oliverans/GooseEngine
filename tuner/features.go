// tuner/features.go
package tuner

import (
    gm "chess-engine/goosemg"
)

// Position aliases engine board to keep the interface stable while
// keeping tuner decoupled from engine package details.
// TODO: If engine exposes a different root type later, update alias.
type Position = gm.Board

// Featurizer defines the interface for linearizable evaluation features.
// Eval returns a white-positive scalar evaluation (in centipawns scale).
// Grad accumulates dE/dθ scaled by `scale` into g (same layout as Params).
// Params returns the current parameter vector (backed by internal storage).
// SetParams replaces the current parameters with the provided vector.
//
// Note: θ layout and wiring are established in step 2. For step 1 we
// scaffold the interface and a baseline LinearEval implementation.
type Featurizer interface {
    Eval(pos *Position) float64
    Grad(pos *Position, scale float64, g []float64)
    Params() []float64
    SetParams([]float64)
}

// NewBoardFromSample creates a minimal engine board from a Sample suitable for
// evaluation. It places pieces, sets side-to-move, clears castling and en passant,
// and computes a consistent zobrist hash. Intended for feature eval only.
func NewBoardFromSample(s Sample) *Position {
    b := &gm.Board{}
    // Place white pieces
    for _, sq := range s.Pieces[P] { b.SetPiece(gm.Square(sq), gm.WhitePawn) }
    for _, sq := range s.Pieces[N] { b.SetPiece(gm.Square(sq), gm.WhiteKnight) }
    for _, sq := range s.Pieces[B] { b.SetPiece(gm.Square(sq), gm.WhiteBishop) }
    for _, sq := range s.Pieces[R] { b.SetPiece(gm.Square(sq), gm.WhiteRook) }
    for _, sq := range s.Pieces[Q] { b.SetPiece(gm.Square(sq), gm.WhiteQueen) }
    for _, sq := range s.Pieces[K] { b.SetPiece(gm.Square(sq), gm.WhiteKing) }
    // Place black pieces (mirror indices already handled at feature level when needed)
    for _, sq := range s.BP[P] { b.SetPiece(gm.Square(sq), gm.BlackPawn) }
    for _, sq := range s.BP[N] { b.SetPiece(gm.Square(sq), gm.BlackKnight) }
    for _, sq := range s.BP[B] { b.SetPiece(gm.Square(sq), gm.BlackBishop) }
    for _, sq := range s.BP[R] { b.SetPiece(gm.Square(sq), gm.BlackRook) }
    for _, sq := range s.BP[Q] { b.SetPiece(gm.Square(sq), gm.BlackQueen) }
    for _, sq := range s.BP[K] { b.SetPiece(gm.Square(sq), gm.BlackKing) }

    if s.STM == 1 {
        b.SetSideToMove(gm.White)
    } else {
        b.SetSideToMove(gm.Black)
    }
    // Sync exported bitboards and fields to reflect piece placement
    b.White = b.WhiteBitboards()
    b.Black = b.BlackBitboards()
    b.Wtomove = b.SideToMove() == gm.White
    // Ensure consistent zobrist key (optional for features)
    _ = b.ComputeZobrist()
    return b
}
