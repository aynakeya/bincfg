package bincfg

import (
	"bytes"
	"crypto/rand"
	"fmt"
	mathrand "math/rand/v2"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// 打包：高 8 位 cfg，低 8 位 type
func mkEnc(cfg int8, typ uint8) uint16 {
	return (uint16(uint8(cfg)) << 8) | uint16(typ)
}

func randBytes(n int) []byte {
	b := make([]byte, n)
	_, _ = rand.Read(b) // cryptorand; 随机性不是重点，忽略 error
	return b
}

func TestMakeEncoding(t *testing.T) {
	require.Equal(t, uint16(0x0000), mkEnc(NoCompression, EncodingPlainText))
	require.Equal(t, uint16(0x0001), mkEnc(NoCompression, EncodingGzip))
	require.Equal(t, uint16(0x0101), mkEnc(BestSpeed, EncodingGzip))
	require.Equal(t, uint16(0xFF02), mkEnc(DefaultCompression, EncodingFlate))
	require.Equal(t, uint16(0x0803), mkEnc(8, EncodingLZW))
	require.Equal(t, uint16(0xFE04), mkEnc(HuffmanOnly, EncodingZlib))
}

func repeatData(s string, n int) []byte {
	var b strings.Builder
	b.Grow(len(s) * n)
	for i := 0; i < n; i++ {
		b.WriteString(s)
	}
	return []byte(b.String())
}

func TestEncodeDecode_Plain_Roundtrip(t *testing.T) {
	src := randBytes(32 * 1024)
	var encBuf, decBuf bytes.Buffer

	err := Encode(mkEnc(0, EncodingPlainText), bytes.NewReader(src), &encBuf)
	require.NoError(t, err)
	require.Equal(t, src, encBuf.Bytes(), "plaintext Encode should be pass-through")

	err = Decode(mkEnc(0, EncodingPlainText), bytes.NewReader(encBuf.Bytes()), &decBuf)
	require.NoError(t, err)
	require.Equal(t, src, decBuf.Bytes())
}

func TestEncodeDecode_EmptyInput_AllEncodings(t *testing.T) {
	types := []uint8{EncodingPlainText, EncodingGzip, EncodingFlate, EncodingLZW, EncodingZlib}
	for _, typ := range types {
		t.Run(fmt.Sprintf("typ=%d", typ), func(t *testing.T) {
			var encBuf, decBuf bytes.Buffer
			err := Encode(mkEnc(DefaultCompression, typ), bytes.NewReader(nil), &encBuf)
			require.NoError(t, err)
			// 解码应回到空
			err = Decode(mkEnc(DefaultCompression, typ), bytes.NewReader(encBuf.Bytes()), &decBuf)
			require.NoError(t, err)
			require.Empty(t, decBuf.Bytes())
		})
	}
}

func TestEncodeDecode_Gzip_Roundtrip_MultipleLevels(t *testing.T) {
	src := repeatData("AAAAAABBBBBBCCCCCCDDDDDD", 20_000) // 高可压缩
	levels := []int8{NoCompression, BestSpeed, DefaultCompression, BestCompression, HuffmanOnly}
	for _, lv := range levels {
		t.Run(fmt.Sprintf("gzip_lv=%d", lv), func(t *testing.T) {
			var encBuf, decBuf bytes.Buffer
			err := Encode(mkEnc(lv, EncodingGzip), bytes.NewReader(src), &encBuf)
			require.NoError(t, err)
			err = Decode(mkEnc(lv, EncodingGzip), bytes.NewReader(encBuf.Bytes()), &decBuf)
			require.NoError(t, err)
			require.Equal(t, src, decBuf.Bytes())
		})
	}
}

func TestEncodeDecode_Zlib_Roundtrip_MultipleLevels(t *testing.T) {
	src := repeatData("XYZXYZXYZXYZ", 50_000)
	levels := []int8{NoCompression, BestSpeed, DefaultCompression, BestCompression, HuffmanOnly}
	for _, lv := range levels {
		t.Run(fmt.Sprintf("zlib_lv=%d", lv), func(t *testing.T) {
			var encBuf, decBuf bytes.Buffer
			err := Encode(mkEnc(lv, EncodingZlib), bytes.NewReader(src), &encBuf)
			require.NoError(t, err)
			err = Decode(mkEnc(lv, EncodingZlib), bytes.NewReader(encBuf.Bytes()), &decBuf)
			require.NoError(t, err)
			require.Equal(t, src, decBuf.Bytes())
		})
	}
}

func TestEncodeDecode_Flate_Roundtrip_MultipleLevels(t *testing.T) {
	src := repeatData("1234567890", 80_000)
	levels := []int8{NoCompression, BestSpeed, DefaultCompression, BestCompression, HuffmanOnly}
	for _, lv := range levels {
		t.Run(fmt.Sprintf("flate_lv=%d", lv), func(t *testing.T) {
			var encBuf, decBuf bytes.Buffer
			err := Encode(mkEnc(lv, EncodingFlate), bytes.NewReader(src), &encBuf)
			require.NoError(t, err)
			err = Decode(mkEnc(lv, EncodingFlate), bytes.NewReader(encBuf.Bytes()), &decBuf)
			require.NoError(t, err)
			require.Equal(t, src, decBuf.Bytes())
		})
	}
}

func TestEncodeDecode_LZW_Roundtrip_LitWidths(t *testing.T) {
	src := repeatData("The quick brown fox jumps over the lazy dog. ", 10_000)

	for _, lw := range []int8{8} {
		t.Run(fmt.Sprintf("lzw_lw=%d", lw), func(t *testing.T) {
			var encBuf, decBuf bytes.Buffer
			err := Encode(mkEnc(lw, EncodingLZW), bytes.NewReader(src), &encBuf)
			require.NoError(t, err)
			err = Decode(mkEnc(lw, EncodingLZW), bytes.NewReader(encBuf.Bytes()), &decBuf)
			require.NoError(t, err)
			require.Equal(t, src, decBuf.Bytes())
		})
	}
}

func TestLZW_InvalidCfg_FallbackTo8(t *testing.T) {
	src := repeatData("HELLOHELLOHELLO_", 50_000)

	var encInvalid, enc8 bytes.Buffer
	require.NoError(t, Encode(mkEnc(1, EncodingLZW), bytes.NewReader(src), &encInvalid)) // 1 无效，应当回退为 8
	require.NoError(t, Encode(mkEnc(8, EncodingLZW), bytes.NewReader(src), &enc8))

	// 由于实现将非法 cfg 归一到 8，编码结果应完全一致
	require.Equal(t, enc8.Bytes(), encInvalid.Bytes())

	// 解码也应一致还原
	var decInvalid, dec8 bytes.Buffer
	require.NoError(t, Decode(mkEnc(1, EncodingLZW), bytes.NewReader(encInvalid.Bytes()), &decInvalid))
	require.NoError(t, Decode(mkEnc(8, EncodingLZW), bytes.NewReader(enc8.Bytes()), &dec8))
	require.Equal(t, src, decInvalid.Bytes())
	require.Equal(t, src, dec8.Bytes())
}

func TestUnsupportedEncodingType(t *testing.T) {
	src := randBytes(1024)
	var encBuf bytes.Buffer
	err := Encode(mkEnc(0, 0xFF), bytes.NewReader(src), &encBuf)
	require.ErrorIs(t, err, ErrEncodingNotSupported)

	var decBuf bytes.Buffer
	err = Decode(mkEnc(0, 0xFF), bytes.NewReader(encBuf.Bytes()), &decBuf)
	require.ErrorIs(t, err, ErrEncodingNotSupported)
}

func TestDecode_CorruptedData_ShouldError(t *testing.T) {
	src := repeatData("data-to-corrupt-", 20_000)

	// 先 gzip 正常编码
	var encBuf bytes.Buffer
	require.NoError(t, Encode(mkEnc(DefaultCompression, EncodingGzip), bytes.NewReader(src), &encBuf))

	// 截断破坏数据
	corrupted := append([]byte(nil), encBuf.Bytes()...)
	if len(corrupted) > 10 {
		corrupted = corrupted[:len(corrupted)-10]
	}

	// 解码应报错
	var decBuf bytes.Buffer
	err := Decode(mkEnc(DefaultCompression, EncodingGzip), bytes.NewReader(corrupted), &decBuf)
	require.Error(t, err)
}

func TestCompressionLevel_Effect_SizeOrdering(t *testing.T) {
	// 高度可压缩数据，确保体积差异明显
	src := repeatData(strings.Repeat("A", 256), 30_000)

	check := func(t *testing.T, typ uint8) {
		var noComp, bestComp bytes.Buffer

		require.NoError(t, Encode(mkEnc(NoCompression, typ), bytes.NewReader(src), &noComp))
		require.NoError(t, Encode(mkEnc(BestCompression, typ), bytes.NewReader(src), &bestComp))

		// 对高度可压缩数据，BestCompression 体积应 < NoCompression
		require.Greater(t, noComp.Len(), bestComp.Len(),
			fmt.Sprintf("type=%d expect BestCompression smaller than NoCompression", typ))
	}

	t.Run("gzip", func(t *testing.T) { check(t, EncodingGzip) })
	t.Run("zlib", func(t *testing.T) { check(t, EncodingZlib) })
	t.Run("flate", func(t *testing.T) { check(t, EncodingFlate) })
}

func TestLargeStream_Roundtrip(t *testing.T) {
	// 生成 ~5MB 的随机数据（混入可压缩模式以降低时间）
	chunks := make([][]byte, 200)
	for i := range chunks {
		if i%3 == 0 {
			// 可压缩块
			chunks[i] = repeatData(strings.Repeat(string(byte(65+mathrand.IntN(6))), 64), 400) // 约 25KB
		} else {
			chunks[i] = randBytes(25 * 1024)
		}
	}
	src := bytes.Join(chunks, nil)

	types := []uint8{EncodingGzip, EncodingZlib, EncodingFlate}
	for _, typ := range types {
		t.Run(fmt.Sprintf("typ=%d", typ), func(t *testing.T) {
			var encBuf, decBuf bytes.Buffer
			require.NoError(t, Encode(mkEnc(DefaultCompression, typ), bytes.NewReader(src), &encBuf))
			require.NoError(t, Decode(mkEnc(DefaultCompression, typ), bytes.NewReader(encBuf.Bytes()), &decBuf))
			require.Equal(t, src, decBuf.Bytes())
		})
	}
}
