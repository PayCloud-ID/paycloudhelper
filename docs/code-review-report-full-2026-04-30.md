# Security-first Production Readiness Release-Gate Audit (Go) — `paycloudhelper`

**Repository**: `github.com/PayCloud-ID/paycloudhelper`  
**Review date**: 2026-04-30  
**Branch / commit**: `develop` @ `1ea22a49eb14a4673e141f24d28eb6c741744f93`  
**Reviewer mode**: Principal Go reviewer · security-first · release-gate artifact  

---

## Executive Summary

**Overall health score**: **74 / 100** *(updated after concurrency/race/CI, credential logging, CI pinning, and sdk/shared tests — 2026-05-02)*  
**Deployment readiness**: **Conditionally Ready** *(remaining blockers: TLS defaults **SEC-1**, JWT middleware panics **SEC-2/SEC-3**)*  

**Rationale (1 paragraph)**: This repository is a **shared Go library** (not a standalone microservice), but it ships production-critical middleware and infrastructure clients (Redis, RabbitMQ, JWT auth middleware, Sentry, OTel). **Resolved (2026-05-01 / 2026-05-02):** `go test -race ./...` **passes**, CI runs **`-race`** on **pinned** `ubuntu-24.04` + Go **1.25.9**, reconnect loops use **timer reuse** where applicable, legacy RabbitMQ paths return **errors** instead of panics, **`rmq-autoconnect` logs redacted AMQP URIs** (no credentials in log text), **`sdk/shared/*` placeholder packages have tests**, and **Bitbucket Pipelines config removed** (GitHub Actions only). **Still open before full release:** **SEC-1** (`InsecureSkipVerify`), **SEC-2/SEC-3** (JWT handling).

### Finding counts by severity

- **Critical**: 0  
- **High**: 4  
- **Medium**: 5  
- **Low**: 2  
- **Good**: 6

---

## Repository Snapshot

### Go / toolchain

- **Go toolchain (local evidence)**: `go1.25.9 darwin/arm64` (tool output)  
- **`go.mod`**: `go 1.25.0` + `toolchain go1.25.9`

### Module and build targets

- **Module name**: `github.com/PayCloud-ID/paycloudhelper` (`go.mod:1`)
- **Entrypoints**: **No `main` packages found** (search only found `grpc.NewServer()` / `echo.New()` in tests).  
  - **Impact**: This is a **library**, not a deployable microservice. Operational recommendations below are framed for *consumer services*.

### Approximate size

- **Go LOC (all `*.go`)**: ~**20,576** lines (shell `wc -l` total)  
- **Test LOC (`*_test.go`)**: ~**12,166** lines (shell `wc -l` total)  
- **File counts**: **130** Go files, **72** test files (git count)

### Test inventory and quality signals

- `go test ./...`: **PASS** (tool output)
- `go test -cover ./...`: **PASS** with multiple packages reporting coverage; examples:
  - Root package coverage: **86.6%** (tool output)
  - `phjson`: **100%** (tool output)
  - `phsentry`: **65.2%** (tool output)
- `go test -race ./...`: **PASS** *(as of 2026-05-01 after fixes for `CONC-1`, `CONC-2`)*.
- ~~Packages with **no test files**~~: **`sdk/shared/*`** now include placeholder `_test.go` files *(2026-05-02)*.

### Runtime dependencies and external touchpoints (from `go.mod`)

- **HTTP server framework**: `github.com/labstack/echo/v4` (`go.mod:58`)
- **Redis**: `github.com/redis/go-redis/v9` (`go.mod:64`), distributed locks via `github.com/go-redsync/redsync/v4` (`go.mod:11`)
- **RabbitMQ**: `github.com/rabbitmq/amqp091-go` (`go.mod:15`)
- **JWT**: `github.com/golang-jwt/jwt/v5` (`go.mod:56`)
- **Sentry**: `github.com/getsentry/sentry-go` (`go.mod:10`)
- **OpenTelemetry**: multiple `go.opentelemetry.io/otel/*` modules (`go.mod:16–23`)
- **Env loading**: `github.com/joho/godotenv` (`go.mod:12`), executed during package `init()` (`init.go:12–16`, `init.go:52–76`)

