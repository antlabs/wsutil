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
package frame

import (
	"io"

	"github.com/antlabs/wsutil/enum"
	"github.com/antlabs/wsutil/mask"
)

func ReadFrameFromReaderV2(r io.Reader, headArray *[enum.MaxFrameHeaderSize]byte, buf *[]byte) (f Frame2, err error) {
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
	f.Payload = buf
	f.FrameHeader = h
	if h.Mask {
		mask.Mask(*f.Payload, h.MaskKey)
	}

	return f, nil
}
