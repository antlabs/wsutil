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

package frame

import (
	"crypto/rand"
	"encoding/binary"
	"io"
	"math"

	"github.com/antlabs/wsutil/bytespool"
	"github.com/antlabs/wsutil/enum"
	"github.com/antlabs/wsutil/errs"
	"github.com/antlabs/wsutil/fixedwriter"
	"github.com/antlabs/wsutil/mask"
	"github.com/antlabs/wsutil/opcode"
)

func newMask(mask []byte) {
	rand.Read(mask)
}

type FrameHeader struct {
	PayloadLen int64
	Opcode     opcode.Opcode
	MaskValue  [4]byte
	Rsv1       bool
	Rsv2       bool
	Rsv3       bool
	Mask       bool
	Fin        bool
}

type Frame struct {
	FrameHeader
	Payload []byte
}

func ReadHeader(r io.Reader, headArray *[enum.MaxFrameHeaderSize]byte) (h FrameHeader, size int, err error) {
	// var headArray [enum.MaxFrameHeaderSize]byte
	head := (*headArray)[:2]

	n, err := io.ReadFull(r, head)
	if err != nil {
		return
	}
	if n != 2 {
		err = io.ErrUnexpectedEOF
		return
	}
	size = 2

	h.Fin = head[0]&(1<<7) > 0
	h.Rsv1 = head[0]&(1<<6) > 0
	h.Rsv2 = head[0]&(1<<5) > 0
	h.Rsv3 = head[0]&(1<<4) > 0
	h.Opcode = opcode.Opcode(head[0] & 0xF)

	have := 0
	h.Mask = head[1]&(1<<7) > 0
	if h.Mask {
		have += 4
		size += 4
	}

	h.PayloadLen = int64(head[1] & 0x7F)

	switch {
	// 长度
	case h.PayloadLen >= 0 && h.PayloadLen <= 125:
		if h.PayloadLen == 0 && !h.Mask {
			return
		}
	case h.PayloadLen == 126:
		// 2字节长度
		have += 2
		size += 2
	case h.PayloadLen == 127:
		// 8字节长度
		have += 8
		size += 8
	default:
		// 预期之外的, 直接报错
		return h, 0, errs.ErrFramePayloadLength
	}

	head = head[:have]
	_, err = io.ReadFull(r, head)
	if err != nil {
		return
	}

	switch h.PayloadLen {
	case 126:
		h.PayloadLen = int64(binary.BigEndian.Uint16(head[:2]))
		head = head[2:]
	case 127:
		h.PayloadLen = int64(binary.BigEndian.Uint64(head[:8]))
		head = head[8:]
	}

	if h.Mask {
		copy(h.MaskValue[:], head)
	}

	return
}

// https://datatracker.ietf.org/doc/html/rfc6455#section-5.2
// (the most significant bit MUST be 0)
func writeHeader(w io.Writer, h FrameHeader) (err error) {
	var head [enum.MaxFrameHeaderSize]byte

	if h.Fin {
		head[0] |= 1 << 7
	}

	if h.Rsv1 {
		head[0] |= 1 << 6
	}

	if h.Rsv2 {
		head[0] |= 1 << 5
	}

	if h.Rsv3 {
		head[0] |= 1 << 5
	}

	head[0] |= byte(h.Opcode & 0xF)

	have := 2
	switch {
	case h.PayloadLen <= 125:
		head[1] = byte(h.PayloadLen)
	case h.PayloadLen <= math.MaxUint16:
		head[1] = 126
		binary.BigEndian.PutUint16(head[2:], uint16(h.PayloadLen))
		have += 2 // 2前
	default:
		head[1] = 127
		binary.BigEndian.PutUint64(head[2:], uint64(h.PayloadLen))
		have += 8
	}

	if h.Mask {
		head[1] |= 1 << 7
		have += copy(head[have:], h.MaskValue[:])
	}

	_, err = w.Write(head[:have])
	return err
}

func writeMessage(w io.Writer, op opcode.Opcode, writeBuf []byte, isClient bool) (err error) {
	var f Frame
	f.Fin = true
	f.Opcode = op
	f.Payload = writeBuf
	f.PayloadLen = int64(len(writeBuf))
	defer func() {
		f.Payload = nil
	}()
	if isClient {
		f.Mask = true
		newMask(f.MaskValue[:])
	}

	return WriteFrame(w, f)
}

func WriteFrame(w io.Writer, f Frame) (err error) {
	buf := bytespool.GetBytes(len(f.Payload) + enum.MaxFrameHeaderSize)

	var ws fixedwriter.FixedWriter
	ws.Reset(*buf)

	defer func() {
		ws.Free()
		bytespool.PutBytes(buf)
	}()
	if err = writeHeader(&ws, f.FrameHeader); err != nil {
		return
	}

	wIndex := ws.Len()
	_, err = ws.Write(f.Payload)
	if err != nil {
		return
	}
	if f.Mask {
		key := binary.LittleEndian.Uint32(f.MaskValue[:])
		mask.Mask(ws.Bytes()[wIndex:], key)
	}

	// fmt.Printf("writeFrame %#v\n", tmpWriter.Bytes())
	_, err = w.Write(ws.Bytes())
	return err
}
