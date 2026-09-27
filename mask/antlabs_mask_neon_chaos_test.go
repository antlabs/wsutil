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
	"testing"
	"time"
	"unsafe"
)

// ============================================================================
// 混沌测试 (chaos testing)
//
// 这部分与 antlabs_mask_neon_test.go 的"结构化测试"目标不同:
// 结构化测试穷举已知的维度(长度/key/偏移), 而这里模拟的是
// 真实程序里难以预测的对抗性场景 —— 别名、复用、GC 干扰、并发交错、
// 以及用随机化手段去撞汇编里可能存在的、想不到的边界。
//
// 关于保护页(mprotect PROT_NONE): macOS 27 上对匿名映射的后续页调用
// mprotect 会返回 EINVAL(纯 C 亦如此), 无法用来做越界检测。
// 所以这里改用"全量哨兵校验": 把 payload 放在随机偏移, 逐个字节
// 校验 [0,off) 和 [off+len,total) 区间完全未被触碰。任何前向或
// 后向越界写都会命中这些区域。
// ============================================================================

// chaosRNG 返回一个确定性种子的随机源, 失败可复现
func chaosRNG(seed int64) *rand.Rand {
	return rand.New(rand.NewSource(seed))
}

// ----------------------------------------------------------------------------
// 1. 全量哨兵: payload 埋在随机偏移, 逐字节校验周围内存未被污染
//
// 这是替代保护页的主力手段。与固定 64 字节哨兵相比:
//   - 前后哨兵可达数 KB, 能发现远距离越界
//   - 偏移随机化, 能撞上不同对齐/页位置
//   - 每次都校验全部哨兵字节, 而不是抽查
// ----------------------------------------------------------------------------

func Test_Chaos_FullSentinel(t *testing.T) {
	seeds := []int64{1, 2, 3, 4, 5, 6, 7, 8}
	itersPerSeed := 3000

	for _, seed := range seeds {
		rng := chaosRNG(seed)
		for i := 0; i < itersPerSeed; i++ {
			// 大后备数组, 保证前后都有充足的哨兵空间
			total := 64 + rng.Intn(3000) + 64
			off := 8 + rng.Intn(56) // 保证前后至少有 8 字节
			n := total - off - 8
			if n < 0 {
				continue
			}

			backing := make([]byte, total)
			rng.Read(backing)

			// 快照, 用于事后逐字节比对
			snapshot := append([]byte(nil), backing...)

			payload := backing[off : off+n]
			key := rng.Uint32()

			// 期望结果
			want := make([]byte, n)
			kb := [4]byte{byte(key), byte(key >> 8), byte(key >> 16), byte(key >> 24)}
			for j := 0; j < n; j++ {
				want[j] = snapshot[off+j] ^ kb[j&3]
			}

			Mask(payload, key)

			// 校验 payload 正确
			if !bytes.Equal(want, payload) {
				t.Fatalf("seed=%d iter=%d: payload 内容错误 (off=%d len=%d key=%#x)",
					seed, i, off, n, key)
			}
			// 校验 payload 之外**每一个字节**都没变
			if diff := firstDiff(snapshot, backing, off, n); diff >= 0 {
				t.Fatalf("seed=%d iter=%d: 越界写! 偏移 %d 被改写 (payload 区间 [%d,%d) len=%d)",
					seed, i, diff, off, off+n, n)
			}
		}
	}
}

// firstDiff 返回 changed 相对 base 的第一个差异位置, 跳过 [skip, skip+n) 区间; 无差异返回 -1
func firstDiff(base, changed []byte, skip, n int) int {
	for i := range base {
		if i >= skip && i < skip+n {
			continue
		}
		if base[i] != changed[i] {
			return i
		}
	}
	return -1
}

// ----------------------------------------------------------------------------
// 2. 别名与重叠视图
//
// mask 是就地操作, 真实调用方经常会传同一个大 buffer 的不同子区间。
// 这里验证重叠/相邻/同一底层数组的各种视图组合下结果仍然可预测。
// ----------------------------------------------------------------------------

