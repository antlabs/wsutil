// Copyright 2021-2024 antlabs. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
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

func newCompressContextTakeover(w io.WriteCloser, level int, bit uint8) (*flate.Writer, *sync.Pool) {
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

type CompressContextTakeover struct {
	dict historyDict
	bit  uint8
}

type wrapBuffer struct {
	*bytes.Buffer
}

func (w *wrapBuffer) Close() error {
	return nil
}

var enTail = []byte{0, 0, 0xff, 0xff}

func NewCompressContextTakeover(bit uint8) (en *CompressContextTakeover, err error) {
	size := 1 << bit
	en = &CompressContextTakeover{}
	en.dict.InitHistoryDict(size)
	en.bit = bit
	return en, nil
}

func (e *CompressContextTakeover) Compress(payload *[]byte) (encodePayload *[]byte, err error) {

	encodeBuf := bytespool.GetBytes(len(*payload) + enum.MaxFrameHeaderSize)

	bit := uint8(0)
	var dict []byte
	if e != nil {
		bit = e.bit
		dict = e.dict.GetData()
	}
	w, p := newCompressContextTakeover(nil, DefaultCompressionLevel, bit)

	defer func() {
		// 如果没有出错
		if err == nil {
			p.Put(w)
		}
	}()
	out := wrapBuffer{Buffer: bytes.NewBuffer((*encodeBuf)[:0])}

	w.ResetDict(out, dict)
	if _, err = io.Copy(w, bytes.NewReader(*payload)); err != nil {
		return nil, err
	}

	if err = w.Flush(); err != nil {
		return nil, err
	}

	if out.Len() >= 4 {
		last4 := out.Bytes()[out.Len()-4:]
		if !bytes.Equal(last4, enTail) {
			return nil, ErrUnexpectedFlateStream
		}
		out.Truncate(out.Len() - 4)
	}

	if unsafe.SliceData(*encodeBuf) != unsafe.SliceData(out.Buffer.Bytes()) {
		bytespool.PutBytes(encodeBuf)
	}

	outBuf := out.Bytes()
	return &outBuf, nil
}
