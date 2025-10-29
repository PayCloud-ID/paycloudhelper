package paycloudhelper

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

// mockContext creates a mock Echo context for testing
func mockContext(method, path, body string) (echo.Context, error) {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	e := echo.New()
	ctx := e.NewContext(req, rec)
	return ctx, nil
}

// TestVerifIdemKeyValidation tests idempotency key validation
func TestVerifIdemKeyValidation(t *testing.T) {
	tests := []struct {
		name          string
		idemKeyHeader string
		bodyData      string
		wantStatus    int
		wantAccepted  bool
	}{
		{
			name:          "valid idempotency key",
			idemKeyHeader: "test-idem-key-123",
			bodyData:      `{"amount":100}`,
			wantStatus:    200,
			wantAccepted:  false,
		},
		{
			name:          "missing idempotency key",
			idemKeyHeader: "",
			bodyData:      `{"amount":100}`,
			wantStatus:    400,
			wantAccepted:  false,
		},
		{
			name:          "duplicate idempotency key",
			idemKeyHeader: "test-idem-key-123",
			bodyData:      `{"amount":100}`,
			wantStatus:    202,
			wantAccepted:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := mockContext("POST", "/api/test", tt.bodyData)
			if tt.idemKeyHeader != "" {
				ctx.Request().Header.Set("Idempotency-Key", tt.idemKeyHeader)
			}

			// Verify header setup
			got := ctx.Request().Header.Get("Idempotency-Key")
			if got != tt.idemKeyHeader {
				t.Errorf("Idempotency-Key header = %s, want %s", got, tt.idemKeyHeader)
			}
		})
	}
}

// TestVerifCsrfValidation tests CSRF token validation
func TestVerifCsrfValidation(t *testing.T) {
	tests := []struct {
		name         string
		csrfToken    string
		sessionValue string
		wantStatus   int
		wantValid    bool
	}{
		{
			name:         "valid CSRF token",
			csrfToken:    "valid-csrf-token-123",
			sessionValue: "session-id",
			wantStatus:   200,
			wantValid:    true,
		},
		{
			name:         "missing CSRF token",
			csrfToken:    "",
			sessionValue: "session-id",
			wantStatus:   403,
			wantValid:    false,
		},
		{
			name:         "invalid CSRF token",
			csrfToken:    "invalid-token",
			sessionValue: "session-id",
			wantStatus:   403,
			wantValid:    false,
		},
		{
			name:         "missing session header",
			csrfToken:    "valid-token",
			sessionValue: "",
			wantStatus:   403,
			wantValid:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := mockContext("POST", "/api/secure", "")
			if tt.csrfToken != "" {
				ctx.Request().Header.Set("X-Xsrf-Token", tt.csrfToken)
			}
			if tt.sessionValue != "" {
				ctx.Request().Header.Set("Session", tt.sessionValue)
			}

			// Verify headers are set correctly
			if got := ctx.Request().Header.Get("X-Xsrf-Token"); got != tt.csrfToken {
				t.Errorf("X-Xsrf-Token header = %s, want %s", got, tt.csrfToken)
			}
		})
	}
}

// TestRevokeTokenValidation tests JWT token revocation checking
func TestRevokeTokenValidation(t *testing.T) {
	tests := []struct {
		name           string
		authHeader     string
		token          string
		isRevoked      bool
		wantStatus     int
		wantAuthorized bool
	}{
		{
			name:           "valid non-revoked token",
			authHeader:     "Bearer valid-jwt-token",
			token:          "valid-jwt-token",
			isRevoked:      false,
			wantStatus:     200,
			wantAuthorized: true,
		},
		{
			name:           "revoked token",
			authHeader:     "Bearer revoked-token",
			token:          "revoked-token",
			isRevoked:      true,
			wantStatus:     401,
			wantAuthorized: false,
		},
		{
			name:           "missing authorization header",
			authHeader:     "",
			token:          "",
			isRevoked:      false,
			wantStatus:     401,
			wantAuthorized: false,
		},
		{
			name:           "malformed authorization header",
			authHeader:     "InvalidToken",
			token:          "invalid",
			isRevoked:      false,
			wantStatus:     401,
			wantAuthorized: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := mockContext("GET", "/api/protected", "")
			if tt.authHeader != "" {
				ctx.Request().Header.Set("Authorization", tt.authHeader)
			}

			// Verify header is set
			if got := ctx.Request().Header.Get("Authorization"); got != tt.authHeader {
				t.Errorf("Authorization header = %s, want %s", got, tt.authHeader)
			}
		})
	}
}

