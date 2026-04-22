package phtrace

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config controls OTLP exporter behavior for traces and metrics. Only
// ServiceName and Endpoint are strictly required; the rest have safe defaults.
//
// Typical usage in a service main():
//
//	shutdown, err := phtrace.Init(ctx, phtrace.FromEnv(
//	    phtrace.WithServiceName("paycloud-be-createordersnap-manager"),
//	    phtrace.WithServiceVersion(build.Version),
//	))
type Config struct {
	// Enabled toggles OTel export. When false, Init returns a no-op Shutdown
	// and all package helpers degrade to no-ops. Env: OTEL_ENABLED (default true
	// when any OTEL_EXPORTER_OTLP_ENDPOINT is set, false otherwise).
	Enabled bool

	// ServiceName is the logical service identifier. Required.
	// Env: OTEL_SERVICE_NAME.
	ServiceName string

	// ServiceVersion is the build/semver version. Env: OTEL_SERVICE_VERSION.
	ServiceVersion string

	// Environment is the deployment env (prod, stg, dev). Env: OTEL_DEPLOYMENT_ENV.
	Environment string

	// Endpoint is the OTLP collector host:port (gRPC). Required when enabled.
	// Env: OTEL_EXPORTER_OTLP_ENDPOINT (e.g. "otel-collector:4317").
	Endpoint string

	// Insecure disables TLS for the OTLP gRPC connection. Typically true in
	// dev / in-cluster where the collector is an internal service.
	// Env: OTEL_EXPORTER_OTLP_INSECURE (default true).
	Insecure bool

	// SamplingRatio is the head-based sampling ratio for traces (0..1). 1.0
	// captures everything, 0.05 captures 5%. Parent-based sampling is applied
	// so downstream spans inherit parent decisions.
	// Env: OTEL_TRACES_SAMPLER_ARG (default 1.0).
	SamplingRatio float64

	// DialTimeout is the timeout applied while connecting to the OTLP endpoint
	// during Init. Kept short so a flaky collector cannot block service startup.
	// Env: OTEL_DIAL_TIMEOUT (default 5s).
	DialTimeout time.Duration

	// BatchTimeout is the maximum delay between span export batches.
	// Env: OTEL_BATCH_TIMEOUT (default 5s).
	BatchTimeout time.Duration

	// BatchMaxExportSize is the maximum spans per export batch.
	// Env: OTEL_BATCH_MAX_EXPORT_SIZE (default 512).
	BatchMaxExportSize int

	// MetricExportInterval is how often the periodic metric reader pushes.
	// Env: OTEL_METRIC_EXPORT_INTERVAL (default 15s).
	MetricExportInterval time.Duration

	// ResourceAttributes are extra key=value pairs attached to every span and
	// metric. Env: OTEL_RESOURCE_ATTRIBUTES (comma-separated key=value).
	ResourceAttributes map[string]string
}

// Option configures a Config via functional options, following the
// paycloudhelper public API style rule for options with >= 3 fields.
type Option func(*Config)

// WithServiceName sets Config.ServiceName (required).
func WithServiceName(v string) Option { return func(c *Config) { c.ServiceName = v } }

// WithServiceVersion sets Config.ServiceVersion.
func WithServiceVersion(v string) Option { return func(c *Config) { c.ServiceVersion = v } }

// WithEnvironment sets Config.Environment.
func WithEnvironment(v string) Option { return func(c *Config) { c.Environment = v } }

// WithEndpoint sets Config.Endpoint (required when enabled).
func WithEndpoint(v string) Option { return func(c *Config) { c.Endpoint = v } }

// WithInsecure sets Config.Insecure.
func WithInsecure(v bool) Option { return func(c *Config) { c.Insecure = v } }

// WithSamplingRatio sets Config.SamplingRatio (clamped to [0,1] at Init).
func WithSamplingRatio(v float64) Option { return func(c *Config) { c.SamplingRatio = v } }

// WithEnabled sets Config.Enabled explicitly.
func WithEnabled(v bool) Option { return func(c *Config) { c.Enabled = v } }

// WithResourceAttribute adds or replaces a single resource attribute.
func WithResourceAttribute(k, v string) Option {
	return func(c *Config) {
		if c.ResourceAttributes == nil {
			c.ResourceAttributes = map[string]string{}
		}
		c.ResourceAttributes[k] = v
	}
}

// FromEnv loads a Config from environment variables and applies any caller
// overrides as functional options on top. The loader is intentionally
// forgiving: unset env vars fall back to safe defaults so local developer
// runs do not require every OTEL_* variable to be set.
func FromEnv(opts ...Option) Config {
	c := Config{
		ServiceName:          strings.TrimSpace(os.Getenv("OTEL_SERVICE_NAME")),
		ServiceVersion:       strings.TrimSpace(os.Getenv("OTEL_SERVICE_VERSION")),
		Environment:          firstNonEmpty(os.Getenv("OTEL_DEPLOYMENT_ENV"), os.Getenv("APP_ENV"), "dev"),
		Endpoint:             strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")),
		Insecure:             envBool("OTEL_EXPORTER_OTLP_INSECURE", true),
		SamplingRatio:        envFloat("OTEL_TRACES_SAMPLER_ARG", 1.0),
		DialTimeout:          envDuration("OTEL_DIAL_TIMEOUT", 5*time.Second),
		BatchTimeout:         envDuration("OTEL_BATCH_TIMEOUT", 5*time.Second),
		BatchMaxExportSize:   envInt("OTEL_BATCH_MAX_EXPORT_SIZE", 512),
		MetricExportInterval: envDuration("OTEL_METRIC_EXPORT_INTERVAL", 15*time.Second),
		ResourceAttributes:   parseResourceAttrs(os.Getenv("OTEL_RESOURCE_ATTRIBUTES")),
	}
	// Default: enabled iff endpoint is set. OTEL_ENABLED overrides.
	if v := strings.TrimSpace(os.Getenv("OTEL_ENABLED")); v != "" {
		c.Enabled = truthy(v)
	} else {
		c.Enabled = c.Endpoint != ""
	}
	for _, o := range opts {
		o(&c)
	}
	return c
}

// withDefaults backfills zero-value config fields with sensible defaults to
// keep Init robust when consumers construct a Config by hand.
func (c Config) withDefaults() Config {
	if c.DialTimeout <= 0 {
		c.DialTimeout = 5 * time.Second
	}
	if c.BatchTimeout <= 0 {
		c.BatchTimeout = 5 * time.Second
	}
	if c.BatchMaxExportSize <= 0 {
		c.BatchMaxExportSize = 512
	}
	if c.MetricExportInterval <= 0 {
		c.MetricExportInterval = 15 * time.Second
	}
	if c.SamplingRatio == 0 {
		c.SamplingRatio = 1.0
	}
	if c.Environment == "" {
		c.Environment = "dev"
	}
	return c
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

func truthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "t", "true", "y", "yes", "on":
		return true
	}
	return false
}

func envBool(key string, def bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	return truthy(v)
}

func envInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func envFloat(key string, def float64) float64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return def
	}
	return f
}

func envDuration(key string, def time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}

func parseResourceAttrs(raw string) map[string]string {
	out := map[string]string{}
	if strings.TrimSpace(raw) == "" {
		return out
	}
	for _, kv := range strings.Split(raw, ",") {
		pair := strings.SplitN(strings.TrimSpace(kv), "=", 2)
		if len(pair) != 2 {
			continue
		}
		k := strings.TrimSpace(pair[0])
		v := strings.TrimSpace(pair[1])
		if k != "" {
			out[k] = v
		}
	}
	return out
}
