# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [v1.9.1] - 2026-04-29

### Changed

- Migrated module identity from `bitbucket.org/paycloudid/paycloudhelper` to
  `github.com/PayCloud-ID/paycloudhelper` across all package imports.
- Updated repository documentation and agent guidance to use GitHub module and
  import examples consistently.

### Fixed

- Resolved mixed-module import breakage risk by removing remaining Bitbucket
  import paths from runtime and test code.

## [v1.9.0] - 2026-04-22

### Added

- **New `phtrace` subpackage**: OpenTelemetry tracing and metrics helpers for
  PayCloud services supporting Grafana Tempo/Loki/Prometheus backend via OTLP
  gRPC.
  - `phtrace.Config` + `FromEnv(opts ...Option)`: env-driven configuration with
    functional options (`WithServiceName`, `WithServiceVersion`, `WithEndpoint`,
    `WithInsecure`, `WithSamplingRatio`, `WithEnvironment`, `WithEnabled`,
    `WithResourceAttribute`). Env vars: `OTEL_ENABLED`, `OTEL_SERVICE_NAME`,
    `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_INSECURE`,
    `OTEL_TRACES_SAMPLER_ARG`, `OTEL_DIAL_TIMEOUT`, `OTEL_BATCH_TIMEOUT`,
    `OTEL_METRIC_EXPORT_INTERVAL`, `OTEL_RESOURCE_ATTRIBUTES`.
  - `phtrace.Init(ctx, cfg) (Shutdown, error)`: one-shot initialization with
    `sync.Once`, OTLP gRPC trace + metric exporters, parent-based trace
    sampling, W3C TraceContext + Baggage propagation, `otel.SetErrorHandler`
    wiring. Returns `Shutdown` closure safe to call multiple times.
  - `phtrace.IsEnabled()`, `phtrace.Tracer(name)`, `phtrace.Meter(name)`,
    `phtrace.Propagator()`, `phtrace.Resource()`: zero-cost helpers that
    return no-op providers when OTel is disabled (so consumer code stays the
    same in dev and prod).
  - `phtrace.AMQPCarrier` + `InjectAMQP(ctx, headers)` / `ExtractAMQP(ctx, headers)`:
    W3C `traceparent` propagation over `amqp091-go` headers for cross-service
    RabbitMQ tracing.
  - `phtrace.PhaseHistogram` + `NewPhaseHistogram(meter, name, buckets)` /
    `MustPhaseHistogram(...)` + `Record` / `Observe`: preconfigured
    millisecond-unit histogram with explicit bucket boundaries tuned for
    QR-MPM phase timing (`qrmpm_phase_duration_ms`). Cached per
    (meterName, histName).
  - **Context-aware log helpers** (`log.go`): `LogDCtx`, `LogICtx`, `LogWCtx`,
    `LogECtx` automatically prepend `[trace_id=... span_id=...]` from the
    ctx's active span. `phtrace.WithFields(ctx, ...)` returns a
    `LogContextCtx` for operation-scoped logging with trace enrichment;
    `LogE`/`LogECtx` additionally call `span.RecordError` on the active span.
  - **Canonical log field keys** for Loki query consistency:
    `FieldTraceID`, `FieldSpanID`, `FieldTicketID`, `FieldReffNo`,
    `FieldMerchantID`, `FieldOrderID`, `FieldTrxID`, `FieldTrxNo`,
    `FieldService`, `FieldRoute`, `FieldVendor`.
- **Tests**: `phtrace/{config,rmqprop,log,metrics}_test.go` cover env parsing,
  defaults, carrier round-trip with the standard TraceContext propagator,
  prefix building with/without spans, and histogram caching / nil-safe
  behavior. All tests pass under `go test -race ./phtrace/...`.

### Dependencies

