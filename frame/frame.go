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
	"github.com/antlabs/wsutil/fixedreader"
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

func ReadFrame(r *fixedreader.FixedReader, headArray *[enum.MaxFrameHeaderSize]byte) (f Frame, err error) {
	// 如果剩余可写缓存区放不下一个frame header, 就把数据往前移动
	// 所有的的buf分配都是paydload + frame head 的长度, 挪完之后，肯定是能放下一个frame header的
	if r.Len()-r.R < enum.MaxFrameHeaderSize {
		r.LeftMove()
		if r.Len() < enum.MaxFrameHeaderSize {
			panic("readFrame r.Len() < enum.MaxFrameHeaderSize")
		}
	}

	h, _, err := ReadHeader(r, headArray)
	if err != nil {
		return f, err
	}

	// 如果缓存区不够, 重新分配

	// h.payloadLen 是要读取body的总数据
	// h.w - h.r 是已经读取未处理的数据
	// 还需要读取的数据等于 h.payloadLen - (h.w - h.r)

	// 已读取未处理的数据
	readUnhandle := int64(r.W - r.R)
	// 情况 1，需要读的长度 > 剩余可用空间(未写的+已经被读取走的)
	if h.PayloadLen-readUnhandle > r.Available() {
		// 1.取得旧的buf
		oldBuf := r.Ptr()
		// 2.获取新的buf
		newBuf := bytespool.GetBytes(int(h.PayloadLen) + enum.MaxFrameHeaderSize)
		// 3.重置缓存区
		r.Reset(newBuf)
		// 4.将旧的buf放回池子里
		bytespool.PutBytes(oldBuf)

		// 情况 2。 空间是够的，需要挪一挪, 把已经读过的覆盖掉
	} else if h.PayloadLen-readUnhandle > int64(r.WriteCap()) {
		r.LeftMove()
	}

	// 返回可写的缓存区
	payload := r.WriteCapBytes()
	// 前面的reset已经保证了，buffer的大小是够的
	needRead := h.PayloadLen - readUnhandle

	if needRead > 0 {
		// payload是一块干净可写的空间，使用needRead框下范围
		payload = payload[:needRead]
		// 新建一对新的r w指向尾部的内存区域
		right := r.CloneAvailable()
		if _, err = io.ReadFull(right, payload); err != nil {
			return f, err
		}

		// right 也有可能超读, 直接加上payload的长度，会把超读的数据给丢了
		// 为什么会发生超读呢，right持的buf 会 >= payload的长度
		r.W += right.W
	}

	f.Payload = r.Bytes()[r.R : r.R+int(h.PayloadLen)]
	f.FrameHeader = h
	r.R += int(h.PayloadLen)
	if h.Mask {
		key := binary.LittleEndian.Uint32(h.MaskValue[:])
		mask.Mask(f.Payload, key)
	}

	return f, nil
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

	tmpWriter := fixedwriter.NewFixedWriter(*buf)
	var ws io.Writer = tmpWriter

	defer func() {
		tmpWriter.Free()
		bytespool.PutBytes(buf)
	}()
	if err = writeHeader(ws, f.FrameHeader); err != nil {
		return
	}

	wIndex := tmpWriter.Len()
	_, err = ws.Write(f.Payload)
	if err != nil {
		return
	}
	if f.Mask {
		key := binary.LittleEndian.Uint32(f.MaskValue[:])
		mask.Mask(tmpWriter.Bytes()[wIndex:], key)
	}

	// fmt.Printf("writeFrame %#v\n", tmpWriter.Bytes())
	_, err = w.Write(tmpWriter.Bytes())
	return err
}