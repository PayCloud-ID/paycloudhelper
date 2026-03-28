package paycloudhelper

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"bitbucket.org/paycloudid/paycloudhelper/phhelper"
)

var (
	auditTrailMqClient *AmqpClient

	// auditIDCounter provides unique, collision-free IDs for audit messages.
	auditIDCounter atomic.Int64

	// Rate-limited logging for pushMessageAudit when client is not ready.
	// Prevents log flooding under sustained RabbitMQ failure.
	auditNotReadyLastLog   time.Time
	auditNotReadyLogMu     sync.Mutex
	auditNotReadyLogWindow = 30 * time.Second
)

// nextAuditID returns a unique, monotonically increasing ID for audit messages.
func nextAuditID() int {
	return int(auditIDCounter.Add(1))
}

// SetUpRabbitMq service must call this func in main function
// NOTE : for audittrail purpose
func SetUpRabbitMq(host, port, vhost, username, password, auditTrailQue, appName string) *AmqpClient {
	// set connection to rabbit mq
	urlStr := host + ":" + port + "/" + vhost
	uriConnection := "amqp://" + username + ":" + password + "@" + urlStr
	if phhelper.GetAppName() == "" {
		phhelper.SetAppName(appName)
	}

	LogI("%s init audittrail rabbitmq url=%s queue=%s", buildLogPrefix("SetUpRabbitMq"), urlStr, auditTrailQue)
	auditTrailMqClient = NewAmqpClient(auditTrailQue, "audittrail-"+GetAppName(), uriConnection, nil)

	return auditTrailMqClient
}

// LogAudittrailProcess add audittrail process
func LogAudittrailProcess(funcName, desc, info string, key *[]string) {
	if funcName == "" || desc == "" {
		return
	}

	LogI("%s func=%s desc=%s info=%s", buildLogPrefix("LogAudittrailProcess"), funcName, desc, info)

	go func() {
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

		messagePayload := MessagePayloadAudit{
			Id:       nextAuditID(),
			Command:  CmdAuditTrailProcess,
			Time:     time.Now().Format(time.DateTime),
			ModuleId: GetAppName(),
			Data:     dataAudittrail,
		}

		pushMessageAudit(messagePayload)
	}()
}

// LogAudittrailData add audittrail data
func LogAudittrailData(funcName, desc, source, commType string, key *[]string, data *RequestAndResponse) {
	if funcName == "" || data == nil || data.Response.Detail.StatusCode == 0 {
		return
	}

	LogI("%s func=%s desc=%s source=%s type=%s", buildLogPrefix("LogAudittrailData"), funcName, desc, source, commType)

	go func() {
		//set data audit trail
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

		auditPayload := MessagePayloadAudit{
			Id:       nextAuditID(),
			Command:  CmdAuditTrailData,
			Time:     time.Now().Format(time.DateTime),
			ModuleId: GetAppName(),
			Data:     dataAudittrail,
		}

		pushMessageAudit(auditPayload)
	}()
}

// logAuditErrorWithSentry logs error and optionally sends to Sentry if initialized
// This is a backward-compatible helper that safely checks for Sentry client
func logAuditErrorWithSentry(msg string, err error) {
	LogE(msg)

	// Only send to Sentry if client is initialized (backward compatible)
	if GetSentryClient() != nil {
		SendSentryError(err, GetAppName(), "audittrail", "pushMessageAudit")
	}
}

// pushMessageAudit push message to audit trail queue
func pushMessageAudit(data interface{}) {
	// Early exit: no client or client not ready (RISK-005)
	if auditTrailMqClient == nil {
		logAuditNotReadyRateLimited("client is nil")
		return
	}
	if !auditTrailMqClient.IsReady() {
		logAuditNotReadyRateLimited("client not ready")
		return
	}
	if auditTrailMqClient.queueName == "" {
		err := fmt.Errorf("%s queue name is empty", buildLogPrefix("pushMessageAudit"))
		logAuditErrorWithSentry(fmt.Sprintf("%s queue name is empty", buildLogPrefix("pushMessageAudit")), err)
		return
	}

	auditTrailQueueName := auditTrailMqClient.queueName

	msgBytes, err := jsonMarshalNoEsc(data)
	if err != nil {
		logAuditErrorWithSentry(fmt.Sprintf("%s convert data to bytes failed err=%v", buildLogPrefix("pushMessageAudit"), err), err)
		return
	}

	err = auditTrailMqClient.Push(msgBytes)
	if err != nil {
		logAuditErrorWithSentry(
			fmt.Sprintf("%s publish message failed queue=%s err=%v", buildLogPrefix("pushMessageAudit"), auditTrailQueueName, err),
			err,
		)
		return
	}

	LogI("%s publish message async success queue=%s conn=%s", buildLogPrefix("pushMessageAudit"), auditTrailQueueName, auditTrailMqClient.connName)
}

// logAuditNotReadyRateLimited logs audit client-not-ready errors at most once per auditNotReadyLogWindow
// to prevent log flooding under sustained RabbitMQ failure.
func logAuditNotReadyRateLimited(reason string) {
	auditNotReadyLogMu.Lock()
	defer auditNotReadyLogMu.Unlock()
	now := time.Now()
	if now.Sub(auditNotReadyLastLog) < auditNotReadyLogWindow {
		return
	}
	auditNotReadyLastLog = now
	LogW("%s skipping audit publish: %s", buildLogPrefix("pushMessageAudit"), reason)
}
