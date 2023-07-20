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
		// .w 成员
		wAddr := wv.Elem().FieldByName("w").UnsafeAddr()
		if wAddr != 0 {
			bufio2.ClearWriter((*bufio.Writer)(unsafe.Pointer(wAddr)))
			*(**uintptr)(unsafe.Pointer(wAddr)) = nil
		}

		// 按道理hijack之后不会再调用finishResponse, 等有时间再分析
		// 下面的代码如果打开会导致崩溃
		// .conn成员
		// connVal := wv.Elem().FieldByName("conn")
		// if connVal.UnsafeAddr() != 0 {
		// 	if connVal.Type().Kind() == reflect.Ptr {
		// 		connVal = connVal.Elem()
		// 	}

		// 	bufrAddr := connVal.FieldByName("bufr").UnsafeAddr()
		// 	if bufrAddr != 0 {
		// 		// bufio2.ClearReader((*bufio.Reader)(unsafe.Pointer(bufrAddr)))
		// 		// *(**uintptr)(unsafe.Pointer(bufrAddr)) = nil
		// 	}
		// 	bufwAddr := connVal.FieldByName("bufw").UnsafeAddr()
		// 	if bufwAddr != 0 {
		// 		// bufio2.ClearWriter((*bufio.Writer)(unsafe.Pointer(bufwAddr)))
		// 		// *(**uintptr)(unsafe.Pointer(bufwAddr)) = nil
		// 	}
		// }
	}
}
