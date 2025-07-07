//go:build amd64
// +build amd64

#include "textflag.h"

// Маска VPSHUFB: для каждого из 8 пикселей (4 байта) делает
// [B,G,R,A]→[R,G,B,A]
DATA ·shuffleMask+0x00(SB)/8, $0x07040506_03000102

// Следующие 8 байт: пиксели 2 и 3
DATA ·shuffleMask+0x08(SB)/8, $0x0F0C0D0E_0B08090A

// [4,5]
DATA ·shuffleMask+0x10(SB)/8, $0x17141516_13101112

// [6,7]
DATA ·shuffleMask+0x18(SB)/8, $0x1F1C1D1E_1B18191A

GLOBL ·shuffleMask(SB), RODATA|NOPTR, $32

// void swapBGRtoRGB_AVX2_asm(uint8_t* src, int len, uint8_t* dst)
TEXT ·swapBGRtoRGB_AVX2_asm(SB), NOSPLIT, $0-24
    // стек: [srcBase+0][srcLen+8][dstBase+16]
    MOVQ srcBase+0(FP), SI
    MOVQ srcLen+8(FP), CX
    MOVQ dstBase+16(FP), DI

    // загрузили маску в Y0
    VMOVDQU ·shuffleMask(SB), Y0

mainAVX:
    CMPQ CX, $32
    JL tailScalar

    VMOVDQU (SI), Y1
    VPSHUFB Y0, Y1, Y2
    VMOVDQU Y2, (DI)

    ADDQ $32, SI
    ADDQ $32, DI
    SUBQ $32, CX
    JMP mainAVX

tailScalar:
    CMPQ CX, $4
    JL done

scalarLoop:
    // переставляем BGRx->RGBx по-байтно
    MOVBLZX 2(SI), AX
    MOVB    AL,    (DI)
    MOVBLZX 1(SI), AX
    MOVB    AL, 1(DI)
    MOVBLZX 0(SI), AX
    MOVB    AL, 2(DI)
    MOVBLZX 3(SI), AX
    MOVB    AL, 3(DI)

    ADDQ $4, SI
    ADDQ $4, DI
    SUBQ $4, CX
    CMPQ CX, $4
    JGE scalarLoop

done:
    VZEROUPPER
    RET
