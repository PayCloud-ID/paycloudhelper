# settlement-manager — Redis v8→v9 + Pool Delegation Plan (Group B)

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Move `paycloud-be-settlement-manager` off `github.com/go-redis/redis/v8` (EOL) and off its own pool, onto paycloudhelper's single go-redis **v9** pool — preserving exact cache semantics. Also delete its **dead redsync code**.

**Architecture:** Two phases. **Phase A (v8→v9):** swap the go-redis import path, rename the two v8-only option fields, and **remove** the unused redsync wiring entirely (this service never locks). **Phase B (delegate):** replace `redis.NewClient(...)` + `Ping` in `InitRedis()` with `pch.InitializeRedisWithRetry(...)` + `pch.GetRedisPoolClient()`, built from the service's existing `helpers.Getenv(...)` calls. Provider funcs (`RedisSet/RedisGet/RedisList/RedisArr`) stay behavior-identical. Direct `RedisPoolClient` readers move to a `GetRedisPoolClient()` accessor.

**Tech Stack:** Go · `github.com/PayCloud-ID/paycloudhelper@v1.10.2` · `github.com/redis/go-redis/v9`

**Why this is Group B:** the provider funcs use **v8** `*redis.Client`, incompatible with pch's **v9** client. The v8→v9 swap (Phase A) must precede delegation (Phase B). Unlike `settlementpg-module`, this service has **no distributed lock** — its `RedisSync`/`InitRedSync` are dead code to delete (smaller, simpler migration), but it has **direct `RedisPoolClient` var reads** (healthcheck + shutdown) to reroute.

**Critical facts (verified against code):**
- Provider: `providers/redis.go`. Uses `go-redis/redis/v8 v8.11.5` + `goredis/v8` + `redsync/v4`. v8-only fields: **`IdleTimeout`** (line 54), **`MaxConnAge`** (line 56).
- Env vars (via `helpers.Getenv(key, default...)`): `REDIS_HOST`(def `127.0.0.1`), `REDIS_PORT`(def `6379`), `REDIS_USERNAME`(def `default`), **`REDIS_PASS`** (no default), `REDIS_DB` (**no default — `InitRedis` errors if unset**; preserve).
- **redsync is DEAD CODE:** no `NewMutex` / `RedisSync.*` usage anywhere outside the provider (verified). `RedisSync` var + `InitRedSync()` + the `goredis/v8` & `redsync/v4` imports can be deleted.
- **No double pool:** only its own `providers.InitRedis()` runs (`config/app.go:56,61`, inside a 5-attempt retry loop).
- **Direct `RedisPoolClient` external reads (5):** `main.go:73` (`redisClient: providers.RedisPoolClient` → `serverContext.redisClient`, closed nil-safely at `main.go:100-101`), `controllers/healthcheck.go:28,31` and `:164,167` (nil check + `Ping`).
- Provider funcs: `GetRedisClient`, `InitRedis`, `InitRedSync`(dead), `RedisSet`, `RedisGet`, `RedisList`, `RedisArr`.
- Call sites (15): `RedisArr`×4, `RedisGet`×4, `RedisSet`×1 (+1 commented), `RedisList`×1, `RedisPoolClient`×5, `InitRedis`×2. Only the 5 `RedisPoolClient` reads + 2 close/struct sites change; the `Redis*` op call sites do **not**.
- Latent race: unsynchronized `RedisPoolClient` read/write — fixed for free once pch owns the pool.

**Base reference:** `docs/plans/2026-05-25-redis-provider-migration.md` (analysis F1–F10; Group B addendum).

**Service dir:** `/Users/natan/go/src/paycloud-be-settlement-manager`
**Provider file:** `providers/redis.go`

---

## Task 0: Baseline + version gate

**Step 0.1 — Clean baseline**
```bash
cd /Users/natan/go/src/paycloud-be-settlement-manager
go build ./... && go test ./... && go test -race ./... && echo BASELINE_GREEN
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

## Task 1: Phase A — v8→v9 imports + delete dead redsync

**Files:** `providers/redis.go`

**Step 1.1 — Swap imports and drop redsync**

Find (lines ~13-19):
```go
	pch "github.com/PayCloud-ID/paycloudhelper"
	"github.com/PayCloud-ID/paycloudhelper/phjson"

	"github.com/go-redis/redis/v8"
	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v8"
```
Replace with:
```go
	pch "github.com/PayCloud-ID/paycloudhelper"
	"github.com/PayCloud-ID/paycloudhelper/phjson"

	"github.com/redis/go-redis/v9"
