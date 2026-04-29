package phtrace

import (
	"context"
	"fmt"

	"github.com/PayCloud-ID/paycloudhelper/phlogger"
	"go.opentelemetry.io/otel/trace"
)

// Standard log field keys used across all PayCloud services so Loki/Grafana
// queries are consistent. These are the canonical names — always use these
// constants instead of raw strings when adding fields via WithFields.
const (
	FieldTraceID    = "trace_id"
	FieldSpanID     = "span_id"
	FieldTicketID   = "ticket_id"
	FieldReffNo     = "reff_no"
	FieldMerchantID = "merchant_id"
	FieldOrderID    = "order_id"
	FieldTrxID      = "trx_id"
	FieldTrxNo      = "trx_no"
	FieldService    = "service"
	FieldRoute      = "route"
	FieldVendor     = "vendor"
)

// traceFields extracts trace_id and span_id from ctx when a valid span is
// recording. Returns two zero-length strings when ctx has no span so callers
// can skip the fields without allocating.
func traceFields(ctx context.Context) (traceID, spanID string) {
	if ctx == nil {
		return "", ""
	}
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return "", ""
	}
	return sc.TraceID().String(), sc.SpanID().String()
}

// buildPrefix creates the `[trace_id=... span_id=... k=v ...] ` prefix string
// that is prepended to log messages. Fields must be alternating key, value
// pairs; odd trailing keys are silently dropped (mirrors phlogger.LogContext).
func buildPrefix(ctx context.Context, fields []string) string {
	traceID, spanID := traceFields(ctx)
	if traceID == "" && len(fields) == 0 {
		return ""
	}
	// Preallocate a generous buffer; log prefixes are typically < 200 bytes.
	buf := make([]byte, 0, 128+len(fields)*16)
	buf = append(buf, '[')
	first := true
	if traceID != "" {
		buf = append(buf, FieldTraceID...)
		buf = append(buf, '=')
		buf = append(buf, traceID...)
		first = false
	}
	if spanID != "" {
		if !first {
			buf = append(buf, ' ')
		}
		buf = append(buf, FieldSpanID...)
		buf = append(buf, '=')
		buf = append(buf, spanID...)
		first = false
	}
	for i := 0; i+1 < len(fields); i += 2 {
		if !first {
			buf = append(buf, ' ')
		}
		buf = append(buf, fields[i]...)
		buf = append(buf, '=')
		buf = append(buf, fields[i+1]...)
		first = false
	}
	buf = append(buf, ']', ' ')
	return string(buf)
}

// LogDCtx logs at Debug level with `[trace_id=... span_id=...] ` prefix drawn
// from ctx. Safe when ctx has no active span — the prefix is simply omitted.
func LogDCtx(ctx context.Context, format string, args ...interface{}) {
	if p := buildPrefix(ctx, nil); p != "" {
		phlogger.LogD(p+format, args...)
		return
	}
	phlogger.LogD(format, args...)
}

// LogICtx logs at Info level with trace context.
func LogICtx(ctx context.Context, format string, args ...interface{}) {
	if p := buildPrefix(ctx, nil); p != "" {
		phlogger.LogI(p+format, args...)
		return
	}
	phlogger.LogI(format, args...)
}

// LogWCtx logs at Warning level with trace context.
func LogWCtx(ctx context.Context, format string, args ...interface{}) {
	if p := buildPrefix(ctx, nil); p != "" {
		phlogger.LogW(p+format, args...)
		return
	}
	phlogger.LogW(format, args...)
}

// LogECtx logs at Error level with trace context. Additionally records an
// exception event on the active span (if any) so errors show up in Tempo.
func LogECtx(ctx context.Context, format string, args ...interface{}) {
	if span := trace.SpanFromContext(ctx); span.IsRecording() {
		span.RecordError(fmt.Errorf(format, args...))
	}
	if p := buildPrefix(ctx, nil); p != "" {
		phlogger.LogE(p+format, args...)
		return
	}
	phlogger.LogE(format, args...)
}

// WithFields returns a LogContextCtx carrying ctx plus an immutable set of
// extra fields prefixed to every log line. Use this when a function logs
// multiple times for the same logical operation and you want consistent fields.
//
// Example:
//
//	lc := phtrace.WithFields(ctx, phtrace.FieldTicketID, ticketID, phtrace.FieldReffNo, reff)
//	lc.LogI("qrmpm start")
//	lc.LogI("qrmpm done status=%s", status)
func WithFields(ctx context.Context, fields ...string) *LogContextCtx {
	return &LogContextCtx{ctx: ctx, fields: append([]string(nil), fields...)}
}

// LogContextCtx is a trace-aware sibling of phlogger.LogContext. Prefix is
// rebuilt per log call so trace/span IDs reflect the currently-active span
// on ctx (useful when a parent context spawns child spans).
type LogContextCtx struct {
	ctx    context.Context
	fields []string
}

// With returns a new LogContextCtx extending this one with extra fields.
func (l *LogContextCtx) With(fields ...string) *LogContextCtx {
	if len(fields) == 0 {
		return l
	}
	merged := make([]string, 0, len(l.fields)+len(fields))
	merged = append(merged, l.fields...)
	merged = append(merged, fields...)
	return &LogContextCtx{ctx: l.ctx, fields: merged}
}

// Ctx returns the underlying context (helpful when spawning child spans).
func (l *LogContextCtx) Ctx() context.Context { return l.ctx }

// LogD logs at Debug level with trace + field prefix.
func (l *LogContextCtx) LogD(format string, args ...interface{}) {
	phlogger.LogD(buildPrefix(l.ctx, l.fields)+format, args...)
}

// LogI logs at Info level with trace + field prefix.
func (l *LogContextCtx) LogI(format string, args ...interface{}) {
	phlogger.LogI(buildPrefix(l.ctx, l.fields)+format, args...)
}

// LogW logs at Warning level with trace + field prefix.
func (l *LogContextCtx) LogW(format string, args ...interface{}) {
	phlogger.LogW(buildPrefix(l.ctx, l.fields)+format, args...)
}

// LogE logs at Error level with trace + field prefix (records span exception).
func (l *LogContextCtx) LogE(format string, args ...interface{}) {
	if span := trace.SpanFromContext(l.ctx); span.IsRecording() {
		span.RecordError(fmt.Errorf(format, args...))
	}
	phlogger.LogE(buildPrefix(l.ctx, l.fields)+format, args...)
}
