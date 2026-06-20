# config-module — Redis Pool Delegation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make `paycloud-be-config-module` source its `*redis.Client` from paycloudhelper's single pool, while keeping its `Cache` interface and `RedisImpl` methods (`DeleteByPattern`, `GetMasterByFunc`, `StoreMasterByFunc`, SCAN helpers) fully intact.

**Architecture:** "Delegate, don't rewrite." Change **only** the connection construction inside `InitRedis()`: replace `redis.NewClient(...)` + `Ping` with `pch.InitializeRedisWithRetry(...)` + `pch.GetRedisPoolClient()`. The returned `*redis.Client` is stored in `RedisImpl.client` exactly as before. Every method, the `Cache` interface, `initRedSync`, and all SCAN/pattern logic are untouched. DI wiring in `server.go` is unchanged.

**Tech Stack:** Go 1.25 · `github.com/PayCloud-ID/paycloudhelper@v1.10.2` · `github.com/redis/go-redis/v9` · `github.com/PayCloud-ID/paycloudhelper/phjson`

**Why this is the safest of the four:** `RedisImpl` is a struct holding one `*redis.Client`. We only change where that pointer comes from. `RedisImpl.Store` marshals with `phjson` then calls `client.Set(string(jsonData), ...)` directly — it never routes through `pch.StoreRedisWithContext`, so there is **no double-encode risk**.

**Critical facts (verified):**
- Password is `cfg.Redis.Password` ← **`REDIS_PWD`** (`configs/env.go:174`). Build options from `cfg`, never `pch.InitRedisFromEnv()`.
- DB default is **0** (`configs/env.go:172`) — matches pch default, but pass `cfg.Redis.DB` explicitly anyway.
- Pool sizing: `cfg.Redis.PoolSize` (`REDIS_POOL_SIZE`, default 0 → fallback `4*GOMAXPROCS`), `MinIdleConns` fallback 5, plus `DialTimeout/ReadTimeout/WriteTimeout`. Preserve.
- `cfg.Redis.Timeout` (`REDIS_TIMEOUT`, default 500ms) was the ping timeout. Reuse as the retry base delay + the `CLIENT SETNAME` ctx.
- No consumer calls `cache.Sync()` or acquires a redsync lock (verified) — `initRedSync` stays but is non-critical.

**Base reference:** `docs/plans/2026-05-25-redis-provider-migration.md` (analysis F1–F10).

**Service dir:** `/Users/natan/go/src/paycloud-be-config-module`

---

## Task 0: Baseline + version gate

**Step 0.1 — Clean baseline**
```bash
cd /Users/natan/go/src/paycloud-be-config-module
go build ./... && go test ./... && echo BASELINE_GREEN
```

**Step 0.2 — Bump + verify symbols**
```bash
go get github.com/PayCloud-ID/paycloudhelper@v1.10.2 && go mod tidy
go doc github.com/PayCloud-ID/paycloudhelper InitializeRedisWithRetry
go doc github.com/PayCloud-ID/paycloudhelper GetRedisPoolClient
```
Expected: both print signatures. If not found, stop and escalate.

**Step 0.3 — Commit bump**
```bash
git add go.mod go.sum && git commit -m "chore(deps): bump paycloudhelper to v1.10.2"
```

---

## Task 1: Delegate the connection inside `InitRedis()`

**Files:**
- Modify: `providers/redis.go` (function `InitRedis` only)

**Step 1.1 — Replace the body of `InitRedis()`**

Find the function (lines ~32-86) — from `func InitRedis() (*RedisImpl, error) {` through its `return impl, nil`. Replace the **connection construction** while keeping the pool-size/min-idle logic and the `RedisImpl`/`initRedSync` tail:

