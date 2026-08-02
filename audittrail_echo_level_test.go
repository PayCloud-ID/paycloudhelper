package paycloudhelper

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/PayCloud-ID/paycloudhelper/phlogger"
	amqp "github.com/rabbitmq/amqp091-go"
)

// captureMarked counts, per level, the log lines emitted while fn runs whose
// message contains marker. Filtering by marker keeps the assertion immune to
// unrelated lines from async audit goroutines still finishing from other tests.
//
// The log level is deliberately NOT changed: phlogger fires hooks after every
// emit regardless of the configured level, and mutating the global logger races
// with those goroutines.
func captureMarked(t *testing.T, marker string, fn func()) map[string]int {
	t.Helper()
	phlogger.ClearLogHooks()
	t.Cleanup(phlogger.ClearLogHooks)

	var mu sync.Mutex
	seen := map[string]int{}
	for _, level := range []string{"debug", "info", "warn", "error"} {
		phlogger.RegisterLogHook(level, func(level, message string) {
			if !strings.Contains(message, marker) {
				return
			}
			mu.Lock()
			defer mu.Unlock()
			seen[level]++
		})
	}

	fn()

	mu.Lock()
	defer mu.Unlock()
	out := make(map[string]int, len(seen))
	for k, v := range seen {
		out[k] = v
	}
	return out
}

func auditTestData() *RequestAndResponse {
	return &RequestAndResponse{
		Response: ResponseAudit{Detail: Detail{StatusCode: 200, Message: "ok"}},
	}
}

// TestLogAudittrailDataEchoesAtDebug pins the level of the per-event audit echo.
// It carries no transaction identifier, so at info it cost one line per request
// in every consumer while being unable to confirm any specific record was
// audited. The record itself still reaches the queue regardless of log level.
func TestLogAudittrailDataEchoesAtDebug(t *testing.T) {
	orig := auditTrailMqClient.Load()
	auditTrailMqClient.Store(nil)
	t.Cleanup(func() { auditTrailMqClient.Store(orig) })

	seen := captureMarked(t, "LogAudittrailData]", func() {
		LogAudittrailData("TestFunc", "desc", "internal", "grpc", nil, auditTestData())
	})

	if seen["debug"] == 0 {
		t.Error("LogAudittrailData emitted no debug line; the echo must stay available at debug")
	}
	if seen["info"] != 0 {
		t.Errorf("LogAudittrailData emitted %d info line(s), want 0 — the echo is debug-only", seen["info"])
	}
}

// TestLogAudittrailDataV2EchoesAtDebug holds the V2 path to the same contract so
// a service on V2 does not keep the noise V1 shed.
func TestLogAudittrailDataV2EchoesAtDebug(t *testing.T) {
	seen := captureMarked(t, "LogAudittrailDataV2]", func() {
		LogAudittrailDataV2("TestFunc", "desc", "internal", "grpc", nil, auditTestData())
	})

	if seen["info"] != 0 {
		t.Errorf("LogAudittrailDataV2 emitted %d info line(s), want 0", seen["info"])
	}
}

// TestPushMessageAuditSuccessIsRateLimited pins the successful-publish line to a
// heartbeat: many publishes inside one window must not produce many info lines,
// and rate limiting must never suppress an actual publish.
func TestPushMessageAuditSuccessIsRateLimited(t *testing.T) {
	const messages = 20

	orig := auditTrailMqClient.Load()
	t.Cleanup(func() { auditTrailMqClient.Store(orig) })

	var pushed int
	var mu sync.Mutex
	notify := make(chan amqp.Confirmation, messages+2)
	client := &AmqpClient{
		m:             &sync.Mutex{},
		isReady:       true,
		queueName:     "audit-q",
		connName:      "trail-conn",
		done:          make(chan bool),
		notifyConfirm: notify,
		publishForTest: func(context.Context, string, string, bool, bool, amqp.Publishing) error {
			mu.Lock()
			defer mu.Unlock()
			pushed++
			return nil
		},
	}
	auditTrailMqClient.Store(client)
	for i := 0; i < messages; i++ {
		notify <- amqp.Confirmation{Ack: true}
	}

	// Generous timeout: confirmations are pre-buffered, so nothing should wait.
	// A short deadline makes Push's select race the ready confirmation when the
	// suite runs packages in parallel, which has nothing to do with what this
	// test asserts.
	prevTO := PushTimeout
	prevResend := amqpResendDelayForTestNs.Load()
	PushTimeout = 30 * time.Second
	amqpResendDelayForTestNs.Store(uint64(time.Millisecond))
	t.Cleanup(func() {
		PushTimeout = prevTO
		amqpResendDelayForTestNs.Store(prevResend)
	})

	seen := captureMarked(t, "publish message async success", func() {
		for i := 0; i < messages; i++ {
			pushMessageAudit(MessagePayloadAudit{Id: i, Command: CmdAuditTrailData})
		}
	})

	mu.Lock()
	got := pushed
	mu.Unlock()
	if got != messages {
		t.Errorf("published %d messages, want %d — rate limiting must not drop publishes", got, messages)
	}
	// At most one: zero when an earlier test already consumed this window.
	if seen["info"] > 1 {
		t.Errorf("got %d info lines for %d publishes in one window, want at most 1", seen["info"], messages)
	}
}

// TestAuditPublishHeartbeatWindow documents the cadence so changing it is a
// deliberate edit rather than a silent one.
func TestAuditPublishHeartbeatWindow(t *testing.T) {
	if auditPublishOkLogWindow != time.Minute {
		t.Errorf("auditPublishOkLogWindow = %v, want 1m", auditPublishOkLogWindow)
	}
}
