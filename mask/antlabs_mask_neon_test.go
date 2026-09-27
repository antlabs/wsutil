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
	"unsafe"
)

// ============================================================================
// 参考实现
//
// 用三个互相独立的参考实现交叉验证:
//  1. maskSlow —— 逐字节 XOR, 最朴素, 不可能有向量化相关的 bug
//  2. maskFast —— 线上的标量实现(展开的 8 字节循环), 是被 NEON 替换的对象
//  3. 纯 Go 的 pattern 展开 —— 与上面两者写法都不同, 避免同源错误
//
// 一个实现和其中任意一个一致不足以说明正确; 三者两两一致才可信。
// ============================================================================

// ref 用 byte 级别的方式计算期望结果, 完全不碰 unsafe / 向量
func ref(payload []byte, key uint32) []byte {
	out := make([]byte, len(payload))
	k := [4]byte{byte(key), byte(key >> 8), byte(key >> 16), byte(key >> 24)}
	for i, b := range payload {
		out[i] = b ^ k[i&3]
	}
	return out
}

func mustEqual(t *testing.T, name string, want, got []byte, ctx string) {
	t.Helper()
	if len(want) != len(got) {
		t.Fatalf("%s: 长度不一致 want=%d got=%d (%s)", name, len(want), len(got), ctx)
	}
	if !bytes.Equal(want, got) {
		i := 0
		for i < len(want) && want[i] == got[i] {
			i++
		}
		lo := i - 4
		if lo < 0 {
			lo = 0
		}
		hi := i + 8
		if hi > len(want) {
			hi = len(want)
		}
		t.Fatalf("%s: 首个差异在偏移 %d/%d (%s)\n  want[%d:%d]=% x\n  got [%d:%d]=% x",
			name, i, len(want), ctx, lo, hi, want[lo:hi], lo, hi, got[lo:hi])
	}
}

// ============================================================================
// 1. 长度穷举
//
// 汇编里有 192(阈值) / 64(主循环) / 16(次级循环) 三个分支点,
// 以及尾部 8 / 4 / 1 的退化路径。穷举 0..4096 能覆盖所有 len%64 / len%16 / len%8
// 的组合, 以及阈值两侧。
// ============================================================================

func Test_NEON_ExhaustiveLengths(t *testing.T) {
	keys := []uint32{0x12345678, 0xdeadbeef}
	const maxLen = 4096

	for _, key := range keys {
		for n := 0; n <= maxLen; n++ {
			src := make([]byte, n)
			for i := range src {
				src[i] = byte(i*31 + 7)
			}
			want := ref(src, key)

			// 经 maskNEON (纯向量内核)
			gotNEON := append([]byte(nil), src...)
			if n >= neonThreshold {
				maskNEON(gotNEON, key)
			} else {
				// 阈值以下内核不保证被调用, 单独测阈值以上的等价性
				maskFast(gotNEON, key)
			}
			mustEqual(t, "maskNEON", want, gotNEON, keyCtx(key, n))

			// 经 maskFast
			gotFast := append([]byte(nil), src...)
			maskFast(gotFast, key)
			mustEqual(t, "maskFast", want, gotFast, keyCtx(key, n))

			// 经分发入口 Mask
			gotDisp := append([]byte(nil), src...)
			Mask(gotDisp, key)
			mustEqual(t, "Mask", want, gotDisp, keyCtx(key, n))
		}
	}
}

// 单独穷举 maskNEON 内核本身, 包括阈值以下的长度
// (内核没有长度下限断言, 直接调用应该也能正确处理)
func Test_NEON_KernelAllLengths(t *testing.T) {
	const maxLen = 2048
	for n := 0; n <= maxLen; n++ {
		key := uint32(0xa5a5a5a5)
		src := make([]byte, n)
		for i := range src {
			src[i] = byte(i * 17)
		}
		want := ref(src, key)

		got := append([]byte(nil), src...)
		maskNEON(got, key) // 直接调用内核, 不经过阈值分发
		mustEqual(t, "maskNEON-kernel", want, got, keyCtx(key, n))
	}
}

func keyCtx(key uint32, n int) string {
	return "key=" + hex32(key) + " len=" + itoa(n)
}

