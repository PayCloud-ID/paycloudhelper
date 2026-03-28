# paycloudhelper — AI Agent Guidelines

> **Primary instructions**: See [AGENTS.md](../AGENTS.md) for complete AI agent guidelines.
>
> This file provides GitHub Copilot-specific context.
> All agents share the same source of truth in `AGENTS.md`.

## Quick Reference

- **Go shared library** — imported by ~30 PayCloud microservices, not a standalone service
- **Module:** `bitbucket.org/paycloudid/paycloudhelper`
- **Auto-init:** `init()` runs automatically on import → explicit calls needed for Redis/RabbitMQ/Sentry
- **Backward compat:** NEVER change existing exported function signatures
- **Logging:** `LogI/LogE/LogW/LogD` with `[FunctionName]` context prefix
- **Redis locks:** Use `AcquireLockWithRetry` + `sync.Once` for init, never raw nil checks
- **Tests:** `go test -race ./...` required for any concurrency/init changes
- **Versioning:** PATCH=bugfix, MINOR=new features, MAJOR=breaking (avoid)
- **Improvements backlog:** `.github/IMPROVEMENTS.md`

## Architecture & Design Patterns

### Global State Management
- **App-level globals** stored in `phhelper/globenv.go` (`globAppName`, `globAppEnv`)
- Access via `GetAppName()`, `SetAppName()`, `GetAppEnv()`, `SetAppEnv()`
- **Auto-initialization** via `init()` in `init.go` - loads `.env`, sets app name/env from env vars
- All subpackages (`phhelper/`, `phlogger/`, `phsentry/`) follow this pattern

### Initialization Flow
1. `init()` runs automatically: `AddValidatorLibs()` → `InitializeLogger()` → `InitializeApp()`
2. Consumer services must explicitly call:
   - `InitializeRedis(opt)` - with custom `redis.Options`
   - `SetUpRabbitMq(...)` - for audit trail publishing
   - `InitSentry(options)` - for error tracking

### Redis Patterns
- **Singleton pattern**: One `redisPoolClient` per app, lazy-initialized via `GetRedisPoolClient()`
- **Options precedence**: `InitRedisOptions()` merges provided options with defaults (env vars as fallback)
- **Distributed locks**: Use `InitRedSyncOnce()` + mutex map (`StoreMutex`, `GetMutex`, `RemoveMutex`)
- **Key conventions**: `"redis_lock:{AppName}:"`, `"csrf-{token}"`, `"revoke_token_{merchantId}"`
- **Timeout handling**: `DefaultRedisTimeout` = 1s + custom `ReadTimeout`

### RabbitMQ Integration
- **AmqpClient** (`amqp.go`): Auto-reconnecting AMQP wrapper with mutex-protected state, finite retries, and publish timeouts
- **Audit trail V1**: Async publishing via goroutines in `LogAudittrailProcess()` / `LogAudittrailData()`
- **Audit trail V2**: Worker pool with circuit breaker via `SetUpAuditTrailPublisher()` + `LogAudittrailDataV2()` / `LogAudittrailProcessV2()` (falls back to V1 when publisher is nil)
- **Connection naming**: `"audittrail-{AppName}"` for observability

### Echo Middleware Conventions
- **VerifIdemKey**: Idempotency via Redis + MD5 hashing, returns `202 Accepted` for duplicate requests
- **VerifCsrf**: CSRF token validation against Redis, expires based on `Session` header (default 9s)
- **RevokeToken**: JWT validation using RSA public key from `APP_PUBLIC_KEY` env var, checks Redis for revoked tokens

## Code Quality Standards (from `.instructions.md`)

### Maintainability Rules
- **No duplication**: Refactor shared logic into helpers before adding new features
- **Backward compatibility**: Changes MUST NOT break existing consumers of this library
- **Environment validation**: Follow centralized approach - structured `EnvVar` arrays, single retrieval, multi-level validation
- **Test coverage**: Add tests for new features, bug fixes, and refactorings

### Performance Patterns
- **Single env var retrieval**: Read once, validate, then use
- **Efficient Redis**: Connection pooling, timeout handling, error checks
- **Async operations**: Use goroutines for non-blocking audit trail logging

