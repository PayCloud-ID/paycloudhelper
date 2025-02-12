package paycloudhelper

import "fmt"

type ResponseApi struct {
	Code         int         `json:"code"`
	Status       string      `json:"status"`
	Message      string      `json:"message"`
	InternalCode string      `json:"internal_code,omitempty"`
	Data         interface{} `json:"data,omitempty"`
}

func (r *ResponseApi) Out(code int, message, internalCode string, status string, data interface{}) {
	r.Code = code
	r.InternalCode = internalCode
	r.Status = status
	r.Message = message
	r.Data = data
}

// InternalServerError is method for internal server error
func (r *ResponseApi) InternalServerError(err error) {
	LoggerErrorHub(err)
	r.Out(500, err.Error(), "", "internal server error", nil)
}

// BadRequest is method for bad request
func (r *ResponseApi) BadRequest(message string, intenalCode string) {
	LoggerErrorHub(message)
	r.Out(400, message, intenalCode, "bad request", message)
}

// Unauthorized unauthorized user
func (r *ResponseApi) Unauthorized(message string, intenalCode string) {
	LoggerErrorHub(message)
	r.Out(401, message, intenalCode, "unauthorized", nil)
}

// Accepted in process response
func (r *ResponseApi) Accepted(data interface{}) {
	r.Out(202, "your request in process", "", "accepted", data)
}

func (r *ResponseApi) Success(message string, data interface{}) {
	r.Out(200, message, "", "success", data)
}

func LoggerErrorHub(err interface{}, args ...interface{}) {
	logData := fmt.Sprintf("ERR : %v", err)
	if len(args) > 0 {
		logData += fmt.Sprintf(" %v ", args[0])
	}
	LogE(logData)
}