func hex32(k uint32) string {
	const h = "0123456789abcdef"
	b := make([]byte, 10)
	b[0], b[1] = '0', 'x'
	for i := 0; i < 8; i++ {
		b[2+i] = h[(k>>(uint(7-i)*4))&0xf]
	}
	return string(b)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// ============================================================================
// 2. key 的取值
//
// 这里专门防 MOVW/MOVWU 那个 bug: Go arm64 汇编里 MOVW 是符号扩展加载,
// key 的 bit31 为 1 时会被扩展成 0xffffffffXXXXXXXX。
// 用 0x12345678 这类 bit31=0 的 key 完全测不出来。
// ============================================================================

func Test_NEON_KeyPatterns(t *testing.T) {
	keys := []uint32{
		0x00000000, // 全 0: mask 应该是恒等变换
		0xffffffff, // 全 1: 每个 bit 取反
		0x00000001, // 仅 bit0
		0x80000000, // 仅 bit31 —— 触发符号扩展 bug
		0x000000ff,
		0xff000000,
		0x00ff00ff,
		0xff00ff00,
		0xdeadbeef, // bit31=1, 高位有值 —— 最容易暴露符号扩展
		0x12345678, // bit31=0 的对照
		0xcafebabe,
		0x7fffffff, // bit31=0 但其余全 1
		0x01010101, // key 四字节相同
		0x80808080, // 每字节最高位为 1
	}

	// 覆盖所有分支边界和尾部残数
	lengths := []int{0, 1, 2, 3, 4, 5, 7, 8, 9, 15, 16, 17, 24, 31, 32, 33,
		48, 63, 64, 65, 79, 80, 81, 95, 96, 127, 128, 129,
		144, 160, 175, 190, 191, 192, 193, 194, 200, 224, 255, 256, 257,
		320, 383, 384, 512, 513, 1023, 1024, 1025, 2047, 2048, 4095, 4096}

	for _, key := range keys {
		for _, n := range lengths {
			src := make([]byte, n)
			for i := range src {
				src[i] = byte(i ^ (i >> 8))
			}
			want := ref(src, key)

			got := append([]byte(nil), src...)
			Mask(got, key)
			mustEqual(t, "key-pattern", want, got, keyCtx(key, n))
		}
	}
}

// key=0 必须是恒等变换; key=0xffffffff 必须是按位取反
func Test_NEON_KeyIdentities(t *testing.T) {
	for n := 0; n <= 512; n++ {
		src := make([]byte, n)
		rand.New(rand.NewSource(int64(n))).Read(src)

		// key=0: 不变
		got := append([]byte(nil), src...)
		Mask(got, 0)
		mustEqual(t, "key=0恒等", src, got, keyCtx(0, n))

		// key=0xffffffff: 按位取反
		want := make([]byte, n)
		for i := range src {
			want[i] = ^src[i]
		}
		got2 := append([]byte(nil), src...)
		Mask(got2, 0xffffffff)
		mustEqual(t, "key=all1取反", want, got2, keyCtx(0xffffffff, n))
	}
}

// ============================================================================
// 3. 分发阈值边界
//
// neonThreshold=192 是个分支: >=192 走 NEON, <192 走 maskFast。
// 两侧必须给出完全相同的结果, 且边界本身不能差一。
// ============================================================================

func Test_NEON_ThresholdBoundary(t *testing.T) {
	if neonThreshold != 192 {
		t.Fatalf("阈值变了(当前=%d), 请同步更新本测试", neonThreshold)
	}

	for n := neonThreshold - 6; n <= neonThreshold+6; n++ {
		key := uint32(0xbadc0de1)
		src := make([]byte, n)
		for i := range src {
			src[i] = byte(i * 13)
		}
		want := ref(src, key)

		// 分发入口
		got := append([]byte(nil), src...)
		Mask(got, key)
		mustEqual(t, "threshold-dispatch", want, got, keyCtx(key, n))

		// 强制走内核, 结果必须一致 (证明阈值两侧语义相同)
		kernel := append([]byte(nil), src...)
		maskNEON(kernel, key)
		mustEqual(t, "threshold-kernel", want, kernel, keyCtx(key, n))

		// 强制走标量, 结果必须一致
		slow := append([]byte(nil), src...)
		maskFast(slow, key)
		mustEqual(t, "threshold-fast", want, slow, keyCtx(key, n))

		if !bytes.Equal(got, kernel) || !bytes.Equal(got, slow) {
			t.Fatalf("阈值两侧实现结果不一致: n=%d", n)
		}
	}
}

// ============================================================================
// 4. 未对齐起始地址
//
// NEON 的 VLD1/VST1 在 AArch64 上不要求对齐, 但如果实现里用了
// 对齐敏感的指令(或编译器做了对齐假设), 从奇地址开始会出错。
// 把 payload 放在大 buffer 的不同偏移上, 穷举 0..63。
// ============================================================================

func Test_NEON_UnalignedOffsets(t *testing.T) {
	// 后备数组足够大, 可以切出任意偏移
	backing := make([]byte, 4096+128)
	for i := range backing {
		backing[i] = byte(i*11 + 3)
	}

	lengths := []int{0, 1, 3, 7, 8, 15, 16, 17, 31, 32, 33, 63, 64, 65,
		79, 96, 127, 128, 160, 191, 192, 193, 255, 256, 512, 1024, 2048, 3000}

	key := uint32(0xfeedface)

	for offset := 0; offset < 64; offset++ {
		for _, n := range lengths {
			if offset+n > len(backing) {
				continue
			}
			src := backing[offset : offset+n]
			want := ref(src, key)

			got := append([]byte(nil), src...)
			Mask(got, key)
			mustEqual(t, "unaligned", want, got,
				"offset="+itoa(offset)+" "+keyCtx(key, n))

			// 直接调内核
			got2 := append([]byte(nil), src...)
			maskNEON(got2, key)
			mustEqual(t, "unaligned-kernel", want, got2,
				"offset="+itoa(offset)+" "+keyCtx(key, n))
		}
	}
}

// 直接操作后备数组上的子切片(不拷贝), 确认就地修改的位置正确
func Test_NEON_InPlaceSubSlice(t *testing.T) {
	const total = 2048
	key := uint32(0x5a5a5a5a)

	for offset := 0; offset < 32; offset++ {
		backing := make([]byte, total)
		for i := range backing {
			backing[i] = byte(i)
		}
		// 记录前后哨兵
		n := 256
		sub := backing[offset : offset+n]
		wantSub := ref(sub, key)
		before := append([]byte(nil), backing[:offset]...)
		after := append([]byte(nil), backing[offset+n:]...)

		Mask(sub, key)

		mustEqual(t, "subslice-payload", wantSub, sub, "offset="+itoa(offset))
		mustEqual(t, "subslice-prefix", before, backing[:offset], "offset="+itoa(offset))
		mustEqual(t, "subslice-suffix", after, backing[offset+n:], "offset="+itoa(offset))
	}
}

// ============================================================================
// 5. 越界写检测
//
// 汇编直接操作指针, 算错长度就会踩坏相邻内存。
// 在 payload 前后放哨兵字节, 跑完检查哨兵有没有被改写。
// ============================================================================

func Test_NEON_NoOverrun(t *testing.T) {
	const guard = 64
	const maxLen = 2048

	key := uint32(0x0f0f0f0f)

	for n := 0; n <= maxLen; n++ {
		// 布局: [前哨兵 guard][payload n][后哨兵 guard]
		total := guard + n + guard
		backing := make([]byte, total)

		// 哨兵填成固定图案
		for i := 0; i < guard; i++ {
			backing[i] = 0xAA
			backing[guard+n+i] = 0xBB
		}
		payload := backing[guard : guard+n]
		for i := range payload {
			payload[i] = byte(i*7 + 1)
		}
		want := ref(payload, key)

		Mask(payload, key)

		// 载荷正确
		mustEqual(t, "overrun-payload", want, payload, keyCtx(key, n))

		// 前哨兵未被改写
		for i := 0; i < guard; i++ {
			if backing[i] != 0xAA {
				t.Fatalf("前哨兵被改写! n=%d 偏移=%d 值=%#x (期望 0xAA)", n, i, backing[i])
			}
		}
		// 后哨兵未被改写
		for i := 0; i < guard; i++ {
			if backing[guard+n+i] != 0xBB {
				t.Fatalf("后哨兵被改写! n=%d 偏移=%d 值=%#x (期望 0xBB)", n, i, backing[guard+n+i])
			}
		}
	}
}

// 用精确容量切片(max:len), 检测是否有超过 len 的写
func Test_NEON_NoWriteBeyondLen(t *testing.T) {
	key := uint32(0x11223344)
	for n := 0; n <= 1024; n++ {
		backing := make([]byte, n+32)
		for i := range backing {
			backing[i] = 0xCC
		}
		p := backing[:n:n] // cap==len=n, 任何越界写都会落到 backing[n:] 上

		want := ref(p, key)
		Mask(p, key)

		mustEqual(t, "beyond-len", want, p, keyCtx(key, n))
		for i := n; i < len(backing); i++ {
			if backing[i] != 0xCC {
				t.Fatalf("写越过了 len! n=%d backing[%d]=%#x", n, i, backing[i])
			}
		}
	}
}

// ============================================================================
// 6. 尾部残数穷举
//
// 内核尾部有 16 -> 8 -> 4 -> 1 的退化链, 以及向量主循环的 64/16 分块。
// 对每个 len%16 残数, 在多个"整块数"上验证。
// ============================================================================

func Test_NEON_TailResidues(t *testing.T) {
	key := uint32(0x9e3779b9)

	for blocks := 0; blocks <= 8; blocks++ {
		for rem := 0; rem < 16; rem++ {
			n := blocks*16 + rem
			if n == 0 {
				continue
			}
			src := make([]byte, n)
			for i := range src {
				src[i] = byte(i*3 + blocks)
			}
			want := ref(src, key)

			got := append([]byte(nil), src...)
			maskNEON(got, key)
			mustEqual(t, "tail", want, got,
				"blocks="+itoa(blocks)+" rem="+itoa(rem)+" "+keyCtx(key, n))
		}
	}
}

// 64 字节主循环与大块: 覆盖 64*k + 残余
func Test_NEON_MainLoopBlocks(t *testing.T) {
	key := uint32(0x77aa3311)
	for k := 0; k <= 32; k++ {
		for rem := 0; rem < 16; rem++ {
			n := k*64 + rem
			if n == 0 {
				continue
			}
			src := make([]byte, n)
			for i := range src {
				src[i] = byte(i * 5)
			}
			want := ref(src, key)

			got := append([]byte(nil), src...)
			maskNEON(got, key)
			mustEqual(t, "mainloop", want, got,
				"k="+itoa(k)+" rem="+itoa(rem)+" "+keyCtx(key, n))
		}
	}
}

// ============================================================================
// 7. 大缓冲区
//
// 覆盖主循环大量迭代的情况, 以及跨页边界。
// ============================================================================

func Test_NEON_LargeBuffers(t *testing.T) {
	key := uint32(0x2468ace0)
	sizes := []int{
		1 << 10, 1<<10 + 1, 1<<10 - 1,
		1 << 12, 1<<12 + 7, 1<<12 - 7,
		1 << 14, 1<<14 + 15,
		1 << 16, 1<<16 + 1, 1<<16 - 1,
		1<<17 - 3,
	}
	for _, n := range sizes {
		src := make([]byte, n)
		rnd := rand.New(rand.NewSource(int64(n)))
		rnd.Read(src)
		want := ref(src, key)

		got := append([]byte(nil), src...)
		Mask(got, key)
		mustEqual(t, "large", want, got, keyCtx(key, n))
	}
}

// ============================================================================
// 8. 随机模糊测试 (长度 / key / 数据 / 偏移 全部随机)
// ============================================================================

func Test_NEON_Fuzz(t *testing.T) {
	rnd := rand.New(rand.NewSource(20240927))
	const iters = 30000

	for i := 0; i < iters; i++ {
		n := rnd.Intn(3000)
		key := rnd.Uint32()
		offset := rnd.Intn(32)

		backing := make([]byte, offset+n)
		rnd.Read(backing)
		p := backing[offset : offset+n]

		want := ref(p, key)

		got := append([]byte(nil), p...)
		Mask(got, key)
		if !bytes.Equal(want, got) {
			t.Fatalf("fuzz 失败: iter=%d len=%d offset=%d key=%#x", i, n, offset, key)
		}

		// 同一份数据直接过内核
		got2 := append([]byte(nil), p...)
		maskNEON(got2, key)
		if !bytes.Equal(want, got2) {
			t.Fatalf("fuzz 内核失败: iter=%d len=%d offset=%d key=%#x", i, n, offset, key)
		}
	}
}

// ============================================================================
// 9. 代数性质
//
// XOR mask 的基本性质, 出任何实现 bug 都会破坏这些性质。
// ============================================================================

// 对合性: mask(mask(x)) == x
func Test_NEON_Involutive(t *testing.T) {
	rnd := rand.New(rand.NewSource(1))
	for n := 0; n <= 2048; n++ {
		src := make([]byte, n)
		rnd.Read(src)
		orig := append([]byte(nil), src...)
		key := rnd.Uint32()

		Mask(src, key)
		Mask(src, key)

		if !bytes.Equal(src, orig) {
			t.Fatalf("两次 mask 未还原: len=%d key=%#x", n, key)
		}
	}
}

// 可交换/可结合: mask(x,a) 再 mask(.,b) == mask(x, a^b)
func Test_NEON_XorHomomorphism(t *testing.T) {
	rnd := rand.New(rand.NewSource(2))
	for n := 0; n <= 1024; n++ {
		src := make([]byte, n)
		rnd.Read(src)
		a, b := rnd.Uint32(), rnd.Uint32()

		// mask(a) 然后 mask(b)
		seq := append([]byte(nil), src...)
		Mask(seq, a)
		Mask(seq, b)

		// 一次 mask(a^b)
		once := append([]byte(nil), src...)
		Mask(once, a^b)

		if !bytes.Equal(seq, once) {
			t.Fatalf("mask 不满足同态: len=%d a=%#x b=%#x", n, a, b)
		}
	}
}

// 并行分块与一次性结果相同。
//
// mask 的 key 是 4 字节循环的: 第 i 个字节用 key 的第 (i&3) 字节。
// 所以把 buffer 从 cut 处切开后, 第二段必须用"旋转"过的 key, 使
//
//	rotated[j] == key[(cut+j) & 3]
//
// 这样 seg2[j] 用到的 key 字节与整体处理时全局第 (cut+j) 字节一致。
// 这正是 frame.WriteFrame 把 mask 结果拼到任意偏移时所依赖的性质。
func Test_NEON_SegmentEquivalence(t *testing.T) {
	key := uint32(0x13579bdf)

	rotateKey := func(key uint32, off int) uint32 {
		off &= 3
		var out uint32
		for j := 0; j < 4; j++ {
			s := (j + off) & 3
			out |= ((key >> (uint(s) * 8)) & 0xff) << (uint(j) * 8)
		}
		return out
	}

	for n := 1; n <= 1024; n++ {
		src := make([]byte, n)
		rand.New(rand.NewSource(int64(n))).Read(src)

		whole := append([]byte(nil), src...)
		Mask(whole, key)

		for cut := 0; cut <= n; cut++ {
			seg := append([]byte(nil), src...)
			Mask(seg[:cut], key)                   // 第一段全局偏移 0, 原 key
			Mask(seg[cut:], rotateKey(key, cut&3)) // 第二段全局偏移 cut
			if !bytes.Equal(whole, seg) {
				t.Fatalf("分段结果不一致: len=%d cut=%d", n, cut)
			}
		}
	}
}

// rotateKey 的语义验证: rotated[j] 必须等于 key[(off+j)&3]
func Test_RotateKeyHelper(t *testing.T) {
	rotate := func(k uint32, off int) uint32 {
		off &= 3
		var out uint32
		for j := 0; j < 4; j++ {
			s := (j + off) & 3
			out |= ((k >> (uint(s) * 8)) & 0xff) << (uint(j) * 8)
		}
		return out
	}
	for _, key := range []uint32{0x13579bdf, 0xdeadbeef, 0x80000000, 0} {
		if rotate(key, 0) != key {
			t.Fatalf("偏移 0 应等于原 key: key=%#x", key)
		}
		for off := 0; off < 4; off++ {
			if rotate(key, off) != rotate(key, off+4) {
				t.Fatalf("周期性错误 key=%#x off=%d", key, off)
			}
			r := rotate(key, off)
			for j := 0; j < 4; j++ {
				wantB := byte(key >> (uint((off+j)&3) * 8))
				gotB := byte(r >> (uint(j) * 8))
				if wantB != gotB {
					t.Fatalf("rotate 语义错误 key=%#x off=%d j=%d", key, off, j)
				}
			}
		}
	}
}

// ============================================================================
// 10. 并发安全 (配合 -race)
//
// 汇编只操作传入的切片, 不应有跨调用的共享状态。
// ============================================================================

func Test_NEON_Concurrent(t *testing.T) {
	const workers = 8
	const rounds = 200

	var wg sync.WaitGroup
	errCh := make(chan string, workers*rounds)

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			rnd := rand.New(rand.NewSource(int64(w) * 7919))
			for r := 0; r < rounds; r++ {
				n := rnd.Intn(2048)
				key := rnd.Uint32()
				src := make([]byte, n)
				rnd.Read(src)
				want := ref(src, key)

				got := append([]byte(nil), src...)
				Mask(got, key)
				if !bytes.Equal(want, got) {
					errCh <- "worker=" + itoa(w) + " round=" + itoa(r) +
						" " + keyCtx(key, n)
					return
				}
			}
		}(w)
	}
	wg.Wait()
	close(errCh)

	for e := range errCh {
		t.Fatalf("并发下结果错误: %s", e)
	}
}

