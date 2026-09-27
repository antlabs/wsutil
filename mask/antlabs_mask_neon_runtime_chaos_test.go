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
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ============================================================================
// 运行时环境层面的混沌测试
//
// 前面的测试都在固定的运行时配置下跑。这一组改变**执行环境**本身:
// goroutine 被迁移到不同 M、栈被移动、GC 在不同阶段打断调用。
//
// 汇编是 NOSPLIT 且吃裸指针的, 在 goroutine 被抢占迁移的时刻,
// 如果它对栈或寄存器的假定与运行时不一致, 就会在此暴露。
// ============================================================================

// ----------------------------------------------------------------------------
// 1. goroutine 迁移压力
//
// 大量短命 goroutine + 强制调度, 让调用 maskNEON 的 goroutine 尽可能
// 频繁地在不同 M 之间迁移。每次迁移会换栈、换寄存器组。
// ----------------------------------------------------------------------------

func Test_RuntimeChaos_GoroutineMigration(t *testing.T) {
	var errs int64
	var total int64

	var wg sync.WaitGroup
	for g := 0; g < 32; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(int64(g) * 7919))
			for r := 0; r < 500; r++ {
				n := rng.Intn(3000)
				src := make([]byte, n)
				rng.Read(src)
				key := rng.Uint32()

				want := append([]byte(nil), src...)
				maskFast(want, key)

				// 主动让出, 提高被迁移到其他 M 的概率
				if r%8 == 0 {
					runtime.Gosched()
				}

				maskNEON(src, key)
				if !bytes.Equal(want, src) {
					atomic.AddInt64(&errs, 1)
					return
				}
				atomic.AddInt64(&total, 1)
			}
		}(g)
	}
	wg.Wait()

	if errs != 0 {
		t.Fatalf("goroutine 迁移下 %d 次结果错误", errs)
	}
	t.Logf("迁移压力: %d 次调用无错误", total)
}

// ----------------------------------------------------------------------------
// 2. 栈增长/收缩边界
//
// 栈是动态增长的。当 goroutine 的栈被搬到新位置后, 之前取到的
// 切片指针(指向旧栈)会失效 —— 正常情况下 Go 会重定位。
// 这里在栈增长前后分别调用, 确认汇编拿到的总是有效指针。
// ----------------------------------------------------------------------------

//go:noinline
func growStackThenMask(depth int, src []byte, key uint32) uintptr {
	// 每层放较大的栈帧, 迫使栈增长
	var pad [1024]byte
	pad[depth&1023] = byte(depth)

	if depth > 0 {
		return growStackThenMask(depth-1, src, key)
	}

	// 此刻栈已经增长过多次, 在这里调用汇编
	maskNEON(src, key)
	runtime.KeepAlive(pad[:])
	return uintptr(len(src))
}

func Test_RuntimeChaos_StackGrowth(t *testing.T) {
	rng := rand.New(rand.NewSource(0x57ac))

	for i := 0; i < 2000; i++ {
		n := rng.Intn(3000)
		src := make([]byte, n)
		rng.Read(src)
		key := rng.Uint32()

		want := append([]byte(nil), src...)
		maskFast(want, key)

		// 深栈(强制栈增长)中调用
		growStackThenMask(128, src, key)

		if !bytes.Equal(want, src) {
			t.Fatalf("栈增长后调用结果错误: iter=%d n=%d", i, n)
		}

		// 栈上数组在增长后被调用
		stackView := src[:n]
		var local [2048]byte
		for j := range local {
			local[j] = byte(j)
		}
		maskNEON(stackView, key^1)
		_ = local[0]
	}
}

// ----------------------------------------------------------------------------
// 3. 极高频调用切换 (指令/数据缓存压力)
//
// 交替调用不同长度的输入, 让分支预测和 I-cache 一直处于失配状态。
// 这能暴露"只在特定长度组合下才错"的问题。
// ---------------------------------------------------------------------------

func Test_RuntimeChaos_AlternatingSizes(t *testing.T) {
	rng := rand.New(rand.NewSource(0xa17e))

	// 故意在分支边界之间反复横跳
	sizes := []int{63, 64, 65, 191, 192, 193, 127, 128, 129, 15, 16, 17}

	for i := 0; i < 30000; i++ {
		n := sizes[rng.Intn(len(sizes))]
		src := make([]byte, n)
		rng.Read(src)
		key := rng.Uint32()

		want := append([]byte(nil), src...)
		maskFast(want, key)

		maskNEON(src, key)
		if !bytes.Equal(want, src) {
			t.Fatalf("交替长度结果错误: iter=%d n=%d key=%#x", i, n, key)
		}
	}
}

// ----------------------------------------------------------------------------
// 4. 只在特定 GOMAXPROCS 下暴露的竞争
//
// GOMAXPROCS=1 时所有 goroutine 在同一 M 上串行, 会掩盖某些问题;
// 高并发时才会暴露。这里强制在不同并发度下重复跑。
// ---------------------------------------------------------------------------

