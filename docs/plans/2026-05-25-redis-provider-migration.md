# Redis Provider Migration — 4 Services

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Remove duplicated `providers/redis.go` from four consumer services so they use `paycloudhelper v1.10.2` for all Redis lifecycle management.

**Architecture:** Each service currently opens its own `*redis.Client` alongside paycloudhelper's pool — two pools per process. After migration every service calls `pch.InitRedisFromEnv()` (or `pch.InitializeRedisWithRetry`) once in startup, then uses `pch.StoreRedisWithContext / GetRedisWithContext / DeleteRedisWithContext / AcquireLock / ReleaseLock` at call sites. `paycloud-be-config-module` is a partial migration (keeps its `Cache` interface) because it has domain-specific SCAN/pattern operations.

**Tech Stack:** Go 1.25 · `github.com/PayCloud-ID/paycloudhelper v1.10.2` · `github.com/redis/go-redis/v9` · `github.com/PayCloud-ID/paycloudhelper/phjson`

**paycloudhelper reference:** `docs/redis-integration.md`

---

## ⚠️ Production Readiness Analysis (2026-05-25 review)

This base plan was re-audited against the **actual** `paycloudhelper/redis.go` and each service's real config. The original "full delete + rewrite call sites" approach below is **NOT production-safe as written**. Ten defects were found, several breaking. The corrected strategy is **"Delegate, don't rewrite"** and lives in the per-service plans.

**Six services, two groups.** Group A is already on go-redis **v9** (delegate only). Group B is still on go-redis **v8** (EOL) and needs a v8→v9 phase *before* delegating. **(2026-06-08: a Group C addendum below adds 5 SNAP-BI interface managers that are on go-redis v8 and need the v9 import bump only — no pool delegation.)**

| Per-service plan | Group | Strategy |
|---|---|---|
| `2026-05-25-redis-migration-clientpg-module.md` | A (v9) | Delegate connection to pch, keep provider API |
| `2026-05-25-redis-migration-transaction-module.md` | A (v9) | Delegate connection, keep provider API + lock semantics |
| `2026-05-25-redis-migration-clientpg-manager.md` | A (v9) | Delegate connection, keep provider API |
| `2026-05-25-redis-migration-config-module.md` | A (v9) | Delegate connection inside `RedisImpl`, keep `Cache` interface |
| `2026-05-25-redis-migration-settlementpg-module.md` | **B (v8→v9)** | v8→v9 first, **keep** redsync lock, then delegate |
| `2026-05-25-redis-migration-settlement-manager.md` | **B (v8→v9)** | v8→v9 first, **delete dead** redsync, then delegate |

### Defects found in the original approach

