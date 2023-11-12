package qoinhubpackageprivate

import (
	"log"
	"time"
)

// add audittrail process
//
func LogAudittrailProcess(funcName, desc, info string, key *[]string) {

	log.Println("[INFO] : ", info)

	go func() {

		dataAudittrail := AuditTrialProcess{
			Subject:     *AppName,
			Function:    funcName,
			Description: desc,
			Key:         *key,
			Data: DataAudittrailProcess{
				Time: time.Now().Format(TIME_FORMAT),
				Info: info,
			},
		}

		messagePayload := MessagePayloadAudit{
			Id:       int(time.Now().UnixNano() / 100000000),
			Command:  AUDITTRAIL_PROCESS,
			Time:     time.Now().Format(TIME_FORMAT),
			ModuleId: *AppName,
			Data:     dataAudittrail,
		}

		PushMessage(messagePayload)
	}()
}

// add audittrail data
func LogAudittrailData(funcName, desc, source, commType string, key *[]string, data *RequestAndResponse) {

	log.Println("add new audittrail data")

	go func() {
		// set data audittrial
		dataAudittrail := &AuditTrialData{
			Subject:           *AppName,
			Function:          funcName,
			Description:       desc,
			Key:               *key,
			Source:            source,
			CommunicationType: commType,
			Data:              data,
		}

		auditPayload := MessagePayloadAudit{
			Id:       int(time.Now().UnixNano() / 10000000),
			Command:  AUDITTRAIL_DATA,
			Time:     time.Now().Format(TIME_FORMAT),
			ModuleId: *AppName,
			Data:     dataAudittrail,
		}

		PushMessage(auditPayload)
	}()

}
