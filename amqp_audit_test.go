package paycloudhelper

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
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

// ---------------------------------------------------------------------------
// Dial / hooks / Close / Consume / Push confirmation paths
// ---------------------------------------------------------------------------

func TestAmqpClient_connect_DialHookError(t *testing.T) {
	prev := amqpDialHook
	t.Cleanup(func() { amqpDialHook = prev })
	amqpDialHook = func(addr string, cfg amqp.Config) (*amqp.Connection, error) {
		_ = addr
		_ = cfg
		return nil, errors.New("dial refused")
	}
	c := defaultAmqpClient()
	_, err := c.connect("amqp://127.0.0.1:5672/")
	if err == nil {
		t.Fatal("expected dial error")
	}
}

func TestAmqpClient_Close_NotReady(t *testing.T) {
	c := &AmqpClient{m: &sync.Mutex{}, isReady: false}
	if err := c.Close(); err != errAlreadyClosed {
		t.Fatalf("Close() err=%v want errAlreadyClosed", err)
	}
}

func TestAmqpClient_Close_TestHooks(t *testing.T) {
	done := make(chan bool)
	c := &AmqpClient{
		m:                   &sync.Mutex{},
		isReady:             true,
		done:                done,
		channelCloseForTest: func() error { return nil },
		connCloseForTest:    func() error { return nil },
	}
	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if c.isReady {
		t.Fatal("expected isReady false after Close")
	}
}

func TestAmqpClient_Close_ChannelHookError(t *testing.T) {
	done := make(chan bool)
	c := &AmqpClient{
		m:                   &sync.Mutex{},
		isReady:             true,
		done:                done,
		channelCloseForTest: func() error { return fmt.Errorf("close ch") },
	}
	err := c.Close()
	if err == nil || err.Error() != "close ch" {
		t.Fatalf("Close err=%v", err)
	}
}

func TestAmqpClient_Consume_WithTestHook(t *testing.T) {
	ch := make(chan amqp.Delivery)
	close(ch)
	c := &AmqpClient{
		m:       &sync.Mutex{},
		isReady: true,
		consumeForTest: func() (<-chan amqp.Delivery, error) {
			return ch, nil
		},
	}
	out, err := c.Consume()
	if err != nil {
		t.Fatalf("Consume: %v", err)
	}
	if out == nil {
		t.Fatal("nil delivery channel")
	}
}

