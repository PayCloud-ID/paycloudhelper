package paycloudhelper

import "time"

func hasAnyTraceKey(d TransactionTrace) bool {
	return d.TrxId != 0 || d.TrxNo != "" || d.TrxReferenceNo != "" || d.TrxPayCode1 != "" ||
		d.TicketId != "" || d.MerchantCode != "" || d.MId != 0
}

// LogTransactionTrace publishes one trace event on the per-transaction audit
// queue (the same publisher as LogAuditTrailTrx). Best-effort, non-blocking.
// A record with no event or no transaction key is skipped.
func LogTransactionTrace(data TransactionTrace) {
	if !auditTrxEnabled.Load() || auditTrxPublisher == nil {
		return
	}
	if data.Event == "" || !hasAnyTraceKey(data) {
		LogW("[LogTransactionTrace] skipped: event=%q has no transaction key", data.Event)
		return
	}
	if data.V == 0 {
		data.V = 1
	}
	if data.Kind == "" {
		data.Kind = TraceKindHop
	}
	if data.Service == "" {
		data.Service = GetAppName()
	}
	if data.EventTime == "" {
		data.EventTime = time.Now().Format(time.RFC3339Nano)
	}
	auditTrxPublisher.Submit(MessagePayloadAudit{
		Id:       nextAuditID(),
		Command:  CmdTransactionTrace,
		Time:     time.Now().Format(time.DateTime),
		ModuleId: GetAppName(),
		Data:     data,
	})
}
