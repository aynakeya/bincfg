package bincfg

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func equalMapBytes(t *testing.T, a, b map[string][]byte) {
	require.Equal(t, len(a), len(b), "map size mismatch")
	for k, va := range a {
		vb, ok := b[k]
		require.True(t, ok, "missing key %q", k)
		require.Equal(t, va, vb, "value mismatch for key %q", k)
	}
}

func TestVarData_RoundTrip_Basic(t *testing.T) {
	t.Parallel()

	in := map[string][]byte{
		"foo": []byte("hello world"),
		"bar": {1, 2, 3, 4},
		"baz": {},
		"中文":  []byte("内容"),
	}

	blob := VarDataDump(in)
	require.NotEmpty(t, blob, "dump should not be empty")

	out, err := VarDataParse(blob)
	require.NoError(t, err)
	equalMapBytes(t, in, out)
}

func TestVarData_DeterministicOutput(t *testing.T) {
	t.Parallel()

	in := map[string][]byte{
		"b": []byte("B"),
		"a": []byte("A"),
		"c": []byte("C"),
	}

	blob1 := VarDataDump(in)
	blob2 := VarDataDump(in)
	require.Equal(t, blob1, blob2, "dump must be deterministic for same input")
}

func TestVarData_ZeroLengthValue(t *testing.T) {
	t.Parallel()

	in := map[string][]byte{
		"empty": {},
		"data":  []byte("x"),
	}

	blob := VarDataDump(in)
	require.NotEmpty(t, blob)

	out, err := VarDataParse(blob)
	require.NoError(t, err)
	equalMapBytes(t, in, out)
}

func TestVarData_Parse_TruncatedBuffer(t *testing.T) {
	t.Parallel()

	in := map[string][]byte{
		"k1": []byte("v1"),
		"k2": []byte("v2"),
	}
	blob := VarDataDump(in)
	require.NotEmpty(t, blob)

	// 截断 1 字节
	trunc := blob[:len(blob)-1]
	_, err := VarDataParse(trunc)
	require.Error(t, err, "parsing truncated buffer must fail")
}

func TestVarData_Parse_BrokenCString(t *testing.T) {
	t.Parallel()

	in := map[string][]byte{
		"foo": []byte("a"),
		"bar": []byte("b"),
	}
	blob := VarDataDump(in)
	require.NotEmpty(t, blob)

	// 找到头部之后（first.Name 即 nameStart）到数据区前的第一个 0 字节（C 字符串终止），把它改成非 0
	// 利用第一个 DataInfo 的 Name 作为 nameStart
	nameStart := int(uint32(blob[0]) |
		uint32(blob[1])<<8 |
		uint32(blob[2])<<16 |
		uint32(blob[3])<<24)

	require.Greater(t, len(blob), nameStart)
	// dataStart = 所有 Offset 的最小值；为了简单，扫描最小的绝对 Offset
	dataStart := len(blob)
	for i := 0; i < nameStart; i += 16 {
		off := int(uint32(blob[i+4]) |
			uint32(blob[i+5])<<8 |
			uint32(blob[i+6])<<16 |
			uint32(blob[i+7])<<24)
		if off < dataStart {
			dataStart = off
		}
	}
	require.Greater(t, dataStart, nameStart)

	bad := append([]byte(nil), blob...) // 拷贝
	// 把字符串表里第一个 0 改成 1
	replaced := false
	for i := nameStart; i < dataStart; i++ {
		if bad[i] == 0 {
			bad[i] = 1
			replaced = true
			break
		}
	}
	require.True(t, replaced, "should find a NUL in name table")

	_, err := VarDataParse(bad)
	require.Error(t, err, "broken c-string should fail parsing")
}

func TestVarData_Parse_BadAlignment(t *testing.T) {
	t.Parallel()

	in := map[string][]byte{
		"k": []byte("v"),
	}
	blob := VarDataDump(in)
	require.NotEmpty(t, blob)

	// 破坏第一个 Name 的 16 字节对齐（把低位改成 1）
	bad := append([]byte(nil), blob...)
	bad[0] = 1 // 使 first.Name%16 != 0
	_, err := VarDataParse(bad)
	require.Error(t, err, "invalid first name offset should fail")
}

func TestVarData_EmptyInputDump(t *testing.T) {
	t.Parallel()

	in := map[string][]byte{}
	blob := VarDataDump(in)
	// 你当前的实现对空输入会返回空切片；解析应失败（没有头部）
	require.Equal(t, 0, len(blob))

	_, err := VarDataParse(blob)
	require.Error(t, err)
}

// 可选：验证解析出的切片内容是只读视角（引用原缓冲）。
// 这里只测试不会 panic；是否“零拷贝”取决于你的实现需求。
func TestVarData_Parse_SlicesReferToBlob(t *testing.T) {
	t.Parallel()

	in := map[string][]byte{
		"k1": []byte("abc"),
		"k2": []byte("def"),
	}
	blob := VarDataDump(in)
	out, err := VarDataParse(blob)
	require.NoError(t, err)

	// 取一个值并做拷贝对比
	got := out["k1"]
	require.True(t, bytes.Equal(got, []byte("abc")))
}
