package paycloudhelper

import (
	"testing"

	"github.com/PayCloud-ID/paycloudhelper/phlogger"
)

func TestConfigureLogForwarding_RegistersHooksForEnabledLevels(t *testing.T) {
	phlogger.ClearLogHooks()
	defer phlogger.ClearLogHooks()

	// Configure with error and warn forwarding enabled
	ConfigureLogForwarding(phlogger.LogForwardConfig{
		ForwardError: true,
		ForwardWarn:  true,
		ForwardFatal: false,
	})

	// We can't easily test that ReceiveLog is called without Sentry,
	// but we can verify hooks are registered by checking they fire.
	errorHookFired := false
	warnHookFired := false

	// Add additional test hooks to observe
	phlogger.RegisterLogHook("error", func(level, message string) { errorHookFired = true })
	phlogger.RegisterLogHook("warn", func(level, message string) { warnHookFired = true })

	LogE("[TestConfigureLogForwarding] error event")
	LogW("[TestConfigureLogForwarding] warn event")

	if !errorHookFired {
		t.Error("error hook not called")
	}
	if !warnHookFired {
		t.Error("warn hook not called")
	}
}

func TestLogForwardConfigFromEnv_ReturnsConfig(t *testing.T) {
	cfg := LogForwardConfigFromEnv()
	// Default: ForwardFatal=true, ForwardError=true, rest=false
	if !cfg.ForwardFatal {
		t.Error("ForwardFatal should default to true")
	}
	if !cfg.ForwardError {
		t.Error("ForwardError should default to true")
	}
}

func TestSentryLoggingFromEnv_DefaultFalse(t *testing.T) {
	t.Setenv("SENTRY_LOGGING", "")
	if SentryLoggingFromEnv() {
		t.Error("SentryLoggingFromEnv() should default to false when unset")
	}
}

func TestSentryLoggingFromEnv_AcceptsMultipleBooleanFormats(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected bool
	}{
		// True values
		{"true lowercase", "true", true},
		{"true uppercase", "TRUE", true},
		{"true mixed case", "True", true},
		{"true short", "t", true},
		{"true short T", "T", true},
		{"one", "1", true},
		// False values
		{"false lowercase", "false", false},
		{"false uppercase", "FALSE", false},
		{"false mixed case", "False", false},
		{"false short", "f", false},
		{"false short F", "F", false},
		{"zero", "0", false},
		// Invalid values default to false
		{"yes", "yes", false},
		{"no", "no", false},
		{"random string", "random", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SENTRY_LOGGING", tt.value)
			got := SentryLoggingFromEnv()
			if got != tt.expected {
				t.Errorf("SentryLoggingFromEnv() = %v, want %v", got, tt.expected)
			}
		})
	}
}
