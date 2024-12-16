package engine

import (
	"math/bits"

	"github.com/dylhunn/dragontoothmg"
)

var KingMoves [65]uint64

var SeePieceValue = [7]int{
	dragontoothmg.King:   5000,
	dragontoothmg.Pawn:   100,
	dragontoothmg.Knight: 300,
	dragontoothmg.Bishop: 300,
	dragontoothmg.Rook:   500,
	dragontoothmg.Queen:  900}

func see(b *dragontoothmg.Board, move dragontoothmg.Move, debug bool) int {
	// Prepare values
	var gain = [32]int{}
	var depth uint8 = 0
	sideToMove := b.Wtomove

	// Get initial squares of capture move
	initSquare := move.From()
	targetSquare := move.To()

	// Get bitboard of all pieces attacking square
	whitePieceAttackers := getPiecesAttackingSquare(targetSquare, b.White, b.Black, true)
	blackPieceAttackers := getPiecesAttackingSquare(targetSquare, b.Black, b.White, false)
	attadef := (whitePieceAttackers | blackPieceAttackers)

	// Get initial pieces captured
	var targetPiece dragontoothmg.Piece
	var attacker dragontoothmg.Piece
	if sideToMove {
		targetPiece, _ = GetPieceTypeAtPosition(targetSquare, &b.Black)
		attacker, _ = GetPieceTypeAtPosition(initSquare, &b.White)
	} else {
		targetPiece, _ = GetPieceTypeAtPosition(targetSquare, &b.White)
		attacker, _ = GetPieceTypeAtPosition(initSquare, &b.Black)
	}

	// Ugly en passant-capturer, we should only get valid captures in SEE anyway...
	if targetPiece == 0 {
		targetPiece = 1
	}

	var attackerBB = PositionBB[initSquare]
	gain[depth] = SeePieceValue[targetPiece]
	//attadef &^= attackerBB

	if debug {
		println("fen: ", b.ToFen(), "\tboard: ", b.White.All|b.Black.All, "\tfrom: ", initSquare, "\tto: ", targetSquare, "\tattadef: ", attadef)
		println("depth: ", depth, "\tside: ", sideToMove, "\tattacker: ", attacker, "\tpiece taken: ", targetPiece, "\tnew score: ", gain[depth], "\tAttadef: ", attadef, "\tAttackerBB: ", attackerBB)
	}

	sideToMove = !sideToMove

	// We "already made the first move", so we swap side before going into the loop
	//sideToMove = !sideToMove
	for done := true; done; done = attackerBB != 0 {
		depth++
		gain[depth] = SeePieceValue[attacker] - gain[depth-1]

		if attadef != 0 && debug {
			println("depth: ", depth, "\tGain: ", gain[depth], "\tComparison: ", SeePieceValue[attacker], "-", gain[depth-1], " = ", SeePieceValue[attacker]-gain[depth-1], "\tside to move: ", sideToMove, "\t --------- ", "attacker: ", attacker)
		}

		// If we're in a losing position after the last trade, we break
		if max(-gain[depth-1], gain[depth]) < 0 {
			break
		}

		attadef ^= attackerBB
		if debug {
			println("Depth: ", depth, "Attadef: ", attadef)
		}

		attackerBB, attacker = getClosestAttacker(b, attadef, sideToMove, targetSquare)
		sideToMove = !sideToMove
	}

	for x := depth - 1; x > 0; x-- {
		gain[x-1] = -max(-gain[x-1], gain[x])
		if debug {
			println("Depth: ", x-1, "Highest value: ", gain[x-1])
		}
	}

	if debug {
		println("Gain: ", gain[0])
	}

	return gain[0]
}

func getPiecesAttackingSquare(targetSquare uint8, usBB dragontoothmg.Bitboards, enemyBB dragontoothmg.Bitboards, sideToMove bool) uint64 {
	/*
		Calculate attacks from "supersquare" - if this square was one of every type of piece, what can it hit?
		Has to take into account xraying from diagonal & orthogonal movement (love how the chess programming wiki expands my vocabulary!)
		Currently, our bishops/rooks/queens don't xray through opponent bishops/rooks/queens... Fix? Or no? Probably not, unless we dynamically update
		the attadef as we go ...
		Xray through pawns
	*/
	orthogonalAttacksXray := dragontoothmg.CalculateRookMoveBitboard(targetSquare, ((usBB.All & ^(usBB.Rooks|usBB.Queens))|(enemyBB.All & ^(enemyBB.Rooks|enemyBB.Queens)))) & ^(usBB.All & ^(usBB.Rooks | usBB.Queens | enemyBB.Rooks | enemyBB.Queens))

	var attackBB uint64
	var pawnBB uint64

	targetBB := PositionBB[targetSquare]

	// Check which of our pawns we can xray through
	for x := usBB.Pawns; x != 0; x &= x - 1 {
		bb := PositionBB[bits.TrailingZeros64(x)]
		var pawnAttackBBEast, pawnAttackBBWest uint64
		pawnAttackBBEast, pawnAttackBBWest = PawnCaptureBitboards(bb, sideToMove)
		if ((pawnAttackBBEast | pawnAttackBBWest) & targetBB) > 0 {
			attackBB |= bb
			pawnBB |= bb
		}
	}

	diagonalAttacksXray := dragontoothmg.CalculateBishopMoveBitboard(targetSquare, ((usBB.All & ^(usBB.Bishops|usBB.Queens|pawnBB))|enemyBB.All)) & ^(usBB.All & ^(usBB.Bishops | usBB.Queens)) //(^usBB.All & ^(usBB.Bishops | usBB.Queens))

	hitPieces := attackBB | orthogonalAttacksXray&(usBB.Rooks|usBB.Queens)
	hitPieces |= diagonalAttacksXray & (usBB.Bishops | usBB.Queens)
	hitPieces |= KnightMasks[targetSquare] & usBB.Knights
	hitPieces |= KingMoves[targetSquare] & usBB.Kings

	return hitPieces
}

func getClosestAttacker(b *dragontoothmg.Board, attadef uint64, sideToMove bool, targetSquare uint8) (uint64, dragontoothmg.Piece) {
	var usBB dragontoothmg.Bitboards
	if sideToMove {
		usBB = b.White
	} else {
		usBB = b.Black
	}
	// Get closest diagonal hit
	diagonalAttack := dragontoothmg.CalculateBishopMoveBitboard(targetSquare, attadef) & ^(usBB.All &^ (usBB.Bishops | usBB.Queens))
	diagonalAttack &= attadef
	//println("Diagonal attack: ", diagonalAttack)

	// Get closest orthogonal hit
	orthogonalAttack := dragontoothmg.CalculateRookMoveBitboard(targetSquare, attadef) & ^(usBB.All & ^(usBB.Rooks | usBB.Queens))
	orthogonalAttack &= attadef
	//println("Orthogonal attack: ", orthogonalAttack)

	east, west := PawnCaptureBitboards(PositionBB[targetSquare], !sideToMove)
	hitPieces := ((east | west) | diagonalAttack | orthogonalAttack | (KnightMasks[targetSquare] & usBB.Knights)) & attadef
	return minAttacker(hitPieces, usBB)
}

func minAttacker(attadef uint64, bb dragontoothmg.Bitboards) (uint64, dragontoothmg.Piece) {
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
		// ... Or it used to be here, but I'm too cool for school! I create my own failures instead
		//Definitely ignore this last sentence.
		return PositionBB[bits.TrailingZeros64(subset)], piece
	}

	return 0, piece
}
