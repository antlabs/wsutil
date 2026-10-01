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

import (
	"bytes"
	"math/rand"
	"runtime"
	"sync"
	"testing"
)

// ============================================================================
// 参考实现
//
// 用三个互相独立的参考实现交叉验证:
//  1. maskSlow —— 逐字节 XOR, 最朴素, 不可能有向量化相关的 bug
//  2. maskFast —— 线上的标量实现(展开的 8 字节循环), 是被替换的对象
//  3. ref —— 纯 Go 按 i&3 取 key 字节, 与上面两者写法都不同
//
// 一个实现和其中任意一个一致不足以说明正确; 三者两两一致才可信。
// 与 NEON 版测试同样的方法论。
// ============================================================================

// ref 用 byte 级别的方式计算期望结果, 完全不碰 unsafe / 向量
func refAMD64(payload []byte, key uint32) []byte {
	out := make([]byte, len(payload))
	k := [4]byte{byte(key), byte(key >> 8), byte(key >> 16), byte(key >> 24)}
	for i, b := range payload {
		out[i] = b ^ k[i&3]
	}
	return out
}

func mustEqualAMD64(t *testing.T, name string, want, got []byte, ctx string) {
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
// 汇编里有 128(主循环) / 64(次级循环) / 32(尾循环, AVX2) / 16(SSE2)
// 以及尾部 8 / 4 / 1 的退化路径。穷举 0..4096 能覆盖所有
// len%128 / len%64 / len%32 / len%16 / len%8 的组合。
// ============================================================================

func Test_AVX2_ExhaustiveLengths(t *testing.T) {
	keys := []uint32{0x12345678, 0xdeadbeef}
	const maxLen = 4096

	for _, key := range keys {
		for n := 0; n <= maxLen; n++ {
			src := make([]byte, n)
			for i := range src {
				src[i] = byte(i*31 + 7)
			}
			want := refAMD64(src, key)

			// 纯向量内核(不带阈值分发), 任意长度直接调用
			gotVec := append([]byte(nil), src...)
			maskAVX2(gotVec, key)
			mustEqualAMD64(t, "maskAVX2-kernel", want, gotVec, ctxAMD64(key, n))

			// SSE2 内核
			gotSSE := append([]byte(nil), src...)
			maskSSE2(gotSSE, key)
			mustEqualAMD64(t, "maskSSE2-kernel", want, gotSSE, ctxAMD64(key, n))

			// 标量实现
			gotFast := append([]byte(nil), src...)
			maskFast(gotFast, key)
			mustEqualAMD64(t, "maskFast", want, gotFast, ctxAMD64(key, n))

			// 分发入口(按当前 CPU 能力决定走哪条)
			gotDisp := append([]byte(nil), src...)
			Mask(gotDisp, key)
			mustEqualAMD64(t, "Mask", want, gotDisp, ctxAMD64(key, n))
		}
	}
}

func ctxAMD64(key uint32, n int) string {
	return "key=" + hex32AMD64(key) + " len=" + itoaAMD64(n)
}

func hex32AMD64(k uint32) string {
	const h = "0123456789abcdef"
	b := make([]byte, 10)
	b[0], b[1] = '0', 'x'
	for i := 0; i < 8; i++ {
		b[2+i] = h[(k>>(uint(7-i)*4))&0xf]
	}
	return string(b)
}

func itoaAMD64(n int) string {
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
// 这里防的是 "只测低位 key" 这类盲区:
// 广播/移位如果有符号扩展或截断 bug, 高位为 1 的 key 才暴露得出来。
// ============================================================================

func Test_AVX2_KeyPatterns(t *testing.T) {
	keys := []uint32{
		0x00000000, // 全 0: mask 应该是恒等变换
		0xffffffff, // 全 1: 每个 bit 取反
		0x00000001, // 仅 bit0
		0x80000000, // 仅 bit31
		0x000000ff,
		0xff000000,
		0x00ff00ff,
		0xff00ff00,
		0xdeadbeef,
		0x12345678,
		0xcafebabe,
		0x7fffffff,
		0x01010101, // key 四字节相同
		0x80808080,
	}

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
			want := refAMD64(src, key)

			for _, impl := range []struct {
				name string
				fn   func([]byte, uint32)
			}{
				{"maskAVX2", maskAVX2},
				{"maskSSE2", maskSSE2},
				{"Mask", Mask},
			} {
				got := append([]byte(nil), src...)
				impl.fn(got, key)
				mustEqualAMD64(t, "key-pattern/"+impl.name, want, got, ctxAMD64(key, n))
			}
		}
	}
}

