/*
validate header request
*/

package paycloudhelper

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/thedevsaddam/govalidator"
)

type Headers struct {
	IdempotencyKey string `json:"idem_key"`
	Session        string `json:"session"`
	Csrf           string `json:"csrf"`
	RequestID      string `json:"request_id"` // Request ID for tracing
}

// generateRequestID creates a new unique request ID
func generateRequestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based ID if random fails
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// GetOrGenerateRequestID extracts X-Request-ID header or generates a new one
func GetOrGenerateRequestID(headerValue string) string {
	if headerValue != "" {
		return headerValue
	}
	return generateRequestID()
}

func (h *Headers) ValiadateHeaderIdem() interface{} {

	validator := govalidator.New(govalidator.Options{
		Data: h,
		Rules: govalidator.MapData{
			"idem_key": []string{"required", "char_libs", "max:50"},
			"session":  []string{"numeric_null_libs", "max:60"},
		},
		RequiredDefault: true,
	}).ValidateStruct()

	if len(validator) > 0 {
		return validator
	}

	return nil
}

func (h *Headers) ValiadateHeaderCsrf() interface{} {

	validator := govalidator.New(govalidator.Options{
		Data: h,
		Rules: govalidator.MapData{
			"csrf": []string{"required", "char_libs", "max:50"},
		},
		RequiredDefault: true,
	}).ValidateStruct()

	if len(validator) > 0 {
		return validator
	}

	return nil
}
