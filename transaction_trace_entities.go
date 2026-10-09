package paycloudhelper

import "time"

const CmdTransactionTrace = "transaction-trace"

const (
	TraceKindHop       = "hop"
	TraceKindMilestone = "milestone"
	TraceKindPoll      = "poll"

	TraceLegCreateOrder = "create_order"
	TraceLegCallback    = "callback"
	TraceLegLifecycle   = "lifecycle"

	TraceDirectionInbound  = "inbound"
	TraceDirectionOutbound = "outbound"
	TraceDirectionInternal = "internal"

	TraceStatusProcessing = "processing"
	TraceStatusSuccess    = "success"
	TraceStatusFailed     = "failed"
	TraceStatusExpired    = "expired"
	TraceStatusAborted    = "aborted"
	TraceStatusSkipped    = "skipped"
)

// TraceHTTP describes the HTTP side of a hop. Headers are never carried.
type TraceHTTP struct {
	Method string `json:"method,omitempty"`
	URL    string `json:"url,omitempty"`
	Status int    `json:"status,omitempty"`
}

// TransactionTrace is one inbound/outbound hop or lifecycle milestone of a
// transaction. Field names are the owner's search vocabulary; keep them stable.
// createdAt, expireAt and truncated are written by the consumer, not here.
type TransactionTrace struct {
	V         int    `json:"v"`
	Event     string `json:"event"`
	Kind      string `json:"kind"`
	Leg       string `json:"leg"`
	Direction string `json:"direction"`
	Service   string `json:"service"`
	Function  string `json:"function"`
	Peer      string `json:"peer,omitempty"`
	Transport string `json:"transport"`
	Status    string `json:"status"`

	MId            int64  `json:"mId,omitempty"`
	MerchantCode   string `json:"merchantCode,omitempty"`
	TrxId          int64  `json:"trxId,omitempty"`
	TrxNo          string `json:"trxNo,omitempty"`
	TrxReferenceNo string `json:"trxReferenceNo,omitempty"`
	TrxPayCode1    string `json:"trxPayCode1,omitempty"`
	TrxPcId        int    `json:"trxPcId,omitempty"`
	TicketId       string `json:"ticketId,omitempty"`
	ExternalId     string `json:"externalId,omitempty"`
	RequestId      string `json:"requestId,omitempty"`
	Traceparent    string `json:"traceparent,omitempty"`

	// Callback hops only (ADR D10): both directions are countable per attempt.
	CallbackDirection string `json:"callbackDirection,omitempty"` // vendor_to_paycloud | paycloud_to_merchant
	AttemptId         string `json:"attemptId,omitempty"`         // vendor X-External-Id / requestId, or <trxNo>:<cycle>:<attempt>
	Cycle             int    `json:"cycle,omitempty"`             // 1 auto, 2+ temporal replay, 100+ manual
	Attempt           int    `json:"attempt,omitempty"`           // 1..N within the cycle

	HTTP       *TraceHTTP `json:"http,omitempty"`
	Request    any        `json:"request,omitempty"`
	Response   any        `json:"response,omitempty"`
	Message    string     `json:"message,omitempty"`
	ErrorCode  string     `json:"errorCode,omitempty"`
	DurationMs int64      `json:"durationMs,omitempty"`

	EventTime string `json:"eventTime"`
	// omitzero, not omitempty: encoding/json still emits a zero time.Time
	// under omitempty. The consumer writes createdAt; producers leave it zero.
	CreatedAt time.Time `json:"createdAt,omitzero"`
}
