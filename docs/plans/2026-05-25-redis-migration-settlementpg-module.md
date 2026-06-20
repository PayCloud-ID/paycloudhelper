# settlementpg-module — Redis v8→v9 + Pool Delegation Plan (Group B)

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Move `paycloud-be-settlementpg-module` off `github.com/go-redis/redis/v8` (EOL) and off its own connection pool, onto paycloudhelper's single go-redis **v9** pool — preserving its distributed-lock behavior and exact cache semantics.

**Architecture:** Two phases. **Phase A (v8→v9):** swap the provider's go-redis import path and rename the two v8-only option fields; the redsync pool adapter moves `goredis/v8 → goredis/v9` (redsync v4 API is unchanged). **Phase B (delegate):** replace `redis.NewClient(...)` + `Ping` inside `InitRedis()` with `pch.InitializeRedisWithRetry(...)` + `pch.GetRedisPoolClient()`, built from the service's existing `helpers.Getenv(...)` calls. The provider's public funcs (`RedisSet/RedisGet/RedisList/RedisArr`), `RedisSync`, and the lock code in `app/services/transaction.go` stay behavior-identical.

**Tech Stack:** Go · `github.com/PayCloud-ID/paycloudhelper@v1.10.2` · `github.com/redis/go-redis/v9` · `github.com/go-redsync/redsync/v4` + `redis/goredis/v9`

**Why this is Group B (heavier than the v9 services):** the 4 v9 services in the base plan could delegate directly because their provider funcs already used v9 types. This service's funcs use **v8** `*redis.Client`, which is incompatible with pch's **v9** client — so the v8→v9 swap (Phase A) must happen *before* delegation (Phase B).

**Critical facts (verified against code):**
- Provider: `app/providers/redis.go`. Uses `go-redis/redis/v8 v8.11.5` + `goredis/v8`. v8-only fields: **`IdleTimeout`** (line 56) and **`MaxConnAge`** (line 58).
- Env vars (via `helpers.Getenv(key, default...)`): `REDIS_HOST`(def `127.0.0.1`), `REDIS_PORT`(def `6379`), `REDIS_USERNAME`(def `default`), **`REDIS_PASS`** (no default — third password-var spelling in the fleet), `REDIS_DB` (**no default — `InitRedis` errors if unset**; preserve this).
- **redsync IS used:** `app/services/transaction.go:133-146` checks `providers.RedisSync == nil`, calls `providers.InitRedSync()`, then `providers.RedisSync.NewMutex(lockName)` + `mutex.Lock()` loop. Lock key: `lock:TransactionSettlementSummary:<date>:<merchant>`. **Must preserve.**
- **No double pool:** the service never calls a paycloudhelper Redis init — only its own `providers.InitRedis()` (`main.go:57,62`, inside a 5-attempt retry loop).
- **No shutdown `Close()`** of the redis pool anywhere.
- Provider funcs: `GetRedisClient`, `InitRedis`, `InitRedSync`, `RedisSet`, `RedisGet`, `RedisList`, `RedisArr`.
- Call sites (8, all in `app/services/transaction.go`): `RedisGet`×4, `RedisSet`×3, `RedisSync`/`InitRedSync` lock block. **None change.**
- Latent race: `GetRedisClient`/`InitRedis` read/write `RedisPoolClient` unsynchronized — fixed for free once pch (atomic pool) owns the client.

**Base reference:** `docs/plans/2026-05-25-redis-provider-migration.md` (analysis F1–F10; Group B addendum).

**Service dir:** `/Users/natan/go/src/paycloud-be-settlementpg-module`
**Provider file:** `app/providers/redis.go`

---

## Task 0: Baseline + version gate

**Step 0.1 — Clean baseline (race included — this service holds a distributed lock)**
```bash
cd /Users/natan/go/src/paycloud-be-settlementpg-module
go build ./... && go test ./... && go test -race ./... && echo BASELINE_GREEN
```
Expected: `BASELINE_GREEN`. If red, stop.

