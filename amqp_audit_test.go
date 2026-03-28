package paycloudhelper

import (
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Phase 0 — AmqpClient Push / IsReady tests
// ---------------------------------------------------------------------------

// TestAmqpClient_Push_NotReady verifies Push returns an error immediately
// when isReady is false, without blocking.
func TestAmqpClient_Push_NotReady(t *testing.T) {
	client := &AmqpClient{
		m:       &sync.Mutex{},
		isReady: false,
		done:    make(chan bool),
	}

	err := client.Push([]byte(`{"test":"data"}`))
	if err == nil {
		t.Fatal("expected error from Push when not ready, got nil")
	}
}

// ---------------------------------------------------------------------------
// Phase 1A — Push retry and IsReady tests
// ---------------------------------------------------------------------------

// TestPush_ReturnsErrorAfterMaxRetries verifies Push does not retry infinitely.
func TestPush_ReturnsErrorAfterMaxRetries(t *testing.T) {
	client := &AmqpClient{
		m:       &sync.Mutex{},
		isReady: false,
		done:    make(chan bool),
	}

	start := time.Now()
	err := client.Push([]byte(`{"test":"timeout"}`))
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error from Push, got nil")
	}

	// Push should return fast since isReady=false (immediate error, no retries).
	if elapsed > 2*time.Second {
		t.Errorf("Push took %v, expected fast return when not ready", elapsed)
	}
}

// TestIsReady_ReturnsCorrectState verifies IsReady reflects the internal state.
func TestIsReady_ReturnsCorrectState(t *testing.T) {
	client := &AmqpClient{
		m:       &sync.Mutex{},
		isReady: false,
	}

	if client.IsReady() {
		t.Error("expected IsReady()=false for new client")
	}

	client.m.Lock()
	client.isReady = true
	client.m.Unlock()

	if !client.IsReady() {
		t.Error("expected IsReady()=true after setting isReady")
	}
}

// TestIsReady_ThreadSafe verifies IsReady is safe for concurrent access.
func TestIsReady_ThreadSafe(t *testing.T) {
	client := &AmqpClient{
		m:       &sync.Mutex{},
		isReady: false,
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = client.IsReady()
		}()
	}
	wg.Wait()
}

// ---------------------------------------------------------------------------
// Phase 1B — WaitForReady and PushWithTTL tests
// ---------------------------------------------------------------------------

// TestWaitForReady_ReturnsFalse verifies WaitForReady returns false on timeout.
func TestWaitForReady_ReturnsFalse(t *testing.T) {
	client := &AmqpClient{
		m:       &sync.Mutex{},
		isReady: false,
	}

	start := time.Now()
	result := client.WaitForReady(200 * time.Millisecond)
	elapsed := time.Since(start)

	if result {
		t.Error("expected WaitForReady to return false on timeout")
	}
	if elapsed < 150*time.Millisecond {
		t.Errorf("WaitForReady returned too fast: %v", elapsed)
	}
}

// TestWaitForReady_ReturnsTrue verifies WaitForReady returns true when ready.
func TestWaitForReady_ReturnsTrue(t *testing.T) {
	client := &AmqpClient{
		m:       &sync.Mutex{},
		isReady: false,
	}

	// Set ready after a short delay.
	go func() {
		time.Sleep(50 * time.Millisecond)
		client.m.Lock()
		client.isReady = true
		client.m.Unlock()
	}()

	result := client.WaitForReady(1 * time.Second)
	if !result {
		t.Error("expected WaitForReady to return true")
	}
}

// TestPushWithTTL_NotReady verifies PushWithTTL returns error when not ready.
func TestPushWithTTL_NotReady(t *testing.T) {
	client := &AmqpClient{
		m:       &sync.Mutex{},
		isReady: false,
	}

	err := client.PushWithTTL([]byte(`{}`), "60000")
	if err == nil {
		t.Fatal("expected error from PushWithTTL when not ready")
	}
}
