package paycloudhelper

import (
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Phase 1B — V2 function tests
// ---------------------------------------------------------------------------

// TestLogAudittrailDataV2_FallsBack verifies V2 falls back to V1 when no publisher.
func TestLogAudittrailDataV2_FallsBack(t *testing.T) {
	origPub := auditPublisher
	origClient := auditTrailMqClient
	auditPublisher = nil
	auditTrailMqClient = nil
	defer func() {
		auditPublisher = origPub
		auditTrailMqClient = origClient
	}()

	data := &RequestAndResponse{
		Response: ResponseAudit{
			Detail: Detail{StatusCode: 200, Message: "ok"},
		},
	}

	// Should fall back to V1 which will hit nil-client early exit in goroutine.
	LogAudittrailDataV2("TestFunc", "desc", "internal", "http", nil, data)
	time.Sleep(50 * time.Millisecond)
}

// TestLogAudittrailProcessV2_FallsBack verifies V2 falls back to V1.
func TestLogAudittrailProcessV2_FallsBack(t *testing.T) {
	origPub := auditPublisher
	origClient := auditTrailMqClient
	auditPublisher = nil
	auditTrailMqClient = nil
	defer func() {
		auditPublisher = origPub
		auditTrailMqClient = origClient
	}()

	LogAudittrailProcessV2("TestFunc", "desc", "info", nil)
	time.Sleep(50 * time.Millisecond)
}

// TestLogAudittrailDataV2_EmptyFuncName verifies early return.
func TestLogAudittrailDataV2_EmptyFuncName(t *testing.T) {
	origPub := auditPublisher
	auditPublisher = NewAuditPublisher(nil, WithBufferSize(10))
	defer func() { auditPublisher = origPub }()

	data := &RequestAndResponse{
		Response: ResponseAudit{
			Detail: Detail{StatusCode: 200, Message: "ok"},
		},
	}

	LogAudittrailDataV2("", "desc", "internal", "http", nil, data)

	// Channel should be empty — early exit before Submit.
	if len(auditPublisher.msgChan) != 0 {
		t.Errorf("expected 0 messages, got %d", len(auditPublisher.msgChan))
	}
}

// TestLogAudittrailDataV2_NilData verifies early return.
func TestLogAudittrailDataV2_NilData(t *testing.T) {
	origPub := auditPublisher
	auditPublisher = NewAuditPublisher(nil, WithBufferSize(10))
	defer func() { auditPublisher = origPub }()

	LogAudittrailDataV2("TestFunc", "desc", "internal", "http", nil, nil)

	if len(auditPublisher.msgChan) != 0 {
		t.Errorf("expected 0 messages, got %d", len(auditPublisher.msgChan))
	}
}

// TestLogAudittrailDataV2_ZeroStatusCode verifies early return.
func TestLogAudittrailDataV2_ZeroStatusCode(t *testing.T) {
	origPub := auditPublisher
	auditPublisher = NewAuditPublisher(nil, WithBufferSize(10))
	defer func() { auditPublisher = origPub }()

	data := &RequestAndResponse{
		Response: ResponseAudit{
			Detail: Detail{StatusCode: 0},
		},
	}

	LogAudittrailDataV2("TestFunc", "desc", "internal", "http", nil, data)

	if len(auditPublisher.msgChan) != 0 {
		t.Errorf("expected 0 messages, got %d", len(auditPublisher.msgChan))
	}
}

// TestLogAudittrailDataV2_SubmitsToPublisher verifies valid data is submitted.
func TestLogAudittrailDataV2_SubmitsToPublisher(t *testing.T) {
	origPub := auditPublisher
	auditPublisher = NewAuditPublisher(nil, WithBufferSize(10))
	defer func() { auditPublisher = origPub }()

	data := &RequestAndResponse{
		Response: ResponseAudit{
			Detail: Detail{StatusCode: 200, Message: "ok"},
		},
	}
	keys := []string{"key1"}

	LogAudittrailDataV2("TestFunc", "desc", "internal", "http", &keys, data)

	if len(auditPublisher.msgChan) != 1 {
		t.Errorf("expected 1 message in channel, got %d", len(auditPublisher.msgChan))
	}
}

// TestLogAudittrailProcessV2_EmptyParams verifies early return.
func TestLogAudittrailProcessV2_EmptyParams(t *testing.T) {
	origPub := auditPublisher
	auditPublisher = NewAuditPublisher(nil, WithBufferSize(10))
	defer func() { auditPublisher = origPub }()

	LogAudittrailProcessV2("", "desc", "info", nil)
	LogAudittrailProcessV2("Func", "", "info", nil)

	if len(auditPublisher.msgChan) != 0 {
		t.Errorf("expected 0 messages, got %d", len(auditPublisher.msgChan))
	}
}

// TestLogAudittrailProcessV2_SubmitsToPublisher verifies valid process data is submitted.
func TestLogAudittrailProcessV2_SubmitsToPublisher(t *testing.T) {
	origPub := auditPublisher
	auditPublisher = NewAuditPublisher(nil, WithBufferSize(10))
	defer func() { auditPublisher = origPub }()

	keys := []string{"k1"}
	LogAudittrailProcessV2("TestFunc", "desc", "info", &keys)

	if len(auditPublisher.msgChan) != 1 {
		t.Errorf("expected 1 message in channel, got %d", len(auditPublisher.msgChan))
	}
}

// TestSetUpAuditTrailPublisher_SetsGlobal verifies SetUpAuditTrailPublisher
// sets the package-level auditPublisher.
func TestSetUpAuditTrailPublisher_SetsGlobal(t *testing.T) {
	origPub := auditPublisher
	origClient := auditTrailMqClient
	defer func() {
		auditPublisher = origPub
		auditTrailMqClient = origClient
	}()

	auditPublisher = nil

	// Use bogus host — publisher will be created but client won't connect.
	pub := SetUpAuditTrailPublisher("localhost", "65535", "/", "guest", "guest", "test-q", "test-app")
	if pub == nil {
		t.Fatal("SetUpAuditTrailPublisher returned nil")
	}

	if auditPublisher == nil {
		t.Error("auditPublisher global not set after SetUpAuditTrailPublisher")
	}

	// Clean up: stop publisher and close underlying client.
	pub.Stop()
	if pub.client != nil {
		close(pub.client.done)
	}
}

// TestGetAuditPublisher verifies GetAuditPublisher returns current state.
func TestGetAuditPublisher(t *testing.T) {
	origPub := auditPublisher
	defer func() { auditPublisher = origPub }()

	auditPublisher = nil
	if GetAuditPublisher() != nil {
		t.Error("expected nil when no publisher set")
	}

	p := NewAuditPublisher(nil)
	auditPublisher = p
	if GetAuditPublisher() != p {
		t.Error("expected GetAuditPublisher to return the set publisher")
	}
}
