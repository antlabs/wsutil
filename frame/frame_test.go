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
package frame

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/antlabs/wsutil/enum"
	"github.com/antlabs/wsutil/fixedreader"
	"github.com/antlabs/wsutil/fixedwriter"
	"github.com/antlabs/wsutil/opcode"
)

var (
	noMaskData   = []byte{0x81, 0x05, 0x48, 0x65, 0x6c, 0x6c, 0x6f}
	haveMaskData = []byte{0x81, 0x85, 0x37, 0xfa, 0x21, 0x3d, 0x7f, 0x9f, 0x4d, 0x51, 0x58}
)

func Test_Frame_Read(t *testing.T) {
	t.Run("Read.Size", func(t *testing.T) {
		var out bytes.Buffer
		var fw fixedwriter.FixedWriter
		err := writeMessageInner(&out, opcode.Text, nil, true, &fw)
		if err != nil {
			t.Fatal(err)
		}
		var headArray [14]byte
		outLen := out.Len()
		_, size, err := ReadHeader(&out, &headArray)
		if err != nil {
			t.Fatal(err)
		}
		if size != outLen {
			t.Fatalf("size:%d, outLen:%d", size, outLen)
		}
		fmt.Printf("%d:%d\n", size, outLen)
	})

	t.Run("Read.Data.NoMask", func(t *testing.T) {
		r := bytes.NewReader(noMaskData)

		var headArray [14]byte
		h, _, err := ReadHeader(r, &headArray)
		if err != nil {
			t.Fatal(err)
		}
		all, err := io.ReadAll(r)
		if err != nil {
			t.Fatal(err)
		}

		// fmt.Printf("opcode:%d", h.opcode)
		if string(all) != "Hello" {
			t.Fatalf("payload:%s", string(all))
		}

		if h.PayloadLen != int64(len("Hello")) {
			t.Fatalf("payloadLen:%d", h.PayloadLen)
		}
	})

	t.Run("ReadBinaryData", func(t *testing.T) {
		// 阈值
		threshold := 1.0

		buf := make([]byte, int((1024+enum.MaxFrameHeaderSize)*threshold))

		all, err := os.ReadFile("./testdata/binary_1024.dat")
		if err != nil {
			t.Fatal(err)
		}

		rb := bytes.NewReader(all)
		r := fixedreader.NewFixedReader(rb, &buf)

		headArray := [enum.MaxFrameHeaderSize]byte{}

		defer func() {
			// fmt.Printf("## move:%d\n", r.CountMove)
		}()

		for i := 0; i < 3; i++ {
			f, err := ReadFrame(r, &headArray)
			if err != nil {
				t.Fatal(err)
			}

			if len(f.Payload) == 0 {
				fmt.Printf("%#v, r.r = %d, r.w  %d\n", f, r.R, r.W)
				t.Fatal("payload is empty")
			}

			if len(f.Payload) != 1024 {
				fmt.Printf("%#v, r.r = %d, r.w  %d\n", f, r.R, r.W)
				t.Fatal("payload is not 1024")
			}
			rb.Reset(all)
			r.ResetReader(rb)
		}
	})
}

// func Test_Save_File(t *testing.T) {
// 	dat := strings.Repeat("1", 1024*2)
// 	var buf bytes.Buffer
// 	writeMessage(&buf, opcode.Binary, []byte(dat), true)
// 	os.WriteFile("./testdata/binary_2048.dat", buf.Bytes(), 0o644)
// }

