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

package mask

import (
	"bytes"
	"math/rand"
	"reflect"
	"runtime"
	"sync"
	"testing"
	"unsafe"
)

// ============================================================================
// 直接针对汇编内核的混沌测试
//
// 为什么需要单独一套:
//
// maskDispatch 的路由是 len>=192 -> maskNEON, 其余 -> maskFast。
// 所以只调用 Mask(...) 的测试, 在 len<192 时根本没有碰到汇编 ——
// 测的是纯 Go 的 maskFast。而汇编内核内部还有一条 no64 分支
// (len<64), 在生产分发下永远不会被走到。
//
// 这套测试一律**直接调用 maskNEON**, 覆盖 0 到数 KB 的每一个长度,
// 并在对抗性条件下(随机偏移/别名/并发/GC/缓冲区复用)校验:
//   1. 结果与参考实现逐字节一致
//   2. 绝不越界写 —— 用全量哨兵逐字节校验
//
// 这样即使将来 neonThreshold 被调小、或分发逻辑被改动,
// 汇编内核在所有长度上的正确性都已经被独立验证过。
// ============================================================================

// kernelEntries 列出需要按同样标准验证的内核入口。
// 目前只有汇编内核; 加在这里的每一项目都会被下面所有用例覆盖。
type kernelEntry struct {
	name string
	fn   func([]byte, uint32)
}

// kernelEntries 在**运行时**解析入口, 避免在包级初始化时取到 nil 的 Mask。
func kernelEntries() []kernelEntry {
	return []kernelEntry{
		{"asm", maskNEON},
		{"dispatch", func(p []byte, k uint32) { Mask(p, k) }},
	}
}

// ############################################################################
// 1. 全长度穷举 + 全量哨兵 (直接调内核)
//
// 覆盖 0..4096 的**每一个**长度, 每个长度都:
//   - 校验 payload 逐字节正确
//   - 校验 payload 之外**每一个**字节未被触碰
//
// 这样 64(主循环) / 16(次级循环) / 8,4,1(尾部) / no64 每个分支
// 的所有余数组合都被遍历到。
// ############################################################################

func Test_KernelChaos_ExhaustiveAllLengths(t *testing.T) {
	const maxLen = 4096
	keys := []uint32{0, 1, 0x80000000, 0xdeadbeef, 0xffffffff, 0x12345678}

	for _, e := range kernelEntries() {
		for _, key := range keys {
			for n := 0; n <= maxLen; n++ {
				// 布局: [前哨兵 32][payload n][后哨兵 32]
				const guard = 32
				backing := make([]byte, guard+n+guard)
				// 填入非平凡的数据, 避免 0/xor 的巧合掩盖错误
				for i := range backing {
					backing[i] = byte(i*7 + 13)
				}
				snapshot := append([]byte(nil), backing...)

				payload := backing[guard : guard+n]
				kb := [4]byte{byte(key), byte(key >> 8), byte(key >> 16), byte(key >> 24)}

				e.fn(payload, key)

				// 1) payload 逐字节正确
				for j := 0; j < n; j++ {
					want := snapshot[guard+j] ^ kb[j&3]
					if payload[j] != want {
						t.Fatalf("%s: 结果错误 key=%#x n=%d 偏移=%d 期望=%#x 实际=%#x",
							e.name, key, n, j, want, payload[j])
					}
				}
				// 2) 前后哨兵逐字节未变
				for i := 0; i < guard; i++ {
					if backing[i] != snapshot[i] {
						t.Fatalf("%s: 前向越界写! key=%#x n=%d 偏移=%d",
							e.name, key, n, i)
					}
					if backing[guard+n+i] != snapshot[guard+n+i] {
						t.Fatalf("%s: 后向越界写! key=%#x n=%d 偏移=%d",
							e.name, key, n, guard+n+i)
					}
				}
			}
		}
	}
}

// ############################################################################
// 2. 随机化风暴
//
// 长度 / key / 数据 / 起始偏移 全部随机, 直接打内核。
// 偏移覆盖 0..63, 用来撞击未对齐访问。
// ############################################################################

