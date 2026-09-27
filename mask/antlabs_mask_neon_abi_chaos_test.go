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
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ============================================================================
// ABI / 运行时契约层面的混沌测试
//
// 前面的测试都在验证"算出来的字节对不对"。这一组验证的是汇编与
// Go 运行时之间的契约 —— 这类问题往往不影响单次结果, 却会在
// 特定调度或内存布局下炸掉整个进程:
//
//   - 保留寄存器(R18_PLATFORM/g/R29)是否被破坏
//   - 在深栈 / 抢占点 / GC 扫描期间调用是否安全
//   - 通过 reflect 等非常规路径调用是否安全
//   - 汇编是否真的被编译进来并执行(而非静默回退到 Go 实现)
// ============================================================================

// ----------------------------------------------------------------------------
// 7. 汇编确实被执行 (函数身份自检)
//
// 防止"因为构建约束写错, 悄悄回退到 Go 实现"这类问题 ——
// 那种情况下所有结果比对都会通过, 但测的根本不是汇编。
//
// 这里只做函数身份与可达性检查, 不去读机器码: 读机器码需要
// uintptr -> unsafe.Pointer 转换(会被 go vet 警告), 且它想证明的
// 事情(代码段里确实是 NEON)已由函数身份覆盖。汇编真的在执行这一点,
// 由变异测试与性能差异共同佐证。
// ----------------------------------------------------------------------------

func Test_AbiChaos_AssemblyActuallyUsed(t *testing.T) {
	fnName := func(f func([]byte, uint32)) string {
		if f == nil {
			return "<nil>"
		}
		return runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name()
	}

	asmName := fnName(maskNEON)
	fastName := fnName(maskFast)

	if asmName == "" || fastName == "" {
		t.Fatalf("函数身份解析失败: asm=%q fast=%q", asmName, fastName)
	}
	if asmName == fastName {
		t.Fatalf("汇编内核与 Go 实现是同一个函数 —— 汇编未被编译进来: %s", asmName)
	}
	if !strings.Contains(asmName, "maskNEON") {
		t.Fatalf("汇编内核函数名异常: %s", asmName)
	}
	if strings.Contains(fastName, "maskNEON") {
		t.Fatalf("maskFast 指向了 NEON 实现: %s", fastName)
	}

	// 分发器必须指向 maskDispatch, 且阈值不能是 0(否则汇编永不执行)
	if runtime.FuncForPC(reflect.ValueOf(Mask).Pointer()).Name() !=
		runtime.FuncForPC(reflect.ValueOf(maskDispatch).Pointer()).Name() {
		t.Fatal("Mask 未指向 maskDispatch")
	}
	if neonThreshold <= 0 {
		t.Fatalf("neonThreshold=%d 会让汇编永不执行", neonThreshold)
	}

	// 内核必须能处理阈值以下的长度(将来若下调阈值不会出问题)
	for _, n := range []int{0, 1, 8, 16, 63, 64, 65, 191, 192, 193} {
		p := make([]byte, n)
		for i := range p {
			p[i] = byte(i + 1)
		}
		maskNEON(p, 0xA5A5A5A5) // 不得 panic / 越界
	}

	t.Logf("汇编内核=%s  Go 实现=%s  threshold=%d", asmName, fastName, neonThreshold)
}

func Test_AbiChaos_ReservedRegisters(t *testing.T) {
	rng := rand.New(rand.NewSource(0xab1))

	// 覆盖所有分支: no64 / 主循环 / 次级循环 / 各种尾部残数
	lengths := []int{0, 1, 7, 8, 15, 16, 17, 31, 32, 63, 64, 65,
		127, 128, 191, 192, 193, 255, 256, 511, 512, 1023, 1024, 4096, 16384}

	for _, n := range lengths {
		p := make([]byte, n)
		rng.Read(p)

		clobbered := regClobberProbe(p, rng.Uint32())
		if clobbered != 0 {
			names := []string{}
			if clobbered&1 != 0 {
				names = append(names, "R18_PLATFORM")
			}
			if clobbered&2 != 0 {
				names = append(names, "g(R28)")
			}
			if clobbered&4 != 0 {
				names = append(names, "R29(FP)")
			}
			t.Fatalf("保留寄存器被破坏! n=%d 位图=%#b 涉及: %v", n, clobbered, names)
		}
	}

	// 随机长度再打一遍
	for i := 0; i < 20000; i++ {
		n := rng.Intn(5000)
		p := make([]byte, n)
		rng.Read(p)
		if c := regClobberProbe(p, rng.Uint32()); c != 0 {
			t.Fatalf("保留寄存器被破坏 (随机) iter=%d n=%d 位图=%#b", i, n, c)
		}
	}
}

