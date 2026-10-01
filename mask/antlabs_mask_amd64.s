// Copyright 2021-2024 antlabs. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build amd64 && !wsutil_nosimd

#include "textflag.h"

// ============================================================================
// CPU 特性检测辅助
//
// 为什么不用 golang.org/x/sys/cpu: 本模块目前只有 klauspost/compress 一个
// 依赖, 为了两个寄存器级的检测再引入一个依赖不划算, 这里照抄
// runtime/internal/cpu 的做法自己实现。
// ============================================================================

// func cpuid(eaxArg, ecxArg uint32) (eax, ebx, ecx, edx uint32)
//
// CPUID 会改写 AX/BX/CX/DX。amd64 上 Go 的 g 寄存器是 R14(不再是 BX),
// 所以 BX 可以像 runtime 一样直接用作 CPUID 的输出寄存器。
TEXT ·cpuid(SB), NOSPLIT, $0-24
	MOVL	eaxArg+0(FP), AX
	MOVL	ecxArg+4(FP), CX
	CPUID
	MOVL	AX, eax+8(FP)
	MOVL	BX, ebx+12(FP)
	MOVL	CX, ecx+16(FP)
	MOVL	DX, edx+20(FP)
	RET

// func xgetbv() (eax, edx uint32)
//
// 读 XCR0 低 32 位。调用前必须确认 CPUID.1:ECX.OSXSAVE=1,
// 否则该指令直接 #UD(SIGILL)。
TEXT ·xgetbv(SB), NOSPLIT, $0-8
	MOVL	$0, CX
	XGETBV
	MOVL	AX, eax+0(FP)
	MOVL	DX, edx+4(FP)
	RET

// ============================================================================
// AVX2 内核 (256bit)
// ============================================================================

// func maskAVX2(payload []byte, key uint32)
//
// 用 AVX2 对 payload 就地做 XOR mask。
// 调用方需保证 CPU 支持 AVX2(VEX.256 编码的广播/XOR/读写), 见
// antlabs_mask_amd64.go 的运行时检测。
//
// AArch64 下 NEON 是架构基线, 这里不同: AVX2 是运行时特性, 内核本身
// 不能假设"被调用就一定支持", 但既然进入了本函数, 执行 VEX 指令是安全的。
//
// 内核不依赖调用方的长度前提: 任意长度(含 0)都能正确处理。
// 小尺寸下展开的标量实现(maskFast)更快, 由分发层按阈值选择;
// 单元测试会绕过分发层直接用任意长度调用内核, 这里必须兜住。
//
// 栈帧布局: payload_base=0(FP), payload_len=8(FP), payload_cap=16(FP), key=24(FP)
TEXT ·maskAVX2(SB), NOSPLIT, $0-28
	MOVQ	payload_base+0(FP), SI
	MOVQ	payload_len+8(FP), R8
	MOVL	key+24(FP), DX

	// R9 = key64 = key | (key << 32), 标量尾部按 8 字节循环时用
	MOVQ	DX, R9
	SHLQ	$32, R9
	ORQ	DX, R9

	// len < 32: 全程走标量尾部, 不触碰 YMM
	CMPQ	R8, $32
	JB	scalar

	// Y0 = 8 个 32bit lane 都是 key
	// (低 4 字节来自 X0, VEX.256 广播, 需要 AVX2 的整数广播指令)
	MOVQ	DX, X0
	VPBROADCASTD	X0, Y0

	CMPQ	R8, $128
	JB	loop32

	// 主循环: 每轮 128 字节 (4 × 32B)
	// 先用 4 个独立寄存器装满, 再做 4 次 XOR, 最后统一写回:
	// 依赖链之间没有串行, 现代乱序核心可以吃满 2 load + 2 store/cycle。
loop128:
	VMOVDQU	(SI), Y1
	VMOVDQU	32(SI), Y2
	VMOVDQU	64(SI), Y3
	VMOVDQU	96(SI), Y4
	VPXOR	Y0, Y1, Y1
	VPXOR	Y0, Y2, Y2
	VPXOR	Y0, Y3, Y3
	VPXOR	Y0, Y4, Y4
	VMOVDQU	Y1, (SI)
	VMOVDQU	Y2, 32(SI)
	VMOVDQU	Y3, 64(SI)
	VMOVDQU	Y4, 96(SI)
	ADDQ	$128, SI
	SUBQ	$128, R8
	CMPQ	R8, $128
	JAE	loop128

	// 剩下的 32..127 字节, 每次 32 字节
