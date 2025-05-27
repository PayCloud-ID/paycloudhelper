package paycloudhelper

import (
	"time"
)

// LogAudittrailProcess add audittrail process
func LogAudittrailProcess(funcName, desc, info string, key *[]string) {
	if funcName == "" || desc == "" {
		return
	}

	LogD("LogAudittrailProcess: func:%s desc:%s info:%s", funcName, desc, info)

	go func() {
		dataAudittrail := AuditTrialProcess{
			Subject:     GetAppName(),
			Function:    funcName,
			Description: desc,
			Data: DataAudittrailProcess{
				Time: time.Now().Format(TIME_FORMAT),
				Info: info,
			},
		}
		if key != nil {
			dataAudittrail.Key = *key
		}

		messagePayload := MessagePayloadAudit{
			Id:       int(time.Now().UnixNano() / 100000000),
			Command:  AUDITTRAIL_PROCESS,
			Time:     time.Now().Format(TIME_FORMAT),
			ModuleId: GetAppName(),
			Data:     dataAudittrail,
		}

		PushMessage(messagePayload)
	}()
}

// LogAudittrailData add audittrail data
func LogAudittrailData(funcName, desc, source, commType string, key *[]string, data *RequestAndResponse) {
	if funcName == "" {
		return
	}

	if data == nil {
		return
	} else if data.Response.Detail.StatusCode == 0 {
		return
	}

	LogD("[AMQP] LogAudittrailData: func:%s desc:%s source:%s type:%s", funcName, desc, source, commType)

	go func() {
		// set data audittrail
		dataAudittrail := &AuditTrialData{
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
			Command:  AUDITTRAIL_DATA,
			Time:     time.Now().Format(TIME_FORMAT),
			ModuleId: GetAppName(),
			Data:     dataAudittrail,
		}

		PushMessage(auditPayload)
	}()

}
