// Copyright 2017 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package deflate

import (
	"bytes"
	"errors"
	"io"
	"sync"
	"unsafe"

	"github.com/antlabs/wsutil/bytespool"
	"github.com/antlabs/wsutil/enum"
	"github.com/klauspost/compress/flate"
)

var (
	ErrUnexpectedFlateStream = errors.New("internal error, unexpected bytes at end of flate stream")
	ErrWriteClosed           = errors.New("write close")
)

const (
	minCompressionLevel     = -2 // flate.HuffmanOnly not defined in Go < 1.6
	maxCompressionLevel     = flate.BestCompression
	DefaultCompressionLevel = 1
)

var (
	// bitPoolSize         = 15 - 8 + 1
	minBit              = 8
	flateWriterBitPools [15 - 8 + 1]sync.Pool
	flateWriterPools    [maxCompressionLevel - minCompressionLevel + 1]sync.Pool
	flateReaderPool     = sync.Pool{New: func() interface{} {
		return flate.NewReader(nil)
	}}
)

/*
func isValidCompressionLevel(level int) bool {
	return minCompressionLevel <= level && level <= maxCompressionLevel
}
*/

func newCompressNoContextTakeover(w io.WriteCloser, level int, bit uint8) (*flate.Writer, *sync.Pool) {
	var fw *flate.Writer
	var p *sync.Pool
	if bit > 0 {

		p = &flateWriterBitPools[bit-uint8(minBit)]

		fw, _ = p.Get().(*flate.Writer)
		if fw == nil {
			fw, _ = flate.NewWriterWindow(w, 1<<bit)
		} else {
			fw.Reset(w)
		}
	} else {

		p = &flateWriterPools[level-minCompressionLevel]
		fw, _ = p.Get().(*flate.Writer)
		if fw == nil {
			fw, _ = flate.NewWriter(w, level)
		} else {
			fw.Reset(w)
		}
	}

	return fw, p
}

func CompressNoContextTakeover(payload *[]byte, level int, bit uint8) (encodeBuf *[]byte, err error) {

	encodeBuf = bytespool.GetBytes(len(*payload) + enum.MaxFrameHeaderSize)
	out := wrapBuffer{Buffer: bytes.NewBuffer((*encodeBuf)[:0])}

	w, p := newCompressNoContextTakeover(&out, DefaultCompressionLevel, bit)
	if _, err = io.Copy(w, bytes.NewReader(*payload)); err != nil {
		return nil, err
	}

	defer p.Put(w)

	if err = w.Flush(); err != nil {
		return nil, err
	}

	if unsafe.SliceData(*encodeBuf) != unsafe.SliceData(out.Buffer.Bytes()) {
		bytespool.PutBytes(encodeBuf)
	}

	if out.Len() >= 4 {
		last4 := out.Bytes()[out.Len()-4:]
		if !bytes.Equal(last4, enTail) {
			return nil, ErrUnexpectedFlateStream
		}

		out.Truncate(out.Len() - 4)
	}

	outBuf := out.Bytes()
	return &outBuf, nil
}
