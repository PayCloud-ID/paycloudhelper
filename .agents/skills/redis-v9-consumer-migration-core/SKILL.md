---
name: redis-v9-consumer-migration-core
description: Guides PayCloud consumer services through safe migration from paycloudhelper v1.x to v2.x with redis/go-redis/v9 compatibility and rollout controls.
applyTo: '**/*.go, **/go.mod, **/.env*, **/config*.go'
---

# Redis v9 Consumer Migration (Core)

## Use When

- A consumer service upgrades `bitbucket.org/paycloudid/paycloudhelper` to `v2.x`.
- The service currently imports `github.com/go-redis/redis/v8` directly.
- Teams need a production-safe rollout and rollback path.

## Breaking Change Surface

1. `go-redis` import path changes to `github.com/redis/go-redis/v9`.
2. Types in signatures from paycloudhelper now use v9 package symbols.
3. Rebuild is required in each consumer service after dependency update.

## Migration Steps

1. Update module dependency:

```bash
go get bitbucket.org/paycloudid/paycloudhelper@v2.0.0
go mod tidy
```

2. Replace direct v8 imports:

```go
// before
import redis "github.com/go-redis/redis/v8"

// after
import redis "github.com/redis/go-redis/v9"
```

3. Keep redis startup via paycloudhelper stable entrypoint:

```go
err := pchelper.InitializeRedisWithRetry(pchelper.RedisInitOptions{
    Options: redis.Options{Addr: cfg.RedisAddr, DB: cfg.RedisDB},
    MaxRetries: 3,
    RetryDelay: 100 * time.Millisecond,
    FailFast: true,
})
if err != nil {
    return fmt.Errorf("init redis: %w", err)
}
```

4. Validate lock paths still treat contention as non-fatal (`acquired=false`, `err=nil`).
5. Run build + tests + race detector in service repo.

## Required Validation Gates

- `go build ./...`
- `go vet ./...`
- `go test ./...`
- `go test -race ./...` (mandatory for init/lock usage)

## Rollout Pattern

1. Deploy to staging with Redis integration tests.
2. Observe lock and cache metrics for 24h.
3. Roll production gradually (canary or shard-based).
4. Roll back by pinning previous paycloudhelper version if needed.
