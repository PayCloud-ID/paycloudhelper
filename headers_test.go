package paycloudhelper

import (
	"regexp"
	"testing"
)

// TestGenerateRequestID tests request ID generation
func TestGenerateRequestID(t *testing.T) {
	tests := []struct {
		name string
		test func(string) bool
	}{
		{
			name: "generates non-empty ID",
			test: func(id string) bool {
				return len(id) > 0
			},
		},
		{
			name: "generates unique IDs",
			test: func(id string) bool {
				id2 := generateRequestID()
				return id != id2
			},
		},
		{
			name: "generates hex format (32 chars or timestamp)",
			test: func(id string) bool {
				// Should be either 32 hex chars or numeric timestamp
				if len(id) == 32 {
					// Hex format
					matched, _ := regexp.MatchString(`^[a-f0-9]{32}$`, id)
					return matched
				}
				// Numeric format (fallback)
				matched, _ := regexp.MatchString(`^\d{18,19}$`, id)
				return matched
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := generateRequestID()
			if !tt.test(id) {
				t.Errorf("generateRequestID() = %s, test failed", id)
			}
		})
	}
}

// TestGetOrGenerateRequestID tests request ID retrieval or generation
func TestGetOrGenerateRequestID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		check    func(string) bool
	}{
		{
			name:     "returns provided ID",
			input:    "custom-request-id-123",
			expected: "custom-request-id-123",
			check: func(s string) bool {
				return s == "custom-request-id-123"
			},
		},
		{
			name:     "generates new ID if empty",
			input:    "",
			expected: "",
			check: func(s string) bool {
				return len(s) > 0 && s != ""
			},
		},
		{
			name:     "generates new ID if whitespace",
			input:    "   ",
			expected: "",
			check: func(s string) bool {
				return len(s) > 0
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetOrGenerateRequestID(tt.input)
			if tt.expected != "" {
				if got != tt.expected {
					t.Errorf("GetOrGenerateRequestID(%q) = %q, want %q", tt.input, got, tt.expected)
				}
			} else {
				if !tt.check(got) {
					t.Errorf("GetOrGenerateRequestID(%q) = %q, check failed", tt.input, got)
				}
			}
		})
	}
}

// TestHeadersValidationIdem tests idempotency header validation
func TestHeadersValidationIdem(t *testing.T) {
	tests := []struct {
		name      string
		headers   Headers
		wantError bool
	}{
		{
			name: "valid headers",
			headers: Headers{
				IdempotencyKey: "valid-key-123",
				Session:        "9",
			},
			wantError: false,
		},
		{
			name: "empty idempotency key",
			headers: Headers{
				IdempotencyKey: "",
				Session:        "9",
			},
			wantError: true,
		},
		{
			name: "missing session defaults to valid",
			headers: Headers{
				IdempotencyKey: "valid-key-123",
				Session:        "",
			},
			wantError: false, // Session is optional and gets default
		},
		{
			name: "non-numeric session",
			headers: Headers{
				IdempotencyKey: "valid-key-123",
				Session:        "not-a-number",
			},
			wantError: true,
		},
		{
			name: "key with invalid characters",
			headers: Headers{
				IdempotencyKey: "key@#$%",
				Session:        "9",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.headers.ValiadateHeaderIdem()
			hasError := result != nil
			if hasError != tt.wantError {
				t.Errorf("ValiadateHeaderIdem() error = %v, wantError %v, result %v", hasError, tt.wantError, result)
			}
		})
	}
}

// TestHeadersValidationCsrf tests CSRF header validation
func TestHeadersValidationCsrf(t *testing.T) {
	tests := []struct {
		name      string
		headers   Headers
		wantError bool
	}{
		{
			name: "valid CSRF token",
			headers: Headers{
				Csrf: "valid-csrf-token-abc123",
			},
			wantError: false,
		},
		{
			name: "empty CSRF token",
			headers: Headers{
				Csrf: "",
			},
			wantError: true,
		},
		{
			name: "CSRF token too long",
			headers: Headers{
				Csrf: "a" + string(make([]byte, 100)), // Exceeds max length
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.headers.ValiadateHeaderCsrf()
			hasError := result != nil
			if hasError != tt.wantError {
				t.Errorf("ValiadateHeaderCsrf() error = %v, wantError %v", hasError, tt.wantError)
			}
		})
	}
}

// TestHeadersStructure tests Headers struct fields
func TestHeadersStructure(t *testing.T) {
	header := &Headers{
		IdempotencyKey: "idem-123",
		Session:        "9",
		Csrf:           "csrf-abc",
		RequestID:      "req-xyz",
	}

	if header.IdempotencyKey != "idem-123" {
		t.Errorf("IdempotencyKey = %q, want %q", header.IdempotencyKey, "idem-123")
	}
	if header.Session != "9" {
		t.Errorf("Session = %q, want %q", header.Session, "9")
	}
	if header.Csrf != "csrf-abc" {
		t.Errorf("Csrf = %q, want %q", header.Csrf, "csrf-abc")
	}
	if header.RequestID != "req-xyz" {
		t.Errorf("RequestID = %q, want %q", header.RequestID, "req-xyz")
	}
}
