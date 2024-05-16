package bytespool

import (
	"testing"
)

func TestGetBytes(t *testing.T) {
	type args struct {
		n     int
		write int
	}
	tests := []struct {
		name   string
		args   args
		wantRv *[]byte
	}{
		{name: "测试1025", args: args{n: 2048, write: 2000}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := make([]byte, tt.args.write)
			PutBytes(&buf)
			payload := GetBytes(tt.args.n)
			if cap(*payload) < tt.args.n {
				panic("too small")
			}
		})
	}
}