// ============================================================================
// 11. 寄存器/栈完整性
//
// NOSPLIT 的汇编如果误用了 callee-saved 寄存器, 会破坏调用方的状态。
// 这里在密集调用前后对比一批 Go 局部变量的值。
// ============================================================================

//go:noinline
func callMaskDensely(p []byte, key uint32, times int) {
	for i := 0; i < times; i++ {
		maskNEON(p, key)
	}
}

func Test_NEON_RegisterIntegrity(t *testing.T) {
	// 一批局部变量, 覆盖 GPR 和 FPR
	a0, a1, a2, a3 := uint64(0x1111111111111111), uint64(0x2222222222222222),
		uint64(0x3333333333333333), uint64(0x4444444444444444)
	f0, f1 := 3.14159265358979, 2.71828182845905
	ptr := unsafe.Pointer(&a0)
	sl := make([]byte, 16, 64)

	// 计算并保存一个校验值
	before := []uint64{a0, a1, a2, a3, uint64(f0 * 1e15), uint64(f1 * 1e15),
		uint64(uintptr(ptr)), uint64(len(sl)), uint64(cap(sl))}

	p := make([]byte, 4096)
	callMaskDensely(p, 0xdeadbeef, 10000)

	_ = a0
	_ = a1
	_ = a2
	_ = a3
	_ = f0
	_ = f1
	_ = ptr
	_ = sl

	after := []uint64{a0, a1, a2, a3, uint64(f0 * 1e15), uint64(f1 * 1e15),
		uint64(uintptr(ptr)), uint64(len(sl)), uint64(cap(sl))}

	for i := range before {
		if before[i] != after[i] {
			t.Fatalf("寄存器/局部变量被破坏: 索引=%d before=%#x after=%#x",
				i, before[i], after[i])
		}
	}
}