- Added OpenTelemetry SDK (indirect `go.opentelemetry.io/otel@v1.43.0`):
  `otel`, `otel/trace`, `otel/metric`, `otel/propagation`, `otel/sdk`,
  `otel/sdk/metric`, `otel/exporters/otlp/otlptrace{,grpc}`,
  `otel/exporters/otlp/otlpmetric{,grpc}`, `otel/semconv/v1.26.0`.
- Bumped `google.golang.org/grpc` from v1.76.0 to v1.80.0 transitively.

### Compatibility

- New subpackage only. No existing symbols touched; services that do not
  import `github.com/PayCloud-ID/paycloudhelper/phtrace` are unaffected.
- Backward compatible (MINOR): SemVer MINOR bump to v1.9.0.

## [v1.8.2] - 2026-04-23

### Added

- **Script documentation policy** (`.agents/rules/script-documentation.md`): Added a repository rule requiring a standard header in every shell script (`Purpose`, `Usage`, `Options`, `What It Reads`, `What It Affects / Does`, `Exit Behavior`).

### Changed

- **Script path portability** (`scripts/check-no-direct-s3minio-http.sh`, `scripts/proto/update-s3minio-proto.sh`): Replaced machine-specific absolute paths with relative defaults for local and CI usage.
- **`scripts/generate-makefile.sh`**: Formalized CLI options (`--service-path`, `--dry-run`, `-h/--help`) and added optional post-generation sanity checks (`make -n help`, `bash -n run.sh`).
- **`scripts/run_tests.sh`**: Tightened option handling (`-race`, `-cover`, `-coverprofile`, `-short`, `-v/--verbose`) and now prints `go tool cover -func` summary when `-coverprofile` is used.
- **`scripts/proto/update-s3minio-proto.sh`**: Added `S3MINIO_MANAGER_PROTO` override and changed missing-source behavior to a non-failing skip.
- **`scripts/proto/check-stub-drift.sh`**: Hardened CI drift checks by refreshing proto artifacts and failing when drift is detected.

### Added (Internal)

- **Agent instruction alignment** (`AGENTS.md`, `.github/copilot-instructions.md`): Added explicit references to the script-header documentation rule.

### Changed (Internal)

- **Script doc-header rollout** (`scripts/check-no-direct-s3minio-http.sh`, `scripts/generate-makefile.sh`, `scripts/run_tests.sh`, `scripts/proto/new-service-scaffold.sh`, `scripts/proto/update-s3minio-proto.sh`, `scripts/proto/gen-s3minio-client.sh`, `scripts/proto/check-stub-drift.sh`): Applied the standardized shell-script header format to touched scripts.
- **Planning docs refresh** (`docs/plans/*`): Updated internal execution/planning documents to match script-policy and proto-workflow updates.

## [v1.8.1] - 2026-04-19

### Added

- **`AuditTrailTrx` transaction audit trail** (`audittrail_trx_entities.go`, `audittrail_trx.go`):
  Dedicated transaction lifecycle audit with structured `AuditTrailTrx` type, 15 lifecycle
  state constants (`AuditTrxState*`), 4 status constants (`AuditTrxStatus*`), command constant
  `CmdAuditTrailTrx`, and extensible `Metadata` field.
- **`SetUpAuditTrailTrxPublisher()`**: Creates separate AMQP client + `AuditPublisher` worker pool
  for transaction audit. Supports enable/disable via first parameter and reuses existing
  functional options.
- **`LogAuditTrailTrx(data AuditTrailTrx)`**: One-call audit publishing. Auto-sets `Service`
  from `AppName` and `EventTime` from `time.Now()`. Requires at least one of `ReffNo`/`OrderNo`.
- **`IsAuditTrailTrxEnabled()` / `GetAuditTrailTrxPublisher()`**: Status checks and lifecycle access.
- **Unit tests** (`audittrail_trx_test.go`): Entity constants, publisher setup, enable/disable,
  correlation validation, auto-defaults, metadata passthrough, concurrent safety, unique IDs.

## [v1.8.0-beta.2] - 2026-04-14

