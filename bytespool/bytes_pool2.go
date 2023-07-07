package bytespool

import (
	"sync"

	"github.com/antlabs/wsutil/enum"
)

const (
	startSize = 256
	// 256 512 1k 2k 4k 8k 16k 32k 64k
	poolSize = 10
	maxSize  = 64*1024 + enum.MaxFrameHeaderSize
)

var poolsNew = make([]sync.Pool, 0, poolSize)

func init() {
	prev := startSize
	for i := 0; i < poolSize; i++ {
		prev2 := prev
		poolsNew = append(poolsNew, sync.Pool{
			New: func() interface{} {
				buf := make([]byte, prev2+enum.MaxFrameHeaderSize)
				return &buf
			},
		})
		prev *= 2
	}
}

func GetBytes2(n int) (rv *[]byte) {
	prev := startSize
	for i := 0; i < len(poolsNew); i++ {
		if n <= prev+enum.MaxFrameHeaderSize {
			return poolsNew[i].Get().(*[]byte)
		}
		prev *= 2
	}
	buf := make([]byte, n)
	return &buf
}

func PutBytes2(buf *[]byte) {
	if cap(*buf) > maxSize {
		return
	}

	prev := startSize
	for i := 0; i < len(poolsNew); i++ {
		if cap(*buf) <= prev+enum.MaxFrameHeaderSize {
			poolsNew[i].Put(buf)
			return
		}
		prev *= 2
	}

	poolsNew[9].Put(buf)
}
