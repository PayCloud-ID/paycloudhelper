---

name: Merged Go coverage 90%
overview: >-
  Raise merged go test -short statement coverage (Makefile test-coverage /
  test-coverage-check) from the current baseline toward COVERAGE_GOAL=90% by
  prioritizing high-statement packages, hooks and fakes, and optional CI scope
  levers. Authoritative metric is the total line in coverage-func.txt from
  make coverage-inventory.
todos:

- id: phase0-instrument
content: >-
Regenerate coverage-func.txt (make coverage-inventory); prioritize files
by uncovered statements; confirm COVERAGE_PKGS scope vs full ./...
status: pending
- id: phase1-root-amqp-redis-audit
content: >-
Root amqp lifecycle (Cc hooks, reconnect), redis legacy paths
(StoreRedisWithLock, GetRedisClient, InitializeRedis), audit trail
publisher seams or fakes; avoid testing LogF (fatal / os.Exit)
status: pending
- id: phase1-root-middleware-helpers
content: >-
csrf, idempotency-key, headers, health, init — table tests, httptest,
miniredis where applicable
status: pending
- id: phase1-root-logger
content: >-
logger.go non-fatal delegators; exclude or shim LogF from the 90%
definition unless product approves injectable exit
status: pending
- id: phase2-phtrace
content: >-
Init + OTLP with in-process or short-timeout collector; FromEnv option
coverage; LogContextCtx methods; MustPhaseHistogram / meter helpers
status: pending
- id: phase2-phsentry
content: >-
SendToSentry* with captureTransport; log hook paths; avoid Init failures
that call LogF without test doubles
status: pending
- id: phase2-phlogger
content: >-
sampler, ratelimit, forward branches; do not invoke LogF in CI tests
status: pending
- id: phase3-s3minio
content: >-
http client branches; grpc adapter parity; wirepb bufconn + error paths;
facade delegation
status: pending
- id: phase4-legacy-policy
content: >-
phaudittrailv0 dial injection or keep excluded from default COVERAGE_PKGS;
sdk/shared when code lands
status: pending
- id: phase5-ratchet-ci
content: >-
After sustained +3–5% gains bump COVERAGE_MIN (Makefile + Bitbucket);
optional test-coverage-integration nightly; go test -race on concurrency
status: pending
isProject: false

---

# Plan: merged Go test coverage to 90%

## Baseline and gate (verify in repo)


| Item                | Location / command                                                         |
| ------------------- | -------------------------------------------------------------------------- |
| Merged total        | `make coverage-inventory` → `coverage-func.txt` line `total:`              |
| CI gate             | `make test-coverage-check` — `COVERAGE_MIN`, `COVERAGE_GOAL` in `Makefile` |
| `-coverpkg` default | `go list ./...` excluding `phaudittrailv0` and `sdk/shared/`*              |


**Note:** Per-package `coverage: X% of statements in [...]` lines from `go test` are **merged-set** attribution, not package-local percentage. Use `**total:`** for progress.

## Gap analysis (current repository)

Typical merged total after recent work: **~64–65%**. Remaining gap to **90%** is on the order of **25 percentage points** — expect **many PRs**, not one sweep.

### Highest leverage areas

1. **Root (`paycloudhelper`)** — `logger.go` (many delegators; `**LogF` must stay untested** in normal CI), `amqp.go` (lifecycle, `**Cc`**, reconnect), `redis.go` (legacy `InitializeRedis`, `GetRedisClient`, `StoreRedisWithLock`), `audittrail*.go` / publisher (async + Rabbit), `sentry.go`, `rmq-autoconnect.go`, `init.go`, middleware and helpers (`csrf`, `idempotency-key`, `headers`, `health`).
2. `**phtrace`** — `Init`, OTLP builders, `Handle`, metric APIs; many lines only reachable with **OTLP receiver** or **very short dial timeouts** + `t.Setenv`.
3. `**phsentry`** — `captureWithBreadcrumb` and send paths; test with `**sentry.NewClient` + custom `Transport`** (no network).
4. `**phlogger`** — Same `**LogF` / `emitF`** constraint as root.
5. `**sdk/services/s3minio/`*** — HTTP, gRPC, wirepb, facade — extend **httptest**, **fake gRPC**, **bufconn**.

### Policy levers

- `**phaudittrailv0`:** Default exclusion keeps merged % meaningful; full `./...` at 90% needs **dial injection** or **broker integration**.
- `**LogF`:** Do not call from tests; optionally document **excluded from coverage goal** or add **build-tag / injectable exit** only with sign-off.

## Phased execution

### Phase 0 — Instrumentation and scope

1. Run `**make coverage-inventory`** before each prioritization pass.
2. Choose gate scope: **default `COVERAGE_PKGS*`* (recommended for CI) vs `**./...**`.
3. Optional: `**COVERAGE_PKGS_CORE**` for a stricter subset; document in `README.md`.

### Phase 1 — Root package

- **AMQP:** Same-package **test hooks** (pattern established for publish/consume/close/`queuePassiveForTest`); cover `**Cc`** via **close hooks** without a real broker.
- **Redis:** Cover legacy wrappers or deprecate with tests on the supported path only.
- **Audit trail:** Inject or fake **publisher / AMQP**; cover **circuit breaker** and **buffer full** deterministically.
- **Middleware / helpers:** **Table + httptest + miniredis** (extend existing patterns).

