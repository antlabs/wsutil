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
	"net/http"
	"reflect"
	"testing"
)

func Test_needDecompression(t *testing.T) {
	type args struct {
		header http.Header
	}
	tests := []struct {
		name string
		args args
		want bool
		got  PermessageDeflateConf
	}{
		{name: "test1", want: false, got: PermessageDeflateConf{
			Enable:        true,
			Decompression: true,
			Compression:   true,
		}, args: args{header: http.Header{"Sec-Websocket-Extensions": {"permessage-deflate; server_no_context_takeover; client_no_context_takeover"}}}},
		{name: "test2", want: true, args: args{header: http.Header{"Sec-Websocket-Extensions": {"xx"}}}},
		{name: "test3", got: PermessageDeflateConf{
			Enable:                true,
			Decompression:         true,
			Compression:           true,
			ServerContextTakeover: false,
			ClientContextTakeover: false,
			ClientMaxWindowBits:   15,
			ServerMaxWindowBits:   9,
		}, want: false, args: args{header: http.Header{"Sec-Websocket-Extensions": {"permessage-deflate; client_no_context_takeover; client_max_window_bits; server_no_context_takeover; server_max_window_bits=9"}}}},
		{name: "test4", got: PermessageDeflateConf{
			Enable:                true,
			Decompression:         true,
			Compression:           true,
			ServerContextTakeover: false,
			ClientContextTakeover: false,
			ClientMaxWindowBits:   15,
			ServerMaxWindowBits:   9,
		}, want: false, args: args{header: http.Header{"Sec-Websocket-Extensions": {"permessage-deflate; client_no_context_takeover; client_max_window_bits; server_no_context_takeover; server_max_window_bits=9, permessage-deflate; client_no_context_takeover; client_max_window_bits; server_no_context_takeover, permessage-deflate; client_no_context_takeover; client_max_window_bits"}}}},
	}
	for index, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pd, err := GetConnPermessageDeflate(tt.args.header)
			if (err != nil) != tt.want {
				t.Errorf("index:%d, genConnPermessageDefalte %v\n", index, err)
				return
			}

			if err != nil {
				return
			}

			if !reflect.DeepEqual(pd, tt.got) {
				t.Errorf("index:%d, want %#v, got %#v\n", index, tt.got, pd)
			}
		})
	}
}
