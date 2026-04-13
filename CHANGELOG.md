# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Added

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
  - Bitbucket Pipelines (`bitbucket-pipelines.yml`): on every push to `develop` and `main` (and default branches), run `buf lint`, `make ci.check.direct-http`, `make ci.check.stub-drift`, `go build ./...`, `go vet ./...`, `go test ./...`. Pipeline fails if any step fails.
- **`helper.ProbeObserveFunc` / `ObserveProbe()`** (`sdk/services/s3minio/helper/observe.go`): Optional hook for gRPC health/readiness latency and HTTP status codes without importing `pchelper` from the transport layer; `grpc` adapter calls it after probes.
- **Documentation**
  - README: **Testing** section (run script, options, coverage, code quality); **Verifying the library** (build/vet/test checklist); **CI (Bitbucket Pipelines)** (how CI runs and how to require passing pipeline for merges).
  - `docs/sdk/s3minio-probe-observe-wiring.md`: Example wiring of `helper.ProbeObserveFunc` from a consumer `main` (after logger init).

### Changed

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

## [1.8.0] and earlier

History before this changelog was introduced. See git tags and release notes for older versions.

Retracted versions (do not use): v1.6.3, v1.6.0, v1.5.2 — see `go.mod` retract block.

[Unreleased]: https://bitbucket.org/paycloudid/paycloudhelper/compare/v1.8.0..HEAD
[1.8.0]: https://bitbucket.org/paycloudid/paycloudhelper/src/v1.8.0
