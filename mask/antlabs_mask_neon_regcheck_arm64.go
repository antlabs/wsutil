//go:build darwin && arm64 && wsutil_neon

package mask

//go:noescape
func regSample() (r18v, gv, fpv uint64)

// regClobberProbe 调用 maskNEON, 并比较调用前后三个保留寄存器。
//
// 纯粹由 Go 侧驱动: 先用 regSample 取快照, 再正常调用 maskNEON,
// 再取一次快照。这样完全不介入 ABI 的帧布局, 避免手工搭帧出错。
//
// 返回位图: 0=R18_PLATFORM  1=g  2=R29
func regClobberProbe(payload []byte, key uint32) uint64 {
	r18a, ga, fpa := regSample()

	maskNEON(payload, key)

	r18b, gb, fpb := regSample()

	var mask uint64
	if r18a != r18b {
		mask |= 1
	}
	if ga != gb {
		mask |= 2
	}
	if fpa != fpb {
		mask |= 4
	}
	return mask
}
