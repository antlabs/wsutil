package limitreader

import (
	"errors"
	"io"
)

var ErrTooBigMessage = errors.New("message too big")

// 限制读取
type limitReader struct {
	r io.Reader // 包装的io.Reader
	m int64     //最大值
}

func NewLimitReader(r io.Reader, m int64) *limitReader {
	return &limitReader{r: r, m: m}
}

// 目前go.mod使用的是go1.20版本， go1.21才有min函数
func minSize(n, m int) int {
	if n < m {
		return n
	}
	return m
}

// 实现io.Reader接口
func (l *limitReader) Read(p []byte) (n int, err error) {
	if l.m < 0 {
		return 0, ErrTooBigMessage
	}
	rn := minSize(int(l.m), len(p))
	if rn == 0 && len(p) > 0 {
		rn = 1
	}
	n, err = l.r.Read(p[:rn])
	l.m -= int64(n)
	if l.m < 0 {
		return 0, ErrTooBigMessage
	}
	return
}
