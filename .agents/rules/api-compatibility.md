# API Compatibility & Versioning — paycloudhelper

## Semantic Versioning Contract

Multiple production PayCloud services depend on this library. Breaking changes cascade to all consumers simultaneously.

| Bump | When | Safety |
|------|------|--------|
| **PATCH** `v1.6.x → v1.6.y` | Bug fixes only, zero behavior change | Auto-update safe |
| **MINOR** `v1.x → v1.y.0` | New features, backward-compatible | Consumers upgrade freely |
| **MAJOR** `v1 → v2` | Breaking changes | Requires simultaneous consumer updates |

## Breaking Change Detection Checklist

**Anything below is a MAJOR bump required:**

- Changing an existing function signature (adding required parameters)
- Removing exported functions, types, or fields
- Changing return types
- Changing behavior that consumers rely on (e.g. default timeout values)
- Removing struct fields that consumers may marshal/unmarshal

## Backward-Compatible Addition Pattern

```go
// ✅ SAFE: New function alongside old — consumers unaffected
func InitializeRedisWithRetry(opts RedisInitOptions) error { ... }
// Old InitializeRedis(opt) still works unchanged

// ✅ SAFE: Variadic options for zero-breaking extensions
func NewMiddleware(opts ...MiddlewareOption) echo.HandlerFunc { ... }

// ❌ BREAKING: Adds required param to existing function
func InitializeRedis(opt redis.Options, poolSize int) error { ... }
```

## Deprecation Workflow

1. Add `// Deprecated: Use NewFunc instead. Will be removed in v2.0.0.` comment
2. Redirect old function to new internally: `func OldFunc() error { return NewFunc() }`
3. Release as MINOR version
4. Wait for all consumer services to migrate (2-3 months minimum)
5. Only then remove in a MAJOR bump

## Release Procedure

```bash
# 1. Run all tests
go test ./...

# 2. Check public API hasn't changed unexpectedly
go doc -all | grep "^func"

# 3. Check for known broken versions to avoid repeating
# See go.mod retract section

# 4. Tag and push
git tag v1.x.y
git push origin v1.x.y

# 5. Consumer services update
go get bitbucket.org/paycloudid/paycloudhelper@v1.x.y
go mod tidy
```

## Known Retracted Versions (do not repeat these mistakes)

| Version | Issue |
|---------|-------|
| `v1.6.3` | Redis error logging too verbose (noisy logs) |
| `v1.6.0` | Unsafe audit trail push — race condition |
| `v1.5.2` | Init bug — panic on nil options |

## New Parameter Rules

- **Optional parameters**: Use `...Option` or `*Config` with nil-safe defaults
- **Required parameters**: Only acceptable in new functions, never in existing
- **Config structs**: Zero values must be safe defaults (avoid requiring field population)

## Integration Testing Before Release

1. Deploy to staging with **one** consumer service first
2. Test all middleware paths: `VerifIdemKey`, `VerifCsrf`, `RevokeToken`
3. Test Redis operations: locks, store/get, idempotency
4. Test RabbitMQ: audit trail logging
5. Monitor 24–48h before wider rollout
