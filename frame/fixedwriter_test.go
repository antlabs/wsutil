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
	"testing"

	"github.com/antlabs/wsutil/fixedwriter"
)

func Test_FixedWriter(t *testing.T) {
	fw := fixedwriter.NewFixedWriter(make([]byte, 1024))
	n, err := fw.Write([]byte("hello"))
	if err != nil {
		t.Errorf("fw.Write() = %v, want nil", err)
	}
	if n != 5 {
		t.Errorf("fw.Write() = %d, want 5", n)
	}

	if fw.Len() != 5 {
		t.Errorf("fw.Len() = %d, want 5", fw.Len())
	}
	if string(fw.Bytes()) != "hello" {
		t.Errorf("fw.Bytes() = %s, want hello", fw.Bytes())
	}
}
