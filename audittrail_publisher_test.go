package paycloudhelper

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// ---------------------------------------------------------------------------
// Phase 1B — AuditPublisher worker pool tests
// ---------------------------------------------------------------------------

// newTestPublisher creates an AuditPublisher with a nil client for unit testing.
// Workers will hit the nil-client early exit in processMessage.
func newTestPublisher(opts ...AuditPublisherOption) *AuditPublisher {
	return NewAuditPublisher(nil, opts...)
}

// TestAuditPublisher_NewDefaults verifies default configuration values.
func TestAuditPublisher_NewDefaults(t *testing.T) {
	p := newTestPublisher()
	defer p.Stop()

	if p.workerCount != 10 {
		t.Errorf("workerCount = %d, want 10", p.workerCount)
	}
	if p.bufferSize != 1000 {
		t.Errorf("bufferSize = %d, want 1000", p.bufferSize)
	}
	if p.maxRetries != 3 {
		t.Errorf("maxRetries = %d, want 3", p.maxRetries)
	}
	if p.pushTimeout != 15*time.Second {
		t.Errorf("pushTimeout = %v, want 15s", p.pushTimeout)
	}
	if p.messageTTL != "" {
		t.Errorf("messageTTL = %q, want empty", p.messageTTL)
	}
	if p.maxConsecFailures != 10 {
		t.Errorf("maxConsecFailures = %d, want 10", p.maxConsecFailures)
	}
	if p.cooldownDuration != 30*time.Second {
		t.Errorf("cooldownDuration = %v, want 30s", p.cooldownDuration)
	}
}

// TestAuditPublisher_Options verifies functional options override defaults.
func TestAuditPublisher_Options(t *testing.T) {
	p := NewAuditPublisher(nil,
		WithWorkerCount(5),
		WithBufferSize(500),
		WithMaxRetries(2),
		WithPublishTimeout(10*time.Second),
		WithMessageTTL("30000"),
		WithCircuitBreakerThreshold(20),
		WithCircuitBreakerCooldown(60*time.Second),
	)
	defer p.Stop()

	if p.workerCount != 5 {
		t.Errorf("workerCount = %d, want 5", p.workerCount)
	}
	if p.bufferSize != 500 {
		t.Errorf("bufferSize = %d, want 500", p.bufferSize)
	}
	if p.maxRetries != 2 {
		t.Errorf("maxRetries = %d, want 2", p.maxRetries)
	}
	if p.pushTimeout != 10*time.Second {
		t.Errorf("pushTimeout = %v, want 10s", p.pushTimeout)
	}
	if p.messageTTL != "30000" {
		t.Errorf("messageTTL = %q, want 30000", p.messageTTL)
	}
	if p.maxConsecFailures != 20 {
		t.Errorf("maxConsecFailures = %d, want 20", p.maxConsecFailures)
	}
	if p.cooldownDuration != 60*time.Second {
		t.Errorf("cooldownDuration = %v, want 60s", p.cooldownDuration)
	}
}

// TestAuditPublisher_OptionsIgnoreInvalid verifies invalid options are ignored.
func TestAuditPublisher_OptionsIgnoreInvalid(t *testing.T) {
	p := NewAuditPublisher(nil,
		WithWorkerCount(0),
		WithWorkerCount(-1),
		WithBufferSize(0),
		WithMaxRetries(-1),
		WithPublishTimeout(0),
		WithCircuitBreakerThreshold(0),
		WithCircuitBreakerCooldown(0),
	)
	defer p.Stop()

	// All should retain defaults since invalid values should be ignored.
	if p.workerCount != 10 {
		t.Errorf("workerCount = %d, want default 10", p.workerCount)
	}
	if p.bufferSize != 1000 {
		t.Errorf("bufferSize = %d, want default 1000", p.bufferSize)
	}
	// maxRetries allows 0 (no retries), so only -1 is invalid
	if p.maxRetries != 3 {
		t.Errorf("maxRetries = %d, want default 3", p.maxRetries)
	}
}

