// Copyright 2021-2024 antlabs. All rights reserved.
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
package fixedwriter

import "fmt"

type FixedWriter struct {
	buf []byte
	w   int
}

func NewFixedWriter(buf []byte) *FixedWriter {
	return &FixedWriter{
		buf: buf,
	}
}

func (fw *FixedWriter) SetW(w int) {
	fw.w = w
}

func (fw *FixedWriter) Reset(buf []byte) {
	fw.buf = buf
	fw.w = 0
}

func (fw *FixedWriter) Write(p []byte) (n int, err error) {
	if len(fw.buf[fw.w:]) < len(p) {
		panic(fmt.Sprintf("fixedWriter: buf is too small: %d:%d < %d", len(fw.buf[fw.w:]), cap(fw.buf), cap(p)))
	}
	n = copy(fw.buf[fw.w:], p)
	fw.w += n
	return n, nil
}

func (fw *FixedWriter) Len() int {
	return fw.w
}

func (fw *FixedWriter) Bytes() []byte {
	return fw.buf[:fw.w]
}

// 释放
func (fw *FixedWriter) Free() {
	fw.buf = nil
	fw.w = 0
}
