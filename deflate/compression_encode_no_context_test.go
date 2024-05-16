// Copyright 2017 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package deflate

import (
	"reflect"
	"testing"
)

func Test_compressNoContextTakeover(t *testing.T) {
	type args struct {
		payload []byte
		level   int
	}
	tests := []struct {
		name          string
		args          args
		wantdecodeBuf []byte
		wantErr       bool
	}{
		{name: "测试压缩1", args: args{payload: []byte("hello"), level: 1}, wantdecodeBuf: []byte("hello"), wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotEncodeBuf, err := CompressNoContextTakeover(&tt.args.payload, tt.args.level, 10)
			if (err != nil) != tt.wantErr {
				t.Errorf("compressNoContextTakeover() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			gotDecode, err := DecompressNoContextTakeover(gotEncodeBuf)
			if (err != nil) != tt.wantErr {
				t.Errorf("decompressNoContextTakeover() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(*gotDecode, tt.wantdecodeBuf) {
				t.Errorf("compressNoContextTakeover() = %v, want %v", gotDecode, tt.wantdecodeBuf)
			}
		})
	}
}