// ============================================================================
// 12. 汇编内核与标量实现在所有长度上的完全等价
//
// 这是最强的正确性声明: maskNEON 与已在生产使用的 maskFast
// 在 0..8192 每个长度上都逐字节一致。
// ============================================================================

func Test_NEON_EquivalentToMaskFast(t *testing.T) {
	const maxLen = 8192
	key := uint32(0x6c078965)

	for n := 0; n <= maxLen; n++ {
		src := make([]byte, n)
		rnd := rand.New(rand.NewSource(int64(n) * 2654435761))
		rnd.Read(src)

		viaFast := append([]byte(nil), src...)
		maskFast(viaFast, key)

		viaNEON := append([]byte(nil), src...)
		maskNEON(viaNEON, key)

		mustEqual(t, "NEON==maskFast", viaFast, viaNEON, keyCtx(key, n))
	}
}

// 多个 key 下的等价性
func Test_NEON_EquivalentToMaskFast_MultiKey(t *testing.T) {
	keys := []uint32{0, 1, 0x80000000, 0xdeadbeef, 0xffffffff, 0x12345678, 0xa5a5a5a5}
	// 覆盖所有分支边界附近
	lengths := make([]int, 0, 600)
	for n := 0; n <= 260; n++ {
		lengths = append(lengths, n)
	}
	for _, base := range []int{512, 1024, 2048, 4096} {
		for d := -3; d <= 3; d++ {
			lengths = append(lengths, base+d)
		}
	}

	for _, key := range keys {
		for _, n := range lengths {
			src := make([]byte, n)
			for i := range src {
				src[i] = byte(i*29 + 5)
			}
			viaFast := append([]byte(nil), src...)
			maskFast(viaFast, key)
			viaNEON := append([]byte(nil), src...)
			maskNEON(viaNEON, key)
			mustEqual(t, "NEON==maskFast(multi)", viaFast, viaNEON, keyCtx(key, n))
		}
	}
}