| # | Defect | Severity | Evidence |
|---|--------|----------|----------|
| **F1** | **Password env var mismatch.** `pch.InitRedisFromEnv()` reads `REDIS_PASSWORD` (`redis.go:259`). `clientpg-module`, `clientpg-manager`, `config-module` all read **`REDIS_PWD`**. Using `InitRedisFromEnv()` → empty password → auth failure / wrong instance. Only `transaction-module` uses `REDIS_PASSWORD`. | 🔴 Breaking | `clientpg-manager/config/env.go:283` (`REDIS_PWD`), `config-module/configs/env.go:174` (`REDIS_PWD`), `clientpg-module/providers/redis.go:76` (`REDIS_PWD`) |
| **F2** | **DB index default mismatch.** `transaction-module` & `clientpg-manager` default `REDIS_DB` to **9**; `pch.InitRedisFromEnv()` defaults to **0** when env unset (`redis.go:247`). If `REDIS_DB` isn't explicitly exported, all keys move to DB 0 → every lookup misses, possible cross-DB collision. | 🔴 Breaking | `config/env.go:506` (`default 9`), `config/env.go:282` (`default 9`) |
| **F3** | **Lost pool/timeout/TLS tuning.** `InitRedisFromEnv()` sets only Addr/Password/DB. Services set `PoolSize`, `MinIdleConns`, `DialTimeout`, `ReadTimeout`, `WriteTimeout`, `ConnMaxLifetime`, `Username`, and `clientpg-module` sets **TLS**. All silently lost → pool exhaustion under load, **no TLS in prod**. | 🔴 High | `clientpg-manager/providers/redis.go:33-41` (pool tuning), `clientpg-module/providers/redis.go:75-92` (TLS + pool) |
| **F4** | **Double-encode bug in base plan Step 1.3.** `StoreRedisMasterByFunc` does `phjson.Marshal(data)` then passes the resulting `string` to `pch.StoreRedisWithContext`, which marshals **again** (`redis.go:372`). Stored value becomes a quoted JSON string → every reader's decode breaks. | 🔴 Breaking | base plan §1.3 vs `redis.go:372` |
| **F5** | **redis.Nil semantic flip.** `transaction-module.GetRedisContext` swallows `redis.Nil` and returns `("", nil)` on a miss. `pch.GetRedisWithContext` returns `("", redis.Nil)`. Call sites such as `providers/payment_channel.go:50` (`if …; errRedis == nil { /*hit*/ }`) flip cache-miss handling. | 🔴 Breaking | `providers/redis.go:204-206`, `providers/payment_channel.go:50` |
| **F6** | **Lock TTL default change.** `transaction-module.GetTrxRedisLockTimeout()` defaults to the **process timeout (3000ms)**; `pch.GetTrxRedisLockTimeout()` defaults to **2000ms**. Migrating to pch's value shrinks the TTL → long jobs lose the lock early → double execution. Mitigation: pass the service's TTL explicitly to `pch.AcquireLock`. | 🟠 Medium | `helpers/transaction.go:39-46` vs `redis.go:680` |
| **F7** | **Lock-key prefix.** `pch.InitializeRedisWithRetry` rewrites `redisLockKey = "redis_lock:<APP_NAME>:"` (`redis.go:84`). `transaction-module` uses `"redis_lock:transaction:"`. Only affects `pch.StoreRedisWithLock` (the service won't call it). `pch.AcquireLock(key,…)` uses the key **verbatim** (`redis.go:505`) → safe **iff** the caller passes its own full key. | 🟠 Medium | `redis.go:84`, `redis.go:505` |
| **F8** | **Operation timeout source.** All pch ops are bounded by the package global `DefaultRedisTimeout` (default **1s**, `redis.go:30`), not the service's per-call timeout. `InitRedisOptions` *adds* `ReadTimeout` onto it (`redis.go:188-189`). transaction-module used 250ms; pch is more lenient (not breaking), config-module used 500ms (pch more lenient). Verify no SLO depends on the tighter bound. | 🟡 Low | `redis.go:30,188-189`; `helpers/transaction.go:16` |
| **F9** | **Internal-package callers.** `transaction-module`'s own `providers/*.go` (e.g. `payment_channel.go`, `redis_transaction_info.go`) call `GetRedisContext` internally. A "delete the file" approach would force edits inside the `providers` package too. The delegate approach avoids this entirely. | 🟠 Scope | `providers/payment_channel.go:50`, `providers/redis_transaction_info.go:63` |
| **F10** | **Target-version symbol check.** The migration must compile against the pinned version. The **delegate** strategy depends only on long-stable APIs — `pch.InitializeRedisWithRetry`, `pch.RedisInitOptions`, `pch.GetRedisPoolClient` — not on newer symbols. Still: verify `go doc` for these after the bump before editing. | 🟡 Gate | `redis.go:73,129` |

### Corrected strategy — "Delegate, don't rewrite"

**Principle:** change **only who owns the connection pool**, not the service's Redis API or semantics.

1. Build pch's options from the service's **existing** config function (`redisOptionsFromEnv()` / `config.Redis*()` / `cfg.Redis`). This preserves `REDIS_PWD`, the DB default, pool sizing, TLS, and Username — neutralising **F1, F2, F3**.
2. Replace the service's internal `redis.NewClient(...)` + `Ping` with `pch.InitializeRedisWithRetry(...)` + `pch.GetRedisPoolClient()`. One process-wide pool (pch's).
3. Keep every public provider function and its body — they now fetch the client via `pch.GetRedisPoolClient()` (fast atomic-load path, lazy reconnect) instead of a private `redis.NewClient`. Call sites are **untouched**, so **F4, F5, F6** cannot occur and **F9** is moot.
4. Where a function caches the client in a package var, re-fetch from pch per call (pch may rebuild the pool on reconnect) — never hold a stale `*redis.Client`.

**Why this is the production-safe choice:** zero call-site edits (no typo risk across 75+ sites), exact behavioral parity (miss/lock/marshal semantics unchanged), trivial rollback (revert one file + `go.mod`), and it leans only on stable pch APIs. The file `providers/redis.go` is **gutted to a thin adapter**, not deleted — collapsing the adapter further is a separate, optional Phase 2 once parity is confirmed in production.

### Group B addendum — go-redis v8 services (`settlementpg-module`, `settlement-manager`)

Added 2026-05-25 after scanning two more services. Both run their **own** provider on **`github.com/go-redis/redis/v8 v8.11.5`** (EOL) and on `paycloudhelper v1.9.1`.

> **Scope correction:** an earlier fleet scan in this effort mislabeled `settlementpg-module` as "already OK (no provider)". That was wrong — its provider is nested at **`app/providers/redis.go`**, which the original `find` (looking only for `providers/redis.go`) missed. It does have a per-service provider.

**Which of the five migration issues actually apply to these two** (verified against code):

| Issue | settlementpg-module | settlement-manager |
|---|---|---|
| #1 Double pool | ❌ N/A — never calls a pch redis init | ❌ N/A |
| #2 Unfixed races | ⚠️ Latent only (unsynchronized `RedisPoolClient` lazy init) | ⚠️ Latent only |
| #3 **go-redis v8 EOL** | ✅ **Primary driver** (`IdleTimeout`/`MaxConnAge` v8 fields) | ✅ **Primary driver** |
| #4 `os.Getenv` lock hot-path | ❌ N/A — no per-op env reads | ❌ N/A |
| #5 Divergent behavior | ✅ (3rd password var spelling `REDIS_PASS`) | ✅ |

So Group B is driven by **#3 (EOL) and #5 (divergence)**, not the double-pool/hot-path issues that motivated Group A. **Urgency is lower; effort-per-service is higher** (v8→v9 API changes precede delegation), but the surface is small (8 and 15 call sites; 4 trivial ops).

