/*
Configuration validation for paycloudhelper library
Validates runtime configuration and provides structured warnings
*/

package paycloudhelper

import (
	"os"
)

// ConfigError represents a configuration validation error
type ConfigError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Level   string `json:"level"` // "warning" or "error"
}

// ValidateConfiguration validates the runtime configuration
// Returns a slice of ConfigError for any issues found
func ValidateConfiguration() []ConfigError {
	errors := make([]ConfigError, 0)

	// Validate APP_NAME (env is authoritative for this warning so Clearenv reflects missing config
	// without racing in-memory SetAppName used elsewhere in tests).
	if os.Getenv("APP_NAME") == "" {
		errors = append(errors, ConfigError{
			Field:   "APP_NAME",
			Message: "APP_NAME environment variable not set - using empty default",
			Level:   "warning",
		})
	}

	// Validate APP_ENV
	appEnv := os.Getenv("APP_ENV")
	if appEnv == "" {
		appEnv = GetAppEnv()
	}
	if appEnv == "" {
		errors = append(errors, ConfigError{
			Field:   "APP_ENV",
			Message: "APP_ENV environment variable not set - using empty default",
			Level:   "warning",
		})
	} else {
		// Validate APP_ENV values
		validEnvs := map[string]bool{
			"develop":    true,
			"staging":    true,
			"production": true,
		}
		if !validEnvs[appEnv] {
			errors = append(errors, ConfigError{
				Field:   "APP_ENV",
				Message: "APP_ENV has unexpected value '" + appEnv + "' (expected: develop, staging, production)",
				Level:   "warning",
			})
		}
	}

	// Validate Redis configuration (if initialized)
	if redisOptions != nil {
		if redisOptions.Addr == "" {
			errors = append(errors, ConfigError{
				Field:   "Redis.Addr",
				Message: "Redis address not configured",
				Level:   "error",
			})
		}

		if redisOptions.Password == "" && os.Getenv("REDIS_PASSWORD") == "" {
			errors = append(errors, ConfigError{
				Field:   "Redis.Password",
				Message: "Redis password not set - may fail with protected Redis instances",
				Level:   "warning",
			})
		}
	}

	// Validate Sentry configuration
	sentryDSN := os.Getenv("SENTRY_DSN")
	if sentryDSN == "" {
		errors = append(errors, ConfigError{
			Field:   "SENTRY_DSN",
			Message: "SENTRY_DSN not set - error tracking disabled",
			Level:   "warning",
		})
	}

	// Validate RabbitMQ configuration (for audit trail)
	rabbitMQURL := os.Getenv("RABBITMQ_URL")
	if rabbitMQURL == "" {
		errors = append(errors, ConfigError{
			Field:   "RABBITMQ_URL",
			Message: "RABBITMQ_URL not set - audit trail may not work",
			Level:   "warning",
		})
	}

	return errors
}

// LogConfigurationWarnings logs all configuration validation warnings
// This is a convenience function to log validation results at startup
func LogConfigurationWarnings() {
	errors := ValidateConfiguration()
	if len(errors) == 0 {
		LogI("%s configuration validation passed", buildLogPrefix("LogConfigurationWarnings"))
		return
	}

	LogW("%s configuration validation found issues count=%d", buildLogPrefix("LogConfigurationWarnings"), len(errors))
	for _, err := range errors {
		switch err.Level {
		case "error":
			LogE("%s error field=%s message=%s", buildLogPrefix("LogConfigurationWarnings"), err.Field, err.Message)
		case "warning":
			LogW("%s warning field=%s message=%s", buildLogPrefix("LogConfigurationWarnings"), err.Field, err.Message)
		}
	}
}

// GetConfigurationStatus returns a summary of configuration validation
// Useful for health check endpoints
func GetConfigurationStatus() map[string]interface{} {
	errors := ValidateConfiguration()

	errorCount := 0
	warningCount := 0
	for _, err := range errors {
		if err.Level == "error" {
			errorCount++
		} else if err.Level == "warning" {
			warningCount++
		}
	}

	status := "healthy"
	if errorCount > 0 {
		status = "unhealthy"
	} else if warningCount > 0 {
		status = "degraded"
	}

	return map[string]interface{}{
		"status":   status,
		"errors":   errorCount,
		"warnings": warningCount,
		"issues":   errors,
	}
}