func Test_Chaos_Aliasing(t *testing.T) {
	rng := chaosRNG(100)

	for i := 0; i < 5000; i++ {
		total := 1 + rng.Intn(1500)
		backing := make([]byte, total)
		rng.Read(backing)
		orig := append([]byte(nil), backing...)

		// 随机切两个可能重叠的区间
		a := rng.Intn(total)
		b := rng.Intn(total)
		if a > b {
			a, b = b, a
		}
		key1, key2 := rng.Uint32(), rng.Uint32()

		// 顺序 mask 两个区间
		Mask(backing[a:b], key1)
		Mask(backing[a:b], key2)

		// 两次 XOR 等价于 XOR (key1^key2)
		expect := append([]byte(nil), orig...)
		Mask(expect[a:b], key1^key2)

		if !bytes.Equal(backing, expect) {
			t.Fatalf("iter=%d: 重叠区间两次 mask 结果异常 (total=%d a=%d b=%d)", i, total, a, b)
		}

		// 区间外的字节必须完全没动
		for j := range orig {
			if j < a || j >= b {
				if backing[j] != orig[j] {
					t.Fatalf("iter=%d: 区间外的字节 %d 被改写", i, j)
				}
			}
		}
	}
}

// 相邻区间首尾相接, 模拟 frame.WriteFrame 把 mask 结果拼到任意偏移的场景
func Test_Chaos_AdjacentSegments(t *testing.T) {
	rng := chaosRNG(200)

	for i := 0; i < 5000; i++ {
		total := 2 + rng.Intn(2000)
		backing := make([]byte, total)
		rng.Read(backing)

		key := rng.Uint32()
		// 参考: 整体 mask
		want := append([]byte(nil), backing...)
		Mask(want, key)

		// 切 1~4 段分别处理, 每段用按其全局偏移旋转后的 key
		got := append([]byte(nil), backing...)
		nseg := 1 + rng.Intn(4)
		pos := 0
		for s := 0; s < nseg; s++ {
			rem := total - pos
			if rem <= 0 {
				break
			}
			var segLen int
			if s == nseg-1 {
				segLen = rem
			} else {
				segLen = 1 + rng.Intn(rem)
			}
			Mask(got[pos:pos+segLen], rotateKeyChaos(key, pos&3))
			pos += segLen
		}

		if !bytes.Equal(want, got) {
			t.Fatalf("iter=%d: 多段拼接结果与整体 mask 不一致 (total=%d nseg=%d)", i, total, nseg)
		}
	}
}

// rotateKeyChaos 返回等效 key: rotated[j] == key[(off+j)&3]
func rotateKeyChaos(key uint32, off int) uint32 {
	off &= 3
	var out uint32
	for j := 0; j < 4; j++ {
		out |= ((key >> (uint(uint((j+off)&3) * 8))) & 0xff) << (uint(j) * 8)
	}
	return out
}

// ----------------------------------------------------------------------------
// 3. 切片视图的极端形态
//
// 汇编直接吃指针, 不感知 len 之外的 cap。这里用极端的三下标切片、
// 超小 cap、指向大数组尾部的视图等, 确保内核只看 len。
// ----------------------------------------------------------------------------

func Test_Chaos_ExtremeSliceViews(t *testing.T) {
	rng := chaosRNG(300)

	for i := 0; i < 4000; i++ {
		// 后备数组大到可以切出各种视图
		backing := make([]byte, 1+rng.Intn(4096))
		rng.Read(backing)
		orig := append([]byte(nil), backing...)

		n := len(backing)
		start := rng.Intn(n + 1)
		end := start + rng.Intn(n-start+1)

		// 三下标切片: cap 被限制成 len, 任何越界写都会落到后备数组上被检出
		view := backing[start:end:end]
		key := rng.Uint32()

		kb := [4]byte{byte(key), byte(key >> 8), byte(key >> 16), byte(key >> 24)}
		want := make([]byte, len(view))
		for j := range view {
			want[j] = orig[start+j] ^ kb[j&3]
		}

		Mask(view, key)

		if !bytes.Equal(want, view) {
			t.Fatalf("iter=%d: 视图内容错误 (start=%d end=%d)", i, start, end)
		}
		if diff := firstDiff(orig, backing, start, end-start); diff >= 0 {
			t.Fatalf("iter=%d: 视图外字节 %d 被改写 (start=%d end=%d)", i, diff, start, end)
		}
	}
}

