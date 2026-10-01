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
	"encoding/binary"
	"math/rand"
	"testing"
)

// Benchmark_AMD64_vs_Default 在同一进程内对比三条路径:
//   - default: maskFast(展开的标量实现, 线上历史行为)
//   - sse2:    maskSSE2(128bit, amd64 基线)
//   - avx2:    maskAVX2(256bit, 需要 CPU 支持)
//
// 用 benchstat 比较最稳妥:
//
//	go test -bench Benchmark_AMD64_vs_Default -count=10 ./mask/ > new.txt
func Benchmark_AMD64_vs_Default(b *testing.B) {
	sizes := []int{16, 32, 48, 64, 96, 128, 192, 256, 512, 1024, 4096, 16384, 65536}

	hasAVX2 := detectAVX2()

	for _, sz := range sizes {
		payload := make([]byte, sz)
		rand.New(rand.NewSource(int64(sz))).Read(payload)
		key := binary.LittleEndian.Uint32(payload[:4])
		nm := benchNameAMD64(sz)

		b.Run(nm+"/default", func(b *testing.B) {
			b.SetBytes(int64(sz))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				maskFast(payload, key)
			}
		})

		b.Run(nm+"/sse2", func(b *testing.B) {
			b.SetBytes(int64(sz))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				maskSSE2(payload, key)
			}
		})

		if hasAVX2 {
			b.Run(nm+"/avx2", func(b *testing.B) {
				b.SetBytes(int64(sz))
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					maskAVX2(payload, key)
				}
			})
		}
	}
}

// Benchmark_AMD64_Dispatch 测分发层本身的开销(含阈值判断 + 调用)。
// 这是 quickws 线上实际走的路径。
func Benchmark_AMD64_Dispatch(b *testing.B) {
	sizes := []int{16, 64, 128, 192, 256, 1024, 4096, 16384, 65536}

	for _, sz := range sizes {
		payload := make([]byte, sz)
		rand.New(rand.NewSource(int64(sz))).Read(payload)
		key := binary.LittleEndian.Uint32(payload[:4])

		b.Run(benchNameAMD64(sz), func(b *testing.B) {
			b.SetBytes(int64(sz))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				Mask(payload, key)
			}
		})
	}
}

func benchNameAMD64(n int) string {
	switch n {
	case 16:
		return "00016B"
	case 32:
		return "00032B"
	case 48:
		return "00048B"
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
