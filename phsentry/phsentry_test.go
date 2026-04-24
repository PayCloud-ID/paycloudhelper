package phsentry

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"bitbucket.org/paycloudid/paycloudhelper/phlogger"
	"github.com/getsentry/sentry-go"
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

func TestInitSentryOptions_EnableLogsDefaultTrue(t *testing.T) {
	prev := sentryClientOptions
	defer func() { sentryClientOptions = prev }()

	InitSentryOptions(SentryOptions{Dsn: "https://examplePublicKey@o0.ingest.sentry.io/0"})
	if sentryClientOptions == nil {
		t.Fatal("InitSentryOptions should initialize sentryClientOptions when DSN is provided")
	}
	if !sentryClientOptions.EnableLogs {
		t.Fatal("InitSentryOptions should enable structured logs by default")
	}
}

func TestRegisterSentryLogHook_IdempotentNoPanic(t *testing.T) {
	phlogger.ClearLogHooks()
	defer phlogger.ClearLogHooks()

	defer func() { sentryLogHookOnce = sync.Once{} }()
	sentryLogHookOnce = sync.Once{}

	RegisterSentryLogHook()
	RegisterSentryLogHook()

	// Should not panic when hooks are active and no client is initialized.
	sentryClient = nil
	phlogger.LogI("[TestRegisterSentryLogHook] info")
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

func TestExtractLogPrefix_BracketPrefix(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantType string
		wantVal  string
	}{
		{
			name:     "standard prefix",
			input:    "[ReadyCheck] readiness check failed err=unhealthy",
			wantType: "ReadyCheck",
			wantVal:  "readiness check failed err=unhealthy",
		},
		{
			name:     "pchelper dotted prefix",
			input:    "[pchelper.InitRedis] failed to connect host=localhost",
			wantType: "pchelper.InitRedis",
			wantVal:  "failed to connect host=localhost",
		},
		{
			name:     "no bracket prefix falls back to Log",
			input:    "plain message without prefix",
			wantType: "Log",
			wantVal:  "plain message without prefix",
		},
		{
			name:     "empty string falls back to Log",
			input:    "",
			wantType: "Log",
			wantVal:  "",
		},
		{
			name:     "prefix only no body",
			input:    "[MyFunc]",
			wantType: "MyFunc",
			wantVal:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotVal := extractLogPrefix(tt.input)
			if gotType != tt.wantType {
				t.Errorf("extractLogPrefix() type = %q, want %q", gotType, tt.wantType)
			}
			if gotVal != tt.wantVal {
				t.Errorf("extractLogPrefix() value = %q, want %q", gotVal, tt.wantVal)
			}
		})
	}
}

func TestReceiveLog_ErrorLevelNilClientNoPanic(t *testing.T) {
	sentryClient = nil
	// Should not panic; nil client guard must work for error level too.
	ReceiveLog("error", "[ReadyCheck] readiness check failed err=unhealthy gRPC services")
}

func TestReceiveLog_InfoLevelNilClientNoPanic(t *testing.T) {
	sentryClient = nil
	ReceiveLog("info", "[InitSentry] Sentry initialized dsn=https://example.sentry.io")
}

func TestBuildSentryTitle_FormatContainsAllParts(t *testing.T) {
	prev := sentryClientOptions
	defer func() { sentryClientOptions = prev }()

	sentryClientOptions = &sentry.ClientOptions{
		Environment: "staging",
	}

	got := buildSentryTitle("main.initSentry")
	want := "[main.initSentry] [env=staging]"
	if got != want {
		t.Errorf("buildSentryTitle() = %q, want %q", got, want)
	}
}

func TestBuildSentryTitle_NilClientOptionsFallsBackToAppEnv(t *testing.T) {
	sentryClientOptions = nil
	got := buildSentryTitle("ReadyCheck")
	// Must start with the prefix bracket; env falls back to phhelper.GetAppEnv().
	if !strings.HasPrefix(got, "[ReadyCheck] [env=") {
		t.Errorf("buildSentryTitle() = %q, expected prefix \"[ReadyCheck] [env=", got)
	}
}

// captureTransport records events without network I/O.
type captureTransport struct {
	mu     sync.Mutex
	events []*sentry.Event
}

func (c *captureTransport) Flush(timeout time.Duration) bool { return true }

func (c *captureTransport) FlushWithContext(ctx context.Context) bool { return true }

func (c *captureTransport) Configure(options sentry.ClientOptions) {}

func (c *captureTransport) SendEvent(event *sentry.Event) {
	if event == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	cp := *event
	c.events = append(c.events, &cp)
}

func (c *captureTransport) Close() {}

func TestReceiveLog_WithClient_CapturesLevels(t *testing.T) {
	prevC := sentryClient
	prevO := sentryClientOptions
	prevD := sentryBreadcrumbData
	defer func() {
		sentryClient = prevC
		sentryClientOptions = prevO
		sentryBreadcrumbData = prevD
	}()

	tr := &captureTransport{}
	sentryClient = nil
	sentryClientOptions = nil
	sentryBreadcrumbData = nil

	client, err := sentry.NewClient(sentry.ClientOptions{
		Dsn:       "http://public@example.com/1",
		Transport: tr,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	sentryClient = client

	ReceiveLog("info", "[Svc] informational note")
	ReceiveLog("error", "[Svc] failure err=broken")
	ReceiveLog("fatal", "[Svc] fatal err=stop")

	tr.mu.Lock()
	n := len(tr.events)
	tr.mu.Unlock()
	if n == 0 {
		t.Fatal("expected at least one captured Sentry event")
	}
}

// TestSendToSentryCapturePaths_WithClient exercises captureWithBreadcrumb via
// public SendToSentry* APIs (in-memory transport, no network).
func TestSendToSentryCapturePaths_WithClient(t *testing.T) {
	prevC := sentryClient
	prevO := sentryClientOptions
	defer func() {
		sentryClient = prevC
		sentryClientOptions = prevO
	}()

	tr := &captureTransport{}
	client, err := sentry.NewClient(sentry.ClientOptions{
		Dsn:         "http://public@example.com/1",
		Transport:   tr,
		Environment: "test",
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	sentryClient = client
	sentryClientOptions = &sentry.ClientOptions{Dsn: "http://public@example.com/1"}

	SendToSentryMessage("hello", "svc", "mod", "fn")
	SendToSentryError(errors.New("err"), "svc", "mod", "fn")
	SendToSentryWarning(errors.New("warn"), "svc", "mod", "fn")
	SendToSentryDebug(errors.New("dbg"), "svc", "mod", "fn")
	SendToSentryEvent(&sentry.Event{Message: "ev"}, "svc", "mod", "fn")

	tr.mu.Lock()
	n := len(tr.events)
	tr.mu.Unlock()
	if n < 5 {
		t.Fatalf("expected at least 5 captured events, got %d", n)
	}
}
