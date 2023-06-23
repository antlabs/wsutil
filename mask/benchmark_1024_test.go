package mask

import (
	"encoding/binary"
	"math/rand"
	"testing"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func newMask(mask []byte) {
	rand.Read(mask)
}

func Benchmark_Mask_Gorilla_1024(t *testing.B) {
	var payload [1024]byte
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

func Benchmark_Mask_Gobwas_1024(t *testing.B) {
	var payload [1024]byte
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

func Benchmark_Mask_Gws_1024(t *testing.B) {
	var payload [1024]byte
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

func Benchmark_Mask_Nhooyr_1024(t *testing.B) {
	var payload [1024]byte
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

func Benchmark_Mask_Slow_1024(t *testing.B) {
	var payload [1024]byte
	var maskValue [4]byte

	for i := 0; i < len(payload); i++ {
		payload[i] = byte(i)
	}
	newMask(maskValue[:])
	key := binary.LittleEndian.Uint32(maskValue[:])
	t.ResetTimer()

	for i := 0; i < t.N; i++ {
		maskSlow(payload[:], key)
	}
}

func Benchmark_Mask_Fast_1024(t *testing.B) {
	var payload [1024]byte
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
