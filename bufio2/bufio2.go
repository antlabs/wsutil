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

//go:build go1.20

package bufio2

import (
	"bufio"
	"io"
	"unsafe"
)

type Reader2 struct {
	buf          []byte
	rd           io.Reader // reader provided by the client
	r, w         int       // buf read and write positions
	err          error
	lastByte     int // last byte read for UnreadByte; -1 means invalid
	lastRuneSize int // size of last rune read for UnreadRune; -1 means invalid
}

//go:nosplit
func ClearReader(r *bufio.Reader) {
	r2 := (*Reader2)(unsafe.Pointer(r))
	r2.buf = nil
	r2.rd = nil
	r2.err = nil
}

type Writer2 struct {
	err error
	buf []byte
	n   int
	wr  io.Writer
}

//go:nosplit
func ClearWriter(w *bufio.Writer) {
	w2 := (*Writer2)(unsafe.Pointer(w))
	w2.err = nil
	w2.buf = nil
	w2.wr = nil
}

//go:nosplit
func ClearReadWriter(rw *bufio.ReadWriter) {
	ClearReader(rw.Reader)
	ClearWriter(rw.Writer)
	rw.Reader = nil
	rw.Writer = nil
}
