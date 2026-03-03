package phsentry

import (
	"context"
	"fmt"
	"strings"
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

const (
	defaultFunctionName = "paycloud-be-func"
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

// sentryDiagnosticWriter bridges Sentry SDK internal debug logs to paycloudhelper log output.
// It intentionally writes via phlogger.Log (golog) directly to avoid hook recursion.
type sentryDiagnosticWriter struct{}

func (w sentryDiagnosticWriter) Write(p []byte) (n int, err error) {
	msg := normalizeSentryDiagnosticMessage(string(p))
	if msg != "" {
		phlogger.Log.Infof("%s", msg)
	}
	return len(p), nil
}

func normalizeSentryDiagnosticMessage(raw string) string {
	msg := strings.TrimSpace(raw)
	if msg == "" {
		return ""
	}

	msg = strings.TrimPrefix(msg, "[Sentry] ")
	msg = strings.TrimPrefix(msg, phhelper.BuildLogPrefix("Sentry")+" ")
	parts := strings.SplitN(msg, " ", 3)
	if len(parts) == 3 && isSentryDate(parts[0]) && isSentryTime(parts[1]) {
		msg = parts[2]
	}

	return fmt.Sprintf("%s %s", phhelper.BuildLogPrefix("Sentry"), msg)
}

func functionLogPrefix(functionName string) string {
	return phhelper.BuildLogPrefix(functionName)
}

func resolveSentryContext(args ...string) (service, module, function string) {
	service = phhelper.GetAppName()
	module = phhelper.GetAppEnv()
	function = defaultFunctionName

	if sentryBreadcrumbData != nil {
		if sentryBreadcrumbData.Service != "" {
			service = sentryBreadcrumbData.Service
		}
		if sentryBreadcrumbData.Module != "" {
			module = sentryBreadcrumbData.Module
		}
		if sentryBreadcrumbData.Function != "" {
			function = sentryBreadcrumbData.Function
		}
	}

	if len(args) > 0 && args[0] != "" {
		service = args[0]
	}
	if len(args) > 1 && args[1] != "" {
		module = args[1]
	}
	if len(args) > 2 && args[2] != "" {
		function = args[2]
	}

	return service, module, function
}

func logNoClient(functionName, service, eventType, detail string) {
	phlogger.LogW("%s Sentry client not initialized service=%s type=%s detail=%s", functionLogPrefix(functionName), service, eventType, detail)
}

func captureWithBreadcrumb(
	level sentry.Level,
	breadcrumbType,
	category,
	breadcrumbMessage,
	service,
	module,
	function string,
	capture func(hub *sentry.Hub),
) {
	hub := sentry.NewHub(sentryClient, sentry.NewScope())
	hub.WithScope(func(scope *sentry.Scope) {
		scope.SetLevel(level)
		scope.AddBreadcrumb(&sentry.Breadcrumb{
			Type:     breadcrumbType,
			Category: category,
			Message:  breadcrumbMessage,
			Data: map[string]interface{}{
				"Service":  service,
				"Module":   module,
				"Function": function,
			},
		}, 5)
		capture(hub)
	})
}

func isSentryDate(s string) bool {
	_, err := time.Parse("2006/01/02", s)
	return err == nil
}

func isSentryTime(s string) bool {
	_, err := time.Parse("15:04:05", s)
	return err == nil
}

func NewSentryData(dt *SentryData) {
	if dt == nil {
		return
	}
	sentryBreadcrumbData = &SentryData{}
	err := mergo.Merge(sentryBreadcrumbData, *dt)
	if err != nil {
		phlogger.LogE("%s failed to initialize Sentry data err=%v", functionLogPrefix("NewSentryData"), err)
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
		DebugWriter:      sentryDiagnosticWriter{},
	}

	//merge default options with input
	err := mergo.Merge(sentryClientOptions, clOpts)
	if err != nil {
		phlogger.LogF("%s failed to merge Sentry options err=%v", functionLogPrefix("InitSentryOptions"), err)
	}

	//merge another custom options if exists
	if options.Options != nil {
		errAdd := mergo.Merge(sentryClientOptions, options.Options)
		if errAdd != nil {
			phlogger.LogE("%s failed to merge additional Sentry options err=%v", functionLogPrefix("InitSentryOptions"), errAdd)
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
		phlogger.LogF("%s failed to initialize Sentry client err=%v", functionLogPrefix("InitSentry"), err)
	} else {
		sentryClient = client

		//setup sentry breadcrumb data
		sd := &SentryData{
			Service:  phhelper.GetAppName(),
			Module:   phhelper.GetAppEnv(),
			Function: defaultFunctionName,
		}
		if options.Data != nil {
			errDt := mergo.Merge(sd, options.Data, mergo.WithOverride)
			if errDt != nil {
				phlogger.LogE("%s failed to merge Sentry data err=%v", functionLogPrefix("InitSentry"), errDt)
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
		logNoClient("SendToSentryMessage", service, "message", message)
		return
	}
	captureWithBreadcrumb(
		sentry.LevelInfo,
		"Info",
		"Information",
		"Details of message stack",
		service,
		module,
		function,
		func(hub *sentry.Hub) {
			hub.CaptureMessage(message)
		},
	)
}

func SendToSentryError(err error, service, module, function string) {
	if err == nil {
		return
	}
	if sentryClient == nil {
		logNoClient("SendToSentryError", service, "error", err.Error())
		return
	}
	captureWithBreadcrumb(
		sentry.LevelError,
		"Error",
		"Information",
		"Details of error stack",
		service,
		module,
		function,
		func(hub *sentry.Hub) {
			hub.CaptureException(err)
		},
	)
}

func SendToSentryWarning(err error, service, module, function string) {
	if err == nil {
		return
	}
	if sentryClient == nil {
		logNoClient("SendToSentryWarning", service, "warning", err.Error())
		return
	}
	captureWithBreadcrumb(
		sentry.LevelWarning,
		"Warning",
		"Information",
		"Details of warning stack",
		service,
		module,
		function,
		func(hub *sentry.Hub) {
			hub.CaptureException(err)
		},
	)
}

func SendToSentryDebug(err error, service, module, function string) {
	if err == nil {
		return
	}
	if sentryClient == nil {
		logNoClient("SendToSentryDebug", service, "debug", err.Error())
		return
	}
	captureWithBreadcrumb(
		sentry.LevelDebug,
		"Debug",
		"Debug",
		"Details of debug message stack",
		service,
		module,
		function,
		func(hub *sentry.Hub) {
			hub.CaptureException(err)
		},
	)
}

func SendToSentryEvent(event *sentry.Event, service, module, function string) {
	if event == nil {
		return
	}
	if sentryClient == nil {
		logNoClient("SendToSentryEvent", service, "event", module)
		return
	}
	captureWithBreadcrumb(
		sentry.LevelInfo,
		"Info",
		"Information",
		"Details of event stack",
		service,
		module,
		function,
		func(hub *sentry.Hub) {
			hub.CaptureEvent(event)
		},
	)
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

	service, module, function := resolveSentryContext(args...)

	switch msgType {
	case "message":
		if v, ok := msg.(string); ok && v != "" {
			SendToSentryMessage(v, service, module, function)
		}
	case "debug":
		if v, ok := msg.(error); ok && v != nil {
			SendToSentryDebug(v, service, module, function)
		}
	case "warning":
		if v, ok := msg.(error); ok && v != nil {
			SendToSentryWarning(v, service, module, function)
		}
	case "event":
		if v, ok := msg.(*sentry.Event); ok && v != nil {
			SendToSentryEvent(v, service, module, function)
		}
	case "error":
	default:
		if v, ok := msg.(error); ok && v != nil {
			SendToSentryError(v, service, module, function)
		}
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
