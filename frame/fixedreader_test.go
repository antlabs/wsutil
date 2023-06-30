package frame

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/antlabs/wsutil/bytespool"
	"github.com/antlabs/wsutil/enum"
	"github.com/antlabs/wsutil/fixedreader"
	"github.com/antlabs/wsutil/opcode"
	"github.com/stretchr/testify/assert"
)

var (
	testBinaryMessage64kb = bytes.Repeat([]byte("1"), 65535)
	testTextMessage64kb   = bytes.Repeat([]byte("中"), 65535/len("中"))
	testBinaryMessage10   = bytes.Repeat([]byte("1"), 10)
)

// func splitString(s string, chunkSize int) []string {
// 	var chunks []string
// 	for i := 0; i < len(s); i += chunkSize {
// 		end := i + chunkSize
// 		if end > len(s) {
// 			end = len(s)
// 		}
// 		chunks = append(chunks, s[i:end])
// 	}
// 	return chunks
// }

func Test_Reader_Small(t *testing.T) {
	var out bytes.Buffer

	tmp := append([]byte(nil), testTextMessage64kb...)
	err := writeMessage(&out, opcode.Text, tmp, true)
	// hexString := hex.EncodeToString(out.Bytes())
	// // 在每两个字符之间插入空格
	// spacedHexString := strings.Join(splitString(hexString, 2), ", ")
	// fmt.Printf("header: %+v\n", spacedHexString[:100])
	assert.NoError(t, err)

	r := fixedreader.NewFixedReader(&out, bytespool.GetBytes(1024+enum.MaxFrameHeaderSize))

	var headArray [14]byte
	f, err := ReadFrame(r, &headArray)
	assert.NoError(t, err)
	// err = os.WriteFile("./test_reader.dat", f.payload, 0o644)

	assert.NoError(t, err)
	assert.Equal(t, f.Payload, testTextMessage64kb)
}

func Test_Reader_WriteMulti_ReadOne(t *testing.T) {
	var out bytes.Buffer

	for i := 1024 * 63; i <= 1024*63+1; i++ {
		need := make([]byte, 0, i)
		got := make([]byte, 0, i)
		for j := 0; j < i; j++ {
			need = append(need, byte(j))
			got = append(got, byte(j))
		}

		for j := 0; j < 1; j++ {
			err := writeMessage(&out, opcode.Text, need, true)
			assert.NoError(t, err)
			err = writeMessage(&out, opcode.Text, need, true)
			assert.NoError(t, err)
		}
		fmt.Printf("i = %d, need: len(%d), write.size:%d\n", i, len(need), out.Len())

		b := bytespool.GetBytes(1024 + enum.MaxFrameHeaderSize)
		r := fixedreader.NewFixedReader(&out, b)
		var headArray [14]byte
		for j := 0; j < 2; j++ {

			f, err := ReadFrame(r, &headArray)
			assert.NoError(t, err)

			assert.NoError(t, err)
			// TODO
			if j == 0 {
				continue
			}
			if !bytes.Equal(f.Payload, got) {
				t.Fatalf("bad test index:%d\n", i)
				return
			}
			// assert.Equal(t, f.payload, got, fmt.Sprintf("index:%d", i))
			if err != nil {
				return
			}
		}
		bytespool.PutBytes(r.Ptr())
		out.Reset()
	}
}

// 测试只写一次数据包，但是分多次读取
func Test_Reader_WriteOne_ReadMulti(t *testing.T) {
	var out bytes.Buffer

	var headArray [14]byte
	for i := 1031; i <= 1024*64; i++ {
		// for i := 2046; i <= 2048; i++ {
		need := make([]byte, 0, i)
		got := make([]byte, 0, i)
		for j := 0; j < i; j++ {
			need = append(need, byte(j))
			got = append(got, byte(j))
		}

		err := writeMessage(&out, opcode.Text, need, true)
		assert.NoError(t, err)

		b := bytespool.GetBytes(1024 + enum.MaxFrameHeaderSize)
		r := fixedreader.NewFixedReader(&out, b)

		f, err := ReadFrame(r, &headArray)
		assert.NoError(t, err)
		if err != nil {
			return
		}

		assert.NoError(t, err)
		// TODO
		if i == 0 {
			continue
		}
		assert.Equal(t, f.Payload, got, fmt.Sprintf("index:%d", i))
		bytespool.PutBytes(r.Ptr())
		out.Reset()
	}
}

