package paycloudgohelper

import (
	"log"

	"github.com/getsentry/sentry-go"
)

var (
	Sentries     *SentryData
	SentryClient *sentry.Client
)

type SentryData struct {
	Dsn             string
	Environment     string
	Release         string
	Debug           bool
	TraceSampleRate float64
	Event           *sentry.Event
}

func NewSentryData() *SentryData {
	return new(SentryData)
}

func InitSentry(Dsn, Environment, Release string, Debug bool) *sentry.Client {
	client, err := sentry.NewClient(sentry.ClientOptions{
		Dsn:              Dsn,
		Environment:      Environment,
		Release:          Release,
		Debug:            Debug,
		AttachStacktrace: false,
		TracesSampleRate: 1.0,
	})

	if err != nil {
		log.Fatalf("sentry.Init: %s", err)
	}

	return client

}

func SendSentryMessage(message string, service, module, function string) {
	hub := sentry.NewHub(SentryClient, sentry.NewScope())
	hub.WithScope(func(scope *sentry.Scope) {
		scope.SetLevel(sentry.LevelError)
		scope.AddBreadcrumb(&sentry.Breadcrumb{
			Type:     "Info",
			Category: "Information",
			Message:  "Details of message stack",
			Data: map[string]interface{}{
				"Service":  service,
				"Module":   module,
				"Function": function,
			},
		}, 5)
		hub.CaptureMessage(message)
	})
}

func SendSentryError(err error, service, module, function string) {
	hub := sentry.NewHub(SentryClient, sentry.NewScope())
	hub.WithScope(func(scope *sentry.Scope) {
		scope.SetLevel(sentry.LevelError)
		scope.AddBreadcrumb(&sentry.Breadcrumb{
			Type:     "Info",
			Category: "Information",
			Message:  "Details of error stack",
			Data: map[string]interface{}{
				"Service":  service,
				"Module":   module,
				"Function": function,
			},
		}, 5)
		hub.CaptureException(err)
	})
}

func SendSentryEvent(event *sentry.Event, service, module, function string) {
	hub := sentry.NewHub(SentryClient, sentry.NewScope())
	hub.WithScope(func(scope *sentry.Scope) {
		scope.SetLevel(sentry.LevelInfo)
		scope.AddBreadcrumb(&sentry.Breadcrumb{
			Type:     "Info",
			Category: "Information",
			Message:  "Details of event stack",
			Data: map[string]interface{}{
				"Service":  service,
				"Module":   module,
				"Function": function,
			},
		}, 5)
		hub.CaptureEvent(event)
	})
}
