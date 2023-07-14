package fixedwriter

import "fmt"

type FixedWriter struct {
	buf []byte
	w   int
}

func NewFixedWriter(buf []byte) *FixedWriter {
	return &FixedWriter{
		buf: buf,
	}
}

func (fw *FixedWriter) SetW(w int) {
	fw.w = w
}

func (fw *FixedWriter) Reset(buf []byte) {
	fw.buf = buf
	fw.w = 0
}

func (fw *FixedWriter) Write(p []byte) (n int, err error) {
	if len(fw.buf[fw.w:]) < len(p) {
		panic(fmt.Sprintf("fixedWriter: buf is too small: %d:%d < %d", len(fw.buf[fw.w:]), cap(fw.buf), cap(p)))
	}
	n = copy(fw.buf[fw.w:], p)
	fw.w += n
	return n, nil
}

func (fw *FixedWriter) Len() int {
	return fw.w
}

func (fw *FixedWriter) Bytes() []byte {
	return fw.buf[:fw.w]
}

// 释放
func (fw *FixedWriter) Free() {
	fw.buf = nil
	fw.w = 0
}
