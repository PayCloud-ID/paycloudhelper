package phhelper

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const LogModulePrefix = "pchelper"

// BuildLogPrefix builds a standardized log prefix.
// If LogModulePrefix is not empty => [<LogModulePrefix>.<functionName>]
// If LogModulePrefix is empty => [<functionName>]
func BuildLogPrefix(functionName string) string {
	fn := strings.TrimSpace(functionName)
	if fn == "" {
		fn = "Log"
	}

	if LogModulePrefix != "" {
		return fmt.Sprintf("[%s.%s]", LogModulePrefix, fn)
	}

	return fmt.Sprintf("[%s]", fn)
}

func JsonMinify(jsonB []byte) ([]byte, error) {
	var buff *bytes.Buffer = new(bytes.Buffer)
	errCompact := json.Compact(buff, jsonB)
	if errCompact != nil {
		newErr := fmt.Errorf("failure encountered compacting json := %v", errCompact)
		return []byte{}, newErr
	}

	b, err := io.ReadAll(buff)
	if err != nil {
		readErr := fmt.Errorf("read buffer error encountered := %v", err)
		return []byte{}, readErr
	}

	return b, nil
}

func JsonMarshalNoEsc(t interface{}) ([]byte, error) {
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(t)
	return buffer.Bytes(), err
}

func JSONEncode(obj interface{}) string {
	jsonObj, _ := json.MarshalIndent(obj, "", "  ")
	return string(jsonObj)
}

// ToJson Encode json from object to JSON and beautify the output.
func ToJson(data interface{}) string {
	jsonResult, _ := json.Marshal(data)

	return string(jsonResult)
}

// ToJsonIndent Encode json from object to JSON and beautify the output.
func ToJsonIndent(data interface{}) string {
	jsonResult, _ := json.MarshalIndent(data, "", " ")

	return string(jsonResult)
}