func TestAmqpClient_Push_PublishHookWithAck(t *testing.T) {
	notify := make(chan amqp.Confirmation, 2)
	done := make(chan bool)
	c := &AmqpClient{
		m:             &sync.Mutex{},
		isReady:       true,
		done:          done,
		notifyConfirm: notify,
		publishForTest: func(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error {
			_ = ctx
			_ = exchange
			_ = key
			_ = mandatory
			_ = immediate
			_ = msg
			return nil
		},
	}
	prevRetries, prevTimeout := PushMaxRetries, PushTimeout
	PushMaxRetries = 3
	PushTimeout = 3 * time.Second
	t.Cleanup(func() {
		PushMaxRetries = prevRetries
		PushTimeout = prevTimeout
	})

	go func() { notify <- amqp.Confirmation{Ack: true} }()

	if err := c.Push([]byte(`{}`)); err != nil {
		t.Fatalf("Push: %v", err)
	}
}

func TestAmqpClient_Push_PublishHookNackThenAck(t *testing.T) {
	notify := make(chan amqp.Confirmation, 4)
	done := make(chan bool)
	c := &AmqpClient{
		m:             &sync.Mutex{},
		isReady:       true,
		done:          done,
		notifyConfirm: notify,
		publishForTest: func(context.Context, string, string, bool, bool, amqp.Publishing) error {
			return nil
		},
	}
	prevRetries, prevTimeout := PushMaxRetries, PushTimeout
	PushMaxRetries = 3
	PushTimeout = 5 * time.Second
	t.Cleanup(func() {
		PushMaxRetries = prevRetries
		PushTimeout = prevTimeout
	})

	go func() {
		notify <- amqp.Confirmation{Ack: false}
		notify <- amqp.Confirmation{Ack: true}
	}()

	if err := c.Push([]byte(`{}`)); err != nil {
		t.Fatalf("Push: %v", err)
	}
}

func TestAmqpClient_PushWithTTL_PublishHook(t *testing.T) {
	c := &AmqpClient{
		m:       &sync.Mutex{},
		isReady: true,
		done:    make(chan bool),
		publishForTest: func(context.Context, string, string, bool, bool, amqp.Publishing) error {
			return nil
		},
	}
	if err := c.PushWithTTL([]byte(`{}`), ""); err != nil {
		t.Fatalf("PushWithTTL: %v", err)
	}
}

func TestAmqpClient_Accessors(t *testing.T) {
	old := GetAppName()
	t.Cleanup(func() { SetAppName(old) })
	SetAppName("acc-test")

	c := defaultAmqpClient()
	if got := c.ConnName(); got != "amqp-acc-test" {
		t.Errorf("ConnName() = %q", got)
	}

	cfg := defaultAmqpConfig()
	c.SetAmqpConfig(cfg)
	if c.AmqpConfig().TLSClientConfig == nil {
		t.Error("expected non-nil TLS from defaultAmqpConfig")
	}
	if c.Channel() != nil {
		t.Log("channel non-nil before connect (ok)")
	}
	if c.InfoLog() == nil || c.ErrLog() == nil {
		t.Fatal("expected non-nil loggers")
	}

	c2 := defaultAmqpClient()
	c2.connName = "fixed-name"
	if c2.ConnName() != "fixed-name" {
		t.Errorf("ConnName = %q", c2.ConnName())
	}
}

func TestNewAmqp_nilClientNoop(t *testing.T) {
	NewAmqp("amqp://127.0.0.1:5672/", nil)
}

func TestAmqpClient_Push_ConfirmationTimeout(t *testing.T) {
	notify := make(chan amqp.Confirmation)
	done := make(chan bool)
	c := &AmqpClient{
		m:             &sync.Mutex{},
		isReady:       true,
		done:          done,
		notifyConfirm: notify,
		publishForTest: func(context.Context, string, string, bool, bool, amqp.Publishing) error {
			return nil
		},
	}
	prev := PushTimeout
	PushTimeout = 80 * time.Millisecond
	t.Cleanup(func() { PushTimeout = prev })

	err := c.Push([]byte(`{}`))
	if err == nil || !strings.Contains(err.Error(), "confirmation timeout") {
		t.Fatalf("Push err=%v want confirmation timeout", err)
	}
}

func TestAmqpClient_Push_ErrShutdownWaitingConfirm(t *testing.T) {
	notify := make(chan amqp.Confirmation)
	done := make(chan bool)
	c := &AmqpClient{
		m:             &sync.Mutex{},
		isReady:       true,
		done:          done,
		notifyConfirm: notify,
		publishForTest: func(context.Context, string, string, bool, bool, amqp.Publishing) error {
			return nil
		},
	}
	prev := PushTimeout
	PushTimeout = 5 * time.Second
	t.Cleanup(func() { PushTimeout = prev })

	go func() {
		time.Sleep(30 * time.Millisecond)
		close(done)
	}()

	err := c.Push([]byte(`{}`))
	if !errors.Is(err, errShutdown) {
		t.Fatalf("Push err=%v want errShutdown", err)
	}
}

func TestAmqpClient_Push_PublishErrorUsesDeadline(t *testing.T) {
	done := make(chan bool)
	c := &AmqpClient{
		m:       &sync.Mutex{},
		isReady: true,
		done:    done,
		publishForTest: func(context.Context, string, string, bool, bool, amqp.Publishing) error {
			return errors.New("publish failed")
		},
	}
	prevT, prevR := PushTimeout, PushMaxRetries
	PushTimeout = 100 * time.Millisecond
	PushMaxRetries = 2
	t.Cleanup(func() {
		PushTimeout = prevT
		PushMaxRetries = prevR
	})

	err := c.Push([]byte(`{}`))
	if err == nil || !strings.Contains(err.Error(), "push timeout") {
		t.Fatalf("Push err=%v want timeout wrapping publish error", err)
	}
}

func TestAmqpClient_Push_AllNacksExhaustsRetries(t *testing.T) {
	notify := make(chan amqp.Confirmation, 8)
	done := make(chan bool)
	c := &AmqpClient{
		m:             &sync.Mutex{},
		isReady:       true,
		done:          done,
		notifyConfirm: notify,
		publishForTest: func(context.Context, string, string, bool, bool, amqp.Publishing) error {
			return nil
		},
	}
	prevT, prevR := PushTimeout, PushMaxRetries
	PushTimeout = 3 * time.Second
	PushMaxRetries = 3
	t.Cleanup(func() {
		PushTimeout = prevT
		PushMaxRetries = prevR
	})

	go func() {
		for i := 0; i < 4; i++ {
			notify <- amqp.Confirmation{Ack: false}
		}
	}()

	err := c.Push([]byte(`{}`))
	if err == nil || !strings.Contains(err.Error(), "failed after") {
		t.Fatalf("Push err=%v want exhausted retries", err)
	}
}

func TestAmqpClient_Consume_QosForTestError(t *testing.T) {
	c := &AmqpClient{
		m:       &sync.Mutex{},
		isReady: true,
		qosForTest: func(prefetchCount, prefetchSize int, global bool) error {
			_ = prefetchCount
			_ = prefetchSize
			_ = global
			return errors.New("qos failed")
		},
	}
	_, err := c.Consume()
	if err == nil || err.Error() != "qos failed" {
		t.Fatalf("Consume err=%v", err)
	}
}

func TestAmqpClient_checkIfQueueExists_nilChannel(t *testing.T) {
	c := &AmqpClient{m: &sync.Mutex{}, queueName: "q"}
	_, err := c.checkIfQueueExists(nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestAmqpClient_Cc_usesTestCloseHooks(t *testing.T) {
	var chCalls, connCalls int
	c := &AmqpClient{
		m: &sync.Mutex{},
		ccChannelCloseForTest: func() error {
			chCalls++
			return nil
		},
		ccConnCloseForTest: func() error {
			connCalls++
			return nil
		},
	}
	if err := c.Cc(); err != nil {
		t.Fatalf("Cc: %v", err)
	}
	if chCalls != 1 || connCalls != 1 {
		t.Fatalf("close hooks chCalls=%d connCalls=%d", chCalls, connCalls)
	}
}

func TestAmqpClient_checkIfQueueExists_passiveHook(t *testing.T) {
	c := &AmqpClient{
		m:         &sync.Mutex{},
		queueName: "q",
		queuePassiveForTest: func(ch *amqp.Channel) (bool, error) {
			_ = ch
			return true, nil
		},
	}
	ok, err := c.checkIfQueueExists(&amqp.Channel{})
	if err != nil || !ok {
		t.Fatalf("got ok=%v err=%v", ok, err)
	}

	c2 := &AmqpClient{
		m:         &sync.Mutex{},
		queueName: "missing",
		queuePassiveForTest: func(ch *amqp.Channel) (bool, error) {
			_ = ch
			return false, errors.New("not found")
		},
	}
	ok2, err2 := c2.checkIfQueueExists(&amqp.Channel{})
	if ok2 || err2 == nil {
		t.Fatalf("expected missing queue, ok=%v err=%v", ok2, err2)
	}
}

func TestAmqpClient_Close_ConnHookError(t *testing.T) {
	done := make(chan bool)
	c := &AmqpClient{
		m:                   &sync.Mutex{},
		isReady:             true,
		done:                done,
		channelCloseForTest: func() error { return nil },
		connCloseForTest:    func() error { return errors.New("conn close") },
	}
	err := c.Close()
	if err == nil || err.Error() != "conn close" {
		t.Fatalf("Close err=%v", err)
	}
}
