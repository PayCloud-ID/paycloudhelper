package paycloudhelper

import (
	"sync/atomic"
	"time"

	"bitbucket.org/paycloudid/paycloudhelper/phhelper"
)

var (
	auditTrxPublisher *AuditPublisher
	auditTrxEnabled   atomic.Bool
)

// SetUpAuditTrailTrxPublisher initializes dedicated transaction-audit publishing.
func SetUpAuditTrailTrxPublisher(
	enabled bool,
	host, port, vhost, username, password, queue, appName string,
	opts ...AuditPublisherOption,
) *AuditPublisher {
	auditTrxEnabled.Store(enabled)

	if !enabled {
		LogI("[SetUpAuditTrailTrxPublisher] disabled by configuration")
		auditTrxPublisher = nil
		return nil
	}

	urlStr := host + ":" + port + "/" + vhost
	uriConnection := "amqp://" + username + ":" + password + "@" + urlStr
	if phhelper.GetAppName() == "" {
		phhelper.SetAppName(appName)
	}

	LogI("[SetUpAuditTrailTrxPublisher] url=%s queue=%s appName=%s", urlStr, queue, appName)

	client := NewAmqpClient(queue, "audittrail-trx-"+GetAppName(), uriConnection, nil)
	publisher := NewAuditPublisher(client, opts...)
	publisher.Start()

	auditTrxPublisher = publisher
	return publisher
}

func GetAuditTrailTrxPublisher() *AuditPublisher {
	return auditTrxPublisher
}

func IsAuditTrailTrxEnabled() bool {
	return auditTrxEnabled.Load() && auditTrxPublisher != nil
}

// LogAuditTrailTrx publishes one transaction audit event to the dedicated queue.
func LogAuditTrailTrx(data AuditTrailTrx) {
	if !auditTrxEnabled.Load() || auditTrxPublisher == nil {
		return
	}

	if data.ReffNo == "" && data.OrderNo == "" {
		LogW("[LogAuditTrailTrx] skipped: both reffNo and orderNo empty, function=%s state=%s", data.Function, data.State)
		return
	}

	if data.Service == "" {
		data.Service = GetAppName()
	}
	if data.EventTime == "" {
		data.EventTime = time.Now().Format(time.RFC3339Nano)
	}

	payload := MessagePayloadAudit{
		Id:       nextAuditID(),
		Command:  CmdAuditTrailTrx,
		Time:     time.Now().Format(time.DateTime),
		ModuleId: GetAppName(),
		Data:     data,
	}

	auditTrxPublisher.Submit(payload)
}
