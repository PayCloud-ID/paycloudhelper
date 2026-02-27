package phlogger

import (
	"os"
	"testing"
)

func TestLogForwardConfigFromEnv_Defaults(t *testing.T) {
	os.Unsetenv("LOG_FORWARD_FATAL")
	os.Unsetenv("LOG_FORWARD_ERROR")
	os.Unsetenv("LOG_FORWARD_WARN")
	os.Unsetenv("LOG_FORWARD_INFO")
	os.Unsetenv("LOG_FORWARD_DEBUG")

	cfg := LogForwardConfigFromEnv()

	if !cfg.ForwardFatal {
		t.Error("ForwardFatal should default to true")
	}
	if !cfg.ForwardError {
		t.Error("ForwardError should default to true")
	}
	if cfg.ForwardWarn {
		t.Error("ForwardWarn should default to false")
	}
	if cfg.ForwardInfo {
		t.Error("ForwardInfo should default to false")
	}
	if cfg.ForwardDebug {
		t.Error("ForwardDebug should default to false")
	}
}

func TestLogForwardConfigFromEnv_EnvOverride(t *testing.T) {
	os.Setenv("LOG_FORWARD_FATAL", "false")
	os.Setenv("LOG_FORWARD_ERROR", "true")
	defer func() {
		os.Unsetenv("LOG_FORWARD_FATAL")
		os.Unsetenv("LOG_FORWARD_ERROR")
	}()

	cfg := LogForwardConfigFromEnv()

	if cfg.ForwardFatal {
		t.Error("ForwardFatal should be false when env=false")
	}
	if !cfg.ForwardError {
		t.Error("ForwardError should be true when env=true")
	}
}

func TestGetEnvBool_VariousValues(t *testing.T) {
	tests := []struct {
		val      string
		def      bool
		expected bool
	}{
		{"true", false, true},
		{"1", false, true},
		{"yes", false, true},
		{"false", true, false},
		{"0", true, false},
		{"no", true, false},
		{"", true, true},   // empty → default
		{"", false, false}, // empty → default
	}

	for _, tt := range tests {
		os.Setenv("TEST_BOOL", tt.val)
		got := getEnvBool("TEST_BOOL", tt.def)
		if got != tt.expected {
			t.Errorf("getEnvBool(%q, %v) = %v, want %v", tt.val, tt.def, got, tt.expected)
		}
	}
	os.Unsetenv("TEST_BOOL")
}