```

**Step 1.2 — Delete the dead redsync declarations + function**

Find (lines ~21-22):
```go
var RedisPoolClient *redis.Client
var RedisSync *redsync.Redsync
```
Replace with (drop `RedisSync`; `RedisPoolClient` is removed in Task 2):
```go
var RedisPoolClient *redis.Client
```

Delete the entire `InitRedSync()` function (lines ~85-101) — the whole `func InitRedSync() error { ... }` block.

**Step 1.3 — Rename v8-only option fields in `InitRedis()`**

In the `redis.Options{...}` literal (lines ~46-57):
- `IdleTimeout: 10 * time.Minute` → `ConnMaxIdleTime: 10 * time.Minute`
- `MaxConnAge:  0` → `ConnMaxLifetime: 0`

**Step 1.4 — Remove the `InitRedSync()` call inside `InitRedis()`**

Find (lines ~75-79):
```go
	// Initialize RedSync after Redis is initialized
	err = InitRedSync()
	if err != nil {
		pch.LogW("[InitRedis] Warning Failed to initialize RedisSync err=%s", err.Error())
	}

	return RedisPoolClient, err
```
Replace with:
```go
	return RedisPoolClient, nil
```

**Step 1.5 — Compile checkpoint**
```bash
go build ./...
```
Expected: compiles (provider on v9, own pool; redsync gone).

---

## Task 2: Phase B — delegate the connection + add accessor

**Files:** `providers/redis.go`

**Step 2.1 — Replace `InitRedis()`**

Replace the whole function with a delegating version (same `helpers.Getenv` calls → `REDIS_PASS` and no-default `REDIS_DB` preserved):
```go
func InitRedis() (*redis.Client, error) {
	rdHost := helpers.Getenv("REDIS_HOST", "127.0.0.1")
	rdPort := helpers.Getenv("REDIS_PORT", "6379")
	rdUsername := helpers.Getenv("REDIS_USERNAME", "default")
	rdPassword := helpers.Getenv("REDIS_PASS")
	rdDB, err := strconv.Atoi(helpers.Getenv("REDIS_DB"))
	if err != nil {
		pch.LogE("[InitRedis] REDIS_DB invalid err=%s", err.Error())
		return nil, err
	}

	pch.LogI("[InitRedis] Starting... %s:%s/%v", rdHost, rdPort, rdDB)
	if err := pch.InitializeRedisWithRetry(pch.RedisInitOptions{
		Options: redis.Options{
			Addr:            rdHost + ":" + rdPort,
			Username:        rdUsername,
			Password:        rdPassword,
			DB:              rdDB,
			MaxRetries:      3,
			MinRetryBackoff: 10 * time.Millisecond,
			MaxRetryBackoff: 500 * time.Millisecond,
			ConnMaxIdleTime: 10 * time.Minute,
			ConnMaxLifetime: 0,
			PoolSize:        10 * runtime.GOMAXPROCS(0),
		},
		MaxRetries: 3,
		RetryDelay: 500 * time.Millisecond,
		FailFast:   true,
	}); err != nil {
		helpers.LoggerErrorHub(err)
		return nil, err
	}

	client, err := pch.GetRedisPoolClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), getRedisTimeout())
	defer cancel()
	client.Do(ctx, "CLIENT", "SETNAME", helpers.Getenv("APP_NAME"))
	pch.LogI("[InitRedis] %v open redis pool connection successfully", client.ClientGetName(ctx))
	return client, nil
}
```

**Step 2.2 — Replace `GetRedisClient()` and the `RedisPoolClient` var with accessors**

Find (lines ~21, ~24-30):
```go
var RedisPoolClient *redis.Client
...
func GetRedisClient() (rc *redis.Client, err error) {
	if RedisPoolClient == nil {
		pch.LogW("[GetRedisClient] empty redis pool... init new redis client")
		return InitRedis()
	}
	return RedisPoolClient, nil
}
```
Replace with (remove the package var; add a nil-able accessor for the healthcheck/shutdown readers):
```go
func GetRedisClient() (rc *redis.Client, err error) {
	if c, e := pch.GetRedisPoolClient(); e == nil {
		return c, nil
	}
	pch.LogW("[GetRedisClient] pool not ready, initializing")
	return InitRedis()
}

// GetRedisPoolClient returns the shared paycloudhelper client (nil if Redis is
// unavailable). Signature kept for the healthcheck and shutdown handle.
func GetRedisPoolClient() *redis.Client {
	c, _ := pch.GetRedisPoolClient()
	return c
}
```

**Step 2.3 — Leave the op funcs UNCHANGED**

Do not touch `getRedisTimeout()`, `RedisSet`, `RedisGet`, `RedisList`, `RedisArr` — they call `GetRedisClient()` and marshal/`Set` directly (no double-encode). `RedisGet`/`RedisList` compare against `redis.Nil` — now v9's sentinel, same package, compatible.

**Step 2.4 — Build + tidy**
```bash
go build ./... && go mod tidy
```
Expected: compiles; `go.mod` drops direct `go-redis/redis/v8` and `redsync/v4` if unused elsewhere (verify redsync isn't used by another package first — see Task 3.2).

---

## Task 3: Reroute the 5 direct `RedisPoolClient` readers

**Files:** `main.go`, `controllers/healthcheck.go`

**Step 3.1 — `main.go` struct assignment**

Find (line ~73):
```go
		redisClient:           providers.RedisPoolClient,