### Added

- **Sentry structured logging integration**: Added `ConfigureSentryLogging(enable bool)` as a simple opt-in API to forward all `phlogger` levels (`debug`, `info`, `warn`, `error`, `fatal`) to Sentry via log hooks. Recommended runtime gate is `SENTRY_LOGGING` (`false` by default in consumer services).
- **`SentryLoggingFromEnv()` helper**: New convenience function to load SENTRY_LOGGING environment variable and configure structured logging in one call: `pch.ConfigureSentryLogging(pch.SentryLoggingFromEnv())`.
- **`phsentry/log_hook.go`**: Added `RegisterSentryLogHook()` with `sync.Once` to ensure one-time hook wiring and avoid duplicate forwarding registrations.
- **Sentry SDK structured logs enablement**: `InitSentryOptions` now sets `sentry.ClientOptions.EnableLogs = true` (SDK v0.33.0+ compatible) so log ingestion is available when forwarding is enabled.
- **Documentation refresh**: Added Sentry structured logging usage and environment guidance in `README.md` and `AGENTS.md`, including default-off behavior, `SENTRY_LOGGING` workflow, and relationship with legacy granular `ConfigureLogForwarding` options.

- **Documentation**: README Sentry section (including `SENTRY_DEBUG` / `SentryOptions.Debug` vs `SendSentryDebug`), configuration table note, and godoc on `SentryOptions`, `InitSentry`, `InitSentryOptions`, and debug capture helpers.

- **Service-scoped SDK foundation** (`sdk/services/s3minio/`, `sdk/shared/`): Added Phase 1 service SDK layout for S3MinIO with helper/grpc/http/pb/proto/facade packages and shared runtime placeholder packages for transport, observability, and error normalization.
- **Proto governance baseline** (`buf.yaml`, `buf.gen.yaml`, `Makefile`): Added Buf configuration and Makefile targets for S3MinIO proto lint/breaking workflows plus service-scaffold command surface.
- **SDK scaffold pattern** (`scripts/proto/new-service-scaffold.sh`, `docs/sdk/scaffold-pattern.md`): Added generator-backed scaffold pattern for onboarding future service SDKs under `sdk/services/<service>`.

- **`AuditPublisher` worker pool** (`audittrail_publisher.go`): Production-grade audit publishing with bounded concurrency (default 10 workers), buffered channel (default 1000), circuit breaker (10 failures → 30s cooldown), and configurable message TTL. Functional options: `WithWorkerCount`, `WithBufferSize`, `WithMaxRetries`, `WithPublishTimeout`, `WithMessageTTL`, `WithCircuitBreakerThreshold`, `WithCircuitBreakerCooldown`.
- **`SetUpAuditTrailPublisher()`** (`audittrail_v2.go`): New setup function that creates AMQP client + worker pool. Existing `SetUpRabbitMq` unchanged — services migrate at their own pace.
- **`LogAudittrailDataV2()` / `LogAudittrailProcessV2()`** (`audittrail_v2.go`): Worker-pool-based V2 audit functions. Fall back to legacy V1 goroutine-per-call behavior when publisher is nil.
- **`GetAuditPublisher()`**: Returns the package-level `AuditPublisher` for lifecycle management.

