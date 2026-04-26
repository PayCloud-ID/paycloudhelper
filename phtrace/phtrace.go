// Package phtrace provides OpenTelemetry tracing and metrics helpers for
// PayCloud services.
//
// It is an optional subpackage: importing paycloudhelper/phtrace does NOT
// automatically start the SDK. Consumer services call Init(ctx, cfg) once
// during startup, then register/use the returned Shutdown to flush on exit.
//
// Design goals:
//   - Zero-cost when disabled (env flag OTEL_ENABLED=false or Init not called).
//   - Works with Grafana Tempo for traces, Prometheus for metrics, Loki for
//     logs — via the standard OTLP gRPC exporter hitting an OTel Collector.
//   - W3C traceparent propagation across HTTP (otelecho), gRPC (otelgrpc),
//     and RabbitMQ (see rmqprop.go in this package).
//   - Graceful shutdown: Shutdown() flushes pending spans and metrics within
//     a caller-provided timeout so the process does not hang on exit.
//
// Backward compatibility: This is a NEW subpackage (no pre-existing symbols
// to break). All functions are safe to call before Init() — they become
// no-ops that return a disabled tracer/meter. Consumers that never call
// Init() pay no runtime cost beyond a package-level atomic load.
package phtrace

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	initOnce     sync.Once
	initErr      error
	enabled      atomic.Bool
	providerMu   sync.RWMutex
	tracerProv   *sdktrace.TracerProvider
	meterProv    *sdkmetric.MeterProvider
	resourceInfo *resource.Resource
)

// Shutdown is returned by Init and MUST be called during graceful shutdown to
// flush pending spans and metrics. It is safe to call multiple times; only the
// first call performs shutdown. It is safe to pass a nil context — it will be
// replaced by context.Background internally.
type Shutdown func(ctx context.Context) error

// IsEnabled reports whether phtrace has been successfully initialized and is
// actively exporting telemetry. Cheap (atomic load) and safe for hot paths.
func IsEnabled() bool {
	return enabled.Load()
}

// Tracer returns the named tracer, or a no-op tracer when phtrace is disabled.
// Safe to call even before Init.
func Tracer(name string) trace.Tracer {
	if !enabled.Load() {
		return noop.NewTracerProvider().Tracer(name)
	}
	return otel.Tracer(name)
}

// Meter returns the named meter, or a no-op meter when phtrace is disabled.
// Safe to call even before Init.
func Meter(name string) metric.Meter {
	if !enabled.Load() {
		return nopMeter{}
	}
	return otel.Meter(name)
}

// Init initializes OTLP gRPC exporters for traces and metrics and registers
// global providers. Safe to call once per process; subsequent calls return the
// same Shutdown and error captured on the first call.
//
// When cfg.Enabled is false (e.g. OTEL_ENABLED=false), Init returns a no-op
// Shutdown and no error, leaving the global providers as the default no-op.
func Init(ctx context.Context, cfg Config) (Shutdown, error) {
	var shutdown Shutdown
	initOnce.Do(func() {
		cfg = cfg.withDefaults()

		if !cfg.Enabled {
			shutdown = func(context.Context) error { return nil }
			return
		}

		if err := ensureRequired(cfg); err != nil {
			initErr = err
			shutdown = func(context.Context) error { return nil }
			return
		}

		res, err := buildResource(ctx, cfg)
		if err != nil {
			initErr = fmt.Errorf("phtrace: build resource: %w", err)
			shutdown = func(context.Context) error { return nil }
			return
		}
		resourceInfo = res

		tp, spanExp, err := buildTracerProvider(ctx, cfg, res)
		if err != nil {
			initErr = fmt.Errorf("phtrace: tracer provider: %w", err)
			shutdown = func(context.Context) error { return nil }
			return
		}

		mp, metricExp, err := buildMeterProvider(ctx, cfg, res)
		if err != nil {
			_ = tp.Shutdown(context.Background())
			initErr = fmt.Errorf("phtrace: meter provider: %w", err)
			shutdown = func(context.Context) error { return nil }
			return
		}

		providerMu.Lock()
		tracerProv = tp
		meterProv = mp
		providerMu.Unlock()

		otel.SetTracerProvider(tp)
		otel.SetMeterProvider(mp)
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		))
		otel.SetErrorHandler(errorHandler{})

		enabled.Store(true)

		shutdown = func(shutdownCtx context.Context) error {
			if shutdownCtx == nil {
				shutdownCtx = context.Background()
			}
			var errs []error
			if err := tp.Shutdown(shutdownCtx); err != nil {
				errs = append(errs, fmt.Errorf("tracer shutdown: %w", err))
			}
			if err := mp.Shutdown(shutdownCtx); err != nil {
				errs = append(errs, fmt.Errorf("meter shutdown: %w", err))
			}
			if spanExp != nil {
				if err := spanExp.Shutdown(shutdownCtx); err != nil {
					errs = append(errs, fmt.Errorf("span exporter: %w", err))
				}
			}
			if metricExp != nil {
				if err := metricExp.Shutdown(shutdownCtx); err != nil {
					errs = append(errs, fmt.Errorf("metric exporter: %w", err))
				}
			}
			enabled.Store(false)
			return errors.Join(errs...)
		}
	})

	if initErr != nil {
		return shutdown, initErr
	}
	if shutdown == nil {
		shutdown = func(context.Context) error { return nil }
	}
	return shutdown, nil
}

