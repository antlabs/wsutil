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
	"io"
	"unsafe"

	"github.com/antlabs/wsutil/bytespool"
	"github.com/antlabs/wsutil/enum"
	"github.com/klauspost/compress/flate"
)

type EnCompressContextTakeover struct {
	dict historyDict
	w    *flate.Writer
}

type wrapBuffer struct {
	*bytes.Buffer
}

func (w *wrapBuffer) Close() error {
	return nil
}

var enTail = []byte{0, 0, 0xff, 0xff}

func NewEncompressContextTakeover(bit uint8) (en *EnCompressContextTakeover, err error) {
	size := 1 << bit
	w, err := flate.NewWriterWindow(nil, size)
	if err != nil {
		return nil, err
	}
	en = &EnCompressContextTakeover{w: w}
	en.dict.InitHistoryDict(size)
	return en, nil
}

func (e *EnCompressContextTakeover) Compress(payload []byte) (encodePayload *[]byte, err error) {

	encodeBuf := bytespool.GetBytes(len(payload) + enum.MaxFrameHeaderSize)

	out := wrapBuffer{Buffer: bytes.NewBuffer((*encodeBuf)[:0])}
	e.w.ResetDict(out, e.dict.GetData())
	if _, err = io.Copy(e.w, bytes.NewReader(payload)); err != nil {
		return nil, err
	}

	if err = e.w.Flush(); err != nil {
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