### Security-sensitive config/artifacts discovered

- **GitHub Actions**: `.github/workflows/ci.yml` — canonical CI; **`runs-on: ubuntu-24.04`**, **`go-version: '1.25.9'`**, build/vet/test/**`-race`/lint/vulncheck** *(pinned — 2026-05-02)*  
- **No Dockerfile / k8s manifests** found in repo (glob results)  
- ~~**Legacy CI**~~: **`bitbucket-pipelines.yml` removed** — GitHub Actions only *(2026-05-02)*

---

## Findings by Category

### Architecture

#### ARCH-1

- **Severity**: Medium  
- **Confidence**: High Confidence  
- **Title**: This repository is a shared library with `init()` side effects (not a microservice); release gate must treat it as supply-chain critical
- **Evidence**:
  - `init.go:12–16` calls `AddValidatorLibs()`, `InitializeLogger()`, `InitializeApp()` during import-time `init()`
  - Entrypoint scan found no `main` packages; only test constructs use `echo.New()` / `grpc.NewServer()` (pattern scan output)
- **Impact**: Import-time I/O/config side-effects can surprise ~30 consumer services (per repo `AGENTS.md`) and make failures appear as “random startup” issues. This also complicates safe configuration management and deterministic testing for consumers.
- **Remediation**:
  - Keep `init()` if it’s contractually required, but ensure side effects are **non-failing, bounded, and observable**.
  - Consider introducing an explicit `Initialize()` entrypoint for consumers and **deprecating import-time behavior** over a release cycle (Needs Verification: backward-compat constraints across consumers).
- **Validation**:
  - Add a consumer-style test that imports the module and asserts `init()` does not perform network calls, does not panic, and logs only at debug level by default.

---

### Security

#### SEC-1

- **Severity**: High  
- **Confidence**: High Confidence  
- **Title**: RabbitMQ TLS verification is explicitly disabled (`InsecureSkipVerify: true`)
- **Evidence**:
  - `amqp.go:163–170` sets `TLSClientConfig: &tls.Config{InsecureSkipVerify: true}`
  - `phaudittrailv0/audittrail-mq-v0.go:91–98` sets `TLSClientConfig: &tls.Config{InsecureSkipVerify: true}`
- **Impact**: Disabling certificate verification enables **man-in-the-middle attacks** (credential theft, message tampering, audit trail poisoning) on any TLS-enabled AMQP connection. In regulated/payment contexts, this is typically unacceptable.
- **Remediation** (recommended order):
  - Change defaults to **verify TLS**. Provide an **explicit opt-in** escape hatch for non-prod (e.g., functional option or config flag) that is **off by default**.
  - Ensure server name / CA roots are configurable (e.g., `RootCAs`, `ServerName`) rather than bypassing verification.
- **Validation**:
  - Unit test: confirm default config does **not** set `InsecureSkipVerify`.
  - Integration (optional): connect to a test broker with a self-signed cert and verify connection fails unless explicit opt-in is enabled.

#### SEC-2

- **Severity**: High  
- **Confidence**: High Confidence  
- **Title**: `RevokeToken` can panic on non-RSA JWT signing method due to unsafe type assertion in log path (remote DoS)
- **Evidence**:
  - `revoke-token.go:62–66`:
    - Checks `if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok { ... }`
    - Inside the `!ok` branch it logs `token.Method.(*jwt.SigningMethodRSA)` (`revoke-token.go:64`) which will **panic** when `ok == false`
- **Impact**: A client can send a JWT with a non-RSA `alg` (or malformed token method) and trigger a **panic in the request path**, crashing the service (availability incident).
- **Remediation**:
  - Replace the log field with a safe representation (e.g., `token.Method.Alg()` or `%T`), and never type-assert in the failure branch.
  - Add a unit test covering non-RSA methods to ensure the middleware returns `401` without panicking.
- **Validation**:
  - `go test ./...`
  - `go test -race ./...` (after concurrency fixes)  
  - Add a test: token with `SigningMethodHS256` must not panic and must return Unauthorized.

#### SEC-3

- **Severity**: High  
- **Confidence**: High Confidence  
- **Title**: `RevokeToken` can panic on missing/malformed `Expired` claim (unchecked type assertion)
- **Evidence**:
  - `revoke-token.go:90` does `tokenClaim["Expired"].(string)` without checking presence/type
  - Parsing error is ignored: `timeData, _ := time.Parse(...)` (`revoke-token.go:90`)
- **Impact**: If a token is validly signed but has unexpected claim types (or missing `Expired`), the middleware can panic → **remote DoS**. Ignoring the parse error also makes expiry enforcement unreliable.
- **Remediation**:
  - Safely fetch and validate claims:
    - Check `Expired` exists and is a string (or use standard `exp` claim handling via `jwt.RegisteredClaims`).
    - Handle `time.Parse` errors; treat invalid/missing expiry as unauthorized.
  - Add tests for: missing `Expired`, invalid format, and wrong type.
- **Validation**:
  - New unit tests in `middleware_revoke_jwt_test.go` (or similar) asserting `401` and no panic for malformed claims.

#### SEC-4

- **Severity**: Medium  
- **Confidence**: High Confidence  
- **Title**: AMQP connection strings include credentials and are logged in cleartext
- **Evidence**:
  - `rmq-autoconnect.go:165–167` builds `amqp://%s:%s@...` and logs it: `LogI("%s connection=%s", ..., connection)` (`rmq-autoconnect.go:166`)
- **Impact**: Logs can leak RabbitMQ usernames/passwords into log aggregation, Sentry breadcrumbs, or incident tooling. This increases blast radius during compromise and violates common secret-handling policies.
- **Remediation**:
  - Redact credentials in logged connection strings (e.g., log host/port/vhost only).
  - Add a unit test that asserts logs do not contain the password (Needs Verification: logging test harness available in repo).
- **Validation**:
  - `go test ./...` with a test ensuring the formatted log line does not contain the password.

**Resolution (2026-05-02):** **Resolved.** `startConnection` logs `redactAMQPURIForLog(host, port, vhost)` (`amqp://***:***@host:port/vhost`) instead of the dial URI; full URI remains in memory for `connect` only. Tests: `TestRedactAMQPURIForLog`, `TestRMqAutoConnect_startConnection_logDoesNotLeakCredentials` (`rmq_autoconnect_test.go`).

---

### Error Handling & Resilience

#### RES-1

- **Severity**: High  
- **Confidence**: High Confidence  
- **Title**: Library code calls `log.Panicln` / panics directly in infrastructure paths, risking process termination
- **Evidence**:
  - `rmq-autoconnect.go:163–171` panics on connection error (`log.Panicln(err.Error())`)
  - `phaudittrailv0/audittrail-mq-v0.go:129–136` panics on channel open error (`log.Panicln(err.Error())`)
  - `phtrace/metrics.go:70–76` panics in `MustPhaseHistogram` on instrument creation errors
- **Impact**: Panics in a shared library can crash consumer services in production, including during transient network failures or instrumentation init errors. This is a reliability and incident-risk multiplier across all dependent services.
- **Remediation**:
  - Replace panics with returned errors wherever possible (especially network/IO paths).
  - If a “must” helper is needed, ensure it is used only in explicitly controlled startup contexts by consumers (and document it).
  - For `rmq-autoconnect.go`, prefer returning error from `startConnection()` and letting caller decide restart/backoff.
- **Validation**:
  - Add tests to assert connection/channel failures return errors rather than panicking.
  - Run `go test -race ./...` after concurrency refactors (see `CONC-1`, `CONC-2`).

**Resolution (2026-05-01):** **Partially addressed.**  

- `**rmq-autoconnect`**: `startConnection` returns `error`; channel open failures return `error` instead of `LogF`/panic; reconnect loop uses mutex + `WaitGroup` (see `CONC-2`).  
- `**phaudittrailv0`**: channel open failure returns `fmt.Errorf("audit trail channel: …")` instead of `log.Panicln`.  
- `**phtrace.MustPhaseHistogram`**: panic retained for intentional startup-only registration failures; documentation updated to direct callers to `NewPhaseHistogram` for recoverable setup (not a network-path fix).

#### RES-2

- **Severity**: Medium  
- **Confidence**: High Confidence  
- **Title**: `init()` loads `.env` by searching CWD/parents/executable path (risk of config drift and surprising behavior)
- **Evidence**:
  - `init.go:18–49` searches `.env` in multiple locations (ENV_FILE, CWD, parent dirs up to 5 levels, executable dir)
  - `init.go:52–61` loads `.env` during `InitializeApp()` called from `init()` (`init.go:12–16`)
- **Impact**: Production services can accidentally load unintended `.env` files based on runtime working directory or packaging layout, causing silent misconfiguration (especially in containerized workloads or when running tests/tools).
- **Remediation**:
  - Default to **not loading `.env`** unless explicitly enabled (e.g., ENV_FILE set), or restrict search behavior to development-only (Needs Verification: existing consumer expectations).
  - At minimum, add a warning (not debug) when a `.env` was found and loaded, including the path (without leaking contents).
- **Validation**:
  - Tests in `init_findenv_test.go` should include a “parent dir `.env` present” scenario and assert expected behavior in production mode (Needs Verification: how prod mode is determined).

---

### Code Quality & Maintainability

#### QUAL-1

- **Severity**: Medium  
- **Confidence**: High Confidence  
- **Title**: Environment access appears in multiple non-`InitializeApp()` files, conflicting with repo convention and increasing config coupling
- **Evidence**:
  - `rmq-autoconnect.go:76–80` reads `os.Getenv("APP_NAME")` for connection metadata
  - `revoke-token.go:67–68` reads `os.Getenv("APP_PUBLIC_KEY")` inside request-path JWT parsing
  - `config.go:26–117` reads multiple env vars for validation (this may be acceptable as validation, but it expands env coupling beyond `InitializeApp()`)
- **Impact**: Configuration becomes scattered, harder to validate centrally, and easier to drift across services. Request-path `Getenv` can also make behavior dependent on runtime env mutations and complicate testing.
- **Remediation**:
  - Plumb required configuration through explicit initialization/config setters (e.g., store parsed RSA key during init rather than reading per request).
  - Keep env reads centralized; `config.go` can validate a cached config struct rather than reading env directly.
- **Validation**:
  - Unit tests verifying config is read/validated once at startup and middleware uses cached values.

---

### Performance & Concurrency

#### CONC-1

- **Severity**: High  
- **Confidence**: High Confidence  
- **Title**: `go test -race ./...` fails due to data race on package-level test override variables used by reconnect loops
- **Evidence**:
  - Race detector report shows concurrent read/write of a shared address tied to `amqpReconnectSleep()` reading `amqpReconnectDelayForTest` while tests clean up and write it.
  - Code paths:
    - `amqp.go:120–133` `amqpReconnectDelayForTest` read without synchronization in `amqpReconnectSleep()`
    - `amqp.go:182` starts goroutine `go c.handleReconnect(addr)`
    - `amqp_audit_test.go:602–615` cleanup writes `amqpReconnectDelayForTest = prevBackoff` while goroutine can still read (`amqp_audit_test.go` excerpt includes cleanup + close(done))
- **Impact**: Release gate should not accept a library that fails `-race`; additionally, consumer services *could* override these package-level knobs at runtime and trigger undefined behavior (especially in highly concurrent services).
- **Remediation**:
  - Make test-only override knobs **thread-safe**:
    - Use `atomic.Value` / typed atomics, or guard with a mutex, or make them instance-scoped on `AmqpClient`.
  - Ensure `handleReconnect` goroutine has a deterministic shutdown and tests wait for it to stop before mutating globals.
- **Validation**:
  - `go test -race ./...` must pass locally and in CI.

**Resolution (2026-05-01):** **Resolved.** AMQP test delay overrides (`amqpReconnectDelayForTestNs`, `amqpReinitDelayForTestNs`, `amqpResendDelayForTestNs`) are stored in `**sync/atomic`** types so reconnect goroutines and `t.Cleanup` do not race. All references in tests updated. Verification: `go test -race ./...` **PASS**.

#### CONC-2

- **Severity**: High  
- **Confidence**: High Confidence  
- **Title**: RabbitMQ auto-reconnect (`rMqAutoConnect`) mutates shared connection/channel state across goroutines without synchronization; `-race` detects it
- **Evidence**:
  - Race detector report references:
    - `rmq-autoconnect.go:59–61` `reset()` reads/closes `r.ch` / `r.conn`
    - `rmq-autoconnect.go:81–82` writes `r.conn` during connect
    - `rmq-autoconnect.go:188–205` reconnect goroutine calls `reset()`, `connect()`, `DeclareQueues()`, and reassigns `notifCloseCh`
  - Test exercising this path:
    - `rmq_autoconnect_test.go:141–183` starts reconnect loop and calls `r.stop()` while goroutine is active
- **Impact**: In real services, concurrent stop/reconnect/declare operations can corrupt internal state, leak goroutines, double-close channels, or publish on a closed channel. These can manifest as intermittent outages.
- **Remediation**:
  - Introduce a mutex protecting `conn`, `ch`, `notifCloseCh`, and `declaredQueues`, or redesign with a single owner goroutine and message passing.
  - Ensure `Stop()` waits for reconnect goroutine to exit (e.g., `WaitGroup`) before `reset()` closes resources.
- **Validation**:
  - Add `go test -race ./...` to CI.
  - Add targeted tests asserting no goroutine leaks and no data races around stop/reconnect.

**Resolution (2026-05-01):** **Resolved.** `rMqAutoConnect` fields (`conn`, `ch`, `notifCloseCh`, URI) are guarded by `**sync.Mutex`**; `**stop()`** cancels the reconnect context, `**WaitGroup**` waits for the listener goroutine, then closes resources; reconnect path uses `**resetLocked()**`; nil `conn` spins briefly until reconnected. `**startConnection**` returns `**error**`. Verification: `go test -race ./...` **PASS**.

#### PERF-1

- **Severity**: Medium  
- **Confidence**: Medium Confidence  
- **Title**: `time.After` in reconnect loops can create timer churn; long backoffs risk resource waste under prolonged outages
- **Evidence**:
  - `amqp.go:216–220` uses `time.After(amqpReconnectSleep())` in loop
  - `rmq-autoconnect.go:85–94` uses `<-rmqAfterHook(...)` with up to 1 hour delays in a loop
- **Impact**: Under repeated failures, allocating many timers can create memory/GC churn. Long unbounded reconnect loops without jitter/backoff policies can also amplify outage behavior.
- **Remediation**:
  - Consider `time.NewTimer` reuse in hot loops.
  - Implement bounded exponential backoff with jitter (and a cap), and expose observability metrics for reconnect attempts.
- **Validation**:
  - Load test or benchmark reconnect loops (Needs Verification: existing perf harness).

**Resolution (2026-05-01):** **Partially addressed.**  

- `**amqp.go`**: `handleReconnect` and `handleReInit` reuse one `time.Timer` via `Reset` for reconnect and init-retry sleeps; `handleReInit` can share that timer with `handleReconnect` through an optional pointer argument (see `handleReInit` signature in `amqp.go`).  
- `**rmq-autoconnect`**: production backoff still uses injectable `rmqAfterHook` (tests rely on instant stubs). Full jittered exponential backoff remains future work.

---

### Testing & Coverage

#### TEST-1

- **Severity**: High  
- **Confidence**: High Confidence  
- **Title**: CI does not run `go test -race ./...`, allowing concurrency regressions to ship
- **Evidence**:
  - `.github/workflows/ci.yml:41–48` runs `go build`, `go vet`, `go test ./...` but no `-race`
- **Impact**: Concurrency bugs (including ones already present per `CONC-1`/`CONC-2`) are not caught in the default gate and can ship into many services.
- **Remediation**:
  - Add a CI job/step: `go test -race ./...` (with `-run`/package filtering only if runtime becomes prohibitive).
- **Validation**:
  - Confirm CI runs and passes `-race` on PRs.

**Resolution (2026-05-01):** **Resolved.** GitHub Actions `.github/workflows/ci.yml` includes `go test -race ./...` after the standard test step. Verification: `go test -race ./...` **PASS** locally.

#### TEST-2

- **Severity**: Medium  
- **Confidence**: High Confidence  
- **Title**: Some shared SDK packages have no tests (`sdk/shared/`*)
- **Evidence**:
  - Tool output lists `sdk/shared/errors`, `sdk/shared/observability`, `sdk/shared/transport` as `[no test files]`
- **Impact**: These packages are likely used as cross-service glue; untested error normalization/transport helpers can introduce subtle regressions that are hard to detect downstream.
- **Remediation**:
  - Add unit tests focusing on stable contracts: error wrapping/normalization, headers/metadata propagation, timeout behavior (Needs Verification: actual exported surfaces).
- **Validation**:
  - `go test ./...` and verify those packages are no longer “no test files”.

**Resolution (2026-05-02):** **Resolved (placeholder coverage).** Added `errors_test.go`, `observability_test.go`, `transport_test.go` documenting reserved namespaces; packages currently contain only `doc.go`. Future exported helpers should get contract-focused tests.

---

### Dependencies & Supply Chain

#### SUP-1

- **Severity**: Medium  
- **Confidence**: High Confidence  
- **Title**: CI runner and Go version resolution are not fully pinned (reproducibility risk)
- **Evidence**:
  - `.github/workflows/ci.yml:16` uses `runs-on: ubuntu-latest`
  - `.github/workflows/ci.yml:24` uses `go-version: '1.25.x'`
- **Impact**: Builds can change behavior when GitHub updates `ubuntu-latest` or when new Go patch versions are released, introducing non-deterministic failures close to release windows.
- **Remediation**:
  - Pin runner to a specific Ubuntu version (e.g., `ubuntu-24.04`) and consider pinning Go to `1.25.9` to match `go.mod` toolchain.
- **Validation**:
  - Re-run CI and ensure it uses the pinned versions.

**Resolution (2026-05-02):** **Resolved.** `.github/workflows/ci.yml` uses `runs-on: ubuntu-24.04` and `go-version: '1.25.9'` on all jobs (`validate`, `extended-checks`, `cleanup-merged-branches`), matching `go.mod` toolchain.

#### SUP-2

- **Severity**: Low  
- **Confidence**: High Confidence  
- **Title**: Dual CI configs exist (GitHub Actions + Bitbucket Pipelines), creating drift risk
- **Evidence**:
  - `.github/workflows/ci.yml` exists
  - `bitbucket-pipelines.yml` exists
- **Impact**: Teams may assume one pipeline is authoritative; drift can allow issues to slip through if different gates are enforced over time.
- **Remediation**:
  - Decide which pipeline is canonical and archive/remove the other, or keep both but enforce identical steps.
- **Validation**:
  - Confirm the canonical CI is documented in `README.md` (Needs Verification: current documentation).

**Resolution (2026-05-02):** **Resolved.** Removed `bitbucket-pipelines.yml`; **GitHub Actions** is the only CI. `README.md` section **CI (GitHub Actions)** documents the workflow and branch-protection expectations.

---

### Environment & Secrets

#### ENV-1

- **Severity**: Medium  
- **Confidence**: High Confidence  
- **Title**: Configuration validation is warning-heavy and allows empty defaults for critical identity fields (`APP_NAME`, `APP_ENV`)
- **Evidence**:
  - `config.go:26–44` warnings for empty `APP_NAME` / `APP_ENV` “using empty default”
  - `init.go:64–72` sets global app name/env only if env vars are present (otherwise empty)
- **Impact**: Empty service identity degrades logging, metrics, audit trail connection names, and can cause key namespace collisions in shared caches (depending on usage).
- **Remediation**:
  - For production (`APP_ENV=production`), consider making missing `APP_NAME` an **error** rather than warning (Needs Verification: consumer rollout tolerance).
- **Validation**:
  - Add tests in `config_test.go` asserting expected behavior by environment.

#### ENV-2

- **Severity**: Low  
- **Confidence**: High Confidence  
- **Title**: Redis password validation warns but does not fail-fast when password is missing
- **Evidence**:
  - `config.go:71–77` warns: “Redis password not set - may fail with protected Redis instances”
- **Impact**: This can cause late failures at runtime; however, in some deployments Redis is intentionally unauthenticated (so failing-fast could be wrong).
- **Remediation**:
  - Keep as warning, but clarify in documentation how to configure authenticated Redis and when missing password is acceptable.
- **Validation**:
  - Documented configuration examples + unit tests around `ValidateConfiguration()`.

---

## Detailed File Reference Matrix


| File                                 | Line(s)               | Finding ID(s) |
| ------------------------------------ | --------------------- | ------------- |
| `amqp.go`                            | 163–170               | SEC-1         |
| `amqp.go`                            | 120–133, 182          | CONC-1        |
| `amqp_audit_test.go`                 | 602–615               | CONC-1        |
| `phaudittrailv0/audittrail-mq-v0.go` | 91–98                 | SEC-1         |
| `rmq-autoconnect.go`                 | 199–207               | SEC-4 *(resolved)* |
| `rmq-autoconnect.go`                 | 59–61, 81–82, 188–205 | CONC-2        |
| `rmq-autoconnect.go`                 | 163–171               | RES-1         |
| `rmq_autoconnect_test.go`            | 141–183               | CONC-2        |
| `revoke-token.go`                    | 62–66                 | SEC-2         |
| `revoke-token.go`                    | 90–95                 | SEC-3         |
| `phtrace/metrics.go`                 | 70–76                 | RES-1         |
| `.github/workflows/ci.yml`           | 41–48                 | TEST-1        |
| `init.go`                            | 12–16, 18–61          | ARCH-1, RES-2 |
| `config.go`                          | 26–44, 71–77          | ENV-1, ENV-2  |


---

## Remediation Plan (3 phases)

### Immediate (blockers / pre-release)

- **SEC-1**: Remove `InsecureSkipVerify: true` defaults in `amqp.go` and `phaudittrailv0/audittrail-mq-v0.go`; add explicit opt-in if needed.
- **SEC-2 / SEC-3**: Fix panic-able JWT parsing/logging in `revoke-token.go` and add regression tests.
- ~~**CONC-1 / CONC-2**~~: **Done (2026-05-01)** — atomics for AMQP test delays; `rmq-autoconnect` mutex + `WaitGroup`; `go test -race ./...` passes.
- ~~**TEST-1**~~: **Done (2026-05-01)** — `go test -race ./...` in GitHub Actions.

### Short Term (next sprint)

- ~~**RES-1**~~: **Partially done (2026-05-01)** — RabbitMQ paths return errors; `MustPhaseHistogram` documented as startup-only (panic retained by design).
- ~~**SEC-4**~~: **Done (2026-05-02)** — AMQP connection logs use redacted URI (`***:***@host:port/vhost`); tests assert no credential leak.
- **RES-2**: Revisit `.env` loading behavior to reduce surprise in production.

### Medium Term (next quarter)

- ~~**TEST-2**~~: **Done (2026-05-02)** — placeholder tests for `sdk/shared/*` (packages still doc-only; expand when APIs land).
- ~~**SUP-1**~~ / ~~**SUP-2**~~: **Done (2026-05-02)** — pinned `ubuntu-24.04` + Go `1.25.9`; removed Bitbucket pipeline; README documents GitHub Actions.
- Strengthen “production profile” config validation rules for service identity (**ENV-1**) (Needs Verification: rollout).

---

## Positive Observations

#### GOOD-1

- **Severity**: Good  
- **Confidence**: High Confidence  
- **Title**: Strong unit test presence and meaningful coverage signal across many packages
- **Evidence**: `go test -cover ./...` reports e.g. root package `86.6%` coverage (tool output)
- **Impact**: Reduces regression risk and increases confidence in refactors.
- **Remediation**: Keep coverage gate; **`sdk/shared/*`** now have minimal tests *(2026-05-02)* — extend when exported helpers ship.
- **Validation**: Continue running `go test -cover ./...` in CI.

#### GOOD-2

- **Severity**: Good  
- **Confidence**: High Confidence  
- **Title**: CI enforces build + vet + tests and a minimum coverage gate
- **Evidence**: `.github/workflows/ci.yml:41–52`
- **Impact**: Prevents many correctness regressions.
- **Remediation**: ~~Add `-race` and pin versions~~ — done *(2026-05-01 / 2026-05-02)*.
- **Validation**: CI green across PRs.

#### GOOD-3

- **Severity**: Good  
- **Confidence**: High Confidence  
- **Title**: Proto/tooling hygiene via Buf lint in CI
- **Evidence**: `.github/workflows/ci.yml:27–34`, `buf.yaml`, `buf.gen.yaml`
- **Impact**: Keeps generated/client stubs consistent.
- **Remediation**: None.
- **Validation**: `buf lint` remains in CI.

#### GOOD-4

- **Severity**: Good  
- **Confidence**: High Confidence  
- **Title**: Configuration validation exists and surfaces warnings early
- **Evidence**: `config.go:19–158`, invoked during app initialization `init.go:74–76`
- **Impact**: Improves operational visibility of misconfiguration.
- **Remediation**: Strengthen production strictness.
- **Validation**: Tests in `config_test.go`.

---

## Open Questions / Assumptions

### Facts (verified)

- This is a **Go library** module (`go.mod:1`) with import-time initialization (`init.go:12–16`).
- `go test -race ./...` **passes** as of 2026-05-01 (after fixes for `CONC-1` / `CONC-2`).
- **`rmq-autoconnect` logs redacted AMQP URIs** (no username/password in log text) as of 2026-05-02 (**SEC-4**).
- TLS verification is disabled in RabbitMQ configs (`amqp.go:163–170`, `phaudittrailv0/audittrail-mq-v0.go:91–98`).
- JWT revocation middleware contains panic-able code paths (`revoke-token.go:62–66`, `revoke-token.go:90`).

### Assumptions (explicit)

- Consumer services rely on these defaults for RabbitMQ and JWT middleware; changing defaults must be done with backward-compatibility in mind.

### Unknowns / Needs Verification

- Whether RabbitMQ connections are intended to be TLS-enabled in production for all consumers (and if so, expected CA distribution strategy).
- Whether `.env` loading in `init()` is a hard requirement for existing consumers in production deployments.
- Whether `rmq-autoconnect.go` is actively used by many consumers vs legacy (`phaudittrailv0`) paths.

---

## Pre-output self-check (audit)

- All **High** findings include exact file+line evidence: **Yes** (`SEC-1`, `SEC-2`, `SEC-3`, `RES-1`, `CONC-1`, `CONC-2`, `TEST-1`).  
- No Critical/High claims without file-line references: **Yes**.  
- Evidence → Impact → Remediation → Validation included for each finding: **Yes**.  
- Duplicates removed: **Yes** (e.g., `-race` failure described once with two concrete race vectors).  
- Facts vs assumptions vs unknowns separated: **Yes**.

