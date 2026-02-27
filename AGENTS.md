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

**Go shared library** (`bitbucket.org/paycloudid/paycloudhelper`) providing common utilities to all PayCloud Hub microservices. This is **not a standalone service** — it is imported by ~30 consumer services.

- **Go 1.23** + toolchain 1.24.3
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

### 3. Logging via Library Helpers Only

```go
// ✅ Structured logging with function context
LogI("[FunctionName] operation: %s", value)
LogE("[FunctionName] error: %v", err)

// ❌ Never
log.Println("message")
fmt.Println("debug")
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

## Key APIs at a Glance

### Logging

```go
LogI("info %s", val)    // Info
LogE("err %v", err)     // Error
LogW("warn %s", msg)    // Warning
LogD("debug %s", data)  // Debug
LogJ(obj)               // JSON compact
LogJI(obj)              // JSON indented
```

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

### Middleware (Echo)

```go
e.Use(VerifCsrf)        // Validates X-Xsrf-Token vs Redis
e.Use(VerifIdemKey)     // Deduplicates by Idempotency-Key + body hash
e.Use(RevokeToken)      // Validates JWT + revocation check in Redis
```

## Versioning

| Bump | When |
|------|------|
| **PATCH** | Bug fixes, zero behavior change |
| **MINOR** | New backward-compatible features |
| **MAJOR** | Breaking changes (requires updating all consumers) |

**Known retractions:** v1.6.3 (verbose Redis logs), v1.6.0 (audit trail race), v1.5.2 (nil panic on init)

## Skills Reference

| Skill | Path | Use When |
|-------|------|----------|
| Middleware Development | `.agents/skills/middleware-development/` | Adding/modifying Echo middleware, response handling |
| Redis Patterns | `.agents/skills/redis-patterns/` | Redis init, store/get, distributed locks, key conventions |
| Library Maintenance | `.agents/skills/library-maintenance/` | Versioning, deprecation, subpackage rules, release workflow |

## Rules Reference

| Rule | File | Purpose |
|------|------|---------|
| Project Context | `.agents/rules/project-context.md` | Architecture, package structure, 5 critical rules |
| API Compatibility | `.agents/rules/api-compatibility.md` | Versioning contract, breaking change detection, deprecation |
| Testing & Validation | `.agents/rules/testing-validation.md` | Test commands, patterns, coverage requirements |

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
go get bitbucket.org/paycloudid/paycloudhelper@v1.x.y && go mod tidy
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
