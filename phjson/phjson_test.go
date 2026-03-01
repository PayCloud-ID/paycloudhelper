package phjson

import (
	"testing"
)

func TestGetConfig_Default(t *testing.T) {
	cfg := GetConfig()
	if cfg == nil {
		t.Fatal("GetConfig() returned nil")
	}
}

func TestNewConfig(t *testing.T) {
	NewConfig(nil)
	cfg := GetConfig()
	if cfg == nil {
		t.Fatal("GetConfig() after NewConfig(nil) returned nil")
	}
}

func TestMarshal(t *testing.T) {
	data := map[string]int{"a": 1, "b": 2}
	b, err := Marshal(data)
	if err != nil {
		t.Fatalf("Marshal() err = %v", err)
	}
	if len(b) == 0 {
		t.Error("Marshal() returned empty bytes")
	}
}

func TestUnmarshal(t *testing.T) {
	var out map[string]int
	err := Unmarshal([]byte(`{"a":1,"b":2}`), &out)
	if err != nil {
		t.Fatalf("Unmarshal() err = %v", err)
	}
	if out["a"] != 1 || out["b"] != 2 {
		t.Errorf("Unmarshal() = %v", out)
	}
}

func TestUnmarshal_InvalidJSON(t *testing.T) {
	var out map[string]int
	err := Unmarshal([]byte(`{invalid}`), &out)
	if err == nil {
		t.Error("Unmarshal() expected error for invalid JSON")
	}
}

func TestMarshalIndent(t *testing.T) {
	b, err := MarshalIndent(map[string]string{"k": "v"}, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent() err = %v", err)
	}
	if len(b) == 0 || b[0] != '{' {
		t.Errorf("MarshalIndent() = %s", b)
	}
}
