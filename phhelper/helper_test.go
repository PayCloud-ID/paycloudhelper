package phhelper

import (
	"testing"
)

func TestJsonMinify(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"valid compact", `{"a":1}`, `{"a":1}`, false},
		{"valid with spaces", `{ "a" : 1 }`, `{"a":1}`, false},
		{"invalid", `{ invalid }`, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := JsonMinify([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("JsonMinify() err = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && string(got) != tt.want {
				t.Errorf("JsonMinify() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestJsonMarshalNoEsc(t *testing.T) {
	got, err := JsonMarshalNoEsc(map[string]string{"x": "<script>"})
	if err != nil {
		t.Fatalf("JsonMarshalNoEsc() err = %v", err)
	}
	if string(got) != `{"x":"<script>"}`+"\n" {
		t.Errorf("JsonMarshalNoEsc() = %q, want JSON without HTML escape", string(got))
	}
}

func TestJSONEncode(t *testing.T) {
	s := JSONEncode(map[string]int{"a": 1})
	if len(s) == 0 || s[0] != '{' {
		t.Errorf("JSONEncode() = %q", s)
	}
}

func TestToJson(t *testing.T) {
	if got := ToJson("hello"); got != `"hello"` {
		t.Errorf("ToJson(hello) = %s", got)
	}
	if got := ToJson(42); got != "42" {
		t.Errorf("ToJson(42) = %s", got)
	}
}

func TestToJsonIndent(t *testing.T) {
	s := ToJsonIndent(map[string]string{"k": "v"})
	if len(s) == 0 || s[0] != '{' {
		t.Errorf("ToJsonIndent() = %q", s)
	}
	hasNewline := false
	for _, c := range s {
		if c == '\n' {
			hasNewline = true
			break
		}
	}
	if !hasNewline {
		t.Error("ToJsonIndent() should produce newlines")
	}
}
