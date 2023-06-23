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

import (
	"bytes"
	"testing"
)

// 正确性测试
func Test_Mask(t *testing.T) {
	key := uint32(0x12345678)
	for i := 0; i < 1000*4; i++ {
		payload := make([]byte, i)
		for j := 0; j < len(payload); j++ {
			payload[j] = byte(j)
		}

		pay1 := append([]byte(nil), payload...)
		pay2 := append([]byte(nil), payload...)

		maskFast(pay1, key)
		maskSlow(pay2, key)

		if !bytes.Equal(pay1, pay2) {
			t.Fatalf("i = %d, fast.payload != slow.payload:%v, %v", i, pay1, pay2)
		}
	}
}

// TODO 边界测试
func Test_Mask_Boundary(t *testing.T) {
}