func Test_Frame_ReadWrite(t *testing.T) {
	t.Run("ReadWrite.Mask", func(t *testing.T) {
		r := bytes.NewReader(haveMaskData)

		buf := make([]byte, 512)
		rr := fixedreader.NewFixedReader(r, &buf)
		var headArray [14]byte
		f, err := ReadFrame(rr, &headArray)
		if err != nil {
			t.Fatal(err)
		}

		if string(f.Payload[:f.PayloadLen]) != "Hello" {
			t.Fatalf("payload:%s", string(f.Payload[:f.PayloadLen]))
		}

		var w bytes.Buffer
		var fw fixedwriter.FixedWriter
		fmt.Printf("fin: %v, rsv1: %v, rsv2: %v, rsv3: %v, opcode: %v, payloadLen: %v, mask: %v, maskKey: %v\n", f.GetFin(), f.GetRsv1(), f.GetRsv2(), f.GetRsv3(), f.Opcode, f.PayloadLen, f.Mask, f.MaskKey)
		if err := WriteFrame(&fw, &w, f.Payload, true, f.GetRsv1(), f.Mask, opcode.Text, f.MaskKey); err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(w.Bytes(), haveMaskData) {
			t.Fatalf("not equal:%v %v\n", w.Bytes(), haveMaskData)
		}
	})

	t.Run("ReadWrite.NoMask", func(t *testing.T) {
		// br := bytes.NewReader([]byte{0x81, 0x05, 0x48, 0x65, 0x6c, 0x6c, 0x6f})

		var w bytes.Buffer
		var h FrameHeader
		h.PayloadLen = int64(5)
		h.Opcode = 1

		var head [14]byte
		var have int
		var err error
		if have, err = writeHeader(head[:], true, false, false, false, opcode.Text, 5 /*hello 的长度*/, false, 0); err != nil {
			t.Fatal(err)
		}

		w.Write(head[:have])
		_, err = w.WriteString("Hello")
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(w.Bytes(), noMaskData) {
			t.Fatalf("not equal:%v [%b], %v [%b]\n", w.Bytes(), w.Bytes()[0], noMaskData, noMaskData[0])
		}
	})
}

func Test_Frame_Write(t *testing.T) {
	t.Run("WriteFrameToBytes.Binary", func(t *testing.T) {
		var fw fixedwriter.FixedWriter
		fw.Reset(make([]byte, 1024))

		var out1 bytes.Buffer
		var out2 bytes.Buffer
		WriteFrame(&fw, &out1, []byte("12345"), true, false, true, opcode.Binary, 0x12345678)
		WriteFrameToBytes(&out2, []byte("12345"), true, false, true, opcode.Binary, 0x12345678)

		if !bytes.Equal(out1.Bytes(), out2.Bytes()) {
			t.Errorf("not equal:WriteFrameResult(%v):WriteFrameToBytes(%v)\n", out1.Bytes(), out2.Bytes())
		}
	})

	t.Run("WriteFrameToBytes.Text", func(t *testing.T) {
		var fw fixedwriter.FixedWriter
		fw.Reset(make([]byte, 1024))

		var out1 bytes.Buffer
		var out2 bytes.Buffer
		WriteFrame(&fw, &out1, []byte("12345"), true, false, true, opcode.Text, 0x12345678)
		WriteFrameToBytes(&out2, []byte("12345"), true, false, true, opcode.Text, 0x12345678)

		if !bytes.Equal(out1.Bytes(), out2.Bytes()) {
			t.Errorf("not equal:WriteFrameResult(%v):WriteFrameToBytes(%v)\n", out1.Bytes(), out2.Bytes())
		}
	})

	t.Run("WriteFrameToBytes.Binary.NoMask", func(t *testing.T) {
		var fw fixedwriter.FixedWriter
		fw.Reset(make([]byte, 1024))

		var out1 bytes.Buffer
		var out2 bytes.Buffer
		WriteFrame(&fw, &out1, []byte("12345"), true, false, false, opcode.Binary, 0x12345678)
		WriteFrameToBytes(&out2, []byte("12345"), true, false, false, opcode.Binary, 0x12345678)

		if !bytes.Equal(out1.Bytes(), out2.Bytes()) {
			t.Errorf("not equal:WriteFrameResult(%v):WriteFrameToBytes(%v)\n", out1.Bytes(), out2.Bytes())
		}
	})

	t.Run("WriteFrameToBytes.Text.NoMask", func(t *testing.T) {
		var fw fixedwriter.FixedWriter
		fw.Reset(make([]byte, 1024))

		var out1 bytes.Buffer
		var out2 bytes.Buffer
		WriteFrame(&fw, &out1, []byte("12345"), true, false, false, opcode.Text, 0x12345678)
		WriteFrameToBytes(&out2, []byte("12345"), true, false, false, opcode.Text, 0x12345678)

		if !bytes.Equal(out1.Bytes(), out2.Bytes()) {
			t.Errorf("not equal:WriteFrameResult(%v):WriteFrameToBytes(%v)\n", out1.Bytes(), out2.Bytes())
		}
	})
}
