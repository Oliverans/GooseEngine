//go:build amd64

#include "textflag.h"

// func pextBMI2(x, mask uint64) uint64
TEXT ·pextBMI2(SB), NOSPLIT, $0-24
    MOVQ x+0(FP), AX       // AX = x (source)
    MOVQ mask+8(FP), CX    // CX = mask

    // PEXTQ mask, src, dest
    // dest = PEXT(src, mask)  => AX = PEXT(AX, CX)
    PEXTQ CX, AX, AX

    MOVQ AX, ret+16(FP)    // store result
    RET
