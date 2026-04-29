package paycloudhelper

import (
	"context"
	"time"

	"github.com/PayCloud-ID/paycloudhelper/phsentry"
	"github.com/getsentry/sentry-go"
)

func NewSentryData(dt *phsentry.SentryData) {
	phsentry.NewSentryData(dt)
}

func GetSentryClient() *sentry.Client {
	return phsentry.GetSentryClient()
}

func GetSentryData() *phsentry.SentryData {
	return phsentry.GetSentryData()
}

func GetSentryClientOptions() *sentry.ClientOptions {
	return phsentry.GetSentryClientOptions()
}

// InitSentryOptions prepares merged Sentry client options without instantiating the client.
// See phsentry.SentryOptions (including Debug / service-level SENTRY_DEBUG wiring).
func InitSentryOptions(options phsentry.SentryOptions) {
	phsentry.InitSentryOptions(options)
}

// InitSentry initializes the global Sentry client from options; nil when Dsn is empty.
// Debug enables sentry-go SDK diagnostic logs (not the same as SendSentryDebug events).
func InitSentry(options phsentry.SentryOptions) *sentry.Client {
	return phsentry.InitSentry(options)
}

func SendToSentryMessage(message string, service, module, function string) {
	phsentry.SendToSentryMessage(message, service, module, function)
}

func SendToSentryError(err error, service, module, function string) {
	phsentry.SendToSentryError(err, service, module, function)
}

func SendToSentryWarning(err error, service, module, function string) {
	phsentry.SendToSentryWarning(err, service, module, function)
}

func SendToSentryDebug(err error, service, module, function string) {
	phsentry.SendToSentryDebug(err, service, module, function)
}

func SendToSentryEvent(event *sentry.Event, service, module, function string) {
	phsentry.SendToSentryEvent(event, service, module, function)
}

func SendSentryError(err error, args ...string) {
	phsentry.SendSentryError(err, args...)
}

func SendSentryWarning(err error, args ...string) {
	phsentry.SendSentryWarning(err, args...)
}

func SendSentryDebug(err error, args ...string) {
	phsentry.SendSentryDebug(err, args...)
}

func SendSentryMessage(msg string, args ...string) {
	phsentry.SendSentryMessage(msg, args...)
}

func SendSentryEvent(event *sentry.Event, args ...string) {
	phsentry.SendSentryEvent(event, args...)
}

// FlushSentry waits up to timeout for buffered Sentry events to drain.
// Call before process shutdown to avoid losing queued errors.
func FlushSentry(timeout time.Duration) {
	phsentry.FlushSentry(timeout)
}

// SentryEnabled returns true if Sentry has been initialized.
func SentryEnabled() bool {
	return phsentry.SentryEnabled()
}

// SendSentryErrorWithContext captures an error with request context (e.g. from Echo).
// Extra string args are passed through to phsentry for breadcrumb metadata.
func SendSentryErrorWithContext(ctx context.Context, err error, args ...string) {
	phsentry.SendSentryErrorWithContext(ctx, err, args...)
}
