# clientpg-manager — Redis Pool Delegation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make `paycloud-be-clientpg-manager` use paycloudhelper's single Redis pool instead of its own, with **zero behavioral change** to its 75 Redis call sites (auth/session hot path).

**Architecture:** "Delegate, don't rewrite." Keep the whole `providers` Redis API. Replace only `InitRedis` / `getRedisClient` to source the client from `pch.GetRedisPoolClient()`, built from the service's existing `config.Redis*()` getters (so `REDIS_PWD` and `REDIS_DB=9` default are preserved). Provider functions, their `encoding/json` marshaling, and the `RedisTimeout` (5s) wrappers stay untouched.

**Tech Stack:** Go 1.25 · `github.com/PayCloud-ID/paycloudhelper@v1.10.2` · `github.com/redis/go-redis/v9`

**Why low-risk despite 75 call sites:** none of the call sites change. They call `providers.StoreRedis` / `GetRedis` / `GetRedisPoolClient` etc.; those functions keep their exact bodies and only fetch the client from a different source.

**Critical facts (verified):**
- Password comes from `config.RedisPassword()` → `REDIS_PWD` (`config/env.go:281`). **Must build options from this getter**, never `pch.InitRedisFromEnv()` (which reads `REDIS_PASSWORD`).
- DB default is **9** (`config/env.go:282`). Preserve via `config.RedisDB()`.
- Pool tuning: `PoolSize:50, MinIdleConns:10, DialTimeout:3s, ReadTimeout:2s, WriteTimeout:2s, PoolTimeout:3s, ConnMaxLifetime:5m` (`providers/redis.go:33-41`). Preserve all.
- `main.go:50` logs-and-continues on init failure (NOT fatal). `main.go:53` does `defer providers.RedisPoolClient.Close()` — this var is being removed, so the defer must change.

**Base reference:** `docs/plans/2026-05-25-redis-provider-migration.md` (analysis F1–F10).

**Service dir:** `/Users/natan/go/src/paycloud-be-clientpg-manager`

---

## Task 0: Baseline + version gate

**Step 0.1 — Clean baseline**
```bash
cd /Users/natan/go/src/paycloud-be-clientpg-manager
go build ./... && go test ./... && echo BASELINE_GREEN
```
Expected: `BASELINE_GREEN`.

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
- Modify: `providers/redis.go`

**Step 1.1 — Replace `InitRedis()`**

Find (lines ~27-54, the function that does `RedisPoolClient = redis.NewClient(...)` + Ping):
```go
func InitRedis() (*redis.Client, error) {
	RedisPoolClient = redis.NewClient(&redis.Options{ ... })
	...
	res, err := RedisPoolClient.Ping(ctx).Result()
	...
	return RedisPoolClient, nil
}
```
Replace with:
```go
// InitRedis initializes the process-wide paycloudhelper Redis pool using this
// service's config getters (REDIS_PWD via config.RedisPassword(), REDIS_DB
// default 9 via config.RedisDB()) and returns the shared client. Pool tuning
// matches the previous per-service client exactly.
func InitRedis() (*redis.Client, error) {
	if err := pchelper.InitializeRedisWithRetry(pchelper.RedisInitOptions{
		Options: redis.Options{
			Addr:            config.RedisHost() + ":" + config.RedisPort(),
			Username:        "default",
			Password:        config.RedisPassword(),
			DB:              config.RedisDB(),
			MaxRetries:      3,
			PoolSize:        50,
			MinIdleConns:    10,
			DialTimeout:     3 * time.Second,
			ReadTimeout:     2 * time.Second,
			WriteTimeout:    2 * time.Second,
			PoolTimeout:     3 * time.Second,
			ConnMaxLifetime: 5 * time.Minute,
		},
		MaxRetries: 3,
		RetryDelay: 500 * time.Millisecond,
		FailFast:   true,
	}); err != nil {
		pchelper.LogE("[InitRedis] error initializing redis pool, err=%v", err)
		return nil, err
	}
	return pchelper.GetRedisPoolClient()
}
```

**Step 1.2 — Replace `getRedisClient()`**

Find (lines ~56-62):
```go
func getRedisClient() (rc *redis.Client, err error) {
	if RedisPoolClient == nil {
		pchelper.LogI("[getRedisClient] empty redis pool... init new redis client")
		return InitRedis()
	}
	return RedisPoolClient, nil
}
```
Replace with:
```go
func getRedisClient() (rc *redis.Client, err error) {
	if c, e := pchelper.GetRedisPoolClient(); e == nil {
		return c, nil
	}
	pchelper.LogI("[getRedisClient] pool not ready, initializing")
	return InitRedis()
}
```

