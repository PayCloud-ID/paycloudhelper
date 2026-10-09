package paycloudhelper

import (
	"encoding/json"
	"strings"
	"testing"
)

// goldenTransactionTraceJSON is the byte-exact body both paycloudhelper and
// paycloudhelper-rs pc-audit must emit for the same logical record. Field
// order follows the owner vocabulary. createdAt is a consumer field and is
// absent. Keep this string identical to the Rust golden.
const goldenTransactionTraceJSON = `{"v":1,"event":"tm.order_created","kind":"milestone","leg":"create_order","direction":"internal","service":"transaction-module","function":"persistOrderTransaction","peer":"mongo","transport":"grpc","status":"success","mId":202610001,"merchantCode":"202610001","trxId":14964329,"trxNo":"T26100014964329","trxReferenceNo":"PRS20261009115424","trxPayCode1":"T26100159355214","trxPcId":3,"ticketId":"tkt-1","externalId":"ext-1","requestId":"req-1","traceparent":"00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01","callbackDirection":"paycloud_to_merchant","attemptId":"T26100014964329:1:2","cycle":1,"attempt":2,"http":{"method":"POST","url":"https://vendor.example/qr","status":200},"request":{"Authorization":"Bearer live-token"},"response":{"status_code":200},"message":"ok","errorCode":"00","durationMs":42,"eventTime":"2026-10-09T11:54:24.123456789+07:00"}`

func canonicalTransactionTrace() TransactionTrace {
	return TransactionTrace{
		V:                 1,
		Event:             "tm.order_created",
		Kind:              TraceKindMilestone,
		Leg:               TraceLegCreateOrder,
		Direction:         TraceDirectionInternal,
		Service:           "transaction-module",
		Function:          "persistOrderTransaction",
		Peer:              "mongo",
		Transport:         "grpc",
		Status:            TraceStatusSuccess,
		MId:               202610001,
		MerchantCode:      "202610001",
		TrxId:             14964329,
		TrxNo:             "T26100014964329",
		TrxReferenceNo:    "PRS20261009115424",
		TrxPayCode1:       "T26100159355214",
		TrxPcId:           3,
		TicketId:          "tkt-1",
		ExternalId:        "ext-1",
		RequestId:         "req-1",
		Traceparent:       "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
		CallbackDirection: "paycloud_to_merchant",
		AttemptId:         "T26100014964329:1:2",
		Cycle:             1,
		Attempt:           2,
		HTTP: &TraceHTTP{
			Method: "POST",
			URL:    "https://vendor.example/qr",
			Status: 200,
		},
		Request:    map[string]any{"Authorization": "Bearer live-token"},
		Response:   map[string]any{"status_code": 200},
		Message:    "ok",
		ErrorCode:  "00",
		DurationMs: 42,
		EventTime:  "2026-10-09T11:54:24.123456789+07:00",
	}
}

func TestCmdTransactionTrace_Value(t *testing.T) {
	if CmdTransactionTrace != "transaction-trace" {
		t.Fatalf("got %q", CmdTransactionTrace)
	}
}

func TestTransactionTrace_JSONNamesMatchOwnerVocabulary(t *testing.T) {
	b, err := json.Marshal(TransactionTrace{
		Event: "tm.order_created", Kind: TraceKindMilestone, Leg: TraceLegCreateOrder,
		Direction: TraceDirectionInternal, TrxId: 14964329, TrxNo: "T26100014964329",
		TrxReferenceNo: "PRS20261009115424", TrxPayCode1: "T26100159355214", TrxPcId: 3,
		MId: 202610001, MerchantCode: "202610001",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`"trxId":14964329`, `"trxNo":"T26100014964329"`, `"trxReferenceNo"`,
		`"trxPayCode1"`, `"trxPcId":3`, `"mId":202610001`, `"merchantCode"`,
		`"event":"tm.order_created"`,
	} {
		if !strings.Contains(string(b), want) {
			t.Errorf("missing %s in %s", want, b)
		}
	}
}

func TestTransactionTrace_JSONMatchesRustGolden(t *testing.T) {
	b, err := json.Marshal(canonicalTransactionTrace())
	if err != nil {
		t.Fatal(err)
	}
	if got := string(b); got != goldenTransactionTraceJSON {
		t.Fatalf("JSON drifted from the Rust golden\n got: %s\nwant: %s", got, goldenTransactionTraceJSON)
	}
}

func TestLogTransactionTrace_SkipsWhenNoKeys(t *testing.T) {
	origPub := auditTrxPublisher
	origEnabled := auditTrxEnabled.Load()
	t.Cleanup(func() {
		auditTrxPublisher = origPub
		auditTrxEnabled.Store(origEnabled)
	})

	auditTrxEnabled.Store(true)
	auditTrxPublisher = nil
	LogTransactionTrace(TransactionTrace{Event: "x"}) // must not panic
	if hasAnyTraceKey(TransactionTrace{}) {
		t.Fatal("empty record must have no key")
	}
	if !hasAnyTraceKey(TransactionTrace{TicketId: "t"}) {
		t.Fatal("ticketId is a key")
	}
}

func TestLogTransactionTrace_KeylessDoesNotSubmit(t *testing.T) {
	origPub := auditTrxPublisher
	origEnabled := auditTrxEnabled.Load()
	t.Cleanup(func() {
		auditTrxPublisher = origPub
		auditTrxEnabled.Store(origEnabled)
	})

	pub := NewAuditPublisher(nil, WithBufferSize(4))
	auditTrxPublisher = pub
	auditTrxEnabled.Store(true)

	LogTransactionTrace(TransactionTrace{Event: "x"})
	LogTransactionTrace(TransactionTrace{TrxNo: "T1"})
	if len(pub.msgChan) != 0 {
		t.Fatalf("keyless or event-less records submitted %d messages", len(pub.msgChan))
	}
}

func TestLogTransactionTrace_SubmitsTransactionTraceCommand(t *testing.T) {
	origPub := auditTrxPublisher
	origEnabled := auditTrxEnabled.Load()
	t.Cleanup(func() {
		auditTrxPublisher = origPub
		auditTrxEnabled.Store(origEnabled)
	})

	pub := NewAuditPublisher(nil, WithBufferSize(4))
	auditTrxPublisher = pub
	auditTrxEnabled.Store(true)

	LogTransactionTrace(canonicalTransactionTrace())
	if len(pub.msgChan) != 1 {
		t.Fatalf("submitted %d messages, want 1", len(pub.msgChan))
	}
	msg := <-pub.msgChan
	if msg.payload.Command != CmdTransactionTrace {
		t.Fatalf("command %q", msg.payload.Command)
	}
	b, err := json.Marshal(msg.payload.Data)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(b); got != goldenTransactionTraceJSON {
		t.Fatalf("submitted data drifted\n got: %s\nwant: %s", got, goldenTransactionTraceJSON)
	}
}
