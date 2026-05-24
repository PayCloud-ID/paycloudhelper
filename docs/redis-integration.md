# Redis Integration Guide

How to wire up Redis in a PayCloud microservice using `paycloudhelper`.

---

## Overview

`paycloudhelper` manages the Redis connection pool, distributed locks (redsync), and all key operations centrally. Services do **not** need their own `providers/redis.go`. The library exposes two initialization paths:

| Function | Use when |
|---|---|
| `InitRedisFromEnv()` | Standard — reads `REDIS_*` env vars, skips silently if `REDIS_HOST` is unset |
| `InitializeRedisWithRetry(opts)` | Advanced — full control over options, pool size, FailFast mode |

---

## Quick setup

### 1. Environment variables

Add to your service `.env` (or container environment):

```env
REDIS_HOST=127.0.0.1
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=9
```

`REDIS_HOST` is the toggle. When it is absent, `InitRedisFromEnv()` returns `nil` and Redis stays off — useful for services that do not always need caching.

| Variable | Required to enable Redis | Default |
|---|---|---|
| `REDIS_HOST` | Yes | — |
| `REDIS_PORT` | No | `6379` |
| `REDIS_PASSWORD` | No | `""` |
| `REDIS_DB` | No | `0` |

### 2. Call in `main.go`

`paycloudhelper`'s `init()` loads `.env` automatically on import. Call `InitRedisFromEnv()` right after, before your server starts:

```go
package main

import (
    "log"
    "os"

    pch "github.com/PayCloud-ID/paycloudhelper"
)

func main() {
    // paycloudhelper init() has already loaded .env and set APP_NAME / APP_ENV.

    if err := pch.InitRedisFromEnv(); err != nil {
        log.Fatalf("redis: %v", err)
    }

    // Optional: guard features that require Redis
    if pch.RedisEnabled() {
        pch.LogI("[Main] Redis ready")
    }

    // ... start HTTP server, workers, etc.
}
```

That's it. No `providers/redis.go`, no duplicated connection logic.

---

## Using Redis

All operations use the pool initialized above. Import only the root package.

```go
import (
    "time"
    pch "github.com/PayCloud-ID/paycloudhelper"
)

// Store (JSON-serialized automatically)
err := pch.StoreRedis("session:abc", myStruct, 30*time.Minute)

// Store with context (preferred — propagates deadline/tracing)
err := pch.StoreRedisWithContext(ctx, "session:abc", myStruct, 30*time.Minute)

// Retrieve
raw, err := pch.GetRedis("session:abc")

// Retrieve with context
raw, err := pch.GetRedisWithContext(ctx, "session:abc")

// Delete
err := pch.DeleteRedis("session:abc")

// Atomic store under distributed lock
err := pch.StoreRedisWithLock("account:123", data, 5*time.Minute)
```

### Distributed locking

```go
// One-shot acquire/release
locked, err := pch.AcquireLock("payment:txn-xyz", 10*time.Second)
if err != nil { /* infrastructure error */ }
if !locked { /* another process holds it */ }
defer pch.ReleaseLock("payment:txn-xyz")

// Acquire with built-in retry
mutex, acquired, err := pch.AcquireLockWithRetry(
    "payment:txn-xyz",
    10*time.Second, // TTL
    3,              // max retries
    200*time.Millisecond,
)
if acquired {
    defer pch.ReleaseLockWithRetry(mutex, 3)
}
```

Lock TTL and backoff can be tuned via:

```env
TRANSACTION_REDIS_LOCK_TIMEOUT=2000   # milliseconds, minimum 700
TRANSACTION_REDIS_BACKOFF=10          # milliseconds
```

### Direct client access

When you need a raw `*redis.Client` (pipeline, Lua scripts, SCAN, etc.):

```go
client, err := pch.GetRedisPoolClient()
if err != nil {
    return err
}
// client is *github.com/redis/go-redis/v9.Client
pipe := client.Pipeline()
```

---

## Advanced initialization

For services that need non-default pool sizing or must fail fast:

```go
import (
    pch "github.com/PayCloud-ID/paycloudhelper"
    "github.com/redis/go-redis/v9"
    "runtime"
)

err := pch.InitializeRedisWithRetry(pch.RedisInitOptions{
    Options: redis.Options{
        Addr:     "127.0.0.1:6379",
        Password: os.Getenv("REDIS_PASSWORD"),
        DB:       9,
        PoolSize: 10 * runtime.GOMAXPROCS(0),
    },
    MaxRetries: 5,
    RetryDelay: 2 * time.Second,
    FailFast:   true, // return error instead of logging and continuing
})
```

---

## Checking if Redis is available

`RedisEnabled()` returns `true` only after a successful `Ping` during initialization. Use it to guard optional features:

```go
func (s *Server) handleCache(ctx context.Context, key string) {
    if !pch.RedisEnabled() {
        // fall back to DB or skip caching
        return
    }
    val, err := pch.GetRedisWithContext(ctx, key)
    // ...
}
```

---

## Health checks

`paycloudhelper` includes a built-in health check that covers Redis:

```go
status := pch.CheckHealth()
// map[string]interface{}{
//   "redis":  map[string]interface{}{"status": "healthy", "latency_ms": 0.4},
//   "config": map[string]interface{}{"status": "healthy"},
//   ...
// }
```

Wire it to your health endpoint:

```go
e.GET("/health", func(c echo.Context) error {
    return c.JSON(http.StatusOK, pch.CheckHealth())
})
```