**Step 1.3 — Replace exported `GetRedisPoolClient()` and remove the `RedisPoolClient` var; add `CloseRedis()`**

Find (lines ~17-25):
```go
var (
	RedisPoolClient      *redis.Client
	RedisDefaultDuration = 60 * time.Second
	RedisTimeout         = 5 * time.Second
)

func GetRedisPoolClient() *redis.Client {
	return RedisPoolClient
}
```
Replace with:
```go
var (
	RedisDefaultDuration = 60 * time.Second
	RedisTimeout         = 5 * time.Second
)

// GetRedisPoolClient returns the shared paycloudhelper client (nil if Redis is
// unavailable). Signature preserved for existing callers (middleware, health).
func GetRedisPoolClient() *redis.Client {
	c, _ := pchelper.GetRedisPoolClient()
	return c
}

// CloseRedis closes the shared pool on shutdown; nil-safe.
func CloseRedis() {
	if c, err := pchelper.GetRedisPoolClient(); err == nil && c != nil {
		_ = c.Close()
	}
}
```

> `RedisDefaultDuration` and `RedisTimeout` are kept — the wrapper functions (`StoreRedis`, `GetRedis`, `DeleteRedis`) use `RedisTimeout`, and callers may reference `RedisDefaultDuration`.

**Step 1.4 — Leave all operation functions UNCHANGED**

Do **not** touch `StoreRedisCtx`, `StoreRedis`, `GetRedisCtx`, `GetRedis`, `DeleteRedisCtx`, `DeleteRedis`, `CheckRedisConn`. They call `getRedisClient()` and marshal with `encoding/json` directly — behavior is identical, no double-encode.

**Step 1.5 — Fix imports**
```bash
goimports -w providers/redis.go
```
`context`, `encoding/json`, `time`, `errgroup`, `config`, `redis`, `pchelper` all remain in use.

---

## Task 2: Fix the shutdown defer in `main.go`

**Files:**
- Modify: `main.go`

**Step 2.1 — Replace the var-based close**

Find (line ~53):
```go
defer providers.RedisPoolClient.Close()
```
Replace with:
```go
defer providers.CloseRedis()
```
This removes a nil-pointer panic risk when init fails (old code logged and continued; the old var was non-nil so `.Close()` was safe — the new helper restores that safety).

**Step 2.2 — Confirm no other reader of the removed var**
```bash
grep -rn "providers.RedisPoolClient" . --include="*.go" | grep -v "_test.go"
```
Expected: **no output**. (`providers.GetRedisPoolClient()` — the function — may still appear; that's fine.)

---

## Task 3: Verify

**Step 3.1 — Static + unit + race**
```bash
cd /Users/natan/go/src/paycloud-be-clientpg-manager
go build ./... && go vet ./... && go test ./... && go test -race ./...
```
Expected: all pass.

**Step 3.2 — Single pool, no v8**
```bash
grep -rn "redis.NewClient" . --include="*.go" | grep -v "_test.go"   # expect: no output
grep -rn "go-redis/redis/v8" . --include="*.go"                      # expect: no output
```

**Step 3.3 — Auth/session smoke (manual)**

With real env (`REDIS_HOST/PORT/PWD/DB`), run the service and exercise: login (writes session keys), a CSRF-protected request, a logout (deletes a key), and `/health`. Confirm sessions read back correctly (proves key format + marshaling unchanged) and `/health` reports Redis healthy.

---

## Task 4: Commit

```bash
cd /Users/natan/go/src/paycloud-be-clientpg-manager
git add providers/redis.go main.go
git commit -m "refactor(redis): delegate connection pool to paycloudhelper

InitRedis/getRedisClient/GetRedisPoolClient now source the client from
pch.GetRedisPoolClient(), built from config.Redis*() getters (REDIS_PWD and
DB=9 default preserved, pool tuning identical). All 75 call sites and their
encoding/json marshaling unchanged. Shutdown uses nil-safe CloseRedis().
Eliminates the duplicate connection pool.

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

## Rollback
```bash
git revert <commit>
go get github.com/PayCloud-ID/paycloudhelper@v1.10.1 && go mod tidy && go build ./...
```

## Parity checklist
- [ ] Password via `config.RedisPassword()` (`REDIS_PWD`) — not `REDIS_PASSWORD`
- [ ] `config.RedisDB()` (default 9) preserved
- [ ] Pool tuning (50/10/3s/2s/2s/3s/5m) preserved
- [ ] `GetRedisPoolClient()` signature unchanged (returns `*redis.Client`)
- [ ] No call-site edits except `main.go` shutdown defer
- [ ] `go test -race ./...` green
