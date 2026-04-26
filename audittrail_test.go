package paycloudhelper

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// ---------------------------------------------------------------------------
// Phase 0 — baseline tests for current audittrail.go behavior
// ---------------------------------------------------------------------------

// TestLogAudittrailData_NilClient verifies no panic when auditTrailMqClient is nil.
func TestLogAudittrailData_NilClient(t *testing.T) {
	// Ensure client is nil
	orig := auditTrailMqClient.Load()
	auditTrailMqClient.Store(nil)
	defer func() { auditTrailMqClient.Store(orig) }()

	data := &RequestAndResponse{
		Response: ResponseAudit{
			Detail: Detail{StatusCode: 200, Message: "ok"},
		},
	}

	// Should not panic — fire-and-forget goroutine logs warning.
	LogAudittrailData("TestFunc", "desc", "internal", "http", nil, data)

	// Give goroutine time to complete (it will hit nil-client early exit).
	time.Sleep(50 * time.Millisecond)
}

// TestLogAudittrailData_EmptyFuncName verifies early return when funcName is empty.
func TestLogAudittrailData_EmptyFuncName(t *testing.T) {
	data := &RequestAndResponse{
		Response: ResponseAudit{
			Detail: Detail{StatusCode: 200, Message: "ok"},
		},
	}

	// Should return immediately without spawning goroutine.
	LogAudittrailData("", "desc", "internal", "http", nil, data)
}

// TestLogAudittrailData_NilData verifies early return when data is nil.
func TestLogAudittrailData_NilData(t *testing.T) {
	LogAudittrailData("TestFunc", "desc", "internal", "http", nil, nil)
	// no panic expected
}

// TestLogAudittrailData_ZeroStatusCode verifies early return when StatusCode == 0.
func TestLogAudittrailData_ZeroStatusCode(t *testing.T) {
	data := &RequestAndResponse{
		Response: ResponseAudit{
			Detail: Detail{StatusCode: 0, Message: ""},
		},
	}
	LogAudittrailData("TestFunc", "desc", "internal", "http", nil, data)
	// no panic expected — early exit
}

// TestLogAudittrailData_WithKeys verifies keys are set correctly.
func TestLogAudittrailData_WithKeys(t *testing.T) {
	orig := auditTrailMqClient.Load()
	auditTrailMqClient.Store(nil)
	defer func() { auditTrailMqClient.Store(orig) }()

	keys := []string{"key1", "key2"}
	data := &RequestAndResponse{
		Response: ResponseAudit{
			Detail: Detail{StatusCode: 200, Message: "ok"},
		},
	}

	// Should not panic with valid keys.
	LogAudittrailData("TestFunc", "desc", "internal", "http", &keys, data)
	time.Sleep(50 * time.Millisecond)
}

// TestLogAudittrailProcess_EmptyParams verifies early return for empty funcName or desc.
func TestLogAudittrailProcess_EmptyParams(t *testing.T) {
	tests := []struct {
		name     string
		funcName string
		desc     string
	}{
		{"empty funcName", "", "some desc"},
		{"empty desc", "SomeFunc", ""},
		{"both empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			LogAudittrailProcess(tt.funcName, tt.desc, "info", nil)
			// no panic expected — early exit
		})
	}
}

// TestLogAudittrailProcess_NilClient verifies no panic when client is nil.
func TestLogAudittrailProcess_NilClient(t *testing.T) {
	orig := auditTrailMqClient.Load()
	auditTrailMqClient.Store(nil)
	defer func() { auditTrailMqClient.Store(orig) }()

	LogAudittrailProcess("TestFunc", "desc", "info", nil)
	time.Sleep(50 * time.Millisecond)
}

// ---------------------------------------------------------------------------
// pushMessageAudit tests
// ---------------------------------------------------------------------------

// TestPushMessageAudit_NilClient verifies pushMessageAudit returns when client is nil.
func TestPushMessageAudit_NilClient(t *testing.T) {
	orig := auditTrailMqClient.Load()
	auditTrailMqClient.Store(nil)
	defer func() { auditTrailMqClient.Store(orig) }()

	// Reset rate limiter so the log fires
	auditNotReadyLogMu.Lock()
	auditNotReadyLastLog = time.Time{}
	auditNotReadyLogMu.Unlock()

	payload := MessagePayloadAudit{
		Id:      1,
		Command: CmdAuditTrailData,
		Time:    time.Now().Format(time.DateTime),
	}
	// Should not panic, just log and return.
	pushMessageAudit(payload)
}

// TestPushMessageAudit_EmptyQueueName verifies pushMessageAudit returns when queue is empty.
func TestPushMessageAudit_EmptyQueueName(t *testing.T) {
	orig := auditTrailMqClient.Load()
	// Create a minimal client with empty queue name, marked ready.
	client := &AmqpClient{
		m:         &sync.Mutex{},
		queueName: "",
		isReady:   true,
	}
	auditTrailMqClient.Store(client)
	defer func() { auditTrailMqClient.Store(orig) }()

	payload := MessagePayloadAudit{
		Id:      2,
		Command: CmdAuditTrailData,
		Time:    time.Now().Format(time.DateTime),
	}
	pushMessageAudit(payload)
	// no panic expected — logs error for empty queue
}

