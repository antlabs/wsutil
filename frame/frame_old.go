package frame

import (
	"encoding/binary"
	"io"
	"math"

	"github.com/antlabs/wsutil/bytespool"
	"github.com/antlabs/wsutil/enum"
	"github.com/antlabs/wsutil/fixedwriter"
	"github.com/antlabs/wsutil/mask"
	"github.com/antlabs/wsutil/opcode"
)

// 在未来的一段时间可能会删除
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