```go
func InitRedis() (*RedisImpl, error) {
	cfg := configs.Get()
	pchelper.LogI("[InitRedis] connecting host=%s port=%s db=%d", cfg.Redis.Host, cfg.Redis.Port, cfg.Redis.DB)

	poolSize := cfg.Redis.PoolSize
	if poolSize <= 0 {
		poolSize = defaultRedisPoolMultiplier * runtime.GOMAXPROCS(0)
	}
	minIdle := cfg.Redis.MinIdleConns
	if minIdle <= 0 {
		minIdle = 5
	}

	opts := redis.Options{
		Addr:            cfg.Redis.Host + ":" + cfg.Redis.Port,
		DB:              cfg.Redis.DB,
		MaxRetries:      3,
		MinRetryBackoff: 10 * time.Millisecond,
		MaxRetryBackoff: 500 * time.Millisecond,
		PoolSize:        poolSize,
		MinIdleConns:    minIdle,
		DialTimeout:     cfg.Redis.DialTimeout,
		ReadTimeout:     cfg.Redis.ReadTimeout,
		WriteTimeout:    cfg.Redis.WriteTimeout,
	}
	if cfg.Redis.Username != "" {
		opts.Username = cfg.Redis.Username
	}
	if cfg.Redis.Password != "" {
		opts.Password = cfg.Redis.Password
	}

	// Delegate pool ownership to paycloudhelper (single process-wide pool).
	if err := pchelper.InitializeRedisWithRetry(pchelper.RedisInitOptions{
		Options:    opts,
		MaxRetries: 3,
		RetryDelay: cfg.Redis.Timeout,
		FailFast:   true,
	}); err != nil {
		helpers.LoggerErrorHub(err)
		return nil, err
	}

	client, err := pchelper.GetRedisPoolClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Redis.Timeout)
	defer cancel()
	client.Do(ctx, "CLIENT", "SETNAME", cfg.App.Name)
	pchelper.LogI("[InitRedis] pool connected name=%s", client.ClientGetName(ctx))

	impl := &RedisImpl{client: client}
	if err = impl.initRedSync(); err != nil {
		pchelper.LogW("[InitRedis] RedisSync init failed err=%v", err)
	}
	return impl, nil
}
```

**Step 1.2 — Leave EVERYTHING else in the file unchanged**

Do not touch: the `Cache` interface, `RedisImpl` struct, `Client()`, `Sync()`, `Ping()`, `MasterDuration()`, `Store()`, `Delete()`, `DeleteByPattern()`, `Get()`, `GetKeyList()`, `GetMasterByFunc()`, `StoreMasterByFunc()`, `RemoveMasterByFunc()`, `CheckConn()`, `initRedSync()`, and the `getKeyRedisStore`/`checkKeyArr*` helpers.

**Step 1.3 — Fix imports**

`redis` (Options type), `runtime` (GOMAXPROCS), `context`, `time`, `configs`, `helpers`, `pchelper`, `phjson`, `goredis`, `redsync`, `errgroup`, `strings` all remain in use. Run:
```bash
goimports -w providers/redis.go
go build ./...
```
If the compiler flags an unused import, remove only that one.

---

## Task 2: Verify (server.go DI + shutdown are already compatible)

**Step 2.1 — Confirm DI + shutdown need no change**

`server.go:135` (`if v, err := providers.InitRedis(); …`) still works — same return type. `server.go:316` (`s.cache.Client().Close()`) still works — `Client()` returns the pch-owned client; closing it on shutdown is correct. No edits required. Confirm:
```bash
grep -n "providers.InitRedis\|s.cache.Client()" server.go
```

**Step 2.2 — Static + unit + race**
```bash
cd /Users/natan/go/src/paycloud-be-config-module
go build ./... && go vet ./... && go test ./... && go test -race ./...
```
Expected: all pass — including `configs/load_test.go` (env parsing) and any provider tests.

**Step 2.3 — Single pool, no stray client**
```bash
grep -rn "redis.NewClient" . --include="*.go" | grep -v "_test.go"   # expect: no output
```

**Step 2.4 — gRPC cache smoke (manual)**

With real env, run the service and exercise a cached gRPC path (e.g. `snapbi` read → `cache.Get`, then `cache.StoreMasterByFunc`) and a `DeleteByPattern` path (e.g. reapply merchant config). Confirm values round-trip and pattern deletes still clear keys (proves SCAN + key format unchanged).

---

## Task 3: Commit

```bash
cd /Users/natan/go/src/paycloud-be-config-module
git add providers/redis.go
git commit -m "refactor(redis): delegate connection pool to paycloudhelper

InitRedis() now builds options from configs.Get() (REDIS_PWD, pool sizing
preserved) and obtains the client via pch.InitializeRedisWithRetry +
pch.GetRedisPoolClient(). Cache interface, RedisImpl methods, SCAN/pattern
helpers, and DI wiring unchanged. Eliminates the duplicate connection pool.

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

## Rollback
```bash
git revert <commit>
go get github.com/PayCloud-ID/paycloudhelper@v1.10.1 && go mod tidy && go build ./...
```

## Parity checklist
- [ ] Password via `cfg.Redis.Password` (`REDIS_PWD`)
- [ ] `cfg.Redis.DB` passed explicitly
- [ ] PoolSize / MinIdleConns / timeouts preserved
- [ ] `Cache` interface + all `RedisImpl` methods byte-for-byte unchanged
- [ ] `RedisImpl.Store` still marshals with `phjson` then `Set` (no double-encode)
- [ ] DI in `server.go` untouched
- [ ] `go test -race ./...` green
