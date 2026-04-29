package paycloudhelper

import (
	"testing"

	"github.com/PayCloud-ID/paycloudhelper/phlogger"
)

func TestRootLogI_FiresHooks(t *testing.T) {
	phlogger.ClearLogHooks()
	called := false
	phlogger.RegisterLogHook("info", func(level, message string) {
		called = true
	})
	defer phlogger.ClearLogHooks()

	LogI("[TestRootLogI_FiresHooks] test message")

	if !called {
		t.Fatal("hook was not fired when calling root LogI")
	}
}

func TestRootLogE_FiresHooks(t *testing.T) {
	phlogger.ClearLogHooks()
	called := false
	phlogger.RegisterLogHook("error", func(level, message string) {
		called = true
	})
	defer phlogger.ClearLogHooks()

	LogE("[TestRootLogE_FiresHooks] test error")

	if !called {
		t.Fatal("hook was not fired when calling root LogE")
	}
}

func TestRootLogW_FiresHooks(t *testing.T) {
	phlogger.ClearLogHooks()
	called := false
	phlogger.RegisterLogHook("warn", func(level, message string) {
		called = true
	})
	defer phlogger.ClearLogHooks()

	LogW("[TestRootLogW_FiresHooks] test warning")

	if !called {
		t.Fatal("hook was not fired when calling root LogW")
	}
}