// ############################################################################
// 2. 栈契约压力
//
// 汇编声明 NOSPLIT 且 $0 栈帧(不占栈)。这类函数:
//   - 不能被抢占点打断(不能调用任何可能增长的函数)
//   - 必须不触碰比声明更多的栈空间
//
// 这里在接近栈边界处反复调用, 并在多个 goroutine 里同时进行。
// ############################################################################

//go:noinline
func stackStressCall(depth int, p []byte, key uint32, counts *int64) {
	// 在栈上放一批数据, 让每层栈帧尽可能大, 逼近平常的栈增长点
	var pad [512]byte
	pad[depth&511] = byte(depth)

	if depth <= 0 {
		// 此刻栈已经很深, 在这里调用汇编
		maskNEON(p, key)
		atomic.AddInt64(counts, 1)
		runtime.KeepAlive(pad[:])
		return
	}
	stackStressCall(depth-1, p, key, counts)
	runtime.KeepAlive(pad[:])
}

func Test_AbiChaos_StackStress(t *testing.T) {
	var calls int64
	var errs int64

	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(int64(g) * 31337))
			for r := 0; r < 50; r++ {
				n := rng.Intn(3000)
				p := make([]byte, n)
				rng.Read(p)
				key := rng.Uint32()

				want := append([]byte(nil), p...)
				maskFast(want, key)

				// 深栈调用
				stackStressCall(64, p, key, &calls)

				// 注意: 不能在非 test goroutine 里调 t.Fatalf, 它只终止
				// 当前协程且会被 testing 框架忽略。这里累积错误, 主协程再断言。
				if !bytes.Equal(want, p) {
					atomic.AddInt64(&errs, 1)
				}
			}
		}(g)
	}
	wg.Wait()

	if errs != 0 {
		t.Fatalf("深栈调用有 %d 次结果错误", errs)
	}
	if calls == 0 {
		t.Fatal("没有实际调用到汇编, 测试无效")
	}
	t.Logf("深栈下调用 %d 次, 无异常", calls)
}

// 抢占点前后的调用。Go 的异步抢占会中断 goroutine 并扫描其栈,
// 汇编若在此刻留下不一致的栈状态就会出问题。
func Test_AbiChaos_AsyncPreempt(t *testing.T) {
	stop := make(chan struct{})
	var wg sync.WaitGroup

	// 一个 CPU 密集的 goroutine 制造大量抢占点
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			x := uint64(0)
			for {
				select {
				case <-stop:
					return
				default:
				}
				for i := 0; i < 1000; i++ {
					x = x*6364136223846793005 + 1442695040888963407
				}
				runtime.Gosched()
			}
		}()
	}

	// 同时疯狂调用汇编, 让抢占随时可能落在调用点附近
	var errs int64
	deadline := time.Now().Add(400 * time.Millisecond)
	for time.Now().Before(deadline) {
		p := make([]byte, 1+rand.Intn(4000))
		rand.Read(p)
		key := rand.Uint32()

		want := append([]byte(nil), p...)
		maskFast(want, key)

		maskNEON(p, key)
		if !bytes.Equal(want, p) {
			atomic.AddInt64(&errs, 1)
		}
	}

	close(stop)
	wg.Wait()

	if errs != 0 {
		t.Fatalf("抢占干扰下 %d 次结果错误", errs)
	}
}

// ############################################################################
// 3. GC 扫描期间的指针有效性
//
// 汇编吃的是裸指针, 且不告诉 GC 它引用了什么。如果汇编在 GC
// 正在移动/扫描对象时持有了失效的指针, 结果就会错乱。
// 这里刻意制造高频率 GC + 大量小对象移动, 同时持续调用汇编。
// ############################################################################

