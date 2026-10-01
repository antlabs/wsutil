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

package mask

// amd64 上的 SIMD 加速实现, 默认启用。
//
// 与 darwin/arm64 的 NEON 版(opt-in, 需要 wsutil_neon 构建标签)不同,
// 这里默认开启, 原因:
//   - SSE2 是 amd64 架构基线(Go 本身要求), 不存在"没有"的情况
//   - AVX2 是运行时特性, 由 detectAVX2 按 Intel SDM 的标准序列检测,
//     检测不通过就退回 SSE2, 不会执行到不支持的指令
//
// 想要完全关掉回到标量实现:
//
//	go build -tags wsutil_nosimd ./...
//
// 关掉后行为与历史版本完全一致(见 antlabs_mask_init_generic.go)。
//
// 实测(Intel Core Ultra 9 275HX, linux/amd64, benchstat 10 轮):
// 见 antlabs_mask_amd64_bench_test.go 与 benchmark_dispatch_test.go。

//go:noescape
func maskAVX2(payload []byte, key uint32)

//go:noescape
func maskSSE2(payload []byte, key uint32)

//go:noescape
func cpuid(eaxArg, ecxArg uint32) (eax, ebx, ecx, edx uint32)

//go:noescape
func xgetbv() (eax, edx uint32)

// 分发阈值。三档: AVX2(256bit) / SSE2(128bit) / 标量。
//
// 阈值全部来自实测交叉点(Intel Core Ultra 9 275HX, 内核直调对比):
//
//	 32B: 标量 1.57ns  sse2 1.70ns  avx2 2.74ns   <- 标量赢
//	 64B: 标量 1.77ns  sse2 1.77ns  avx2 2.31ns   <- 标量/SSE2 平手
//	 96B: 标量 2.17ns  sse2 2.04ns  avx2 2.17ns   <- sse2 开始赢
//	128B: 标量 2.37ns  sse2 2.04ns  avx2 2.56ns   <- sse2 稳赢
//	256B: 标量 3.94ns  sse2 2.59ns  avx2 2.95ns   <- sse2 仍胜 avx2
//	512B: 标量 6.99ns  sse2 4.00ns  avx2 3.81ns   <- avx2 反超
//	 1KB: 标量 13.2ns  sse2 7.27ns  avx2 7.12ns   <- avx2 险胜
//	 4KB: 标量 51.6ns  sse2 31.9ns  avx2 16.8ns   <- avx2 1.9x
//	16KB: 标量  203ns  sse2  108ns  avx2 62.2ns   <- avx2 3.3x
//
// 三个反直觉的点, 都是实测结论:
//  1. AVX2 在 64..511B 上输给 SSE2 —— 256bit 内核的广播(setup)与
//     VZEROUPPER 摊不开, 所以这一段交给 SSE2, 而不是"有 AVX2 就用 AVX2"
//  2. SSE2 从 64B 起就不输标量(64B 平手, 96B 起稳定领先) —— 阈值取 64
//     而不是更保守的 128, 让 64..127B 这一段也吃到向量化的收益
//  3. AVX2 的收益要到 4KB 才明显拉开(47%), 512B..2KB 只是险胜 ——
//     阈值取 512 就能覆盖住, 再往上加没有意义
const (
	amd64AVX2Threshold = 512
	amd64SSE2Threshold = 64
)

// 由 init 写入, 之后只读
var amd64HasAVX2 bool

func init() {
	amd64HasAVX2 = detectAVX2()
	Mask = maskDispatch
}

// maskDispatch 是 Mask 的实现。
//
// 关于小尺寸为什么要付一次额外的调用: 试过把标量逻辑内联进本函数,
// 实测反而更慢(96B: 3.33ns vs 调用版 2.53ns)。原因是 maskFast 的
// 64/32/16 字节块是完全展开的, 内联版为了控制函数体积用了循环,
// 每次迭代多出 6 条控制指令, 96B 要跑 3 轮, 把省下的调用开销又吐了回去。
// 结论: 复用 maskFast 比在分发层里再写一份展开的标量实现更划算。
//
// 代价是 <64B 的极小消息比优化前多付约 0.3~0.5ns 的分发开销
// (占端到端 WriteMessage 的不到 0.5%, 见 quickws 的 mask 基准)。
func maskDispatch(payload []byte, key uint32) {
	// 256bit: 需要 CPU 支持, 且尺寸够大才划算
	if amd64HasAVX2 && len(payload) >= amd64AVX2Threshold {
		maskAVX2(payload, key)
		return
	}
	// 128bit: SSE2 是 amd64 架构基线, 无需检测
	if len(payload) >= amd64SSE2Threshold {
		maskSSE2(payload, key)
		return
	}
	maskFast(payload, key)
}

// detectAVX2 检测 CPU 与操作系统是否都支持 AVX2。
//
// 三个条件缺一不可(照 Intel SDM 的标准序列):
//  1. CPUID.1:ECX.OSXSAVE=1 —— OS 已启用 XSAVE/XCR0 管理, 否则 XGETBV 会 #UD
//  2. XCR0 的 XMM(bit1)与 YMM(bit2)都置位 —— 内核/OS 保存了 YMM 状态,
//     否则 AVX 指令在上下文切换时会丢数据
//  3. CPUID.7.0:EBX.AVX2=1 —— CPU 本身支持 AVX2
func detectAVX2() bool {
	maxID, _, _, _ := cpuid(0, 0)
	if maxID < 7 {
		return false
	}

	_, _, ecx1, _ := cpuid(1, 0)
	const (
		osxsaveBit = 1 << 27
		avxBit     = 1 << 28
	)
	if ecx1&osxsaveBit == 0 || ecx1&avxBit == 0 {
		return false
	}

	xcr0, _ := xgetbv()
	const (
		xmmState = 1 << 1
		ymmState = 1 << 2
	)
	if xcr0&(xmmState|ymmState) != xmmState|ymmState {
		return false
	}

	_, ebx7, _, _ := cpuid(7, 0)
	const avx2Bit = 1 << 5
	return ebx7&avx2Bit != 0
}
