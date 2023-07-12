package frame

import (
	"encoding/binary"
	"io"

	"github.com/antlabs/wsutil/enum"
	"github.com/antlabs/wsutil/mask"
)

func ReadFrameFromReader(r io.Reader, headArray *[enum.MaxFrameHeaderSize]byte, buf *[]byte) (f Frame, err error) {
	h, _, err := ReadHeader(r, headArray)
	if err != nil {
		return f, err
	}

	if cap(*buf) < int(h.PayloadLen) {
		// TODO sync.Pool 处理
		*buf = make([]byte, h.PayloadLen)
	}
	*buf = (*buf)[:h.PayloadLen]
	n1, err := io.ReadFull(r, *buf)
	if err != nil {
		return f, err
	}
	if n1 != int(h.PayloadLen) {
		return f, io.ErrUnexpectedEOF
	}
	f.Payload = *buf
	f.FrameHeader = h
	if h.Mask {
		key := binary.LittleEndian.Uint32(h.MaskValue[:])
		mask.Mask(f.Payload, key)
	}

	return f, nil
}