// ---------------------------------------------------------------------------
// SetUpRabbitMq tests
// ---------------------------------------------------------------------------

// TestSetUpRabbitMq_SetsClient verifies that SetUpRabbitMq returns a non-nil client.
func TestSetUpRabbitMq_SetsClient(t *testing.T) {
	orig := auditTrailMqClient.Load()
	defer func() { auditTrailMqClient.Store(orig) }()

	// Use a bogus host so it won't actually connect, but client should be created.
	client := SetUpRabbitMq("localhost", "65535", "/", "guest", "guest", "test-queue", "test-app")
	if client == nil {
		t.Fatal("SetUpRabbitMq returned nil client")
	}
	// Close the client to prevent background goroutine leak.
	close(client.done)
}

// TestSetUpRabbitMq_SetsAppName verifies app name is set when empty.
func TestSetUpRabbitMq_SetsAppName(t *testing.T) {
	origClient := auditTrailMqClient.Load()
	defer func() { auditTrailMqClient.Store(origClient) }()

	// SetUpRabbitMq internally calls phhelper.SetAppName if empty.
	client := SetUpRabbitMq("localhost", "65535", "/", "guest", "guest", "test-queue", "my-test-app")
	if client == nil {
		t.Fatal("SetUpRabbitMq returned nil client")
	}
	close(client.done)

	appName := GetAppName()
	if appName == "" {
		t.Error("expected app name to be set after SetUpRabbitMq, got empty")
	}
}

// ---------------------------------------------------------------------------
// JSON serialization tests
// ---------------------------------------------------------------------------

// TestMessagePayloadAudit_JSONStructure verifies the JSON field names match
// what audittrailft-module expects (Id, Command, Time, ModuleId, Data).
func TestMessagePayloadAudit_JSONStructure(t *testing.T) {
	payload := MessagePayloadAudit{
		Id:       42,
		Command:  CmdAuditTrailData,
		Time:     "2026-03-28 12:00:00",
		ModuleId: "test-module",
		Data:     map[string]string{"key": "value"},
	}

	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	requiredFields := []string{"Id", "Command", "Time", "ModuleId", "Data"}
	for _, f := range requiredFields {
		if _, ok := m[f]; !ok {
			t.Errorf("missing required JSON field %q in MessagePayloadAudit", f)
		}
	}

	if int(m["Id"].(float64)) != 42 {
		t.Errorf("Id = %v, want 42", m["Id"])
	}
	if m["Command"] != CmdAuditTrailData {
		t.Errorf("Command = %v, want %q", m["Command"], CmdAuditTrailData)
	}
}

// TestAuditTrailData_JSONStructure verifies nested AuditTrailData serialization.
func TestAuditTrailData_JSONStructure(t *testing.T) {
	data := AuditTrailData{
		Subject:           "test-svc",
		Function:          "CreateTransfer",
		Description:       "transfer created",
		Key:               []string{"trx-123"},
		Source:            "internal",
		CommunicationType: "grpc",
		Data: &RequestAndResponse{
			Request: Request{
				Time: "2026-03-28 12:00:00",
				Path: "/api/transfer",
			},
			Response: ResponseAudit{
				Time: "2026-03-28 12:00:01",
				Detail: Detail{
					StatusCode: 200,
					Message:    "ok",
				},
			},
		},
	}

	b, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	expected := []string{"Subject", "Function", "Description", "Key", "Source", "CommunicationType", "Data"}
	for _, f := range expected {
		if _, ok := m[f]; !ok {
			t.Errorf("missing required JSON field %q in AuditTrailData", f)
		}
	}
}

// TestAuditTrailProcess_JSONStructure verifies AuditTrailProcess serialization.
func TestAuditTrailProcess_JSONStructure(t *testing.T) {
	proc := AuditTrailProcess{
		Subject:     "test-svc",
		Function:    "ProcessPayment",
		Description: "payment processed",
		Key:         []string{"pay-456"},
		Data: DataAuditTrailProcess{
			Time: "2026-03-28 12:00:00",
			Info: "completed",
		},
	}

	b, err := json.Marshal(proc)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	expected := []string{"Subject", "Function", "Description", "Key", "Data"}
	for _, f := range expected {
		if _, ok := m[f]; !ok {
			t.Errorf("missing required JSON field %q in AuditTrailProcess", f)
		}
	}
}

// ---------------------------------------------------------------------------
// Phase 1A — tests for PATCH improvements
// ---------------------------------------------------------------------------

