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
	window := 500 * time.Millisecond

	// Reset global limiter state for this key
	globalRateLimiter.entries.Delete(key)

	// First call should emit (no panic, no error)
	LogIRated(key, window, "test emit")

	// Second/third call within window should be suppressed
	LogIRated(key, window, "should be suppressed")
	LogIRated(key, window, "should be suppressed 2")

	// After window, next call should emit with suppressed count
	time.Sleep(window + 10*time.Millisecond)
	LogIRated(key, window, "after window") // should log "[+2 suppressed]"
}
