package phtrace

import (
	"context"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func TestAMQPCarrier_GetSet(t *testing.T) {
	h := amqp.Table{}
	c := NewAMQPCarrier(h)
	c.Set("traceparent", "00-abc-def-01")
	if got := c.Get("traceparent"); got != "00-abc-def-01" {
		t.Fatalf("want 00-abc-def-01, got %q", got)
	}
	if got := c.Get("nope"); got != "" {
		t.Fatalf("absent key must return empty, got %q", got)
	}
}

func TestAMQPCarrier_StringAndBytes(t *testing.T) {
	h := amqp.Table{
		"k1": "s",
		"k2": []byte("b"),
		"k3": 42,
	}
	c := NewAMQPCarrier(h)
	if c.Get("k1") != "s" {
		t.Errorf("string: %q", c.Get("k1"))
	}
	if c.Get("k2") != "b" {
		t.Errorf("bytes: %q", c.Get("k2"))
	}
	if c.Get("k3") != "" {
		t.Errorf("non-string typed values must normalize to empty, got %q", c.Get("k3"))
	}
}

func TestAMQPCarrier_Keys(t *testing.T) {
	h := amqp.Table{"a": "1", "b": "2"}
	keys := NewAMQPCarrier(h).Keys()
	if len(keys) != 2 {
		t.Errorf("want 2 keys, got %d (%v)", len(keys), keys)
	}
}

func TestInjectExtract_RoundTrip(t *testing.T) {
	// Use the baseline W3C TraceContext propagator (Propagator() falls back to
	// it when phtrace is disabled, which it is during unit tests).
	prop := propagation.TraceContext{}

	tid, _ := trace.TraceIDFromHex("0102030405060708090a0b0c0d0e0f10")
	sid, _ := trace.SpanIDFromHex("1112131415161718")
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    tid,
		SpanID:     sid,
		TraceFlags: trace.FlagsSampled,
		Remote:     true,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)

	headers := amqp.Table{}
	prop.Inject(ctx, NewAMQPCarrier(headers))
	if _, ok := headers["traceparent"]; !ok {
		t.Fatalf("traceparent header was not injected: %v", headers)
	}

	extracted := prop.Extract(context.Background(), NewAMQPCarrier(headers))
	got := trace.SpanContextFromContext(extracted)
	if got.TraceID() != tid {
		t.Errorf("trace id mismatch: want %s, got %s", tid, got.TraceID())
	}
	if got.SpanID() != sid {
		t.Errorf("span id mismatch: want %s, got %s", sid, got.SpanID())
	}
}

func TestInjectAMQP_NilCtxAndHeaders(t *testing.T) {
	if out := InjectAMQP(nil, nil); out != nil {
		t.Errorf("nil ctx must short-circuit and return original nil headers")
	}
	got := InjectAMQP(context.Background(), nil)
	if got == nil {
		t.Errorf("nil headers with valid ctx should allocate a new Table")
	}
}

func TestExtractAMQP_NilHeaders(t *testing.T) {
	ctx := ExtractAMQP(nil, nil)
	if ctx == nil {
		t.Errorf("nil ctx input must not return nil context")
	}
}