**Two extra rules unique to Group B** (the v9 services don't need these):

1. **Phase A (v8→v9) must precede delegation.** The provider funcs use v8 `*redis.Client`, which is **type-incompatible** with pch's v9 client returned by `GetRedisPoolClient()`. Swap `go-redis/redis/v8 → redis/go-redis/v9` and rename v8-only fields **`IdleTimeout → ConnMaxIdleTime`**, **`MaxConnAge → ConnMaxLifetime`** first. (v9 is already an *indirect* dep via paycloudhelper, so no new module.)
2. **redsync differs between the two:**
   - `settlementpg-module` **uses** redsync (`app/services/transaction.go:146` `RedisSync.NewMutex` + `mutex.Lock()`). **Keep it** — migrate the pool adapter `goredis/v8 → goredis/v9`; the redsync v4 lock API and lock key are unchanged.
   - `settlement-manager`'s redsync is **dead code** (no `NewMutex` anywhere). **Delete** `RedisSync` + `InitRedSync()` + the redsync/goredis imports.

Env-var parity for both: build options from the same `helpers.Getenv(...)` calls — **`REDIS_PASS`** (note: not `REDIS_PASSWORD`/`REDIS_PWD`), and `REDIS_DB` has **no default** (errors when unset — preserve).

---

## Group C addendum — SNAP-BI interface managers (v8→v9 only)

Added 2026-06-08 after a full sweep of all 9 `*interface-manager` repos in `go/src`. Five SNAP-BI interface managers still run **`github.com/go-redis/redis/v8 v8.11.5`** on **`paycloudhelper v1.9.1`** and use Redis (external-id dedup / token cache). They were **not** in the original six-service scope.

| Repo | go-redis | paycloudhelper | Redis pool | v9 action |
|---|---|---|---|---|
| paycloud-be-paydiainterface-manager | **v8** | v1.9.1 | pch shared (no own pool) | v8→v9 import bump |
| paycloud-be-gdvinterface-manager | **v8** | v1.9.1 | pch shared | v8→v9 import bump |
| paycloud-be-nobuinterface-manager | **v8** | v1.9.1 | pch shared | v8→v9 import bump |
| paycloud-be-bagsnapinterface-manager | **v8** | v1.9.1 | pch shared | v8→v9 import bump |
| paycloud-be-snapbiinterface-manager | **v8** | v1.9.1 | pch shared | v8→v9 import bump |

**Already on v9 (no action):** `qoinhubinterface-manager` (pch `v1.9.2-beta.1` + go-redis/v9).
**No Redis at all (no action):** `bagqrisinterface-manager`, `ftsnapinterface-manager`, `qrisinterface-manager`.

**Scope is lighter than Group A/B.** All five already delegate to the **pch shared pool** (`pchelper.GetRedisPoolClient()` via a thin `RedisHelper`; the only `redis.NewClient` calls are in `*_test.go`). So there is **no "delegate the pool" work** here — they need only the mechanical **v8→v9 import swap** (`github.com/go-redis/redis/v8` → `github.com/redis/go-redis/v9`; rename `IdleTimeout → ConnMaxIdleTime` if set) when they bump `paycloudhelper` to v2.x. None use redsync/distributed locks. **No key changes** — key naming is handled separately by the namespacing/TTL plan.

> **Coordination with the namespacing plan:** four of these (paydia, gdv, nobu, bagsnap) also carry the `snap_external_id_* → snapbi:external-id:*` rename on their `feat/redis` branch (Task 3.8 of `paycloud-docs/backend/redis/2026-05-23-redis-key-namespacing-and-ttl-fixes.md`). Sequence the v9 bump and the key rename on the **same branch** so `middlewares/externalbi.go` is touched once. `snapbiinterface-manager` has **no** key rename (its `snapbi_int:*` keyspace is already namespaced and separate) — it needs the v9 bump only.

---

> **⛔ The per-service task sections below are the ORIGINAL "full-delete" approach, covering only the four Group A (v9) services. They are retained for reference only and are superseded by the six per-service plan files (four Group A + two Group B). Do NOT execute them as written — they contain defects F1–F9, and they do not cover the Group B v8→v9 services at all. Use the per-service plans instead.**

---

## Execution order

| # | Service | Effort | Risk | Scope |
|---|---------|--------|------|-------|
| 1 | `paycloud-be-clientpg-module` | S | Low | Full delete |
| 2 | `paycloud-be-transaction-module` | M | Low | Full delete |
| 3 | `paycloud-be-clientpg-manager` | L | Low | Full delete |
| 4 | `paycloud-be-config-module` | M | Medium | Partial (keep Cache interface) |

Each task is fully independent — do them one at a time, `go build ./...` must pass before moving on.

---

## Task 1 — `paycloud-be-clientpg-module`

**Context:** 261-line `providers/redis.go`, flat package-level functions, 9 call-site files (13 lines). Uses `encoding/json` (not phjson). Has `REDIS_TLS` support and three pattern/master-cache helpers (`DeleteRedisByPattern`, `GetRedisMasterByFunc`, `StoreRedisMasterByFunc`) that need the raw `*redis.Client` — these must survive as local helpers.

**Files:**
- Modify: `/paycloud-be-clientpg-module/main.go`
- Create: `/paycloud-be-clientpg-module/helpers/redis_cache.go`
- Delete: `/paycloud-be-clientpg-module/providers/redis.go`
- Modify (call sites): see step 4

---

### Step 1.1 — Bump paycloudhelper dependency

```bash
cd /paycloud-be-clientpg-module
go get github.com/PayCloud-ID/paycloudhelper@v1.10.2
go mod tidy
```

Expected: `go.mod` now shows `paycloudhelper v1.10.2`.

---

### Step 1.2 — Replace Redis init in `main.go`

Open `main.go`. Find:
```go
rdc, err := providers.InitRedis()
if err != nil {
    fatal("init redis", err)
}
app.Redis = rdc
```

Replace with:
```go
if err := pch.InitRedisFromEnv(); err != nil {
    fatal("init redis", err)
}
```

If `REDIS_TLS` is used in any environment, use this instead:
```go
host := os.Getenv("REDIS_HOST")
port := os.Getenv("REDIS_PORT")
if port == "" { port = "6379" }
db, _ := strconv.Atoi(os.Getenv("REDIS_DB"))
opts := redis.Options{
    Addr:         host + ":" + port,
    Username:     os.Getenv("REDIS_USERNAME"),
    Password:     os.Getenv("REDIS_PWD"),
    DB:           db,
    MaxRetries:   3,
    PoolSize:     50,
    MinIdleConns: 10,
    DialTimeout:  3 * time.Second,
    ReadTimeout:  2 * time.Second,
}
if enabled, _ := strconv.ParseBool(os.Getenv("REDIS_TLS")); enabled {
    opts.TLSConfig = &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
}
if err := pch.InitializeRedisWithRetry(pch.RedisInitOptions{
    Options:    opts,
    MaxRetries: 3,
    RetryDelay: 500 * time.Millisecond,
    FailFast:   true,
}); err != nil {
    fatal("init redis", err)
}
```

Also find the Redis close on shutdown (if any) and remove it — paycloudhelper owns the lifecycle.

Remove the `app.Redis` assignment. Any place the service struct holds `app.Redis *redis.Client` and passes it to services — replace that field with calls to `pch.GetRedisPoolClient()` at the point of use (or remove the field entirely).

---

### Step 1.3 — Create `helpers/redis_cache.go`

These three helpers need raw `*redis.Client` access (SCAN, Keys, Unlink). Move them out of providers into helpers, backed by pch's pool:

```go
// helpers/redis_cache.go
package helpers

import (
    "context"
    "strings"
    "time"

    pch "github.com/PayCloud-ID/paycloudhelper"
    "github.com/PayCloud-ID/paycloudhelper/phjson"
)

// GetRedisMasterDuration returns the cache TTL for master-key helpers.
func GetRedisMasterDuration() time.Duration {
    // Keep existing env-read logic from providers.GetRedisMasterDuration()
    durMinute, err := strconv.Atoi(os.Getenv("REDIS_MASTER_DURATION_MINUTE"))
    if err != nil {
        return time.Minute
    }
    return time.Duration(durMinute) * time.Minute
}

// DeleteRedisByPattern deletes an exact key and all keys matching pattern+"_*".
func DeleteRedisByPattern(ctx context.Context, pattern string) error {
    pattern = IsEnvSb() + pattern
    if err := pch.DeleteRedisWithContext(ctx, pattern); err != nil {
        return err
    }
    keys, err := GetRedisKeyList(ctx, pattern+"_*")
    if err != nil {
        return err
    }
    for _, k := range keys {
        if err := pch.DeleteRedisWithContext(ctx, k); err != nil {
            return err
        }
    }
    return nil
}

// GetRedisKeyList returns all keys matching pattern using KEYS (not SCAN).
func GetRedisKeyList(ctx context.Context, pattern string) ([]string, error) {
    c, err := pch.GetRedisPoolClient()
    if err != nil {
        return nil, err
    }
    return c.Keys(ctx, pattern).Result()
}

// GetRedisMasterByFunc retrieves the value for a composite key.
func GetRedisMasterByFunc(ctx context.Context, key1 string, keyArr []string) (string, error) {
    checkKey := false
    for i := range keyArr {
        if keyArr[i] == "" {
            keyArr[i] = "%"
            checkKey = true
        }
    }
    var key string
    if !checkKey {
        key = getKeyRedis(key1, keyArr)
    } else {
        pattern := getKeyRedis(key1, keyArr)
        resKl, err := GetRedisKeyList(ctx, pattern)
        if err != nil {
            return "", err
        }
        if len(resKl) == 0 {
            return "", nil
        }
        key = resKl[0]
    }
    return pch.GetRedisWithContext(ctx, key)
}

// StoreRedisMasterByFunc writes data under the composite key, deleting the old value first.
func StoreRedisMasterByFunc(ctx context.Context, key1 string, keyArr []string, data interface{}) error {
    key := getKeyRedis(key1, keyArr)
    _ = pch.DeleteRedisWithContext(ctx, key)
    jsonData, err := phjson.Marshal(data)
    if err != nil {
        return err
    }
    return pch.StoreRedisWithContext(ctx, key, string(jsonData), GetRedisMasterDuration())
}

func getKeyRedis(key1 string, keyArr []string) string {
    if len(keyArr) == 0 {
        return key1
    }
    return IsEnvSb() + key1 + "_" + strings.Join(keyArr, "_")
}
```

> **Note:** `IsEnvSb()` is already in `helpers/` — reuse it. Add `"os"` and `"strconv"` imports if not already present. Remove `import "encoding/json"` — we now use `phjson`.

---

### Step 1.4 — Replace call sites (9 files, 13 lines)

Run these replacements across the service:

| File(s) | Old call | New call |
|---------|----------|----------|
| `controllers/userrole.go` | `providers.StoreRedis(ctx, key, data, dur)` | `pch.StoreRedisWithContext(ctx, key, data, dur)` |
| `controllers/rolepermission.go` | `providers.StoreRedis(ctx, key, data, dur)` | `pch.StoreRedisWithContext(ctx, key, data, dur)` |
| `controllers/rolepermission.go` | `providers.DeleteRedis(ctx, key)` | `pch.DeleteRedisWithContext(ctx, key)` |
| `controllers/register-user.go` | `providers.StoreRedis(ctx, key, data, dur)` | `pch.StoreRedisWithContext(ctx, key, data, dur)` |
| `controllers/healthcheck.go` | `providers.RedisClient` | `pch.GetRedisPoolClient()` + unwrap error |
| `controllers/healthcheck.go` | `providers.InitRedis()` (re-init path) | `pch.InitRedisFromEnv()` |
| `services/user_role_ops_service.go` | `providers.StoreRedis(ctx, key, data, dur)` | `pch.StoreRedisWithContext(ctx, key, data, dur)` |
| `workers/send-receive.go` | `providers.*` | `pch.*` or `helpers.*` as appropriate |
| `workers/rabbitmq.go` | `providers.*` | `pch.*` as appropriate |

Any call to `providers.GetRedisMasterByFunc`, `providers.StoreRedisMasterByFunc`, `providers.DeleteRedisByPattern`, `providers.GetRedisKeyList` → replace with `helpers.GetRedisMasterByFunc`, `helpers.StoreRedisMasterByFunc`, `helpers.DeleteRedisByPattern`, `helpers.GetRedisKeyList`.

Add import to each modified file:
```go
pch "github.com/PayCloud-ID/paycloudhelper"
```

Remove `"paycloud-be-clientpg-module/providers"` import from any file where all `providers.*` references are gone.

---

### Step 1.5 — Delete `providers/redis.go`

```bash
rm /paycloud-be-clientpg-module/providers/redis.go
```

---

### Step 1.6 — Build and verify

```bash
cd /paycloud-be-clientpg-module
go build ./...
go vet ./...
go test ./...
```

Expected: all pass, zero `providers` import errors.

---

### Step 1.7 — Commit

```bash
cd /paycloud-be-clientpg-module
git add -p
git commit -m "refactor(redis): remove providers/redis.go, use paycloudhelper v1.10.2

Replace per-service Redis provider with pch.InitRedisFromEnv and pch
CRUD functions. Pattern helpers moved to helpers/redis_cache.go backed
by pch.GetRedisPoolClient() — single connection pool, no duplication.

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

---

## Task 2 — `paycloud-be-transaction-module`

**Context:** 297-line `providers/redis.go`, 17 call-site files (24 lines). The provider already wraps `pchelper.LogI/LogE` internally. Has a 6-attempt exponential-backoff loop in `main.go` — this collapses into a single `pch.InitializeRedisWithRetry` call. Has distributed lock functions (`AcquireLockContext`, `ReleaseLockContext`) that delegate to `helpers.StoreMutex/GetMutex/RemoveMutex` — pch owns its own mutex map, so those helpers become unused.

**Files:**
- Modify: `/paycloud-be-transaction-module/server.go` (main entry is in server.go loop, not main.go)
- Check: `/paycloud-be-transaction-module/helpers/` for `StoreMutex/GetMutex/RemoveMutex`
- Delete: `/paycloud-be-transaction-module/providers/redis.go`
- Modify (call sites): 17 files listed in step 4

---

### Step 2.1 — Bump paycloudhelper dependency

```bash
cd /paycloud-be-transaction-module
go get github.com/PayCloud-ID/paycloudhelper@v1.10.2
go mod tidy
```

---

### Step 2.2 — Replace Redis init in `server.go` (retry loop)

Find the block in `server.go` (or `main.go`):
```go
const (
    redisInitMaxAttempts  = 6
    redisInitInitialDelay = 500 * time.Millisecond
    redisInitMaxDelay     = 30 * time.Second
)
// ...
delay := redisInitInitialDelay
var errRedis error
for attempt := 1; attempt <= redisInitMaxAttempts; attempt++ {
    if _, errRedis = providers.InitRedis(); errRedis == nil {
        break
    }
    // ... backoff logic
}
if errRedis != nil {
    pchelper.LogF(...)
    return
}
```

Replace the entire block with:
```go
if err := pch.InitializeRedisWithRetry(pch.RedisInitOptions{
    Options: redis.Options{
        Addr:            config.RedisHost() + ":" + config.RedisPort(),
        Password:        config.RedisPassword(),
        DB:              config.RedisDB(),
        Username:        "default",
        MaxRetries:      3,
        MinRetryBackoff: 10 * time.Millisecond,
        MaxRetryBackoff: 500 * time.Millisecond,
        ConnMaxIdleTime: 10 * time.Minute,
        PoolSize:        10 * runtime.GOMAXPROCS(0),
    },
    MaxRetries: redisInitMaxAttempts,
    RetryDelay: redisInitInitialDelay,
    FailFast:   true,
}); err != nil {
    pch.LogF("[main] Redis init failed err=%v alert=true", err)
    return
}
```

Add import `"github.com/redis/go-redis/v9"` and `"runtime"` to the file if not present. Remove `redisInitMaxDelay` constant (no longer needed). Remove `redisInitMaxAttempts` and `redisInitInitialDelay` constants only if they are now unused (if used in the new call above, keep them).

---

### Step 2.3 — Check helpers for `StoreMutex/GetMutex/RemoveMutex`

```bash
grep -rn "StoreMutex\|GetMutex\|RemoveMutex" \
  /paycloud-be-transaction-module/helpers/
```

If these functions exist only to support `providers/redis.go`'s lock functions — and nothing else in the service calls them directly — delete the file or functions after `providers/redis.go` is removed in step 2.5. Verify with `go build ./...` after deletion.

---

### Step 2.4 — Replace call sites (17 files, 24 lines)

**Function mapping:**

| Old (providers package) | New (pch package) |
|-------------------------|-------------------|
| `providers.StoreRedis(id, data, dur)` | `pch.StoreRedis(id, data, dur)` |
| `providers.StoreRedisContext(ctx, id, data, dur)` | `pch.StoreRedisWithContext(ctx, id, data, dur)` |
| `providers.GetRedis(id)` | `pch.GetRedis(id)` |
| `providers.GetRedisContext(ctx, id)` | `pch.GetRedisWithContext(ctx, id)` |
| `providers.StoreRedisWithLock(id, data, dur)` | `pch.StoreRedisWithLock(id, data, dur)` |
| `providers.StoreRedisWithLockContext(ctx, id, data, dur)` | `pch.StoreRedisWithContext(ctx, id, data, dur)` wrapped with `pch.AcquireLock`/`pch.ReleaseLock` |
| `providers.AcquireLockContext(ctx, key, ttl)` | `pch.AcquireLock(key, ttl)` |
| `providers.ReleaseLockContext(ctx, key)` | `pch.ReleaseLock(key)` |
| `providers.InitRedSync()` | `pch.InitRedSyncOnce()` |
| `providers.RedisDefaultDuration` | Keep as a constant in the service: `const redisDefaultDuration = 180 * time.Second` |
| `providers.redisLockKey` / `providers.RedisLockMainKey` | Keep as constants in the relevant service files |

> **Lock context note:** `pch.AcquireLock` and `pch.ReleaseLock` do not take a `context.Context` parameter — they use an internal timeout. Remove `ctx` from those call sites.

**Files to touch** (grep to confirm line numbers before editing):
```
server.go
app/runtime.go
http_server/metrics.go
controllers/create_order.go
controllers/set_expired_temporal.go
scheduled/set_expired_daily.go
routes/rabbit_consumer.go
services/transaction_expiry.go
services/healthcheck.go
services/notify_transaction_finish.go
services/update_transaction_payment.go
services/transaction.go
services/check_status.go
services/update_transaction.go
services/set_expired.go
services/set_pmt_chan.go
services/create_order_h2h.go
```

In each file: add `pch "github.com/PayCloud-ID/paycloudhelper"` import, remove `"paycloud-be-transaction-module/providers"` import when all references are replaced.

---

### Step 2.5 — Delete `providers/redis.go`

```bash
rm /paycloud-be-transaction-module/providers/redis.go
```

---

### Step 2.6 — Build and verify

```bash
cd /paycloud-be-transaction-module
go build ./...
go vet ./...
go test ./...
go test -race ./...
```

---

### Step 2.7 — Commit

```bash
cd /paycloud-be-transaction-module
git add -p
git commit -m "refactor(redis): remove providers/redis.go, use paycloudhelper v1.10.2

Replace 297-line per-service Redis provider. Exponential-backoff retry
loop collapses into pch.InitializeRedisWithRetry. Lock call sites now use
pch.AcquireLock / pch.ReleaseLock directly. Single connection pool.

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

---

## Task 3 — `paycloud-be-clientpg-manager`

**Context:** 179-line `providers/redis.go`, 27 call-site files (75 lines). Largest call-site count but most are mechanical renames. Three function groups: context-less wrappers (`StoreRedis`, `GetRedis`, `DeleteRedis`), context-aware variants (`StoreRedisCtx`, `GetRedisCtx`, `DeleteRedisCtx`), and raw pool access. One special case: `security/check-permission.go` holds `providers.GetRedis` as a function value in a variable. Uses `encoding/json` — switch to `phjson`.

**Files:**
- Modify: `/paycloud-be-clientpg-manager/main.go`
- Modify: `/paycloud-be-clientpg-manager/security/check-permission.go`
- Modify (call sites): 25 more files (see step 4)
- Delete: `/paycloud-be-clientpg-manager/providers/redis.go`

---

### Step 3.1 — Bump paycloudhelper dependency

```bash
cd /paycloud-be-clientpg-manager
go get github.com/PayCloud-ID/paycloudhelper@v1.10.2
go mod tidy
```

---

### Step 3.2 — Replace init in `main.go`

Find:
```go
// Redis connection
if _, err := providers.InitRedis(); err != nil {
    pchelper.LogE("[main] ERR InitRedis: %v", err)
}
defer providers.RedisPoolClient.Close()
```

Replace with:
```go
if err := pch.InitRedisFromEnv(); err != nil {
    pch.LogF("[main] ERR InitRedis: %v", err)
    return
}
```

> The `defer Close()` is removed — paycloudhelper manages the pool lifecycle. If you need an explicit close on shutdown, use `pch.GetRedisPoolClient()` to get the client, then `defer client.Close()`.

---

### Step 3.3 — Fix `security/check-permission.go` (function value)

Find:
```go
// providers.GetRedis in production and can be replaced in tests via
// SetRedisGetterForTest. The signature matches providers.GetRedis exactly so
var redisGetter = providers.GetRedis
```

Replace with:
```go
// pch.GetRedis in production and can be replaced in tests via
// SetRedisGetterForTest. The signature matches pch.GetRedis exactly so
var redisGetter = pch.GetRedis
```

Add `pch "github.com/PayCloud-ID/paycloudhelper"` import. Remove `providers` import.

---

### Step 3.4 — Replace call sites (27 files, 75 lines)

**Function mapping:**

| Old | New | Note |
|-----|-----|------|
| `providers.StoreRedis(id, data, dur)` | `pch.StoreRedis(id, data, dur)` | Same signature |
| `providers.GetRedis(id)` | `pch.GetRedis(id)` | Same signature |
| `providers.DeleteRedis(id)` | `pch.DeleteRedis(id)` | Same signature |
| `providers.StoreRedisCtx(ctx, id, data, dur)` | `pch.StoreRedisWithContext(ctx, id, data, dur)` | Rename only |
| `providers.GetRedisCtx(ctx, id)` | `pch.GetRedisWithContext(ctx, id)` | Rename only |
| `providers.DeleteRedisCtx(ctx, id)` | `pch.DeleteRedisWithContext(ctx, id)` | Rename only |
| `providers.GetRedisPoolClient()` | `pch.GetRedisPoolClient()` (unwrap `(client, err)`) | Same name, diff return |
| `providers.RedisPoolClient` | `pch.GetRedisPoolClient()` (unwrap err) | 2 sites in main.go area |
| `providers.CheckRedisConn(ctx)` | see below | healthcheck only |

**`CheckRedisConn` replacement** (in `controllers/healthcheck.go`):
```go
// Old
if err := providers.CheckRedisConn(ctx); err != nil { ... }

// New
c, err := pch.GetRedisPoolClient()
if err != nil { /* handle */ }
if _, err := c.Ping(ctx).Result(); err != nil { /* handle */ }
```

**`encoding/json` → `phjson`** in files that marshal for Redis writes:
```go
// Old
import "encoding/json"
payload, err := json.Marshal(data)

// New
import "github.com/PayCloud-ID/paycloudhelper/phjson"
payload, err := phjson.Marshal(data)
```

Only change `json.Marshal` calls that produce Redis payloads — do not change JSON for HTTP responses.

**Files to touch:**
```
main.go
middleware/csrf.go
middleware/auth.go
middleware/rate-limit.go
grpc/server/controllers/auth.go
security/check-permission.go           ← already done in step 3.3
security/captcha/captcha.go
controllers/web.go
controllers/profile-photo.go
controllers/logout.go
controllers/authentic.go
controllers/healthcheck.go
controllers/auth_otp_state.go
controllers/auth_otp_handlers.go
controllers/profile.go
controllers/auth-async.go
controllers/check-session-login.go
controllers/forgot.go
helpers/merchant_register_data.go
usecases/auth_login.go
usecases/auth_registration.go
usecases/refresh_token.go
usecases/email.go
usecases/auth_registration_validation.go
usecases/token_utils.go
usecases/signature.go
services/rabbitmq.go
```

In each file: add `pch "github.com/PayCloud-ID/paycloudhelper"` import, remove `providers` import when clear.

---

### Step 3.5 — Delete `providers/redis.go`

```bash
rm /paycloud-be-clientpg-manager/providers/redis.go
```

---

### Step 3.6 — Build and verify

```bash
cd /paycloud-be-clientpg-manager
go build ./...
go vet ./...
go test ./...
go test -race ./...
```

---

### Step 3.7 — Commit

```bash
cd /paycloud-be-clientpg-manager
git add -p
git commit -m "refactor(redis): remove providers/redis.go, use paycloudhelper v1.10.2

Replace 179-line per-service Redis provider across 27 files. Context-less
wrappers map 1:1, context variants renamed (StoreRedisCtx→StoreRedisWithContext
etc). encoding/json→phjson for Redis marshaling. Single connection pool.

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

---

## Task 4 — `paycloud-be-config-module` (partial migration)

**Context:** 326-line `providers/redis.go` that defines a `Cache` interface and `RedisImpl` struct. The interface (`Store`, `Get`, `Delete`, `DeleteByPattern`, `GetMasterByFunc`, `StoreMasterByFunc`, `MasterDuration`, `CheckConn`, `Client`, `Sync`) is injected via DI across 14 files — it cannot be deleted. **Only `InitRedis()` changes**: replace the `redis.NewClient()` + Ping block with `pch.InitializeRedisWithRetry` so the underlying `*redis.Client` comes from paycloudhelper's pool.

**Files:**
- Modify: `/paycloud-be-config-module/providers/redis.go` (InitRedis function only)
- No call-site changes required — the `Cache` interface stays intact

---

### Step 4.1 — Bump paycloudhelper dependency

```bash
cd /paycloud-be-config-module
go get github.com/PayCloud-ID/paycloudhelper@v1.10.2
go mod tidy
```

---

### Step 4.2 — Replace `InitRedis()` body in `providers/redis.go`

Find the entire `InitRedis()` function (lines 32–86 in current file):
```go
func InitRedis() (*RedisImpl, error) {
    cfg := configs.Get()
    pchelper.LogI("[InitRedis] connecting host=%s port=%s db=%d", ...)

    poolSize := cfg.Redis.PoolSize
    if poolSize <= 0 { poolSize = defaultRedisPoolMultiplier * runtime.GOMAXPROCS(0) }
    minIdle := cfg.Redis.MinIdleConns
    if minIdle <= 0 { minIdle = 5 }

    redisOpts := &redis.Options{ ... }
    // ... password/username conditionals ...
    client := redis.NewClient(redisOpts)

    ctx, cancel := context.WithTimeout(context.Background(), cfg.Redis.Timeout)
    defer cancel()

    res, err := client.Ping(ctx).Result()
    if err != nil { ... return nil, err }

    client.Do(ctx, "CLIENT", "SETNAME", cfg.App.Name)
    pchelper.LogI(...)

    impl := &RedisImpl{client: client}
    if err = impl.initRedSync(); err != nil { ... }
    return impl, nil
}
```

Replace with:
```go
func InitRedis() (*RedisImpl, error) {
    cfg := configs.Get()
    pchelper.LogI("[InitRedis] connecting host=%s port=%s db=%d",
        cfg.Redis.Host, cfg.Redis.Port, cfg.Redis.DB)

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

**Remove now-unused imports** from the top of `providers/redis.go`:
- `"golang.org/x/sync/errgroup"` — only used by the old `CheckConn` (keep if `CheckConn` still uses it)
- Check each import: only remove what is truly unused after this change

**Keep everything else unchanged:** `Cache` interface, all `RedisImpl` methods, `initRedSync`, helper functions (`getKeyRedisStore`, `checkKeyArrGetStore`, etc.).

---

### Step 4.3 — Update `server.go` close block (optional cleanup)

Find:
```go
if s.cache != nil && s.cache.Client() != nil {
    if err := s.cache.Client().Close(); err != nil {
        pchelper.LogE("[Server.waitForShutdown] failed to close Redis connection: %v", err)
    }
}
```

This still works correctly — `s.cache.Client()` returns the same `*redis.Client` pch holds. Leave it as-is, or remove it (paycloudhelper's pool is process-scoped and does not need explicit close). Either is safe.

---

### Step 4.4 — Build and verify

```bash
cd /paycloud-be-config-module
go build ./...
go vet ./...
go test ./...
go test -race ./...
```

---

### Step 4.5 — Commit

```bash
cd /paycloud-be-config-module
git add providers/redis.go go.mod go.sum
git commit -m "refactor(redis): delegate connection init to paycloudhelper v1.10.2

Replace manual redis.NewClient+Ping in providers.InitRedis() with
pch.InitializeRedisWithRetry. Cache interface and RedisImpl methods
unchanged — only the pool ownership moves to paycloudhelper.
Eliminates duplicate connection pool per process.

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

---

## Validation gate (all 4 services)

After completing all tasks:

```bash
for svc in \
  paycloud-be-clientpg-module \
  paycloud-be-transaction-module \
  paycloud-be-clientpg-manager \
  paycloud-be-config-module; do
  echo "=== $svc ==="
  cd /$svc
  go build ./... && go test -race ./... && echo "OK"
  grep -r "go-redis/redis/v8" . --include="*.go" && echo "FAIL: v8 import found" || true
  grep -rn "providers\..*Redis" . --include="*.go" \
    | grep -v "^./providers/" | grep -v "_test.go" \
    && echo "FAIL: stale providers.Redis call" || echo "providers.Redis: clean"
done
```

Expected output: `OK` and `providers.Redis: clean` for every service.

---

## Rollback

If a service has issues after deployment, pin the previous paycloudhelper version:
```bash
go get github.com/PayCloud-ID/paycloudhelper@v1.10.2
go mod tidy
go build ./...
```

The old `providers/redis.go` code is in git history — restore it with `git checkout HEAD~1 -- providers/redis.go` if needed.