- **Unit tests**
  - Root package: `LockError` (`Error`, `Unwrap`), Redis options (`InitRedisOptions`, `GetTrxRedisBackoff`, `GetTrxRedisLockTimeout`, `GetRedisPoolClient` when not initialized), mutex map (`StoreMutex`, `GetMutex`, `RemoveMutex`), init/app env (`SetAppName`, `SetAppEnv`, `GetAppName`, `GetAppEnv`, `InitializeApp`), validator constants and header validation (idem key, CSRF), `LoggerErrorHub`.
  - `phhelper`: globenv (Get/Set app name and env), helpers (`JsonMinify`, `JsonMarshalNoEsc`, `JSONEncode`, `ToJson`, `ToJsonIndent`).
  - `phjson`: config, `Marshal`, `Unmarshal`, `MarshalIndent`, invalid JSON handling.
  - **Audit trail V1** (`audittrail_test.go`): nil client, empty params, nil data, zero status code, keys handling, JSON structures, rate-limited logging, not-ready early exit, concurrent ID uniqueness.
  - **AMQP client** (`amqp_audit_test.go`): push not-ready, max retries, IsReady state, thread safety, WaitForReady timeout/success, PushWithTTL not-ready.
  - **Audit publisher** (`audittrail_publisher_test.go`): defaults, options, worker pool processing, backpressure, circuit breaker trip/reset, stop/drain idempotency, nil/not-ready client.
  - **Audit trail V2** (`audittrail_v2_test.go`): V1 fallback, empty params early exit, nil data, zero status code, submit to publisher, SetUpAuditTrailPublisher globals, GetAuditPublisher.
- **Scripts**
  - `scripts/run_tests.sh` — run all tests from repo root with options: `-v`, `-race`, `-cover`, `-coverprofile`, `-short`, `-h`.
  - `scripts/generate-makefile.sh` — auto-detect library vs binary service and emit `Makefile` + `run.sh` (optional `proto`, `cicd`/`docker-build`, `db-validate` when applicable).
- **Build**
  - Root `Makefile` — `help`, `deps`, `build`, `vet`, `test`, `test-race`, `test-cover`, `fmt`, `clean`.
  - Root `run.sh` — for this library repo, runs `go test -race ./...` (`--debug` sets `GODEBUG`).
- **CI**
  - GitHub Actions: on every push to active branches, run `buf lint`, `make ci.check.direct-http`, `make ci.check.stub-drift`, `go build ./...`, `go vet ./...`, `go test ./...`. Workflow fails if any step fails.
- **`helper.ProbeObserveFunc` / `ObserveProbe()`** (`sdk/services/s3minio/helper/observe.go`): Optional hook for gRPC health/readiness latency and HTTP status codes without importing `pchelper` from the transport layer; `grpc` adapter calls it after probes.
- **Documentation**
  - README: **Testing** section (run script, options, coverage, code quality); **Verifying the library** (build/vet/test checklist); **CI** (how CI runs and how to require passing checks for merges).
  - `docs/sdk/s3minio-probe-observe-wiring.md`: Example wiring of `helper.ProbeObserveFunc` from a consumer `main` (after logger init).

### Changed

- **Sentry section naming/clarity**: README now distinguishes `Sentry Error Tracking` from `Sentry Structured Logging`, with explicit guidance that `SENTRY_DEBUG` affects SDK diagnostics only (not log forwarding) and `SENTRY_LOGGING` controls structured log forwarding.

- **S3MinIO SDK runtime ownership**: `sdk/services/s3minio/{helper,grpc,http,pb}` now contains direct implementations and tests; runtime no longer depends on legacy package forwarding.
- **Proto tooling paths**: S3MinIO proto update/generation/drift scripts now target only `sdk/services/s3minio/*` paths.
- **S3MinIO proto snapshot** (`sdk/services/s3minio/proto/s3minio.proto`): Re-synced with `paycloud-be-s3minio-manager/proto/s3minio.proto` (comments, field docs, service documentation; no wire changes).
- **S3MinIO proto scripts** (`scripts/proto/update-s3minio-proto.sh`, `gen-s3minio-client.sh`, `check-stub-drift.sh`): `S3MINIO_MANAGER_PROTO` overrides the default manager path; update skips when the source file is missing (e.g. CI without a checkout); `gen-s3minio-client.sh` validates the hand-maintained `pb/client.go` surface via `go test` instead of invoking `protoc` with an incompatible `go_package` output layout; drift detection compares the proto checksum before and after refresh instead of diffing `pb/client.go`.
- **`buf.yaml`**: Lint rules scoped to the `sdk/services/s3minio/proto` module with `STANDARD` minus layout/RPC-naming rules that conflict with the internal parity proto (same wire contract as production).
- **`phhelper` app globals**: `GetAppName` / `SetAppName` / `GetAppEnv` / `SetAppEnv` now use an `RWMutex` so background AMQP workers and tests do not race.
- **`auditTrailMqClient`**: Stored in `atomic.Pointer[AmqpClient]`; `pushMessageAudit` loads the pointer once per publish (safe with async V1 audit goroutines).
- **`ValidateConfiguration`**: `APP_NAME` warning uses `os.Getenv("APP_NAME")`; `APP_ENV` prefers env then falls back to `GetAppEnv()`.