// TestAuditPublisher_WorkerPool_ProcessesMessages verifies workers consume messages.
func TestAuditPublisher_WorkerPool_ProcessesMessages(t *testing.T) {
	// Use nil client — workers will hit early exit in processMessage
	// but the message should still be consumed from the channel.
	p := NewAuditPublisher(nil, WithWorkerCount(2), WithBufferSize(10))
	p.Start()

	for i := 0; i < 5; i++ {
		p.Submit(MessagePayloadAudit{
			Id:      nextAuditID(),
			Command: CmdAuditTrailData,
			Time:    time.Now().Format(time.DateTime),
		})
	}

	// Give workers time to process.
	time.Sleep(100 * time.Millisecond)
	p.Stop()

	// Channel should be drained after stop.
	if len(p.msgChan) != 0 {
		t.Errorf("msgChan still has %d messages after Stop", len(p.msgChan))
	}
}

// TestAuditPublisher_Backpressure_FullBuffer verifies Submit drops messages
// when the buffer is full.
func TestAuditPublisher_Backpressure_FullBuffer(t *testing.T) {
	// Create publisher with tiny buffer, do NOT start workers.
	p := NewAuditPublisher(nil, WithBufferSize(2), WithWorkerCount(1))
	// Don't start — channel won't be drained.

	// Fill the buffer.
	p.Submit(MessagePayloadAudit{Id: 1, Command: CmdAuditTrailData})
	p.Submit(MessagePayloadAudit{Id: 2, Command: CmdAuditTrailData})

	// Third submit should be dropped (buffer full).
	p.Submit(MessagePayloadAudit{Id: 3, Command: CmdAuditTrailData})

	if len(p.msgChan) != 2 {
		t.Errorf("msgChan length = %d, want 2 (third should be dropped)", len(p.msgChan))
	}

	p.Stop()
}

// TestAuditPublisher_CircuitBreaker_TripsAfterFailures verifies the circuit breaker
// opens after consecutive failures reach the threshold.
func TestAuditPublisher_CircuitBreaker_TripsAfterFailures(t *testing.T) {
	p := NewAuditPublisher(nil,
		WithCircuitBreakerThreshold(3),
		WithCircuitBreakerCooldown(5*time.Second),
	)

	// Simulate 3 consecutive failures.
	for i := 0; i < 3; i++ {
		p.recordFailure()
	}

	if p.circuitOpen.Load() != 1 {
		t.Error("expected circuit breaker to be OPEN after 3 failures")
	}

	// Submit should be dropped when circuit is open.
	p.Submit(MessagePayloadAudit{Id: 99, Command: CmdAuditTrailData})
	if len(p.msgChan) != 0 {
		t.Error("expected message to be dropped when circuit is open")
	}

	p.Stop()
}

// TestAuditPublisher_CircuitBreaker_ResetsAfterCooldown verifies the circuit
// breaker closes after cooldown period.
func TestAuditPublisher_CircuitBreaker_ResetsAfterCooldown(t *testing.T) {
	p := NewAuditPublisher(nil,
		WithCircuitBreakerThreshold(2),
		WithCircuitBreakerCooldown(200*time.Millisecond),
	)

	// Trip the breaker.
	p.recordFailure()
	p.recordFailure()

	if p.circuitOpen.Load() != 1 {
		t.Fatal("expected circuit breaker to be OPEN")
	}

	// Wait for cooldown.
	time.Sleep(350 * time.Millisecond)

	if p.circuitOpen.Load() != 0 {
		t.Error("expected circuit breaker to be CLOSED after cooldown")
	}
	if p.consecutiveFailures.Load() != 0 {
		t.Error("expected consecutive failures to be reset after cooldown")
	}

	p.Stop()
}

