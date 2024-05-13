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
	"reflect"
	"testing"
)

func TestDecompressNoContextTakeover(t *testing.T) {
	type args struct {
		payload []byte
	}
	tests := []struct {
		name    string
		args    args
		want    []byte
		wantErr bool
	}{
		{name: "测试1", args: args{payload: []byte("hellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohello")}, want: []byte("hellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohellohello"), wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 压缩下一段数据
			gotPayload, err := CompressNoContextTakeover(tt.args.payload, 1)
			if (err != nil) != tt.wantErr {
				t.Errorf("CompressNoContextTakeover() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			// 新建上下文解压缩
			de, err := NewDecompressContextTakeover(8)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewDecompressContextTakeover() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// 解压
			gotPayload2, err := de.Decompress(*gotPayload, 0)
			if (err != nil) != tt.wantErr {
				t.Errorf("Decompress() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !reflect.DeepEqual(*gotPayload2, tt.want) {
				t.Errorf("DecompressNoContextTakeover() = %v, want %v", gotPayload2, tt.want)
			}
		})
	}
}
