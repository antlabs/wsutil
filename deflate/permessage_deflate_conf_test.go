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
	"fmt"
	"net/http"
	"testing"
)

func Test_needDecompression(t *testing.T) {
	type args struct {
		header http.Header
	}
	tests := []struct {
		name   string
		args   args
		want   bool
		enable bool
	}{
		{name: "test1", args: args{header: http.Header{"Sec-Websocket-Extensions": {"permessage-deflate; server_no_context_takeover; client_no_context_takeover"}}}, want: false, enable: true},
		{name: "test2", args: args{header: http.Header{"Sec-Websocket-Extensions": {"xx"}}}, want: true, enable: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pd, err := GetConnPermessageDeflate(tt.args.header)
			if (err != nil) != tt.want {
				t.Errorf("genConnPermessageDefalte %v\n", err)
				return
			}
			fmt.Printf("%#v\n", pd)
			if pd.Enable != tt.enable {
				t.Errorf("needDecompression() = %v, want %v", pd, tt.want)
			}
		})
	}
}
