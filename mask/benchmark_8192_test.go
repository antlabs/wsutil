package mask

import (
	"encoding/binary"
	"testing"
)

func Benchmark_Mask_Gorilla_8192(t *testing.B) {
	var payload [8192]byte
	var maskValue [4]byte

	for i := 0; i < len(payload); i++ {
		payload[i] = byte(i)
	}
	newMask(maskValue[:])
	t.ResetTimer()

	for i := 0; i < t.N; i++ {
		maskBytes(maskValue, 0, payload[:])
	}
}

func Benchmark_Mask_Gobwas_8192(t *testing.B) {
	var payload [8192]byte
	var maskValue [4]byte

	for i := 0; i < len(payload); i++ {
		payload[i] = byte(i)
	}
	newMask(maskValue[:])
	t.ResetTimer()

	for i := 0; i < t.N; i++ {
		Cipher(payload[:], maskValue, 0)
	}
}

func Benchmark_Mask_Gws_8192(t *testing.B) {
	var payload [8192]byte
	var maskValue [4]byte

	for i := 0; i < len(payload); i++ {
		payload[i] = byte(i)
	}
	newMask(maskValue[:])
	t.ResetTimer()

	for i := 0; i < t.N; i++ {
		MaskXOR(payload[:], maskValue[:])
	}
}

func Benchmark_Mask_Nhooyr_8192(t *testing.B) {
	var payload [8192]byte
	var maskValue [4]byte

	for i := 0; i < len(payload); i++ {
		payload[i] = byte(i)
	}
	newMask(maskValue[:])
	t.ResetTimer()

	key := binary.LittleEndian.Uint32(maskValue[:])
	for i := 0; i < t.N; i++ {
		maskNhooyr(key, payload[:])
	}
}

func Benchmark_Mask_Slow_8192(t *testing.B) {
	var payload [8192]byte
	var maskValue [4]byte

	for i := 0; i < len(payload); i++ {
		payload[i] = byte(i)
	}
	key := binary.LittleEndian.Uint32(maskValue[:])
	newMask(maskValue[:])
	t.ResetTimer()

	for i := 0; i < t.N; i++ {
		maskSlow(payload[:], key)
	}
}

func Benchmark_Mask_Fast_8192(t *testing.B) {
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

// func Benchmark_Mask_Big_8192(t *testing.B) {
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
