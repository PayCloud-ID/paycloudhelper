# clientpg-module — Redis Pool Delegation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make `paycloud-be-clientpg-module` use paycloudhelper's single Redis connection pool instead of opening its own, **without changing any Redis call-site behavior**.

**Architecture:** "Delegate, don't rewrite." Keep the entire `providers` Redis API and every function body. Replace only the connection bootstrap (`InitRedis` / `getRedisClient`) so the client is sourced from `pch.GetRedisPoolClient()`. The service's own `redisOptionsFromEnv()` (which reads `REDIS_PWD`, `REDIS_TLS`, pool sizing) is reused verbatim to build pch's options — so password, TLS, and tuning are preserved.

**Tech Stack:** Go 1.25 · `github.com/PayCloud-ID/paycloudhelper@v1.10.2` · `github.com/redis/go-redis/v9`

**Why this service is low-risk:** its provider functions marshal with `encoding/json` and call `client.Set(...)` directly — they never use pch's marshaling, so there is **no double-encode risk**. They already fetch the client through `getRedisClient()`, so changing that one function reroutes everything.

**Base reference:** `docs/plans/2026-05-25-redis-provider-migration.md` (analysis F1–F10).

**Service dir:** `/Users/natan/go/src/paycloud-be-clientpg-module`

---

## Task 0: Baseline + version gate

**Step 0.1 — Confirm clean baseline**

```bash
cd /Users/natan/go/src/paycloud-be-clientpg-module
go build ./... && go test ./... && echo BASELINE_GREEN
```
Expected: `BASELINE_GREEN`. If red, stop — fix or report before migrating.

**Step 0.2 — Bump paycloudhelper and verify required symbols exist**

```bash
go get github.com/PayCloud-ID/paycloudhelper@v1.10.2
go mod tidy
go doc github.com/PayCloud-ID/paycloudhelper InitializeRedisWithRetry
go doc github.com/PayCloud-ID/paycloudhelper RedisInitOptions
go doc github.com/PayCloud-ID/paycloudhelper GetRedisPoolClient
```
Expected: all three `go doc` calls print a signature. If any is "not found", **stop** — the target version lacks the API; escalate to pick the correct version.

**Step 0.3 — Commit the bump alone**
```bash
git add go.mod go.sum && git commit -m "chore(deps): bump paycloudhelper to v1.10.2"
```

---

## Task 1: Delegate the connection in `providers/redis.go`

**Files:**
- Modify: `providers/redis.go` (functions `InitRedis`, `getRedisClient`; remove `RedisClient` var + `redisClientMu`; add `GetRedisClient` accessor)

**Step 1.1 — Replace `InitRedis()`**

Find (lines ~47-51):
```go
func InitRedis() (*redis.Client, error) {
	return getRedisClient()
}
```
Replace with:
```go
// InitRedis initializes the process-wide paycloudhelper Redis pool using this
// service's env configuration (REDIS_PWD, REDIS_TLS, pool sizing preserved via
// redisOptionsFromEnv) and returns the shared client.
func InitRedis() (*redis.Client, error) {
	opts, err := redisOptionsFromEnv()
	if err != nil {
		return nil, err
	}
	if err := pchelper.InitializeRedisWithRetry(pchelper.RedisInitOptions{
		Options:    *opts,
		MaxRetries: 3,
		RetryDelay: 500 * time.Millisecond,
		FailFast:   true,
	}); err != nil {
		return nil, fmt.Errorf("init redis pool: %w", err)
	}
	return pchelper.GetRedisPoolClient()
}
```

> Note: the existing import alias is `pchelper` (see top of file). Use it; do not add a second alias.

**Step 1.2 — Replace `getRedisClient()`**

Find (lines ~95-117, the whole function with `redisClientMu`):
```go
func getRedisClient() (*redis.Client, error) {
	redisClientMu.Lock()
	defer redisClientMu.Unlock()
	if RedisClient != nil {
		return RedisClient, nil
	}
	opts, err := redisOptionsFromEnv()
	...
	RedisClient = rc
	return rc, nil
}
```
Replace the entire function with:
```go
// getRedisClient returns the shared paycloudhelper pool, initializing it lazily
// on first use. Always fetches from pch (fast atomic-load path) so a lazy
// reconnect inside pch is never masked by a stale cached pointer.
func getRedisClient() (*redis.Client, error) {
	if c, err := pchelper.GetRedisPoolClient(); err == nil {
		return c, nil
	}
	return InitRedis()
}

// GetRedisClient is an exported accessor for callers that need the raw client
// (e.g. health checks). Prefer this over the removed RedisClient package var.
func GetRedisClient() (*redis.Client, error) {
	return getRedisClient()
}
```

**Step 1.3 — Remove the now-dead `RedisClient` var and `redisClientMu`**

