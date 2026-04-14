package phsentry

import (
	"sync"

	"bitbucket.org/paycloudid/paycloudhelper/phlogger"
)

var (
	// sentryLogHookOnce ensures RegisterSentryLogHook runs only once.
	sentryLogHookOnce sync.Once
	// sentryLogHookErr captures any error from RegisterSentryLogHook.
	sentryLogHookErr error
)

// RegisterSentryLogHook registers the Sentry log hook with phlogger.
// This allows all logs emitted via phlogger (LogI, LogE, etc.) to be forwarded to Sentry.
// Multiple calls are safe; only the first call takes effect.
//
// The hook integrates with the Sentry SDK's structured logging by:
// - Forwarding info/debug/warn logs as breadcrumbs
// - Forwarding error/fatal logs as exception events
// - Extracting [FunctionName] prefixes for issue grouping
//
// Call this function during service initialization after InitSentry() has completed.
// If called before InitSentry(), the hook will silently skip until a Sentry client is initialized.
//
// Example:
//
//	phsentry.InitSentry(options)
//	phsentry.RegisterSentryLogHook()
//	phlogger.LogE("[main] database error: %v", err)  // forwarded to Sentry
func RegisterSentryLogHook() {
	sentryLogHookOnce.Do(func() {
		// Register hook for all log levels
		phlogger.RegisterLogHook("debug", func(level, message string) {
			ReceiveLog(level, message)
		})
		phlogger.RegisterLogHook("info", func(level, message string) {
			ReceiveLog(level, message)
		})
		phlogger.RegisterLogHook("warn", func(level, message string) {
			ReceiveLog(level, message)
		})
		phlogger.RegisterLogHook("error", func(level, message string) {
			ReceiveLog(level, message)
		})
		phlogger.RegisterLogHook("fatal", func(level, message string) {
			ReceiveLog(level, message)
		})
	})
}