// key=0 必须是恒等变换; key=0xffffffff 必须是按位取反
func Test_AVX2_KeyIdentities(t *testing.T) {
	for n := 0; n <= 512; n++ {
		src := make([]byte, n)
		rand.New(rand.NewSource(int64(n))).Read(src)

		for _, impl := range []struct {
			name string
			fn   func([]byte, uint32)
		}{
			{"maskAVX2", maskAVX2},
			{"maskSSE2", maskSSE2},
		} {
			// key=0: 不变
			got := append([]byte(nil), src...)
			impl.fn(got, 0)
			mustEqualAMD64(t, "key=0恒等/"+impl.name, src, got, ctxAMD64(0, n))

			// key=0xffffffff: 按位取反
			want := make([]byte, n)
			for i := range src {
				want[i] = ^src[i]
			}
			got2 := append([]byte(nil), src...)
			impl.fn(got2, 0xffffffff)
			mustEqualAMD64(t, "key=all1取反/"+impl.name, want, got2, ctxAMD64(0xffffffff, n))
		}
	}
}

// ============================================================================
// 3. 分发阈值边界
// ============================================================================

func Test_AVX2_ThresholdBoundary(t *testing.T) {
	key := uint32(0xcafebabe)

	// AVX2 阈值两侧
	for n := amd64AVX2Threshold - 8; n <= amd64AVX2Threshold+8 && n >= 0; n++ {
		src := make([]byte, n)
		for i := range src {
			src[i] = byte(i*3 + 1)
		}
		want := refAMD64(src, key)

		got := append([]byte(nil), src...)
		Mask(got, key)
		mustEqualAMD64(t, "threshold-avx2", want, got, ctxAMD64(key, n))
	}

	// SSE2 阈值两侧
	for n := amd64SSE2Threshold - 8; n <= amd64SSE2Threshold+8 && n >= 0; n++ {
		src := make([]byte, n)
		for i := range src {
			src[i] = byte(i*5 + 2)
		}
		want := refAMD64(src, key)

		got := append([]byte(nil), src...)
		Mask(got, key)
		mustEqualAMD64(t, "threshold-sse2", want, got, ctxAMD64(key, n))
	}
}

// 小尺寸(< amd64SSE2Threshold)走的是 maskFast, 这里验证分发入口
// 在该区间内与 maskFast 逐字节一致 —— 确保阈值判断本身没有把边界
// 上的长度错误地送进向量内核, 也确保两边对 key 的字节序处理一致。
//
// 重点覆盖尾部 0..3 字节: maskFast 尾部用 maskSlow 逐字节取 key,
// 向量内核用 RORL 滚动取 key, 两个内核的边界行为必须一致。
func Test_AVX2_SmallSizeEquivalence(t *testing.T) {
	keys := []uint32{
		0, 1, 0xffffffff, 0x80000000, 0xdeadbeef,
		0x01010101, 0x12345678, 0x00ff00ff, 0xff00ff00,
	}

	for _, key := range keys {
		for n := 0; n < amd64SSE2Threshold; n++ {
			src := make([]byte, n)
			for i := range src {
				src[i] = byte(i*23 + 11)
			}

			want := append([]byte(nil), src...)
			maskFast(want, key)

			got := append([]byte(nil), src...)
			Mask(got, key) // 走内联标量段

			mustEqualAMD64(t, "inline-scalar", want, got, ctxAMD64(key, n))

			// 同时用独立参考实现交叉验证
			refWant := refAMD64(src, key)
			mustEqualAMD64(t, "inline-scalar-ref", refWant, got, ctxAMD64(key, n))
		}
	}
}

// ============================================================================
// 4. 非对齐起点
//
// 向量 load/store 用 VMOVDQU/MOVOU 本身就是非对齐安全的,
// 但切片起点落在任意字节偏移时容易在"指针推进"上出错。
// ============================================================================

func Test_AVX2_UnalignedOffsets(t *testing.T) {
	key := uint32(0x5a5a5a5a)
	const maxLen = 512

	for off := 0; off < 64; off++ {
		backing := make([]byte, off+maxLen+64)
		for i := range backing {
			backing[i] = 0xEE
		}
		for n := 0; n <= maxLen; n += 7 {
			p := backing[off : off+n]
			for i := range p {
				p[i] = byte(i * 13)
			}
			want := refAMD64(p, key)

			got := append([]byte(nil), p...)
			Mask(got, key)
			mustEqualAMD64(t, "unaligned", want, got, ctxAMD64(key, n)+" off="+itoaAMD64(off))
		}
	}
}

// ============================================================================
// 5. 边界: 前后哨兵不能被改写
//
// 向量路径有 VMOVDQU Y(256bit) 这种 32 字节的读写,
// 尾部如果不精确, 最容易越界写。哨兵是最直接的证据。
// ============================================================================