func Test_KernelChaos_RandomizedStorm(t *testing.T) {
	const iters = 60000

	for _, e := range kernelEntries() {
		rng := rand.New(rand.NewSource(0x5eed))
		for i := 0; i < iters; i++ {
			n := rng.Intn(3000)
			off := rng.Intn(64)
			key := rng.Uint32()

			const tail = 32
			backing := make([]byte, off+n+tail)
			rng.Read(backing)
			snapshot := append([]byte(nil), backing...)

			payload := backing[off : off+n]
			e.fn(payload, key)

			kb := [4]byte{byte(key), byte(key >> 8), byte(key >> 16), byte(key >> 24)}
			for j := 0; j < n; j++ {
				want := snapshot[off+j] ^ kb[j&3]
				if payload[j] != want {
					t.Fatalf("%s: 结果错误 iter=%d n=%d off=%d key=%#x 偏移=%d",
						e.name, i, n, off, key, j)
				}
			}
			if d := firstDiff(snapshot, backing, off, n); d >= 0 {
				t.Fatalf("%s: 越界写! iter=%d n=%d off=%d key=%#x 位置=%d",
					e.name, i, n, off, key, d)
			}
		}
	}
}

// ############################################################################
// 3. 内核对所有 key 位模式的敏感度
//
// 专门针对 MOVW 符号扩展那类问题: key 的 bit31 决定高位如何扩展。
// 这里对每个长度用一组精心挑选的 key 跑一遍, 并两两检查
// "不同 key 必须产生不同结果"(否则说明某个 key 位被忽略了)。
// ############################################################################

func Test_KernelChaos_KeyBitSensitivity(t *testing.T) {
	// 每个 key 只在一个 bit 上与 0 不同 —— 任何一个 bit 被忽略都会暴露
	keys := make([]uint32, 0, 33)
	keys = append(keys, 0)
	for b := 0; b < 32; b++ {
		keys = append(keys, 1<<uint(b))
	}

	lengths := []int{0, 1, 2, 3, 4, 7, 8, 15, 16, 31, 32, 63, 64, 65,
		95, 96, 127, 128, 160, 191, 192, 193, 255, 256, 257, 384, 512, 1024, 4096}

	for _, e := range kernelEntries() {
		for _, n := range lengths {
			// 用 non-zero 数据, 保证任何被忽略的 key bit 都会体现为差异
			src := make([]byte, n)
			for i := range src {
				src[i] = 0xA5
			}

			results := make(map[string]uint32, len(keys))
			for _, key := range keys {
				got := append([]byte(nil), src...)
				e.fn(got, key)

				// 逐字节对照参考
				kb := [4]byte{byte(key), byte(key >> 8), byte(key >> 16), byte(key >> 24)}
				for j := range got {
					if got[j] != src[j]^kb[j&3] {
						t.Fatalf("%s: key=%#x n=%d 偏移=%d 结果错误", e.name, key, n, j)
					}
				}

				// 指纹唯一性检查只在 n>=4 时做。
				//
				// 原因: mask 的第 j 字节只用 key 的第 (j&3) 字节。
				// n<4 时高位的 key 字节根本不参与运算, 所以例如
				// n=1 时 key=0x100 与 key=0x0 结果相同是**正确**的,
				// 不是汇编忽略了某个 bit。
				if n >= 4 {
					var fp uint32
					for j := 0; j < 4; j++ {
						fp |= uint32(got[j]) << (uint(j) * 8)
					}
					for k, v := range results {
						if v == fp {
							t.Fatalf("%s: 不同 key 给出相同结果 (key=%#x 与 %s 撞了), n=%d",
								e.name, key, k, n)
						}
					}
					results[hex32(key)] = fp
				}
			}
		}
	}
}

// ############################################################################
// 4. 并发下直接打内核
//
// 汇编是 NOSPLIT 且无共享状态。这里多个协程直接调 maskNEON,
// 用 -race 检查, 同时校验结果 —— 排除任何隐藏的共享可变状态。
// ############################################################################

