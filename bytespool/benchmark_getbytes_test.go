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

func Benchmark_GetBytesAndPutBytes2_1024(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b := GetBytes2(1024)
		PutBytes2(b)
	}
}

func Benchmark_GetBytesAndPutBytes_64k(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b := GetBytes(1024 * 64)
		PutBytes(b)
	}
}

func Benchmark_GetBytesAndPutBytes2_64(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b := GetBytes2(1024 * 64)
		PutBytes2(b)
	}
}
