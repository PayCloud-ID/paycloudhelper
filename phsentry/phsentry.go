package phsentry

import (
	"context"
	"time"

	"bitbucket.org/paycloudid/paycloudhelper/phhelper"
	"bitbucket.org/paycloudid/paycloudhelper/phlogger"
	"dario.cat/mergo"
	"github.com/getsentry/sentry-go"
)

var (
	sentryBreadcrumbData *SentryData
	sentryClient         *sentry.Client
	sentryClientOptions  *sentry.ClientOptions
)

type SentryData struct {
	Service  string `json:"service"`
	Module   string `json:"module"`
	Function string `json:"function"`
}

type SentryOptions struct {
	Dsn         string
	Environment string
	Release     string
	Debug       bool
	Options     *sentry.ClientOptions
	Data        *SentryData
}

func NewSentryData(dt *SentryData) {
	if dt == nil {
		return
	}
	sentryBreadcrumbData = &SentryData{}
	err := mergo.Merge(sentryBreadcrumbData, *dt)
	if err != nil {
		phlogger.LogE("[SENTRY] ERR initialized sentry data %s", err.Error())
		return
	}
}

func GetSentryClient() *sentry.Client {
	return sentryClient
}

func GetSentryData() *SentryData {
	return sentryBreadcrumbData
}

func GetSentryDataMap() (dt map[string]interface{}) {
	if sentryBreadcrumbData == nil {
		return
	}
	dt = make(map[string]interface{})
	_ = mergo.Map(&dt, sentryBreadcrumbData, mergo.WithOverride)
	return
}

func GetSentryClientOptions() *sentry.ClientOptions {
	return sentryClientOptions
}

func InitSentryOptions(options SentryOptions) {
	if options.Dsn == "" {
		return
	}
	clOpts := sentry.ClientOptions{
		Dsn:         options.Dsn,
		Environment: options.Environment,
		Release:     options.Release,
		Debug:       options.Debug,
	}

	//default options
	sentryClientOptions = &sentry.ClientOptions{
		AttachStacktrace: false,
		TracesSampleRate: 1.0,
	}

	//merge default options with input
	err := mergo.Merge(sentryClientOptions, clOpts)
	if err != nil {
		phlogger.LogF("[SENTRY] ERR merge options. %s", err.Error())
	}

	//merge another custom options if exists
	if options.Options != nil {
		errAdd := mergo.Merge(sentryClientOptions, options.Options)
		if errAdd != nil {
			phlogger.LogE("[SENTRY] ERR merge additional options. %s", errAdd.Error())
		}
	}
}

func InitSentry(options SentryOptions) *sentry.Client {
	if options.Dsn == "" {
		return nil
	}

	InitSentryOptions(options)
	client, err := sentry.NewClient(*sentryClientOptions)
	if err != nil {
		phlogger.LogF("sentry.Init: %s", err)
	} else {
		sentryClient = client

		//setup sentry breadcrumb data
		sd := &SentryData{
			Service:  phhelper.GetAppName(),
			Module:   phhelper.GetAppEnv(),
			Function: "paycloud-be-func",
		}
		if options.Data != nil {
			errDt := mergo.Merge(sd, options.Data, mergo.WithOverride)
			if errDt != nil {
				phlogger.LogE("[SENTRY] ERR merge sentry data. %s", errDt.Error())
			}
		}
		NewSentryData(sd)
	}

	return client
}

