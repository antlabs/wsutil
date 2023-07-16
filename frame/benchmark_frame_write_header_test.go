package frame

import (
	"bytes"
	"testing"

	"github.com/antlabs/wsutil/fixedwriter"
	"github.com/antlabs/wsutil/opcode"
)

type WriteNull struct {
	buf [1024 + 14]byte
}

func (w *WriteNull) Write(p []byte) (n int, err error) {
	n = copy(w.buf[:], p)
	return n, nil
}

func Benchmark_Write_Header(b *testing.B) {
	var head [14]byte
	for i := 0; i < b.N; i++ {
		var f Frame
		f.Fin = true
		f.Opcode = opcode.Binary
		f.PayloadLen = 1024
		writeHeader(head[:], f.FrameHeader)
	}
}

func Benchmark_WriteFrame2(b *testing.B) {
	var buf bytes.Buffer
	payload := make([]byte, 1024)
	for i := range payload {
		payload[i] = 1
	}

	buf.Write(payload)
	b.ResetTimer()
	var ws fixedwriter.FixedWriter
	var w WriteNull

	for i := 0; i < b.N; i++ {
		//
		WriteFrame2(&ws, &w, payload, false, false, opcode.Binary, 0)
		buf.Reset()
	}
}

func Benchmark_WriteFrame(b *testing.B) {
	var buf bytes.Buffer
	payload := make([]byte, 1024)
	for i := range payload {
		payload[i] = 1
	}

	buf.Write(payload)
	b.ResetTimer()
	var ws fixedwriter.FixedWriter
	var w WriteNull

	var f Frame
	for i := 0; i < b.N; i++ {
		//
		WriteFrame(&w, f.FrameHeader, payload, &ws)
		buf.Reset()
	}
}
