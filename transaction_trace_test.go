package paycloudhelper

import (
	"encoding/json"
	"strings"
	"testing"
)

// goldenTransactionTraceJSON is the byte-exact PA body both paycloudhelper and
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

func TestTransactionTrace_FTFieldsSerialise(t *testing.T) {
	tr := TransactionTrace{
		Event: "ftm.transfer_created", Domain: TraceDomainFundtransfer,
		TransferId: 42, ReferenceNumber: "FT-1-20261025-AB", PartnerReferenceNo: "P-9",
		DetailId: 7, DetailPartnerReferenceNo: "P-9-1", VendorReferenceNo: "V-1", MerchantId: 5,
	}
	b, err := json.Marshal(tr)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]any{
		"domain": "fundtransfer", "transferId": float64(42), "referenceNumber": "FT-1-20261025-AB",
		"partnerReferenceNo": "P-9", "detailId": float64(7), "detailPartnerReferenceNo": "P-9-1",
		"vendorReferenceNo": "V-1", "merchantId": float64(5),
	} {
		if got[key] != want {
			t.Errorf("field %s = %v; want %v", key, got[key], want)
		}
	}
	for _, key := range []string{"trxId", "trxNo", "mId", "trxReferenceNo", "trxPayCode1", "trxPcId", "ticketId"} {
		if _, exists := got[key]; exists {
			t.Errorf("FT trace carries PA key %s", key)
		}
	}
	b, err = json.Marshal(TransactionTrace{Event: "x", TrxNo: "T1"})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"domain", "transferId", "referenceNumber", "partnerReferenceNo", "detailId", "detailPartnerReferenceNo", "vendorReferenceNo", "merchantId"} {
		if strings.Contains(string(b), `"`+key+`"`) {
			t.Errorf("PA trace carries empty FT key %s", key)
		}
	}
}

func TestHasAnyTraceKey_FTKeys(t *testing.T) {
	for name, tr := range map[string]TransactionTrace{
		"transferId":     {TransferId: 1},
		"refused create": {PartnerReferenceNo: "P-1"},
	} {
		t.Run(name, func(t *testing.T) {
			if !hasAnyTraceKey(tr) {
				t.Fatal("FT key must allow publishing")
			}
		})
	}
	if hasAnyTraceKey(TransactionTrace{DetailId: 7}) {
		t.Fatal("detailId alone must not allow publishing")
	}
}

func TestLogTransactionTrace_FTDefaultsToV2(t *testing.T) {
	origPub, origEnabled := auditTrxPublisher, auditTrxEnabled.Load()
	t.Cleanup(func() {
		auditTrxPublisher = origPub
		auditTrxEnabled.Store(origEnabled)
	})
	pub := NewAuditPublisher(nil, WithBufferSize(4))
	auditTrxPublisher = pub
	auditTrxEnabled.Store(true)
	for _, tc := range []struct {
		name        string
		trace       TransactionTrace
		wantVersion int
	}{
		{"FT transfer", TransactionTrace{Event: "ftm.transfer_created", Domain: TraceDomainFundtransfer, TransferId: 42}, 2},
		{"FT refused create", TransactionTrace{Event: "fth2h.disbursement_received", Domain: TraceDomainFundtransfer, PartnerReferenceNo: "P-1"}, 2},
		{"PA", TransactionTrace{Event: "tm.order_created", TrxNo: "T1"}, 1},
		{"explicit version", TransactionTrace{V: 3, Event: "ftm.transfer_created", Domain: TraceDomainFundtransfer, TransferId: 42}, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			LogTransactionTrace(tc.trace)
			if len(pub.msgChan) != 1 {
				t.Fatalf("submitted %d messages; want 1", len(pub.msgChan))
			}
			msg := <-pub.msgChan
			got, ok := msg.payload.Data.(TransactionTrace)
			if !ok || got.V != tc.wantVersion {
				t.Fatalf("payload = %+v; want schema v%d", msg.payload.Data, tc.wantVersion)
			}
			if got.EventTime == "" || got.Kind != TraceKindHop || msg.payload.Command != CmdTransactionTrace {
				t.Fatalf("missing trace defaults: %+v", got)
			}
		})
	}
}
