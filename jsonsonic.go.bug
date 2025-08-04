package paycloudhelper

import (
	"github.com/bytedance/sonic"
)

// MarshalJSON encodes the given data into a JSON byte slice using sonic.
func MarshalJSON(v interface{}) ([]byte, error) {
	return sonic.Marshal(v)
}

// UnmarshalJSON decodes the given JSON byte slice into the given interface using sonic.
func UnmarshalJSON(data []byte, v interface{}) error {
	return sonic.Unmarshal(data, v)
}
