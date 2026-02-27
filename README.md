# paycloudhelper

**Go shared library** — common utilities for all PayCloud Hub microservices.

Module: `bitbucket.org/paycloudid/paycloudhelper`  
Go: 1.23 + toolchain 1.24.3

---

## Table of Contents
- [Overview](#overview)
- [Quick Start](#quick-start)
- [Package Structure](#package-structure)
- [API Reference](#api-reference)
- [Configuration](#configuration)
- [Versioning](#versioning)
- [Contributing](#contributing)

---

## Overview

`paycloudhelper` is a **shared library** imported by ~30 PayCloud microservices. It is **not a standalone service**. On import, `init()` runs automatically and bootstraps the logger and app identity. Consumer services then explicitly opt into Redis, RabbitMQ, and Sentry.

### Auto-Initialization Flow

```
import paycloudhelper → init() runs:
  AddValidatorLibs() → InitializeLogger() → InitializeApp()

Consumer must explicitly call:
  InitializeRedisWithRetry(opts)   → Redis pool + RedSync
  SetUpRabbitMq(...)               → Audit trail
  InitSentry(options)              → Error tracking (optional)
  ConfigureLogForwarding(cfg)      → Log → Sentry forwarding (optional)
```

---

## Quick Start

```go
import pch "bitbucket.org/paycloudid/paycloudhelper"

// In main() — after godotenv.Load()
pch.InitializeRedisWithRetry(pch.RedisInitOptions{...})
pch.SetUpRabbitMq(...)
pch.InitSentry(pch.SentryOptions{Dsn: os.Getenv("SENTRY_DSN")})

// Optional: forward Fatal logs to Sentry automatically
pch.ConfigureLogForwarding(pch.LogForwardConfig{
    ForwardFatal: true, // default true when Sentry is enabled
})
```

---

## Package Structure

| Package | Path | Purpose |
|---------|------|---------|
| Root | `.` | Public API — all below re-exported here |
| `phlogger` | `phlogger/` | Logger wrapper (`kataras/golog`) + rate limiter + forwarding hooks |
| `phsentry` | `phsentry/` | Sentry error tracking, log receiver |
| `phhelper` | `phhelper/` | Global state (`APP_NAME`, `APP_ENV`), JSON/string helpers |
| `phaudittrailv0` | `phaudittrailv0/` | Legacy v0 audit trail (RabbitMQ) |
| `phjson` | `phjson/` | Sonic JSON wrapper for high-throughput consumers |

---

## API Reference

### Logging

```go
pch.LogI("[FuncName] started id=%s", id)    // Info
pch.LogE("[FuncName] error: %v", err)        // Error
pch.LogW("[FuncName] warn: %s", msg)         // Warning
pch.LogD("[FuncName] debug key=%s", key)     // Debug (off in production)
pch.LogF("[FuncName] fatal: %v", err)        // Fatal — process exits
pch.LogJ(obj)                                // JSON (compact)
pch.LogJI(obj)                               // JSON (indented)
```

#### Rate-Limited Logging (opt-in)

```go
// Log at most 1 message per key per 10 seconds
pch.LogIRated("cache.miss", 10*time.Second, "[FuncName] cache miss key=%s", key)
pch.LogERated("db.error", 5*time.Second, "[FuncName] db error: %v", err)
```

#### Log Forwarding to Sentry

```go
// Call once at startup — configure which levels forward to Sentry
pch.ConfigureLogForwarding(pch.LogForwardConfig{
    ForwardFatal: true,  // default: true when Sentry enabled
    ForwardError: false, // default: false
    ForwardWarn:  false, // default: false
    // OR load from env: pch.LogForwardConfigFromEnv()
})
```

Environment variables (evaluated by `LogForwardConfigFromEnv()`):

| Env Var | Default | Effect |
|---------|---------|--------|
| `LOG_FORWARD_FATAL` | `true` | Forward Fatal logs to Sentry |
| `LOG_FORWARD_ERROR` | `false` | Forward Error logs to Sentry |
| `LOG_FORWARD_WARN` | `false` | Forward Warn logs to Sentry |
| `LOG_FORWARD_INFO` | `false` | Forward Info logs to Sentry |

### Response

```go
var resp pch.ResponseApi
resp.Success("ok", data)            // 200
resp.Accepted(data)                 // 202
resp.BadRequest("msg", "ERR_CODE")  // 400
resp.Unauthorized("msg", "")        // 401
resp.InternalServerError(err)       // 500
return c.JSON(resp.Code, resp)
```

### Redis

```go
pch.StoreRedis(key, value, duration)
pch.GetRedis(key)
pch.StoreRedisWithLock(key, value, duration)
pch.AcquireLockWithRetry(key, ttl, retries, delay)
pch.ReleaseLockWithRetry(mutex, retries)
```

### Sentry

```go
pch.InitSentry(pch.SentryOptions{
    Dsn:         os.Getenv("SENTRY_DSN"),
    Environment: os.Getenv("APP_ENV"),
    Release:     "v1.7.0",
})
pch.SendSentryError(err)
pch.SendSentryMessage("something happened")
pch.FlushSentry(2 * time.Second)  // call before process exit
```

### Middleware (Echo)

```go
e.Use(pch.VerifCsrf)       // X-Xsrf-Token validation
e.Use(pch.VerifIdemKey)    // Idempotency-Key deduplication
e.Use(pch.RevokeToken)     // JWT + Redis revocation check
```

---

## Configuration

All configuration is loaded from environment variables in `InitializeApp()`:

| Var | Required | Default | Purpose |
|-----|----------|---------|---------|
| `APP_NAME` | Yes | `""` | Service name (used in Sentry, logs) |
| `APP_ENV` | Yes | `""` | `develop` / `staging` / `production` |
| `REDIS_HOST` | For Redis | `""` | Redis server |
| `REDIS_PORT` | For Redis | `6379` | Redis port |
| `REDIS_PASSWORD` | No | `""` | Redis auth |
| `SENTRY_DSN` | For Sentry | `""` | Sentry project DSN |
| `LOG_FORWARD_FATAL` | No | `true` | Forward Fatal → Sentry |
| `LOG_FORWARD_ERROR` | No | `false` | Forward Error → Sentry |
| `LOG_FORWARD_WARN` | No | `false` | Forward Warn → Sentry |
| `LOG_FORWARD_INFO` | No | `false` | Forward Info → Sentry |
| `TRANSACTION_REDIS_LOCK_TIMEOUT` | No | `2000` (ms) | Distributed lock TTL |
| `TRANSACTION_REDIS_BACKOFF` | No | `10` (ms) | Lock retry backoff |

---

## Versioning

| Bump | When |
|------|------|
| **PATCH** | Bug fixes, zero behavior change |
| **MINOR** | New backward-compatible features |
| **MAJOR** | Breaking changes — requires coordinating all consumer updates |

---

## Contributing

1. `git checkout -b feat/your-feature`
2. Write failing test first (TDD)
3. Implement minimal code
4. `go test -race ./...` — must pass
5. `go build ./...` — must pass
6. `git tag vX.Y.Z` when ready to release

See `.agents/rules/` and `AGENTS.md` for full development rules.
