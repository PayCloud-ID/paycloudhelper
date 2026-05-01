package phtrace

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// DefaultPhaseBuckets are the histogram bucket boundaries (milliseconds) used
// by QR-MPM phase timing. They follow a power-of-two-ish progression tuned to
// the latency analysis in docs/2026-04-22-qrmpm-performance-analysis.md so
// p50/p99 sit on distinct buckets.
//
// Units: milliseconds.
var DefaultPhaseBuckets = []float64{
	5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000,
}

// PhaseHistogram wraps a Float64Histogram pre-configured with milliseconds
// unit and QR-MPM phase bucket boundaries. Thread-safe; create once at
// startup per service and reuse across goroutines.
type PhaseHistogram struct {
	hist metric.Float64Histogram
	name string
}

// phaseHistRegistry caches per-name histograms so callers can fetch-or-create
// without tracking references across packages. Keyed by (meterName, histName).
var (
	phaseHistMu sync.Mutex
	phaseHists  = map[string]*PhaseHistogram{}
)

// NewPhaseHistogram creates (or returns a cached) phase-duration histogram.
// The meterName selects which instrumentation scope the instrument belongs to
// (typically the service name). Safe to call multiple times; subsequent calls
// with the same (meterName, histName) return the cached instrument.
//
// When phtrace is disabled this returns a no-op histogram that records nothing.
func NewPhaseHistogram(meterName, histName string, buckets []float64) (*PhaseHistogram, error) {
	phaseHistMu.Lock()
	defer phaseHistMu.Unlock()

	key := meterName + "|" + histName
	if h, ok := phaseHists[key]; ok {
		return h, nil
	}

	m := Meter(meterName)
	if len(buckets) == 0 {
		buckets = DefaultPhaseBuckets
	}
	h, err := m.Float64Histogram(histName,
		metric.WithUnit("ms"),
		metric.WithDescription("QR-MPM transaction phase duration in milliseconds"),
		metric.WithExplicitBucketBoundaries(buckets...),
	)
	if err != nil {
		return nil, err
	}

	ph := &PhaseHistogram{hist: h, name: histName}
	phaseHists[key] = ph
	return ph, nil
}

// MustPhaseHistogram is like NewPhaseHistogram but panics on error. Use only at
// process startup for instrumentation registration mistakes; it is not intended
// for network or broker I/O paths — use [NewPhaseHistogram] when errors should
// propagate to the caller.
func MustPhaseHistogram(meterName, histName string, buckets []float64) *PhaseHistogram {
	h, err := NewPhaseHistogram(meterName, histName, buckets)
	if err != nil {
		panic("phtrace: " + err.Error())
	}
	return h
}

// Record logs duration (ms) tagged with the given phase and any extra
// attributes. Zero and negative durations are accepted but may indicate a
// clock bug upstream and are left to the caller's judgment.
func (p *PhaseHistogram) Record(ctx context.Context, phase string, duration time.Duration, extra ...attribute.KeyValue) {
	if p == nil || p.hist == nil {
		return
	}
	attrs := make([]attribute.KeyValue, 0, 1+len(extra))
	attrs = append(attrs, attribute.String("phase", phase))
	attrs = append(attrs, extra...)
	p.hist.Record(ctx, float64(duration.Milliseconds()), metric.WithAttributes(attrs...))
}

// Observe is a convenience helper returning a done func that records elapsed
// time when deferred. Example:
//
//	defer phaseHist.Observe(ctx, "rmq_publish")()
func (p *PhaseHistogram) Observe(ctx context.Context, phase string, extra ...attribute.KeyValue) func() {
	start := time.Now()
	return func() {
		p.Record(ctx, phase, time.Since(start), extra...)
	}
}
