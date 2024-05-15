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

	"github.com/klauspost/compress/flate"
)

func TestEnCompressContextTakeover_Compress(t *testing.T) {
	type fields struct {
		dict historyDict
		w    *flate.Writer
	}
	type args struct {
		payload []byte
	}
	tests := []struct {
		name              string
		fields            fields
		args              args
		wantEncodePayload *[]byte
		wantErr           bool
	}{
		{name: "压缩测试1", args: args{payload: []byte("hello world 12345678910")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, err := NewCompressContextTakeover(8)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewCompressContextTakeover() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			gotEncodePayload, err := e.Compress(&tt.args.payload)
			if (err != nil) != tt.wantErr {
				t.Errorf("CompressContextTakeover.Compress() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			decodePayload, err := DecompressNoContextTakeover(gotEncodePayload)
			if (err != nil) != tt.wantErr {
				t.Errorf("CompressContextTakeover.Compress() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !reflect.DeepEqual(tt.args.payload, *decodePayload) {
				t.Errorf("CompressContextTakeover.Compress() = %v, want %v", tt.args.payload, *decodePayload)
			}
		})
	}
}
