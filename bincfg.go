package bincfg

import (
	"bytes"
	"github.com/inconshreveable/go-update"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
)

func BinaryPath() (string, error) {
	p, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(p)
}

func Self() string {
	p, _ := BinaryPath()
	return p
}

func ReadRaw(path string) ([]byte, error) {
	if path == "" {
		return nil, ErrNotFound
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	header, err := ReadHeader(f)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	err = header.ReadPayload(f, &buf)

	return buf.Bytes(), err
}

func WriteRaw(path string, payload []byte, encoding uint16) error {
	if path == "" {
		return ErrNotFound
	}
	tmpDir := filepath.Dir(path)
	var err error
	var encodedPayload bytes.Buffer

	err = Encode(encoding, bytes.NewReader(payload), &encodedPayload)
	if err != nil {
		return err
	}
	payload = encodedPayload.Bytes()

	var h Header
	copy(h.Magic[:], []byte(magic))
	h.Version = version
	h.Encoding = encoding
	h.Len = uint32(len(payload))
	h.CRC32 = crc32.ChecksumIEEE(payload)

	trailer := make([]byte, headerSize)

	h.Dump(trailer)

	src, err := os.Open(path)
	if err != nil {
		return err
	}

	srcStat, err := src.Stat()
	if err != nil {
		return err
	}

	var baseSize int64
	baseSize = srcStat.Size()

	originHeader, err := ReadHeader(src)
	if err == nil {
		baseSize = baseSize - headerSize - int64(originHeader.Len)
	}

	tmp, err := os.CreateTemp(tmpDir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmp.Chmod(srcStat.Mode())
	tmpName := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName) // cleanup
	}()

	if _, err = io.CopyN(tmp, src, baseSize); err != nil {
		return err
	}
	_ = src.Close()
	if len(payload) > 0 {
		if _, err = tmp.Write(payload); err != nil {
			return err
		}
	}
	if _, err = tmp.Write(trailer); err != nil {
		return err
	}
	_, _ = tmp.Seek(0, io.SeekStart)
	return update.Apply(tmp, update.Options{
		TargetPath: path,
		TargetMode: srcStat.Mode(),
	})
}
