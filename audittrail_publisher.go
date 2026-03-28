package paycloudhelper

import (
	"sync"
	"sync/atomic"
	"time"
)

// AuditPublisher provides production-grade audit message publishing with bounded
// concurrency via a worker pool, backpressure via a buffered channel, and a
// circuit breaker to avoid wasting resources when RabbitMQ is down.
type AuditPublisher struct {
	client     *AmqpClient
	msgChan    chan auditMessage
	done       chan struct{}
	wg         sync.WaitGroup
	stopOnce   sync.Once
	messageTTL string // empty = no expiration

	// Worker pool config
	workerCount int
	bufferSize  int
	maxRetries  int
	pushTimeout time.Duration

	// Circuit breaker state (atomic for lock-free access)
	consecutiveFailures atomic.Int64
	circuitOpen         atomic.Int32 // 0=closed, 1=open
	maxConsecFailures   int
	cooldownDuration    time.Duration
}

// auditMessage holds a pre-built payload ready for the worker to marshal and push.
type auditMessage struct {
	payload MessagePayloadAudit
}

// AuditPublisherOption configures an AuditPublisher via functional options.
type AuditPublisherOption func(*AuditPublisher)

// WithWorkerCount sets the number of worker goroutines (default 10).
func WithWorkerCount(n int) AuditPublisherOption {
	return func(p *AuditPublisher) {
		if n > 0 {
			p.workerCount = n
		}
	}
}

// WithBufferSize sets the channel buffer size (default 1000).
func WithBufferSize(n int) AuditPublisherOption {
	return func(p *AuditPublisher) {
		if n > 0 {
			p.bufferSize = n
		}
	}
}

// WithMaxRetries sets the maximum push retry count per message (default 3).
func WithMaxRetries(n int) AuditPublisherOption {
	return func(p *AuditPublisher) {
		if n >= 0 {
			p.maxRetries = n
		}
	}
}

// WithPublishTimeout sets the total timeout for a single push call (default 15s).
func WithPublishTimeout(d time.Duration) AuditPublisherOption {
	return func(p *AuditPublisher) {
		if d > 0 {
			p.pushTimeout = d
		}
	}
}

// WithMessageTTL sets the message TTL for published audit messages.
// Empty string means no expiration (recommended for audit data).
func WithMessageTTL(ttl string) AuditPublisherOption {
	return func(p *AuditPublisher) {
		p.messageTTL = ttl
	}
}

// WithCircuitBreakerThreshold sets consecutive failures before circuit opens (default 10).
func WithCircuitBreakerThreshold(n int) AuditPublisherOption {
	return func(p *AuditPublisher) {
		if n > 0 {
			p.maxConsecFailures = n
		}
	}
}

// WithCircuitBreakerCooldown sets the cooldown duration when circuit is open (default 30s).
func WithCircuitBreakerCooldown(d time.Duration) AuditPublisherOption {
	return func(p *AuditPublisher) {
		if d > 0 {
			p.cooldownDuration = d
		}
	}
}

// NewAuditPublisher creates an AuditPublisher with the given AMQP client and options.
// Call Start() to launch worker goroutines.
func NewAuditPublisher(client *AmqpClient, opts ...AuditPublisherOption) *AuditPublisher {
	p := &AuditPublisher{
		client:            client,
		workerCount:       10,
		bufferSize:        1000,
		maxRetries:        3,
		pushTimeout:       15 * time.Second,
		messageTTL:        "", // no expiration for audit
		maxConsecFailures: 10,
		cooldownDuration:  30 * time.Second,
	}
	for _, opt := range opts {
		opt(p)
	}
	p.msgChan = make(chan auditMessage, p.bufferSize)
	p.done = make(chan struct{})
	return p
}

// Start launches the worker goroutines. Call Stop() to shut down gracefully.
func (p *AuditPublisher) Start() {
	for i := 0; i < p.workerCount; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
	LogI("[AuditPublisher.Start] started workers=%d buffer=%d ttl=%q", p.workerCount, p.bufferSize, p.messageTTL)
}

// Submit adds a message to the worker pool. Non-blocking: if the buffer is
// full or the circuit breaker is open, the message is dropped with a warning log.
func (p *AuditPublisher) Submit(payload MessagePayloadAudit) {
	// Circuit breaker check
	if p.circuitOpen.Load() == 1 {
		logAuditNotReadyRateLimited("circuit breaker open — dropping audit message")
		return
	}

	select {
	case p.msgChan <- auditMessage{payload: payload}:
	default:
		LogW("[AuditPublisher.Submit] buffer full (%d), dropping audit message id=%d", p.bufferSize, payload.Id)
	}
}

// Stop gracefully shuts down the publisher: closes the channel, waits for
// workers to drain remaining messages, and returns.
func (p *AuditPublisher) Stop() {
	p.stopOnce.Do(func() {
		close(p.done)
		close(p.msgChan)
		p.wg.Wait()
		LogI("[AuditPublisher.Stop] all workers stopped")
	})
}

// worker is a long-running goroutine that reads messages from msgChan
// and pushes them to RabbitMQ sequentially (no concurrent channel access).
func (p *AuditPublisher) worker(id int) {
	defer p.wg.Done()
	for msg := range p.msgChan {
		p.processMessage(msg)
	}
}

// processMessage marshals and pushes a single audit message.
func (p *AuditPublisher) processMessage(msg auditMessage) {
	if p.client == nil || !p.client.IsReady() {
		p.recordFailure()
		return
	}

	msgBytes, err := jsonMarshalNoEsc(msg.payload)
	if err != nil {
		LogE("[AuditPublisher.processMessage] marshal failed err=%v", err)
		return
	}

	err = p.client.PushWithTTL(msgBytes, p.messageTTL)
	if err != nil {
		LogE("[AuditPublisher.processMessage] push failed id=%d err=%v", msg.payload.Id, err)
		p.recordFailure()
		return
	}

	// Success: reset failure counter
	p.consecutiveFailures.Store(0)
}

// recordFailure increments the failure counter and trips the circuit breaker
// if the threshold is reached.
func (p *AuditPublisher) recordFailure() {
	failures := p.consecutiveFailures.Add(1)
	if int(failures) >= p.maxConsecFailures && p.circuitOpen.CompareAndSwap(0, 1) {
		LogW("[AuditPublisher] circuit breaker OPEN after %d consecutive failures, cooldown=%v", failures, p.cooldownDuration)
		go p.cooldownReset()
	}
}

// cooldownReset resets the circuit breaker after the cooldown period.
func (p *AuditPublisher) cooldownReset() {
	select {
	case <-time.After(p.cooldownDuration):
	case <-p.done:
		return
	}
	p.consecutiveFailures.Store(0)
	p.circuitOpen.Store(0)
	LogI("[AuditPublisher] circuit breaker CLOSED — resuming audit publishing")
}
