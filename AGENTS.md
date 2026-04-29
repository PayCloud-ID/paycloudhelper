# AI Agent Instructions — paycloudhelper

> Single source of truth for GitHub Copilot, Cursor, Claude Code, and other AI agents.

## Quick Reference

| Resource | Path | Description |
|----------|------|-------------|
| This file | `AGENTS.md` | Primary AI agent instructions |
| Skills | `.agents/skills/` | Domain expertise packages |
| Rules | `.agents/rules/` | Development rules and patterns |
| Improvements backlog | `.github/IMPROVEMENTS.md` | Known issues, P0–P3 priorities |

## Repository Overview

**Go shared library** (`github.com/PayCloud-ID/paycloudhelper`) providing common utilities to all PayCloud Hub microservices. This is **not a standalone service** — it is imported by ~30 consumer services.

- **Go 1.25.0** + toolchain 1.25.9
- **Auto-initializes** on import via `init()` → consumer services call explicit init functions for Redis/RabbitMQ/Sentry

### Package Structure

| Package | Purpose |
|---------|---------|
| Root (`.`) | Public API: middleware, Redis, logging, response, RabbitMQ, locks |
| `phhelper/` | Global state (`globAppName`, `globAppEnv`), string/JSON helpers |
| `phlogger/` | Logger wrapper (`kataras/golog`) |
| `phsentry/` | Sentry error tracking |
| `phaudittrailv0/` | Legacy v0 audit trail |
| `phjson/` | Sonic-based JSON wrapper for consumer opt-in performance |
| `sdk/services/` | Service-scoped SDK layout for shared service clients and proto snapshots |
| `sdk/shared/` | Shared runtime helpers for transport, observability, and error normalization |

### Startup Flow

```
import paycloudhelper → init() automatic:
  AddValidatorLibs() → InitializeLogger() → InitializeApp()

Consumer must call explicitly:
  InitializeRedisWithRetry(opts) → Redis + RedSync
  SetUpRabbitMq(...)             → Audit trail
  InitSentry(options)            → Error tracking
```

## Critical Conventions

### 1. Backward Compatibility is Absolute

```go
// ✅ Add new function — consumers unaffected
func InitializeRedisWithRetry(opts RedisInitOptions) error { }

// ❌ NEVER change existing signatures — breaks all consumers
func InitializeRedis(opt redis.Options, NEW string) error { }
```

### 2. No `os.Getenv()` Outside `InitializeApp()`

```go
// ✅ Only in InitializeApp()
if appName := os.Getenv("APP_NAME"); appName != "" { ... }

// ❌ Never in middleware or helper files
```

### 3. Logging via Library Helpers Only (pchelper/phlogger)

Consumers must use the **root package** with alias `pchelper`; only other paycloudhelper subpackages may import `phlogger` directly.

- **Format:** Every log line MUST include the calling function in square brackets: `[Type.MethodName]` for methods, `[FuncName]` for plain functions.
- **Style:** Prefer key=value pairs after the function name.
- **Levels:** Use the standard decision tree — failure/unexpected → `LogE`; degraded/recoverable → `LogW`; tracing → `LogD`; normal operations → `LogI`; unrecoverable → `LogF`.

```go
// ✅ Root package alias in consumer services
import pchelper "github.com/PayCloud-ID/paycloudhelper"

// ✅ Methods: [ReceiverType.MethodName]
pchelper.LogI("[Server.initializeConnections] gRPC connected host=%s", host)
pchelper.LogE("[MerchantController.GetMerchant] gRPC error code=%s err=%v", code, err)

// ✅ Plain functions: [FuncName]
pchelper.LogI("[InitRedis] connected port=%s", port)

// ✅ Error shorthand
pchelper.LogErr(err)

// ❌ Never in consumer code
import "github.com/PayCloud-ID/paycloudhelper/phlogger"
log.Println("message")
fmt.Println("debug")
pchelper.LogE("error=%v", err)  // missing [Type.FuncName] prefix
```

### 4. Sync.Once for Singleton Init

```go
// ✅ Race-safe
func InitRedSyncOnce() error {
    redisSyncInitOnce.Do(func() { ... })
    return redisSyncInitErr
}

// ❌ Race condition
if redisSync == nil { redisSync = redsync.New(pool) }
```

### 5. Tests Required for All Changes

```bash
go test ./...        # happy path + errors
go test -race ./...  # for init / concurrency changes
```

### 6. Update README and CHANGELOG When Changes Occur

- **README.md** — Update whenever you add or change user-facing behavior, APIs, configuration, or workflows (e.g. new script, new env var, new section).
- **CHANGELOG.md** — Update on every release-worthy change. Add entries under `[Unreleased]` in the appropriate category: **Added**, **Changed**, **Fixed**, **Deprecated**, **Removed**, **Security**. When cutting a release, move `[Unreleased]` content into a new version heading and add a link at the bottom.
- If you only fix a typo or internal refactor with no user impact, a CHANGELOG line is optional; README only if it affects documented behavior.

### 7. Script Changes Must Include Standard Header Docs

Whenever creating or updating any shell script (`*.sh`), include/update the standardized inline header directly below the shebang with:

