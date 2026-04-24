package phtrace

import (
	"context"
	"testing"
)

// TestInit_DisabledReturnsNoopShutdown exercises the cfg.Enabled==false branch
// of Init (no OTLP dial). Init uses sync.Once — keep this the only Init test
// in the package unless a test-only reset is added.
func TestInit_DisabledReturnsNoopShutdown(t *testing.T) {
	ctx := context.Background()
	sd, err := Init(ctx, Config{
		Enabled:     false,
		ServiceName: "coverage-test",
	})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if sd == nil {
		t.Fatal("expected non-nil Shutdown")
	}
	if IsEnabled() {
		t.Fatal("expected IsEnabled() false when disabled")
	}
	if err := sd(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}