**Step 0.2 — Bump paycloudhelper + verify symbols**
```bash
go get github.com/PayCloud-ID/paycloudhelper@v1.10.2 && go mod tidy
go doc github.com/PayCloud-ID/paycloudhelper InitializeRedisWithRetry
go doc github.com/PayCloud-ID/paycloudhelper GetRedisPoolClient
```
Expected: both print signatures. If "not found", stop and escalate (target version lacks the API).

> `go mod tidy` will keep `redis/go-redis/v9` (already an indirect dep) and should drop the direct `go-redis/redis/v8` once Phase A removes its imports (Task 2). Don't hand-edit `go.mod`.

**Step 0.3 — Commit bump**
```bash
git add go.mod go.sum && git commit -m "chore(deps): bump paycloudhelper to v1.10.2"
```

---

## Task 1: Phase A — migrate the provider imports v8 → v9

**Files:** `app/providers/redis.go`

**Step 1.1 — Swap import paths**

Find (lines ~15-21):
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
	"github.com/go-redsync/redsync/v4"
	goredis "github.com/go-redsync/redsync/v4/redis/goredis/v9"
```

> The v8 adapter was imported as the package name `goredis` implicitly (`goredis/v8`); the v9 path also ends in `goredis` but add the explicit `goredis` alias to be safe. `redsync.New(goredis.NewPool(client))` is unchanged.

**Step 1.2 — Rename the two v8-only option fields (these are renamed in Phase B anyway, but rename now so the file compiles if you build between phases)**

In `InitRedis()` (lines ~48-59), the `redis.Options{...}` literal:
- `IdleTimeout: 10 * time.Minute` → `ConnMaxIdleTime: 10 * time.Minute`
- `MaxConnAge:  0` → `ConnMaxLifetime: 0`

**Step 1.3 — Build (compile-only checkpoint)**
```bash
go build ./...
```
Expected: compiles. (At this point the provider uses v9 with its own pool; Phase B delegates the pool next.)

---

## Task 2: Phase B — delegate the connection in `InitRedis()` / `GetRedisClient()`

**Files:** `app/providers/redis.go`

**Step 2.1 — Replace `InitRedis()`**

Replace the whole function (lines ~34-84) with a version that delegates to pch, reusing the **same** `helpers.Getenv` calls (so `REDIS_PASS` and the no-default `REDIS_DB` behavior are preserved) and keeping the `RedisSync` bootstrap:
```go
func InitRedis() (*redis.Client, error) {
	rdHost := helpers.Getenv("REDIS_HOST", "127.0.0.1")
	rdPort := helpers.Getenv("REDIS_PORT", "6379")
	rdUsername := helpers.Getenv("REDIS_USERNAME", "default")
	rdPassword := helpers.Getenv("REDIS_PASS")
	rdDB, err := strconv.Atoi(helpers.Getenv("REDIS_DB"))
	if err != nil {
		pch.LogE("InitRedis : REDIS_DB invalid err=%s", err.Error())
		return nil, err
	}

	pch.LogI("InitRedis : Starting... %s:%s/%v", rdHost, rdPort, rdDB)
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
		pch.LogE("InitRedis : %v", err)
		return nil, err
	}

	client, err := pch.GetRedisPoolClient()
	if err != nil {
		return nil, err
	}
	RedisPoolClient = client

	ctx, cancel := context.WithTimeout(context.Background(), getRedisTimeout())
	defer cancel()
	client.Do(ctx, "CLIENT", "SETNAME", helpers.Getenv("APP_NAME"))
	pch.LogI("InitRedis : %v", client.ClientGetName(ctx))
	pch.LogI("InitRedis : open redis pool connection successfully")

	if err := InitRedSync(); err != nil {
		pch.LogW("Warning: Failed to initialize RedisSync: %s", err.Error())
	}
	return client, nil
}
```

**Step 2.2 — Replace `GetRedisClient()`**

Find (lines ~26-32):
```go
func GetRedisClient() (rc *redis.Client, err error) {
	if RedisPoolClient == nil {
		pch.LogW("GetRedisClient : empty redis pool... init new redis client")
		return InitRedis()
	}
	return RedisPoolClient, nil
}
```
Replace with:
```go
func GetRedisClient() (rc *redis.Client, err error) {
	if c, e := pch.GetRedisPoolClient(); e == nil {
		return c, nil
	}
	pch.LogW("GetRedisClient : empty redis pool... initializing")
	return InitRedis()
}
```

**Step 2.3 — Leave the rest UNCHANGED**

Do not touch `InitRedSync()` (it calls `GetRedisClient()` → now pch's v9 client; `goredis.NewPool` + `redsync.New` unchanged), `getRedisTimeout()`, `RedisSet`, `RedisGet`, `RedisList`, `RedisArr`. Keep `RedisSync` and `RedisPoolClient` package vars (the lock code in `transaction.go` reads `providers.RedisSync`).

**Step 2.4 — Build + tidy**
```bash
go build ./... && go mod tidy
```
Expected: compiles; `go.mod` no longer lists a direct `go-redis/redis/v8`.

---

## Task 3: Verify

**Step 3.1 — Static + unit + race (mandatory — distributed lock)**
```bash
cd /Users/natan/go/src/paycloud-be-settlementpg-module
go build ./... && go vet ./... && go test ./... && go test -race ./...
```
Expected: all pass.

**Step 3.2 — No v8, single pool**
```bash
grep -rn "go-redis/redis/v8\|goredis/v8" . --include="*.go" | grep -v "_test.go"   # expect: no output
grep -rn "redis.NewClient" . --include="*.go" | grep -v "_test.go"                 # expect: no output
```

**Step 3.3 — Lock + cache smoke (manual, real Redis)**

With real env (`REDIS_HOST/PORT/USERNAME/PASS/DB`), run the service and trigger `UpdateSummaryTransactionSettlement` twice concurrently for the same merchant+day. Confirm:
1. The redsync lock `lock:TransactionSettlementSummary:<date>:<merchant>` still serializes them (inspect with `redis-cli KEYS 'lock:TransactionSettlementSummary:*'` during the window).
2. The settlement-summary cache key round-trips (`RedisGet`/`RedisSet` of `TransactionSettlementSummary*`).
3. `SETNAME` shows one client connection (one pool).

---

## Task 4: Commit

```bash
cd /Users/natan/go/src/paycloud-be-settlementpg-module
git add app/providers/redis.go
git commit -m "refactor(redis): migrate v8->v9 and delegate pool to paycloudhelper