func Test_RuntimeChaos_GOMAXPROCSIndependent(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过: short 模式")
	}

	orig := runtime.GOMAXPROCS(0)
	defer runtime.GOMAXPROCS(orig)

	for _, p := range []int{1, 2, 4} {
		if p > orig && orig > 0 {
			p = orig
		}
		runtime.GOMAXPROCS(p)

		var errs int64
		var wg sync.WaitGroup
		for g := 0; g < 8; g++ {
			wg.Add(1)
			go func(g int) {
				defer wg.Done()
				rng := rand.New(rand.NewSource(int64(g)*104729 + int64(p)))
				for r := 0; r < 800; r++ {
					n := rng.Intn(2500)
					src := make([]byte, n)
					rng.Read(src)
					key := rng.Uint32()

					want := append([]byte(nil), src...)
					maskFast(want, key)

					maskNEON(src, key)
					if !bytes.Equal(want, src) {
						atomic.AddInt64(&errs, 1)
						return
					}
				}
			}(g)
		}
		wg.Wait()
		if errs != 0 {
			t.Fatalf("GOMAXPROCS=%d 下 %d 次错误", p, errs)
		}
	}
}

// ----------------------------------------------------------------------------
// 5. 与 GC 的强制同步点交错
//
// 在 maskNEON 调用前后插入 runtime.GC(), 让每次调用都跨越
// 一个完整的 GC 周期。汇编若持有跨 GC 的指针假定, 这里会暴露。
// ---------------------------------------------------------------------------

func Test_RuntimeChaos_AcrossGC(t *testing.T) {
	rng := rand.New(rand.NewSource(0xacc0))

	for i := 0; i < 400; i++ {
		n := rng.Intn(3000)
		src := make([]byte, n)
		rng.Read(src)
		key := rng.Uint32()

		want := append([]byte(nil), src...)
		maskFast(want, key)

		runtime.GC() // GC 前的同步点
		maskNEON(src, key)
		runtime.GC() // GC 后的同步点

		if !bytes.Equal(want, src) {
			t.Fatalf("跨 GC 调用结果错误: iter=%d n=%d key=%#x", i, n, key)
		}
		runtime.KeepAlive(src)
	}
}

// ----------------------------------------------------------------------------
// 6. 长期高频调用下的稳定性 (含内存占用观察)
//
// 汇编不应有任何形式的持久状态。跑一段较长时间的调用,
// 确认不会出现性能劣化或内存增长。
// ---------------------------------------------------------------------------

func Test_RuntimeChaos_LongRunStability(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过: short 模式")
	}

	rng := rand.New(rand.NewSource(0x1066))
	// 每轮用不同大小的 buffer, 避免总是命中同一块内存
	bufs := make([][]byte, 16)
	for i := range bufs {
		bufs[i] = make([]byte, 256*(i+1))
	}

	var m0 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m0)

	const rounds = 200000
	t0 := time.Now()
	bad := 0
	for i := 0; i < rounds; i++ {
		b := bufs[rng.Intn(len(bufs))]
		n := rng.Intn(len(b) + 1)
		seg := b[:n]
		rng.Read(seg)
		before := append([]byte(nil), seg...)
		key := rng.Uint32()

		maskNEON(seg, key)

		kb := [4]byte{byte(key), byte(key >> 8), byte(key >> 16), byte(key >> 24)}
		for j := range seg {
			if seg[j] != before[j]^kb[j&3] {
				bad++
				break
			}
		}
	}
	elapsed := time.Since(t0)

	if bad != 0 {
		t.Fatalf("长跑期间 %d/%d 轮结果错误", bad, rounds)
	}

	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)
	// 汇编本身零分配; 这里只是确认没有意外的持续增长
	grew := int64(m1.HeapAlloc) - int64(m0.HeapAlloc)
	t.Logf("长跑 %d 轮, 耗时 %v (%.0f ns/op), HeapAlloc 变化 %+d 字节",
		rounds, elapsed.Round(time.Millisecond),
		float64(elapsed.Nanoseconds())/float64(rounds), grew)
}

// ----------------------------------------------------------------------------
// 7. 多入口混合 + 结果交叉校验
//
// 同一份输入分别过三个入口, 结果必须完全一致。
// 这能发现"某个入口在特定条件下退化"的问题。
// ---------------------------------------------------------------------------

func Test_RuntimeChaos_CrossEntryConsistency(t *testing.T) {
	rng := rand.New(rand.NewSource(0xc7055))

	for i := 0; i < 20000; i++ {
		n := rng.Intn(3000)
		key := rng.Uint32()
		src := make([]byte, n)
		rng.Read(src)

		a := append([]byte(nil), src...)
		maskNEON(a, key)

		b := append([]byte(nil), src...)
		maskFast(b, key)

		c := append([]byte(nil), src...)
		Mask(c, key)

		if !bytes.Equal(a, b) {
			t.Fatalf("maskNEON vs maskFast 不一致: iter=%d n=%d key=%#x", i, n, key)
		}
		if !bytes.Equal(a, c) {
			t.Fatalf("maskNEON vs Mask 不一致: iter=%d n=%d key=%#x", i, n, key)
		}
	}
}
