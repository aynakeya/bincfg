package bincfg

import (
	"github.com/inconshreveable/go-update"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
)

func binaryPath() (string, error) {
	p, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(p)
}

func ReadRaw() ([]byte, error) {
	self, err := binaryPath()
	if err != nil {
		return nil, err
	}
	f, err := os.Open(self)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	header, err := ReadHeader(f)
	if err != nil {
		return nil, err
	}

	payload := make([]byte, header.Len)
	err = header.ReadPayload(f, payload)

	return payload, err
}

func WriteRaw(payload []byte) error {
	self, err := binaryPath()
	if err != nil {
		return err
	}
	tmpDir := filepath.Dir(self)

	var h Header
	copy(h.Magic[:], []byte(magic))
	h.Version = version
	h.Len = uint32(len(payload))
	h.CRC32 = crc32.ChecksumIEEE(payload)

	trailer := make([]byte, headerSize)

	h.Dump(trailer)
	
	src, err := os.Open(self)
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

	tmp, err := os.CreateTemp(tmpDir, filepath.Base(self)+".tmp-*")
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
		TargetPath: self,
		TargetMode: srcStat.Mode(),
	})
}