### Removed

- Legacy S3MinIO compatibility packages: `phs3minio/`, `phs3miniogrpc/`, `phs3miniohttp/`, `phs3miniopb/`.

### Fixed

- **S3MinIO gRPC marshaling compatibility** (`sdk/services/s3minio/pb/client.go`): Replaced direct `grpc.Invoke` calls that sent compatibility structs (non-protobuf messages) with an internal bridge to generated wire protobuf stubs (`sdk/services/s3minio/pb/wirepb/*`). This fixes runtime errors like `grpc: error while marshaling: proto: failed to marshal, message is *pb.DownloadRequest, want proto.Message` in consumers resolving profile images via `GetMinIOPresignedUrl`.
- **`go test -race ./...` (root package)**: Audittrail tests vs. async `pushMessageAudit`, atomic AMQP client pointer, and mutex-protected app name/env removed data races; `TestValidateConfiguration` uses `t.Setenv` per subtest instead of `os.Clearenv()`.
- **Audit trail — `Push()` infinite retry (RISK-001)**: `AmqpClient.Push()` now retries at most `PushMaxRetries` (default 3) times with a total timeout of `PushTimeout` (default 15s). Previously it retried forever with 5s delays, causing goroutine leaks under RabbitMQ degradation.
- **Audit trail — ContentType (RISK-006)**: `UnsafePush` now publishes with `ContentType: "application/json"` instead of `"text/plain"`. Non-breaking — consumers don't filter on content type.
- **Audit trail — Id collision (RISK-008)**: Audit message IDs now use an atomic counter (`nextAuditID()`) instead of truncated `time.Now().UnixNano()`, eliminating collisions under high throughput.
- **Audit trail — wasted CPU when not ready (RISK-005)**: `pushMessageAudit` now checks `IsReady()` before JSON marshal. Rate-limited logging (once per 30s) prevents log flooding under sustained RabbitMQ failure.

### Added (Internal)

- `AmqpClient.IsReady() bool` — thread-safe check of connection readiness.
- `AmqpClient.WaitForReady(timeout time.Duration) bool` — blocks until client is ready or timeout expires.
- `AmqpClient.PushWithTTL(data []byte, ttl string) error` — push with configurable message TTL (empty string = no expiration).
- `PushMaxRetries` and `PushTimeout` package-level vars for Push() retry configuration.
- `nextAuditID()` — atomic counter for collision-free audit message IDs.

### Security

- None.

---

## [1.7.1-beta.1] and earlier

History before this changelog was introduced. See git tags and release notes for older versions.

Retracted versions (do not use): v1.6.3, v1.6.0, v1.5.2 — see `go.mod` retract block.

[v1.8.1]: https://github.com/PayCloud-ID/paycloudhelper/compare/v1.8.0-beta.2..v1.8.1
[v1.8.2]: https://github.com/PayCloud-ID/paycloudhelper/compare/v1.8.1..v1.8.2
[v1.8.0-beta.2]: https://github.com/PayCloud-ID/paycloudhelper/compare/v1.7.1-beta.1..v1.8.0-beta.2
[1.7.1-beta.1]: https://github.com/PayCloud-ID/paycloudhelper/tree/v1.7.1-beta.1
