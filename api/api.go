package api

import (
	"time"

	"github.com/antlabs/wsutil/opcode"
)

// WebSocket Writer的缩写
type WsWriter interface {
	//WriteCloseTimeout(sc StatusCode, t time.Duration) (err error)
	WriteControl(op opcode.Opcode, data []byte) (err error)
	WriteMessage(op opcode.Opcode, writeBuf []byte) (err error)
	WritePing(data []byte) (err error)
	WritePong(data []byte) (err error)
	WriteTimeout(op opcode.Opcode, data []byte, t time.Duration) (err error)
}