// ----------------------------------------------------------------------------
// 4. 内存复用 / 状态污染
//
// 汇编若残留了任何跨调用的状态(寄存器没清零、误用了静态缓冲),
// 在缓冲区被反复复用时就会暴露。这里用同一个 buffer 反复跑
// 不同长度和不同 key, 每次都校验结果。
// ----------------------------------------------------------------------------

func Test_Chaos_BufferReuse(t *testing.T) {
	rng := chaosRNG(400)
	const bufSize = 4096
	buf := make([]byte, bufSize)

	for i := 0; i < 20000; i++ {
		n := rng.Intn(bufSize + 1)
		key := rng.Uint32()

		// 每次填入新的随机内容
		seg := buf[:n]
		rng.Read(seg)
		before := append([]byte(nil), seg...)

		Mask(seg, key)

		kb := [4]byte{byte(key), byte(key >> 8), byte(key >> 16), byte(key >> 24)}
		for j := range seg {
			if seg[j] != before[j]^kb[j&3] {
				t.Fatalf("iter=%d: 复用缓冲区结果错误 (n=%d key=%#x 偏移=%d)", i, n, key, j)
			}
		}
	}

	// 特意用同一块内存跑一遍边界值序列, 看看是否存在残余状态
	keys := []uint32{0xffffffff, 0, 0xdeadbeef, 0x80000000, 0x12345678}
	lens := []int{0, 1, 63, 64, 65, 191, 192, 193, 255, 256, 1023, 1024}
	for _, k := range keys {
		for _, n := range lens {
			seg := buf[:n]
			for j := range seg {
				seg[j] = 0
			}
			Mask(seg, k)
			kb := [4]byte{byte(k), byte(k >> 8), byte(k >> 16), byte(k >> 24)}
			for j := range seg {
				if seg[j] != kb[j&3] {
					t.Fatalf("边界序列: key=%#x n=%d 偏移=%d 结果错误", k, n, j)
				}
			}
		}
	}
}

// ----------------------------------------------------------------------------
// 5. GC 压力 + 栈增长干扰
//
// 汇编是 NOSPLIT 且不占栈帧, 但它被在深栈里调用时, Go 可能正在
// 做栈复制。这里在大量分配和深递归中调用 mask, 确认结果不受影响。
// ----------------------------------------------------------------------------

//go:noinline
func deepCallMask(depth int, payload []byte, key uint32, want []byte, t *testing.T) {
	if depth <= 0 {
		Mask(payload, key)
		return
	}
	// 每层都分配, 制造 GC 压力和栈增长
	_ = make([]byte, 256+depth)
	deepCallMask(depth-1, payload, key, want, t)
}

func Test_Chaos_GCPressure(t *testing.T) {
	rng := chaosRNG(500)

	for i := 0; i < 300; i++ {
		n := rng.Intn(2500)
		src := make([]byte, n)
		rng.Read(src)

		payload := append([]byte(nil), src...)
		key := rng.Uint32()

		want := append([]byte(nil), src...)
		Mask(want, key)

		// 制造垃圾, 提高 GC 触发概率
		garbage := make([][]byte, 64)
		for j := range garbage {
			garbage[j] = make([]byte, 512)
		}
		runtime.GC()

		deepCallMask(64, payload, key, want, t)

		if !bytes.Equal(want, payload) {
			t.Fatalf("iter=%d: GC 压力下结果错误 (n=%d key=%#x)", i, n, key)
		}
		_ = garbage
		runtime.KeepAlive(garbage)
	}
}

// ----------------------------------------------------------------------------
// 6. 并发风暴
//
// 大量协程以随机尺寸/key 并发调用, 混合 Mask / maskNEON / maskFast,
// 并在过程中穿插 GC。配合 -race 使用。
// 汇编只操作传入的切片, 不应有共享可变状态; 这里就是验证这一点。
// ----------------------------------------------------------------------------

