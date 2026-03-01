package paycloudhelper

import (
	"testing"
)

// AddValidatorLibs is called from init(); calling it again would panic (rule already defined).
// Validation behavior is covered by TestHeaders_ValiadateHeaderIdem_* and TestHeaders_ValiadateHeaderCsrf_*.

func TestValidatorConstants(t *testing.T) {
	if Numeric == "" {
		t.Error("Numeric constant should not be empty")
	}
	if Key == "" {
		t.Error("Key constant should not be empty")
	}
}

func TestHeaders_ValiadateHeaderIdem_WithValidatorRules(t *testing.T) {
	// Rules registered in init()
	tests := []struct {
		name    string
		headers Headers
		wantNil bool
	}{
		{
			name:    "valid idem key alphanumeric",
			headers: Headers{IdempotencyKey: "abc-123_XYZ", Session: ""},
			wantNil: true,
		},
		{
			name:    "empty idem key",
			headers: Headers{IdempotencyKey: "", Session: "123"},
			wantNil: false,
		},
		{
			name:    "idem key too long",
			headers: Headers{IdempotencyKey: "a123456789012345678901234567890123456789012345678901", Session: ""},
			wantNil: false,
		},
		{
			name:    "invalid chars in idem key",
			headers: Headers{IdempotencyKey: "key@with#special", Session: ""},
			wantNil: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.headers.ValiadateHeaderIdem()
			if (got == nil) != tt.wantNil {
				t.Errorf("ValiadateHeaderIdem() = %v, wantNil %v", got, tt.wantNil)
			}
		})
	}
}

func TestHeaders_ValiadateHeaderCsrf_WithValidatorRules(t *testing.T) {
	// Rules are registered in init()
	tests := []struct {
		name    string
		headers Headers
		wantNil bool
	}{
		{
			name:    "valid csrf",
			headers: Headers{Csrf: "token-abc_123"},
			wantNil: true,
		},
		{
			name:    "empty csrf",
			headers: Headers{Csrf: ""},
			wantNil: false,
		},
		{
			name:    "csrf too long",
			headers: Headers{Csrf: "a123456789012345678901234567890123456789012345678901"},
			wantNil: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.headers.ValiadateHeaderCsrf()
			if (got == nil) != tt.wantNil {
				t.Errorf("ValiadateHeaderCsrf() = %v, wantNil %v", got, tt.wantNil)
			}
		})
	}
}