func Test_Reset(t *testing.T) {
	var out bytes.Buffer
	out.Write([]byte("1234"))
	r := fixedreader.NewFixedReader(&out, bytespool.GetBytes(1024+enum.MaxFrameHeaderSize))

	small := make([]byte, 2)

	r.Read(small)
	r.Reset(bytespool.GetBytes(1024*2 + enum.MaxFrameHeaderSize))
	assert.Equal(t, r.Bytes()[:2], []byte("34"))
	// assert.Equal(t, r.free()[:2], []byte{0, 0})
}

func Test_Reader_WriteMulti_ReadOne_64512(t *testing.T) {
	var out bytes.Buffer

	for i := 64512; i <= 64512; i++ {
		need := make([]byte, 0, i)
		got := make([]byte, 0, i)
		for j := 0; j < i; j++ {
			need = append(need, byte(j))
			got = append(got, byte(j))
		}

		for j := 0; j < 1; j++ {
			err := writeMessage(&out, opcode.Text, need, true)
			assert.NoError(t, err)
			err = writeMessage(&out, opcode.Text, need, true)
			assert.NoError(t, err)
		}
		fmt.Printf("i = %d, need: len(%d), write.size:%d\n", i, len(need), out.Len())

		b := bytespool.GetBytes(1024 + enum.MaxFrameHeaderSize)
		r := fixedreader.NewFixedReader(&out, b)
		var headArray [14]byte
		for j := 0; j < 2; j++ {

			f, err := ReadFrame(r, &headArray)
			assert.NoError(t, err)

			assert.NoError(t, err)
			// TODO
			if j == 0 {
				continue
			}
			if !bytes.Equal(f.Payload, got) {
				t.Fatalf("bad test index:%d\n", i)
				return
			}
			// assert.Equal(t, f.payload, got, fmt.Sprintf("index:%d", i))
			if err != nil {
				return
			}
		}
		bytespool.PutBytes(r.Ptr())
		out.Reset()
	}
}

func Test_Reader_WriteMulti_ReadOne_65536(t *testing.T) {
	var out bytes.Buffer

	var headArray [14]byte
	for i := 65536; i <= 64512; i++ {
		need := make([]byte, 0, i)
		got := make([]byte, 0, i)
		for j := 0; j < i; j++ {
			need = append(need, byte(j))
			got = append(got, byte(j))
		}

		for j := 0; j < 1; j++ {
			err := writeMessage(&out, opcode.Text, need, true)
			assert.NoError(t, err)
			err = writeMessage(&out, opcode.Text, need, true)
			assert.NoError(t, err)
		}
		fmt.Printf("i = %d, need: len(%d), write.size:%d\n", i, len(need), out.Len())

		b := bytespool.GetBytes(1024 + enum.MaxFrameHeaderSize)
		r := fixedreader.NewFixedReader(&out, b)
		for j := 0; j < 2; j++ {

			f, err := ReadFrame(r, &headArray)
			if err != io.EOF {
				assert.NoError(t, err)
			}

			if !bytes.Equal(f.Payload, got) {
				t.Fatalf("bad test index:%d\n", i)
				return
			}
			// assert.Equal(t, f.payload, got, fmt.Sprintf("index:%d", i))
			if err != nil {
				return
			}
		}
		bytespool.PutBytes(r.Ptr())
		out.Reset()
	}
}
