package paycloudhelper

import (
	"os"
	"strconv"
	"time"

	"bitbucket.org/paycloudid/paycloudhelper/phlogger"
	"bitbucket.org/paycloudid/paycloudhelper/phsentry"
	"github.com/kataras/golog"
)

// ── Re-exported types for consumer convenience ────────────────────────────────

// SamplerConfig controls log sampling behavior per key per period.
// See phlogger.SamplerConfig for full documentation.
type SamplerConfig = phlogger.SamplerConfig

// LogContext is a child logger with key-value context prefix.
// See phlogger.LogContext for full documentation.
type LogContext = phlogger.LogContext

// MetricsHook is a callback for high-frequency event counting.
// See phlogger.MetricsHook for full documentation.
type MetricsHook = phlogger.MetricsHook

// KeyedLimiter provides per-key token bucket rate limiting.
// See phlogger.KeyedLimiter for full documentation.
type KeyedLimiter = phlogger.KeyedLimiter

var (
	Log                  = phlogger.Log
	Logf                 = Log.Logf
	GinLevel golog.Level = phlogger.GinLevel
)

// LogD logs at Debug level (delegates to phlogger wrapper with hooks).
func LogD(format string, args ...interface{}) {
	phlogger.LogD(format, args...)
}

// LogI logs at Info level (delegates to phlogger wrapper with hooks).
func LogI(format string, args ...interface{}) {
	phlogger.LogI(format, args...)
}

// LogW logs at Warning level (delegates to phlogger wrapper with hooks).
func LogW(format string, args ...interface{}) {
	phlogger.LogW(format, args...)
}

// LogE logs at Error level (delegates to phlogger wrapper with hooks).
func LogE(format string, args ...interface{}) {
	phlogger.LogE(format, args...)
}

// LogF logs at Fatal level (process exits after hook execution).
func LogF(format string, args ...interface{}) {
	phlogger.LogF(format, args...)
}

// LogIRated logs at Info level with rate limiting using key and the default 50ms window.
func LogIRated(key string, format string, args ...interface{}) {
	phlogger.LogIRated(key, format, args...)
}

// LogERated logs at Error level with rate limiting using key and the default 50ms window.
func LogERated(key string, format string, args ...interface{}) {
	phlogger.LogERated(key, format, args...)
}

// LogWRated logs at Warning level with rate limiting using key and the default 50ms window.
func LogWRated(key string, format string, args ...interface{}) {
	phlogger.LogWRated(key, format, args...)
}

// LogDRated logs at Debug level with rate limiting using key and the default 50ms window.
func LogDRated(key string, format string, args ...interface{}) {
	phlogger.LogDRated(key, format, args...)
}

// LogIRatedW logs at Info level with rate limiting using key and an explicit window.
func LogIRatedW(key string, window time.Duration, format string, args ...interface{}) {
	phlogger.LogIRatedW(key, window, format, args...)
}

// LogERatedW logs at Error level with rate limiting using key and an explicit window.
func LogERatedW(key string, window time.Duration, format string, args ...interface{}) {
	phlogger.LogERatedW(key, window, format, args...)
}

// LogWRatedW logs at Warning level with rate limiting using key and an explicit window.
func LogWRatedW(key string, window time.Duration, format string, args ...interface{}) {
	phlogger.LogWRatedW(key, window, format, args...)
}

// LogDRatedW logs at Debug level with rate limiting using key and an explicit window.
func LogDRatedW(key string, window time.Duration, format string, args ...interface{}) {
	phlogger.LogDRatedW(key, window, format, args...)
}

// ── Sampler configuration ─────────────────────────────────────────────────────

// InitializeSampler sets the global sampler config.
// Called automatically by InitializeLogger() with env-aware defaults.
// Use to override defaults at startup.
func InitializeSampler(cfg SamplerConfig) {
	phlogger.InitializeSampler(cfg)
}

// SamplerConfigForEnv returns production-tuned sampler defaults for the given environment.
func SamplerConfigForEnv(env string) SamplerConfig {
	return phlogger.SamplerConfigForEnv(env)
}

// SamplerConfigFromAppEnv returns a SamplerConfig based on APP_ENV.
func SamplerConfigFromAppEnv() SamplerConfig {
	return phlogger.SamplerConfigFromAppEnv()
}

// ── Context logger ────────────────────────────────────────────────────────────

// NewLogContext creates a child logger with key-value context fields.
// Fields are prepended as [key=value key2=value2] to every message.
func NewLogContext(fields ...string) *LogContext {
	return phlogger.NewLogContext(fields...)
}

// ── Metrics hooks ─────────────────────────────────────────────────────────────

// RegisterMetricsHook sets the global metrics callback for high-frequency events.
// Consumers wire their own backend (prometheus, statsd, etc.).
func RegisterMetricsHook(hook MetricsHook) {
	phlogger.RegisterMetricsHook(hook)
}

// IncrementMetric records one occurrence of a named event via the registered hook.
func IncrementMetric(event string) {
	phlogger.IncrementMetric(event)
}