## Common Workflows

### Adding New Middleware
1. Create file `{feature}.go` in root
2. Return `echo.HandlerFunc` wrapper (see `csrf.go`, `idempotency-key.go`)
3. Use `ResponseApi` struct for consistent error responses
4. Validate inputs with `govalidator` + custom rules (see `validator.go`)

### Adding Environment Variables
1. Update consumer services to pass values via initialization functions
2. Do NOT add env var parsing to this library (it's a helper, not a service)
3. Use `os.Getenv()` only in `InitializeApp()` for `APP_NAME`/`APP_ENV`

### Logging Best Practices
- Use short log helpers: `LogI`, `LogE`, `LogW`, `LogD` (from `logger.go`)
- JSON logging: `LogJ(obj)` (compact) or `LogJI(obj)` (indented)
- Structured context: Include function name, queue name, connection name in messages
- Example: `LogI("[AMQP] LogAuditTrailProcess func=%s desc=%s info=%s", funcName, desc, info)`

## Module Organization
- **Root package**: Core helpers, middleware, response handling
- **`phhelper/`**: Shared utilities (JSON, global state)
- **`phlogger/`**: Logging wrapper around `kataras/golog`
- **`phsentry/`**: Sentry error tracking integration
- **`phaudittrailv0/`**: Legacy audit trail (v0 protocol)
- **`phjson/`**: JSON manipulation utilities

## Key Dependencies
- **Echo v4**: Web framework for middleware
- **go-redis/redis/v8**: Redis client (sync with redsync for distributed locks)
- **rabbitmq/amqp091-go**: AMQP 0.9.1 client
- **bytedance/sonic**: High-performance JSON (used selectively)
- **thedevsaddam/govalidator**: Request validation with custom rules

## JSON Library Selection Guide

### When to Use Each Library
- **`encoding/json`** (stdlib): Default choice for most operations
  - Usage: `StoreRedis()`, general marshaling, logging helpers (`ToJson`, `ToJsonIndent`)
  - Pros: Reliable, well-tested, stable API
  - Cons: Slower performance on large payloads

- **`jsoniter.ConfigFastest`**: High-performance scenarios with structured data
  - Usage: `VerifIdemKey` middleware (idempotency key body parsing), `RevokeToken` (token unmarshaling)
  - When: Request/response processing, hot paths, known struct types
  - Example: `jsoniter.ConfigFastest.Unmarshal(body, &request)`

- **`phjson` (Sonic wrapper)**: Optional for consumer services
  - Usage: High-throughput services via `phjson.Marshal()`, `phjson.Unmarshal()`
  - When: Consumer services need maximum JSON performance
  - Note: Not used internally by paycloudhelper (consumers opt-in)

### Decision Tree
```
Need to parse JSON?
├─ In middleware hot path (per-request) → jsoniter.ConfigFastest
├─ Storing to Redis → encoding/json (compatibility)
├─ Logging/debugging → encoding/json (ToJson, ToJsonIndent helpers)
└─ Consumer service optimization → phjson (Sonic)
```

## Distributed Lock Patterns

### Basic Lock (Simple Operations)
```go
// For quick operations < 2s
locked, err := AcquireLock(key, ttl)
if err != nil {
    return err
}
if !locked {
    return errors.New("already being processed")
}
defer ReleaseLock(key)

// ... critical section ...
```

### Lock with Retry (Recommended for Production)
```go
// For operations that may encounter contention
mutex, acquired, err := AcquireLockWithRetry(
    "redis_lock:myapp:resource_id", 
    2*time.Second,  // TTL
    3,              // max retries
    50*time.Millisecond, // retry delay
)
if err != nil || !acquired {
    return fmt.Errorf("failed to acquire lock: %w", err)
}
defer ReleaseLockWithRetry(mutex, 3)

// ... critical section ...
```

### Helper Function (Convenience Wrapper)
```go
// Combines store + lock in one call
err := StoreRedisWithLock(key, data, duration)
// Automatically acquires lock, stores data, releases lock
```

### Lock Configuration
- **TTL**: Use env `TRANSACTION_REDIS_LOCK_TIMEOUT` (default: 2000ms, min: 700ms)
- **Backoff**: Use env `TRANSACTION_REDIS_BACKOFF` (default: 10ms)
- **Key naming**: Always use `redisLockKey` prefix: `"redis_lock:{AppName}:{resource}"`

### When to Use Locks
- ✅ Preventing duplicate Redis writes (race conditions)
- ✅ Distributed transaction coordination
- ✅ Rate limiting critical operations
- ❌ NOT needed for read-only operations
- ❌ NOT needed when Echo middleware already serializes requests

## Versioning & Release Guidelines

### Semantic Versioning Rules
This library follows **strict semantic versioning** because multiple production services depend on it:

- **PATCH (v1.6.x → v1.6.y)**: Bug fixes only, zero breaking changes
  - Example: Fix Redis error logging, improve error messages
  - Safe: Auto-update in consumer services
  
- **MINOR (v1.x → v1.y.0)**: New features, backward-compatible
  - Example: Add new middleware, new helper functions
  - Safe: Consumer services can upgrade without code changes
  - Rule: New parameters MUST have default values
  
- **MAJOR (v1.x → v2.0.0)**: Breaking changes
  - Example: Change function signatures, remove deprecated features, change behavior
  - Requires: Update all consumer services simultaneously
  - Avoid: Use deprecation + MINOR version instead when possible

### Release Checklist
1. **Pre-Release Testing**
   ```bash
   # Run all tests
   go test ./...
   
   # Verify no breaking changes to public API
   go doc -all | grep "^func" > api-current.txt
   # Compare with previous version
   
   # Check backwards compatibility
   go list -m -versions bitbucket.org/paycloudid/paycloudhelper
   ```

2. **Version Retractions (Historical Issues)**
   - v1.6.3: Redis error logging bug (too verbose)
   - v1.6.0: Unsafe audit trail push (race condition)
   - v1.5.2: Init bug (panic on nil options)
   - **Before releasing**: Review `go.mod` retractions to avoid similar issues

3. **Integration Testing**
   - Deploy to staging service first (pick one consumer service)
   - Test all middleware: `VerifIdemKey`, `VerifCsrf`, `RevokeToken`
   - Test Redis operations: locks, store/get, idempotency
   - Test RabbitMQ: audit trail logging
   - Monitor for 24-48 hours before wider rollout

4. **Deployment Strategy**
   ```bash
   # Tag the release
   git tag v1.x.y
   git push origin v1.x.y
   
   # Consumer services update
   go get bitbucket.org/paycloudid/paycloudhelper@v1.x.y
   go mod tidy
   ```

5. **Communication**
   - Document breaking changes in release notes
   - Notify teams of required updates (for MAJOR versions)
   - Update consumer service documentation

## Production-Safe Development Workflows

### Workflow 1: Adding New Features (MINOR Version)

1. **Design Phase**
   - Ensure backward compatibility: new features MUST NOT change existing behavior
   - Add optional parameters with defaults (avoid breaking function signatures)
   - Example: Adding `InitSentryWithOptions()` alongside existing `InitSentry()`

2. **Implementation**
   ```go
   // ✅ GOOD: New function, doesn't break existing code
   func NewFeature(opts ...Option) error {
       // implementation
   }
   
   // ❌ BAD: Changes existing function signature
   // func ExistingFunc(old, NEW string) error { }
   ```

3. **Testing**
   - Add unit tests for new feature
   - Test with existing consumer service (no changes to consumer)
   - Verify `go test ./...` passes

4. **Documentation**
   - Update this file with usage examples
   - Add inline comments for exported functions
   - Update consumer service examples if needed

### Workflow 2: Bug Fixes (PATCH Version)

1. **Identify Issue**
   - Reproduce in isolated test case
   - Check if fix changes public API behavior (if yes, needs careful review)

2. **Fix Implementation**
   ```go
   // ✅ GOOD: Internal fix, same behavior for consumers
   func GetRedis(id string) (string, error) {
       // Fixed: Added timeout handling
       ctx, cancel := context.WithTimeout(context.Background(), DefaultRedisTimeout)
       defer cancel()
       // ... rest unchanged
   }
   ```

3. **Regression Testing**
   - Add test case for the bug
   - Run full test suite: `go test ./...`
   - Test in staging with consumer service (verify fix works, no side effects)

### Workflow 3: Deprecation (Prepare for MAJOR)

1. **Mark as Deprecated**
   ```go
   // Deprecated: Use NewFunction instead. Will be removed in v2.0.0.
   func OldFunction() error {
       return NewFunction() // Redirect to new implementation
   }
   ```

2. **Add New Function** (MINOR version)
   - Release new function in v1.x.0
   - Keep old function working (redirect internally)

3. **Communication Period**
   - Document in release notes
   - Give teams 2-3 months to migrate
   - Monitor usage (add deprecation warnings if possible)

4. **Remove in MAJOR** (v2.0.0)
   - Only after all consumer services have migrated

### Workflow 4: Middleware Changes

**Critical**: Middleware runs on every request in production services

1. **Testing Requirements**
   - Test with valid requests (happy path)
   - Test with invalid headers (error handling)
   - Test Redis failures (circuit breaking)
   - Load test: 1000+ req/s for 5 minutes

2. **Performance Impact**
   ```go
   // ✅ GOOD: Efficient, minimal allocations
   func VerifCsrf(next echo.HandlerFunc) echo.HandlerFunc {
       return func(c echo.Context) error {
           csrf := c.Request().Header.Get("X-Xsrf-Token")
           // ... validate ...
       }
   }
   
   // ❌ BAD: Allocates on every request
   // func VerifCsrf(next echo.HandlerFunc) echo.HandlerFunc {
   //     config := loadConfig() // Don't do this per-request
   // }
   ```

3. **Error Handling**
   - Always return proper HTTP status codes
   - Log errors with context: function name, request path
   - Don't panic (recover if third-party code might panic)

### Workflow 5: Dependency Updates

1. **Review Changes**
   ```bash
   go list -u -m all
   go get -u github.com/labstack/echo/v4  # Update specific dep
   ```

2. **Test Matrix**
   - Run tests with new dependency
   - Test in staging with consumer service
   - Check for deprecated API usage

3. **Pin Critical Versions**
   - Redis: `github.com/go-redis/redis/v8` (v8 is stable)
   - Echo: Major version in go.mod
   - AMQP: Pin to tested version

## Common Pitfalls to Avoid

### ❌ Breaking Changes Disguised as Fixes
```go
// BAD: Changed default behavior (breaks consumers expecting old behavior)
func InitRedis(opt redis.Options) {
    // Changed default timeout from 1s to 5s ← BREAKING
}
```

### ❌ Nil Pointer Panics
```go
// BAD: Can panic if redisOptions is nil
func GetOptions() redis.Options {
    return *redisOptions  // panic if nil
}

// GOOD: Safe nil handling
func GetRedisPoolClient() (*redis.Client, error) {
    if redisOptions == nil {
        return nil, errors.New("nil redis options")
    }
    // ...
}
```

### ❌ Race Conditions in Init
```go
// BAD: Multiple goroutines can race
func InitRedSync() {
    if redisSync == nil {  // Race: check-then-act
        redisSync = redsync.New(pool)
    }
}

// GOOD: Use sync.Once
func InitRedSyncOnce() error {
    redisSyncInitOnce.Do(func() {
        // Guaranteed to run once
    })
    return redisSyncInitErr
}
```

### ❌ Unbounded Goroutines
```go
// GOOD: Already implemented correctly
func LogAudittrailData(...) {
    go func() {  // Non-blocking async
        pushMessageAudit(messagePayload)
    }()
}
// Note: Single goroutine per call, short-lived, safe pattern
```

## Testing Approach
- **Unit tests**: `go test ./...` before every commit
- **Integration tests**: Test with consumer service in staging
- **Load tests**: For middleware changes (1000+ req/s)
- **Backward compat**: Verify old consumer code still works unchanged
- **Manual testing**: Deploy to staging service for 24-48h before production

## Improvement Roadmap
See `.github/IMPROVEMENTS.md` for:
- Detailed analysis of current codebase issues
- Prioritized improvement plan (P0-P3)
- Implementation roadmap with timelines
- Backward compatibility strategies
- Testing and rollback procedures
