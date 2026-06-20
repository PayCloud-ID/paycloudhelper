# transaction-module — Redis Pool Delegation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make `paycloud-be-transaction-module` use paycloudhelper's single Redis pool, while **exactly preserving** its cache-miss semantics, distributed-lock keys/TTLs, and per-call timeouts.

**Architecture:** "Delegate, don't rewrite." Replace only `InitRedis` (build pool) and `GetRedisClient` (fetch client). Everything that carries behavior — `withRedisTimeout`, `StoreRedis(Context)`, `GetRedis(Context)` (which **swallows `redis.Nil`**), the redsync lock functions, `InitRedSync`, `redisLockKey`, `RedisDefaultDuration`, `RedisLockMainKey` — stays byte-for-byte. The service keeps its own `RedisSync` instance built over pch's pool, so lock keys and TTLs are identical to today.

**Tech Stack:** Go 1.25 · `github.com/PayCloud-ID/paycloudhelper@v1.10.2` · `github.com/redis/go-redis/v9` · redsync/v4 · phjson

**Why this is the most delicate of the four — and how each risk is neutralized:**
- **F5 (redis.Nil):** `GetRedisContext` returns `("", nil)` on a miss; pch's `GetRedisWithContext` returns `("", redis.Nil)`. Some callers (incl. in-package `providers/payment_channel.go:50`) branch on `err == nil`. → We **keep `GetRedisContext` untouched**, so the miss semantics never change.
- **F6 (lock TTL):** service default lock TTL = process timeout (3000ms); pch default = 2000ms. → We **keep the service's lock functions and `helpers.GetTrxRedisLockTimeout()`**, never pch's lock TTL.
- **F7 (lock key prefix):** service uses `"redis_lock:transaction:"`; pch rewrites its own `redisLockKey` to `"redis_lock:<APP_NAME>:"`. → We **keep the service's `RedisSync` + `redisLockKey`**; pch's lock key global is irrelevant because the service never calls `pch.StoreRedisWithLock`.
- **F8 (timeout):** ops use `withRedisTimeout` (`helpers.GetTrxRedisTimeout()`, 250ms) wrapping `client.Set/Get` directly — **not** pch's `DefaultRedisTimeout`. Preserved.
- **F9 (in-package callers):** because the provider API is preserved, `providers/*.go` internal callers need no edits.

**Critical facts (verified):**
- Password = `config.RedisPassword()` → `REDIS_PASSWORD`, default "" (`config/env.go:499`). DB = `config.RedisDB()`, **default 9** (`config/env.go:506`). Build options from these getters.
- Pool: `PoolSize: 10*GOMAXPROCS(0)`, `ConnMaxIdleTime: 10m`, `ConnMaxLifetime: 0` (`providers/redis.go:55-66`). Preserve.
- 4 sites read the package var `providers.RedisPoolClient` directly — these must move to the `GetRedisClient()` accessor (Task 2).

**Base reference:** `docs/plans/2026-05-25-redis-provider-migration.md` (analysis F1–F10).

**Service dir:** `/Users/natan/go/src/paycloud-be-transaction-module`

---

## Task 0: Baseline + version gate

**Step 0.1 — Clean baseline (race included — this service has lock concurrency)**
```bash
cd /Users/natan/go/src/paycloud-be-transaction-module
go build ./... && go test ./... && go test -race ./... && echo BASELINE_GREEN
```
Expected: `BASELINE_GREEN`. If red, stop.

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

## Task 1: Delegate the connection in `providers/redis.go`

**Files:**
- Modify: `providers/redis.go` (`InitRedis`, `GetRedisClient`, vars block)

**Step 1.1 — Remove the `RedisPoolClient` package var**

Find (lines ~21-27):
```go
var (
	RedisPoolClient      *redis.Client
	RedisDefaultDuration = 180 * time.Second
	RedisSync            *redsync.Redsync
	RedisLockMainKey     = "redis_lock:"
	redisLockKey         = RedisLockMainKey + "transaction:"
)
```
Replace with (drop only `RedisPoolClient`):
```go
var (
	RedisDefaultDuration = 180 * time.Second
	RedisSync            *redsync.Redsync
	RedisLockMainKey     = "redis_lock:"
	redisLockKey         = RedisLockMainKey + "transaction:"
)
```

**Step 1.2 — Replace `InitRedis()`**

