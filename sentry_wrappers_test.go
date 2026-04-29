package paycloudhelper

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/PayCloud-ID/paycloudhelper/phsentry"
	"github.com/getsentry/sentry-go"
)

func TestSentryRootWrappers_NoClientSafe(t *testing.T) {
	NewSentryData(nil)
	NewSentryData(&phsentry.SentryData{Service: "s", Module: "m", Function: "f"})

	_ = GetSentryClient()
	_ = GetSentryData()
	_ = GetSentryClientOptions()

	phsentry.InitSentryOptions(phsentry.SentryOptions{Dsn: ""})
	_ = InitSentry(phsentry.SentryOptions{Dsn: ""})

	SendToSentryMessage("m", "svc", "mod", "fn")
	SendToSentryError(errors.New("e"), "svc", "mod", "fn")
	SendToSentryWarning(errors.New("w"), "svc", "mod", "fn")
	SendToSentryDebug(errors.New("d"), "svc", "mod", "fn")
	SendToSentryEvent(sentry.NewEvent(), "svc", "mod", "fn")

	SendSentryError(errors.New("x"), "a", "b", "c")
	SendSentryWarning(errors.New("x"))
	SendSentryDebug(errors.New("x"))
	SendSentryMessage("msg", "a", "b", "c")
	SendSentryEvent(sentry.NewEvent(), "a", "b", "c")

	FlushSentry(1 * time.Millisecond)
	if SentryEnabled() {
		t.Log("SentryEnabled true (unexpected in unit env without DSN)")
	}

	SendSentryErrorWithContext(context.Background(), errors.New("ctx"))
}
