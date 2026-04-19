package paycloudhelper

import "time"

const CmdAuditTrailTrx = "audit-trail-trx"

const (
	AuditTrxStateRequestReceived     = "request_received"
	AuditTrxStateRequestValidated    = "request_validated"
	AuditTrxStateOrderCreated        = "order_created"
	AuditTrxStateChannelSelected     = "channel_selected"
	AuditTrxStateChannelProcessed    = "channel_processed"
	AuditTrxStateVendorRequestSent   = "vendor_request_sent"
	AuditTrxStateVendorTokenAcquired = "vendor_token_acquired"
	AuditTrxStateQrGenerated         = "qr_generated"
	AuditTrxStateVendorRequestFailed = "vendor_request_failed"
	AuditTrxStateTransactionUpdated  = "transaction_updated"
	AuditTrxStatePaymentNotified     = "payment_notified"
	AuditTrxStateResponseReturned    = "response_returned"
	AuditTrxStateOrderExpired        = "order_expired"
	AuditTrxStatePaymentReceived     = "payment_received"
	AuditTrxStateStatusChecked       = "status_checked"
)

const (
	AuditTrxStatusProcessing = "processing"
	AuditTrxStatusSuccess    = "success"
	AuditTrxStatusFailed     = "failed"
	AuditTrxStatusExpired    = "expired"
)

// AuditTrailTrx represents one transaction lifecycle event.
type AuditTrailTrx struct {
	ReffNo   string `json:"reffNo"`
	OrderNo  string `json:"orderNo"`
	TicketId string `json:"ticketId,omitempty"`

	Status  string `json:"status"`
	State   string `json:"state"`
	Message string `json:"message"`

	Service           string `json:"service"`
	Function          string `json:"function"`
	Description       string `json:"description"`
	CommunicationType string `json:"communicationType"`

	EventTime  string `json:"eventTime"`
	DurationMs int64  `json:"durationMs,omitempty"`

	Amount      string `json:"amount,omitempty"`
	Currency    string `json:"currency,omitempty"`
	MerchantNo  string `json:"merchantNo,omitempty"`
	PaymentCode string `json:"paymentCode,omitempty"`
	QrValue     string `json:"qrValue,omitempty"`
	Rrn         string `json:"rrn,omitempty"`
	ErrorCode   string `json:"errorCode,omitempty"`
	VendorName  string `json:"vendorName,omitempty"`

	Request  interface{} `json:"request,omitempty"`
	Response interface{} `json:"response,omitempty"`

	Metadata map[string]interface{} `json:"metadata,omitempty"`

	CreatedAt time.Time `json:"createdAt,omitempty"`
}
