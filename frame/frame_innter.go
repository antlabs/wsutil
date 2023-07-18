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

	return WriteFrame(ws, w, writeBuf, false, isClient, op, maskValue)
}