- `Purpose`
- `Usage`
- `Options`
- `What It Reads`
- `What It Affects / Does`
- `Exit Behavior`

Do not leave script documentation for a follow-up commit.

## Key APIs at a Glance

### Logging (phlogger via root package)

| Function | Level | Use |
|----------|-------|-----|
| `LogI(format, args...)` | Info | Normal operations, startup, state changes |
| `LogE(format, args...)` | Error | Failures — gRPC/DB/validation errors |
| `LogW(format, args...)` | Warn | Degraded but recoverable — retries, fallbacks |
| `LogD(format, args...)` | Debug | Verbose tracing (silent at default info level) |
| `LogF(format, args...)` | Fatal | Unrecoverable — process exits after hooks |
| `LogJ(obj)` / `LogJI(obj)` | Info | Compact / indented JSON |
| `LogErr(err)` | Error | Error value only (no format string) |

Rated (unstable format key): `LogIRated("key", format, args...)`, `LogIRatedW("key", window, format, args...)`. Request-scoped: `NewLogContext("k", "v").LogI(format, args...)`. **Every format must start with `[Type.FuncName]` or `[FuncName]` and prefer key=value.** For the full PayCloud logging standard (sampling, Sentry forwarding, metrics, anti-patterns, code-generation rules), see the pchelper-logging-standard instructions.

### Response (`response.go`)

```go
var response ResponseApi
response.Success("ok", data)            // 200
response.Accepted(data)                 // 202
response.BadRequest("msg", "ERR_CODE")  // 400
response.Unauthorized("msg", "")        // 401
response.InternalServerError(err)       // 500
return c.JSON(response.Code, response)
```

### Redis (`redis.go`)

```go
StoreRedis(key, value, duration)                            // store
GetRedis(key)                                               // retrieve
StoreRedisWithLock(key, value, duration)                    // atomic
AcquireLockWithRetry(key, ttl, retries, delay)             // distributed lock
ReleaseLockWithRetry(mutex, retries)                       // release
```

### Audit Trail (V1 + V2)

**V1 — goroutine-per-call** (legacy, still supported):

```go
client := SetUpRabbitMq(host, port, vhost, user, pass, queue, appName)
LogAudittrailData(funcName, desc, source, commType, &keys, &reqResp)
LogAudittrailProcess(funcName, desc, info, &keys)
```

**V2 — worker pool with circuit breaker** (recommended for new services):

```go
pub := SetUpAuditTrailPublisher(host, port, vhost, user, pass, queue, appName,
    WithWorkerCount(10), WithBufferSize(1000), WithMessageTTL("60000"))
LogAudittrailDataV2(funcName, desc, source, commType, &keys, &reqResp)
LogAudittrailProcessV2(funcName, desc, info, &keys)
// Falls back to V1 when publisher is nil
```

Key types: `AuditPublisher`, `MessagePayloadAudit`, `AuditTrailData`, `AuditTrailProcess`, `RequestAndResponse`.

### Middleware (Echo)

```go
e.Use(VerifCsrf)        // Validates X-Xsrf-Token vs Redis
e.Use(VerifIdemKey)     // Deduplicates by Idempotency-Key + body hash
e.Use(RevokeToken)      // Validates JWT + revocation check in Redis
```

### Sentry Structured Logging (`phsentry/log_hook.go`)

Paycloudhelper integrates **structured logging with Sentry** (SDK v0.33.0+) to forward all logs to Sentry for centralized error tracking and observability.

**Architecture:**
- Hook-based integration: All logs via `LogI()`, `LogE()`, `LogW()`, `LogD()`, `LogF()` are forwarded to Sentry
- Error/fatal logs become exception events (grouped by `[FunctionName]` prefix)
- Info/warn/debug logs become breadcrumbs (contextual trace data)
- Completely opt-in via `ConfigureSentryLogging(enable bool)`

**Setup (consumer service):**

```go
import pchelper "github.com/PayCloud-ID/paycloudhelper"

// 1. Initialize Sentry (early in startup)
pchelper.InitSentry(pchelper.SentryOptions{
    Dsn:         os.Getenv("SENTRY_DSN"),
    Environment: os.Getenv("APP_ENV"),
    Release:     version,
})

// 2. Enable structured logging to Sentry via environment variable (recommended)
pchelper.ConfigureSentryLogging(pchelper.SentryLoggingFromEnv())

// 3. Emit logs (automatically forwarded to Sentry when enabled)
pchelper.LogE("[Server.start] failed to bind port err=%v", err)   // → Sentry exception
pchelper.LogI("[Server.start] listening on port=%s", port)        // → Sentry breadcrumb
```

**Environment variable:**

