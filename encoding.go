package bincfg

import (
	"compress/flate"
	"compress/gzip"
	"compress/lzw"
	"compress/zlib"
	"errors"
	"github.com/klauspost/compress/zstd"
	"io"
)

var (
	ErrEncodingNotSupported = errors.New("bincfg: encoding not supported")
)

const (
	NoCompression      int8 = flate.NoCompression
	BestSpeed          int8 = flate.BestSpeed
	BestCompression    int8 = flate.BestCompression
	DefaultCompression int8 = flate.DefaultCompression
	HuffmanOnly        int8 = flate.HuffmanOnly
)

const (
	EncodingPlainText uint8 = iota
	// EncodingGzip option: NoCompression, BestSpeed, BestCompression, HuffmanOnly, DefaultCompression
	EncodingGzip
	// EncodingFlate option: NoCompression, BestSpeed, BestCompression, HuffmanOnly, DefaultCompression
	EncodingFlate
	// EncodingLZW option: LSB or MSB (fixed to LSB here), and literal width (2 to 8)
	EncodingLZW
	// EncodingZlib option: NoCompression, BestSpeed, BestCompression, HuffmanOnly, DefaultCompression
	EncodingZlib
	// EncodingZstd option: 1 = zstd.SpeedFastest,  2 = zstd.SpeedDefault,
	// 3 = zstd.SpeedBetterCompression, 4 = zstd.SpeedBestCompression
	EncodingZstd
)

var encodingRegistry = map[uint8]Encoder{
	EncodingPlainText: plainTextEncoding{},
	EncodingGzip:      gzipEncoding{},
	EncodingFlate:     flateEncoding{},
	EncodingLZW:       lzwEncoding{},
	EncodingZlib:      zlibEncoding{},
	EncodingZstd:      zstdEncoding{},
}

func Encoding(encType uint8, encCfg int8) uint16 {
	return uint16(uint8(encCfg))<<8 | uint16(encType)
}

func Encode(encoding uint16, src io.Reader, writer io.Writer) error {
	encType := uint8(encoding & 0x00FF)
	encCfg := int8((encoding & 0xFF00) >> 8)
	enc, ok := encodingRegistry[encType]
	if !ok {
		return ErrEncodingNotSupported
	}
	return enc.Encode(encCfg, src, writer)
}

func Decode(encoding uint16, src io.Reader, writer io.Writer) error {
	encType := uint8(encoding & 0x00FF)
	encCfg := int8((encoding & 0xFF00) >> 8)
	enc, ok := encodingRegistry[encType]
	if !ok {
		return ErrEncodingNotSupported
	}
	return enc.Decode(encCfg, src, writer)
}

type Encoder interface {
	Encode(cfg int8, src io.Reader, writer io.Writer) error
	Decode(cfg int8, src io.Reader, writer io.Writer) error
}

type plainTextEncoding struct{}

func (p plainTextEncoding) Encode(cfg int8, src io.Reader, writer io.Writer) error {
	_, err := io.Copy(writer, src)
	return err
}

func (p plainTextEncoding) Decode(cfg int8, src io.Reader, writer io.Writer) error {
	_, err := io.Copy(writer, src)
	return err
}

type gzipEncoding struct{}

func (g gzipEncoding) Encode(cfg int8, src io.Reader, writer io.Writer) error {
	zw, err := gzip.NewWriterLevel(writer, int(cfg))
	if err != nil {
		return err
	}
	if _, err = io.Copy(zw, src); err != nil {
		_ = zw.Close()
		return err
	}
	if err = zw.Close(); err != nil {
		return err
	}
	return nil
}

func (g gzipEncoding) Decode(cfg int8, src io.Reader, writer io.Writer) error {
	zr, err := gzip.NewReader(src)
	if err != nil {
		return err
	}
	if _, err = io.Copy(writer, zr); err != nil {
		return err
	}
	return zr.Close()
}

type flateEncoding struct{}

func (f flateEncoding) Encode(cfg int8, src io.Reader, writer io.Writer) error {
	zw, err := flate.NewWriter(writer, int(cfg))
	if err != nil {
		return err
	}
	if _, err = io.Copy(zw, src); err != nil {
		_ = zw.Close()
		return err
	}
	return zw.Close()
}

func (f flateEncoding) Decode(cfg int8, src io.Reader, writer io.Writer) error {
	zr := flate.NewReader(src)
	if _, err := io.Copy(writer, zr); err != nil {
		return err
	}
	return zr.Close()
}

type lzwEncoding struct{}

func (l lzwEncoding) Encode(cfg int8, src io.Reader, writer io.Writer) error {
	lw := int(cfg)
	if lw < 2 || lw > 8 {
		lw = 8
	}
	zw := lzw.NewWriter(writer, lzw.LSB, lw)
	if _, err := io.Copy(zw, src); err != nil {
		_ = zw.Close()
		return err
	}
	return zw.Close()
}

func (l lzwEncoding) Decode(cfg int8, src io.Reader, writer io.Writer) error {
	lw := int(cfg)
	if lw < 2 || lw > 8 {
		lw = 8
	}
	zr := lzw.NewReader(src, lzw.LSB, lw)
	if _, err := io.Copy(writer, zr); err != nil {
		return err
	}
	return zr.Close()
}

type zlibEncoding struct{}

func (z zlibEncoding) Encode(cfg int8, src io.Reader, writer io.Writer) error {
	zw, err := zlib.NewWriterLevel(writer, int(cfg))
	if err != nil {
		return err
	}
	if _, err = io.Copy(zw, src); err != nil {
		_ = zw.Close()
		return err
	}
	return zw.Close()
}

func (z zlibEncoding) Decode(cfg int8, src io.Reader, writer io.Writer) error {
	zr, err := zlib.NewReader(src)
	if err != nil {
		return err
	}
	if _, err = io.Copy(writer, zr); err != nil {
		return err
	}
	return zr.Close()
}

type zstdEncoding struct{}

func (z zstdEncoding) Encode(cfg int8, src io.Reader, w io.Writer) error {
	zw, err := zstd.NewWriter(w, zstd.WithEncoderLevel(zstd.EncoderLevelFromZstd(int(cfg))))
	if err != nil {
		return err
	}
	if _, err = io.Copy(zw, src); err != nil {
		_ = zw.Close()
		return err
	}
	return zw.Close()
}

func (z zstdEncoding) Decode(cfg int8, r io.Reader, w io.Writer) error {
	zr, err := zstd.NewReader(r)
	if err != nil {
		return err
	}
	if _, err = io.Copy(w, zr); err != nil {
		return err
	}
	zr.Close()
	return nil
}
