package paycloudhelper

import (
	"bytes"
	"encoding/json"
	"sync"
)

var (
	auditJSONMu      sync.RWMutex
	auditJSONMarshal func(interface{}) ([]byte, error) = defaultAuditJSONMarshalNoEsc
)

// defaultAuditJSONMarshalNoEsc matches the historical helpers.jsonMarshalNoEsc behavior:
// encoding/json Encoder with HTML escaping disabled. Encoder.Encode appends a trailing newline.
func defaultAuditJSONMarshalNoEsc(v interface{}) ([]byte, error) {
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(v)
	return buffer.Bytes(), err
}

// jsonMarshalNoEsc serializes values for audit trail message bodies (V1 pushMessageAudit and
// V2 AuditPublisher workers). It delegates to ConfigureAuditJSONMarshal when set.
func jsonMarshalNoEsc(v interface{}) ([]byte, error) {
	auditJSONMu.RLock()
	fn := auditJSONMarshal
	auditJSONMu.RUnlock()
	return fn(v)
}

// ConfigureAuditJSONMarshal sets the JSON marshaler used for audit trail payloads pushed to
// RabbitMQ (pushMessageAudit, AuditPublisher.processMessage). Passing nil restores the default
// (encoding/json Encoder with SetEscapeHTML(false), trailing newline from Encode).
//
// Call once during service startup. Typical sonic opt-in:
//
//	phjson.ConfigureForAuditTrail()
//	pchelper.ConfigureAuditJSONMarshal(phjson.Marshal)
//
// Opt-in marshalers may differ slightly from the default (e.g. no trailing newline); downstream
// consumers should parse JSON tolerantly.
func ConfigureAuditJSONMarshal(fn func(interface{}) ([]byte, error)) {
	auditJSONMu.Lock()
	defer auditJSONMu.Unlock()
	if fn == nil {
		auditJSONMarshal = defaultAuditJSONMarshalNoEsc
		return
	}
	auditJSONMarshal = fn
}
