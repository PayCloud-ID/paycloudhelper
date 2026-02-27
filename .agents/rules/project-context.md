# Project Context — paycloudhelper

## Purpose

`bitbucket.org/paycloudid/paycloudhelper` is a **Go shared library** — not a standalone service. It provides common utilities (middleware, Redis, logging, RabbitMQ, response helpers) imported by all PayCloud Hub microservices.

## Module Identity

| Item | Value |
|------|-------|
| Module | `bitbucket.org/paycloudid/paycloudhelper` |
| Go version | `1.23` + toolchain `1.24.3` |
| Library type | Shared helper — consumers import it |

## Package Structure

| Package | Path | Purpose |
|---------|------|---------|
| Root | `.` | Public API — middleware, Redis, logging, response, RabbitMQ |
| `phhelper` | `phhelper/` | Global state (`globAppName`, `globAppEnv`) + JSON/string helpers |
| `phlogger` | `phlogger/` | Logger wrapper around `kataras/golog` |
| `phsentry` | `phsentry/` | Sentry error tracking integration |
| `phaudittrailv0` | `phaudittrailv0/` | Legacy audit trail (v0 protocol) |
| `phjson` | `phjson/` | Sonic-based JSON wrapper for high-throughput consumers |

## Auto-Initialization (`init.go`)

When any service imports this library, `init()` runs automatically:

```
init() → AddValidatorLibs() → InitializeLogger() → InitializeApp()
```

Consumer services must **explicitly** call:
- `InitializeRedis(opt)` or `InitializeRedisWithRetry(opts)` — for Redis + RedSync
- `SetUpRabbitMq(...)` — for audit trail publishing
- `InitSentry(options)` — for Sentry error tracking

## Critical Rules

### 1. Backward Compatibility is Non-Negotiable

```go
// ✅ New optional function — consumers unaffected
func NewFeature(opts ...Option) error { }

// ❌ Never change existing signatures
func ExistingFunc(old string, NEW string) error { } // BREAKS ALL CONSUMERS
```

### 2. No Direct `os.Getenv()` Except in `InitializeApp()`

```go
// ✅ Only in InitializeApp()
if appName := os.Getenv("APP_NAME"); appName != "" { ... }

// ❌ Never spread os.Getenv() through middleware files
// Middleware must receive values via function parameters
```

### 3. All Logging via Library Helpers

```go
// ✅ Use library log helpers
LogI("[FunctionName] operation: %s", value)
LogE("[FunctionName] error: %v", err)
LogW("[FunctionName] warning: %s", detail)

// ❌ Never use stdlib logging
log.Println("message")
fmt.Println("debug")
```

### 4. Sync.Once for Init — Never Raw Nil Checks

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
go test ./...   # Must pass before every commit
```

## Key Files

| File | Purpose |
|------|---------|
| `init.go` | Auto-init entry point |
| `redis.go` | Redis pool, distributed locks, store/get helpers |
| `logger.go` | Log shorthand aliases (`LogI`, `LogE`, `LogD`, `LogW`) |
| `response.go` | `ResponseApi` struct with HTTP helper methods |
| `csrf.go` | `VerifCsrf` Echo middleware |
| `idempotency-key.go` | `VerifIdemKey` Echo middleware |
| `revoke-token.go` | `RevokeToken` Echo middleware |
| `amqp.go` | Auto-reconnecting AMQP client |
| `audittrail.go` | Async audit trail publishing |
| `validator.go` | `govalidator` custom rules |
| `config.go` | `ValidateConfiguration()` + `LogConfigurationWarnings()` |
| `mutex.go` | Distributed lock mutex map helpers |
| `helpers.go` | Misc utilities (`JSONEncode`, `GetOrGenerateRequestID`, etc.) |