func Test_AVX2_NoOverrun(t *testing.T) {
	const guard = 64
	const maxLen = 2048

	key := uint32(0x0f0f0f0f)

	for n := 0; n <= maxLen; n++ {
		total := guard + n + guard
		backing := make([]byte, total)

		for i := 0; i < guard; i++ {
			backing[i] = 0xAA
			backing[guard+n+i] = 0xBB
		}
		payload := backing[guard : guard+n]
		for i := range payload {
			payload[i] = byte(i*7 + 1)
		}
		want := refAMD64(payload, key)

		Mask(payload, key)

		mustEqualAMD64(t, "overrun-payload", want, payload, ctxAMD64(key, n))

		for i := 0; i < guard; i++ {
			if backing[i] != 0xAA {
				t.Fatalf("前哨兵被改写! n=%d 偏移=%d 值=%#x (期望 0xAA)", n, i, backing[i])
			}
		}
		for i := 0; i < guard; i++ {
			if backing[guard+n+i] != 0xBB {
				t.Fatalf("后哨兵被改写! n=%d 偏移=%d 值=%#x (期望 0xBB)", n, i, backing[guard+n+i])
			}
		}
	}
}

// 用精确容量切片(max:len), 检测是否有超过 len 的写
func Test_AVX2_NoWriteBeyondLen(t *testing.T) {
	key := uint32(0x11223344)
	for n := 0; n <= 1024; n++ {
		backing := make([]byte, n+64)
		for i := range backing {
			backing[i] = 0xCC
		}
		p := backing[:n:n] // cap==len=n, 任何越界写都会落到 backing[n:] 上

		want := refAMD64(p, key)
		Mask(p, key)

		mustEqualAMD64(t, "beyond-len", want, p, ctxAMD64(key, n))
		for i := n; i < len(backing); i++ {
			if backing[i] != 0xCC {
				t.Fatalf("写越过了 len! n=%d backing[%d]=%#x", n, i, backing[i])
			}
		}
	}
}

// ============================================================================
// 6. 尾部残数穷举
// ============================================================================

func Test_AVX2_TailResidues(t *testing.T) {
	key := uint32(0x9e3779b9)

	// AVX2: 主循环 128, 尾循环 32, 标量尾 8/4/1
	for blocks := 0; blocks <= 4; blocks++ {
		for rem := 0; rem < 32; rem++ {
			n := blocks*128 + rem
			src := make([]byte, n)
			for i := range src {
				src[i] = byte(i*11 + 5)
			}
			want := refAMD64(src, key)

			got := append([]byte(nil), src...)
			maskAVX2(got, key)
			mustEqualAMD64(t, "avx2-tail", want, got, ctxAMD64(key, n))
		}
	}

	// SSE2: 主循环 64, 尾循环 16
	for blocks := 0; blocks <= 6; blocks++ {
		for rem := 0; rem < 16; rem++ {
			n := blocks*64 + rem
			src := make([]byte, n)
			for i := range src {
				src[i] = byte(i*11 + 5)
			}
			want := refAMD64(src, key)

			got := append([]byte(nil), src...)
			maskSSE2(got, key)
			mustEqualAMD64(t, "sse2-tail", want, got, ctxAMD64(key, n))
		}
	}
}

// ============================================================================
// 7. 大缓冲
// ============================================================================

func Test_AVX2_LargeBuffers(t *testing.T) {
	key := uint32(0xa5a5a5a5)
	sizes := []int{4096, 8192, 16384, 65536, 1 << 20}

	for _, n := range sizes {
		src := make([]byte, n)
		rand.New(rand.NewSource(int64(n))).Read(src)
		// 让 key 也能覆盖高位
		key = key*2654435761 + 1

		want := refAMD64(src, key)

		got := append([]byte(nil), src...)
		Mask(got, key)
		mustEqualAMD64(t, "large", want, got, ctxAMD64(key, n))
	}
}

// ============================================================================
// 8. 随机模糊
// ============================================================================

func Test_AVX2_Fuzz(t *testing.T) {
	rng := rand.New(rand.NewSource(20241002))

	for iter := 0; iter < 4000; iter++ {
		n := rng.Intn(3000)
		key := rng.Uint32()

		src := make([]byte, n)
		rng.Read(src)

		want := refAMD64(src, key)

		for _, impl := range []struct {
			name string
			fn   func([]byte, uint32)
		}{
			{"maskAVX2", maskAVX2},
			{"maskSSE2", maskSSE2},
			{"maskFast", maskFast},
			{"Mask", Mask},
		} {
			got := append([]byte(nil), src...)
			impl.fn(got, key)
			mustEqualAMD64(t, "fuzz/"+impl.name, want, got, ctxAMD64(key, n))
		}
	}
}

