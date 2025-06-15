package paycloudhelper

import (
	"bitbucket.org/paycloudid/paycloudhelper/phsentry"
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

func InitSentryOptions(options phsentry.SentryOptions) {
	phsentry.InitSentryOptions(options)
}

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