func Test_AbiChaos_GCPointerValidity(t *testing.T) {
	stop := make(chan struct{})
	var allocWG sync.WaitGroup

	// 后台协程: 疯狂分配小对象, 制造频繁 GC 和对象移动
	allocWG.Add(1)
	go func() {
		defer allocWG.Done()
		var keep [][]byte
		for {
			select {
			case <-stop:
				return
			default:
			}
			for i := 0; i < 64; i++ {
				keep = append(keep, make([]byte, 64+rand.Intn(2048)))
			}
			if len(keep) > 256 {
				keep = keep[len(keep)/2:]
			}
		}
	}()

	var wg sync.WaitGroup
	var errs int64
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(int64(g) * 7919))
			for r := 0; r < 3000; r++ {
				n := rng.Intn(3000)
				p := make([]byte, n)
				rng.Read(p)
				key := rng.Uint32()

				want := append([]byte(nil), p...)
				maskFast(want, key)

				maskNEON(p, key)
				if !bytes.Equal(want, p) {
					atomic.AddInt64(&errs, 1)
					return
				}
			}
		}(g)
	}

	wg.Wait()
	close(stop)
	allocWG.Wait()

	if errs != 0 {
		t.Fatalf("GC 移动干扰下 %d 次结果错误", errs)
	}
	runtime.GC() // 收尾一次, 确保没有遗留问题
}

// ############################################################################
// 4. 通过 reflect 调用
//
// reflect.Call 会走 "ABIInternal -> ABI0" 的转换路径, 对栈和寄存器
// 的假定与直接调用不同。这是汇编函数容易暴露问题的调用方式。
// ############################################################################

func Test_AbiChaos_ViaReflect(t *testing.T) {
	fn := reflect.ValueOf(maskNEON)
	if fn.Kind() != reflect.Func {
		t.Fatal("maskNEON 不是函数")
	}

	rng := rand.New(rand.NewSource(0x2ef1))
	for i := 0; i < 5000; i++ {
		n := rng.Intn(3000)
		p := make([]byte, n)
		rng.Read(p)
		key := rng.Uint32()

		want := append([]byte(nil), p...)
		maskFast(want, key)

		fn.Call([]reflect.Value{reflect.ValueOf(p), reflect.ValueOf(key)})

		if !bytes.Equal(want, p) {
			t.Fatalf("reflect 调用结果错误: iter=%d n=%d key=%#x", i, n, key)
		}
	}

	// 覆盖边界长度
	for _, n := range []int{0, 1, 7, 8, 63, 64, 65, 191, 192, 193, 1024} {
		p := make([]byte, n)
		for j := range p {
			p[j] = byte(j + 1)
		}
		want := append([]byte(nil), p...)
		maskFast(want, 0xDEADBEEF)
		fn.Call([]reflect.Value{reflect.ValueOf(p), reflect.ValueOf(uint32(0xDEADBEEF))})
		if !bytes.Equal(want, p) {
			t.Fatalf("reflect 边界调用结果错误: n=%d", n)
		}
	}
}

// ############################################################################
// 5. 栈地址敏感性
//
// 切片底层数组在不同栈/堆位置时, 汇编的地址计算不应出偏差。
// 这里把 payload 放在: 栈上数组、堆上数组、另一个 goroutine 的栈上,
// 以及紧贴栈帧边界的数组上。
// ############################################################################

//go:noinline
func stackArrayPayload(n int, key uint32) []byte {
	var arr [4096]byte
	for i := range arr {
		arr[i] = byte(i*3 + 1)
	}
	p := arr[:n]
	maskNEON(p, key)
	out := make([]byte, n)
	copy(out, p)
	return out
}

func Test_AbiChaos_MemoryLocation(t *testing.T) {
	rng := rand.New(rand.NewSource(0x10c))

	for i := 0; i < 3000; i++ {
		n := rng.Intn(4000)
		key := rng.Uint32()

		// 期望结果
		ref := make([]byte, n)
		for j := range ref {
			ref[j] = byte(j*3 + 1)
		}
		kb := [4]byte{byte(key), byte(key >> 8), byte(key >> 16), byte(key >> 24)}
		for j := range ref {
			ref[j] ^= kb[j&3]
		}

		// 1) 栈上数组
		got := stackArrayPayload(n, key)
		if !bytes.Equal(ref[:n], got) {
			t.Fatalf("栈上数组结果错误: iter=%d n=%d", i, n)
		}

		// 2) 堆上数组
		heap := make([]byte, n)
		for j := range heap {
			heap[j] = byte(j*3 + 1)
		}
		maskNEON(heap, key)
		if !bytes.Equal(ref[:n], heap) {
			t.Fatalf("堆上数组结果错误: iter=%d n=%d", i, n)
		}

		// 3) 另一个 goroutine 的栈上
		ch := make(chan []byte, 1)
		go func(n int, key uint32) {
			ch <- stackArrayPayload(n, key)
		}(n, key)
		other := <-ch
		if !bytes.Equal(ref[:n], other) {
			t.Fatalf("其他 goroutine 栈上结果错误: iter=%d n=%d", i, n)
		}
	}
}

