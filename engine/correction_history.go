package engine

import gm "chess-engine/goosemg"

const (
	correctionHistorySize   = 1 << 16
	correctionHistoryMask   = correctionHistorySize - 1
	correctionHistoryGrain  = int32(256)
	correctionHistoryMax    = int32(8192)
	correctionPawnWeight    = int64(1)
	correctionMinorWeight   = int64(1)
	correctionNonPawnWeight = int64(1)
)

type correctionHistory struct {
	pawn    [2][correctionHistorySize]int16
	minor   [2][correctionHistorySize]int16
	nonPawn [2][2][correctionHistorySize]int16
}

type correctionHistoryIndices struct {
	side         int
	pawn         int
	minor        int
	whiteNonPawn int
	blackNonPawn int
}

type correctionHistoryRead struct {
	pawn         int16
	minor        int16
	whiteNonPawn int16
	blackNonPawn int16
	correction   int32
}

type correctionHistoryUpdate struct {
	direction int8
	saturated bool
}

func mixStructurePart(value, salt uint64) uint64 {
	value += salt
	value = (value ^ (value >> 30)) * 0xBF58476D1CE4E5B9
	value = (value ^ (value >> 27)) * 0x94D049BB133111EB
	return value ^ (value >> 31)
}

func minorStructureKey(b *gm.Board) uint64 {
	return mixStructurePart(b.White.Knights, 0x243F6A8885A308D3) ^
		mixStructurePart(b.White.Bishops, 0x13198A2E03707344) ^
		mixStructurePart(b.Black.Knights, 0xA4093822299F31D0) ^
		mixStructurePart(b.Black.Bishops, 0x082EFA98EC4E6C89)
}

func nonPawnStructureKey(b *gm.Board, color gm.Color) uint64 {
	pieces := &b.White
	colorSalt := uint64(0x452821E638D01377)
	if color == gm.Black {
		pieces = &b.Black
		colorSalt = 0xBE5466CF34E90C6C
	}

	return mixStructurePart(pieces.Knights, colorSalt^0xC0AC29B7C97C50DD) ^
		mixStructurePart(pieces.Bishops, colorSalt^0x3F84D5B5B5470917) ^
		mixStructurePart(pieces.Rooks, colorSalt^0x9216D5D98979FB1B) ^
		mixStructurePart(pieces.Queens, colorSalt^0xD1310BA698DFB5AC) ^
		mixStructurePart(pieces.Kings, colorSalt^0x2FFD72DBD01ADFB7)
}

func correctionHistoryIndex(key uint64) int {
	return int(key & correctionHistoryMask)
}

func correctionIndices(b *gm.Board) correctionHistoryIndices {
	side := 0
	if !b.Wtomove {
		side = 1
	}
	return correctionHistoryIndices{
		side:         side,
		pawn:         correctionHistoryIndex(pawnStructureKey(b.White.Pawns, b.Black.Pawns)),
		minor:        correctionHistoryIndex(minorStructureKey(b)),
		whiteNonPawn: correctionHistoryIndex(nonPawnStructureKey(b, gm.White)),
		blackNonPawn: correctionHistoryIndex(nonPawnStructureKey(b, gm.Black)),
	}
}

func (h *correctionHistory) read(b *gm.Board) correctionHistoryRead {
	indices := correctionIndices(b)
	read := correctionHistoryRead{
		pawn:         h.pawn[indices.side][indices.pawn],
		minor:        h.minor[indices.side][indices.minor],
		whiteNonPawn: h.nonPawn[indices.side][0][indices.whiteNonPawn],
		blackNonPawn: h.nonPawn[indices.side][1][indices.blackNonPawn],
	}
	weighted := correctionPawnWeight*int64(read.pawn) +
		correctionMinorWeight*int64(read.minor) +
		correctionNonPawnWeight*(int64(read.whiteNonPawn)+int64(read.blackNonPawn))
	weight := correctionPawnWeight + correctionMinorWeight + 2*correctionNonPawnWeight
	read.correction = int32(weighted / (int64(correctionHistoryGrain) * weight))
	return read
}

func applyStaticCorrection(raw, correction int32) int32 {
	return Max32(-Checkmate+1, Min32(Checkmate-1, raw+correction))
}

func updateCorrectionEntry(entry *int16, target, weight int32) {
	value := int32(*entry)
	value += (target - value) * weight / 256
	value = Max32(-correctionHistoryMax, Min32(correctionHistoryMax, value))
	*entry = int16(value)
}

func (h *correctionHistory) update(b *gm.Board, depth int8, rawStaticEval, bestScore int32) correctionHistoryUpdate {
	indices := correctionIndices(b)
	target := int64(bestScore-rawStaticEval) * int64(correctionHistoryGrain)
	saturated := target > int64(correctionHistoryMax) || target < -int64(correctionHistoryMax)
	if target > int64(correctionHistoryMax) {
		target = int64(correctionHistoryMax)
	} else if target < -int64(correctionHistoryMax) {
		target = -int64(correctionHistoryMax)
	}
	weight := int32(2 * Min(int(depth)+1, 16))
	target32 := int32(target)
	updateCorrectionEntry(&h.pawn[indices.side][indices.pawn], target32, weight)
	updateCorrectionEntry(&h.minor[indices.side][indices.minor], target32, weight)
	updateCorrectionEntry(&h.nonPawn[indices.side][0][indices.whiteNonPawn], target32, weight)
	updateCorrectionEntry(&h.nonPawn[indices.side][1][indices.blackNonPawn], target32, weight)

	direction := int8(0)
	if target > 0 {
		direction = 1
	} else if target < 0 {
		direction = -1
	}
	return correctionHistoryUpdate{direction: direction, saturated: saturated}
}

func (h *correctionHistory) clear() {
	*h = correctionHistory{}
}

func correctionHistoryUpdateEligible(stopped, isRoot, inCheck, didNull bool, excludedMove gm.Move, legalMoves int,
	bestScore, correctedStaticEval int32, nodeBound int8, bestMoveCapture, bestMovePromotion bool) bool {
	if stopped || isRoot || inCheck || didNull || excludedMove != 0 || legalMoves == 0 ||
		abs32(bestScore) >= Checkmate || bestMoveCapture || bestMovePromotion {
		return false
	}

	switch ttBound(nodeBound) {
	case ExactFlag:
		return true
	case BetaFlag:
		return bestScore > correctedStaticEval
	case AlphaFlag:
		return bestScore < correctedStaticEval
	}
	return false
}