// ============================================================================
// 9. 代数性质
//
// XOR mask 是自身的逆变换。做两遍必须回到原值 —— 这个性质能抓到
// "写入位置和读取位置不一致" 这类 bug, 是逐字节对比之外的独立证据。
// ============================================================================

func Test_AVX2_Involutive(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for n := 0; n <= 1500; n++ {
		src := make([]byte, n)
		rng.Read(src)
		key := rng.Uint32()

		got := append([]byte(nil), src...)
		Mask(got, key)
		Mask(got, key)
		mustEqualAMD64(t, "involutive", src, got, ctxAMD64(key, n))
	}
}

// ============================================================================
// 10. 并发
//
// 内核是无状态的, 多 goroutine 并发调用不同缓冲区必须互不干扰。
// ============================================================================

func Test_AVX2_Concurrent(t *testing.T) {
	const workers = 16
	const iters = 300

	var wg sync.WaitGroup
	errCh := make(chan string, workers)

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(int64(w)))
			for i := 0; i < iters; i++ {
				n := rng.Intn(2000)
				key := rng.Uint32()
				src := make([]byte, n)
				rng.Read(src)
				want := refAMD64(src, key)

				got := append([]byte(nil), src...)
				Mask(got, key)
				if !bytes.Equal(want, got) {
					errCh <- "worker " + itoaAMD64(w) + " 不一致"
					return
				}
			}
		}(w)
	}

	wg.Wait()
	close(errCh)
	for msg := range errCh {
		t.Fatal(msg)
	}
}

// ============================================================================
// 11. 空与 nil
// ============================================================================

func Test_AVX2_EmptyAndNil(t *testing.T) {
	for _, impl := range []struct {
		name string
		fn   func([]byte, uint32)
	}{
		{"maskAVX2", maskAVX2},
		{"maskSSE2", maskSSE2},
		{"maskFast", maskFast},
		{"Mask", Mask},
	} {
		impl.fn(nil, 0x12345678)
		impl.fn([]byte{}, 0x12345678)
		impl.fn(make([]byte, 0, 64), 0x12345678)
	}
}

// ============================================================================
// 12. 与标量实现逐字节等价
// ============================================================================

func Test_AVX2_EquivalentToMaskFast(t *testing.T) {
	rng := rand.New(rand.NewSource(99))
	key := uint32(0xdeadbeef)

	for n := 0; n <= 4096; n++ {
		src := make([]byte, n)
		rng.Read(src)

		want := append([]byte(nil), src...)
		maskFast(want, key)

		gotAVX := append([]byte(nil), src...)
		maskAVX2(gotAVX, key)

		gotSSE := append([]byte(nil), src...)
		maskSSE2(gotSSE, key)

		mustEqualAMD64(t, "avx2-vs-fast", want, gotAVX, ctxAMD64(key, n))
		mustEqualAMD64(t, "sse2-vs-fast", want, gotSSE, ctxAMD64(key, n))
	}
}

func Test_AVX2_EquivalentToMaskFast_MultiKey(t *testing.T) {
	keys := []uint32{0, 1, 0xffffffff, 0x80000000, 0xdeadbeef, 0xcafebabe, 0x01010101}
	for _, key := range keys {
		for n := 0; n <= 1000; n++ {
			src := make([]byte, n)
			for i := range src {
				src[i] = byte(i*29 + int(key&0xff))
			}

			want := append([]byte(nil), src...)
			maskFast(want, key)

			gotAVX := append([]byte(nil), src...)
			maskAVX2(gotAVX, key)
			mustEqualAMD64(t, "multikey-avx2", want, gotAVX, ctxAMD64(key, n))

			gotSSE := append([]byte(nil), src...)
			maskSSE2(gotSSE, key)
			mustEqualAMD64(t, "multikey-sse2", want, gotSSE, ctxAMD64(key, n))
		}
	}
}

// ============================================================================
// 13. 检测逻辑
// ============================================================================

func Test_AVX2_DetectionConsistent(t *testing.T) {
	// 检测结果必须稳定(不能每次调用都不一样)
	first := detectAVX2()
	for i := 0; i < 100; i++ {
		if detectAVX2() != first {
			t.Fatal("detectAVX2 结果不稳定")
		}
	}

	// cpuid/xgetbv 不能 panic(即 CPUID 本身可用)
	maxID, _, _, _ := cpuid(0, 0)
	if maxID < 1 {
		t.Skipf("CPUID 叶 0 返回 maxID=%d, 预期 >=1", maxID)
	}
}

// 确保测试确实跑在 amd64 上
func Test_AVX2_ArchGuard(t *testing.T) {
	if runtime.GOARCH != "amd64" {
		t.Skipf("AVX2 测试仅在 amd64 有意义, 当前 %s", runtime.GOARCH)
	}
}