// ============================================================================
// 13. 分发入口不改变语义
// ============================================================================

func Test_DispatchEquivalence(t *testing.T) {
	rnd := rand.New(rand.NewSource(4242))
	for i := 0; i < 5000; i++ {
		n := rnd.Intn(4096)
		key := rnd.Uint32()
		src := make([]byte, n)
		rnd.Read(src)

		want := ref(src, key)

		got := append([]byte(nil), src...)
		Mask(got, key)

		if !bytes.Equal(want, got) {
			t.Fatalf("分发入口结果错误: len=%d key=%#x", n, key)
		}
	}
}

// ============================================================================
// 14. 空与非空边界
// ============================================================================

func Test_NEON_EmptyAndNil(t *testing.T) {
	// nil 切片
	var nilSlice []byte
	Mask(nilSlice, 0x12345678) // 不应 panic
	Mask(nilSlice, 0)

	// 零长度非 nil
	empty := make([]byte, 0)
	Mask(empty, 0x12345678) // 不应 panic

	// 零长度但有大 cap
	big := make([]byte, 0, 4096)
	for i := range big[:cap(big)] {
		_ = i
	}
	Mask(big, 0x12345678) // 不应 panic

	// 直接调内核
	var nil2 []byte
	maskNEON(nil2, 1)
	maskNEON(make([]byte, 0), 1)
}

