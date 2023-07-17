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
func writeHeader(head []byte, fin bool, rsv1, rsv2, rsv3 bool, code opcode.Opcode, payloadLen int, mask bool, maskValue uint32) (have int, err error) {
	head[0] = 0
	head[1] = 0
	head[2] = 0
	head[3] = 0
	head[4] = 0
	head[5] = 0
	head[6] = 0
	head[7] = 0
	head[8] = 0
	head[9] = 0
	head[10] = 0
	head[11] = 0
	head[12] = 0
	head[13] = 0

	if fin {
		head[0] |= 1 << 7
	}

	if rsv1 {
		head[0] |= 1 << 6
	}

	if rsv2 {
		head[0] |= 1 << 5
	}

	if rsv3 {
		head[0] |= 1 << 5
	}

	head[0] |= byte(code & 0xF)

	have = 2
	switch {
	case payloadLen <= 125:
		head[1] = byte(payloadLen)
	case payloadLen <= math.MaxUint16:
		head[1] = 126
		binary.BigEndian.PutUint16(head[2:], uint16(payloadLen))
		have += 2 // 2前
	default:
		head[1] = 127
		binary.BigEndian.PutUint64(head[2:], uint64(payloadLen))
		have += 8
	}

	if mask {
		head[1] |= 1 << 7
		// have += copy(head[have:], maskValue[:])
		binary.LittleEndian.PutUint32(head[have:], maskValue)
		have += 4
	}

	return have, err
}

// https://datatracker.ietf.org/doc/html/rfc6455#section-5.2
// (the most significant bit MUST be 0)
func writeHeaderOld(head []byte, h FrameHeader) (have int, err error) {
	// var head [enum.MaxFrameHeaderSize]byte
	head[0] = 0
	head[1] = 0
	head[2] = 0
	head[3] = 0
	head[4] = 0
	head[5] = 0
	head[6] = 0
	head[7] = 0
	head[8] = 0
	head[9] = 0
	head[10] = 0
	head[11] = 0
	head[12] = 0
	head[13] = 0

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

	have = 2
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

	return have, err
}

func writeMessageOld(w io.Writer, op opcode.Opcode, writeBuf []byte, isClient bool, ws *fixedwriter.FixedWriter) (err error) {
	var f Frame
	f.Fin = true
	f.Opcode = op
	f.PayloadLen = int64(len(writeBuf))
	if isClient {
		f.Mask = true
		newMask(f.MaskValue[:])
	}

	return WriteFrameOld(w, f.FrameHeader, writeBuf, ws)
}

func WriteFrame(ws *fixedwriter.FixedWriter, w io.Writer, payload []byte, rsv1 bool, isClient bool, code opcode.Opcode, maskValue uint32) (err error) {
	buf := bytespool.GetBytes(len(payload) + enum.MaxFrameHeaderSize)

	var wIndex int
	ws.Reset(*buf)

	if wIndex, err = writeHeader(*buf, true, rsv1, false, false, code, len(payload), isClient, maskValue); err != nil {
		goto free
	}

	ws.SetW(wIndex)
	_, err = ws.Write(payload)
	if err != nil {
		goto free
	}
	if isClient {
		mask.Mask(ws.Bytes()[wIndex:], maskValue)
	}

	_, err = w.Write(ws.Bytes())

free:
	ws.Free()
	bytespool.PutBytes(buf)
	return
}

// 不会对外使用
func WriteFrameOld(w io.Writer, f FrameHeader, payload []byte, ws *fixedwriter.FixedWriter) (err error) {
	buf := bytespool.GetBytes(len(payload) + enum.MaxFrameHeaderSize)

	var wIndex int
	ws.Reset(*buf)

	if wIndex, err = writeHeaderOld(*buf, f); err != nil {
		goto free
	}

	ws.SetW(wIndex)
	_, err = ws.Write(payload)
	if err != nil {
		goto free
	}
	if f.Mask {
		key := binary.LittleEndian.Uint32(f.MaskValue[:])
		mask.Mask(ws.Bytes()[wIndex:], key)
	}

	_, err = w.Write(ws.Bytes())

free:
	ws.Free()
	bytespool.PutBytes(buf)
	return
}
