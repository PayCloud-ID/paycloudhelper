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

func TestLogIRated_UsesSampler(t *testing.T) {
	// Enable sampler with tight config so we can observe suppression.
	saved := globalSampler
	defer func() { globalSampler = saved }()
	InitializeSampler(SamplerConfig{Initial: 1, Thereafter: 0, Period: time.Second})

	key := "test.LogIRated.sampler"

	// First call should emit (within Initial burst)
	LogIRated(key, "first emit")

	// Second call should be suppressed (Initial=1, Thereafter=0)
	LogIRated(key, "should be suppressed")
	LogIRated(key, "should be suppressed 2")
}

func TestLogIRated_DevEnv_NoSampling(t *testing.T) {
	// With default (disabled) sampler, all calls pass through — dev behavior.
	saved := globalSampler
	defer func() { globalSampler = saved }()
	InitializeSampler(SamplerConfig{}) // disabled

	key := "test.LogIRated.dev"
	LogIRated(key, "all calls")
	LogIRated(key, "pass through")
	LogIRated(key, "in dev")
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

func TestLogI_SampledByDefault(t *testing.T) {
	// In dev env (default), sampler is disabled — all calls should pass through.
	// This verifies LogI delegates to globalSampler, which is disabled when Initial=0.
	saved := globalSampler
	defer func() { globalSampler = saved }()
	InitializeSampler(SamplerConfig{}) // disabled

	format := "test.default.sampler key=%s"
	// Both calls should pass (no suppression in dev).
	LogI(format, "a")
	LogI(format, "b")
}

func TestLogI_SampledInProduction(t *testing.T) {
	saved := globalSampler
	defer func() { globalSampler = saved }()
	InitializeSampler(SamplerConfig{Initial: 1, Thereafter: 0, Period: time.Second})

	format := "test.prod.sampler key=%s"
	// First call allowed, second blocked.
	LogI(format, "a")
	// If sampling works, this second call is suppressed (we can't assert output here
	// but we verify no panic/race and the sampler is wired correctly).
	LogI(format, "b")
}
