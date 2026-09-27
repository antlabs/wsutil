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

// 用 NEON(Advanced SIMD) 加速 mask 的 opt-in 实现。
//
// 启用方式:
//
//	go build -tags wsutil_neon ./...
//	go test -tags wsutil_neon ./...
//
// 这个构建约束有意限制得比较窄(darwin + arm64 + 显式 tag):
//   - 只在 macOS/Apple Silicon 上验证过, linux/arm64 理论上同样成立
//     但未做过充分测试, 所以没有放开
//   - 作为 opt-in 而不是默认路径, 避免影响其他平台和现有用户
//   - 未启用 tag 时, 行为与之前完全一致(见 antlabs_mask_init_generic.go)
//
// 实测(Apple M2 Max, darwin/arm64, 对比 maskFast):
//
//	 64B: 慢  7%   (VDUP 等 setup 摊不开)
//	128B: 持平
//	192B: 快  8%   <- 交叉点, 即 neonThreshold
//	256B: 快 23%
//	  1KB: 快 28%
//	  4KB: 快 40%
//	 64KB: 快 42%
//
// 阈值取自实测交叉点: 低于它的长度走 maskFast(展开的标量实现)更快。
const neonThreshold = 192

func init() {
	Mask = maskDispatch
}

func maskDispatch(payload []byte, key uint32) {
	if len(payload) >= neonThreshold {
		maskNEON(payload, key)
		return
	}
	maskFast(payload, key)
}

//go:noescape
func maskNEON(payload []byte, key uint32)