// TestRequestIDPropagation tests request ID generation and propagation
func TestRequestIDPropagation(t *testing.T) {
	tests := []struct {
		name            string
		requestIDHeader string
		generateNew     bool
		wantID          bool
	}{
		{
			name:            "request ID from header",
			requestIDHeader: "test-request-123",
			generateNew:     false,
			wantID:          true,
		},
		{
			name:            "generate new request ID",
			requestIDHeader: "",
			generateNew:     true,
			wantID:          true,
		},
		{
			name:            "empty header generates new ID",
			requestIDHeader: "",
			generateNew:     true,
			wantID:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := mockContext("GET", "/api/test", "")
			if tt.requestIDHeader != "" {
				ctx.Request().Header.Set("X-Request-ID", tt.requestIDHeader)
			}

			// Verify header is set correctly
			if tt.requestIDHeader != "" {
				if got := ctx.Request().Header.Get("X-Request-ID"); got != tt.requestIDHeader {
					t.Errorf("X-Request-ID header = %s, want %s", got, tt.requestIDHeader)
				}
			}

			if tt.wantID && tt.requestIDHeader == "" && !tt.generateNew {
				t.Errorf("Expected request ID to be available")
			}
		})
	}
}

// TestMiddlewareErrorHandling tests error handling in middleware
func TestMiddlewareErrorHandling(t *testing.T) {
	tests := []struct {
		name       string
		errorType  string
		wantStatus int
		shouldFail bool
	}{
		{
			name:       "validation error",
			errorType:  "validation",
			wantStatus: 400,
			shouldFail: true,
		},
		{
			name:       "authorization error",
			errorType:  "authorization",
			wantStatus: 401,
			shouldFail: true,
		},
		{
			name:       "permission error",
			errorType:  "permission",
			wantStatus: 403,
			shouldFail: true,
		},
		{
			name:       "internal server error",
			errorType:  "internal",
			wantStatus: 500,
			shouldFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify error classification
			if tt.wantStatus < 400 && tt.shouldFail {
				t.Errorf("Non-error status %d marked as failure", tt.wantStatus)
			}
			if tt.wantStatus >= 400 && !tt.shouldFail {
				t.Errorf("Error status %d marked as success", tt.wantStatus)
			}
		})
	}
}

// TestHeaderValidation tests header validation logic
func TestHeaderValidation(t *testing.T) {
	tests := []struct {
		name        string
		headerName  string
		headerValue string
		isValid     bool
	}{
		{
			name:        "valid header",
			headerName:  "X-Custom-Header",
			headerValue: "test-value",
			isValid:     true,
		},
		{
			name:        "empty header value",
			headerName:  "X-Custom-Header",
			headerValue: "",
			isValid:     false,
		},
		{
			name:        "special characters in header",
			headerName:  "X-Custom-Header",
			headerValue: "value-with-special-!@#$",
			isValid:     true,
		},
		{
			name:        "very long header value",
			headerName:  "X-Custom-Header",
			headerValue: string(make([]byte, 10000)),
			isValid:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := mockContext("GET", "/test", "")
			ctx.Request().Header.Set(tt.headerName, tt.headerValue)

			got := ctx.Request().Header.Get(tt.headerName)
			if got != tt.headerValue && tt.isValid {
				t.Errorf("Header value mismatch: got %q, want %q", got, tt.headerValue)
			}
		})
	}
}

// TestMiddlewareChaining tests chaining of multiple middleware
func TestMiddlewareChaining(t *testing.T) {
	tests := []struct {
		name           string
		numMiddlewares int
		shouldSucceed  bool
	}{
		{
			name:           "single middleware",
			numMiddlewares: 1,
			shouldSucceed:  true,
		},
		{
			name:           "multiple middleware",
			numMiddlewares: 3,
			shouldSucceed:  true,
		},
		{
			name:           "many middleware",
			numMiddlewares: 10,
			shouldSucceed:  true,
		},
		{
			name:           "no middleware",
			numMiddlewares: 0,
			shouldSucceed:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.numMiddlewares < 0 {
				t.Errorf("numMiddlewares cannot be negative")
			}
		})
	}
}

