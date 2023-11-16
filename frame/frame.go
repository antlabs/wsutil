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
	"bytes"
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

type FrameHeader struct {
	PayloadLen int64
	Opcode     opcode.Opcode
	MaskKey    uint32
	Mask       bool
	Head       byte
}

func (f *FrameHeader) GetFin() bool {
	return f.Head&(1<<7) > 0
}

func (f *FrameHeader) GetRsv1() bool {
	return f.Head&(1<<6) > 0
}

func (f *FrameHeader) GetRsv2() bool {
	return f.Head&(1<<5) > 0
}

func (f *FrameHeader) GetRsv3() bool {
	return f.Head&(1<<4) > 0
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
	h.Head = head[0]

	// h.Fin = head[0]&(1<<7) > 0
	// h.Rsv1 = head[0]&(1<<6) > 0
	// h.Rsv2 = head[0]&(1<<5) > 0
	// h.Rsv3 = head[0]&(1<<4) > 0
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
		h.MaskKey = binary.LittleEndian.Uint32(head[:4])
	}

	return
}

// https://datatracker.ietf.org/doc/html/rfc6455#section-5.2
// (the most significant bit MUST be 0)
func WriteHeader(head []byte, fin bool, rsv1, rsv2, rsv3 bool, code opcode.Opcode, payloadLen int, mask bool, maskValue uint32) (have int, err error) {
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

// fw 是个临时空间，先聚合好数据，再写入 w
func WriteFrame(fw *fixedwriter.FixedWriter, w io.Writer, payload []byte, fin bool, rsv1 bool, isMask bool, code opcode.Opcode, maskValue uint32) (err error) {
	buf := bytespool.GetBytes(len(payload) + enum.MaxFrameHeaderSize)

	var wIndex int
	fw.Reset(*buf)

	if wIndex, err = WriteHeader(*buf, fin, rsv1, false, false, code, len(payload), isMask, maskValue); err != nil {
		goto free
	}

	fw.SetW(wIndex)
	_, err = fw.Write(payload)
	if err != nil {
		goto free
	}
	if isMask {
		mask.Mask(fw.Bytes()[wIndex:], maskValue)
	}

	_, err = w.Write(fw.Bytes())

free:
	fw.Free()
	bytespool.PutBytes(buf)
	return
}

func WriteFrameToBytes(w *bytes.Buffer, payload []byte, fin bool, rsv1 bool, isMask bool, code opcode.Opcode, maskValue uint32) (err error) {
	var head [enum.MaxFrameHeaderSize]byte

	var wIndex int

	if wIndex, err = WriteHeader(head[:], fin, rsv1, false, false, code, len(payload), isMask, maskValue); err != nil {
		return err
	}

	_, err = w.Write(head[:wIndex])
	if err != nil {
		return err
	}

	wIndex = w.Len()
	_, err = w.Write(payload)
	if err != nil {
		return err
	}
	if isMask {
		mask.Mask(w.Bytes()[wIndex:], maskValue)
	}

	return
}
