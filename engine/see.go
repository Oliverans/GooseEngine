package engine

import (
	"math/bits"
	"time"

	"github.com/dylhunn/dragontoothmg"
)

var KingMoves [65]uint64

var seeTime time.Duration
var SeePieceValue = map[dragontoothmg.Piece]int{dragontoothmg.King: 5000, dragontoothmg.Pawn: 100, dragontoothmg.Knight: 300, dragontoothmg.Bishop: 300, dragontoothmg.Rook: 500, dragontoothmg.Queen: 900}

func see(b *dragontoothmg.Board, move dragontoothmg.Move, debug bool) int {
	/*
		TWO TEST POSITIONS:
		- test position, white and black pieces shouldn't be able to hit because they're blocked (opponent pieces may decide not to capture ...):
			3kq3/4r3/2n1R3/4p3/3b4/2B2N2/1Q6/3K4 w - - 0 1
			Black should end up -4, a rook up after making all trades
			xraying through enemy pieces shouldn't be possible
			unsure about ... xraying enemy pieces ... probably not

		- test position, just normal captures...
			3kq3/4r3/2n2b2/4p3/8/2B2N2/1Q2R3/3K4 w - - 0 1
			Black should end up -8, a queen up after making all trades

		- test position, uneven material quality
			3kq3/4r3/2n1r3/4p2r/2N5/2B2NB1/1Q6/3K4 w - - 0 1
			white should end up +2 when the capturing ends
			not completing all trades, as black is down too much material

	*/

	gain := [32]int{}
	depth := uint8(0)
	sideToMove := b.Wtomove
	initSquare := move.From()
	targetSquare := move.To()

	whitePieceAttackers := getPiecesAttackingSquare(b, targetSquare, b.White, b.Black, true)
	blackPieceAttackers := getPiecesAttackingSquare(b, targetSquare, b.Black, b.White, false)
	attadef := (whitePieceAttackers | blackPieceAttackers)
	if attadef == 0 {
		return 0
	}

	if debug {
		println("fen: ", b.ToFen(), "\tboard: ", b.White.All|b.Black.All, "\tfrom: ", initSquare, "\tto: ", targetSquare)
		println("Attadef: ", attadef)
	}
	var targetPiece dragontoothmg.Piece
	var attacker dragontoothmg.Piece
	if sideToMove {
		targetPiece, _ = GetPieceTypeAtPosition(targetSquare, b.Black)
		attacker, _ = GetPieceTypeAtPosition(initSquare, b.White)
	} else {
		targetPiece, _ = GetPieceTypeAtPosition(targetSquare, b.White)
		attacker, _ = GetPieceTypeAtPosition(initSquare, b.Black)
	}

	gain[depth] = SeePieceValue[targetPiece]
	var attackerBB = PositionBB[initSquare]
	attadef ^= attackerBB
	if debug {
		println("depth: ", depth, "\tside: ", sideToMove, "\tattacker: ", attacker, "\tpiece taken: ", targetPiece, "\tnew score: ", gain[depth], "\tbb: ", attadef)
	}
	sideToMove = !sideToMove

	for ok := true; ok; ok = attadef != 0 {
		depth++
		gain[depth] = SeePieceValue[attacker] - gain[depth-1]

		//if attadef == 0 {
		//	break
		//}

		sideToMove = !sideToMove
		attackerBB, attacker = getClosestAttacker(b, attadef, sideToMove, targetSquare)
		attadef ^= attackerBB

		if attadef != 0 && debug {
			println("depth: ", depth, "\tside: ", !sideToMove, "\tattacker: ", attacker, "\tcurr score: ", gain[depth], "\tprev score: ", -gain[depth-1], "\tpiece score: ", SeePieceValue[attacker], "\tbb: ", attadef, "\tattackerBB: ", attackerBB)
		}

		//attadef ^= attackerBB

		if (max(-gain[depth-1], gain[depth]) < 0) || attackerBB == 0 {
			break
		}
	}

	for depth = depth; depth > 0; depth-- {
		if debug {
			println("Depth: ", depth, "\tscore: ", gain[depth-1])
		}
		gain[depth-1] = -max(-gain[depth-1], gain[depth])
	}

	if debug {
		println("Gain: ", gain[0])
	}
	return gain[0]
}