Find the function (lines ~48-89). Replace its body to delegate to pch while preserving all option fields and the `SETNAME` + `InitRedSync` tail:
```go
// InitRedis initializes the process-wide paycloudhelper Redis pool from this
// service's config (REDIS_PASSWORD; REDIS_DB default 9) and bootstraps RedSync.
// Pool tuning matches the previous per-service client.
func InitRedis() (*redis.Client, error) {
	redisDb := config.RedisDB()
	redisHost := config.RedisHost()
	redisPort := config.RedisPort()
	redisPassword := config.RedisPassword()

	pchelper.LogI("[InitRedis] starting host=%s port=%s db=%v", redisHost, redisPort, redisDb)

	if err := pchelper.InitializeRedisWithRetry(pchelper.RedisInitOptions{
		Options: redis.Options{
			Addr:            redisHost + ":" + redisPort,
			Username:        "default",
			Password:        redisPassword,
			DB:              redisDb,
			MaxRetries:      3,
			MinRetryBackoff: 10 * time.Millisecond,
			MaxRetryBackoff: 500 * time.Millisecond,
			ConnMaxLifetime: 0,                // no limit (v9)
			ConnMaxIdleTime: 10 * time.Minute, // v9 (was IdleTimeout)
			PoolSize:        10 * runtime.GOMAXPROCS(0),
		},
		MaxRetries: 3,
		RetryDelay: 1 * time.Second,
		FailFast:   true,
	}); err != nil {
		helpers.LoggerErrorHub(err)
		return nil, err
	}

	client, err := pchelper.GetRedisPoolClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), helpers.GetTrxRedisTimeout())
	defer cancel()
	client.Do(ctx, "CLIENT", "SETNAME", config.AppName())
	pchelper.LogI("[InitRedis] pool_connected client_name=%v", client.ClientGetName(ctx))

	// Initialize RedSync after Redis is initialized (service-owned instance over pch pool)
	if err := InitRedSync(); err != nil {
		pchelper.LogW("[InitRedis] RedisSync init failed err=%s", err.Error())
	}

	return client, nil
}
```

**Step 1.3 — Replace `GetRedisClient()`**

Find (lines ~36-42):
```go
func GetRedisClient() (rc *redis.Client, err error) {
	if RedisPoolClient == nil {
		pchelper.LogW("[GetRedisClient] pool empty initializing")
		return InitRedis()
	}
	return RedisPoolClient, nil
}
```
Replace with:
```go
// GetRedisClient returns the shared paycloudhelper client, initializing the pool
// on first use. Always fetches from pch (fast atomic-load path) so a pch lazy
// reconnect is never masked by a stale pointer.
func GetRedisClient() (rc *redis.Client, err error) {
	if c, e := pchelper.GetRedisPoolClient(); e == nil {
		return c, nil
	}
	pchelper.LogW("[GetRedisClient] pool empty initializing")
	return InitRedis()
}
```

**Step 1.4 — Leave the rest of the file UNCHANGED**

Do not touch: `withRedisTimeout`, `StoreRedis`, `StoreRedisContext`, `StoreRedisWithLock`, `StoreRedisWithLockContext`, `GetRedis`, `GetRedisContext` (keep the `redis.Nil → "", nil` swallow), `AcquireLockContext`, `ReleaseLockContext`, `InitRedSync`. `InitRedSync` still reads `if RedisSync == nil` and builds from `GetRedisClient()` — now the pch pool. Lock keys/TTLs unchanged.

**Step 1.5 — Imports unchanged; tidy**
```bash
go build ./...   # reports anything unused
```
`context`, `errors`, `fmt`, `runtime`, `time`, `config`, `helpers`, `redsync`, `goredis`, `redis`, `pchelper`, `phjson` all remain in use.

---

## Task 2: Route the 4 direct `RedisPoolClient` readers through the accessor

These four read the removed var directly. Each must use `providers.GetRedisClient()` (reconnect-aware). One site (`scheduled/set_expired_daily.go:113`) already uses the accessor — leave it.

**Files:** `app/runtime.go`, `services/healthcheck.go`, `services/transaction_expiry.go`

**Step 2.1 — `app/runtime.go:106`**

Find:
```go
redisFn: func() *redis.Client { return providers.RedisPoolClient },
```
Replace:
```go
redisFn: func() *redis.Client { c, _ := providers.GetRedisClient(); return c },
```

**Step 2.2 — `services/healthcheck.go` (≈ lines 56-59)**