// IncrementMetricBy records `n` occurrences of a named event via the registered hook.
func IncrementMetricBy(event string, n int64) {
	phlogger.IncrementMetricBy(event, n)
}

// ── KeyedLimiter (token bucket) ───────────────────────────────────────────────

// NewKeyedLimiter creates a per-key token bucket limiter.
// r is events/second, burst is max burst per key.
func NewKeyedLimiter(r float64, burst int) *KeyedLimiter {
	return phlogger.NewKeyedLimiter(r, burst)
}

func InitializeLogger() {
	phlogger.InitializeLogger()
}

func LogSetLevel(levelName string) {
	phlogger.LogSetLevel(levelName)
}

// LogJ logs arg as compact JSON.
func LogJ(arg interface{}) {
	phlogger.LogJ(arg)
}

// LogJI logs arg as indented JSON.
func LogJI(arg interface{}) {
	phlogger.LogJI(arg)
}

// LogErr logs an error value.
func LogErr(err error) {
	phlogger.LogErr(err)
}

// ConfigureLogForwarding registers Sentry forwarding hooks based on cfg.
// Call once at startup AFTER InitSentry(). Safe to call multiple times —
// each call adds hooks cumulatively; use phlogger.ClearLogHooks() if you need a reset.
//
// Example (startup):
//
//	pch.InitSentry(pch.SentryOptions{...})
//	pch.ConfigureLogForwarding(pch.LogForwardConfigFromEnv())
func ConfigureLogForwarding(cfg phlogger.LogForwardConfig) {
	if cfg.ForwardFatal {
		phlogger.RegisterLogHook("fatal", func(level, message string) {
			phsentry.ReceiveLog(level, message)
		})
	}
	if cfg.ForwardError {
		phlogger.RegisterLogHook("error", func(level, message string) {
			phsentry.ReceiveLog(level, message)
		})
	}
	if cfg.ForwardWarn {
		phlogger.RegisterLogHook("warn", func(level, message string) {
			phsentry.ReceiveLog(level, message)
		})
	}
	if cfg.ForwardInfo {
		phlogger.RegisterLogHook("info", func(level, message string) {
			phsentry.ReceiveLog(level, message)
		})
	}
	if cfg.ForwardDebug {
		phlogger.RegisterLogHook("debug", func(level, message string) {
			phsentry.ReceiveLog(level, message)
		})
	}
}

// LogForwardConfigFromEnv returns a LogForwardConfig loaded from environment variables.
// See phlogger.LogForwardConfigFromEnv for variable names and defaults.
func LogForwardConfigFromEnv() phlogger.LogForwardConfig {
	return phlogger.LogForwardConfigFromEnv()
}

// ConfigureSentryLogging enables or disables forwarding of all log levels to Sentry.
// When enabled (true), all logs emitted via LogI, LogE, LogW, LogD, LogF are forwarded
// to Sentry as structured events and breadcrumbs.
//
// Call once at startup AFTER InitSentry() has completed.
// This is the recommended simpler API compared to ConfigureLogForwarding for basic setup.
//
// Example (startup):
//
//	pch.InitSentry(pch.SentryOptions{...})
//	pch.ConfigureSentryLogging(true)  // enable all log levels to Sentry
//	pch.LogE("[Main] startup error: %v", err)  // forwarded to Sentry
//
// To enable via environment variable (default):
//
//	// In your service
//	enableSentryLogging := os.Getenv("SENTRY_LOGGING") == "true"
//	pch.ConfigureSentryLogging(enableSentryLogging)
func ConfigureSentryLogging(enable bool) {
	if enable {
		phsentry.RegisterSentryLogHook()
	}
}

// SentryLoggingFromEnv loads the SENTRY_LOGGING environment variable and returns its boolean value.
// Parses common boolean formats: "1", "t", "T", "true", "TRUE", "True", "0", "f", "F", "false", "FALSE", "False".
// Returns false for invalid or unset values (default behavior).
//
// Call once at startup AFTER InitSentry() has completed, then pass result to ConfigureSentryLogging.
// This is the recommended pattern for environment-driven sentry logging control.
//
// Example (typical service startup):
//
//	pch.InitSentry(pch.SentryOptions{Dsn: os.Getenv("SENTRY_DSN"), ...})
//	pch.ConfigureSentryLogging(pch.SentryLoggingFromEnv())
//
// Environment variable examples:
//
//	SENTRY_LOGGING=true   → enable
//	SENTRY_LOGGING=1      → enable
//	SENTRY_LOGGING=false  → disable (default)
//	SENTRY_LOGGING=0      → disable (default)
//	(unset)               → disable (default)
func SentryLoggingFromEnv() bool {
	val := os.Getenv("SENTRY_LOGGING")
	if val == "" {
		return false // unset defaults to false
	}
	parsed, err := strconv.ParseBool(val)
	if err != nil {
		return false // invalid values default to false
	}
	return parsed
}