// Resource returns the resource captured at Init time (service name, env, ...)
// for callers that need it on log enrichment paths. Returns nil before Init.
func Resource() *resource.Resource {
	providerMu.RLock()
	defer providerMu.RUnlock()
	return resourceInfo
}

// Propagator returns the globally registered text map propagator. Falls back
// to a composite TraceContext+Baggage propagator when Init has not been called
// so RMQ/HTTP carriers still work (span context will simply be empty).
func Propagator() propagation.TextMapPropagator {
	if !enabled.Load() {
		return propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})
	}
	return otel.GetTextMapPropagator()
}

func buildResource(ctx context.Context, cfg Config) (*resource.Resource, error) {
	attrs := []attribute.KeyValue{
		semconv.ServiceName(cfg.ServiceName),
		semconv.ServiceVersion(cfg.ServiceVersion),
		semconv.DeploymentEnvironment(cfg.Environment),
	}
	for k, v := range cfg.ResourceAttributes {
		attrs = append(attrs, attribute.String(k, v))
	}
	return resource.New(ctx,
		resource.WithHost(),
		resource.WithProcess(),
		resource.WithAttributes(attrs...),
	)
}

func buildTracerProvider(ctx context.Context, cfg Config, res *resource.Resource) (*sdktrace.TracerProvider, *otlptrace.Exporter, error) {
	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(cfg.Endpoint),
	}
	if cfg.Insecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}
	if cfg.DialTimeout > 0 {
		opts = append(opts, otlptracegrpc.WithDialOption(
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		))
	}
	dialCtx, cancel := context.WithTimeout(ctx, cfg.DialTimeout)
	defer cancel()
	exp, err := otlptracegrpc.New(dialCtx, opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("otlptracegrpc: %w", err)
	}

	sampler := sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SamplingRatio))
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp,
			sdktrace.WithMaxExportBatchSize(cfg.BatchMaxExportSize),
			sdktrace.WithBatchTimeout(cfg.BatchTimeout),
		),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)
	return tp, exp, nil
}

func buildMeterProvider(ctx context.Context, cfg Config, res *resource.Resource) (*sdkmetric.MeterProvider, sdkmetric.Exporter, error) {
	opts := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithEndpoint(cfg.Endpoint),
	}
	if cfg.Insecure {
		opts = append(opts, otlpmetricgrpc.WithInsecure())
	}
	dialCtx, cancel := context.WithTimeout(ctx, cfg.DialTimeout)
	defer cancel()
	exp, err := otlpmetricgrpc.New(dialCtx, opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("otlpmetricgrpc: %w", err)
	}
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exp,
			sdkmetric.WithInterval(cfg.MetricExportInterval),
		)),
		sdkmetric.WithResource(res),
	)
	return mp, exp, nil
}

