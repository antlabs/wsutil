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

//go:build goexperiment.arenas

package bytespool

import (
	"arena"
)

type BytesPool struct {
	mem   *arena.Arena
	isSet bool
}

func New() *BytesPool {
	return &BytesPool{
		mem: arena.NewArena(),
	}
}

func (p *BytesPool) Init() {
	p.isSet = true
}

func (p *BytesPool) GetBytes(n int) (rv *[]byte) {
	bs := arena.MakeSlice[byte](p.mem, n, n)
	return &bs
}

func (p *BytesPool) PutBytes(bytes *[]byte) {
}

func (b *BytesPool) Free() {
	b.mem.Free()
	return
}
