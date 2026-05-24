package paycloudhelper

import (
	"testing"
	"time"
)

func TestAmqpReinitSleep_UsesDefaultWhenNoOverride(t *testing.T) {
	prev := amqpReinitDelayForTestNs.Load()
	defer func() { amqpReinitDelayForTestNs.Store(prev) }()

	amqpReinitDelayForTestNs.Store(0)
	if got, want := amqpReinitSleep(), reInitDelay; got != want {
		t.Fatalf("amqpReinitSleep()=%v want %v", got, want)
	}
}

func TestAmqpReinitSleep_UsesOverrideWhenSet(t *testing.T) {
	prev := amqpReinitDelayForTestNs.Load()
	defer func() { amqpReinitDelayForTestNs.Store(prev) }()

	amqpReinitDelayForTestNs.Store(uint64((12 * time.Millisecond).Nanoseconds()))
	if got := amqpReinitSleep(); got != 12*time.Millisecond {
		t.Fatalf("amqpReinitSleep()=%v want %v", got, 12*time.Millisecond)
	}
}
