package phjson

import (
	"github.com/bytedance/sonic"
)

// Marshal encodes the given data into a JSON byte slice using sonic.
func Marshal(v interface{}) ([]byte, error) {
	return sonic.Marshal(v)
}

// Unmarshal decodes the given JSON byte slice into the given interface using sonic.
func Unmarshal(data []byte, v interface{}) error {
	return sonic.Unmarshal(data, v)
}

// MarshalIndent encodes the given data into a JSON byte slice using sonic, with indentation.
func MarshalIndent(v interface{}, prefix, indent string) ([]byte, error) {
	return sonic.MarshalIndent(v, prefix, indent)
}