// TestAuditPublisher_Stop_DrainsMessages verifies Stop() processes remaining
// messages in the channel.
func TestAuditPublisher_Stop_DrainsMessages(t *testing.T) {
	p := NewAuditPublisher(nil, WithWorkerCount(1), WithBufferSize(100))
	p.Start()

	// Submit several messages.
	for i := 0; i < 10; i++ {
		p.Submit(MessagePayloadAudit{
			Id:      nextAuditID(),
			Command: CmdAuditTrailData,
			Time:    time.Now().Format(time.DateTime),
		})
	}

	// Stop should wait for workers to drain the channel.
	p.Stop()

	if len(p.msgChan) != 0 {
		t.Errorf("msgChan has %d messages after Stop, expected 0", len(p.msgChan))
	}
}

// TestAuditPublisher_Stop_Idempotent verifies Stop() can be called multiple times.
func TestAuditPublisher_Stop_Idempotent(t *testing.T) {
	p := NewAuditPublisher(nil, WithWorkerCount(1), WithBufferSize(10))
	p.Start()

	p.Stop()
	p.Stop() // Should not panic.
}

// TestAuditPublisher_ProcessMessage_NilClient verifies processMessage handles nil client.
func TestAuditPublisher_ProcessMessage_NilClient(t *testing.T) {
	p := NewAuditPublisher(nil)

	// Should not panic with nil client.
	p.processMessage(auditMessage{
		payload: MessagePayloadAudit{Id: 1, Command: CmdAuditTrailData},
	})

	// Should have recorded a failure.
	if p.consecutiveFailures.Load() == 0 {
		t.Error("expected failure to be recorded for nil client")
	}
}

// TestAuditPublisher_ProcessMessage_ClientNotReady verifies processMessage
// handles not-ready client.
func TestAuditPublisher_ProcessMessage_ClientNotReady(t *testing.T) {
	client := &AmqpClient{
		m:       &sync.Mutex{},
		isReady: false,
	}
	p := NewAuditPublisher(client)

	p.processMessage(auditMessage{
		payload: MessagePayloadAudit{Id: 2, Command: CmdAuditTrailData},
	})

	if p.consecutiveFailures.Load() == 0 {
		t.Error("expected failure to be recorded for not-ready client")
	}
}

// TestAuditPublisher_ProcessMessage_PushError verifies a ready client whose
// PushWithTTL fails records a failure (processMessage push error branch).
func TestAuditPublisher_ProcessMessage_PushError(t *testing.T) {
	client := &AmqpClient{
		m:         &sync.Mutex{},
		isReady:   true,
		queueName: "audit",
	}
	client.publishForTest = func(context.Context, string, string, bool, bool, amqp.Publishing) error {
		return errors.New("publish failed")
	}
	p := NewAuditPublisher(client)
	before := p.consecutiveFailures.Load()
	p.processMessage(auditMessage{
		payload: MessagePayloadAudit{Id: 7, Command: CmdAuditTrailData},
	})
	if p.consecutiveFailures.Load() <= before {
		t.Fatalf("expected failure count to increase, before=%d after=%d", before, p.consecutiveFailures.Load())
	}
}

// TestAuditPublisher_ProcessMessage_SuccessClearsFailures verifies a successful
// push resets the consecutive failure counter.
func TestAuditPublisher_ProcessMessage_SuccessClearsFailures(t *testing.T) {
	client := &AmqpClient{
		m:         &sync.Mutex{},
		isReady:   true,
		queueName: "audit",
	}
	client.publishForTest = func(context.Context, string, string, bool, bool, amqp.Publishing) error {
		return nil
	}
	p := NewAuditPublisher(client)
	p.consecutiveFailures.Store(4)
	p.processMessage(auditMessage{
		payload: MessagePayloadAudit{Id: 8, Command: CmdAuditTrailData},
	})
	if p.consecutiveFailures.Load() != 0 {
		t.Fatalf("want consecutiveFailures=0 after success, got %d", p.consecutiveFailures.Load())
	}
}
