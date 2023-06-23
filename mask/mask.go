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

import "unsafe"

var mask func(payload []byte, key uint32)

func init() {
	i := uint32(1)
	b := *(*bool)(unsafe.Pointer(&i))

	if b {
		// 小端机器
		mask = maskFast
	} else {
		// 大端机器
		mask = maskSlow
	}
}
