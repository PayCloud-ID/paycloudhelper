---
name: library-maintenance
description: Guides versioning, deprecation, backward compatibility, and release workflow for the paycloudhelper shared Go library.
applyTo: '**/*.go'
---
# Library Maintenance — paycloudhelper

## This Is a Shared Library

Changes here affect **all** PayCloud microservices that import `paycloudhelper`. Design for the consumer, not the implementation.

## Adding a New Feature (MINOR version)

### Step 1: Design for Backward Compatibility
- New function must NOT change any existing exported function
- New parameters must be optional (variadic or pointer with nil = default)
- If changing behavior is unavoidable → create a new function, deprecate old

### Step 2: Implement
```go
// New function with optional config
func NewFeature(opts ...NewFeatureOption) error {
    cfg := defaultNewFeatureConfig()
    for _, o := range opts {
        o(&cfg)
    }
    // implementation
}
```

### Step 3: Test
```bash
go test ./...       # must pass
go test -race ./... # must pass for any init/goroutine changes
```

### Step 4: Release
```bash
git tag v1.X.0
git push origin v1.X.0
```

## Fixing a Bug (PATCH version)

1. Write a test that reproduces the bug
2. Fix the bug
3. Verify `go test ./...` passes (including the new regression test)
4. Tag: `v1.x.Y` (bump patch only)

## Deprecating a Function

```go
// Deprecated: Use InitializeRedisWithRetry instead. Will be removed in v2.0.0.
// InitializeRedis initializes a Redis connection with default settings.
func InitializeRedis(opt redis.Options) {
    // Redirect to new implementation
    _ = InitializeRedisWithRetry(RedisInitOptions{Options: opt})
}
```

## Subpackage Rules

| Subpackage | Independence | May Import Root? |
|-----------|-------------|-----------------|
| `phhelper/` | Standalone | No |
| `phlogger/` | Standalone | No |
| `phsentry/` | Standalone | No |
| `phaudittrailv0/` | Standalone | No |
| `phjson/` | Standalone | No |
| Root (`.`) | Uses subpackages | — |

**Never** import root package from a subpackage — circular dependency.

## RabbitMQ / Audit Trail Patterns

```go
// Async audit trail — never block
go func() {
    pushMessageAudit(payload)
}()

// ✅ Pattern used by LogAudittrailData/LogAudittrailProcess
// Single goroutine per call, short-lived — SAFE
// ❌ Never create goroutine pools or unbounded goroutines
```

## Logger Usage in This Library

```go
// Root package: use aliases from logger.go
LogI("[FunctionName] status: %s", value)
LogE("[FunctionName] error: %v", err)
LogW("[FunctionName] warning: %s", msg)
LogD("[FunctionName] debug: %s", detail)

// Subpackages: use phlogger directly
phlogger.Log.Infof("[FunctionName] ...")
```

## Common Pitfalls

```go
// ❌ Race condition in init
var once sync.Once  // declare at package level
func initSomething() {
    if globalVar == nil { globalVar = ... }  // BAD — race
}

// ✅ Correct
func initSomething() {
    once.Do(func() { globalVar = ... })
}

// ❌ Panic on nil pointer
func GetOptions() redis.Options {
    return *redisOptions  // panics if nil
}

// ✅ Safe nil check
func GetOptions() (redis.Options, error) {
    if redisOptions == nil {
        return redis.Options{}, errors.New("redis not initialized")
    }
    return *redisOptions, nil
}

// ❌ Breaking change disguised as improvement
func InitializeRedis(opt redis.Options, timeout time.Duration) {} // was 1 param

// ✅ Backward-compatible extension
func InitializeRedisWithTimeout(opt redis.Options, timeout time.Duration) {}
```

## `.github/` Docs Reference

| File | Contents |
|------|---------|
| `IMPROVEMENTS.md` | Backlog of known issues, P0–P3 prioritization |
| `P0-IMPLEMENTATION-SUMMARY.md` | Completed P0 improvements |
| `P1-IMPLEMENTATION-SUMMARY.md` | Completed P1 improvements |
| `P2-IMPLEMENTATION-SUMMARY.md` | Completed P2 improvements |

Always check `IMPROVEMENTS.md` before starting a new improvement to avoid duplicate work.
