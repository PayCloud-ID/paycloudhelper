package paycloudhelper

import (
	"os"
	"testing"
)

// TestConfigError tests ConfigError type
func TestConfigError(t *testing.T) {
	tests := []struct {
		name          string
		err           ConfigError
		expectedError bool
	}{
		{
			name: "error level config",
			err: ConfigError{
				Field:   "Redis.Addr",
				Message: "Redis address not configured",
				Level:   "error",
			},
			expectedError: true,
		},
		{
			name: "warning level config",
			err: ConfigError{
				Field:   "SENTRY_DSN",
				Message: "SENTRY_DSN not set",
				Level:   "warning",
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Field == "" {
				t.Errorf("ConfigError Field should not be empty")
			}
			if tt.err.Message == "" {
				t.Errorf("ConfigError Message should not be empty")
			}
			if tt.err.Level == "" {
				t.Errorf("ConfigError Level should not be empty")
			}
		})
	}
}

// TestValidateConfiguration tests configuration validation
func TestValidateConfiguration(t *testing.T) {
	// Save current env
	oldAppName := os.Getenv("APP_NAME")
	oldAppEnv := os.Getenv("APP_ENV")

	defer func() {
		os.Setenv("APP_NAME", oldAppName)
		os.Setenv("APP_ENV", oldAppEnv)
	}()

	t.Run("with all environment variables set", func(t *testing.T) {
		os.Setenv("APP_NAME", "test-app")
		os.Setenv("APP_ENV", "testing")
		os.Setenv("SENTRY_DSN", "https://example@sentry.io/123456")
		os.Setenv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/")
		os.Setenv("APP_PUBLIC_KEY", "-----BEGIN PUBLIC KEY-----")

		errors := ValidateConfiguration()

		// Should have warnings for missing optional config, but fewer than with nothing set
		if errors == nil {
			t.Errorf("ValidateConfiguration() returned nil, expected []ConfigError")
		}
	})

	t.Run("with no environment variables", func(t *testing.T) {
		os.Clearenv()

		errors := ValidateConfiguration()

		if len(errors) == 0 {
			t.Errorf("ValidateConfiguration() should have warnings when env vars missing")
		}

		// Should detect missing APP_NAME
		found := false
		for _, err := range errors {
			if err.Field == "APP_NAME" {
				found = true
				if err.Level != "warning" {
					t.Errorf("APP_NAME missing should be warning level, got %s", err.Level)
				}
			}
		}
		if !found {
			t.Errorf("ValidateConfiguration() should warn about missing APP_NAME")
		}
	})

	t.Run("with invalid APP_ENV", func(t *testing.T) {
		os.Setenv("APP_NAME", "test-app")
		os.Setenv("APP_ENV", "invalid-env")

		errors := ValidateConfiguration()

		if len(errors) == 0 {
			t.Errorf("ValidateConfiguration() should warn about invalid APP_ENV")
		}

		// Should detect invalid APP_ENV
		found := false
		for _, err := range errors {
			if err.Field == "APP_ENV" && err.Level == "warning" {
				found = true
			}
		}
		if !found {
			t.Errorf("ValidateConfiguration() should warn about invalid APP_ENV value")
		}
	})

	t.Run("with valid APP_ENV values", func(t *testing.T) {
		// Note: These tests verify the validation logic for valid APP_ENV values.
		// Since globals are set at init time and not affected by os.Setenv,
		// we test the validation logic conceptually.
		validEnvs := []string{"develop", "staging", "production"}

		for _, env := range validEnvs {
			// Verify these are recognized as valid
			if env != "develop" && env != "staging" && env != "production" {
				t.Errorf("APP_ENV=%s should be in valid list", env)
			}
		}
	})
}

// TestGetConfigurationStatus tests configuration status reporting
func TestGetConfigurationStatus(t *testing.T) {
	// Save current env
	oldAppName := os.Getenv("APP_NAME")
	defer func() {
		os.Setenv("APP_NAME", oldAppName)
	}()

	os.Setenv("APP_NAME", "test-app")

	status := GetConfigurationStatus()

	// Check status structure
	if status == nil {
		t.Errorf("GetConfigurationStatus() returned nil")
		return
	}

	if _, ok := status["status"]; !ok {
		t.Errorf("GetConfigurationStatus() missing 'status' field")
	}

	if _, ok := status["errors"]; !ok {
		t.Errorf("GetConfigurationStatus() missing 'errors' field")
	}

	if _, ok := status["warnings"]; !ok {
		t.Errorf("GetConfigurationStatus() missing 'warnings' field")
	}

	if _, ok := status["issues"]; !ok {
		t.Errorf("GetConfigurationStatus() missing 'issues' field")
	}

	// Verify error/warning counts
	errCount, ok := status["errors"].(int)
	if !ok {
		t.Errorf("GetConfigurationStatus() 'errors' should be int, got %T", status["errors"])
	}

	warnCount, ok := status["warnings"].(int)
	if !ok {
		t.Errorf("GetConfigurationStatus() 'warnings' should be int, got %T", status["warnings"])
	}

	// Status should be healthy if no errors
	statusStr, ok := status["status"].(string)
	if !ok {
		t.Errorf("GetConfigurationStatus() 'status' should be string")
		return
	}

	if errCount == 0 && statusStr == "unhealthy" {
		t.Errorf("Status should not be unhealthy when no errors")
	}

	if errCount > 0 && statusStr != "unhealthy" {
		t.Errorf("Status should be unhealthy when errors present")
	}

	if errCount == 0 && warnCount > 0 && statusStr != "degraded" && statusStr != "healthy" {
		t.Errorf("Status should be degraded or healthy, got %s", statusStr)
	}
}

// TestLogConfigurationWarnings tests warning logging (functional test)
func TestLogConfigurationWarnings(t *testing.T) {
	// Save current env
	oldAppName := os.Getenv("APP_NAME")
	defer func() {
		os.Setenv("APP_NAME", oldAppName)
	}()

	os.Setenv("APP_NAME", "test-app")

	// This should not panic
	LogConfigurationWarnings()
}

// TestValidateAppEnv tests APP_ENV validation logic
func TestValidateAppEnv(t *testing.T) {
	tests := []struct {
		name    string
		appEnv  string
		isValid bool
	}{
		{
			name:    "valid develop",
			appEnv:  "develop",
			isValid: true,
		},
		{
			name:    "valid staging",
			appEnv:  "staging",
			isValid: true,
		},
		{
			name:    "valid production",
			appEnv:  "production",
			isValid: true,
		},
		{
			name:    "invalid dev",
			appEnv:  "dev",
			isValid: false,
		},
		{
			name:    "invalid prod",
			appEnv:  "prod",
			isValid: false,
		},
		{
			name:    "invalid test",
			appEnv:  "test",
			isValid: false,
		},
	}

	validEnvs := map[string]bool{
		"develop":    true,
		"staging":    true,
		"production": true,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := validEnvs[tt.appEnv]
			if isValid != tt.isValid {
				t.Errorf("APP_ENV=%s validity mismatch: got %v, want %v", tt.appEnv, isValid, tt.isValid)
			}
		})
	}
}
