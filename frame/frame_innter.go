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
	"math/rand"

	"github.com/antlabs/wsutil/fixedwriter"
	"github.com/antlabs/wsutil/opcode"
)

func writeMessageInner(w io.Writer, op opcode.Opcode, writeBuf []byte, isClient bool, ws *fixedwriter.FixedWriter) (err error) {
	maskValue := uint32(0)
	if isClient {
		maskValue = rand.Uint32()
	}

	return WriteFrame(ws, w, writeBuf, true, false, isClient, op, maskValue)
}
