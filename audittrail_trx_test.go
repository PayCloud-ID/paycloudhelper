package paycloudhelper

import (
	"sync"
	"testing"
	"time"
)

func TestCmdAuditTrailTrx_Value(t *testing.T) {
	if CmdAuditTrailTrx != "audit-trail-trx" {
		t.Errorf("expected 'audit-trail-trx', got %q", CmdAuditTrailTrx)
	}
}

func TestAuditTrxState_Constants(t *testing.T) {
	states := []string{
		AuditTrxStateRequestReceived,
		AuditTrxStateRequestValidated,
		AuditTrxStateOrderCreated,
		AuditTrxStateChannelSelected,
		AuditTrxStateChannelProcessed,
		AuditTrxStateVendorRequestSent,
		AuditTrxStateVendorTokenAcquired,
		AuditTrxStateQrGenerated,
		AuditTrxStateVendorRequestFailed,
		AuditTrxStateTransactionUpdated,
		AuditTrxStatePaymentNotified,
		AuditTrxStateResponseReturned,
		AuditTrxStateOrderExpired,
		AuditTrxStatePaymentReceived,
		AuditTrxStateStatusChecked,
	}

	seen := make(map[string]bool)
	for _, s := range states {
		if s == "" {
			t.Error("state constant must not be empty")
		}
		if seen[s] {
			t.Errorf("duplicate state: %q", s)
		}
		seen[s] = true
	}
}

func TestAuditTrxStatus_Constants(t *testing.T) {
	statuses := []string{
		AuditTrxStatusProcessing,
		AuditTrxStatusSuccess,
		AuditTrxStatusFailed,
		AuditTrxStatusExpired,
	}

	for _, s := range statuses {
		if s == "" {
			t.Error("status constant must not be empty")
		}
	}
}

func TestSetUpAuditTrailTrxPublisher_Disabled(t *testing.T) {
	origPub := auditTrxPublisher
	origEnabled := auditTrxEnabled.Load()
	defer func() {
		auditTrxPublisher = origPub
		auditTrxEnabled.Store(origEnabled)
	}()

	pub := SetUpAuditTrailTrxPublisher(false, "h", "p", "v", "u", "pw", "q", "app")

	if pub != nil {
		t.Error("expected nil publisher when disabled")
	}
	if IsAuditTrailTrxEnabled() {
		t.Error("expected IsAuditTrailTrxEnabled to return false")
	}
}

func TestSetUpAuditTrailTrxPublisher_Enabled(t *testing.T) {
	origPub := auditTrxPublisher
	origEnabled := auditTrxEnabled.Load()
	defer func() {
		if auditTrxPublisher != nil && auditTrxPublisher != origPub {
			auditTrxPublisher.Stop()
		}
		auditTrxPublisher = origPub
		auditTrxEnabled.Store(origEnabled)
	}()

	pub := SetUpAuditTrailTrxPublisher(
		true,
		"localhost", "5672", "/", "guest", "guest", "test-q", "test-app",
		WithWorkerCount(2),
		WithBufferSize(10),
	)

	if pub == nil {
		t.Fatal("expected non-nil publisher when enabled")
	}
	if !IsAuditTrailTrxEnabled() {
		t.Error("expected IsAuditTrailTrxEnabled to return true")
	}
	if GetAuditTrailTrxPublisher() != pub {
		t.Error("GetAuditTrailTrxPublisher should return the created publisher")
	}
}

func TestLogAuditTrailTrx_DisabledNoOp(t *testing.T) {
	origPub := auditTrxPublisher
	origEnabled := auditTrxEnabled.Load()
	defer func() {
		auditTrxPublisher = origPub
		auditTrxEnabled.Store(origEnabled)
	}()

	auditTrxEnabled.Store(false)
	auditTrxPublisher = nil

	LogAuditTrailTrx(AuditTrailTrx{
		ReffNo:  "test-ref",
		OrderNo: "test-order",
		State:   AuditTrxStateOrderCreated,
	})
}

func TestLogAuditTrailTrx_NilPublisherNoOp(t *testing.T) {
	origPub := auditTrxPublisher
	origEnabled := auditTrxEnabled.Load()
	defer func() {
		auditTrxPublisher = origPub
		auditTrxEnabled.Store(origEnabled)
	}()

	auditTrxEnabled.Store(true)
	auditTrxPublisher = nil

	LogAuditTrailTrx(AuditTrailTrx{
		ReffNo: "test-ref",
		State:  AuditTrxStateOrderCreated,
	})
}

