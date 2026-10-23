package phlogger

import (
	"sync/atomic"
	"testing"
	"time"
)

// aggressiveSampler sets an aggressive sampler that would suppress most logs
// if errors/fatals were not exempt: Initial=1, Thereafter=1000 means only the
// 1st call and every 1000th call after would pass.
func aggressiveSampler(t *testing.T) {
	t.Helper()
	withSampler(t, SamplerConfig{Initial: 1, Thereafter: 1000, Period: time.Second})
}

// ── LogE: never sampled ───────────────────────────────────────────────────────

// TestLogE_NeverSampled_AllCallsEmitAndFireHook proves that with an aggressive
// sampler configured, all 50 LogE calls fire the error hook.
// Before the fix, only the first would pass and the rest would be suppressed.
func TestLogE_NeverSampled_AllCallsEmitAndFireHook(t *testing.T) {
	aggressiveSampler(t)

	count := hookForLevel(t, "error")

	const calls = 50
	for i := 0; i < calls; i++ {
		LogE("[TestLogE_NeverSampled] payment failed ref_no=TXN%05d", i)
	}

	if got := count.Load(); got != calls {
		t.Errorf("LogE must never be sampled: expected all %d calls to fire hook, got %d", calls, got)
	}
}

// ── LogF: never sampled ───────────────────────────────────────────────────────

// TestLogF_NeverSampled_AllCallsFireHook proves that with an aggressive sampler
// all LogF calls fire the fatal hook.
// Before the fix, the sampler would suppress LogF after the first call, silently
// returning without calling os.Exit — a critical safety violation.
//
// Uses the package-level fatalHandler seam to prevent the test process from exiting.
func TestLogF_NeverSampled_AllCallsFireHook(t *testing.T) {
	aggressiveSampler(t)

	// Replace fatalHandler to prevent os.Exit in the test process.
	origFatalHandler := fatalHandler
	fatalHandler = func(_ string, _ ...interface{}) { /* no-op: do not exit */ }
	t.Cleanup(func() { fatalHandler = origFatalHandler })

	count := hookForLevel(t, "fatal")

	const calls = 50
	for i := 0; i < calls; i++ {
		LogF("[TestLogF_NeverSampled] unrecoverable state ref_no=TXN%05d", i)
	}

	if got := count.Load(); got != calls {
		t.Errorf("LogF must never be sampled: expected all %d calls to fire hook, got %d", calls, got)
	}
}

// ── Debug/Info/Warn: still sampled ───────────────────────────────────────────

// TestLogI_StillSampled verifies that Info logs remain subject to sampling.
func TestLogI_StillSampled(t *testing.T) {
	aggressiveSampler(t)

	count := hookForLevel(t, "info")

	const calls = 50
	for i := 0; i < calls; i++ {
		LogI("[TestLogI_StillSampled] merchant refresh tick")
	}

	got := count.Load()
	if got >= calls {
		t.Errorf("LogI should still be sampled: expected fewer than %d hooks, got %d", calls, got)
	}
	if got == 0 {
		t.Error("LogI should emit at least once (initial burst)")
	}
}

// TestLogW_StillSampled verifies that Warning logs remain subject to sampling.
func TestLogW_StillSampled(t *testing.T) {
	aggressiveSampler(t)

	count := hookForLevel(t, "warn")

	const calls = 50
	for i := 0; i < calls; i++ {
		LogW("[TestLogW_StillSampled] retry attempt exhausted")
	}

	got := count.Load()
	if got >= calls {
		t.Errorf("LogW should still be sampled: expected fewer than %d hooks, got %d", calls, got)
	}
	if got == 0 {
		t.Error("LogW should emit at least once (initial burst)")
	}
}

// TestLogD_StillSampled verifies that Debug logs remain subject to sampling.
func TestLogD_StillSampled(t *testing.T) {
	aggressiveSampler(t)
	LogSetLevel("debug")
	t.Cleanup(func() { LogSetLevel("info") })

	count := hookForLevel(t, "debug")

	const calls = 50
	for i := 0; i < calls; i++ {
		LogD("[TestLogD_StillSampled] cache lookup key=k")
	}

	got := count.Load()
	if got >= calls {
		t.Errorf("LogD should still be sampled: expected fewer than %d hooks, got %d", calls, got)
	}
	if got == 0 {
		t.Error("LogD should emit at least once (initial burst)")
	}
}

// ── LogF: fatalHandler seam fires on every call ───────────────────────────────

// TestLogF_FatalHandlerSeam verifies that the fatalHandler seam is invoked
// on every non-suppressed LogF call, confirming the process-exit path is always reached.
func TestLogF_FatalHandlerSeam_CalledOnEveryLogF(t *testing.T) {
	disableSampler(t)

	var exitCalls atomic.Int64
	origFatalHandler := fatalHandler
	fatalHandler = func(_ string, _ ...interface{}) { exitCalls.Add(1) }
	t.Cleanup(func() { fatalHandler = origFatalHandler })

	const calls = 5
	for i := 0; i < calls; i++ {
		LogF("[TestLogF_FatalHandlerSeam] fatal scenario %d", i)
	}

	if got := exitCalls.Load(); got != calls {
		t.Errorf("fatalHandler should be called once per LogF: expected %d, got %d", calls, got)
	}
}
