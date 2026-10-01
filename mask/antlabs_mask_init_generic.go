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

//go:build !(darwin && arm64 && wsutil_neon) && !(amd64 && !wsutil_nosimd)

package mask

import "unsafe"

// 默认实现: 与历史版本完全一致。
// NEON 版本启用时, init 由 antlabs_mask_neon_arm64.go 提供;
// amd64 上默认由 antlabs_mask_amd64.go 提供, 用 -tags wsutil_nosimd 关掉。
func init() {
	i := uint32(1)
	b := *(*bool)(unsafe.Pointer(&i))

	if b {
		// 小端机器
		Mask = maskFast
	} else {
		// 大端机器
		Mask = maskSlow
	}
}