---

## Migrating from a per-service `providers/redis.go`

If your service has its own Redis provider file, follow these steps.

### Before (per-service pattern)

```go
// providers/redis.go — 80 lines of duplicated init/connect/ping/redsync boilerplate
func InitRedis() (*redis.Client, error) {
    rdHost := os.Getenv("REDIS_HOST")
    rdPort := os.Getenv("REDIS_PORT")
    // ...
    RedisPoolClient = redis.NewClient(&redis.Options{ ... })
    // ...
}
```

```go
// main.go
if _, err := providers.InitRedis(); err != nil {
    log.Fatal(err)
}
```

### After (centralized pattern)

```go
// providers/redis.go — delete this file entirely
```

```go
// main.go
if err := pch.InitRedisFromEnv(); err != nil {
    log.Fatalf("redis: %v", err)
}
```

Replace all `providers.StoreRedis` / `providers.GetRedis` / `providers.DeleteRedis` calls with the `pch.*` equivalents. The signatures are identical.

### Migration checklist

- [ ] Delete `providers/redis.go` (or your equivalent)
- [ ] Replace `providers.InitRedis()` in `main.go` with `pch.InitRedisFromEnv()`
- [ ] Replace `providers.RedisPoolClient` direct access with `pch.GetRedisPoolClient()`
- [ ] Replace `providers.StoreRedis` → `pch.StoreRedisWithContext`
- [ ] Replace `providers.GetRedis` → `pch.GetRedisWithContext`
- [ ] Replace `providers.DeleteRedis` → `pch.DeleteRedisWithContext`
- [ ] Replace `providers.AcquireLock` / `ReleaseLock` → `pch.AcquireLock` / `pch.ReleaseLock`
- [ ] Update `go.mod`: replace `github.com/go-redis/redis/v8` with `github.com/redis/go-redis/v9`
- [ ] Run `go build ./... && go test ./...`

---

## Testing

Use `miniredis` for unit tests — no real Redis required:

```go
import (
    "testing"
    "github.com/alicebob/miniredis/v2"
    pch "github.com/PayCloud-ID/paycloudhelper"
    "github.com/redis/go-redis/v9"
)

func TestMyFeature(t *testing.T) {
    mr, err := miniredis.Run()
    if err != nil {
        t.Fatal(err)
    }
    t.Cleanup(mr.Close)

    pch.InitializeRedis(redis.Options{Addr: mr.Addr()})

    // test pch.StoreRedis, pch.GetRedis, etc.
}
```

`miniredis` is already a dev dependency of `paycloudhelper` (`go.mod`), so no extra `go get` is needed in consumer services.

---

## Known limitations & future work

These issues are tracked here because fixing them requires a **breaking API change** across all ~30 consumer services. They are not urgent — the current implementation is safe for production — but should be addressed in a future major version.

### 1. `DefaultRedisTimeout` — public mutable global

`DefaultRedisTimeout` is an exported `var`. Any goroutine can write to it, and it is read by every Redis operation. It is safe today because services only set it during startup before concurrent operations begin, but nothing enforces that ordering.

**Future fix:** make it unexported; expose configuration via a `WithReadTimeout(d time.Duration)` functional option on `InitializeRedisWithRetry` / `InitRedisFromEnv`. Requires a minor version bump with a deprecation notice.

### 2. `os.Getenv` in the lock hot path

`GetTrxRedisBackoff()` and `GetTrxRedisLockTimeout()` call `os.Getenv` on every lock acquire and release. For services that call distributed locks at high frequency this adds a measurable syscall overhead per operation.

**Future fix:** cache both values at init time (e.g., call them once inside `InitRedisFromEnv` / `InitializeRedisWithRetry` and store in unexported package vars). Zero impact on the public API.

### 3. Package-global state — no multi-instance support

All Redis state (`redisPoolClient`, `redisOptions`, `redisSync`, …) lives in package-level variables. This means:
- A process can only hold **one** Redis connection pool.
- Testing is harder: tests must call `resetRedisClientStateForTesting()` and run sequentially.
- Two services compiled into the same binary would share state (unusual but possible with plugins).

**Future fix:** introduce a `type Client struct` that holds all state as fields, with methods mirroring the current package-level functions. Provide a package-level default instance for backward compatibility:

```go
// New API (future)
c, err := pch.NewRedisClientFromEnv()
c.Store(ctx, key, val, ttl)

// Backward compat shim — delegates to a package-level *Client
func StoreRedisWithContext(ctx context.Context, id string, data interface{}, d time.Duration) error {
    return defaultClient.Store(ctx, id, data, d)
}
```

This is a **minor** version addition (new API) with old functions kept as shims — no service changes required until they opt in.

### 4. `PoolSize` not explicit

`InitRedisOptions` does not set `PoolSize`; go-redis v9 defaults to `10 × GOMAXPROCS(0)`. That is a reasonable default, but services with high parallelism (many goroutines making simultaneous Redis calls) should tune this explicitly.

**Workaround today:** pass `PoolSize` directly via `InitializeRedisWithRetry`:

```go
pch.InitializeRedisWithRetry(pch.RedisInitOptions{
    Options: redis.Options{
        Addr:     net.JoinHostPort(host, port),
        PoolSize: 20 * runtime.GOMAXPROCS(0), // tune for your workload
    },
    FailFast: true,
})
```

**Future fix:** expose `REDIS_POOL_SIZE` env var in `InitRedisFromEnv`.
