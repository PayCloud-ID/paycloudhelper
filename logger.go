package paycloudhelper

import (
	"time"

	"bitbucket.org/paycloudid/paycloudhelper/phlogger"
	"bitbucket.org/paycloudid/paycloudhelper/phsentry"
	"github.com/kataras/golog"
)

var (
	Log      = phlogger.Log
	Logf     = Log.Logf
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

// LogIRated logs at Info level with rate limiting. key identifies the log site.
func LogIRated(key string, window time.Duration, format string, args ...interface{}) {
	phlogger.LogIRated(key, window, format, args...)
}

// LogERated logs at Error level with rate limiting.
func LogERated(key string, window time.Duration, format string, args ...interface{}) {
	phlogger.LogERated(key, window, format, args...)
}

// LogWRated logs at Warning level with rate limiting.
func LogWRated(key string, window time.Duration, format string, args ...interface{}) {
	phlogger.LogWRated(key, window, format, args...)
}

// LogDRated logs at Debug level with rate limiting.
func LogDRated(key string, window time.Duration, format string, args ...interface{}) {
	phlogger.LogDRated(key, window, format, args...)
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