```bash
SENTRY_LOGGING=true   # enable structured logging (also accepts 1, t, T)
SENTRY_LOGGING=false  # disable (also accepts 0, f, F - default)
# (unset or invalid)   → disable (default)

**How it works:**
- Every log call triggers registered hooks after the message is formatted
- Hook functions receive `(level string, message string)` 
- `ReceiveLog()` processes the message and decides whether to send an exception event or breadcrumb
- Error/fatal logs → exception event (appears as issue in Sentry dashboard)
- Info/warn/debug logs → breadcrumb (appears as context in related issues)
- `[FunctionName]` prefix in brackets is extracted for grouping (e.g., `[InitRedis]` → issue title starts with `[InitRedis]`)

**Integration with existing log forwarding:**
- `ConfigureSentryLogging()` is the new recommended entry point
- Legacy `ConfigureLogForwarding()` still works for granular per-level control
- Both can coexist; hooks are cumulative

**Testing / debugging:**
- Set `SENTRY_LOGGING=true` to enable
- Logs are sent synchronously to Sentry; calls block briefly
- Use `pchelper.FlushSentry(2 * time.Second)` before process exit to ensure delivery
- Check Sentry dashboard under your DSN project for captured events/breadcrumbs

## Versioning

| Bump | When | Examples |
|------|------|---------|
| **PATCH** | Bug fix, zero behavior change, no new public API | Fix nil panic, fix typo in log message |
| **MINOR** | New backward-compatible additions (new functions, new optional config) | `ConfigureLogForwarding()`, `LogIRated()` |
| **MAJOR** | Any breaking change to existing public API signatures OR removal of exported symbols | Rename `InitSentry` params, remove `LogErr` |

### Breaking Change Decision Tree

```
Does the change touch an EXISTING exported function signature?
├─ YES → MAJOR bump required. Coordinate consumer updates first.
└─ NO  → Does it add new exported symbols?
          ├─ YES → MINOR bump (v1.X.0)
          └─ NO  → PATCH bump (v1.6.X)
```

### Backward-Compatibility Contract

- NEVER change existing exported function signatures
- NEVER remove exported symbols without a deprecation cycle (MINOR → MAJOR)
- New optional parameters → use variadic `opts ...Option` pattern
- Deprecated functions retain original signature + route to new implementation

### Service SDK Layout Rule

- New shared service integrations should be added under `sdk/services/<service>/`.
- Use `make proto.service.scaffold SERVICE=<service>` to prepare the baseline structure for additional service SDKs.

**Known retractions:** v1.6.3 (verbose Redis logs), v1.6.0 (audit trail race), v1.5.2 (nil panic on init)

## Skills Reference

| Skill | Path | Use When |
|-------|------|----------|
| Middleware Development | `.agents/skills/middleware-development/` | Adding/modifying Echo middleware, response handling |
| Redis Patterns | `.agents/skills/redis-patterns/` | Redis init, store/get, distributed locks, key conventions |
| Library Maintenance | `.agents/skills/library-maintenance/` | Versioning, deprecation, subpackage rules, release workflow |
| Redis v9 Consumer Migration Core | `.agents/skills/redis-v9-consumer-migration-core/` | Upgrading service dependency from paycloudhelper v1.x to v2.x safely |
| Redis v9 Migration for Echo APIs | `.agents/skills/redis-v9-consumer-migration-echo-api/` | Migrating API services using paycloudhelper middleware and Redis request-path logic |
| Redis v9 Migration for Workers | `.agents/skills/redis-v9-consumer-migration-worker/` | Migrating queue/consumer services using distributed locks and retries |
| Redis v9 Migration for Schedulers | `.agents/skills/redis-v9-consumer-migration-scheduler/` | Migrating cron/job services that require singleton execution locks |

## Consumer Migration Assets

- Use `prompt-migrate-bitbucket-pipelines-to-github-actions.md` from repo root to generate a GitHub Actions workflow from the current Bitbucket pipeline.
- For consumer service migration planning, start with the core skill and then select the service-profile skill (`echo-api`, `worker`, or `scheduler`).
- Keep migration rollouts staged: build/vet/test/race in service repo before promoting to production.

## Rules Reference

| Rule | File | Purpose |
|------|------|---------|
| Project Context | `.agents/rules/project-context.md` | Architecture, package structure, 5 critical rules |
| API Compatibility | `.agents/rules/api-compatibility.md` | Versioning contract, breaking change detection, deprecation |
| Testing & Validation | `.agents/rules/testing-validation.md` | Test commands, patterns, coverage requirements |
| Documentation & Changelog | `.agents/rules/documentation-changelog.md` | When and how to update README and CHANGELOG |
| Script Documentation | `.agents/rules/script-documentation.md` | Mandatory inline header format for script create/update |

## Developer Workflows

```bash
# Run all tests
go test ./...

# Race detection (required for concurrency changes)
go test -race ./...

# Build check
go build ./...

# Vet
go vet ./...

# Tag release
git tag v1.x.y && git push origin v1.x.y

# Consumer updates
go get github.com/PayCloud-ID/paycloudhelper@v2.0.0 && go mod tidy
```

## Agent Compatibility

### GitHub Copilot
- Reads `.github/copilot-instructions.md` → delegates to `AGENTS.md`
- Skills via `.github/skills/` symlink → `.agents/skills/`

### Cursor
- Rules via `.cursor/rules/` symlink → `.agents/rules/`
- Skills via `.cursor/skills/` symlink → `.agents/skills/`

### Claude Code
- Reads `AGENTS.md` directly
- All resources accessible via `.agents/`
