package phtrace

import (
	"context"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestBuildPrefix_NoSpanNoFields(t *testing.T) {
	if p := buildPrefix(context.Background(), nil); p != "" {
		t.Errorf("empty prefix expected, got %q", p)
	}
}

func TestBuildPrefix_WithSpanOnly(t *testing.T) {
	tid, _ := trace.TraceIDFromHex("0102030405060708090a0b0c0d0e0f10")
	sid, _ := trace.SpanIDFromHex("1112131415161718")
	sc := trace.NewSpanContext(trace.SpanContextConfig{TraceID: tid, SpanID: sid})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)

	p := buildPrefix(ctx, nil)
	if !strings.Contains(p, "trace_id=0102030405060708090a0b0c0d0e0f10") {
		t.Errorf("trace_id missing: %q", p)
	}
	if !strings.Contains(p, "span_id=1112131415161718") {
		t.Errorf("span_id missing: %q", p)
	}
	if !strings.HasSuffix(p, "] ") {
		t.Errorf("prefix must end with '] ', got %q", p)
	}
}

func TestBuildPrefix_WithFields(t *testing.T) {
	p := buildPrefix(context.Background(), []string{FieldTicketID, "T-1", FieldReffNo, "R-2"})
	if !strings.Contains(p, "ticket_id=T-1") {
		t.Errorf("ticket_id missing: %q", p)
	}
	if !strings.Contains(p, "reff_no=R-2") {
		t.Errorf("reff_no missing: %q", p)
	}
}

func TestBuildPrefix_OddFieldCount(t *testing.T) {
	p := buildPrefix(context.Background(), []string{"a", "1", "dangling"})
	if !strings.Contains(p, "a=1") {
		t.Errorf("a=1 missing: %q", p)
	}
	if strings.Contains(p, "dangling") {
		t.Errorf("odd trailing key must be dropped, got %q", p)
	}
}

func TestWithFields_Chain(t *testing.T) {
	lc := WithFields(context.Background(), "a", "1").With("b", "2")
	if len(lc.fields) != 4 {
		t.Errorf("expected 4 fields after With, got %d", len(lc.fields))
	}
	if lc.fields[0] != "a" || lc.fields[2] != "b" {
		t.Errorf("field order corrupted: %v", lc.fields)
	}
}
