package paycloudhelper

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

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

func jsonMarshalNoEsc(t interface{}) ([]byte, error) {
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
