package paycloudhelper

import (
	"context"
	"testing"
	"time"

	"github.com/PayCloud-ID/paycloudhelper/phlogger"
)

// Exercises root logger.go delegators to improve merged -coverpkg coverage.
func TestLoggerDelegates_CallWithoutPanic(t *testing.T) {
	ctx := context.Background()

	phlogger.ClearLogHooks()
	defer phlogger.ClearLogHooks()

	LogD("[TestLoggerDelegates] debug")
	LogI("[TestLoggerDelegates] info")
	LogW("[TestLoggerDelegates] warn")
	LogE("[TestLoggerDelegates] err=%v", context.Canceled)

	LogDCtx(ctx, "[TestLoggerDelegates] dctx")
	LogICtx(ctx, "[TestLoggerDelegates] ictx")
	LogWCtx(ctx, "[TestLoggerDelegates] wctx")
	LogECtx(ctx, "[TestLoggerDelegates] ectx err=%v", context.Canceled)

	lc := LogCtx(ctx, FieldTicketID, "t1")
	if lc == nil {
		t.Fatal("LogCtx returned nil")
	}
	lc.LogI("[TestLoggerDelegates] ctxlog")

	LogIRated("k1", "[TestLoggerDelegates] irated")
	LogERated("k2", "[TestLoggerDelegates] erated")
	LogWRated("k3", "[TestLoggerDelegates] wrated")
	LogDRated("k4", "[TestLoggerDelegates] drated")

	w := 10 * time.Millisecond
	LogIRatedW("k5", w, "[TestLoggerDelegates] iratedw")
	LogERatedW("k6", w, "[TestLoggerDelegates] eratedw")
	LogWRatedW("k7", w, "[TestLoggerDelegates] wratedw")
	LogDRatedW("k8", w, "[TestLoggerDelegates] dratedw")

	InitializeSampler(SamplerConfig{Initial: 1, Thereafter: 1, Period: time.Second})
	_ = SamplerConfigForEnv("production")
	_ = SamplerConfigFromAppEnv()

	if NewLogContext("a", "b") == nil {
		t.Fatal("NewLogContext returned nil")
	}

	RegisterMetricsHook(func(string, int64) {})
	IncrementMetric("evt")
	IncrementMetricBy("evt", 2)

	_ = NewKeyedLimiter(10, 5)

	LogSetLevel("info")
	LogJ(map[string]any{"k": "v"})
	LogJI(map[string]any{"k": "v"})
	LogErr(context.Canceled)

	ConfigureLogForwarding(phlogger.LogForwardConfig{
		ForwardFatal: false, ForwardError: true, ForwardWarn: true,
		ForwardInfo: true, ForwardDebug: true,
	})
	ConfigureSentryLogging(false)
}
