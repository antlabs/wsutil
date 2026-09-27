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

// 这些基准只在启用 wsutil_neon 时才有意义(编译器会把 default 分支优化掉)。
// 想看优化效果:
//
//	go test -tags wsutil_neon -bench Benchmark_NEON ./mask/ -benchmem
package mask

import "testing"

// Benchmark_NEON_vs_Default 在同一进程内对比两条路径。
//
// Mask 在启用 tag 时指向 NEON 分发, maskFast 是历史实现,
// 两者交错采样, 用 benchstat 比较最稳妥:
//
//	go test -tags wsutil_neon -bench Benchmark_NEON_vs_Default -count=10 ./mask/ > new.txt
func Benchmark_NEON_vs_Default(b *testing.B) {
	sizes := []int{64, 128, 192, 256, 512, 1024, 4096, 16384, 65536}
	for _, sz := range sizes {
		payload := make([]byte, sz)
		nm := benchName(sz)

		b.Run(nm+"/default", func(b *testing.B) {
			fn := maskFast
			b.SetBytes(int64(sz))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				fn(payload, 0x12345678)
			}
		})

		b.Run(nm+"/neon", func(b *testing.B) {
			fn := Mask
			b.SetBytes(int64(sz))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				fn(payload, 0x12345678)
			}
		})
	}
}

func benchName(n int) string {
	switch n {
	case 64:
		return "00064B"
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
