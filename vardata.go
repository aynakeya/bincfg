package bincfg

import (
	"bytes"
	"encoding/binary"
	"errors"
	"sort"
)

type VarData struct {
	Name    uint32 // c string offset
	Offset  uint32 // data offset
	Length  uint32 // data length
	padding uint32 // reserved, for future usage
}

func BuildVarDataFromBytes(data [16]byte) VarData {
	return VarData{
		Name:    binary.LittleEndian.Uint32(data[0:4]),
		Offset:  binary.LittleEndian.Uint32(data[4:8]),
		Length:  binary.LittleEndian.Uint32(data[8:12]),
		padding: binary.LittleEndian.Uint32(data[12:16]),
	}
}

func VarDataParse(b []byte) (map[string][]byte, error) {
	if len(b) < 16 {
		return nil, errors.New("buffer too small")
	}

	first := BuildVarDataFromBytes([16]byte(b[0:16]))
	if first.Name%16 != 0 {
		return nil, errors.New("invalid first name offset (not aligned by 16)")
	}
	cnt := int(first.Name / 16)
	if cnt <= 0 || 16*cnt > len(b) {
		return nil, errors.New("invalid entry count inferred")
	}

	infos := make([]VarData, cnt)
	for i := 0; i < cnt; i++ {
		base := i * 16
		infos[i] = BuildVarDataFromBytes([16]byte(b[base : base+16]))
		if i == 0 {
			continue
		}
		if infos[i].Name < infos[i-1].Name {
			return nil, errors.New("name offsets not sorted")
		}
	}

	nameStart := uint32(16 * cnt)

	dataStart := ^uint32(0)
	for _, di := range infos {
		if di.Offset < dataStart {
			dataStart = di.Offset
		}
	}
	if dataStart < nameStart || int(dataStart) > len(b) {
		return nil, errors.New("invalid dataStart")
	}
	nameEnd := dataStart

	out := make(map[string][]byte, cnt)
	seenNamePos := make(map[uint32]struct{}, cnt)

	for idx, di := range infos {
		if di.Name < nameStart || di.Name >= nameEnd {
			return nil, errors.New("name offset out of name table")
		}
		if _, dup := seenNamePos[di.Name]; dup {
			return nil, errors.New("duplicate name pointer")
		}
		seenNamePos[di.Name] = struct{}{}

		nextEnd := nameEnd
		if idx+1 < len(infos) {
			nextEnd = infos[idx+1].Name
		}

		key, ok := readCString(b, int(di.Name), int(nextEnd))
		if !ok || key == "" {
			return nil, errors.New("invalid c-string")
		}

		start := int(di.Offset)
		end := start + int(di.Length)
		if start < int(dataStart) || end < start || end > len(b) {
			return nil, errors.New("data slice out of bounds")
		}
		if _, exists := out[key]; exists {
			return nil, errors.New("duplicate key")
		}
		out[key] = b[start:end]
	}

	return out, nil
}

func readCString(b []byte, pos, limit int) (string, bool) {
	if pos < 0 || pos >= limit || limit > len(b) {
		return "", false
	}
	i := pos
	for i < limit && b[i] != 0 {
		i++
	}
	if i >= limit { // 未遇到 NUL
		return "", false
	}
	return string(b[pos:i]), true
}

func VarDataDump(datas map[string][]byte) []byte {
	cnt := len(datas)
	keys := make([]string, 0, cnt)
	for k := range datas {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var (
		nameBuf bytes.Buffer
		dataBuf bytes.Buffer
		entries = make([]VarData, 0, cnt)
	)

	nameOffset := uint32(16 * cnt)

	for _, key := range keys {
		value := datas[key]

		namePos := uint32(nameBuf.Len())
		nameBuf.WriteString(key)
		nameBuf.WriteByte(0)

		offset := uint32(dataBuf.Len())
		dataBuf.Write(value)

		entries = append(entries, VarData{
			Name:    nameOffset + namePos,
			Offset:  offset,
			Length:  uint32(len(value)),
			padding: 0,
		})
	}

	dataStart := nameOffset + uint32(nameBuf.Len())

	buf := new(bytes.Buffer)
	for _, e := range entries {
		e.Offset = dataStart + e.Offset
		_ = binary.Write(buf, binary.LittleEndian, e)
	}

	buf.Write(nameBuf.Bytes())
	buf.Write(dataBuf.Bytes())

	return buf.Bytes()
}
