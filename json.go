package bincfg

import "encoding/json"

// ReadJSON 读取并反序列化为 v
func ReadJSON(v any) error {
	b, err := ReadRaw()
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

// WriteJSON 将对象写入为 JSON，并保存进二进制（内部调用 WriteRaw）
func WriteJSON(v any, pretty bool) error {
	var b []byte
	var err error
	if pretty {
		b, err = json.MarshalIndent(v, "", "  ")
	} else {
		b, err = json.Marshal(v)
	}
	if err != nil {
		return err
	}
	return WriteRaw(b)
}
