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
	"testing"

	"github.com/antlabs/wsutil/fixedreader"
	"github.com/antlabs/wsutil/opcode"
)

var (
	noMaskData   = []byte{0x81, 0x05, 0x48, 0x65, 0x6c, 0x6c, 0x6f}
	haveMaskData = []byte{0x81, 0x85, 0x37, 0xfa, 0x21, 0x3d, 0x7f, 0x9f, 0x4d, 0x51, 0x58}
)

func Test_Frame_Read_Size(t *testing.T) {
	var out bytes.Buffer
	err := writeMessage(&out, opcode.Text, nil, true)
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
}

func Test_Frame_Read_NoMask(t *testing.T) {
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
}

func Test_Frame_Mask_Read_And_Write(t *testing.T) {
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
	if err := WriteFrame(&w, f); err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(w.Bytes(), haveMaskData) {
		t.Fatalf("not equal")
	}
}

func Test_Frame_Write_NoMask(t *testing.T) {
	// br := bytes.NewReader([]byte{0x81, 0x05, 0x48, 0x65, 0x6c, 0x6c, 0x6f})

	var w bytes.Buffer
	var h FrameHeader
	h.PayloadLen = int64(5)
	h.Opcode = 1
	h.Fin = true
	if err := writeHeader(&w, h); err != nil {
		t.Fatal(err)
	}

	_, err := w.WriteString("Hello")
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(w.Bytes(), noMaskData) {
		t.Fatal("not equal")
	}
}