func getClosestAttacker(b *dragontoothmg.Board, attadef uint64, sideToMove bool, targetSquare uint8) (uint64, dragontoothmg.Piece) {

	bb := b.White
	themBB := b.Black
	if !sideToMove {
		bb = b.Black
		themBB = b.White
	}
	// Get closest diagonal hit
	diagonalAttack := dragontoothmg.CalculateBishopMoveBitboard(targetSquare, attadef) & ^(bb.All)
	diagonalAttack &= attadef

	// Get closest orthogonal hit
	orthogonalAttack := dragontoothmg.CalculateRookMoveBitboard(targetSquare, attadef) & ^(bb.All)
	orthogonalAttack &= attadef

	east, west := PawnCaptureBitboards(PositionBB[targetSquare], !sideToMove)
	hitPieces := ((east | west) | diagonalAttack | orthogonalAttack | (KnightMasks[targetSquare] & themBB.Knights)) & attadef
	return minAttacker(b, hitPieces, sideToMove)
}

func getPiecesAttackingSquare(b *dragontoothmg.Board, targetSquare uint8, usBB dragontoothmg.Bitboards, enemyBB dragontoothmg.Bitboards, sideToMove bool) uint64 {
	// Calculate attacks from "supersquare" - if this square was one of every type of piece, what can it hit?
	// Has to take into account xraying from diagonal & orthogonal movement
	// Currently, our bishops/rooks/queens don't xray through opponent bishops/rooks/queens... Fix? Or no? Prolly not.
	diagonalAttacksXray := dragontoothmg.CalculateBishopMoveBitboard(targetSquare, ((usBB.All & ^(usBB.Bishops|usBB.Queens))|enemyBB.All)) & ^(usBB.All & ^(usBB.Bishops | usBB.Queens)) //(^usBB.All & ^(usBB.Bishops | usBB.Queens))
	orthogonalAttacksXray := dragontoothmg.CalculateRookMoveBitboard(targetSquare, ((usBB.All & ^(usBB.Rooks|usBB.Queens))|enemyBB.All)) & ^(usBB.All & ^(usBB.Rooks | usBB.Queens))

	east, west := PawnCaptureBitboards(PositionBB[targetSquare], !sideToMove)
	hitPieces := (east | west) & usBB.Pawns
	hitPieces |= orthogonalAttacksXray & (usBB.Rooks | usBB.Queens)
	hitPieces |= diagonalAttacksXray & (usBB.Bishops | usBB.Queens)
	hitPieces |= KnightMasks[targetSquare] & usBB.Knights
	hitPieces |= KingMoves[targetSquare] & usBB.Kings

	return hitPieces
}

func minAttacker(b *dragontoothmg.Board, attadef uint64, sideToMove bool) (uint64, dragontoothmg.Piece) {
	bb := b.Black
	if !sideToMove {
		bb = b.White
	}

	var subset uint64
	var piece dragontoothmg.Piece
	if attadef&bb.Pawns > 0 {
		subset = attadef & bb.Pawns
		piece = dragontoothmg.Pawn
	} else if attadef&bb.Knights > 0 {
		subset = attadef & bb.Knights
		piece = dragontoothmg.Knight
	} else if attadef&bb.Bishops > 0 {
		subset = attadef & bb.Bishops
		piece = dragontoothmg.Bishop
	} else if attadef&bb.Rooks > 0 {
		subset = attadef & bb.Rooks
		piece = dragontoothmg.Rook
	} else if attadef&bb.Queens > 0 {
		subset = attadef & bb.Queens
		piece = dragontoothmg.Queen
	} else if attadef&bb.Kings > 0 {
		subset = attadef & bb.Kings
		piece = dragontoothmg.King
	}

	if subset != 0 {
		// Bit-twidling to return a single bit if there are multiple bits.
		// ... Or it used to be here, but I'm too cool for school! I create my own failures instead.
		return PositionBB[bits.TrailingZeros64(subset)], piece
	}

	return 0, piece
}
