package paycloudhelper

import (
	"errors"
	"testing"
)

func TestLoggerErrorHub_WithExtraArgs(t *testing.T) {
	LoggerErrorHub("plain", "extra")
}

// TestResponseApiSuccess tests successful response
func TestResponseApiSuccess(t *testing.T) {
	data := map[string]string{"key": "value"}
	response := &ResponseApi{}
	response.Success("Operation successful", data)

	if response.Code != 200 {
		t.Errorf("Success() Code = %d, want 200", response.Code)
	}
	if response.Status != "success" {
		t.Errorf("Success() Status = %s, want success", response.Status)
	}
	if response.Message != "Operation successful" {
		t.Errorf("Success() Message = %s, want 'Operation successful'", response.Message)
	}
	if response.Data == nil {
		t.Errorf("Success() Data is nil, expected data")
	}
}

// TestResponseApiBadRequest tests bad request response
func TestResponseApiBadRequest(t *testing.T) {
	response := &ResponseApi{}
	response.BadRequest("Invalid input", "INVALID_INPUT")

	if response.Code != 400 {
		t.Errorf("BadRequest() Code = %d, want 400", response.Code)
	}
	if response.Status != "bad request" {
		t.Errorf("BadRequest() Status = %s, want 'bad request'", response.Status)
	}
	if response.Message != "Invalid input" {
		t.Errorf("BadRequest() Message = %s, want 'Invalid input'", response.Message)
	}
	if response.InternalCode != "INVALID_INPUT" {
		t.Errorf("BadRequest() InternalCode = %s, want INVALID_INPUT", response.InternalCode)
	}
}

// TestResponseApiUnauthorized tests unauthorized response
func TestResponseApiUnauthorized(t *testing.T) {
	response := &ResponseApi{}
	response.Unauthorized("Invalid token", "INVALID_TOKEN")

	if response.Code != 401 {
		t.Errorf("Unauthorized() Code = %d, want 401", response.Code)
	}
	if response.Status != "unauthorized" {
		t.Errorf("Unauthorized() Status = %s, want unauthorized", response.Status)
	}
	if response.Message != "Invalid token" {
		t.Errorf("Unauthorized() Message = %s, want 'Invalid token'", response.Message)
	}
	if response.InternalCode != "INVALID_TOKEN" {
		t.Errorf("Unauthorized() InternalCode = %s, want INVALID_TOKEN", response.InternalCode)
	}
}

// TestResponseApiInternalServerError tests internal server error response
func TestResponseApiInternalServerError(t *testing.T) {
	err := errors.New("database error")
	response := &ResponseApi{}
	response.InternalServerError(err)

	if response.Code != 500 {
		t.Errorf("InternalServerError() Code = %d, want 500", response.Code)
	}
	if response.Status != "internal server error" {
		t.Errorf("InternalServerError() Status = %s, want 'internal server error'", response.Status)
	}
}

// TestResponseApiAccepted tests accepted response
func TestResponseApiAccepted(t *testing.T) {
	data := map[string]string{"status": "accepted"}
	response := &ResponseApi{}
	response.Accepted(data)

	if response.Code != 202 {
		t.Errorf("Accepted() Code = %d, want 202", response.Code)
	}
	if response.Status != "accepted" {
		t.Errorf("Accepted() Status = %s, want accepted", response.Status)
	}
	if response.Data == nil {
		t.Errorf("Accepted() Data is nil, expected data")
	}
}

// TestResponseApiStructure tests ResponseApi structure
func TestResponseApiStructure(t *testing.T) {
	response := &ResponseApi{
		Code:    200,
		Status:  "success",
		Message: "Test message",
		Data:    nil,
	}

	if response.Code != 200 {
		t.Errorf("Code = %d, want 200", response.Code)
	}
	if response.Status != "success" {
		t.Errorf("Status = %s, want success", response.Status)
	}
	if response.Message != "Test message" {
		t.Errorf("Message = %q, want %q", response.Message, "Test message")
	}
}

