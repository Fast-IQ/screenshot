// +build amd64,!appengine,!gccgo

#include "textflag.h"

// mask для VPSHUFB: меняем BGRx → RGBx
DATA shuffleMask<>+0x00(SB)/8, $0x0201000006020400
DATA shuffleMask<>+0x08(SB)/8, $0x0A0908000E0C0A00
DATA shuffleMask<>+0x10(SB)/8, $0x1211100016131400
DATA shuffleMask<>+0x18(SB)/8, $0x1A1918001E1C1A00
GLOBL shuffleMask<>(SB), (NOPTR|RODATA), $32

TEXT ·swapBGRtoRGB_AVX2(SB), NOSPLIT, $0-32
    MOVQ src_base+0(FP), SI       // src pointer
    MOVQ src_len+8(FP), CX        // length
    MOVQ dst_base+16(FP), DI      // dst pointer

    VMOVDQU shuffleMask<>(SB), Y0 // загружаем маску

loop_avx:
    CMPQ CX, $32
    JB   tail                     // если <32, идём к хвосту
    VMOVDQU (SI), Y1
    VPSHUFB Y0, Y1, Y1
    VMOVDQU Y1, (DI)
    ADDQ $32, SI
    ADDQ $32, DI
    SUBQ $32, CX
    JMP loop_avx

tail:
    TESTQ CX, CX
    JE done

scalar:
    MOVB 2(SI), R8B   // R
    MOVB 1(SI), R9B   // G
    MOVB (SI), R10B  // B
    MOVB 3(SI), R11B // A
    MOVB R8B, (DI)
    MOVB R9B, 1(DI)
    MOVB R10B, 2(DI)
    MOVB R11B, 3(DI)
    ADDQ $4, SI
    ADDQ $4, DI
    SUBQ $4, CX
    JNZ scalar

done:
    VZEROUPPER
    RET