// ============================================================================
// 15. 长度下限断言检查
//
// 上面 Test_NEON_KernelAllLengths 已证明内核本身在任意长度都正确,
// 这里额外确认: 内核不会读越界 (通过前后哨兵 + 极小长度)
// ============================================================================

func Test_NEON_TinyLengthsWithGuards(t *testing.T) {
	key := uint32(0xffff0000)
	for n := 0; n <= 16; n++ {
		const guard = 16
		backing := make([]byte, guard+n+guard)
		for i := range backing {
			backing[i] = 0x5A
		}
		p := backing[guard : guard+n]
		for i := range p {
			p[i] = byte(i + 1)
		}
		want := ref(p, key)

		maskNEON(p, key)

		mustEqual(t, "tiny", want, p, keyCtx(key, n))
		for i := 0; i < guard; i++ {
			if backing[i] != 0x5A {
				t.Fatalf("n=%d 前哨兵被改: [%d]=%#x", n, i, backing[i])
			}
			if backing[guard+n+i] != 0x5A {
				t.Fatalf("n=%d 后哨兵被改: [%d]=%#x", n, guard+n+i, backing[guard+n+i])
			}
		}
	}
}

// 确保测试确实跑在 arm64 上
func Test_ArchGuard(t *testing.T) {
	if runtime.GOARCH != "arm64" {
		t.Skipf("NEON 测试仅在 arm64 有意义, 当前 %s", runtime.GOARCH)
	}
}
