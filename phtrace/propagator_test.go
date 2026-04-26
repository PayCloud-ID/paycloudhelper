package phtrace

import (
	"testing"

	"go.opentelemetry.io/otel/propagation"
)

func TestPropagator_nonNil(t *testing.T) {
	p := Propagator()
	if p == nil {
		t.Fatal("Propagator() returned nil")
	}
	var _ propagation.TextMapPropagator = p
}

func TestResource_beforeInitNil(t *testing.T) {
	if Resource() != nil {
		t.Fatal("Resource() should be nil before Init")
	}
}
