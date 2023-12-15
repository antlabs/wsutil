package bytespool

import (
	"testing"
)

func Benchmark_GetBytesAndPutBytes_1024(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b := GetBytes(1024)
		PutBytes(b)
	}
}

func Benchmark_GetBytesAndPutBytes_64k(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b := GetBytes(1024 * 64)
		PutBytes(b)
	}
}
