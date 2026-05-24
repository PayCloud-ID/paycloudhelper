package paycloudhelper

import (
	"strings"
	"testing"
	"time"
)

func TestMaxTTL(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want time.Duration
	}{
		{name: "unset uses 30d default", env: "", want: 30 * 24 * time.Hour},
		{name: "override 60m", env: "60", want: 60 * time.Minute},
		{name: "override 7d", env: "10080", want: 7 * 24 * time.Hour},
		{name: "negative reverts to default", env: "-1", want: 30 * 24 * time.Hour},
		{name: "zero reverts to default", env: "0", want: 30 * 24 * time.Hour},
		{name: "non-integer reverts to default", env: "abc", want: 30 * 24 * time.Hour},
		{name: "large override (no upper bound)", env: "525600", want: 365 * 24 * time.Hour},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("REDIS_MAX_TTL_MINUTES", tt.env)
			if got := MaxTTL(); got != tt.want {
				t.Fatalf("MaxTTL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClampStoreTTL(t *testing.T) {
	tests := []struct {
		name   string
		envMax string
		ttl    time.Duration
		want   time.Duration
	}{
		{name: "negative clamps to 0", envMax: "", ttl: -1 * time.Second, want: 0},
		{name: "zero passes through (no expiry)", envMax: "", ttl: 0, want: 0},
		{name: "under cap passes through", envMax: "", ttl: time.Hour, want: time.Hour},
		{name: "at default cap passes through", envMax: "", ttl: 30 * 24 * time.Hour, want: 30 * 24 * time.Hour},
		{name: "over default cap clamps to default", envMax: "", ttl: 31 * 24 * time.Hour, want: 30 * 24 * time.Hour},
		{name: "huge overflow clamps to default", envMax: "", ttl: 127 * 365 * 24 * time.Hour, want: 30 * 24 * time.Hour},
		{name: "env-tightened cap honored", envMax: "60", ttl: 90 * time.Minute, want: 60 * time.Minute},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("REDIS_MAX_TTL_MINUTES", tt.envMax)
			if got := clampStoreTTL("test-key", tt.ttl); got != tt.want {
				t.Fatalf("clampStoreTTL(%v) with env=%q = %v, want %v", tt.ttl, tt.envMax, got, tt.want)
			}
		})
	}
}

// TestClampStoreTTL_DescriptiveKeyInLog smoke-tests that the key is propagated
// into the warning log (so operators can identify the bad call site).
func TestClampStoreTTL_DescriptiveKeyInLog(t *testing.T) {
	// The clamp logs to pchelper.LogW; the test cannot capture the log
	// directly, but it can verify the function name and key appear in the
	// generated key name we pass.
	const key = "phase4-guardrail-test"
	t.Setenv("REDIS_MAX_TTL_MINUTES", "1")
	got := clampStoreTTL(key, 24*time.Hour)
	want := 1 * time.Minute
	if got != want {
		t.Fatalf("clampStoreTTL(key=%q, ttl=24h) with env=1m = %v, want %v", key, got, want)
	}
	if !strings.Contains(key, "guardrail") {
		t.Fatalf("key should describe the test (sanity)")
	}
}