func TestLogAuditTrailTrx_EmptyCorrelationIDs(t *testing.T) {
	origPub := auditTrxPublisher
	origEnabled := auditTrxEnabled.Load()
	defer func() {
		auditTrxPublisher = origPub
		auditTrxEnabled.Store(origEnabled)
	}()

	pub := NewAuditPublisher(nil, WithBufferSize(10))
	auditTrxPublisher = pub
	auditTrxEnabled.Store(true)

	LogAuditTrailTrx(AuditTrailTrx{
		ReffNo:  "",
		OrderNo: "",
		State:   AuditTrxStateOrderCreated,
	})

	if len(pub.msgChan) != 0 {
		t.Errorf("expected 0 messages when both IDs empty, got %d", len(pub.msgChan))
	}
}

func TestLogAuditTrailTrx_SubmitsWithReffNoOnly(t *testing.T) {
	origPub := auditTrxPublisher
	origEnabled := auditTrxEnabled.Load()
	defer func() {
		auditTrxPublisher = origPub
		auditTrxEnabled.Store(origEnabled)
	}()

	pub := NewAuditPublisher(nil, WithBufferSize(10))
	auditTrxPublisher = pub
	auditTrxEnabled.Store(true)

	LogAuditTrailTrx(AuditTrailTrx{
		ReffNo:   "test-ref",
		Status:   AuditTrxStatusProcessing,
		State:    AuditTrxStateOrderCreated,
		Function: "TestFunc",
	})

	if len(pub.msgChan) != 1 {
		t.Errorf("expected 1 message, got %d", len(pub.msgChan))
	}
}

func TestLogAuditTrailTrx_SubmitsWithOrderNoOnly(t *testing.T) {
	origPub := auditTrxPublisher
	origEnabled := auditTrxEnabled.Load()
	defer func() {
		auditTrxPublisher = origPub
		auditTrxEnabled.Store(origEnabled)
	}()

	pub := NewAuditPublisher(nil, WithBufferSize(10))
	auditTrxPublisher = pub
	auditTrxEnabled.Store(true)

	LogAuditTrailTrx(AuditTrailTrx{
		OrderNo:  "test-order",
		Status:   AuditTrxStatusSuccess,
		State:    AuditTrxStateQrGenerated,
		Function: "TestFunc",
	})

	if len(pub.msgChan) != 1 {
		t.Errorf("expected 1 message, got %d", len(pub.msgChan))
	}
}

func TestLogAuditTrailTrx_AutoSetsDefaults(t *testing.T) {
	origPub := auditTrxPublisher
	origEnabled := auditTrxEnabled.Load()
	defer func() {
		auditTrxPublisher = origPub
		auditTrxEnabled.Store(origEnabled)
	}()

	pub := NewAuditPublisher(nil, WithBufferSize(10))
	auditTrxPublisher = pub
	auditTrxEnabled.Store(true)

	before := time.Now()
	LogAuditTrailTrx(AuditTrailTrx{
		ReffNo:   "test-ref",
		State:    AuditTrxStateOrderCreated,
		Function: "TestFunc",
	})

	if len(pub.msgChan) != 1 {
		t.Fatalf("expected 1 message, got %d", len(pub.msgChan))
	}

	msg := <-pub.msgChan
	data, ok := msg.payload.Data.(AuditTrailTrx)
	if !ok {
		t.Fatal("payload Data is not AuditTrailTrx")
	}

	if data.EventTime == "" {
		t.Error("EventTime should be auto-set")
	}

	eventTime, err := time.Parse(time.RFC3339Nano, data.EventTime)
	if err != nil {
		t.Fatalf("EventTime should be RFC3339Nano, got %q: %v", data.EventTime, err)
	}
	if eventTime.Before(before) {
		t.Error("EventTime should be >= test start time")
	}

	if msg.payload.Command != CmdAuditTrailTrx {
		t.Errorf("expected command %q, got %q", CmdAuditTrailTrx, msg.payload.Command)
	}
}

func TestLogAuditTrailTrx_PreservesUserEventTime(t *testing.T) {
	origPub := auditTrxPublisher
	origEnabled := auditTrxEnabled.Load()
	defer func() {
		auditTrxPublisher = origPub
		auditTrxEnabled.Store(origEnabled)
	}()

	pub := NewAuditPublisher(nil, WithBufferSize(10))
	auditTrxPublisher = pub
	auditTrxEnabled.Store(true)

	customTime := "2026-01-01T00:00:00Z"
	LogAuditTrailTrx(AuditTrailTrx{
		ReffNo:    "test-ref",
		State:     AuditTrxStateOrderCreated,
		Function:  "TestFunc",
		EventTime: customTime,
	})

	msg := <-pub.msgChan
	data := msg.payload.Data.(AuditTrailTrx)
	if data.EventTime != customTime {
		t.Errorf("expected preserved EventTime %q, got %q", customTime, data.EventTime)
	}
}