func Test_Chaos_ConcurrentStorm(t *testing.T) {
	const workers = 16
	const rounds = 4000

	var wg sync.WaitGroup
	errs := make(chan string, workers)

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			rng := chaosRNG(int64(w)*104729 + 7)
			buf := make([]byte, 8192)

			for r := 0; r < rounds; r++ {
				n := rng.Intn(4096)
				key := rng.Uint32()
				seg := buf[:n]
				rng.Read(seg)
				before := append([]byte(nil), seg...)

				// 随机选一个入口, 三个入口语义必须完全一致
				switch rng.Intn(3) {
				case 0:
					Mask(seg, key)
				case 1:
					maskNEON(seg, key)
				default:
					maskFast(seg, key)
				}

				kb := [4]byte{byte(key), byte(key >> 8), byte(key >> 16), byte(key >> 24)}
				for j := range seg {
					if seg[j] != before[j]^kb[j&3] {
						errs <- "worker=" + itoa(w) + " round=" + itoa(r) +
							" n=" + itoa(n) + " key=" + hex32(key)
						return
					}
				}

				if r%977 == 0 {
					runtime.Gosched()
				}
			}
		}(w)
	}

	wg.Wait()
	close(errs)
	for e := range errs {
		t.Fatalf("并发风暴下结果错误: %s", e)
	}
}

// 并发 + GC + 不同长度交替, 制造最坏的交错
func Test_Chaos_ConcurrentWithGC(t *testing.T) {
	done := make(chan struct{})
	var allocStop sync.WaitGroup
	allocStop.Add(1)

	// 一个持续分配/释放的协程, 制造 GC 压力
	go func() {
		defer allocStop.Done()
		var keep [][]byte
		for {
			select {
			case <-done:
				return
			default:
			}
			b := make([]byte, 1024)
			keep = append(keep, b)
			if len(keep) > 512 {
				keep = keep[len(keep)/2:]
			}
		}
	}()

	const workers = 8
	var wg sync.WaitGroup
	errs := make(chan string, workers)

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			rng := chaosRNG(int64(w)*7919 + 13)
			for r := 0; r < 2000; r++ {
				n := rng.Intn(3000)
				key := rng.Uint32()
				src := make([]byte, n)
				rng.Read(src)
				want := append([]byte(nil), src...)
				Mask(want, key)

				got := append([]byte(nil), src...)
				Mask(got, key)
				if !bytes.Equal(want, got) {
					errs <- "worker=" + itoa(w) + " round=" + itoa(r)
					return
				}
			}
		}(w)
	}

	wg.Wait()
	close(done)
	allocStop.Wait()
	close(errs)
	for e := range errs {
		t.Fatalf("GC 并发下错误: %s", e)
	}
}

// ----------------------------------------------------------------------------
// 7. 操作序列的代数不变量
//
// 随机生成长串操作(正向 mask / 反向还原 / 换 key / 分段),
// 每一步都校验不变量。这能发现"单次调用正确但状态有残留"类问题。
// ----------------------------------------------------------------------------

func Test_Chaos_OperationSequence(t *testing.T) {
	rng := chaosRNG(600)

	for trial := 0; trial < 2000; trial++ {
		n := rng.Intn(2000)
		orig := make([]byte, n)
		rng.Read(orig)

		buf := append([]byte(nil), orig...)
		// 累积的逻辑 key: 每次 mask 叠加一个 key
		var logical uint32

		ops := 1 + rng.Intn(12)
		opsDone := 0
		for op := 0; op < ops; op++ {
			opsDone = op + 1
			switch rng.Intn(4) {
			case 0: // 整体 mask
				k := rng.Uint32()
				Mask(buf, k)
				logical ^= k

			case 1: // 同一 key 再来一次 = 还原
				k := rng.Uint32()
				Mask(buf, k)
				Mask(buf, k)

			case 2: // 分段处理
				if n == 0 {
					continue
				}
				cut := rng.Intn(n + 1)
				k := rng.Uint32()
				Mask(buf[:cut], rotateKeyChaos(k, 0))
				Mask(buf[cut:], rotateKeyChaos(k, cut&3))
				logical ^= k

			case 3: // 空操作
			}
		}

		// 最终应等于 orig XOR (logical 逐字节)
		kb := [4]byte{byte(logical), byte(logical >> 8), byte(logical >> 16), byte(logical >> 24)}
		for j := 0; j < n; j++ {
			if buf[j] != orig[j]^kb[j&3] {
				t.Fatalf("trial=%d 已执行%d步: 序列不变量被破坏 (n=%d j=%d logical=%#x)",
					trial, opsDone, n, j, logical)
			}
		}
	}
}

