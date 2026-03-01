package phlogger

import "strings"

// LogContext is a child logger that prepends key-value context fields to every
// log message. Use it for request-scoped or operation-scoped logging where you
// want consistent context without repeating fields in every call.
//
// Thread-safe: the prefix is built once at creation time (immutable).
//
// Example:
//
//	ctx := NewLogContext("req_id", "abc-123", "merchant", "M001")
//	ctx.LogI("processing payment amount=%d", 5000)
//	// output: [req_id=abc-123 merchant=M001] processing payment amount=5000
type LogContext struct {
	prefix string
}

// NewLogContext creates a child logger with key-value context fields.
// Fields are provided as alternating key, value strings.
// Odd trailing keys are silently dropped.
//
// Example:
//
//	ctx := NewLogContext("req_id", "abc-123", "user", "U42")
func NewLogContext(fields ...string) *LogContext {
	if len(fields) == 0 {
		return &LogContext{}
	}

	var b strings.Builder
	b.WriteByte('[')
	pairs := 0
	for i := 0; i+1 < len(fields); i += 2 {
		if pairs > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(fields[i])
		b.WriteByte('=')
		b.WriteString(fields[i+1])
		pairs++
	}
	b.WriteString("] ")
	return &LogContext{prefix: b.String()}
}

// With returns a new LogContext that merges the parent's fields with additional
// key-value pairs. Useful for adding scope without losing parent context.
//
// Example:
//
//	parent := NewLogContext("req_id", "abc-123")
//	child := parent.With("step", "validate")
//	child.LogI("checking input") // [req_id=abc-123 step=validate] checking input
func (lc *LogContext) With(fields ...string) *LogContext {
	if len(fields) == 0 {
		return lc
	}
	extra := NewLogContext(fields...)
	if lc.prefix == "" {
		return extra
	}
	// Merge: "[parent_fields] " + "[extra_fields] " → "[parent_fields extra_fields] "
	merged := lc.prefix[:len(lc.prefix)-2] + " " + extra.prefix[1:]
	return &LogContext{prefix: merged}
}

// LogD logs at Debug level with context prefix.
func (lc *LogContext) LogD(format string, args ...interface{}) {
	LogD(lc.prefix+format, args...)
}

// LogI logs at Info level with context prefix.
func (lc *LogContext) LogI(format string, args ...interface{}) {
	LogI(lc.prefix+format, args...)
}

// LogW logs at Warning level with context prefix.
func (lc *LogContext) LogW(format string, args ...interface{}) {
	LogW(lc.prefix+format, args...)
}

// LogE logs at Error level with context prefix.
func (lc *LogContext) LogE(format string, args ...interface{}) {
	LogE(lc.prefix+format, args...)
}

// LogF logs at Fatal level with context prefix.
func (lc *LogContext) LogF(format string, args ...interface{}) {
	LogF(lc.prefix+format, args...)
}
