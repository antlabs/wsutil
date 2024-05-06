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
package fixedreader

import (
	"errors"
	"io"
)

var errNegativeRead = errors.New("fixedreader: reader returned negative count from Read")

// 固定大小的FixedReader, 所有的内存都是提前分配好的
// 标准库的bufio.Reader不能自定义buf传过去, 导致控制力度会差点
type FixedReader struct {
	buf    *[]byte
	rd     io.Reader // reader provided by the client
	R, W   int       // buf read and write positions
	err    error
	isInit bool
}

func (b *FixedReader) Init(r io.Reader, buf *[]byte) {
	b.rd = r
	b.buf = buf
	b.isInit = true
}

func (b *FixedReader) IsInit() bool {
	return b.isInit
}

// newBuffer returns a new Buffer whose buffer has the specified size.
func NewFixedReader(r io.Reader, buf *[]byte) *FixedReader {
	fr := &FixedReader{}
	fr.Init(r, buf)
	return fr
}

func (b *FixedReader) Release() error {
	if b.buf != nil {
		b.buf = nil
	}
	return nil
}

func (b *FixedReader) readErr() error {
	err := b.err
	b.err = nil
	return err
}

// 将缓存区重置为一个新的buf
func (b *FixedReader) Reset(buf *[]byte) {
	if len(*buf) < len((*b.buf)[b.R:b.W]) {
		panic("new buf size is too small")
	}

	copy(*buf, (*b.buf)[b.R:b.W])
	b.W -= b.R
	b.R = 0
	b.buf = buf
}

// 返回底层[]byte的长度
func (b *FixedReader) Len() int {
	return len(*b.buf)
}

func (b *FixedReader) BufPtr() *[]byte {
	return b.buf
}

func (b *FixedReader) Bytes() []byte {
	return *b.buf
}

// 返回剩余可写的缓存区大小
func (b *FixedReader) WriteCap() int {
	return len((*b.buf)[b.W:])
}

// 返回剩余可用的缓存区大小
func (b *FixedReader) Available() int64 {
	return int64(len((*b.buf)[b.W:]) + b.R)
}

// 左移缓存区
func (b *FixedReader) LeftMove() {
	if b.R == 0 {
		return
	}
	// b.CountMove++
	// b.MoveBytes += b.W - b.R
	copy(*b.buf, (*b.buf)[b.R:b.W])
	b.W -= b.R
	b.R = 0
}

// 返回可写的缓存区
func (b *FixedReader) WriteCapBytes() []byte {
	return (*b.buf)[b.W:]
}

func (b *FixedReader) CloneAvailable() *FixedReader {
	buf := (*b.buf)[b.W:]
	return &FixedReader{rd: b.rd, buf: &buf}
}

func (b *FixedReader) Buffered() int { return b.W - b.R }

// 这和一般read接口中不一样
// 传入的p 一定会满足这个大小
func (b *FixedReader) Read(p []byte) (n int, err error) {
	if cap(*b.buf) < cap(p) {
		panic("fixedReader.Reader buf size is too small: cap(b.buf) < cap(p)")
	}

	n = len(p)
	if n == 0 {
		if b.Buffered() > 0 {
			return 0, nil
		}
		return 0, b.readErr()
	}

	var n1 int
	for {

		if b.R == b.W || len((*b.buf)[b.R:b.W]) < len(p) {
			if b.err != nil {
				return 0, b.readErr()
			}
			if b.R == b.W {
				b.R = 0
				b.W = 0
			}
			n1, b.err = b.rd.Read((*b.buf)[b.W:])
			if n1 < 0 {
				panic(errNegativeRead)
			}
			if n1 == 0 {
				return 0, b.readErr()
			}
			b.W += n1
			continue
		}

		n1 = copy(p, (*b.buf)[b.R:b.W])
		b.R += n1

		return n, nil
	}
}

func (b *FixedReader) ReadN(n int) (rvn int, err error) {
	if cap(*b.buf) < n {
		panic("fixedReader.Reader buf size is too small: cap(b.buf) < n")
	}

	if n == 0 {
		if b.Buffered() > 0 {
			return 0, nil
		}
		return 0, b.readErr()
	}

	var n1 int
	for {

		if b.R == b.W || len((*b.buf)[b.R:b.W]) < n {
			if b.err != nil {
				return 0, b.readErr()
			}
			if b.R == b.W {
				b.R = 0
				b.W = 0
			}
			n1, b.err = b.rd.Read((*b.buf)[b.W:])
			if n1 < 0 {
				panic(errNegativeRead)
			}
			if n1 == 0 {
				return 0, b.readErr()
			}
			b.W += n1
			continue
		}

		b.R += n1

		return n, nil
	}
}

func (b *FixedReader) ResetReader(r io.Reader) {
	b.rd = r
}
