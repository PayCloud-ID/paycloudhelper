package paycloudhelper

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestConfigureAuditJSONMarshal_custom(t *testing.T) {
	t.Cleanup(func() { ConfigureAuditJSONMarshal(nil) })

	ConfigureAuditJSONMarshal(func(v interface{}) ([]byte, error) {
		return json.Marshal(v)
	})

	got, err := jsonMarshalNoEsc(map[string]int{"a": 1})
	if err != nil {
		t.Fatalf("jsonMarshalNoEsc: %v", err)
	}
	want, err := json.Marshal(map[string]int{"a": 1})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestConfigureAuditJSONMarshal_nilRestoresEncoderTrailingNewline(t *testing.T) {
	ConfigureAuditJSONMarshal(func(v interface{}) ([]byte, error) {
		return json.Marshal(v)
	})
	ConfigureAuditJSONMarshal(nil)

	got, err := jsonMarshalNoEsc(map[string]string{"k": "v"})
	if err != nil {
		t.Fatalf("jsonMarshalNoEsc: %v", err)
	}
	want := "{\"k\":\"v\"}\n"
	if string(got) != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
