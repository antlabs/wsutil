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
	"testing"
)

type testFixedReaderFail struct{}

func (t *testFixedReaderFail) Read(p []byte) (n int, err error) {
	return 0, errors.New("fail")
}

type testFixedReaderEOF struct{}

func (t *testFixedReaderEOF) Read(p []byte) (n int, err error) {
	return 0, io.EOF
}

// 读取到一半， 返回错误
type testFixedReaderHalfFail struct {
	count int
}

func (t *testFixedReaderHalfFail) Read(p []byte) (n int, err error) {
	if t.count == 0 {
		t.count++
		return len(p) / 2, nil
	}
	return 0, errors.New("fail")
}

func Test_FixedReader(t *testing.T) {
	t.Run("fail", func(t *testing.T) {
		var r testFixedReaderFail
		buf := make([]byte, 1024)
		rr := NewFixedReader(&r, &buf)
		_, err := rr.Read(buf)
		if err == nil {
			t.Errorf("expect error, but got nil")
		}
	})

	t.Run("half fail", func(t *testing.T) {
		var r testFixedReaderHalfFail
		buf := make([]byte, 1024)
		rr := NewFixedReader(&r, &buf)
		_, err := rr.Read(buf)
		if err == nil {
			t.Errorf("expect error, but got nil")
		}
	})

	t.Run("EOF", func(t *testing.T) {
		var r testFixedReaderEOF
		buf := make([]byte, 1024)
		rr := NewFixedReader(&r, &buf)
		_, err := rr.Read(buf)
		if err == nil {
			t.Errorf("expect error, but got nil")
		}
	})
}
