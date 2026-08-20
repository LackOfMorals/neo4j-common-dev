package utils

import (
	"encoding/base64"
	"encoding/json"
)

func Base64Encode(s1, s2 string) string {
	return base64.StdEncoding.EncodeToString([]byte(s1 + ":" + s2))
}

func Unmarshal[T any](data []byte) (T, error) {
	var v T
	err := json.Unmarshal(data, &v)
	return v, err
}

func Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}
