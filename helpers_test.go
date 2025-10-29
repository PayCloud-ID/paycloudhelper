package paycloudhelper

import (
	"testing"
)

// TestJsonMinify tests JSON minification
func TestJsonMinify(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:    "valid json with spaces",
			input:   `{ "key" : "value" }`,
			want:    `{"key":"value"}`,
			wantErr: false,
		},
		{
			name:    "valid json with newlines",
			input:   "{\n  \"key\": \"value\"\n}",
			want:    `{"key":"value"}`,
			wantErr: false,
		},
		{
			name:    "valid json with nested objects",
			input:   `{ "outer": { "inner": "value" } }`,
			want:    `{"outer":{"inner":"value"}}`,
			wantErr: false,
		},
		{
			name:    "invalid json",
			input:   `{ invalid json }`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := JsonMinify([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("JsonMinify() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && string(got) != tt.want {
				t.Errorf("JsonMinify() = %s, want %s", string(got), tt.want)
			}
		})
	}
}

// TestJsonMarshalNoEsc tests JSON marshaling without HTML escaping
func TestJsonMarshalNoEsc(t *testing.T) {
	tests := []struct {
		name    string
		input   interface{}
		wantErr bool
		check   func([]byte) bool
	}{
		{
			name:  "simple object",
			input: map[string]string{"key": "value"},
			check: func(b []byte) bool {
				return string(b) == `{"key":"value"}`+"\n"
			},
		},
		{
			name:  "html special chars not escaped",
			input: map[string]string{"key": "<script>alert('xss')</script>"},
			check: func(b []byte) bool {
				// Should NOT escape < and >
				return string(b) == `{"key":"<script>alert('xss')</script>"}`+"\n"
			},
		},
		{
			name:    "nil input",
			input:   nil,
			wantErr: false,
			check: func(b []byte) bool {
				return string(b) == "null\n"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := jsonMarshalNoEsc(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("jsonMarshalNoEsc() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.check(got) {
				t.Errorf("jsonMarshalNoEsc() = %v, check failed", string(got))
			}
		})
	}
}

// TestJSONEncode tests JSON encoding with indent
func TestJSONEncode(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		check func(string) bool
	}{
		{
			name:  "simple object",
			input: map[string]string{"key": "value"},
			check: func(s string) bool {
				return len(s) > 0 && s[0] == '{'
			},
		},
		{
			name:  "array",
			input: []string{"a", "b", "c"},
			check: func(s string) bool {
				return len(s) > 0 && s[0] == '['
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := JSONEncode(tt.input)
			if !tt.check(got) {
				t.Errorf("JSONEncode() = %s, check failed", got)
			}
		})
	}
}

// TestToJson tests JSON marshaling
func TestToJson(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		check func(string) bool
	}{
		{
			name:  "simple string",
			input: "hello",
			check: func(s string) bool {
				return s == `"hello"`
			},
		},
		{
			name:  "number",
			input: 42,
			check: func(s string) bool {
				return s == "42"
			},
		},
		{
			name:  "object",
			input: map[string]int{"count": 5},
			check: func(s string) bool {
				return len(s) > 0
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToJson(tt.input)
			if !tt.check(got) {
				t.Errorf("ToJson() = %s, check failed", got)
			}
		})
	}
}

// TestToJsonIndent tests JSON marshaling with indentation
func TestToJsonIndent(t *testing.T) {
	input := map[string]interface{}{
		"key":  "value",
		"nest": map[string]string{"inner": "data"},
	}

	got := ToJsonIndent(input)

	// Should contain indentation
	if len(got) == 0 || got[0] != '{' {
		t.Errorf("ToJsonIndent() = %s, expected indented JSON", got)
	}

	// Should have newlines (indentation)
	hasNewline := false
	for _, c := range got {
		if c == '\n' {
			hasNewline = true
			break
		}
	}
	if !hasNewline {
		t.Errorf("ToJsonIndent() should have indentation with newlines")
	}
}
