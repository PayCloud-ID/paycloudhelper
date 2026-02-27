package phlogger

import (
	"testing"
)

func TestRegisterLogHook_CallsOnMatchingLevel(t *testing.T) {
	ClearLogHooks()
	called := false
	RegisterLogHook("error", func(level, message string) {
		called = true
		if level != "error" {
			t.Errorf("expected level 'error', got %q", level)
		}
	})

	fireHooks("error", "test error message")

	if !called {
		t.Fatal("hook was not called for matching level")
	}
}

func TestRegisterLogHook_SkipsNonMatchingLevel(t *testing.T) {
	ClearLogHooks()
	called := false
	RegisterLogHook("fatal", func(level, message string) {
		called = true
	})

	fireHooks("error", "not fatal")

	if called {
		t.Fatal("hook should not be called for non-matching level")
	}
}

func TestRegisterLogHook_MultipleHooksSameLevel(t *testing.T) {
	ClearLogHooks()
	count := 0
	RegisterLogHook("warn", func(level, message string) { count++ })
	RegisterLogHook("warn", func(level, message string) { count++ })

	fireHooks("warn", "some warning")

	if count != 2 {
		t.Fatalf("expected 2 hooks called, got %d", count)
	}
}

func TestClearLogHooks_RemovesAll(t *testing.T) {
	RegisterLogHook("info", func(level, message string) {
		t.Fatal("hook should have been cleared")
	})
	ClearLogHooks()
	fireHooks("info", "should not trigger hook")
}
