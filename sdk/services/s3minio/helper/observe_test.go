package helper

import (
	"testing"
	"time"
)

func TestObserveProbe_NoHook(t *testing.T) {
	t.Parallel()
	if err := ObserveProbe(time.Now(), "health", "grpc", 0); err != nil {
		t.Fatalf("ObserveProbe: %v", err)
	}
}

func TestObserveProbe_WithHook(t *testing.T) {
	var sawProbe, sawTransport string
	var sawCode uint32
	ProbeObserveFunc = func(_ time.Time, probeName, transport string, code uint32) {
		sawProbe = probeName
		sawTransport = transport
		sawCode = code
	}
	t.Cleanup(func() { ProbeObserveFunc = nil })

	if err := ObserveProbe(time.Now(), "ready", "grpc", 503); err != nil {
		t.Fatalf("ObserveProbe: %v", err)
	}
	if sawProbe != "ready" || sawTransport != "grpc" || sawCode != 503 {
		t.Fatalf("hook got probe=%q transport=%q code=%d", sawProbe, sawTransport, sawCode)
	}
}
