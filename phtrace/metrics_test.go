package phtrace

import (
	"context"
	"testing"
	"time"
)

func TestNewPhaseHistogram_Cached(t *testing.T) {
	h1, err := NewPhaseHistogram("test-meter", "test_hist", nil)
	if err != nil {
		t.Fatalf("NewPhaseHistogram: %v", err)
	}
	h2, err := NewPhaseHistogram("test-meter", "test_hist", nil)
	if err != nil {
		t.Fatalf("second NewPhaseHistogram: %v", err)
	}
	if h1 != h2 {
		t.Errorf("expected cached instance, got different pointers")
	}
}

func TestPhaseHistogram_Record_NoPanicWhenDisabled(t *testing.T) {
	h, _ := NewPhaseHistogram("noop-meter", "noop_hist", nil)
	// Should not panic even though phtrace is disabled in tests.
	h.Record(context.Background(), "rmq_publish", 12*time.Millisecond)
}

func TestPhaseHistogram_Observe(t *testing.T) {
	h, _ := NewPhaseHistogram("observe-meter", "observe_hist", nil)
	done := h.Observe(context.Background(), "db_write")
	time.Sleep(1 * time.Millisecond)
	done() // should not panic
}

func TestPhaseHistogram_NilReceiverSafe(t *testing.T) {
	var h *PhaseHistogram
	h.Record(context.Background(), "phase", time.Millisecond)
}

func TestDefaultPhaseBuckets_Monotonic(t *testing.T) {
	for i := 1; i < len(DefaultPhaseBuckets); i++ {
		if DefaultPhaseBuckets[i] <= DefaultPhaseBuckets[i-1] {
			t.Errorf("bucket %d (%f) <= previous (%f)", i, DefaultPhaseBuckets[i], DefaultPhaseBuckets[i-1])
		}
	}
}

func TestMustPhaseHistogram(t *testing.T) {
	name := "must_hist_" + t.Name()
	h := MustPhaseHistogram("must_meter_"+t.Name(), name, []float64{1, 2, 3})
	if h == nil {
		t.Fatal("MustPhaseHistogram returned nil")
	}
	h.Record(context.Background(), "phase_x", 2*time.Millisecond)
}