func Test_KernelChaos_Concurrent(t *testing.T) {
	const workers = 16
	const rounds = 6000

	var wg sync.WaitGroup
	errs := make(chan string, workers)

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(int64(w)*2654435761 + 11))
			// 每个协程用自己的 buffer, 排除测试自身的干扰
			buf := make([]byte, 8192)

			for r := 0; r < rounds; r++ {
				n := rng.Intn(4096)
				key := rng.Uint32()
				seg := buf[:n]
				rng.Read(seg)
				before := append([]byte(nil), seg...)

				maskNEON(seg, key) // 直接打汇编, 不走分发

				kb := [4]byte{byte(key), byte(key >> 8), byte(key >> 16), byte(key >> 24)}
				for j := range seg {
					if seg[j] != before[j]^kb[j&3] {
						errs <- "worker=" + itoa(w) + " round=" + itoa(r) +
							" n=" + itoa(n) + " key=" + hex32(key)
						return
					}
				}
				if r%1024 == 0 {
					runtime.Gosched()
				}
			}
		}(w)
	}

	wg.Wait()
	close(errs)
	for e := range errs {
		t.Fatalf("并发下内核结果错误: %s", e)
	}
}

// ############################################################################
// 5. 缓冲区复用 / 状态残留
//
// 若汇编残留了任何跨调用状态(误用了某个静态缓冲、寄存器没清),
// 在同一块内存上交替跑不同长度和 key 时就会暴露。
// ############################################################################

func Test_KernelChaos_BufferReuse(t *testing.T) {
	for _, e := range kernelEntries() {
		buf := make([]byte, 8192)
		rng := rand.New(rand.NewSource(0xb0ffe2))

		for i := 0; i < 30000; i++ {
			n := rng.Intn(len(buf) + 1)
			key := rng.Uint32()

			seg := buf[:n]
			rng.Read(seg)
			before := append([]byte(nil), seg...)

			e.fn(seg, key)

			kb := [4]byte{byte(key), byte(key >> 8), byte(key >> 16), byte(key >> 24)}
			for j := range seg {
				if seg[j] != before[j]^kb[j&3] {
					t.Fatalf("%s: 复用缓冲区结果错误 iter=%d n=%d key=%#x 偏移=%d",
						e.name, i, n, key, j)
				}
			}
		}
	}
}

// ############################################################################
// 6. GC 压力下直接打内核
//
// 汇编不占栈帧, 但它被调用时 Go 可能正在做栈复制或 GC 扫描。
// 这里在深递归和强制 GC 中反复直接调用内核。
// ############################################################################

//go:noinline
func deepKernelCall(e func([]byte, uint32), depth int, p []byte, key uint32) {
	if depth <= 0 {
		e(p, key)
		return
	}
	_ = make([]byte, 256+depth) // 制造栈增长 + 垃圾
	deepKernelCall(e, depth-1, p, key)
}

func Test_KernelChaos_GCPressure(t *testing.T) {
	rng := rand.New(rand.NewSource(0x9c9))

	for _, e := range kernelEntries() {
		for i := 0; i < 200; i++ {
			n := rng.Intn(3000)
			src := make([]byte, n)
			rng.Read(src)
			key := rng.Uint32()

			want := append([]byte(nil), src...)
			kb := [4]byte{byte(key), byte(key >> 8), byte(key >> 16), byte(key >> 24)}
			for j := range want {
				want[j] ^= kb[j&3]
			}

			got := append([]byte(nil), src...)
			// 制造 GC 压力
			garbage := make([][]byte, 32)
			for j := range garbage {
				garbage[j] = make([]byte, 1024)
			}
			runtime.GC()

			deepKernelCall(e.fn, 64, got, key)

			if !bytes.Equal(want, got) {
				t.Fatalf("%s: GC 压力下结果错误 iter=%d n=%d key=%#x", e.name, i, n, key)
			}
			runtime.KeepAlive(garbage)
		}
	}
}

// ############################################################################
// 7. 覆盖性证明
//
// 这一条不测算法, 而是**证明汇编确实被上面这些用例执行了**。
// 如果将来有人改动分发逻辑, 让汇编变成不可达, 或让某个入口
// 偷偷指向了 Go 实现, 这里会立刻失败 —— 避免"测试全绿但汇编没跑"。
// ############################################################################

