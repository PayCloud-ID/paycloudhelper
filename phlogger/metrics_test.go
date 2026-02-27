package phlogger
package phlogger

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestIncrementMetric_NoHook_NoPanic(t *testing.T) {
	// With no hook registered, IncrementMetric should be a no-op.
	RegisterMetricsHook(nil)
	IncrementMetric("test.event") // must not panic
}

func TestIncrementMetric_CallsHook(t *testing.T) {
	var called atomic.Bool
	var gotEvent string
	var gotCount int64

	RegisterMetricsHook(func(event string, count int64) {
		called.Store(true)
		gotEvent = event
		gotCount = count
	})
	defer RegisterMetricsHook(nil)

	IncrementMetric("cache.miss")

	if !called.Load() {
		t.Fatal("hook should have been called")
	}
	if gotEvent != "cache.miss" {
		t.Fatalf("expected event 'cache.miss', got %q", gotEvent)
	}
	if gotCount != 1 {
		t.Fatalf("expected count 1, got %d", gotCount)
	}
}

func TestIncrementMetricBy_CustomCount(t *testing.T) {
	var gotCount int64
	RegisterMetricsHook(func(event string, count int64) {
		gotCount = count
	})
	defer RegisterMetricsHook(nil)

	IncrementMetricBy("batch.size", 42)

	if gotCount != 42 {
		t.Fatalf("expected count 42, got %d", gotCount)
	}
}

func TestRegisterMetricsHook_Replaces(t *testing.T) {
	var hookID atomic.Int64

	RegisterMetricsHook(func(event string, count int64) {
		hookID.Store(1)
	})
	IncrementMetric("test")
	if hookID.Load() != 1 {
		t.Fatal("first hook should be called")
	}

	RegisterMetricsHook(func(event string, count int64) {
		hookID.Store(2)
	})
	IncrementMetric("test")
	if hookID.Load() != 2 {
		t.Fatal("second hook should replace first")
	}

	RegisterMetricsHook(nil)
}

func TestIncrementMetric_ConcurrentSafe(t *testing.T) {
	var total atomic.Int64
	RegisterMetricsHook(func(event string, count int64) {
		total.Add(count)
	})
	defer RegisterMetricsHook(nil)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			IncrementMetric("concurrent.event")
		}()
	}
	wg.Wait()

	if total.Load() != 100 {
		t.Fatalf("expected 100 total, got %d", total.Load())
	}
}
