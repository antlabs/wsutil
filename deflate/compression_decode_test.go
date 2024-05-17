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
	"os"
	"reflect"
	"testing"
)

// 单数据包直接压缩
func TestDecompressNoContextTakeover(t *testing.T) {
	type args struct {
		payload []byte
	}

	tests := []struct {
		name     string
		args     args
		want     []byte
		fileName string
		bit      uint8
		count    int
		wantErr  bool
	}{
		{name: "测试1", args: args{payload: []byte("hellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohello")}, want: []byte("hellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohello"), wantErr: false},
		{name: "测试1", wantErr: false, count: 13, fileName: "../testdata/1.txt", bit: 9},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 从文件中读取数据
			if len(tt.fileName) > 0 {
				all, err := os.ReadFile(tt.fileName)
				if err != nil {
					t.Errorf("read file error: %v", err)
					return
				}

				all = bytes.Repeat(all, 13)
				tt.args.payload = all
			}
			// 压缩下一段数据
			var encode *CompressContextTakeover
			gotPayload, err := encode.Compress(&tt.args.payload, 0)
			if (err != nil) != tt.wantErr {
				t.Errorf("CompressNoContextTakeover() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			bit := tt.bit
			if bit == 0 {
				bit = 8
			}
			// 新建上下文解压缩
			de, err := NewDecompressContextTakeover(bit)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewDecompressContextTakeover() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// 解压
			gotPayload2, err := de.Decompress(gotPayload, 0)
			if (err != nil) != tt.wantErr {
				t.Errorf("Decompress() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// 比较
			if !reflect.DeepEqual(*gotPayload2, tt.args.payload) {
				t.Errorf("DecompressNoContextTakeover() = %v, want %v", gotPayload2, tt.want)
			}
		})
	}
}

// 多数据包解压缩
func TestDecompressNoContextTakeover2(t *testing.T) {
	type args struct {
		payload []byte
	}
	tests := []struct {
		name     string
		args     args
		want     []byte
		fileName string
		bit      uint8
		count    int
		loop     int
		wantErr  bool
	}{
		{name: "测试1", wantErr: false, count: 13, fileName: "../testdata/1.txt", bit: 9, loop: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 从文件中读取数据
			var needData []byte
			var needAllData []byte
			if len(tt.fileName) > 0 {
				all, err := os.ReadFile(tt.fileName)
				if err != nil {
					t.Errorf("read file error: %v", err)
					return
				}

				needData = all
				needAllData = bytes.Repeat(all, tt.loop)
			}
			bit := tt.bit
			if bit == 0 {
				bit = 8
			}

			var decode [][]byte
			// 新建上下文解压缩
			de, err := NewDecompressContextTakeover(bit)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewDecompressContextTakeover() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			for i := 0; i < tt.loop; i++ {
				// 压缩下一段数据
				var encode *CompressContextTakeover
				gotPayload, err := encode.Compress(&needData, 0)
				if (err != nil) != tt.wantErr {
					t.Errorf("CompressNoContextTakeover() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				decode = append(decode, *gotPayload)
			}

			var gotPayloadTotal []byte
			for i := 0; i < tt.loop; i++ {

				// 解压
				gotPayload2, err := de.Decompress(&decode[i], 0)
				if (err != nil) != tt.wantErr {
					t.Errorf("Decompress() error = %v, wantErr %v", err, tt.wantErr)
					return
				}

				gotPayloadTotal = append(gotPayloadTotal, (*gotPayload2)...)

			}
			// 比较
			if !reflect.DeepEqual(gotPayloadTotal, needAllData) {
				t.Errorf("DecompressNoContextTakeover() = %v, want %v", gotPayloadTotal, needData)
			}
		})
	}
}
