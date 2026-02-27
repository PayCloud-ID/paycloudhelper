package phlogger

import "os"

// LogForwardConfig controls which log levels are forwarded to an external sink (e.g. Sentry).
// By default only Fatal forwarding is enabled when Sentry is initialized.
type LogForwardConfig struct {
	ForwardDebug bool // Forward Debug logs (default: false)
	ForwardInfo  bool // Forward Info logs (default: false)
	ForwardWarn  bool // Forward Warning logs (default: false)
	ForwardError bool // Forward Error logs (default: true)
	ForwardFatal bool // Forward Fatal logs (default: true)
}

// LogForwardConfigFromEnv constructs a LogForwardConfig reading from environment variables.
// Variables: LOG_FORWARD_FATAL (default "true"), LOG_FORWARD_ERROR (default "true"),
// LOG_FORWARD_WARN, LOG_FORWARD_INFO, LOG_FORWARD_DEBUG (default "false").
func LogForwardConfigFromEnv() LogForwardConfig {
	return LogForwardConfig{
		ForwardFatal: getEnvBool("LOG_FORWARD_FATAL", true),
		ForwardError: getEnvBool("LOG_FORWARD_ERROR", true),
		ForwardWarn:  getEnvBool("LOG_FORWARD_WARN", false),
		ForwardInfo:  getEnvBool("LOG_FORWARD_INFO", false),
		ForwardDebug: getEnvBool("LOG_FORWARD_DEBUG", false),
	}
}

// getEnvBool reads an env var and returns its boolean value, defaulting to def if unset or unparseable.
func getEnvBool(key string, def bool) bool {
	val := os.Getenv(key)
	if val == "" {
		return def
	}
	return val == "1" || val == "true" || val == "yes"
}
