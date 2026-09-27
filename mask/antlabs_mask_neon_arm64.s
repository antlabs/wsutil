// Copyright 2021-2024 antlabs. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build darwin && arm64 && wsutil_neon

#include "textflag.h"

// func maskNEON(payload []byte, key uint32)
//
// 用 NEON (Advanced SIMD) 对 payload 就地做 XOR mask。
// AArch64 下 Advanced SIMD 是架构强制要求的, 无需运行时特性检测。
//
// 调用方应保证 len(payload) >= 192: 实测交叉点在此, 更小的尺寸
// 由 maskFast(展开的标量版本)更快, 见 antlabs_mask_neon_arm64.go。
//
// 栈帧布局: payload.ptr=0(FP), payload.len=8(FP), payload.cap=16(FP), key=24(FP)
TEXT ·maskNEON(SB), NOSPLIT, $0-28
	MOVD	payload_base+0(FP), R0
	MOVD	payload_len+8(FP), R1
	// 必须用 MOVWU(零扩展)。Go arm64 汇编里 MOVW 是符号扩展加载,
	// key 的最高位为 1 时(如 0xdeadbeef)会被扩展成 0xffffffffdeadbeef。
	MOVWU	key+24(FP), R2

	// len < 64 必须跳过 64 字节主循环:
	// SUBS $64, R1 在 R1<64 时会下溢成接近 2^64 的无符号数, 随后的
	// CMP/BGE 会把控制流送回去, 造成越界读写 + 死循环。
	// 阈值分发保证生产路径上 len>=192, 但内核不能依赖这个前提
	// (单元测试会直接用任意长度调用它)。
	CMP	$64, R1
	BLT	no64

	// R3 = key64 = key | (key << 32)
	MOVD	R2, R3
	MOVD	R3, R4
	LSL	$32, R4, R4
	ORR	R4, R3, R3

	// 把 4 字节 key 广播到 V0 的四个 32bit lane
	VMOV	R2, V0.S[0]
	VDUP	V0.S[0], V0.S4

	// 主循环: 每轮 64 字节 (4 x 16B lane)
	// 注意: VLD1.P 和 VST1.P 都会对基址寄存器后自增。
	// 这里 load 用不自增的基址形式, 只让 store 推进指针, 否则指针会多走一倍。
loop64:
	VLD1	(R0), [V4.B16, V5.B16, V6.B16, V7.B16]
	VEOR	V0.B16, V4.B16, V4.B16
	VEOR	V0.B16, V5.B16, V5.B16
	VEOR	V0.B16, V6.B16, V6.B16
	VEOR	V0.B16, V7.B16, V7.B16
	VST1.P	[V4.B16, V5.B16, V6.B16, V7.B16], 64(R0)
	SUBS	$64, R1, R1
	CMP	$64, R1
	BGE	loop64

	// 剩下的 16..63 字节, 每次 16 字节
loop16:
	CMP	$16, R1
	BLT	tail
	VLD1	(R0), [V4.B16]
	VEOR	V0.B16, V4.B16, V4.B16
	VST1.P	[V4.B16], 16(R0)
	SUBS	$16, R1, R1
	B	loop16

	// 向量路径尾部 0..15 字节
tail:
	CBZ	R1, done
	B	tail8

	// len < 64 的入口: 补算 key64, 直接进标量尾部链
no64:
	MOVD	R2, R3
	MOVD	R3, R4
	LSL	$32, R4, R4
	ORR	R4, R3, R3
	// 落到 tail8

// 尾部 0..15 字节, 退化成标量
tail8:
	CMP	$8, R1
	BLT	tail4
	MOVD	(R0), R4
	EOR	R3, R4, R4
	MOVD	R4, (R0)
	ADD	$8, R0
	SUB	$8, R1
	B	tail8

tail4:
	CMP	$4, R1
	BLT	tail1
	MOVWU	(R0), R4
	EORW	R3, R4, R4
	MOVW	R4, (R0)
	ADD	$4, R0
	SUB	$4, R1
	B	tail4

tail1:
	CBZ	R1, done
	ANDW	$0xff, R2, R4
	MOVBU	(R0), R5
	EORW	R4, R5, R5
	MOVB	R5, (R0)
	ADD	$1, R0
	RORW	$8, R2, R2
	SUB	$1, R1
	B	tail1

done:
	RET
