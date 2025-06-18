package paycloudhelper

const (
	CmdAuditTrailProcess = "audit-trail-process"
	CmdAuditTrailData    = "audit-trail-data"
)

type MessagePayloadAudit struct {
	Id       int         `json:"Id"`
	Command  string      `json:"Command"`
	Time     string      `json:"Time"`
	ModuleId string      `json:"ModuleId"`
	Data     interface{} `json:"Data"`
}

type AuditTrailProcess struct {
	Subject     string                `json:"Subject,omitempty"`
	Function    string                `json:"Function,omitempty"`
	Description string                `json:"Description,omitempty"`
	Key         []string              `json:"Key"`
	Data        DataAuditTrailProcess `json:"Data"`
}

type DataAuditTrailProcess struct {
	Time string `json:"Time"` // time will be handle in library
	Info string `json:"Info"` // message from service/app want to print in log
}

type AuditTrailData struct {
	Subject           string              `json:"Subject,omitempty"`
	Function          string              `json:"Function,omitempty"`
	Description       string              `json:"Description,omitempty"`
	Key               []string            `json:"Key"`    //
	Source            string              `json:"Source"` // internal or external
	CommunicationType string              `json:"CommunicationType"`
	Data              *RequestAndResponse `json:"Data"`
}

type RequestAndResponse struct {
	Request  Request       `json:"Request"`
	Response ResponseAudit `json:"Response"`
}

type Request struct {
	Time        string      `json:"Time"`
	Path        string      `json:"Path,omitempty"`
	QueryString interface{} `json:"QueryString,omitempty"`
	Header      interface{} `json:"Header,omitempty"`
	Param       interface{} `json:"Param,omitempty"`
	Body        interface{} `json:"Body,omitempty"`
	IpAddress   string      `json:"IpAddress,omitempty"`
	BrowserId   int         `json:"BrowserId,omitempty"`
	Latitude    string      `json:"Latitude,omitempty"`
	Longitude   string      `json:"Longitude,omitempty"`
}

type ResponseAudit struct {
	Time   string `json:"Time"`
	Detail Detail `json:"Detail,omitempty"`
}

type Detail struct {
	StatusCode int         `json:"StatusCode"`
	Message    string      `json:"Message"`
	Data       interface{} `json:"Data,omitempty"`
}
