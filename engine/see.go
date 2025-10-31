package engine

import (
	"math/bits"

	gm "chess-engine/goosemg"
)

var KingMoves [65]uint64

var SeePieceValue = [7]int{
	gm.PieceTypeKing:   5000,
	gm.PieceTypePawn:   100,
	gm.PieceTypeKnight: 300,
	gm.PieceTypeBishop: 300,
	gm.PieceTypeRook:   500,
	gm.PieceTypeQueen:  900}

func see(b *gm.Board, move gm.Move, debug bool) int {
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
	var targetPiece gm.PieceType
	var attacker gm.PieceType
	if sideToMove {
		targetPiece, _ = GetPieceTypeAtPosition(uint8(targetSquare), &b.Black)
		attacker, _ = GetPieceTypeAtPosition(uint8(initSquare), &b.White)
	} else {
		targetPiece, _ = GetPieceTypeAtPosition(uint8(targetSquare), &b.White)
		attacker, _ = GetPieceTypeAtPosition(uint8(initSquare), &b.Black)
	}

	// Ugly en passant-capturer, we should only get valid captures in SEE anyway...
	if targetPiece == gm.PieceTypeNone {
		targetPiece = gm.PieceTypePawn
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

func getPiecesAttackingSquare(targetSquare gm.Square, usBB gm.Bitboards, enemyBB gm.Bitboards, sideToMove bool) uint64 {
	/*
		Calculate attacks from "supersquare" - if this square was one of every type of piece, what can it hit?
		Has to take into account xraying from diagonal & orthogonal movement (love how the chess programming wiki expands my vocabulary!)
		Currently, our bishops/rooks/queens don't xray through opponent bishops/rooks/queens... Fix? Or no? Probably not, unless we dynamically update
		the attadef as we go ...
		Xray through pawns
	*/
	ts := uint8(targetSquare)
	orthogonalAttacksXray := gm.CalculateRookMoveBitboard(ts, ((usBB.All & ^(usBB.Rooks|usBB.Queens))|(enemyBB.All & ^(enemyBB.Rooks|enemyBB.Queens)))) & ^(usBB.All & ^(usBB.Rooks | usBB.Queens | enemyBB.Rooks | enemyBB.Queens))

	var attackBB uint64
	var pawnBB uint64

	targetBB := PositionBB[int(targetSquare)]

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

	diagonalAttacksXray := gm.CalculateBishopMoveBitboard(ts, ((usBB.All & ^(usBB.Bishops|usBB.Queens|pawnBB))|enemyBB.All)) & ^(usBB.All & ^(usBB.Bishops | usBB.Queens)) //(^usBB.All & ^(usBB.Bishops | usBB.Queens))

	hitPieces := attackBB | orthogonalAttacksXray&(usBB.Rooks|usBB.Queens)
	hitPieces |= diagonalAttacksXray & (usBB.Bishops | usBB.Queens)
	hitPieces |= KnightMasks[int(targetSquare)] & usBB.Knights
	hitPieces |= KingMoves[int(targetSquare)] & usBB.Kings

	return hitPieces
}

func getClosestAttacker(b *gm.Board, attadef uint64, sideToMove bool, targetSquare gm.Square) (uint64, gm.PieceType) {
	var usBB gm.Bitboards
	if sideToMove {
		usBB = b.White
	} else {
		usBB = b.Black
	}
	// Get closest diagonal hit
	ts := uint8(targetSquare)
	diagonalAttack := gm.CalculateBishopMoveBitboard(ts, attadef) & ^(usBB.All &^ (usBB.Bishops | usBB.Queens))
	diagonalAttack &= attadef
	//println("Diagonal attack: ", diagonalAttack)

	// Get closest orthogonal hit
	orthogonalAttack := gm.CalculateRookMoveBitboard(ts, attadef) & ^(usBB.All & ^(usBB.Rooks | usBB.Queens))
	orthogonalAttack &= attadef
	//println("Orthogonal attack: ", orthogonalAttack)

	east, west := PawnCaptureBitboards(PositionBB[int(targetSquare)], !sideToMove)
	hitPieces := ((east | west) | diagonalAttack | orthogonalAttack | (KnightMasks[targetSquare] & usBB.Knights)) & attadef
	return minAttacker(hitPieces, usBB)
}

func minAttacker(attadef uint64, bb gm.Bitboards) (uint64, gm.PieceType) {
	var subset uint64
	var piece gm.PieceType

	if attadef&bb.Pawns > 0 {
		subset = attadef & bb.Pawns
		piece = gm.PieceTypePawn
	} else if attadef&bb.Knights > 0 {
		subset = attadef & bb.Knights
		piece = gm.PieceTypeKnight
	} else if attadef&bb.Bishops > 0 {
		subset = attadef & bb.Bishops
		piece = gm.PieceTypeBishop
	} else if attadef&bb.Rooks > 0 {
		subset = attadef & bb.Rooks
		piece = gm.PieceTypeRook
	} else if attadef&bb.Queens > 0 {
		subset = attadef & bb.Queens
		piece = gm.PieceTypeQueen
	} else if attadef&bb.Kings > 0 {
		subset = attadef & bb.Kings
		piece = gm.PieceTypeKing
	}

	if subset != 0 {
		// Bit-twidling to return a single bit if there are multiple bits.
		// ... Or it used to be here, but I'm too cool for school! I create my own failures instead
		//Definitely ignore this last sentence.
		return PositionBB[bits.TrailingZeros64(subset)], piece
	}

	return 0, piece
}