func TestLogAuditTrailTrx_MetadataPassthrough(t *testing.T) {
	origPub := auditTrxPublisher
	origEnabled := auditTrxEnabled.Load()
	defer func() {
		auditTrxPublisher = origPub
		auditTrxEnabled.Store(origEnabled)
	}()

	pub := NewAuditPublisher(nil, WithBufferSize(10))
	auditTrxPublisher = pub
	auditTrxEnabled.Store(true)

	LogAuditTrailTrx(AuditTrailTrx{
		ReffNo:   "test-ref",
		State:    AuditTrxStateOrderCreated,
		Function: "TestFunc",
		Metadata: map[string]interface{}{
			"retryCount": 2,
			"clientIp":   "10.0.1.5",
		},
	})

	msg := <-pub.msgChan
	data := msg.payload.Data.(AuditTrailTrx)
	if data.Metadata == nil {
		t.Fatal("Metadata should not be nil")
	}
	if data.Metadata["retryCount"] != 2 {
		t.Errorf("expected retryCount=2, got %v", data.Metadata["retryCount"])
	}
}

func TestLogAuditTrailTrx_ConcurrentSafe(t *testing.T) {
	origPub := auditTrxPublisher
	origEnabled := auditTrxEnabled.Load()
	defer func() {
		auditTrxPublisher = origPub
		auditTrxEnabled.Store(origEnabled)
	}()

	pub := NewAuditPublisher(nil, WithBufferSize(5000))
	auditTrxPublisher = pub
	auditTrxEnabled.Store(true)

	var wg sync.WaitGroup
	goroutines := 50
	perGoroutine := 100

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < perGoroutine; j++ {
				LogAuditTrailTrx(AuditTrailTrx{
					ReffNo:   "ref-concurrent",
					OrderNo:  "order-concurrent",
					State:    AuditTrxStateOrderCreated,
					Function: "TestFunc",
				})
			}
		}()
	}

	wg.Wait()
	expected := goroutines * perGoroutine
	got := len(pub.msgChan)
	if got != expected {
		t.Errorf("expected %d messages, got %d", expected, got)
	}
}

func TestLogAuditTrailTrx_UsesUniqueIDs(t *testing.T) {
	origPub := auditTrxPublisher
	origEnabled := auditTrxEnabled.Load()
	defer func() {
		auditTrxPublisher = origPub
		auditTrxEnabled.Store(origEnabled)
	}()

	pub := NewAuditPublisher(nil, WithBufferSize(100))
	auditTrxPublisher = pub
	auditTrxEnabled.Store(true)

	count := 50
	for i := 0; i < count; i++ {
		LogAuditTrailTrx(AuditTrailTrx{
			ReffNo:   "ref-id-test",
			State:    AuditTrxStateOrderCreated,
			Function: "TestFunc",
		})
	}

	ids := make(map[int]bool)
	for i := 0; i < count; i++ {
		msg := <-pub.msgChan
		if ids[msg.payload.Id] {
			t.Errorf("duplicate ID: %d", msg.payload.Id)
		}
		ids[msg.payload.Id] = true
	}
}

func TestIsAuditTrailTrxEnabled_States(t *testing.T) {
	origPub := auditTrxPublisher
	origEnabled := auditTrxEnabled.Load()
	defer func() {
		auditTrxPublisher = origPub
		auditTrxEnabled.Store(origEnabled)
	}()

	auditTrxPublisher = nil
	auditTrxEnabled.Store(false)
	if IsAuditTrailTrxEnabled() {
		t.Error("should be disabled when both nil and false")
	}

	auditTrxEnabled.Store(true)
	auditTrxPublisher = nil
	if IsAuditTrailTrxEnabled() {
		t.Error("should be disabled when publisher is nil")
	}

	auditTrxEnabled.Store(false)
	auditTrxPublisher = NewAuditPublisher(nil, WithBufferSize(10))
	if IsAuditTrailTrxEnabled() {
		t.Error("should be disabled when enabled is false")
	}

	auditTrxEnabled.Store(true)
	if !IsAuditTrailTrxEnabled() {
		t.Error("should be enabled when both set")
	}
}
