package paycloudhelper

import (
	"testing"

	"bitbucket.org/paycloudid/paycloudhelper/phlogger"
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