Find (lines ~23-27):
```go
var (
	RedisClient    *redis.Client
	redisClientMu  sync.Mutex
	errRedisNotSet = errors.New("REDIS_HOST is empty")
)
```
Replace with:
```go
var errRedisNotSet = errors.New("REDIS_HOST is empty")
```

**Step 1.4 — Fix imports**

Remove `"sync"` if it is now unused (it was only used by `redisClientMu`). Keep `"crypto/tls"`, `"errors"`, `"fmt"`, `"os"`, `"strconv"`, `"time"`, `"strings"`, `"log"`, `errgroup`, `redis`, `pchelper` — they are still used by `redisOptionsFromEnv`, the master/pattern helpers, and `CheckRedisConn`.

```bash
goimports -w providers/redis.go   # or: go build ./... will report unused
```

**Step 1.5 — Verify `redisOptionsFromEnv`, `withDefaultTimeout`, and all public funcs are UNCHANGED**

Confirm by grep that the following still exist and were not edited: `StoreRedis`, `DeleteRedis`, `DeleteRedisByPattern`, `GetRedis`, `GetRedisKeyList`, `GetRedisMasterByFunc`, `StoreRedisMasterByFunc`, `GetRedisMasterDuration`, `CheckRedisConn`, `redisOptionsFromEnv`, `withDefaultTimeout`.
```bash
grep -c "func " providers/redis.go   # function count should only drop by the removed lazy-init lines, public API intact
```

---

## Task 2: Update the one `RedisClient` package-var reader

**Files:**
- Modify: `controllers/healthcheck.go`

**Step 2.1 — Replace direct `providers.RedisClient` reads**

`controllers/healthcheck.go` references `providers.RedisClient` (≈ lines 42, 89) and `providers.InitRedis()` (≈ line 92). Replace the var reads with the new accessor.

Pattern — find:
```go
redisClient := providers.RedisClient
```
Replace:
```go
redisClient, rcErr := providers.GetRedisClient()
if rcErr != nil {
	// existing "redis unavailable" branch — report unhealthy as before
}
```
Apply the same to the second read (`rc := providers.RedisClient`). The `rc, err = providers.InitRedis()` re-init call stays as-is (still valid).

> Keep the existing health semantics: if the client is unavailable, return the same status/HTTP code the handler returned before. Do not change the response shape.

**Step 2.2 — Confirm no other file reads the removed var**
```bash
grep -rn "providers.RedisClient" . --include="*.go" | grep -v "_test.go"
```
Expected: **no output**. If any remain, replace them with `providers.GetRedisClient()`.

---

## Task 3: Verify (build, vet, race) + parity smoke

**Step 3.1 — Static + unit + race**
```bash
cd /Users/natan/go/src/paycloud-be-clientpg-module
go build ./...
go vet ./...
go test ./...
go test -race ./...
```
Expected: all pass. No `go-redis/redis/v8` and no duplicate-pool code paths remain.

**Step 3.2 — Confirm single pool (no stray `redis.NewClient`)**
```bash
grep -rn "redis.NewClient" . --include="*.go" | grep -v "_test.go"
```
Expected: **no output** (the only client now comes from pch).

**Step 3.3 — Local round-trip smoke (manual, against a real/local Redis)**

Export the same env the service uses (`REDIS_HOST`, `REDIS_PORT`, `REDIS_PWD`, `REDIS_DB`), run the service, hit the health endpoint, and confirm a store/get works through any endpoint that caches. Confirm logs show paycloudhelper's `initRedisClient` prefix (one pool) and **not** two separate "redis pool connection" lines.

---

## Task 4: Commit

```bash
cd /Users/natan/go/src/paycloud-be-clientpg-module
git add providers/redis.go controllers/healthcheck.go
git commit -m "refactor(redis): delegate connection pool to paycloudhelper

InitRedis/getRedisClient now source the client from pch.GetRedisPoolClient()
built from this service's existing redisOptionsFromEnv (REDIS_PWD, REDIS_TLS,
pool sizing preserved). All provider functions and call sites unchanged —
exact behavioral parity. Eliminates the duplicate connection pool.

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

## Rollback

```bash
git revert <commit>   # or: git checkout HEAD~1 -- providers/redis.go controllers/healthcheck.go
go get github.com/PayCloud-ID/paycloudhelper@v1.10.1 && go mod tidy && go build ./...
```

## Parity checklist (must all hold)

- [ ] `REDIS_PWD` still used (not `REDIS_PASSWORD`) — via `redisOptionsFromEnv`
- [ ] `REDIS_TLS` path preserved — via `redisOptionsFromEnv`
- [ ] DB index unchanged (env-driven, default 0 — same as before)
- [ ] `encoding/json` marshaling in provider funcs untouched (no double-encode)
- [ ] No call-site signature changed
- [ ] `go test -race ./...` green