loop32:
	CMPQ	R8, $32
	JB	scalar
	VMOVDQU	(SI), Y1
	VPXOR	Y0, Y1, Y1
	VMOVDQU	Y1, (SI)
	ADDQ	$32, SI
	SUBQ	$32, R8
	JMP	loop32

	// 标量尾部 0..31 字节, 按 8 -> 4 -> 1 退化
	// (与 maskFast 的尾部策略一致, 保证与标量实现逐位一致)
scalar:
	CMPQ	R8, $8
	JB	tail4
loop8:
	MOVQ	(SI), R10
	XORQ	R9, R10
	MOVQ	R10, (SI)
	ADDQ	$8, SI
	SUBQ	$8, R8
	CMPQ	R8, $8
	JAE	loop8

tail4:
	CMPQ	R8, $4
	JB	tail1
	MOVL	(SI), R10
	XORL	DX, R10
	MOVL	R10, (SI)
	ADDQ	$4, SI
	SUBQ	$4, R8

tail1:
	TESTQ	R8, R8
	JZ	done
loop1:
	// MOVBLZX 零扩展加载, XORB 只改低 8 位, 高 24 位保持 0
	MOVBLZX	(SI), R10
	XORB	DL, R10
	MOVB	R10, (SI)
	// key 循环右移一个字节, 让下一个字节的 mask 就位
	RORL	$8, DX
	INCQ	SI
	DECQ	R8
	JNZ	loop1

done:
	// 离开 AVX 代码前清掉 YMM 上半部:
	// 调用方(Go 运行时自己的 SIMD/SSE 代码)如果吃到 AVX-SSE 转换惩罚,
	// 会把本内核省下的时间又吐回去。
	VZEROUPPER
	RET

// ============================================================================
// SSE2 内核 (128bit)
// ============================================================================

// func maskSSE2(payload []byte, key uint32)
//
// 用 SSE2 对 payload 就地做 XOR mask。
// SSE2 是 amd64 架构基线(Go 本身要求 amd64 必须支持 SSE2), 无需运行时检测:
// 它既是"CPU 没有 AVX2"时的加速路径, 也是与 AVX2 对比实验的基线。
//
// 与 AVX2 版同样的约定: 任意长度都能正确处理, 小尺寸由分发层挡在阈值外。
TEXT ·maskSSE2(SB), NOSPLIT, $0-28
	MOVQ	payload_base+0(FP), SI
	MOVQ	payload_len+8(FP), R8
	MOVL	key+24(FP), DX

	// R9 = key64
	MOVQ	DX, R9
	SHLQ	$32, R9
	ORQ	DX, R9

	CMPQ	R8, $16
	JB	scalar

	// X0 = 4 个 32bit lane 都是 key
	// PSHUFD imm=0: 四个 lane 全部取自 lane0, 等价于广播
	MOVQ	DX, X0
	PSHUFD	$0, X0, X0

	CMPQ	R8, $64
	JB	loop16

	// 主循环: 每轮 64 字节 (4 × 16B)
loop64:
	MOVOU	(SI), X1
	MOVOU	16(SI), X2
	MOVOU	32(SI), X3
	MOVOU	48(SI), X4
	PXOR	X0, X1
	PXOR	X0, X2
	PXOR	X0, X3
	PXOR	X0, X4
	MOVOU	X1, (SI)
	MOVOU	X2, 16(SI)
	MOVOU	X3, 32(SI)
	MOVOU	X4, 48(SI)
	ADDQ	$64, SI
	SUBQ	$64, R8
	CMPQ	R8, $64
	JAE	loop64

	// 剩下的 16..63 字节, 每次 16 字节
loop16:
	CMPQ	R8, $16
	JB	scalar
	MOVOU	(SI), X1
	PXOR	X0, X1
	MOVOU	X1, (SI)
	ADDQ	$16, SI
	SUBQ	$16, R8
	JMP	loop16

	// 标量尾部, 同 AVX2 版
scalar:
	CMPQ	R8, $8
	JB	tail4
loop8:
	MOVQ	(SI), R10
	XORQ	R9, R10
	MOVQ	R10, (SI)
	ADDQ	$8, SI
	SUBQ	$8, R8
	CMPQ	R8, $8
	JAE	loop8

tail4:
	CMPQ	R8, $4
	JB	tail1
	MOVL	(SI), R10
	XORL	DX, R10
	MOVL	R10, (SI)
	ADDQ	$4, SI
	SUBQ	$4, R8

tail1:
	TESTQ	R8, R8
	JZ	done
loop1:
	MOVBLZX	(SI), R10
	XORB	DL, R10
	MOVB	R10, (SI)
	RORL	$8, DX
	INCQ	SI
	DECQ	R8
	JNZ	loop1

done:
	// 纯 SSE 路径不需要 VZEROUPPER(全程没有碰过 YMM 上半部)
	RET