func SendToSentryMessage(message string, service, module, function string) {
	if message == "" {
		return
	}
	if sentryClient == nil {
		phlogger.LogW("[SENTRY] ERR_NO_CLIENT service: %s MSG: %s\n", service, message)
		return
	}
	hub := sentry.NewHub(sentryClient, sentry.NewScope())
	hub.WithScope(func(scope *sentry.Scope) {
		scope.SetLevel(sentry.LevelInfo)
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

func SendToSentryError(err error, service, module, function string) {
	if err == nil {
		return
	}
	if sentryClient == nil {
		phlogger.LogW("[SENTRY] ERR_NO_CLIENT service: %s ERR: %s\n", service, err.Error())
		return
	}
	hub := sentry.NewHub(sentryClient, sentry.NewScope())
	hub.WithScope(func(scope *sentry.Scope) {
		scope.SetLevel(sentry.LevelError)
		scope.AddBreadcrumb(&sentry.Breadcrumb{
			Type:     "Error",
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

func SendToSentryWarning(err error, service, module, function string) {
	if err == nil {
		return
	}
	if sentryClient == nil {
		phlogger.LogW("[SENTRY] ERR_NO_CLIENT service: %s WARN: %s\n", service, err.Error())
		return
	}
	hub := sentry.NewHub(sentryClient, sentry.NewScope())
	hub.WithScope(func(scope *sentry.Scope) {
		scope.SetLevel(sentry.LevelWarning)
		scope.AddBreadcrumb(&sentry.Breadcrumb{
			Type:     "Warning",
			Category: "Information",
			Message:  "Details of warning stack",
			Data: map[string]interface{}{
				"Service":  service,
				"Module":   module,
				"Function": function,
			},
		}, 5)
		hub.CaptureException(err)
	})
}

func SendToSentryDebug(err error, service, module, function string) {
	if err == nil {
		return
	}
	if sentryClient == nil {
		phlogger.LogW("[SENTRY] ERR_NO_CLIENT service: %s DEBUG: %s\n", service, err.Error())
		return
	}
	hub := sentry.NewHub(sentryClient, sentry.NewScope())
	hub.WithScope(func(scope *sentry.Scope) {
		scope.SetLevel(sentry.LevelDebug)
		scope.AddBreadcrumb(&sentry.Breadcrumb{
			Type:     "Debug",
			Category: "Debug",
			Message:  "Details of debug message stack",
			Data: map[string]interface{}{
				"Service":  service,
				"Module":   module,
				"Function": function,
			},
		}, 5)
		hub.CaptureException(err)
	})
}

func SendToSentryEvent(event *sentry.Event, service, module, function string) {
	if event == nil {
		return
	}
	if sentryClient == nil {
		phlogger.LogW("[SENTRY] ERR_NO_CLIENT service: %s EVENT: %s\n", service, module)
		return
	}
	hub := sentry.NewHub(sentryClient, sentry.NewScope())
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

// args[0] service string
// args[1] module string
// args[2] function string
func sendToSentry(msg interface{}, msgType string, args ...string) {
	if sentryClient == nil {
		return
	}
	if msg == nil {
		return
	}
	if v, ok := msg.(string); ok && v == "" {
		return
	}

	service := sentryBreadcrumbData.Service
	module := sentryBreadcrumbData.Module
	function := sentryBreadcrumbData.Function
	if len(args) > 0 && args[0] != "" {
		service = args[0]
	}
	if len(args) > 1 && args[1] != "" {
		module = args[1]
	}
	if len(args) > 2 && args[2] != "" {
		function = args[2]
	}

	switch msgType {
	case "message":
		if v, ok := msg.(string); ok && v != "" {
			SendToSentryMessage(v, service, module, function)
		}
		break
	case "debug":
		if v, ok := msg.(error); ok && v != nil {
			SendToSentryDebug(v, service, module, function)
		}
		break
	case "warning":
		if v, ok := msg.(error); ok && v != nil {
			SendToSentryWarning(v, service, module, function)
		}
		break
	case "event":
		if v, ok := msg.(*sentry.Event); ok && v != nil {
			SendToSentryEvent(v, service, module, function)
		}
		break
	case "error":
	default:
		if v, ok := msg.(error); ok && v != nil {
			SendToSentryError(v, service, module, function)
		}
		break
	}
}

func SendSentryError(err error, args ...string) {
	if err == nil {
		return
	}
	sendToSentry(err, "error", args...)
}

func SendSentryWarning(err error, args ...string) {
	if err == nil {
		return
	}
	sendToSentry(err, "warning", args...)
}

func SendSentryDebug(err error, args ...string) {
	if err == nil {
		return
	}
	sendToSentry(err, "debug", args...)
}

func SendSentryMessage(msg string, args ...string) {
	if msg == "" {
		return
	}
	sendToSentry(msg, "message", args...)
}

func SendSentryEvent(event *sentry.Event, args ...string) {
	if event == nil {
		return
	}
	sendToSentry(event, "event", args...)
}

// ReceiveLog forwards a log event to Sentry based on the level.
// This is called by log hook subscribers — do not call directly.
// level: "debug" | "info" | "warn" | "error" | "fatal"
// message: formatted log string
func ReceiveLog(level, message string) {
	if sentryClient == nil {
		return
	}
	hub := sentry.NewHub(sentryClient, sentry.NewScope())
	hub.WithScope(func(scope *sentry.Scope) {
		scope.SetLevel(sentryLevelFor(level))
		addDefaultBreadcrumb(scope, level, message)
		hub.CaptureMessage(message)
	})
}

// FlushSentry waits for buffered Sentry events to be sent, up to timeout.
// Call this before process exit (e.g. in shutdown handler) to avoid losing events.
func FlushSentry(timeout time.Duration) {
	if sentryClient == nil {
		return
	}
	sentryClient.Flush(timeout)
}

// SentryEnabled returns true if a Sentry client has been initialized.
// Use this to guard expensive error construction before calling SendSentryError.
func SentryEnabled() bool {
	return sentryClient != nil
}

// sentryLevelFor maps log level strings to sentry.Level.
func sentryLevelFor(level string) sentry.Level {
	switch level {
	case "fatal":
		return sentry.LevelFatal
	case "error":
		return sentry.LevelError
	case "warn":
		return sentry.LevelWarning
	case "info":
		return sentry.LevelInfo
	default:
		return sentry.LevelDebug
	}
}

// addDefaultBreadcrumb adds service context to a Sentry scope.
func addDefaultBreadcrumb(scope *sentry.Scope, level, message string) {
	if sentryBreadcrumbData == nil {
		return
	}
	scope.AddBreadcrumb(&sentry.Breadcrumb{
		Type:     "default",
		Category: level,
		Message:  message,
		Data:     GetSentryDataMap(),
	}, 10)
}

// SendSentryErrorWithContext sends an error to Sentry with request context.
// ctx may carry a Sentry Hub (set by middleware); falls back to global client.
func SendSentryErrorWithContext(ctx context.Context, err error, args ...string) {
	if err == nil || sentryClient == nil {
		return
	}
	hub := sentry.GetHubFromContext(ctx)
	if hub == nil {
		hub = sentry.NewHub(sentryClient, sentry.NewScope())
	}
	hub.WithScope(func(scope *sentry.Scope) {
		scope.SetLevel(sentry.LevelError)
		addDefaultBreadcrumb(scope, "error", err.Error())
		hub.CaptureException(err)
	})
}
