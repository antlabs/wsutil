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
	"reflect"
	"unsafe"
)

//go:nosplit
func maskFast(payload []byte, key uint32) {
	if len(payload) == 0 {
		return
	}

	base := (*reflect.SliceHeader)(unsafe.Pointer(&payload)).Data
	last := base + uintptr(len(payload))
	if len(payload) >= 8 {
		key64 := uint64(key)<<32 | uint64(key)

		var v *uint64
		for base+128 <= last {

			v = (*uint64)(unsafe.Pointer(base))
			*v ^= key64
			base += 8
			v = (*uint64)(unsafe.Pointer(base))
			*v ^= key64
			base += 8
			v = (*uint64)(unsafe.Pointer(base))
			*v ^= key64
			base += 8
			v = (*uint64)(unsafe.Pointer(base))
			*v ^= key64
			base += 8
			v = (*uint64)(unsafe.Pointer(base))
			*v ^= key64
			base += 8
			v = (*uint64)(unsafe.Pointer(base))
			*v ^= key64
			base += 8
			v = (*uint64)(unsafe.Pointer(base))
			*v ^= key64
			base += 8
			v = (*uint64)(unsafe.Pointer(base))
			*v ^= key64
			base += 8
			v = (*uint64)(unsafe.Pointer(base))
			*v ^= key64
			base += 8
			v = (*uint64)(unsafe.Pointer(base))
			*v ^= key64
			base += 8
			v = (*uint64)(unsafe.Pointer(base))
			*v ^= key64
			base += 8
			v = (*uint64)(unsafe.Pointer(base))
			*v ^= key64
			base += 8
			v = (*uint64)(unsafe.Pointer(base))
			*v ^= key64
			base += 8
			v = (*uint64)(unsafe.Pointer(base))
			*v ^= key64
			base += 8
			v = (*uint64)(unsafe.Pointer(base))
			*v ^= key64
			base += 8
			v = (*uint64)(unsafe.Pointer(base))
			*v ^= key64
			base += 8
		}

		if base == last {
			return
		}

		if base+64 <= last {

			v = (*uint64)(unsafe.Pointer(base))
			*v ^= key64
			base += 8
			v = (*uint64)(unsafe.Pointer(base))
			*v ^= key64
			base += 8
			v = (*uint64)(unsafe.Pointer(base))
			*v ^= key64
			base += 8
			v = (*uint64)(unsafe.Pointer(base))
			*v ^= key64
			base += 8
			v = (*uint64)(unsafe.Pointer(base))
			*v ^= key64
			base += 8
			v = (*uint64)(unsafe.Pointer(base))
			*v ^= key64
			base += 8
			v = (*uint64)(unsafe.Pointer(base))
			*v ^= key64
			base += 8
			v = (*uint64)(unsafe.Pointer(base))
			*v ^= key64
			base += 8
		}

		if base+32 <= last {
			v = (*uint64)(unsafe.Pointer(base))
			*v ^= key64
			base += 8
			v = (*uint64)(unsafe.Pointer(base))
			*v ^= key64
			base += 8
			v = (*uint64)(unsafe.Pointer(base))
			*v ^= key64
			base += 8
			v = (*uint64)(unsafe.Pointer(base))
			*v ^= key64
			base += 8
		}

		if base+16 <= last {
			v = (*uint64)(unsafe.Pointer(base))
			*v ^= key64
			base += 8
			v = (*uint64)(unsafe.Pointer(base))
			*v ^= key64
			base += 8
		}

		if base+8 <= last {
			v = (*uint64)(unsafe.Pointer(base))
			*v ^= key64
			base += 8
		}

		if base == last {
			return
		}
	}

	if base+4 <= last {
		v := (*uint32)(unsafe.Pointer(base))
		*v ^= key
		base += 4
	}

	if base == last {
		return
	}
	if base < last {
		maskSlow(payload[len(payload)-int(last-base):], key)
	}
}
