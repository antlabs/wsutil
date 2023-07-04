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

//go:build !goexperiment.arenas

package bytespool

import (
	"testing"
	"unsafe"

	"github.com/antlabs/wsutil/enum"
)

func Test_Index(t *testing.T) {
	for i := 0; i <= 1024+enum.MaxFrameHeaderSize; i++ {
		i2 := i
		if i2 >= enum.MaxFrameHeaderSize {
			i2 -= (enum.MaxFrameHeaderSize + 1)
		}
		index := selectIndex(i2)
		if index != 0 {
			t.Fatal("index error")
		}

	}

	for i := 1024 + enum.MaxFrameHeaderSize + 1; i <= 2*1024+enum.MaxFrameHeaderSize; i++ {
		i2 := i
		i2 -= (enum.MaxFrameHeaderSize + 1)
		index := selectIndex(i2)
		if index != 1 {
			t.Fatal("index error")
		}
	}

	for i := 1024*2 + enum.MaxFrameHeaderSize + 1; i <= 3*1024+enum.MaxFrameHeaderSize; i++ {
		i2 := i
		i2 -= (enum.MaxFrameHeaderSize + 1)
		index := selectIndex(i2)
		if index != 2 {
			t.Fatal("index error")
		}
	}
}

func Test_GetBytes_Address(t *testing.T) {
	var m map[unsafe.Pointer]bool
	for i := 0; i < 10; i++ {
		p := GetBytes(1)
		if m[unsafe.Pointer(p)] {
			t.Fatal("duplicate pointer")
		}
	}
}
