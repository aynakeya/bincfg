package bincfg

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"
	"os"
)

const (
	magic = "\x07\x21\xbc\xf9" // 0721 bcfg
	// current version
	version = uint16(0x0001)
	// = int8(cfg) << 8 | uint8(type). lower byte for type, higher byte for options
	encoding = uint16(0x0000)
)

const (
	offsetMagic    = 0
	offsetVersion  = offsetMagic + magicSize
	offsetEncoding = offsetVersion + 2
	offsetLen      = offsetEncoding + 2
	offsetCRC32    = offsetLen + 4
)

// headerSize = magic(4) + ver(2) + encoding(2) + len(4) + crc32(4) = 16
const (
	magicSize  = int64(len(magic))
	headerSize = offsetCRC32 + 4
)

var (
	ErrNotFound       = errors.New("bincfg: no config trailer found")
	ErrCorrupted      = errors.New("bincfg: trailer corrupted (crc or size)")
	ErrPendingReplace = errors.New("bincfg: update written to .new; finalize on next start")
)

type Header struct {
	Magic    [magicSize]byte
	Version  uint16
	Encoding uint16
	Len      uint32
	CRC32    uint32
}

func ParseHeader(b [headerSize]byte) Header {
	return Header{
		Magic:    [magicSize]byte(b[offsetMagic:magicSize]),
		Version:  binary.LittleEndian.Uint16(b[offsetVersion:offsetEncoding]),
		Encoding: binary.LittleEndian.Uint16(b[offsetEncoding:offsetLen]),
		Len:      binary.LittleEndian.Uint32(b[offsetLen:offsetCRC32]),
		CRC32:    binary.LittleEndian.Uint32(b[offsetCRC32:headerSize]),
	}
}

func (h Header) Dump(b []byte) {
	copy(b[offsetMagic:magicSize], h.Magic[:])
	binary.LittleEndian.PutUint16(b[offsetVersion:offsetEncoding], h.Version)
	binary.LittleEndian.PutUint16(b[offsetEncoding:offsetLen], h.Encoding)
	binary.LittleEndian.PutUint32(b[offsetLen:offsetCRC32], h.Len)
	binary.LittleEndian.PutUint32(b[offsetCRC32:headerSize], h.CRC32)
}

// Valid check if header is valid
func (h Header) Valid() bool {
	return string(h.Magic[:]) == magic && h.Version == version
}

func (h Header) PayloadOffset(fileSize int64) int64 {
	return fileSize - headerSize - int64(h.Len)
}

// Size returns total size occupied by header and payload
func (h Header) Size() int64 {
	return headerSize + int64(h.Len)
}

func (h Header) ReadPayload(f *os.File, buf io.Writer) error {
	st, err := f.Stat()
	if err != nil {
		return err
	}
	payload := make([]byte, h.Len)
	n, err := f.ReadAt(payload, h.PayloadOffset(st.Size()))
	if err != nil {
		return err
	}
	if n != int(h.Len) {
		return ErrCorrupted
	}
	if crc32.ChecksumIEEE(payload) != h.CRC32 {
		return ErrCorrupted
	}
	err = Decode(h.Encoding, bytes.NewReader(payload), buf)
	if err != nil {
		return err
	}
	return nil
}

func ReadHeader(f *os.File) (header Header, err error) {
	st, err := f.Stat()
	if err != nil {
		return header, err
	}
	if st.Size() < headerSize {
		return header, ErrNotFound
	}

	hb := make([]byte, headerSize)
	n, err := f.ReadAt(hb, st.Size()-headerSize)
	if err != nil {
		return header, err
	}
	if int64(n) != headerSize {
		return header, ErrNotFound
	}
	header = ParseHeader([headerSize]byte(hb))

	if !header.Valid() {
		return header, ErrNotFound
	}
	payloadOffset := header.PayloadOffset(st.Size())
	if payloadOffset < 0 {
		return header, ErrCorrupted
	}
	return header, nil
}
