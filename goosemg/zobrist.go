package goosemg

import "math/rand"

// Zobrist hashing tables for pieces, castling, en passant, and side to move.
var zobristPiece [15][64]uint64 // Zobrist keys for piece (index by piece code) on each square
var zobristCastle [16]uint64    // Zobrist keys for each castling rights state (0-15)
var zobristEnPassant [8]uint64  // Zobrist keys for en passant file (file 0-7)
var zobristSide uint64          // Zobrist key for side to move (Black to move)

// Initialize Zobrist keys (called on package init)
func init() {
    initZobrist()
}

func initZobrist() {
    // Use a fixed seed for reproducibility in tests
    rnd := rand.New(rand.NewSource(0xC0DE))

    // Piece keys
    for p := 0; p < 15; p++ {
        for sq := 0; sq < 64; sq++ {
            zobristPiece[p][sq] = rnd.Uint64()
        }
    }

    // Castling rights keys
    for cr := 0; cr < 16; cr++ {
        zobristCastle[cr] = rnd.Uint64()
    }

    // En passant file keys
    for f := 0; f < 8; f++ {
        zobristEnPassant[f] = rnd.Uint64()
    }

    // Side to move key
    zobristSide = rnd.Uint64()
}

// ComputeZobrist calculates the Zobrist hash for the current board state.
func (b *Board) ComputeZobrist() uint64 {
    var key uint64

    // Pieces
    for sq := 0; sq < 64; sq++ {
        p := b.pieces[sq]
        if p != NoPiece {
            key ^= zobristPiece[p][sq]
        }
    }

    // Side to move (only XOR if Black to move; if White, no XOR needed)
    if b.sideToMove == Black {
        key ^= zobristSide
    }

    // Castling rights
    key ^= zobristCastle[int(b.castlingRights)]

    // En passant file (if any)
    if b.enPassantSquare != NoSquare {
        file := int(b.enPassantSquare % 8)
        key ^= zobristEnPassant[file]
    }

    return key
}
