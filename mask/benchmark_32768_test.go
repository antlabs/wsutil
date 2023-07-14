package mask

import (
	"encoding/binary"
	"testing"
)

func Benchmark_Mask_Fast_32768(t *testing.B) {
	var payload [8192]byte
	var maskValue [4]byte

	for i := 0; i < len(payload); i++ {
		payload[i] = byte(i)
	}
	newMask(maskValue[:])
	key := binary.LittleEndian.Uint32(maskValue[:])
	t.ResetTimer()
	for i := 0; i < t.N; i++ {
		maskFast(payload[:], key)
	}
}

// func Benchmark_Mask_Big_32768(t *testing.B) {
// 	var payload [8192]byte
// 	var maskValue [4]byte

// 	for i := 0; i < len(payload); i++ {
// 		payload[i] = byte(i)
// 	}
// 	newMask(maskValue[:])
// 	key := binary.LittleEndian.Uint32(maskValue[:])
// 	t.ResetTimer()
// 	for i := 0; i < t.N; i++ {
// 		maskBig(payload[:], key)
// 	}
// }
