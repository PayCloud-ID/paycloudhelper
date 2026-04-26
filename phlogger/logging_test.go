// Package phlogger contains integration tests for the pchelper logging
// standard.
//
// Usage:
//
//	# Run all logging tests in paycloudhelper
//	go test -v ./phlogger/...
//
//	# Run all logging tests with race detector
//	go test -race -v ./phlogger/...
//
//	# Run a specific test by name
//	go test -v -run TestHighOutput_ProductionSampler_SuppressesBurst ./phlogger/...
//
//	# Run all tests matching a pattern
//	go test -v -run TestSampler ./phlogger/...
//	go test -v -run TestLogContext ./phlogger/...
//	go test -v -run TestLogIRated ./phlogger/...
//
//	# Run with a custom timeout
//	go test -v -timeout 60s ./phlogger/...
//
// Coverage:
//   - Basic log levels (LogI, LogE, LogW, LogD) via hook observation
//   - Sampler rate limiting: production / staging / disabled modes
//   - Independent keys: different format strings are rate-limited independently
//   - High-output: suppression count under burst load + period reset
//   - LogRated variants (sampler key, time-window key)
//   - LogContext: request-scoped child logger with key=value fields
//   - Log message content: hooks receive correctly formatted strings
package phlogger

import (
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// ── helpers ──────────────────────────────────────────────────────────────────

// hookForLevel registers a hook on the given log level, records call count,
// and clears all hooks after the test completes.
func hookForLevel(t *testing.T, level string) *atomic.Int64 {
	t.Helper()
	var count atomic.Int64
	RegisterLogHook(level, func(_, _ string) { count.Add(1) })
	t.Cleanup(ClearLogHooks)
	return &count
}

// hookCapture registers a hook that appends every received message to a slice.
// The returned pointer is safe to read after the hook has fired.
func hookCapture(t *testing.T, level string) *[]string {
	t.Helper()
	msgs := &[]string{}
	RegisterLogHook(level, func(_, message string) {
		*msgs = append(*msgs, message)
	})
	t.Cleanup(ClearLogHooks)
	return msgs
}

// disableSampler sets sampling to disabled (Initial=0) for the duration of
// the test, then restores environment-aware defaults on cleanup.
func disableSampler(t *testing.T) {
	t.Helper()
	InitializeSampler(SamplerConfig{})
	t.Cleanup(func() {
		InitializeSampler(SamplerConfigFromAppEnv())
	})
}

// withSampler sets a custom SamplerConfig for the duration of the test.
func withSampler(t *testing.T, cfg SamplerConfig) {
	t.Helper()
	InitializeSampler(cfg)
	t.Cleanup(func() {
		InitializeSampler(SamplerConfigFromAppEnv())
	})
}

// ── basic level tests ─────────────────────────────────────────────────────────

// TestLogI_FiresHook verifies that LogI triggers the registered hook.
func TestLogI_FiresHook(t *testing.T) {
	disableSampler(t)
	count := hookForLevel(t, "info")

	LogI("[TestLogI_FiresHook] info message key=%s", "value")

	if count.Load() == 0 {
		t.Fatal("info hook should have fired — got 0 calls")
	}
}

// TestLogE_FiresHook verifies that LogE triggers the registered hook.
func TestLogE_FiresHook(t *testing.T) {
	disableSampler(t)
	count := hookForLevel(t, "error")

	LogE("[TestLogE_FiresHook] error message err=%v", fmt.Errorf("test error"))

	if count.Load() == 0 {
		t.Fatal("error hook should have fired — got 0 calls")
	}
}

// TestLogW_FiresHook verifies that LogW triggers the registered hook.
func TestLogW_FiresHook(t *testing.T) {
	disableSampler(t)
	count := hookForLevel(t, "warn")

	LogW("[TestLogW_FiresHook] warning attempt=%d max=%d", 1, 3)

	if count.Load() == 0 {
		t.Fatal("warn hook should have fired — got 0 calls")
	}
}

// TestLogD_FiresHook verifies that LogD triggers the registered debug hook.
// Requires log level to be "debug" since the default is "info".
func TestLogD_FiresHook(t *testing.T) {
	disableSampler(t)
	LogSetLevel("debug")
	t.Cleanup(func() { LogSetLevel("info") })

	count := hookForLevel(t, "debug")

	LogD("[TestLogD_FiresHook] debug detail redis_key=%s", "cache:001")

	if count.Load() == 0 {
		t.Fatal("debug hook should have fired — got 0 calls")
	}
}

// TestLogJ_LogJI_routeThroughLogI verifies JSON helpers delegate to LogI (info hook).
func TestLogJ_LogJI_routeThroughLogI(t *testing.T) {
	disableSampler(t)
	msgs := hookCapture(t, "info")

	LogJ(map[string]string{"k": "v"})
	LogJI(map[string]string{"k": "v"})

	if len(*msgs) < 2 {
		t.Fatalf("expected 2 info emissions, got %d: %v", len(*msgs), *msgs)
	}
	if !strings.Contains((*msgs)[0], `"k":"v"`) {
		t.Errorf("LogJ output should contain JSON, got %q", (*msgs)[0])
	}
}

// TestLogErr_noPanic verifies LogErr delegates to golog without crashing.
// Note: LogErr uses golog's Error() directly and does not route through emitE / fireHooks.
func TestLogErr_noPanic(t *testing.T) {
	LogErr(fmt.Errorf("audit failure"))
}

// TestInitializeLogger_noPanic exercises gin level registration and sampler init.
func TestInitializeLogger_noPanic(t *testing.T) {
	t.Cleanup(func() { InitializeSampler(SamplerConfigFromAppEnv()) })
	InitializeLogger()
	InitializeLogger()
}

// TestLogWRated_LogDRated_useSamplerKey verifies warn/debug rated paths share sampler logic.
func TestLogWRated_LogDRated_useSamplerKey(t *testing.T) {
	withSampler(t, SamplerConfig{Initial: 1, Thereafter: 0, Period: time.Second})
	LogSetLevel("debug")
	t.Cleanup(func() { LogSetLevel("info") })

	var warnN, debugN atomic.Int64
	RegisterLogHook("warn", func(_, _ string) { warnN.Add(1) })
	RegisterLogHook("debug", func(_, _ string) { debugN.Add(1) })
	t.Cleanup(ClearLogHooks)

	LogWRated("w.key", "[TestLogWRated] one")
	LogWRated("w.key", "[TestLogWRated] two")
	LogDRated("d.key", "[TestLogDRated] one")
	LogDRated("d.key", "[TestLogDRated] two")

	if warnN.Load() != 1 {
		t.Fatalf("LogWRated: want 1 warn hook, got %d", warnN.Load())
	}
	if debugN.Load() != 1 {
		t.Fatalf("LogDRated: want 1 debug hook, got %d", debugN.Load())
	}
}

// TestAllLevels_IndependentHooks verifies each level fires its own hook
// independently in a single test run.
func TestAllLevels_IndependentHooks(t *testing.T) {
	disableSampler(t)
	ClearLogHooks()
	t.Cleanup(ClearLogHooks)

	cases := []struct {
		level string
		emit  func()
	}{
		{"info", func() { LogI("[TestAllLevels] info msg") }},
		{"warn", func() { LogW("[TestAllLevels] warn msg") }},
		{"error", func() { LogE("[TestAllLevels] error msg") }},
	}

	for _, tc := range cases {
		var count atomic.Int64
		RegisterLogHook(tc.level, func(_, _ string) { count.Add(1) })
		tc.emit()
		if count.Load() == 0 {
			t.Errorf("level %q: hook should have fired after emit", tc.level)
		}
	}
}

// ── sampler rate-limit tests ──────────────────────────────────────────────────

// TestSampler_Disabled_AllowsAll verifies that when sampling is disabled
// (Initial=0) every single call reaches the hook.
func TestSampler_Disabled_AllowsAll(t *testing.T) {
	disableSampler(t)
	count := hookForLevel(t, "info")

	const calls = 50
	for i := 0; i < calls; i++ {
		LogI("[TestSampler_Disabled] log line")
	}

	if got := count.Load(); got != calls {
		t.Errorf("disabled sampler: expected all %d calls, got %d", calls, got)
	}
}

// TestSampler_ProductionConfig_LimitsOutput verifies that production-mode
// sampling (Initial=5, Thereafter=50, Period=1s) suppresses messages beyond
// the initial burst within a single period.
func TestSampler_ProductionConfig_LimitsOutput(t *testing.T) {
	withSampler(t, SamplerConfig{
		Initial:    5,
		Thereafter: 50,
		Period:     time.Second,
	})
	count := hookForLevel(t, "info")

	const calls = 20
	for i := 0; i < calls; i++ {
		LogI("[TestSampler_Production] record processing batch_id=B001")
	}

	// Initial=5, Thereafter=50: in 20 calls only the initial 5 fire (no
	// Thereafter multiple falls within 1-15 over-initial calls for Thereafter=50).
	got := count.Load()
	if got < 5 {
		t.Errorf("production sampler should allow initial burst of 5, got %d", got)
	}
	if got > 5 {
		t.Errorf("production sampler should suppress calls beyond initial 5 within 1s, got %d/20", got)
	}
}

// TestSampler_StagingConfig_LimitsOutput verifies staging sampling
// (Initial=10, Thereafter=10, Period=1s).
func TestSampler_StagingConfig_LimitsOutput(t *testing.T) {
	withSampler(t, SamplerConfig{
		Initial:    10,
		Thereafter: 10,
		Period:     time.Second,
	})
	count := hookForLevel(t, "info")

	// 25 calls: initial 10 + 1 thereafter hit (at call 20) = 11 expected.
	for i := 0; i < 25; i++ {
		LogI("[TestSampler_Staging] payment method refresh")
	}

	got := count.Load()
	if got < 10 || got > 12 {
		t.Errorf("staging sampler: expected 10-12 of 25 calls, got %d", got)
	}
}

// TestSampler_PeriodReset_ResetsQuota verifies that after the sampling period
// elapses the initial burst quota is reset and new calls are emitted.
func TestSampler_PeriodReset_ResetsQuota(t *testing.T) {
	withSampler(t, SamplerConfig{
		Initial:    3,
		Thereafter: 0,
		Period:     60 * time.Millisecond,
	})
	count := hookForLevel(t, "info")

	// First period: 3 allowed, rest suppressed.
	for i := 0; i < 10; i++ {
		LogI("[TestSampler_PeriodReset] resettable burst")
	}
	if got := count.Load(); got != 3 {
		t.Errorf("first period: expected exactly 3 allowed, got %d", got)
	}

	time.Sleep(80 * time.Millisecond) // wait for period to elapse

	// Second period: quota resets → 3 more allowed.
	for i := 0; i < 10; i++ {
		LogI("[TestSampler_PeriodReset] resettable burst")
	}
	if got := count.Load(); got != 6 {
		t.Errorf("after period reset: expected 6 total (3+3), got %d", got)
	}
}

// ── independent keys tests ────────────────────────────────────────────────────

// TestSampler_DifferentFormatStrings_AreIndependent verifies that two different
// log format strings use independent rate-limit counters.
// Suppressing one key must NOT suppress the other.
func TestSampler_DifferentFormatStrings_AreIndependent(t *testing.T) {
	withSampler(t, SamplerConfig{
		Initial:    1,
		Thereafter: 0,
		Period:     time.Second,
	})
	count := hookForLevel(t, "info")

	// Each distinct format string should fire once (its own initial burst).
	LogI("[TestSampler_Independent] partner list cache miss")
	LogI("[TestSampler_Independent] payment method cache miss")

	if got := count.Load(); got < 2 {
		t.Errorf("two distinct format strings should each emit on first call, got %d", got)
	}

	// Second calls for same format strings should both be suppressed.
	before := count.Load()
	LogI("[TestSampler_Independent] partner list cache miss")
	LogI("[TestSampler_Independent] payment method cache miss")
	after := count.Load()

	if after > before {
		t.Errorf("second calls for same format strings should be suppressed, but got %d more", after-before)
	}
}

// TestSampler_DifferentLevels_AreIndependent verifies that hooks for distinct
// levels fire independently even for the same format string.
func TestSampler_DifferentLevels_AreIndependent(t *testing.T) {
	disableSampler(t)
	ClearLogHooks()
	t.Cleanup(ClearLogHooks)

	var info, errCount, warn atomic.Int64
	RegisterLogHook("info", func(_, _ string) { info.Add(1) })
	RegisterLogHook("error", func(_, _ string) { errCount.Add(1) })
	RegisterLogHook("warn", func(_, _ string) { warn.Add(1) })

	msg := "[TestSampler_DifferentLevels] status update"
	LogI(msg)
	LogE(msg)
	LogW(msg)

	if info.Load() == 0 {
		t.Error("info hook should have fired")
	}
	if errCount.Load() == 0 {
		t.Error("error hook should have fired")
	}
	if warn.Load() == 0 {
		t.Error("warn hook should have fired")
	}
}

// ── high-output tests ─────────────────────────────────────────────────────────

// TestHighOutput_ProductionSampler_SuppressesBurst emits 1000 identical lines
// under production-mode sampling and verifies effective suppression.
// Expected: 5 initial + floor((1000-5)/50) = 5+19 = 24 allowed.
func TestHighOutput_ProductionSampler_SuppressesBurst(t *testing.T) {
	withSampler(t, SamplerConfig{
		Initial:    5,
		Thereafter: 50,
		Period:     time.Second,
	})
	count := hookForLevel(t, "info")

	const burst = 1000
	for i := 0; i < burst; i++ {
		LogI("[TestHighOutput] batch records processing partner_id=P001")
	}

	got := count.Load()
	// Acceptable range: 20-30 (exact atomic timing may vary by ±1).
	if got > 30 {
		t.Errorf("sampler should suppress high output: expected ≤30 allowed/%d, got %d (%.1f%%)",
			burst, got, 100*float64(got)/burst)
	}
	if got < 5 {
		t.Errorf("sampler must allow at least the initial burst: expected ≥5, got %d", got)
	}
	t.Logf("high-output: %d/%d allowed (%.1f%% suppressed)", got, burst, 100*(1-float64(got)/burst))
}

// TestHighOutput_DisabledSampler_AllowsAll verifies no suppression occurs
// under high volume when sampling is disabled.
func TestHighOutput_DisabledSampler_AllowsAll(t *testing.T) {
	disableSampler(t)
	count := hookForLevel(t, "info")

	const burst = 500
	for i := 0; i < burst; i++ {
		LogI("[TestHighOutput_Disabled] record id=%d processed", i)
	}

	if got := count.Load(); got != burst {
		t.Errorf("disabled sampler: expected all %d calls through, got %d", burst, got)
	}
}

// ── LogIRated / LogERated (sampler key) tests ────────────────────────────────

// TestLogIRated_CustomKey_DeduplicatesAcrossFormats verifies that when two
// callers share the same rated key, they share the rate-limit quota.
func TestLogIRated_CustomKey_DeduplicatesAcrossFormats(t *testing.T) {
	withSampler(t, SamplerConfig{
		Initial:    1,
		Thereafter: 0,
		Period:     time.Second,
	})
	count := hookForLevel(t, "info")

	// Both use the same key — only the first should emit.
	LogIRated("cache.miss", "[GetPartner] cache miss key=%s", "partners")
	LogIRated("cache.miss", "[GetPaymentMethod] cache miss key=%s", "methods")

	if got := count.Load(); got != 1 {
		t.Errorf("same rated key should emit exactly 1 time per period, got %d", got)
	}
}

// TestLogIRated_DifferentKeys_AreIndependent verifies that different rated keys
// do not share quota.
func TestLogIRated_DifferentKeys_AreIndependent(t *testing.T) {
	withSampler(t, SamplerConfig{
		Initial:    1,
		Thereafter: 0,
		Period:     time.Second,
	})
	count := hookForLevel(t, "info")

	LogIRated("partner.refresh", "[RefreshPartners] started")
	LogIRated("payment.refresh", "[RefreshPaymentMethod] started")

	if got := count.Load(); got != 2 {
		t.Errorf("two different rated keys should each emit once, got %d", got)
	}
}

// TestLogERated_HighVolume verifies that LogERated suppresses repeated errors
// effectively under high volume.
func TestLogERated_HighVolume(t *testing.T) {
	withSampler(t, SamplerConfig{
		Initial:    2,
		Thereafter: 10,
		Period:     time.Second,
	})
	count := hookForLevel(t, "error")

	// Simulate a hot error path: 100 repetitions of the same error.
	for i := 0; i < 100; i++ {
		LogERated("grpc.error", "[GetMerchantConfig] gRPC call failed err=connection refused")
	}

	got := count.Load()
	// Initial=2, Thereafter=10 → 2 + floor(98/10) = 2+9 = 11 expected.
	// Accept 8-14 for timing tolerance.
	if got > 14 {
		t.Errorf("LogERated should suppress high-volume errors: expected ≤14/100, got %d", got)
	}
	if got < 2 {
		t.Errorf("LogERated should emit at least the initial burst: expected ≥2, got %d", got)
	}
}

// ── LogIRatedW (time-window) tests ────────────────────────────────────────────

// TestLogIRatedW_SuppressesWithinWindow verifies that LogIRatedW only allows
// one emission per key per window, suppressing subsequent calls.
func TestLogIRatedW_SuppressesWithinWindow(t *testing.T) {
	disableSampler(t) // LogIRatedW uses its own window limiter, not the sampler

	count := hookForLevel(t, "info")

	window := 80 * time.Millisecond

	// All three calls within the window → only the first should emit.
	LogIRatedW("config.load", window, "[LoadConfig] loading configuration")
	LogIRatedW("config.load", window, "[LoadConfig] loading configuration")
	LogIRatedW("config.load", window, "[LoadConfig] loading configuration")

	if got := count.Load(); got != 1 {
		t.Errorf("LogIRatedW: expected 1 call within window, got %d", got)
	}
}

// TestLogIRatedW_AllowsAfterWindow verifies that the next call after the
// window elapses is emitted.
func TestLogIRatedW_AllowsAfterWindow(t *testing.T) {
	disableSampler(t)

	count := hookForLevel(t, "info")
	window := 60 * time.Millisecond

	LogIRatedW("holiday.refresh", window, "[RefreshHoliday] tick")
	time.Sleep(80 * time.Millisecond)
	LogIRatedW("holiday.refresh", window, "[RefreshHoliday] tick")

	if got := count.Load(); got != 2 {
		t.Errorf("LogIRatedW: expected 2 calls after window elapsed, got %d", got)
	}
}

// TestLogIRatedW_DifferentKeys_AreIndependent verifies two different window
// keys do not share state.
func TestLogIRatedW_DifferentKeys_AreIndependent(t *testing.T) {
	disableSampler(t)

	count := hookForLevel(t, "info")
	window := 200 * time.Millisecond

	LogIRatedW("partner.tick", window, "[RefreshPartners] cycle start")
	LogIRatedW("sla.tick", window, "[RefreshSLA] cycle start")

	if got := count.Load(); got != 2 {
		t.Errorf("different LogIRatedW keys should each emit once, got %d", got)
	}
}

// ── LogContext tests ──────────────────────────────────────────────────────────

// TestLogContext_PrependsPrefixFields verifies that NewLogContext prepends
// [key=value ...] fields to every log message.
func TestLogContext_PrependsPrefixFields(t *testing.T) {
	disableSampler(t)
	msgs := hookCapture(t, "info")

	ctx := NewLogContext("req_id", "abc-123", "merchant", "M001")
	ctx.LogI("processing payment amount=%d", 5000)

	if len(*msgs) == 0 {
		t.Fatal("info hook should have captured a message")
	}
	msg := (*msgs)[0]

	if !strings.Contains(msg, "req_id=abc-123") {
		t.Errorf("message should contain 'req_id=abc-123', got: %q", msg)
	}
	if !strings.Contains(msg, "merchant=M001") {
		t.Errorf("message should contain 'merchant=M001', got: %q", msg)
	}
}

// TestLogContext_With_ExtendsWithoutMutatingParent verifies that .With() returns
// a new LogContext containing both parent and child fields, while the parent
// remains unchanged.
func TestLogContext_With_ExtendsWithoutMutatingParent(t *testing.T) {
	disableSampler(t)
	msgs := hookCapture(t, "info")

	parent := NewLogContext("req_id", "r42")
	child := parent.With("step", "validate")

	parent.LogI("parent message")
	child.LogI("child message")

	if len(*msgs) < 2 {
		t.Fatalf("expected 2 messages, got %d", len(*msgs))
	}

	parentMsg, childMsg := (*msgs)[0], (*msgs)[1]

	// Parent should NOT contain "step=validate".
	if strings.Contains(parentMsg, "step=validate") {
		t.Errorf("parent should not contain child field, got: %q", parentMsg)
	}

	// Child should contain both parent and child fields.
	if !strings.Contains(childMsg, "req_id=r42") {
		t.Errorf("child should carry parent field req_id=r42, got: %q", childMsg)
	}
	if !strings.Contains(childMsg, "step=validate") {
		t.Errorf("child should carry own field step=validate, got: %q", childMsg)
	}
}

// TestLogContext_AllLevels verifies that every LogContext method fires the
// correct level hook.
func TestLogContext_AllLevels(t *testing.T) {
	disableSampler(t)

	cases := []struct {
		level string
		emit  func(lc *LogContext)
	}{
		{"info", func(lc *LogContext) { lc.LogI("info msg") }},
		{"warn", func(lc *LogContext) { lc.LogW("warn msg") }},
		{"error", func(lc *LogContext) { lc.LogE("error msg") }},
	}

	for _, tc := range cases {
		t.Run(tc.level, func(t *testing.T) {
			ClearLogHooks()
			var count atomic.Int64
			RegisterLogHook(tc.level, func(_, _ string) { count.Add(1) })
			t.Cleanup(ClearLogHooks)

			lc := NewLogContext("service", "configft")
			tc.emit(lc)

			if count.Load() == 0 {
				t.Errorf("LogContext level=%q hook should fire", tc.level)
			}
		})
	}
}

// TestLogContext_MultipleFields verifies that multiple fields are all present
// in the emitted message.
func TestLogContext_MultipleFields(t *testing.T) {
	disableSampler(t)
	msgs := hookCapture(t, "info")

	ctx := NewLogContext(
		"req_id", "r001",
		"merchant", "M_TEST",
		"batch_id", "B999",
	)
	ctx.LogI("batch processing started records=%d", 100)

	if len(*msgs) == 0 {
		t.Fatal("no message captured")
	}
	msg := (*msgs)[0]

	for _, want := range []string{"req_id=r001", "merchant=M_TEST", "batch_id=B999"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message missing %q, got: %q", want, msg)
		}
	}
}