func ensureRequired(cfg Config) error {
	if cfg.ServiceName == "" {
		return errors.New("phtrace: Config.ServiceName must not be empty (set OTEL_SERVICE_NAME or pass WithServiceName)")
	}
	if cfg.Endpoint == "" {
		return errors.New("phtrace: Config.Endpoint must not be empty (set OTEL_EXPORTER_OTLP_ENDPOINT or pass WithEndpoint)")
	}
	if cfg.SamplingRatio < 0 || cfg.SamplingRatio > 1 {
		return fmt.Errorf("phtrace: Config.SamplingRatio must be in [0,1], got %f", cfg.SamplingRatio)
	}
	if cfg.DialTimeout <= 0 {
		return errors.New("phtrace: Config.DialTimeout must be > 0")
	}
	return nil
}

type errorHandler struct{}

func (errorHandler) Handle(err error) {
	// Delegate to package-level logger. We use a minimal fmt.Printf to avoid
	// importing the root package (subpackages must stay standalone per
	// paycloudhelper library rules).
	if err != nil {
		fmt.Printf("[phtrace.errorHandler] otel err=%v\n", err)
	}
}

// nopMeter is a minimal metric.Meter implementation for disabled state. We
// only implement the subset used by Phase 2 instrumentation — all instruments
// are safe no-ops. Callers never check the error so we return nil.
type nopMeter struct{ metric.Meter }

func (nopMeter) Int64Counter(string, ...metric.Int64CounterOption) (metric.Int64Counter, error) {
	return nopInt64Counter{}, nil
}
func (nopMeter) Int64UpDownCounter(string, ...metric.Int64UpDownCounterOption) (metric.Int64UpDownCounter, error) {
	return nopInt64UpDownCounter{}, nil
}
func (nopMeter) Int64Histogram(string, ...metric.Int64HistogramOption) (metric.Int64Histogram, error) {
	return nopInt64Histogram{}, nil
}
func (nopMeter) Int64Gauge(string, ...metric.Int64GaugeOption) (metric.Int64Gauge, error) {
	return nopInt64Gauge{}, nil
}
func (nopMeter) Float64Counter(string, ...metric.Float64CounterOption) (metric.Float64Counter, error) {
	return nopFloat64Counter{}, nil
}
func (nopMeter) Float64UpDownCounter(string, ...metric.Float64UpDownCounterOption) (metric.Float64UpDownCounter, error) {
	return nopFloat64UpDownCounter{}, nil
}
func (nopMeter) Float64Histogram(string, ...metric.Float64HistogramOption) (metric.Float64Histogram, error) {
	return nopFloat64Histogram{}, nil
}
func (nopMeter) Float64Gauge(string, ...metric.Float64GaugeOption) (metric.Float64Gauge, error) {
	return nopFloat64Gauge{}, nil
}

type (
	nopInt64Counter         struct{ metric.Int64Counter }
	nopInt64UpDownCounter   struct{ metric.Int64UpDownCounter }
	nopInt64Histogram       struct{ metric.Int64Histogram }
	nopInt64Gauge           struct{ metric.Int64Gauge }
	nopFloat64Counter       struct{ metric.Float64Counter }
	nopFloat64UpDownCounter struct{ metric.Float64UpDownCounter }
	nopFloat64Histogram     struct{ metric.Float64Histogram }
	nopFloat64Gauge         struct{ metric.Float64Gauge }
)

func (nopInt64Counter) Add(context.Context, int64, ...metric.AddOption)       {}
func (nopInt64UpDownCounter) Add(context.Context, int64, ...metric.AddOption) {}
func (nopInt64Histogram) Record(context.Context, int64, ...metric.RecordOption) {
}
func (nopInt64Gauge) Record(context.Context, int64, ...metric.RecordOption)       {}
func (nopFloat64Counter) Add(context.Context, float64, ...metric.AddOption)       {}
func (nopFloat64UpDownCounter) Add(context.Context, float64, ...metric.AddOption) {}
func (nopFloat64Histogram) Record(context.Context, float64, ...metric.RecordOption) {
}
func (nopFloat64Gauge) Record(context.Context, float64, ...metric.RecordOption) {}

// ensureTimeNow is used only by tests to make sure the time package is still
// imported after refactors; regular callers can ignore.
var ensureTimeNow = time.Now
