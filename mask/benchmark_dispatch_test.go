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

package mask

import (
	"encoding/binary"
	"math/rand"
	"testing"
)

// Benchmark_MaskDispatch 测 Mask 这个分发入口本身的性能。
//
// 这个文件故意不打构建约束: 同一份基准可以在两种配置下跑出可对比的两组数据 ——
//
//	# 优化后(amd64 默认, Mask 经 SIMD 分发)
//	go test -bench Benchmark_MaskDispatch -count=10 ./mask/ > new.txt
//
//	# 优化前(关掉 SIMD, Mask 直接指向 maskFast)
//	go test -tags wsutil_nosimd -bench Benchmark_MaskDispatch -count=10 ./mask/ > old.txt
//
//	benchstat old.txt new.txt
//
// 这是对调用方最有意义的数字: quickws 调用的就是 Mask, 不是每个内核。
func Benchmark_MaskDispatch(b *testing.B) {
	sizes := []int{16, 32, 64, 96, 128, 192, 256, 512, 1024, 4096, 16384, 65536}

	for _, sz := range sizes {
		payload := make([]byte, sz)
		rand.New(rand.NewSource(int64(sz))).Read(payload)
		key := binary.LittleEndian.Uint32(payload[:4])

		b.Run(benchSizeNameMask(sz), func(b *testing.B) {
			b.SetBytes(int64(sz))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				Mask(payload, key)
			}
		})
	}
}

// Benchmark_MaskFastDirect 直接调用 maskFast(绕过分发入口)。
//
// 用途: 隔离实验。小尺寸在启用 wsutil_amd64 后变慢, 有两个候选原因:
//  1. 分发层多了一次函数调用 —— 那么这份"直连 maskFast"的基准在
//     两种配置下应该测出相同的数
//  2. 同进程内跑 AVX2 大尺寸导致 CPU 降频/状态切换, 连累小尺寸 ——
//     那么这份基准在启用 tag 后也会变慢
//
// 两种配置各跑一次, 对比结果即可区分。
func Benchmark_MaskFastDirect(b *testing.B) {
	sizes := []int{16, 64, 128, 256}

	for _, sz := range sizes {
		payload := make([]byte, sz)
		rand.New(rand.NewSource(int64(sz))).Read(payload)
		key := binary.LittleEndian.Uint32(payload[:4])

		b.Run(benchSizeNameMask(sz), func(b *testing.B) {
			fn := maskFast
			b.SetBytes(int64(sz))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				fn(payload, key)
			}
		})
	}
}

func benchSizeNameMask(n int) string {
	switch n {
	case 16:
		return "00016B"
	case 32:
		return "00032B"
	case 64:
		return "00064B"
	case 96:
		return "00096B"
	case 128:
		return "00128B"
	case 192:
		return "00192B"
	case 256:
		return "00256B"
	case 512:
		return "00512B"
	case 1024:
		return "01KB"
	case 4096:
		return "04KB"
	case 16384:
		return "16KB"
	default:
		return "64KB"
	}
}