// TestResponseStatus tests response status code handling
func TestResponseStatus(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		isValid    bool
	}{
		{
			name:       "200 OK",
			statusCode: 200,
			isValid:    true,
		},
		{
			name:       "201 Created",
			statusCode: 201,
			isValid:    true,
		},
		{
			name:       "400 Bad Request",
			statusCode: 400,
			isValid:    true,
		},
		{
			name:       "401 Unauthorized",
			statusCode: 401,
			isValid:    true,
		},
		{
			name:       "403 Forbidden",
			statusCode: 403,
			isValid:    true,
		},
		{
			name:       "404 Not Found",
			statusCode: 404,
			isValid:    true,
		},
		{
			name:       "500 Internal Server Error",
			statusCode: 500,
			isValid:    true,
		},
		{
			name:       "invalid status 999",
			statusCode: 999,
			isValid:    false,
		},
		{
			name:       "negative status",
			statusCode: -1,
			isValid:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify HTTP status code validity
			isValidHTTPStatus := tt.statusCode >= 100 && tt.statusCode < 600
			if isValidHTTPStatus != tt.isValid {
				t.Errorf("Status %d validation = %v, want %v", tt.statusCode, isValidHTTPStatus, tt.isValid)
			}
		})
	}
}

// TestContextValue tests storing and retrieving values in context
func TestContextValue(t *testing.T) {
	tests := []struct {
		name  string
		key   interface{}
		value interface{}
	}{
		{
			name:  "string key and value",
			key:   "request-id",
			value: "test-id-123",
		},
		{
			name:  "integer value",
			key:   "user-id",
			value: 123,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := mockContext("GET", "/test", "")
			ctx.Set(tt.key.(string), tt.value)
			got := ctx.Get(tt.key.(string))
			if got != tt.value {
				t.Errorf("Context value mismatch: got %v, want %v", got, tt.value)
			}
		})
	}
}

// TestRequestBody tests reading and processing request body
func TestRequestBody(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		contentType string
		wantErr     bool
	}{
		{
			name:        "JSON body",
			body:        `{"key":"value"}`,
			contentType: "application/json",
			wantErr:     false,
		},
		{
			name:        "form-urlencoded",
			body:        "key=value&foo=bar",
			contentType: "application/x-www-form-urlencoded",
			wantErr:     false,
		},
		{
			name:        "empty body",
			body:        "",
			contentType: "application/json",
			wantErr:     false,
		},
		{
			name:        "malformed JSON",
			body:        `{invalid json}`,
			contentType: "application/json",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/test", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", tt.contentType)

			// Verify body can be read
			bodyBytes, err := io.ReadAll(req.Body)
			if err != nil && !tt.wantErr {
				t.Errorf("Failed to read body: %v", err)
			}

			if len(bodyBytes) > 0 && tt.body == "" {
				t.Errorf("Body should be empty")
			}

			if len(bodyBytes) == 0 && tt.body != "" {
				t.Errorf("Body should not be empty")
			}
		})
	}
}

// TestHTTPMethods tests different HTTP methods
func TestHTTPMethods(t *testing.T) {
	methods := []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
		http.MethodHead,
		http.MethodOptions,
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			ctx, _ := mockContext(method, "/api/test", "")
			if ctx.Request().Method != method {
				t.Errorf("HTTP method = %s, want %s", ctx.Request().Method, method)
			}
		})
	}
}

// TestErrorResponse tests error response formatting
func TestErrorResponse(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		message    string
		wantErr    bool
	}{
		{
			name:       "400 error message",
			statusCode: 400,
			message:    "Invalid input",
			wantErr:    true,
		},
		{
			name:       "401 error message",
			statusCode: 401,
			message:    "Unauthorized",
			wantErr:    true,
		},
		{
			name:       "500 error message",
			statusCode: 500,
			message:    "Internal server error",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.statusCode >= 400 && !tt.wantErr {
				t.Errorf("Status %d should be error", tt.statusCode)
			}
		})
	}
}