// ############################################################################
// 6. 函数被内联/包装后的行为
//
// 汇编函数不内联, 但调用它的 Go 包装可能被内联, 导致调用点
// 的栈布局变化。这里用几种不同的包装方式调用, 结果必须一致。
// ############################################################################

//go:noinline
func wrapperNoInline(p []byte, k uint32) { maskNEON(p, k) }

func wrapperPlain(p []byte, k uint32) { maskNEON(p, k) }

//go:noinline
func wrapperVariadic(ps ...[]byte) { maskNEON(ps[0], 0x12345678) }

func wrapperClosure() func([]byte, uint32) {
	return func(p []byte, k uint32) { maskNEON(p, k) }
}

func Test_AbiChaos_CallWrappers(t *testing.T) {
	closure := wrapperClosure()
	rng := rand.New(rand.NewSource(0x77a9))

	for i := 0; i < 5000; i++ {
		n := rng.Intn(2500)
		src := make([]byte, n)
		rng.Read(src)
		key := rng.Uint32()

		want := append([]byte(nil), src...)
		maskFast(want, key)

		ways := []struct {
			name string
			fn   func([]byte, uint32)
		}{
			{"plain", wrapperPlain},
			{"noinline", wrapperNoInline},
			{"closure", closure},
			{"method", (&wrapperType{}).do},
			{"iface", wrapperIface{}.do},
		}

		for _, w := range ways {
			got := append([]byte(nil), src...)
			w.fn(got, key)
			if !bytes.Equal(want, got) {
				t.Fatalf("包装方式 %s 结果错误: iter=%d n=%d key=%#x", w.name, i, n, key)
			}
		}

		// 变参包装
		got := append([]byte(nil), src...)
		wrapperVariadic(got)
		want2 := append([]byte(nil), src...)
		maskFast(want2, 0x12345678)
		if !bytes.Equal(want2, got) {
			t.Fatalf("变参包装结果错误: iter=%d n=%d", i, n)
		}
	}
}

type wrapperType struct{}

func (w *wrapperType) do(p []byte, k uint32) { maskNEON(p, k) }

type wrapperIface struct{}

func (wrapperIface) do(p []byte, k uint32) { maskNEON(p, k) }

// ############################################################################
// 8. 长时间稳定性
//
// 持续调用一段时间, 混合所有长度和入口, 观察是否有累积性失效
// (比如寄存器状态逐步劣化)。
// ############################################################################

func Test_AbiChaos_Soak(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过: short 模式")
	}

	var wg sync.WaitGroup
	var total int64
	var errs int64

	deadline := time.Now().Add(2 * time.Second)
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			r := rand.New(rand.NewSource(int64(g) * 104729))
			// 每个协程用**自己的** buffer。
			// 之前这里是所有协程共享一块 buf, 互相覆写导致大量假失败 ——
			// 那是测试自身的 bug, 与 maskNEON 无关。
			buf := make([]byte, 8192)
			local := int64(0)
			for time.Now().Before(deadline) {
				n := r.Intn(len(buf) + 1)
				key := r.Uint32()
				seg := buf[:n]
				r.Read(seg)
				before := append([]byte(nil), seg...)

				maskNEON(seg, key)

				kb := [4]byte{byte(key), byte(key >> 8), byte(key >> 16), byte(key >> 24)}
				for j := range seg {
					if seg[j] != before[j]^kb[j&3] {
						atomic.AddInt64(&errs, 1)
						break
					}
				}
				local++
			}
			atomic.AddInt64(&total, local)
		}(g)
	}
	wg.Wait()

	if errs != 0 {
		t.Fatalf("soak 期间 %d 次错误", errs)
	}
	t.Logf("soak 完成: %d 次调用无错误", total)
}