Find:
```go
if providers.RedisPoolClient == nil {
	// ...existing unhealthy branch...
}
if _, err := providers.RedisPoolClient.Ping(ctx).Result(); err != nil {
	// ...existing unhealthy branch...
}
```
Replace with:
```go
rc, rcErr := providers.GetRedisClient()
if rcErr != nil || rc == nil {
	// ...existing unhealthy branch (same status/response as before)...
}
if _, err := rc.Ping(ctx).Result(); err != nil {
	// ...existing unhealthy branch...
}
```
Preserve the exact health response shape/status the handler returned before.

**Step 2.3 — `services/transaction_expiry.go` (≈ lines 377, 388)**

This uses the raw client for an atomic `SET … NX EX` and a `DEL` (custom expiry lock). Fetch the client first.

Find (line ~377):
```go
result, err := providers.RedisPoolClient.Do(ctx, "SET", key, value, "NX", "EX", int(expiration.Seconds())).Result()
```
Replace:
```go
rc, rcErr := providers.GetRedisClient()
if rcErr != nil {
	return /* same zero-values + */ rcErr
}
result, err := rc.Do(ctx, "SET", key, value, "NX", "EX", int(expiration.Seconds())).Result()
```
> Match the enclosing function's exact return signature in the error branch. Read the function header before editing.

Find (line ~388):
```go
_, err := providers.RedisPoolClient.Del(ctx, key).Result()
```
Replace:
```go
rc, rcErr := providers.GetRedisClient()
if rcErr != nil {
	return rcErr
}
_, err := rc.Del(ctx, key).Result()
```
> Again, match the enclosing function's return signature.

**Step 2.4 — Confirm the var is fully gone**
```bash
grep -rn "providers.RedisPoolClient" . --include="*.go" | grep -v "_test.go"
```
Expected: **no output**. (`providers.RedisDefaultDuration`, `providers.RedisLockMainKey`, `providers.GetRedisClient` are all still valid and may appear elsewhere — that's correct.)

---

## Task 3: Verify

**Step 3.1 — Static + unit + race (race mandatory — locks)**
```bash
cd /Users/natan/go/src/paycloud-be-transaction-module
go build ./... && go vet ./... && go test ./... && go test -race ./...
```
Expected: all pass.

**Step 3.2 — Single pool, no v8**
```bash
grep -rn "redis.NewClient" . --include="*.go" | grep -v "_test.go"   # expect: no output
grep -rn "go-redis/redis/v8" . --include="*.go"                      # expect: no output
```

**Step 3.3 — Lock + cache-miss smoke (manual, against real Redis)**

1. **Cache miss parity:** call an endpoint that reads a non-existent key; confirm it behaves as a miss (not an error) — proves `GetRedisContext`'s `redis.Nil` swallow is intact.
2. **Lock parity:** trigger two concurrent `StoreRedisWithLock` on the same TicketId; confirm exactly one wins and the lock key is still `redis_lock:transaction:<id>` (inspect with `redis-cli KEYS 'redis_lock:transaction:*'` during the window).
3. **Scheduler lock:** confirm `set_expired_daily` still acquires `redis_lock:transaction-scheduler:set-expired-scheduled`.
4. `/health` reports Redis healthy.

---

## Task 4: Commit

```bash
cd /Users/natan/go/src/paycloud-be-transaction-module
git add providers/redis.go app/runtime.go services/healthcheck.go services/transaction_expiry.go
git commit -m "refactor(redis): delegate connection pool to paycloudhelper

InitRedis/GetRedisClient source the client from pch.GetRedisPoolClient()
built from config.Redis*() (REDIS_PASSWORD, DB=9, pool tuning preserved).
Cache-miss semantics (redis.Nil swallow), service-owned RedSync, lock keys
(redis_lock:transaction:) and TTLs are unchanged. Direct RedisPoolClient
readers routed through GetRedisClient(). Eliminates the duplicate pool.

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

## Rollback
```bash
git revert <commit>
go get github.com/PayCloud-ID/paycloudhelper@v1.10.1 && go mod tidy && go build ./...
```

## Parity checklist
- [ ] Password via `config.RedisPassword()` (`REDIS_PASSWORD`); DB via `config.RedisDB()` (default 9)
- [ ] Pool: `10*GOMAXPROCS`, `ConnMaxIdleTime 10m`, `ConnMaxLifetime 0`
- [ ] `GetRedisContext` still swallows `redis.Nil` (cache-miss parity)
- [ ] Lock key prefix `redis_lock:transaction:` and TTL from `helpers.GetTrxRedisLockTimeout()` unchanged
- [ ] Per-op timeout still `helpers.GetTrxRedisTimeout()` (not pch DefaultRedisTimeout)
- [ ] No `providers.RedisPoolClient` references remain
- [ ] `go test -race ./...` green
