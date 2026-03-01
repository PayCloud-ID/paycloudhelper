package phlogger

import (
	"sync"
	"testing"
	"time"
)

func TestKeyedLimiter_AllowsFirstEvent(t *testing.T) {
	kl := NewKeyedLimiter(10, 1) // 10/sec, burst 1
	if !kl.Allow("key") {
		t.Fatal("first event should always be allowed")
	}
}

func TestKeyedLimiter_IndependentKeys(t *testing.T) {
	kl := NewKeyedLimiter(10, 1)
	if !kl.Allow("a") {
		t.Fatal("first event for 'a' should be allowed")
	}
	if !kl.Allow("b") {
		t.Fatal("first event for 'b' should be allowed (independent key)")
	}
}

func TestKeyedLimiter_RateLimitsPerKey(t *testing.T) {
	// 1 event/sec, burst 1 — second immediate call should be denied.
	kl := NewKeyedLimiter(1, 1)
	if !kl.Allow("key") {
		t.Fatal("first event should be allowed")
	}
	if kl.Allow("key") {
		t.Fatal("immediate second event should be rate limited (1/sec)")
	}
}

func TestKeyedLimiter_BurstAllowsMultiple(t *testing.T) {
	// 1 event/sec but burst of 3 — first 3 should be allowed.
	kl := NewKeyedLimiter(1, 3)
	for i := 0; i < 3; i++ {
		if !kl.Allow("key") {
			t.Fatalf("event %d within burst should be allowed", i+1)
		}
	}
	if kl.Allow("key") {
		t.Fatal("event beyond burst should be rate limited")
	}
}

func TestKeyedLimiter_RefillsOverTime(t *testing.T) {
	kl := NewKeyedLimiter(100, 1) // 100/sec = ~1 per 10ms
	kl.Allow("key")               // exhaust burst

	// Wait enough for 1 token refill
	time.Sleep(20 * time.Millisecond)

	if !kl.Allow("key") {
		t.Fatal("after waiting for refill, event should be allowed")
	}
}

func TestKeyedLimiter_ConcurrentSafe(t *testing.T) {
	kl := NewKeyedLimiter(1000, 100) // generous rate for concurrency test

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// Each goroutine uses a unique key to avoid contention on the same limiter.
			kl.Allow("concurrent-key")
		}(i)
	}
	wg.Wait()
	// No panic or race = pass
}