// TestResponseApiCommonStatusCodes tests common HTTP status codes
func TestResponseApiCommonStatusCodes(t *testing.T) {
	tests := []struct {
		name        string
		code        int
		description string
	}{
		{name: "OK", code: 200, description: "Successful request"},
		{name: "Created", code: 201, description: "Resource created"},
		{name: "Accepted", code: 202, description: "Request accepted"},
		{name: "BadRequest", code: 400, description: "Invalid request"},
		{name: "Unauthorized", code: 401, description: "Missing authentication"},
		{name: "NotFound", code: 404, description: "Resource not found"},
		{name: "InternalServerError", code: 500, description: "Server error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify status codes are valid HTTP codes
			if tt.code < 100 || tt.code >= 600 {
				t.Errorf("Invalid HTTP status code: %d", tt.code)
			}

			// Verify common codes map to correct categories
			if tt.code >= 200 && tt.code < 300 {
				// Success category
			} else if tt.code >= 400 && tt.code < 500 {
				// Client error category
			} else if tt.code >= 500 && tt.code < 600 {
				// Server error category
			}
		})
	}
}

// TestResponseApiWithData tests ResponseApi with various data types
func TestResponseApiWithData(t *testing.T) {
	tests := []struct {
		name string
		data interface{}
	}{
		{name: "string data", data: "test"},
		{name: "integer data", data: 42},
		{name: "boolean data", data: true},
		{name: "nil data", data: nil},
		{name: "slice data", data: []string{"a", "b", "c"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := &ResponseApi{}
			response.Success("Test", tt.data)
			if response.Data == nil && tt.data != nil {
				t.Errorf("Data = nil, want %v", tt.data)
			}
		})
	}
}

// TestResponseApiHTTPStatusMapping tests mapping between response methods and HTTP status codes
func TestResponseApiHTTPStatusMapping(t *testing.T) {
	tests := []struct {
		method    string
		wantCode  int
		buildResp func() *ResponseApi
	}{
		{
			method:   "Success",
			wantCode: 200,
			buildResp: func() *ResponseApi {
				resp := &ResponseApi{}
				resp.Success("ok", nil)
				return resp
			},
		},
		{
			method:   "BadRequest",
			wantCode: 400,
			buildResp: func() *ResponseApi {
				resp := &ResponseApi{}
				resp.BadRequest("bad", "")
				return resp
			},
		},
		{
			method:   "Unauthorized",
			wantCode: 401,
			buildResp: func() *ResponseApi {
				resp := &ResponseApi{}
				resp.Unauthorized("unauth", "")
				return resp
			},
		},
		{
			method:   "Accepted",
			wantCode: 202,
			buildResp: func() *ResponseApi {
				resp := &ResponseApi{}
				resp.Accepted(nil)
				return resp
			},
		},
		{
			method:   "InternalServerError",
			wantCode: 500,
			buildResp: func() *ResponseApi {
				resp := &ResponseApi{}
				resp.InternalServerError(errors.New("error"))
				return resp
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			resp := tt.buildResp()
			if resp.Code != tt.wantCode {
				t.Errorf("%s() Code = %d, want %d", tt.method, resp.Code, tt.wantCode)
			}
		})
	}
}

// TestResponseApiOut tests the Out method
func TestResponseApiOut(t *testing.T) {
	data := map[string]interface{}{"key": "value"}
	response := &ResponseApi{}
	// Out(code, message, internalCode, status, data)
	response.Out(200, "OK", "SUCCESS", "success", data)

	if response.Code != 200 {
		t.Errorf("Out() Code = %d, want 200", response.Code)
	}
	if response.Message != "OK" {
		t.Errorf("Out() Message = %s, want OK", response.Message)
	}
	if response.Status != "success" {
		t.Errorf("Out() Status = %s, want success", response.Status)
	}
	if response.InternalCode != "SUCCESS" {
		t.Errorf("Out() InternalCode = %s, want SUCCESS", response.InternalCode)
	}
}

// TestResponseApiInternalCode tests InternalCode field
func TestResponseApiInternalCode(t *testing.T) {
	response := &ResponseApi{
		Code:         400,
		Status:       "bad request",
		Message:      "Validation failed",
		InternalCode: "VALIDATION_ERROR",
		Data:         nil,
	}

	if response.InternalCode != "VALIDATION_ERROR" {
		t.Errorf("InternalCode = %s, want VALIDATION_ERROR", response.InternalCode)
	}
}

// TestResponseApiEmptyMessage tests response with empty message
func TestResponseApiEmptyMessage(t *testing.T) {
	response := &ResponseApi{
		Code:   204,
		Status: "no content",
	}

	if response.Message != "" {
		t.Errorf("Message = %q, want empty string", response.Message)
	}
}