// ── message content tests ─────────────────────────────────────────────────────

// TestLogI_FormattedMessageContent verifies that hooks receive the fully
// formatted message including all interpolated arguments.
func TestLogI_FormattedMessageContent(t *testing.T) {
	disableSampler(t)
	msgs := hookCapture(t, "info")

	LogI("[RefreshPartners] partner_code=%s status=%d", "PCO01", 1)

	if len(*msgs) == 0 {
		t.Fatal("info hook should have captured a message")
	}
	for _, want := range []string{"RefreshPartners", "partner_code=PCO01", "status=1"} {
		if !strings.Contains((*msgs)[0], want) {
			t.Errorf("message should contain %q, got: %q", want, (*msgs)[0])
		}
	}
}

// TestLogE_FormattedMessageContent verifies that the error hook receives
// the correct formatted error string.
func TestLogE_FormattedMessageContent(t *testing.T) {
	disableSampler(t)
	msgs := hookCapture(t, "error")

	LogE("[GetMerchantConfig] gRPC error merchant_code=%s err=%v",
		"MRC001", fmt.Errorf("connection refused"))

	if len(*msgs) == 0 {
		t.Fatal("error hook should have captured a message")
	}
	for _, want := range []string{"GetMerchantConfig", "MRC001", "connection refused"} {
		if !strings.Contains((*msgs)[0], want) {
			t.Errorf("error message should contain %q, got: %q", want, (*msgs)[0])
		}
	}
}

// TestLogW_FormattedMessageContent verifies the warn hook receives structured fields.
func TestLogW_FormattedMessageContent(t *testing.T) {
	disableSampler(t)
	msgs := hookCapture(t, "warn")

	LogW("[RefreshPaymentChannel] DB retry attempt=%d max=%d err=%v",
		2, 3, fmt.Errorf("timeout"))

	if len(*msgs) == 0 {
		t.Fatal("warn hook should have captured a message")
	}
	for _, want := range []string{"attempt=2", "max=3", "timeout"} {
		if !strings.Contains((*msgs)[0], want) {
			t.Errorf("warn message should contain %q, got: %q", want, (*msgs)[0])
		}
	}
}