Phase A: go-redis/redis/v8 -> redis/go-redis/v9 (IdleTimeout->ConnMaxIdleTime,
MaxConnAge->ConnMaxLifetime; redsync goredis/v8 -> goredis/v9).
Phase B: InitRedis/GetRedisClient delegate to pch.InitializeRedisWithRetry +
pch.GetRedisPoolClient(), built from existing helpers.Getenv (REDIS_PASS,
no-default REDIS_DB preserved). RedisSync + transaction.go lock code unchanged.
Single connection pool; resolves go-redis v8 EOL exposure.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Rollback
```bash
git revert <commit> && git revert <bump-commit>
go get github.com/PayCloud-ID/paycloudhelper@v1.9.1 && go mod tidy && go build ./...
```

## Parity checklist
- [ ] Imports are v9 only; no `go-redis/redis/v8` or `goredis/v8`
- [ ] `REDIS_PASS` (not `REDIS_PASSWORD`/`REDIS_PWD`) still read; `REDIS_DB` still errors when unset
- [ ] Pool tuning preserved (`10*GOMAXPROCS`, `ConnMaxIdleTime 10m`, `ConnMaxLifetime 0`)
- [ ] `RedisSync.NewMutex` lock in `transaction.go` unchanged; lock key identical
- [ ] `RedisSet/RedisGet/RedisList/RedisArr` bodies unchanged (no double-encode — they marshal then `Set` a string)
- [ ] `go test -race ./...` green
