// Copyright 2021-2023 antlabs. All rights reserved.
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

package bytespool

import (
	"sync"

	"github.com/antlabs/wsutil/enum"
)

const (
	page     = 1024
	maxIndex = 64
)

func selectIndex(n int) int {
	index := n / page
	return index
}

var pools = make([]sync.Pool, 0, maxIndex)

var upgradeRespPool = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, 256)
		return &buf
	},
}

func init() {
	for i := 1; i <= maxIndex; i++ {
		j := i
		pools = append(pools, sync.Pool{
			New: func() interface{} {
				buf := make([]byte, j*page+enum.MaxFrameHeaderSize)
				return &buf
			},
		})
	}
}

func GetBytes(n int) (rv *[]byte) {
	if n <= enum.MaxFrameHeaderSize {
		return pools[0].Get().(*[]byte)
	}

	index := selectIndex(n - enum.MaxFrameHeaderSize - 1)
	if index >= len(pools) {
		rv := make([]byte, n+enum.MaxFrameHeaderSize)
		return &rv
	}

	return pools[index].Get().(*[]byte)
}

func PutBytes(bytes *[]byte) {
	if cap(*bytes) < enum.MaxFrameHeaderSize {
		panic("putBytes: bytes is too small")
	}
	newLen := cap(*bytes) - enum.MaxFrameHeaderSize - 1
	index := selectIndex(newLen)
	if index >= len(pools) {
		return
	}
	pools[index].Put(bytes)
}

func GetUpgradeRespBytes() *[]byte {
	return upgradeRespPool.Get().(*[]byte)
}

func PutUpgradeRespBytes(bytes *[]byte) {
	upgradeRespPool.Put(bytes)
}
