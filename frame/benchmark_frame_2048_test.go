package frame

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/antlabs/wsutil/enum"
	"github.com/antlabs/wsutil/fixedreader"
)

func Benchmark_ReadFrame_2048(b *testing.B) {
	b.ReportAllocs()

	// 阈值
	threshold := 2.0

	buf := make([]byte, int((1024+enum.MaxFrameHeaderSize)*threshold))

	all, err := os.ReadFile("./testdata/binary_2048.dat")
	if err != nil {
		b.Fatal(err)
	}

	rb := bytes.NewReader(all)
	r := fixedreader.NewFixedReader(rb, &buf)

	headArray := [enum.MaxFrameHeaderSize]byte{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f, err := ReadFrame(r, &headArray)
		if err != nil {
			b.Fatal(err)
		}

		if len(f.Payload) == 0 {
			fmt.Printf("%#v, r.r = %d, r.w  %d\n", f, r.R, r.W)
			b.Fatal("payload is empty")
		}

		if len(f.Payload) != 2048 {
			fmt.Printf("%#v, r.r = %d, r.w  %d\n", f, r.R, r.W)
			b.Fatal("payload is not 2048")
		}
		rb.Reset(all)
		r.ResetReader(rb)
	}
}

func Benchmark_ReadFromReader_2048(b *testing.B) {
	b.ReportAllocs()

	// 阈值
	threshold := 2.0

	buf := make([]byte, int((1024+enum.MaxFrameHeaderSize)*threshold))

	all, err := os.ReadFile("./testdata/binary_2048.dat")
	if err != nil {
		b.Fatal(err)
	}

	rb := bytes.NewReader(all)
	r := bufio.NewReader(rb)

	headArray := [enum.MaxFrameHeaderSize]byte{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f, err := ReadFromReader(r, &headArray, &buf)
		if err != nil {
			b.Fatal(err)
		}

		if len(f.Payload) == 0 {
			b.Fatal("payload is empty")
		}

		if len(f.Payload) != 2048 {
			b.Fatal("payload is not 2048")
		}

		rb.Reset(all)
		r.Reset(rb)
	}
}
