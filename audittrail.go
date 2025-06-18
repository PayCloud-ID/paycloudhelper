package paycloudhelper

import (
	"time"

	"bitbucket.org/paycloudid/paycloudhelper/phhelper"
)

var (
	auditTrailMqClient *AmqpClient
)

// SetUpRabbitMq service must call this func in main function
// NOTE : for audittrail purpose
func SetUpRabbitMq(host, port, vhost, username, password, auditTrailQue, appName string) *AmqpClient {
	// set connection to rabbit mq
	urlStr := host + ":" + port + "/" + vhost
	uriConnection := "amqp://" + username + ":" + password + "@" + urlStr
	if phhelper.GetAppName() == "" {
		phhelper.SetAppName(appName)
	}

	LogI("[AMQP] Init audit trail rabbitmq url=%s queue=%s", urlStr, auditTrailQue)
	auditTrailMqClient = NewAmqpClient(auditTrailQue, "audittrail-"+GetAppName(), uriConnection, nil)

	return auditTrailMqClient
}

// LogAudittrailProcess add audittrail process
func LogAudittrailProcess(funcName, desc, info string, key *[]string) {
	if funcName == "" || desc == "" {
		return
	}

	LogD("[AMQP] LogAuditTrailProcess func=%s desc=%s info=%s", funcName, desc, info)

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
			Id:       int(time.Now().UnixNano() / 100000000),
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

	LogD("[AMQP] LogAuditTrailData func=%s desc=%s source=%s type=%s", funcName, desc, source, commType)

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
			Id:       int(time.Now().UnixNano() / 10000000),
			Command:  CmdAuditTrailData,
			Time:     time.Now().Format(time.DateTime),
			ModuleId: GetAppName(),
			Data:     dataAudittrail,
		}

		pushMessageAudit(auditPayload)
	}()
}

// pushMessageAudit push message to audit trail queue
func pushMessageAudit(data interface{}) {
	if auditTrailMqClient == nil || auditTrailMqClient.queueName == "" {
		LogE("[AMQP] ERR client/queue does not exists")
		// TODO : send sentry error
		return
	}

	auditTrailQueueName := auditTrailMqClient.queueName

	msgBytes, err := jsonMarshalNoEsc(data)
	if err != nil {
		LogE("[AMQP] ERR convert data to byte. err=%v", err)
		// TODO : send sentry error
		return
	}

	err = auditTrailMqClient.UnsafePush(msgBytes)
	if err != nil {
		// TODO : send sentry error
		LogE("[AMQP] ERR publish message to queue queue=%s err=%v", auditTrailQueueName, err)
		return
	}

	LogD("[AMQP] Publish message async to queue successfully queue=%s conn=%s", auditTrailQueueName, auditTrailMqClient.connName)
}
