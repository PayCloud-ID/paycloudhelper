package phjson

import (
	"github.com/bytedance/sonic"
)

var (
	// ConfigDefault is the default config of APIs, aiming at efficiency and safety.
	ConfigDefault sonic.API
)

// NewConfig initializes the default configuration for sonic APIs.
func NewConfig(config *sonic.Config) {
	if config == nil {
		rConfig := sonic.Config{}
		config = &rConfig
	}
	ConfigDefault = config.Froze()
}

// GetConfig returns the default configuration for sonic APIs.
func GetConfig() sonic.API {
	if ConfigDefault == nil {
		rConfig := sonic.Config{}
		ConfigDefault = rConfig.Froze()
	}
	return ConfigDefault
}

// Marshal encodes the given data into a JSON byte slice using sonic.
func Marshal(v interface{}) ([]byte, error) {
	return GetConfig().Marshal(v)
}

// Unmarshal decodes the given JSON byte slice into the given interface using sonic.
func Unmarshal(data []byte, v interface{}) error {
	return GetConfig().Unmarshal(data, v)
}

// MarshalIndent encodes the given data into a JSON byte slice using sonic, with indentation.
func MarshalIndent(v interface{}, prefix, indent string) ([]byte, error) {
	return GetConfig().MarshalIndent(v, prefix, indent)
}