```
Replace:
```go
		redisClient:           providers.GetRedisPoolClient(),
```
The shutdown close (`main.go:100-101`) is already nil-guarded (`if s.redisClient != nil { s.redisClient.Close() }`) — leave it; it closes pch's pool on shutdown.

**Step 3.2 — `controllers/healthcheck.go` (two blocks: ~28-31 and ~164-167)**

Replace each `providers.RedisPoolClient` read with the accessor. Block 1:
```go
		rc := providers.GetRedisPoolClient()
		if rc == nil {
			return errors.New("redis pool not initialized")
		}
		if _, err = rc.Ping(c.Request().Context()).Result(); err != nil {
			pch.LogE("[ReadyCheck] ERR REDIS CONN err=%v", err)
			return fmt.Errorf("redis error: %v", err)
		}
```
Block 2 (the `errs = append` variant):
```go
	rc := providers.GetRedisPoolClient()
	if rc == nil {
		errs = append(errs, "redis pool not initialized")
	} else if _, err = rc.Ping(c.Request().Context()).Result(); err != nil {
		pch.LogE("[ReadyCheck] ERR REDIS CONN err=%v", err)
		errs = append(errs, fmt.Sprintf("redis error: %v", err))
	}
```

**Step 3.3 — Confirm no stray readers + redsync truly gone**
```bash
grep -rn "providers.RedisPoolClient\|providers.RedisSync\|providers.InitRedSync" . --include="*.go" | grep -v "_test.go"   # expect: no output
grep -rn "go-redis/redis/v8\|goredis/v8\|redsync" . --include="*.go" | grep -v "_test.go"                                  # expect: no output
```
If `redsync` still appears in another package, that package has its own usage — handle separately (out of scope here).

---

## Task 4: Verify

```bash
cd /Users/natan/go/src/paycloud-be-settlement-manager
go build ./... && go vet ./... && go test ./... && go test -race ./...
grep -rn "redis.NewClient" . --include="*.go" | grep -v "_test.go"   # expect: no output (single pool)
```
Expected: all pass.

**Manual smoke (real Redis):** run the service, hit the readiness/health endpoint (proves accessor + Ping), exercise a partner/config read (`RedisArr(ConfigPartners)`) and a settlement-batch cache write (`RedisSet pg_admin+<id>`). Confirm values round-trip and one client connection (one pool) via `SETNAME`.

---

## Task 5: Commit

```bash
cd /Users/natan/go/src/paycloud-be-settlement-manager
git add providers/redis.go main.go controllers/healthcheck.go
git commit -m "refactor(redis): migrate v8->v9, delete dead redsync, delegate pool

Phase A: go-redis/redis/v8 -> redis/go-redis/v9 (IdleTimeout->ConnMaxIdleTime,
MaxConnAge->ConnMaxLifetime); removed unused RedisSync/InitRedSync + redsync deps.
Phase B: InitRedis/GetRedisClient delegate to pch.InitializeRedisWithRetry +
pch.GetRedisPoolClient(), built from existing helpers.Getenv (REDIS_PASS,
no-default REDIS_DB preserved). Direct RedisPoolClient readers (healthcheck,
shutdown handle) routed through GetRedisPoolClient(). Single pool; resolves
go-redis v8 EOL exposure.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Rollback
```bash
git revert <commit> && git revert <bump-commit>
go get github.com/PayCloud-ID/paycloudhelper@v1.9.1 && go mod tidy && go build ./...
```

## Parity checklist
- [ ] Imports v9 only; no `go-redis/redis/v8`, `goredis/v8`, or `redsync`
- [ ] `RedisSync` + `InitRedSync()` deleted; no caller references remain
- [ ] `REDIS_PASS` still read; `REDIS_DB` still errors when unset
- [ ] Pool tuning preserved (`10*GOMAXPROCS`, `ConnMaxIdleTime 10m`, `ConnMaxLifetime 0`)
- [ ] `RedisSet/RedisGet/RedisList/RedisArr` bodies unchanged
- [ ] All 5 `RedisPoolClient` reads routed through `GetRedisPoolClient()`; shutdown close still nil-safe
- [ ] `go test -race ./...` green
