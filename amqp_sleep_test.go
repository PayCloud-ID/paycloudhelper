package paycloudhelper

import (
	"testing"
	"time"
)

func TestAmqpReinitSleep_UsesDefaultWhenNoOverride(t *testing.T) {
	prev := amqpReinitDelayForTest
	defer func() { amqpReinitDelayForTest = prev }()

	amqpReinitDelayForTest = 0
	if got, want := amqpReinitSleep(), reInitDelay; got != want {
		t.Fatalf("amqpReinitSleep()=%v want %v", got, want)
	}
}

func TestAmqpReinitSleep_UsesOverrideWhenSet(t *testing.T) {
	prev := amqpReinitDelayForTest
	defer func() { amqpReinitDelayForTest = prev }()

	amqpReinitDelayForTest = 12 * time.Millisecond
	if got := amqpReinitSleep(); got != 12*time.Millisecond {
		t.Fatalf("amqpReinitSleep()=%v want %v", got, 12*time.Millisecond)
	}
}

