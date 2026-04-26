package phtrace

import (
	"context"
	"testing"
	"time"
)

func TestEnsureRequired_validation(t *testing.T) {
	if err := ensureRequired(Config{Enabled: true}); err == nil {
		t.Fatal("expected error for missing service name/endpoint/dial timeout")
	}
	if err := ensureRequired(Config{Enabled: true, ServiceName: "s"}); err == nil {
		t.Fatal("expected error for missing endpoint")
	}
	if err := ensureRequired(Config{Enabled: true, ServiceName: "s", Endpoint: "x"}); err == nil {
		t.Fatal("expected error for missing dial timeout")
	}
	if err := ensureRequired(Config{Enabled: true, ServiceName: "s", Endpoint: "x", DialTimeout: time.Second, SamplingRatio: 2}); err == nil {
		t.Fatal("expected error for invalid sampling ratio")
	}
}

func TestBuildResource_nonNil(t *testing.T) {
	res, err := buildResource(context.Background(), Config{
		Enabled:        true,
		ServiceName:    "svc",
		ServiceVersion: "v1",
		Environment:    "test",
		ResourceAttributes: map[string]string{
			"k": "v",
		},
	})
	if err != nil {
		t.Fatalf("buildResource: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil resource")
	}
}

func TestTracer_disabledIsNoop(t *testing.T) {
	enabled.Store(false)
	tr := Tracer("x")
	if tr == nil {
		t.Fatal("expected non-nil tracer")
	}
}

func TestNopMeter_instrumentsNoPanic(t *testing.T) {
	enabled.Store(false)
	m := Meter("x")
	c, err := m.Int64Counter("c")
	if err != nil || c == nil {
		t.Fatalf("Int64Counter err=%v c=%v", err, c)
	}
	c.Add(context.Background(), 1)
}

func TestErrorHandler_Handle_noPanic(t *testing.T) {
	var h errorHandler
	h.Handle(nil)
	h.Handle(context.Canceled)
}

func TestNopMeter_allFactoriesAndNoOpMethods(t *testing.T) {
	enabled.Store(false)
	m := Meter("x")

	up, _ := m.Int64UpDownCounter("u")
	up.Add(context.Background(), -1)

	h, _ := m.Int64Histogram("h")
	h.Record(context.Background(), 1)

	g, _ := m.Int64Gauge("g")
	g.Record(context.Background(), 1)

	fc, _ := m.Float64Counter("fc")
	fc.Add(context.Background(), 1.5)

	fu, _ := m.Float64UpDownCounter("fu")
	fu.Add(context.Background(), -1.5)

	fh, _ := m.Float64Histogram("fh")
	fh.Record(context.Background(), 2.5)

	fg, _ := m.Float64Gauge("fg")
	fg.Record(context.Background(), 3.5)
}

func TestBuildTracerProvider_errorPath(t *testing.T) {
	res, err := buildResource(context.Background(), Config{
		Enabled:        true,
		ServiceName:    "svc",
		ServiceVersion: "v1",
		Environment:    "test",
		ResourceAttributes: map[string]string{
			"k": "v",
		},
	})
	if err != nil {
		t.Fatalf("resource: %v", err)
	}

	tp, exp, err := buildTracerProvider(context.Background(), Config{
		Enabled:       true,
		ServiceName:   "svc",
		Endpoint:      "127.0.0.1:0", // grpc client creation is lazy; should not hang with short timeout
		Insecure:      true,
		DialTimeout:   5 * time.Millisecond,
		SamplingRatio: 1,
	}, res)
	if err == nil {
		// We accept success here (grpc client is lazy). This test exists to cover the builder path.
		t.Cleanup(func() {
			if exp != nil {
				_ = exp.Shutdown(context.Background())
			}
		})
		_ = tp.Shutdown(context.Background())
		return
	}
}

func TestBuildMeterProvider_errorPath(t *testing.T) {
	res, err := buildResource(context.Background(), Config{
		Enabled:        true,
		ServiceName:    "svc",
		ServiceVersion: "v1",
		Environment:    "test",
		ResourceAttributes: map[string]string{
			"k": "v",
		},
	})
	if err != nil {
		t.Fatalf("resource: %v", err)
	}

	mp, exp, err := buildMeterProvider(context.Background(), Config{
		Enabled:              true,
		ServiceName:          "svc",
		Endpoint:             "127.0.0.1:0",
		Insecure:             true,
		DialTimeout:          5 * time.Millisecond,
		MetricExportInterval: 10 * time.Millisecond,
		SamplingRatio:        1,
	}, res)
	if err == nil {
		t.Cleanup(func() {
			if exp != nil {
				_ = exp.Shutdown(context.Background())
			}
		})
		_ = mp.Shutdown(context.Background())
		return
	}
}