### Phase 2 — phsentry / phtrace / phlogger

- **phsentry:** `**SendToSentryMessage` / `Error` / `Warning` / `Debug` / `Event`** with **in-memory transport**.
- **phtrace:** `**FromEnv` + all `With*` options**; `**LogContextCtx`** methods; `**MustPhaseHistogram`**; `**Init`** behind short timeout + noop collector when feasible.
- **phlogger:** Branch coverage without `**LogF`**.

### Phase 3 — s3minio

- More **HTTP** status and body errors; **gRPC** nil/error paths; **wirepb** getters and RPCs; **facade** wiring.

### Phase 4 — Legacy

- `**phaudittrailv0`:** Fake dial or stay excluded.
- `**sdk/shared/`*:** Tests when packages contain real code.

### Phase 5 — Ratchet and CI

- Bump `**COVERAGE_MIN`** after **+3–5%** sustained improvement (Makefile + `bitbucket-pipelines.yml`).
- Optional `**make test-coverage-integration`** without `-short` for nightly.
- `**go test -race ./...`** for init / goroutine / pool changes.

## Constraints

- Follow `**AGENTS.md`**: prefer **same-package hooks** or **functional options** over breaking exports.
- **Never call `LogF` from tests** (process exit).

## Outcome

**90% merged** with the current broad `-coverpkg` set is a **multi-iteration** program. Staged gates, exclusions, and integration jobs are valid levers alongside unit tests.

---

## Session progress (2026-04-25)

- **Plan file:** recreated with valid YAML `todos` and the analysis above.
- **Implemented (first slice):**
  - **AMQP `Cc`:** `ccChannelCloseForTest` / `ccConnCloseForTest` hooks + unit test (no broker).
  - **phsentry:** `TestSendToSentryCapturePaths_WithClient` — exercises `captureWithBreadcrumb` via `SendToSentryMessage` / `Error` / `Warning` / `Debug` / `Event` with existing `captureTransport`.
  - **phtrace:** `TestFromEnv_AllFunctionalOptions`; `LogContextCtx` `Ctx` + `LogD`/`LogI`/`LogW`/`LogE` no-panic test; `TestMustPhaseHistogram`.
- **Merged coverage check:** **~66.0%** statements (`make test-coverage-check`, default `COVERAGE_PKGS`).
- **Next slices (see todos):** root `redis`/`audittrail`/`rmq-autoconnect`, more `phtrace.Init`, s3minio HTTP/facade, ratchet `COVERAGE_MIN` when stable above ~70%.

### Session progress (2026-04-25 — continuation)

- **Redis (miniredis):** `InitializeRedis`, `GetRedisClient`, `DeleteRedis` /
`DeleteRedisWithContext`, `StoreRedisWithLock`, `AcquireLock` / `ReleaseLock`,
`AcquireLockWithRetry` / `ReleaseLockWithRetry`, `ReleaseLock` unknown key
(`redis_miniredis_extended_test.go`).
- **phtrace:** `Init` with `Enabled: false` noop shutdown path
(`init_lifecycle_test.go`). **Note:** `Init` uses `sync.Once` — avoid adding a
second `Init` test that expects a fresh provider without a test-only reset.
- **Merged `total:` after this slice:** **68.8%** (`coverage-func.txt` / `make test-coverage-check`).

### Session progress (2026-04-25 — toward 75%)

- **Redis:** `ErrTaken` handling in `AcquireLock` / `AcquireLockWithRetry`;
`redis_concurrent_test.go` for `InitRedSyncOnce` and mutex map; extended
miniredis tests (see continuation block above).
- **HTTP (s3minio):** `Health` OK, `GenerateViewURL` / `View` / `Upload` success,
`DownloadFile` and `Download` error paths (`client_branches_test.go`).
- **Middleware:** `VerifCsrf` missing `X-Xsrf-Token` returns **400**.
- **phtrace:** `Propagator` / `Resource` pre-init (`propagator_test.go`).
- **Config:** `ValidateConfiguration` with live `redisOptions`
(`config_validate_redis_test.go`).
- **Gate:** `COVERAGE_MIN` **65** (Makefile, Bitbucket, README).
- **Merged `total:` (latest):** **~71.4%** — next push toward **75%**: AMQP
`NewAmqp`/`handleReconnect` hooks, audit publisher, `phlogger` branches, more
`wirepb` / `idempotency-key`, or OTLP-enabled `phtrace.Init` with test collector.

### Session progress (2026-04-25 — Phase 1 continuation, 75%+)

- **AMQP:** Added test-only backoff hooks for reconnect / re-init / resend
delays and exercised dial-failure reconnect loops for `NewAmqp` and
`NewAmqpClient` without waiting for production sleeps.
- **Audit trail:** Added `pushMessageAudit` tests for publish success and publish
failure (bounded retries with short resend delay) without a real broker.
- **phlogger:** Added tests for `LogJ` / `LogJI`, rated warn/debug paths, and
`InitializeLogger` idempotency, explicitly keeping `LogF` untested.
- **Merged `total:` (latest):** **75.5%** (`make test-coverage-check`, default `COVERAGE_PKGS`).