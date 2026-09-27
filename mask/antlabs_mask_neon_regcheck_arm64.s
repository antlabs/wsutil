#include "textflag.h"

// func regSample() (r18v, gv, fpv uint64)
//
// 采样 R18_PLATFORM / g(R28) / R29 三个保留寄存器。
//
// 注意: 参数名不能用 g —— 在 Go arm64 汇编里 g 就是 R28 的别名,
// 汇编器会把 "g+8(FP)" 解析成寄存器而不是符号, 报 unknown variable。
//
// 这里只做观测, 不发起 CALL。调用 maskNEON 的动作放在 Go 侧,
// 避免手工搭建 ABI0 参数帧 —— 那会覆盖本函数的 LR 保存区,
// 表现为 SIGBUS 且 PC 跳到堆地址。
TEXT ·regSample(SB), NOSPLIT, $0-24
	MOVD	R18_PLATFORM, R3
	MOVD	R3, r18v+0(FP)
	MOVD	g, R4
	MOVD	R4, gv+8(FP)
	MOVD	R29, R5
	MOVD	R5, fpv+16(FP)
	RET