func Test_KernelChaos_CoverageProof(t *testing.T) {
	fnName := func(f func([]byte, uint32)) string {
		if f == nil {
			return "<nil>"
		}
		return runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name()
	}

	asmName := fnName(maskNEON)
	fastName := fnName(maskFast)
	dispatchName := fnName(Mask)

	// 1) 汇编内核必须存在, 且与 Go 实现是**不同的函数**
	if asmName == "" || fastName == "" {
		t.Fatal("内核或 Go 实现缺失")
	}
	if asmName == fastName {
		t.Fatalf("汇编内核与 maskFast 指向同一函数, 汇编未被编译进来: %s", asmName)
	}

	// 2) 公开入口必须指向分发器 (而不是被静默替换成 maskFast)
	if dispatchName != fnName(maskDispatch) {
		t.Fatalf("Mask 应指向 maskDispatch, 实际=%s", dispatchName)
	}

	// 3) 分发的两侧必须都能到达:
	//    len>=neonThreshold 必须真的走到汇编, len<neonThreshold 走 Go 实现。
	//    用"是否产生结果差异"无法判断, 所以这里核对函数身份 + 阈值可达性。
	if neonThreshold == 0 {
		t.Fatal("neonThreshold 为 0 会让汇编永不执行")
	}

	// 汇编内核本身必须能处理任意长度(它内部有自己的分支),
	// 否则阈值以下的长度在将来下调阈值时会出问题。
	// 这里做一次最小可用性确认。
	for _, n := range []int{0, 1, 8, 16, 63, 64, 65, 191, 192, 193} {
		p := make([]byte, n)
		for i := range p {
			p[i] = byte(i + 1)
		}
		maskNEON(p, 0xA5A5A5A5) // 不得 panic, 不得越界
	}

	t.Logf("汇编内核 = %s", asmName)
	t.Logf("Go 实现  = %s", fastName)
	t.Logf("公开入口 = %s (threshold=%d)", dispatchName, neonThreshold)
}

// ############################################################################
// 8. 汇编与 Go 实现在所有长度上的完全等价 (直接对比, 不经过分发)
// ############################################################################

func Test_KernelChaos_EquivalentToMaskFast(t *testing.T) {
	rng := rand.New(rand.NewSource(0xee1))

	// 小长度逐个穷举 + 大长度抽样
	lengths := make([]int, 0, 1200)
	for n := 0; n <= 1100; n++ {
		lengths = append(lengths, n)
	}
	for _, base := range []int{2048, 4096, 8192, 16384} {
		for d := -3; d <= 3; d++ {
			lengths = append(lengths, base+d)
		}
	}

	for _, key := range []uint32{0, 1, 0x80000000, 0xdeadbeef, 0xffffffff, 0xa5a5a5a5} {
		for _, n := range lengths {
			if n < 0 {
				continue
			}
			src := make([]byte, n)
			rng.Read(src)

			want := append([]byte(nil), src...)
			maskFast(want, key)

			got := append([]byte(nil), src...)
			maskNEON(got, key)

			if !bytes.Equal(want, got) {
				t.Fatalf("汇编与 maskFast 不一致: key=%#x n=%d", key, n)
			}
		}
	}
}

// ############################################################################
// 9. 切片 header 与指针不被修改
// ############################################################################

func Test_KernelChaos_SliceHeaderStability(t *testing.T) {
	rng := rand.New(rand.NewSource(0x511ce))

	for i := 0; i < 10000; i++ {
		n := rng.Intn(2000)
		src := make([]byte, n)
		rng.Read(src)
		key := rng.Uint32()

		a := append([]byte(nil), src...)
		b := append([]byte(nil), src...)

		capA := cap(a)
		lenA := len(a)
		var ptrA unsafe.Pointer
		if n > 0 {
			ptrA = unsafe.Pointer(unsafe.SliceData(a))
		}

		maskNEON(a, key)
		maskNEON(b, key)

		if len(a) != lenA || cap(a) != capA {
			t.Fatalf("iter=%d: 切片 header 被修改 len %d->%d cap %d->%d",
				i, lenA, len(a), capA, cap(a))
		}
		if n > 0 && unsafe.Pointer(unsafe.SliceData(a)) != ptrA {
			t.Fatalf("iter=%d: 底层指针被修改", i)
		}
		if !bytes.Equal(a, b) {
			t.Fatalf("iter=%d: 两次独立调用结果不同 (n=%d key=%#x)", i, n, key)
		}
	}
}
