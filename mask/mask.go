// Copyright 2021-2023 antlabs. All rights reserved.
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

// Mask 由各构建变体的 init 函数赋值:
//   - 默认(非 amd64, 或 amd64 带 wsutil_nosimd): antlabs_mask_init_generic.go
//     (maskFast / maskSlow 按端序选择)
//   - amd64(默认启用): antlabs_mask_amd64.go (AVX2/SSE2 按尺寸分发, 小尺寸内联标量)
//   - darwin/arm64 + -tags wsutil_neon: antlabs_mask_neon_arm64.go
//     (NEON, 小尺寸回退 maskFast)
var Mask func(payload []byte, key uint32)
