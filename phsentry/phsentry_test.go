package phsentry

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSentryEnabled_ReturnsFalseWhenNotInitialized(t *testing.T) {
	sentryClient = nil
	if SentryEnabled() {
		t.Fatal("SentryEnabled() should return false when client is nil")
	}
}

func TestGetSentryDataMap_ReturnsNilWhenNoData(t *testing.T) {
	sentryBreadcrumbData = nil
	m := GetSentryDataMap()
	if m != nil {
		t.Fatalf("expected nil map when data not set, got %v", m)
	}
}

func TestNewSentryData_HandlesNilInput(t *testing.T) {
	NewSentryData(nil)
}

func TestFlushSentry_NilClientNoPanic(t *testing.T) {
	sentryClient = nil
	FlushSentry(1 * time.Second)
}

func TestReceiveLog_NilClientNoPanic(t *testing.T) {
	sentryClient = nil
	ReceiveLog("fatal", "test fatal message")
}

func TestSendSentryErrorWithContext_NilClientNoPanic(t *testing.T) {
	sentryClient = nil
	err := errors.New("test error")
	SendSentryErrorWithContext(context.Background(), err)
}

func TestSendSentryErrorWithContext_NilErrorNoPanic(t *testing.T) {
	SendSentryErrorWithContext(context.Background(), nil)
}

func TestSentryLevelFor_MapsCorrectly(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"fatal", "fatal"},
		{"error", "error"},
		{"warn", "warning"},
		{"info", "info"},
		{"debug", "debug"},
		{"unknown", "debug"},
	}

	for _, tt := range tests {
		got := sentryLevelFor(tt.input)
		if string(got) != tt.expected {
			t.Errorf("sentryLevelFor(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
