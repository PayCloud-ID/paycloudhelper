package paycloudhelper

import (
	"time"

	"bitbucket.org/paycloudid/paycloudhelper/phhelper"
)

// auditPublisher is the package-level worker pool publisher.
// Set by SetUpAuditTrailPublisher(); checked by V2 functions.
var auditPublisher *AuditPublisher

// SetUpAuditTrailPublisher creates an AmqpClient and AuditPublisher with a
// production-grade worker pool. Returns the publisher for lifecycle management.
// Existing SetUpRabbitMq is unchanged — services can migrate at their own pace.
func SetUpAuditTrailPublisher(host, port, vhost, username, password, queue, appName string, opts ...AuditPublisherOption) *AuditPublisher {
	urlStr := host + ":" + port + "/" + vhost
	uriConnection := "amqp://" + username + ":" + password + "@" + urlStr
	if phhelper.GetAppName() == "" {
		phhelper.SetAppName(appName)
	}

	LogI("[SetUpAuditTrailPublisher] url=%s queue=%s", urlStr, queue)
	client := NewAmqpClient(queue, "audittrail-v2-"+GetAppName(), uriConnection, nil)

	publisher := NewAuditPublisher(client, opts...)
	publisher.Start()
	auditPublisher = publisher
	return publisher
}

// GetAuditPublisher returns the package-level AuditPublisher, or nil if not initialized.
func GetAuditPublisher() *AuditPublisher {
	return auditPublisher
}

// LogAudittrailProcessV2 publishes an audit trail process event via the worker pool.
// Falls back to the legacy LogAudittrailProcess if the publisher is not set up.
func LogAudittrailProcessV2(funcName, desc, info string, key *[]string) {
	if auditPublisher == nil {
		LogAudittrailProcess(funcName, desc, info, key)
		return
	}

	if funcName == "" || desc == "" {
		return
	}

	LogI("[LogAudittrailProcessV2] func=%s desc=%s info=%s", funcName, desc, info)

	dataAudittrail := AuditTrailProcess{
		Subject:     GetAppName(),
		Function:    funcName,
		Description: desc,
		Data: DataAuditTrailProcess{
			Time: time.Now().Format(time.DateTime),
			Info: info,
		},
	}
	if key != nil {
		dataAudittrail.Key = *key
	}

	payload := MessagePayloadAudit{
		Id:       nextAuditID(),
		Command:  CmdAuditTrailProcess,
		Time:     time.Now().Format(time.DateTime),
		ModuleId: GetAppName(),
		Data:     dataAudittrail,
	}

	auditPublisher.Submit(payload)
}

// LogAudittrailDataV2 publishes an audit trail data event via the worker pool.
// Falls back to the legacy LogAudittrailData if the publisher is not set up.
func LogAudittrailDataV2(funcName, desc, source, commType string, key *[]string, data *RequestAndResponse) {
	if auditPublisher == nil {
		LogAudittrailData(funcName, desc, source, commType, key, data)
		return
	}

	if funcName == "" || data == nil || data.Response.Detail.StatusCode == 0 {
		return
	}

	LogI("[LogAudittrailDataV2] func=%s desc=%s source=%s type=%s", funcName, desc, source, commType)

	dataAudittrail := &AuditTrailData{
		Subject:           GetAppName(),
		Function:          funcName,
		Description:       desc,
		Source:            source,
		CommunicationType: commType,
		Data:              data,
	}
	if key != nil {
		dataAudittrail.Key = *key
	}

	payload := MessagePayloadAudit{
		Id:       nextAuditID(),
		Command:  CmdAuditTrailData,
		Time:     time.Now().Format(time.DateTime),
		ModuleId: GetAppName(),
		Data:     dataAudittrail,
	}

	auditPublisher.Submit(payload)
}
