package bincfg

import (
	"bytes"
	"encoding/binary"
	"sort"
)

type DataInfo struct {
	Name    uint32 // c string offset
	Offset  uint32 // data offset
	Length  uint32 // data length
	padding uint32
}

func DumpData(datas map[string][]byte) []byte {
	cnt := len(datas)
	keys := make([]string, 0, cnt)
	for k := range datas {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var (
		nameBuf bytes.Buffer
		dataBuf bytes.Buffer
		entries = make([]DataInfo, 0, cnt)
	)

	nameOffset := uint32(16 * cnt)

	for _, key := range keys {
		value := datas[key]

		namePos := uint32(nameBuf.Len())
		nameBuf.WriteString(key)
		nameBuf.WriteByte(0)

		offset := uint32(dataBuf.Len())
		dataBuf.Write(value)

		entries = append(entries, DataInfo{
			Name:    nameOffset + namePos,
			Offset:  nameOffset + uint32(nameBuf.Len()) + offset,
			Length:  uint32(len(value)),
			padding: 0,
		})
	}

	dataStart := nameOffset + uint32(nameBuf.Len())

	buf := new(bytes.Buffer)
	for i, e := range entries {
		e.Offset = dataStart + (entries[i].Offset - (nameOffset + uint32(nameBuf.Len())))
		_ = binary.Write(buf, binary.LittleEndian, e)
	}

	buf.Write(nameBuf.Bytes())
	buf.Write(dataBuf.Bytes())

	return buf.Bytes()
}
