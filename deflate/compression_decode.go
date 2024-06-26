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
	"github.com/antlabs/wsutil/limitreader"
	"github.com/klauspost/compress/flate"
)

var tailBytes = []byte{0x00, 0x00, 0xff, 0xff, 0x01, 0x00, 0x00, 0xff, 0xff}

// 上下文-解压缩
type DeCompressContextTakeover struct {
	dict historyDict
}

// 初始化一个对象
func NewDecompressContextTakeover(bit uint8) (*DeCompressContextTakeover, error) {
	size := 1 << uint(bit)
	de := &DeCompressContextTakeover{}
	de.dict.InitHistoryDict(size)
	return de, nil
}

// 解压缩
// d有值时，上下文接管的情况调用
// d为nil时， 上下文不接管的情况下调用，利用了go，对象为空，调用函数不会panic的特性
func (d *DeCompressContextTakeover) Decompress(payload *[]byte, maxMessage int64) (outBytes2 *[]byte, err error) {
	// 获取dict
	var dict []byte
	if d != nil {
		dict = d.dict.GetData()
	}

	// 拿到解码器
	rc, _ := flateReaderPool.Get().(io.Reader)
	frt, ok := rc.(flate.Resetter)
	if !ok {
		panic("not found flate.Resetter")
	}
	defer func() {
		if err == nil {
			// 如果没有错误，就把解码器放回池里面
			flateReaderPool.Put(frt)
		}
	}()

	frt.Reset(io.MultiReader(bytes.NewReader(*payload), bytes.NewReader(tailBytes)), dict)
	// 从池里面拿buf, 这里的2是经验值，解压缩之后是2倍的大小
	decodeBuf := bytespool.GetBytes(len(*payload)*2 + enum.MaxFrameHeaderSize)
	// 包装下
	out := bytes.NewBuffer((*decodeBuf)[:0])
	// 解压缩

	// 限制大小
	if maxMessage > 0 {
		rc = limitreader.NewLimitReader(rc, maxMessage)
	}
	if _, err := io.Copy(out, rc); err != nil {
		return nil, err
	}
	// 拿到解压缩之后的buf
	outBytes := out.Bytes()
	// 如果解压缩之后的buf和从池里面拿的buf不一样，就把从池里面拿的buf放回去
	if unsafe.SliceData(*decodeBuf) != unsafe.SliceData(outBytes) {
		bytespool.PutBytes(decodeBuf)
	}

	if d != nil {
		// 写入dict
		d.dict.Write(out.Bytes())
	}
	// 返回解压缩之后的buf
	return &outBytes, nil
}