// ----------------------------------------------------------------------------
// 8. 边界锤击
//
// 反复命中每个分支的精确边界值, 并随机改变 data/key。
// 单次测试可能掩盖的问题(比如某个位的累积错误), 在高频重复下会暴露。
// ----------------------------------------------------------------------------

func Test_Chaos_BoundaryHammer(t *testing.T) {
	// 汇编内部分支点: 64(主循环) / 16(次级循环) / 8,4,1(尾部) / 192(分发阈值)
	boundaries := []int{0, 1, 3, 4, 5, 7, 8, 9, 15, 16, 17, 31, 32, 33,
		48, 63, 64, 65, 127, 128, 129, 191, 192, 193,
		255, 256, 257, 383, 384, 385, 512, 1023, 1024, 1025}

	rng := chaosRNG(700)
	for _, n := range boundaries {
		for rep := 0; rep < 300; rep++ {
			src := make([]byte, n)
			rng.Read(src)
			key := rng.Uint32()

			// 交替用三个入口, 结果必须一致
			a := append([]byte(nil), src...)
			Mask(a, key)

			b := append([]byte(nil), src...)
			maskNEON(b, key)

			c := append([]byte(nil), src...)
			maskFast(c, key)

			if !bytes.Equal(a, b) {
				t.Fatalf("边界锤击: Mask 与 maskNEON 不一致 n=%d key=%#x", n, key)
			}
			if !bytes.Equal(a, c) {
				t.Fatalf("边界锤击: Mask 与 maskFast 不一致 n=%d key=%#x", n, key)
			}
		}
	}
}

// ----------------------------------------------------------------------------
// 9. 时间维度上的抖动
//
// 在真实时间流逝中反复调用, 确认结果与系统时间/调度无关
// (即实现里没有依赖时间或全局可变状态)。
// ----------------------------------------------------------------------------

func Test_Chaos_TimeJitter(t *testing.T) {
	src := make([]byte, 777)
	for i := range src {
		src[i] = byte(i * 3)
	}
	key := uint32(0xabcdef01)
	want := append([]byte(nil), src...)
	Mask(want, key)

	deadline := time.Now().Add(300 * time.Millisecond)
	rounds := 0
	for time.Now().Before(deadline) {
		got := append([]byte(nil), src...)
		Mask(got, key)
		if !bytes.Equal(want, got) {
			t.Fatalf("时间抖动下结果不一致 (第 %d 轮)", rounds)
		}
		rounds++
		if rounds%64 == 0 {
			runtime.Gosched()
		}
	}
	t.Logf("时间抖动: %d 轮结果稳定", rounds)
}

// ----------------------------------------------------------------------------
// 10. 指针稳定性
//
// 汇编直接吃切片指针。这里确认大小/内容相同的两个独立切片结果相同,
// 且调用不会改变切片自身的 header (len/cap/ptr)。
// ----------------------------------------------------------------------------

func Test_Chaos_SliceHeaderStability(t *testing.T) {
	rng := chaosRNG(800)

	for i := 0; i < 5000; i++ {
		n := rng.Intn(2000)
		src := make([]byte, n)
		rng.Read(src)
		key := rng.Uint32()

		a := append([]byte(nil), src...)
		b := append([]byte(nil), src...)

		aCap := cap(a)
		var aPtr, bPtr unsafe.Pointer
		if n > 0 {
			aPtr = unsafe.Pointer(unsafe.SliceData(a))
			bPtr = unsafe.Pointer(unsafe.SliceData(b))
			if aPtr == bPtr {
				t.Fatal("测试前提错误: 两个切片共享底层数组")
			}
		}

		Mask(a, key)
		Mask(b, key)

		// 两次独立调用结果必须相同
		if !bytes.Equal(a, b) {
			t.Fatalf("iter=%d: 两个独立切片结果不同 (n=%d key=%#x)", i, n, key)
		}
		// 切片 header 不应被修改
		if cap(a) != aCap {
			t.Fatalf("iter=%d: cap 被修改 %d -> %d", i, aCap, cap(a))
		}
		if n > 0 {
			if unsafe.Pointer(unsafe.SliceData(a)) != aPtr {
				t.Fatalf("iter=%d: 底层指针被修改", i)
			}
			if unsafe.Pointer(unsafe.SliceData(b)) != bPtr {
				t.Fatalf("iter=%d: 第二个切片底层指针被修改", i)
			}
		}
	}
}
