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
package frame

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/antlabs/wsutil/bytespool"
	"github.com/antlabs/wsutil/enum"
	"github.com/antlabs/wsutil/fixedreader"
	"github.com/antlabs/wsutil/fixedwriter"
	"github.com/antlabs/wsutil/opcode"
)

var (
	testBinaryMessage64kb = bytes.Repeat([]byte("1"), 65535)
	testTextMessage64kb   = bytes.Repeat([]byte("中"), 65535/len("中"))
	testBinaryMessage10   = bytes.Repeat([]byte("1"), 10)
)

func Test_Windows_Read(t *testing.T) {
	t.Run("Reader-Small", func(t *testing.T) {
		var out bytes.Buffer

		tmp := append([]byte(nil), testTextMessage64kb...)
		var fw fixedwriter.FixedWriter
		err := writeMessageInner(&out, opcode.Text, tmp, true, &fw)
		// err := writeMessage(&out, opcode.Text, tmp, true, &head, &fw)
		// hexString := hex.EncodeToString(out.Bytes())
		// // 在每两个字符之间插入空格
		// spacedHexString := strings.Join(splitString(hexString, 2), ", ")
		// fmt.Printf("header: %+v\n", spacedHexString[:100])
		if err != nil {
			t.Fatal(err)
		}

		r := fixedreader.NewFixedReader(&out, bytespool.GetBytes(1024+enum.MaxFrameHeaderSize))

		var headArray [14]byte
		f, err := ReadFrame(r, &headArray)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(f.Payload, testTextMessage64kb) {
			t.Fatalf("payload:%s", string(f.Payload))
		}
	})

	t.Run("WriteMulti_ReadOne", func(t *testing.T) {
		var out bytes.Buffer

		// var head [14]byte
		var fw fixedwriter.FixedWriter
		for i := 1024 * 63; i <= 1024*63+1; i++ {
			need := make([]byte, 0, i)
			got := make([]byte, 0, i)
			for j := 0; j < i; j++ {
				need = append(need, byte(j))
				got = append(got, byte(j))
			}

			for j := 0; j < 1; j++ {
				err := writeMessageInner(&out, opcode.Text, need, true, &fw)
				// err := writeMessage(&out, opcode.Text, need, true, &head, &fw)
				if err != nil {
					t.Fatal(err)
				}
				err = writeMessageInner(&out, opcode.Text, need, true, &fw)
				if err != nil {
					t.Fatal(err)
				}
			}
			fmt.Printf("i = %d, need: len(%d), write.size:%d\n", i, len(need), out.Len())

			b := bytespool.GetBytes(1024 + enum.MaxFrameHeaderSize)

			r := fixedreader.NewFixedReader(&out, b)
			var headArray [14]byte
			for j := 0; j < 2; j++ {

				f, err := ReadFrame(r, &headArray)
				if err != nil {
					t.Fatal(err)
					return
				}

				// TODO
				if j == 0 {
					continue
				}
				if !bytes.Equal(f.Payload, got) {
					t.Fatalf("bad test index:%d\n", i)
					return
				}
				// assert.Equal(t, f.payload, got, fmt.Sprintf("index:%d", i))
				if err != nil {
					return
				}
			}
			bytespool.PutBytes(r.BufPtr())
			out.Reset()
		}
	})

	t.Run("WriteOne_ReadMulti", func(t *testing.T) {
		var out bytes.Buffer

		var headArray [14]byte
		var fw fixedwriter.FixedWriter
		b := bytespool.GetBytes(1024 + enum.MaxFrameHeaderSize)
		r := fixedreader.NewFixedReader(&out, b)
		for i := 1031; i <= 1024*64; i++ {
			// for i := 2046; i <= 2048; i++ {
			need := make([]byte, 0, i)
			got := make([]byte, 0, i)
			for j := 0; j < i; j++ {
				need = append(need, byte(j))
				got = append(got, byte(j))
			}

			err := writeMessageInner(&out, opcode.Text, need, true, &fw)
			if err != nil {
				t.Fatal(err)
			}

			f, err := ReadFrame(r, &headArray)
			if err != nil {
				t.Fatal(err)
			}

			// TODO
			if i == 0 {
				continue
			}
			if !bytes.Equal(f.Payload, got) {
				t.Fatalf("bad test index:%d\n", i)
				return
			}

			if len(f.Payload) == 0 {
				t.Fatalf("bad test index:%d\n", i)
				return
			}

			bytespool.PutBytes(r.BufPtr())
			out.Reset()
		}
	})

	t.Run("Test_Reset", func(t *testing.T) {
		var out bytes.Buffer
		out.Write([]byte("1234"))

		r := fixedreader.NewFixedReader(&out, bytespool.GetBytes(1024+enum.MaxFrameHeaderSize))

		small := make([]byte, 2)

		r.Read(small)
		r.Reset(bytespool.GetBytes(1024*2 + enum.MaxFrameHeaderSize))
		if !bytes.Equal(r.Bytes()[:2], []byte("34")) {
			t.Fatal("bad")
		}
	})

	t.Run("WriteMulti_ReadOne_64512", func(t *testing.T) {
		var out bytes.Buffer

		var fw fixedwriter.FixedWriter
		for i := 64512; i <= 64512; i++ {
			need := make([]byte, 0, i)
			got := make([]byte, 0, i)
			for j := 0; j < i; j++ {
				need = append(need, byte(j))
				got = append(got, byte(j))
			}

			for j := 0; j < 1; j++ {
				err := writeMessageInner(&out, opcode.Text, need, true, &fw)
				if err != nil {
					t.Fatalf("bad test index:%d\n", i)
					return
				}
				err = writeMessageInner(&out, opcode.Text, need, true, &fw)
				if err != nil {
					t.Fatalf("bad test index:%d\n", i)
					return
				}
			}
			fmt.Printf("i = %d, need: len(%d), write.size:%d\n", i, len(need), out.Len())

			b := bytespool.GetBytes(1024 + enum.MaxFrameHeaderSize)
			r := fixedreader.NewFixedReader(&out, b)
			var headArray [14]byte
			for j := 0; j < 2; j++ {

				f, err := ReadFrame(r, &headArray)
				if err != nil {
					t.Fatalf("bad test index:%d\n", i)
					return
				}

				// TODO
				if j == 0 {
					continue
				}
				if !bytes.Equal(f.Payload, got) {
					t.Fatalf("bad test index:%d\n", i)
					return
				}
				// assert.Equal(t, f.payload, got, fmt.Sprintf("index:%d", i))
				if err != nil {
					return
				}
			}
			bytespool.PutBytes(r.BufPtr())
			out.Reset()
		}
	})

	t.Run("WriteMulti_ReadOne_65536", func(t *testing.T) {
		var out bytes.Buffer

		var headArray [14]byte
		// var writeHeader [14]byte
		var fw fixedwriter.FixedWriter
		for i := 65536; i <= 64512; i++ {
			need := make([]byte, 0, i)
			got := make([]byte, 0, i)
			for j := 0; j < i; j++ {
				need = append(need, byte(j))
				got = append(got, byte(j))
			}

			for j := 0; j < 1; j++ {
				err := writeMessageInner(&out, opcode.Text, need, true, &fw)
				if err != nil {
					t.Fatalf("bad test index:%d\n", i)
					return
				}
				err = writeMessageInner(&out, opcode.Text, need, true, &fw)
				if err != nil {
					t.Fatalf("bad test index:%d\n", i)
					return
				}
			}
			fmt.Printf("i = %d, need: len(%d), write.size:%d\n", i, len(need), out.Len())

			b := bytespool.GetBytes(1024 + enum.MaxFrameHeaderSize)
			r := fixedreader.NewFixedReader(&out, b)
			for j := 0; j < 2; j++ {

				f, err := ReadFrame(r, &headArray)
				if err != io.EOF {
					if err != nil {
						t.Fatalf("bad test index:%d\n", i)
						return
					}
				}

				if !bytes.Equal(f.Payload, got) {
					t.Fatalf("bad test index:%d\n", i)
					return
				}
				// assert.Equal(t, f.payload, got, fmt.Sprintf("index:%d", i))
				if err != nil {
					return
				}
			}
			bytespool.PutBytes(r.BufPtr())
			out.Reset()
		}
	})
}
