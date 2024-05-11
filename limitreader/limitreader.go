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

// 实现io.Reader接口
func (l *limitReader) Read(p []byte) (n int, err error) {
	if l.m < 0 {
		return 0, ErrTooBigMessage
	}
	n, err = l.r.Read(p)
	l.m -= int64(n)
	return
}
