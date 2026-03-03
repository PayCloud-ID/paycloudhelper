package phsentry

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNormalizeSentryDiagnosticMessage_StripsSdkPrefixAndTimestamp(t *testing.T) {
	raw := "[Sentry] 2026/03/03 14:03:41 Sending error [abc] to host project: 123"
	got := normalizeSentryDiagnosticMessage(raw)
	want := "[pchelper.Sentry] Sending error [abc] to host project: 123"
	if got != want {
		t.Fatalf("normalizeSentryDiagnosticMessage() = %q, want %q", got, want)
	}
}

func TestResolveSentryContext_UsesSafeDefaultsWhenBreadcrumbNil(t *testing.T) {
	sentryBreadcrumbData = nil
	service, module, function := resolveSentryContext()
	if service != "" {
		t.Fatalf("resolveSentryContext() service = %q, want empty string default from app name", service)
	}
	if module != "" {
		t.Fatalf("resolveSentryContext() module = %q, want empty string default from app env", module)
	}
	if function != "paycloud-be-func" {
		t.Fatalf("resolveSentryContext() function = %q, want %q", function, "paycloud-be-func")
	}
}

func TestNormalizeSentryDiagnosticMessage_EmptyInput(t *testing.T) {
	got := normalizeSentryDiagnosticMessage(" \n\t ")
	if got != "" {
		t.Fatalf("normalizeSentryDiagnosticMessage() = %q, want empty string", got)
	}
}

func TestSentryDiagnosticWriter_WriteReturnsInputLength(t *testing.T) {
	w := sentryDiagnosticWriter{}
	input := []byte("[Sentry] 2026/03/03 14:03:41 Sending error [id] to host project: 1\n")
	n, err := w.Write(input)
	if err != nil {
		t.Fatalf("Write() returned error: %v", err)
	}
	if n != len(input) {
		t.Fatalf("Write() = %d, want %d", n, len(input))
	}
}

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
