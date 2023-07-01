package frame

import (
	"io"

	"github.com/antlabs/wsutil/enum"
)

func ReadFromReader(r io.Reader, headArray *[enum.MaxFrameHeaderSize]byte, buf *[]byte) (f Frame, err error) {
	h, _, err := ReadHeader(r, headArray)
	if err != nil {
		return f, err
	}

	if cap(*buf) < int(h.PayloadLen) {
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
	return f, nil
}
