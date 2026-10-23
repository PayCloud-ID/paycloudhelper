package phlogger

import (
	"fmt"
	"time"

	"github.com/PayCloud-ID/paycloudhelper/phhelper"
	"github.com/kataras/golog"
	"github.com/kataras/pio"
)

var (
	Log  = golog.New()
	Logf = Log.Logf
)

// ── Internal emit helpers ────────────────────────────────────────────────────
// These bypass rate limiting and are the single point that calls golog + hooks.

func emitD(format string, args ...interface{}) {
	Log.Debugf(format, args...)
	fireHooks("debug", fmt.Sprintf(format, args...))
}

func emitI(format string, args ...interface{}) {
	Log.Infof(format, args...)
	fireHooks("info", fmt.Sprintf(format, args...))
}

func emitW(format string, args ...interface{}) {
	Log.Warnf(format, args...)
	fireHooks("warn", fmt.Sprintf(format, args...))
}

func emitE(format string, args ...interface{}) {
	Log.Errorf(format, args...)
	fireHooks("error", fmt.Sprintf(format, args...))
}

// fatalHandler is the function that performs the final fatal log write and
// process termination. It is a package-level variable so tests can replace it
// with a no-op to prevent os.Exit without changing production behaviour.
var fatalHandler func(string, ...interface{}) = func(format string, args ...interface{}) {
	Log.Fatalf(format, args...)
}

func emitF(format string, args ...interface{}) {
	// Hooks fire BEFORE fatalHandler because Log.Fatalf calls os.Exit.
	fireHooks("fatal", fmt.Sprintf(format, args...))
	fatalHandler(format, args...)
}

// ── Public log functions ──────────────────────────────────────────────────────
// Debug, Info, and Warn are rate-limited using the format string as key via the
// global sampler. In production: first 5 per second per key, then every 50th.
// In dev: no sampling. Configurable via InitializeSampler() or APP_ENV.
//
// Error and Fatal are NEVER sampled:
//   - A sampled LogE would silently drop error lines and skip fireHooks("error"),
//     causing Sentry/log-forwarding hooks to miss per-transaction detail (ref_no).
//   - A sampled LogF would return WITHOUT calling os.Exit, letting the process
//     continue after a fatal condition — a critical safety violation for payment
//     services.

// LogD logs at Debug level.
func LogD(format string, args ...interface{}) {
	allowed, suppressed := globalSampler.check(format)
	if !allowed {
		return
	}
	if suppressed > 0 {
		format += " [+%d suppressed]"
		args = append(args, suppressed)
	}
	emitD(format, args...)
}

// LogI logs at Info level.
func LogI(format string, args ...interface{}) {
	allowed, suppressed := globalSampler.check(format)
	if !allowed {
		return
	}
	if suppressed > 0 {
		format += " [+%d suppressed]"
		args = append(args, suppressed)
	}
	emitI(format, args...)
}

// LogW logs at Warning level.
func LogW(format string, args ...interface{}) {
	allowed, suppressed := globalSampler.check(format)
	if !allowed {
		return
	}
	if suppressed > 0 {
		format += " [+%d suppressed]"
		args = append(args, suppressed)
	}
	emitW(format, args...)
}

// LogE logs at Error level. Never sampled — all errors must reach hooks and
// log-forwarding/Sentry backends regardless of burst volume. Sampling would
// silently drop per-transaction error detail (ref_no, merchant_id) during
// bursts and skip fireHooks("error"), breaking Sentry/log-forwarding.
func LogE(format string, args ...interface{}) {
	emitE(format, args...)
}

// LogF logs at Fatal level. Never sampled — a LogF that does not reach
// os.Exit is a critical safety violation: the process would continue executing
// after a condition the caller declared unrecoverable.
// Hooks fire synchronously before os.Exit.
func LogF(format string, args ...interface{}) {
	emitF(format, args...)
}

var GinLevel golog.Level = 6

func InitializeLogger() {
	Log.SetTimeFormat("2006-01-02 15:04:05.000")
	Log.SetLevel("info")

	golog.Levels[GinLevel] = &golog.LevelMetadata{
		Name:             "gin",
		AlternativeNames: []string{"http-server"},
		Title:            "[GIN]",
		ColorCode:        pio.Green,
	}

	// Initialize sampler based on APP_ENV.
	InitializeSampler(SamplerConfigFromAppEnv())
}

func LogSetLevel(levelName string) {
	Log.SetLevel(levelName)
}

// LogJ logs arg as compact JSON.
func LogJ(arg interface{}) {
	data := phhelper.ToJson(arg)
	LogI("%s %s", phhelper.BuildLogPrefix("LogJ"), data)
}

// LogJI logs arg as indented JSON.
func LogJI(arg interface{}) {
	data := phhelper.ToJsonIndent(arg)
	LogI("%s %s", phhelper.BuildLogPrefix("LogJI"), data)
}

// LogErr logs an error value.
func LogErr(err error) {
	Log.Error(err)
}

// ── Rate-limited variants with custom key ─────────────────────────────────────
// Use these when the format string is not a stable identifier (e.g. it contains
// a variable value). key should be a stable string like "cache.miss".
//
// LogIRated — uses the global sampler with a custom key.
// LogIRatedW — uses a simple time-window limiter with explicit window for cases
//              where the sampler pattern is not appropriate.

// LogIRated logs at Info level with sampling using a custom key.
func LogIRated(key string, format string, args ...interface{}) {
	logSampled(key, emitI, format, args...)
}

// LogERated logs at Error level with sampling using a custom key.
func LogERated(key string, format string, args ...interface{}) {
	logSampled(key, emitE, format, args...)
}

// LogWRated logs at Warning level with sampling using a custom key.
func LogWRated(key string, format string, args ...interface{}) {
	logSampled(key, emitW, format, args...)
}

// LogDRated logs at Debug level with sampling using a custom key.
func LogDRated(key string, format string, args ...interface{}) {
	logSampled(key, emitD, format, args...)
}

// LogIRatedW logs at Info level with a time-window limiter using a custom key and explicit window.
func LogIRatedW(key string, window time.Duration, format string, args ...interface{}) {
	logWindowed(key, window, emitI, format, args...)
}

// LogERatedW logs at Error level with a time-window limiter.
func LogERatedW(key string, window time.Duration, format string, args ...interface{}) {
	logWindowed(key, window, emitE, format, args...)
}

// LogWRatedW logs at Warning level with a time-window limiter.
func LogWRatedW(key string, window time.Duration, format string, args ...interface{}) {
	logWindowed(key, window, emitW, format, args...)
}

// LogDRatedW logs at Debug level with a time-window limiter.
func LogDRatedW(key string, window time.Duration, format string, args ...interface{}) {
	logWindowed(key, window, emitD, format, args...)
}

// logSampled uses the global sampler with a custom key.
func logSampled(key string, emit func(string, ...interface{}), format string, args ...interface{}) {
	allowed, suppressed := globalSampler.check(key)
	if !allowed {
		return
	}
	if suppressed > 0 {
		format += " [+%d suppressed]"
		args = append(args, suppressed)
	}
	emit(format, args...)
}

// logWindowed uses the old time-window rate limiter for explicit window control.
func logWindowed(key string, window time.Duration, emit func(string, ...interface{}), format string, args ...interface{}) {
	allowed, suppressed := globalRateLimiter.check(key, window)
	if !allowed {
		return
	}
	if suppressed > 0 {
		format += " [+%d suppressed]"
		args = append(args, suppressed)
	}
	emit(format, args...)
}
