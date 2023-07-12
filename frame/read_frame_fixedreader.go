package frame

import (
	"encoding/binary"
	"io"

	"github.com/antlabs/wsutil/bytespool"
	"github.com/antlabs/wsutil/enum"
	"github.com/antlabs/wsutil/fixedreader"
	"github.com/antlabs/wsutil/mask"
)

func ReadFrame(r *fixedreader.FixedReader, headArray *[enum.MaxFrameHeaderSize]byte) (f Frame, err error) {
	return ReadFrameFromWindows(r, headArray, 1.0)
}

func ReadFrameFromWindows(r *fixedreader.FixedReader, headArray *[enum.MaxFrameHeaderSize]byte, multipletimes float32 /*几倍的payload*/) (f Frame, err error) {
	// 如果剩余可写缓存区放不下一个frame header, 就把数据往前移动
	// 所有的的buf分配都是paydload + frame head 的长度, 挪完之后，肯定是能放下一个frame header的
	if r.Len()-r.R < enum.MaxFrameHeaderSize {
		r.LeftMove()
		if r.Len() < enum.MaxFrameHeaderSize {
			panic("readFrame r.Len() < enum.MaxFrameHeaderSize")
		}
	}

	h, _, err := ReadHeader(r, headArray)
	if err != nil {
		return f, err
	}

	// 如果缓存区不够, 重新分配

	// h.payloadLen 是要读取body的总数据
	// h.w - h.r 是已经读取未处理的数据
	// 还需要读取的数据等于 h.payloadLen - (h.w - h.r)

	// 已读取未处理的数据
	readUnhandle := int64(r.W - r.R)
	// 情况 1，需要读的长度 > 剩余可用空间(未写的+已经被读取走的)
	if h.PayloadLen-readUnhandle > r.Available() {
		// 1.取得旧的buf
		oldBuf := r.BufPtr()
		// 2.获取新的buf
		newBuf := bytespool.GetBytes(int(float32(h.PayloadLen+enum.MaxFrameHeaderSize) * multipletimes))
		// 3.重置缓存区
		r.Reset(newBuf)
		// 4.将旧的buf放回池子里
		bytespool.PutBytes(oldBuf)

		// 情况 2。 空间是够的，需要挪一挪, 把已经读过的覆盖掉
	} else if h.PayloadLen-readUnhandle > int64(r.WriteCap()) {
		r.LeftMove()
	}

	// 返回可写的缓存区
	payload := r.WriteCapBytes()
	// 前面的reset已经保证了，buffer的大小是够的
	needRead := h.PayloadLen - readUnhandle

	if needRead > 0 {
		// payload是一块干净可写的空间，使用needRead框下范围
		payload = payload[:needRead]
		// 新建一对新的r w指向尾部的内存区域
		right := r.CloneAvailable()
		if _, err = io.ReadFull(right, payload); err != nil {
			return f, err
		}

		// right 也有可能超读, 直接加上payload的长度，会把超读的数据给丢了
		// 为什么会发生超读呢，right持的buf 会 >= payload的长度
		r.W += right.W
	}

	f.Payload = r.Bytes()[r.R : r.R+int(h.PayloadLen)]
	f.FrameHeader = h
	r.R += int(h.PayloadLen)
	if h.Mask {
		key := binary.LittleEndian.Uint32(h.MaskValue[:])
		mask.Mask(f.Payload, key)
	}

	return f, nil
}
