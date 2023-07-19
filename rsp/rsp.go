package rsp

import (
	"bufio"
	"net/http"
	"reflect"
	"unsafe"

	"github.com/antlabs/wsutil/bufio2"
)

func ClearRsp(w http.ResponseWriter) {
	wv := reflect.ValueOf(w)
	wt := wv.Elem().Type()
	if wt.Name() == "response" {
		wAddr := wv.Elem().FieldByName("w").UnsafeAddr()
		if wAddr != 0 {
			bufio2.ClearWriter((*bufio.Writer)(unsafe.Pointer(wAddr)))
		}
	}
}