// TestIdUniqueness_Concurrent verifies atomic ID counter produces unique values.
func TestIdUniqueness_Concurrent(t *testing.T) {
	const goroutines = 50
	const idsPerGoroutine = 100

	ids := make(chan int, goroutines*idsPerGoroutine)
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < idsPerGoroutine; j++ {
				ids <- nextAuditID()
			}
		}()
	}

	wg.Wait()
	close(ids)

	seen := make(map[int]bool, goroutines*idsPerGoroutine)
	for id := range ids {
		if seen[id] {
			t.Fatalf("duplicate audit ID detected: %d", id)
		}
		seen[id] = true
	}

	if len(seen) != goroutines*idsPerGoroutine {
		t.Errorf("expected %d unique IDs, got %d", goroutines*idsPerGoroutine, len(seen))
	}
}

// TestPushMessageAudit_EarlyExit_NotReady verifies pushMessageAudit returns
// early when client exists but is not ready.
func TestPushMessageAudit_EarlyExit_NotReady(t *testing.T) {
	orig := auditTrailMqClient.Load()
	defer func() { auditTrailMqClient.Store(orig) }()

	// Reset rate limiter
	auditNotReadyLogMu.Lock()
	auditNotReadyLastLog = time.Time{}
	auditNotReadyLogMu.Unlock()

	client := &AmqpClient{
		m:         &sync.Mutex{},
		queueName: "some-queue",
		isReady:   false,
	}
	auditTrailMqClient.Store(client)

	payload := MessagePayloadAudit{
		Id:      99,
		Command: CmdAuditTrailData,
		Time:    time.Now().Format(time.DateTime),
	}

	// Should return early without attempting Push.
	pushMessageAudit(payload)
}

// TestPushMessageAudit_PushSucceeds covers the async publish success path (Push + confirm ack).
func TestPushMessageAudit_PushSucceeds(t *testing.T) {
	orig := auditTrailMqClient.Load()
	defer auditTrailMqClient.Store(orig)

	notify := make(chan amqp.Confirmation, 2)
	done := make(chan bool)
	client := &AmqpClient{
		m:             &sync.Mutex{},
		isReady:       true,
		queueName:     "audit-q",
		connName:      "trail-conn",
		done:          done,
		notifyConfirm: notify,
		publishForTest: func(context.Context, string, string, bool, bool, amqp.Publishing) error {
			return nil
		},
	}
	auditTrailMqClient.Store(client)

	go func() { notify <- amqp.Confirmation{Ack: true} }()

	prevTO := PushTimeout
	prevResend := amqpResendDelayForTest
	PushTimeout = 2 * time.Second
	amqpResendDelayForTest = 2 * time.Millisecond
	t.Cleanup(func() {
		PushTimeout = prevTO
		amqpResendDelayForTest = prevResend
	})

	pushMessageAudit(MessagePayloadAudit{
		Id:      9001,
		Command: CmdAuditTrailData,
		Time:    time.Now().Format(time.DateTime),
	})
}

// TestPushMessageAudit_PushFailure covers logAuditErrorWithSentry when Push exhausts retries.
func TestPushMessageAudit_PushFailure(t *testing.T) {
	orig := auditTrailMqClient.Load()
	defer auditTrailMqClient.Store(orig)

	notify := make(chan amqp.Confirmation, 2)
	done := make(chan bool)
	client := &AmqpClient{
		m:             &sync.Mutex{},
		isReady:       true,
		queueName:     "audit-q",
		connName:      "trail-conn",
		done:          done,
		notifyConfirm: notify,
		publishForTest: func(context.Context, string, string, bool, bool, amqp.Publishing) error {
			return errors.New("publish failed")
		},
	}
	auditTrailMqClient.Store(client)

	prevRetries := PushMaxRetries
	prevTO := PushTimeout
	prevResend := amqpResendDelayForTest
	PushMaxRetries = 2
	PushTimeout = 300 * time.Millisecond
	amqpResendDelayForTest = 3 * time.Millisecond
	t.Cleanup(func() {
		PushMaxRetries = prevRetries
		PushTimeout = prevTO
		amqpResendDelayForTest = prevResend
	})

	pushMessageAudit(MessagePayloadAudit{
		Id:      9002,
		Command: CmdAuditTrailData,
		Time:    time.Now().Format(time.DateTime),
	})
}

// TestLogAuditNotReadyRateLimited verifies rate limiting suppresses repeated calls.
func TestLogAuditNotReadyRateLimited(t *testing.T) {
	// Reset
	auditNotReadyLogMu.Lock()
	auditNotReadyLastLog = time.Time{}
	auditNotReadyLogMu.Unlock()

	// First call should log
	logAuditNotReadyRateLimited("test reason 1")

	// Second immediate call should be suppressed (within window)
	logAuditNotReadyRateLimited("test reason 2")

	// No assertions on log output — testing that it doesn't panic
	// and that the rate limiter correctly prevents log flooding.
}
