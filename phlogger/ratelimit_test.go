package phlogger

import (
	"testing"
	"time"
)

func TestRateLimiter_AllowsFirstEmit(t *testing.T) {
	rl := newRateLimiter()
	allowed, suppressed := rl.check("test.key", 1*time.Second)
	if !allowed {
		t.Fatal("first emit should be allowed")
	}
	if suppressed != 0 {
		t.Fatalf("expected 0 suppressed, got %d", suppressed)
	}
}

func TestRateLimiter_SuppressDuplicates(t *testing.T) {
	rl := newRateLimiter()
	rl.check("test.key", 1*time.Second) // first: allowed

	allowed, _ := rl.check("test.key", 1*time.Second) // within window
	if allowed {
		t.Fatal("second emit within window should be suppressed")
	}
}

func TestRateLimiter_CountsSuppressed(t *testing.T) {
	rl := newRateLimiter()
	rl.check("k", 1*time.Second)
	rl.check("k", 1*time.Second)
	rl.check("k", 1*time.Second)

	// Force window expiry by manipulating lastEmit
	entry, _ := rl.entries.Load("k")
	e := entry.(*rateLimitEntry)
	e.lastEmit = time.Now().Add(-2 * time.Second)

	allowed, suppressed := rl.check("k", 1*time.Second)
	if !allowed {
		t.Fatal("after window expiry, should allow")
	}
	if suppressed != 2 {
		t.Fatalf("expected 2 suppressed, got %d", suppressed)
	}
}

func TestRateLimiter_IndependentKeys(t *testing.T) {
	rl := newRateLimiter()
	rl.check("a", 1*time.Second)
	allowedB, _ := rl.check("b", 1*time.Second)
	if !allowedB {
		t.Fatal("different key should be allowed independently")
	}
}

func TestRateLimiter_ZeroWindow_AlwaysAllows(t *testing.T) {
	rl := newRateLimiter()
	rl.check("x", 0)
	allowed, _ := rl.check("x", 0)
	if !allowed {
		t.Fatal("zero window should always allow (rate limiting disabled)")
	}
}

func TestLogIRated_SuppressesInWindow(t *testing.T) {
	key := "test.LogIRated.suppress"

	// Reset global limiter state for this key
	globalRateLimiter.entries.Delete(key)

	// First call should emit (no panic, no error)
	LogIRated(key, "test emit")

	// Second/third call within default 50ms window should be suppressed
	LogIRated(key, "should be suppressed")
	LogIRated(key, "should be suppressed 2")

	// After window, next call should emit with suppressed count
	time.Sleep(defaultWindow + 10*time.Millisecond)
	LogIRated(key, "after window") // should log "[+2 suppressed]"
}

func TestLogIRatedW_UsesCustomWindow(t *testing.T) {
	key := "test.LogIRatedW.custom"
	customWindow := 200 * time.Millisecond
	globalRateLimiter.entries.Delete(key)

	LogIRatedW(key, customWindow, "first emit")

	// Should be suppressed within custom window
	LogIRatedW(key, customWindow, "suppressed within custom window")

	// After custom window, should emit
	time.Sleep(customWindow + 10*time.Millisecond)
	LogIRatedW(key, customWindow, "after custom window")
}

func TestLogI_RateLimitsOnFormatString(t *testing.T) {
	format := "test.default.ratelimit key=%s"

	// Reset limiter state for this format
	globalRateLimiter.entries.Delete(format)

	// First call should be allowed
	allowed1, _ := globalRateLimiter.check(format, defaultWindow)
	if !allowed1 {
		t.Fatal("first LogI call should be allowed")
	}

	// Second call within 50ms should be suppressed
	allowed2, _ := globalRateLimiter.check(format, defaultWindow)
	if allowed2 {
		t.Fatal("second LogI call within defaultWindow should be suppressed")
	}
}
