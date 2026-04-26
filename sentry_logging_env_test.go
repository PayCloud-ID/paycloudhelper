package paycloudhelper

import (
	"testing"
)

func TestSentryLoggingFromEnv(t *testing.T) {
	tests := []struct {
		env  string
		want bool
	}{
		{"", false},
		{"true", true},
		{"True", true},
		{"1", true},
		{"false", false},
		{"0", false},
		{"maybe", false},
	}
	for _, tt := range tests {
		t.Run(tt.env, func(t *testing.T) {
			if tt.env == "" {
				t.Setenv("SENTRY_LOGGING", "")
			} else {
				t.Setenv("SENTRY_LOGGING", tt.env)
			}
			if got := SentryLoggingFromEnv(); got != tt.want {
				t.Fatalf("SentryLoggingFromEnv() = %v, want %v", got, tt.want)
			}
		})
	}
}
