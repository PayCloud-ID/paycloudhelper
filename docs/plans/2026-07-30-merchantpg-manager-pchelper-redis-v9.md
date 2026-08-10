# Two independent tracks: go-redis v9 for merchantpg-manager (Track A), `pcauth` for all three password consumers (Track B)

> **For Claude:** the `pchelper-redis-v9-migration` skill covers the mechanics (import swap, option
> renames, per-profile checklists). It was stale; **Task 5 fixed it** — see §0.2. Follow this plan for
> sequencing and the skill for the edits.

## 🔧 Correction pass — 2026-08-06

Re-verified end to end against the live repos. The plan's *direction* held; six factual defects and
one structural flaw did not. **The structural fix is the one that matters: Tracks A and B are now
independent.** Full change log in [§9](#9-correction-pass--2026-08-06).

| # | Was | Now |
|---|---|---|
| S | Track A blocked on Track B (`v1.12.0` = `v1.11.0` + `pcauth`, one bump) | **Decoupled.** Track A ships on **`v1.11.1` today**; Track B is a second, trivial bump |
| 1 | "today's `v1.11.0`" is the latest tag | Latest is **`v1.11.1`** (2026-08-03) |
| 2 | 4c's `develop → master` promotion is a **fast-forward** | **False today** — `master` is 1 commit ahead. Trivial merge, but the stated gate command fails |
| 3 | `pcauth` seeded "verbatim"; call sites "behaviourally identical" | **Signature change.** clientpg-manager returns `(string, string)` — no error — at 11 of the 14 sites |
| 4 | 4b touches 9 call sites | **14.** Four were missing, one of them a `VerifyAndMaybeRehash` login-adjacent site |
| 5 | Manager verifies JWTs via `middlewares.VerifyToken`; gate staging on `RevokeToken` | **Misattributed.** The manager has its own middleware; zero pchelper JWT/Redis/middleware symbols. Gate removed |
| 6 | Task 7 items 5–6 edit existing text | **Those texts do not exist** on `master`. Re-targeted |
| 7 | Third version boundary (TTL clamp at `v1.10.2`) unmentioned | Added — with the finding that it is **inert** for this service (§0.4) |

**Goal — two outcomes that were wrongly coupled:**

| Track | Outcome | Blocked by |
|---|---|---|
| **A — Redis** | Get `paycloud-be-merchantpg-manager` off `github.com/go-redis/redis/v8`, which means crossing `paycloudhelper v1.10.0` | Nothing but the bulk-import merge. **Ready now** |
| **B — Passwords** | Ship **`pcauth`** — one canonical bcrypt helper — and put **all three** password consumers on it, closing F-13/F-14 | `pcauth` does not exist yet; must be written, reviewed and released |

Track B exists because `clientpg-manager` and `clientpg-module` hold *mutually incompatible*
implementations today, and the merchant bulk CSV import adds a third (temporary) copy.

**Why they are now decoupled.** The earlier revision cut **`v1.12.0`** (= `v1.11.0` + the additive
`pcauth`) so the manager could bump **once**. That saves one trivial dependency bump and costs the
Redis migration an open-ended wait on a package nobody has written — `pcauth` is absent from
paycloudhelper `main` (`03f05b5`) and from every tag. A `go get` + a three-line import swap is not
worth deferring behind a new library package that needs its own review and release. **Track A targets
`v1.11.1` and ships on its own; Track B takes a second bump to `v1.12.0` whenever it is ready.** The
old fallback in §3 is now the default.

**Repos**

| Repo | Track | Role |
|---|---|---|
| [`paycloudhelper`](https://github.com/PayCloud-ID/paycloudhelper) | B | gains `pcauth`, released as `v1.12.0` |
| [`paycloud-be-merchantpg-manager`](https://github.com/PayCloud-ID/paycloud-be-merchantpg-manager) | A, then B | **A:** `v1.9.1 → v1.11.1`, go-redis v8 → v9. **B:** `→ v1.12.0`, adopts `pcauth`, deletes its local copy |
| [`paycloud-be-clientpg-manager`](https://github.com/PayCloud-ID/paycloud-be-clientpg-manager) | B | source of the canonical algorithm; adopts `pcauth`, deletes its local copy |
| [`paycloud-be-clientpg-module`](https://github.com/PayCloud-ID/paycloud-be-clientpg-module) | B | deletes its **divergent** helper (F-13) — branch-gated |

`merchantpg-module` is **not** touched: it is already at `v1.10.3` (past the break) and never hashes —
it forwards the `(salt, hash)` pair it is handed (bulk-import plan D9).

**Verified 2026-07-30/31, re-verified 2026-08-06** against manager `master` `50cf638` and
`origin/feat/PA-222` `c7ac573`, merchantpg-module `master`, clientpg-manager `staging`,
clientpg-module `origin/{master,develop}`, `PayCloud-ID/paycloudhelper` `main` `03f05b5` + full tag
list + CHANGELOG, `pc-be-services/env-data`, and the reference repos in §0.3.

**Owner decision recorded 2026-07-31:** **no `v2.0.0` is planned** — paycloudhelper stays on the `v1.x`
line. That settles §0.2 and makes Task 5b's branch correct to merge.

---

## 0. Findings that shape the plan

### 0.1 Where the break actually is

`paycloudhelper` **`v1.10.0`** (2026-04-24) CHANGELOG, verbatim:

> `### Changed (Breaking)` — **Redis client**: migrated from `github.com/go-redis/redis/v8` to
> `github.com/redis/go-redis/v9` (`v9.18.0`). Exported types in the root package (`redis.Options`,
> `*redis.Client`, etc.) now use the v9 module path; `goredis/v9` pool adapter with the v9 client.

The manager sits at `v1.9.1` — **below** the break — and holds
`github.com/go-redis/redis/v8 v8.11.5` as a **direct** dependency (`go.mod:9`), with
`go-redsync/redsync/v4 v4.15.0` indirect (`:41`).

**Live pins, re-verified 2026-08-06:**

| Repo / branch | pchelper pin | Redis | Position |
|---|---|---|---|
| `merchantpg-manager` `master` `50cf638` | `v1.9.1` | `go-redis/v8` **direct**, 2 sites | ⬅ Track A |
| `merchantpg-manager` `origin/feat/PA-222` `c7ac573` | `v1.9.1` | `go-redis/v8` **direct**, **3** sites | ⬅ Track A lands here |
| `merchantpg-module` `master` | `v1.10.3` | — | already past the break, untouched |
| `clientpg-manager` `staging` | `v1.10.4` | — | past |
| `clientpg-module` `origin/master` | `v1.10.3` | — | past |
| `clientpg-module` `origin/develop` | `v1.11.0` | — | past |

Manager toolchain: `go 1.25.0` / `toolchain go1.25.10` — **no toolchain blocker** for `v1.10.2+`
(which requires go 1.25; see §0.4).

**⚠️ Correction — the latest tag is `v1.11.1`, not `v1.11.0`.** `v1.11.1` was cut **2026-08-03**
(after this plan was written) and is a docs + audit-trail patch: it demotes the per-event echo and
rate-limits the publish-success line in `audittrail.go`, and carries the Task 5b doc corrections. The
manager calls `LogAudittrailData` / `LogAudittrailProcess`, so that is a free win. **Track A targets
`v1.11.1`.**

**Nuance the one-line "the break is `v1.10.0`" hides.** By *GA tag* that is correct. By *first
appearance*, go-redis v9 lands at **`v1.9.2-beta.1`** — verified across every tag:

| Tag | Redis module |
|---|---|
| `v1.9.0`, `v1.9.1` | `github.com/go-redis/redis/v8 v8.11.5` |
| **`v1.9.2-beta.1`**, `v1.9.2-beta.2` | `github.com/redis/go-redis/v9 v9.18.0` |
| `v1.10.0` … `v1.11.1` | `github.com/redis/go-redis/v9 v9.18.0` |

This matters org-wide, not here: the `v1.9.2-beta.*` tags are the **only** v9-bearing releases that
build on **go 1.24** (both `beta.1` and `beta.2` declare `go 1.24.0`), which is why
`qoinhubinterface-manager` legitimately sits there. Anyone applying the
skill's flat "the break is `v1.10.0`" to that service will reach the wrong conclusion. See §0.4.

### 0.2 The migration skill was stale — fixed (Task 5)

It claimed the break landed in **`v2.0.0`** on `bitbucket.org/paycloudid/paycloudhelper`. Both wrong:
it is `v1.10.0` on `github.com/PayCloud-ID/paycloudhelper`, `v2.0.0` returns **404**, and the owner has
now confirmed no `v2.x` is planned. Its *mechanics* were always fine — the import swap,
`Options.IdleTimeout` → `Options.ConnMaxIdleTime`, the redsync `goredis/v9` adapter, the Echo-API
checklist, the validation gates, the error catalog. Corrected in Task 5; the same error upstream is
Task 5b.

> **Why the version error mattered more than it looks:** the skill's frontmatter `description` — the
> text that decides whether the skill triggers at all — was scoped to "v1.x to v2.x". A bump from
> `v1.9.1` to `v1.11.0` does not read as "v1.x to v2.x", so the skill silently failed to fire for the
> exact case it exists for. This migration was nearly planned without it.

### 0.3 Two PayCloud services have already done this — copy them

| Repo | paycloudhelper | Redis | What it proves |
|---|---|---|---|
| `paycloud-be-ftsnapinterface-manager` | **`v1.9.1`** | `redis/go-redis/v9 v9.17.2` **direct**, v8 demoted to `// indirect` | A service can run v9 **below the break** — the swap and the bump are separable |
| `paycloud-be-fundtransfer-manager` | `v1.10.4` | `redis/go-redis/v9 v9.18.0` | The completed end state — **this is the shape we are aiming at** |

An earlier revision of this plan split the work in two (swap on `v1.9.1`, bump later) to isolate
failures. **Task 1's inventory retired that idea** — see below. `fundtransfer-manager`'s single-step
shape is the target.

### 0.4 This migration is already owned by another doc — and it adds a third version boundary

**Added 2026-08-06.** [`backend/redis/2026-05-23-redis-key-namespacing-and-ttl-fixes.md`](../redis/2026-05-23-redis-key-namespacing-and-ttl-fixes.md)
§TODO-2 names **`merchantpg-manager` explicitly** as one of 11 services still missing the Phase-4
Redis TTL soft-clamp, and frames the remedy as *"this is the go-redis v9 migration, not a version
bump."* **That is this plan's Track A.** The two documents were written independently and neither
cited the other; they are now cross-linked. Track A closing should update that doc's TODO-2 table
(added to Task 7).

**The third boundary.** Beyond *go-redis v8→v9* and *the `v1.10.0` GA line*, there is the **TTL
soft-clamp**, verified 2026-08-06 as first appearing in **`v1.10.2`** (absent in `v1.10.0` /
`v1.10.1`):

| pchelper | Go | Redis | Clamp |
|---|---|---|---|
| `v1.9.1` | 1.25 | v8 | ❌ |
| `v1.9.2-beta.1`, `v1.9.2-beta.2` | **1.24** *(the only go-1.24 tags)* | v9 | ❌ |
| `v1.10.0` – `v1.10.1` | 1.25 | v9 | ❌ |
| **`v1.10.2` – `v1.11.1`** | 1.25 | v9 | ✅ `MaxTTL` / `clampStoreTTL` / `StoreRedisNoExpiry` |

Track A's `v1.11.1` target clears this **by construction** — but state it deliberately rather than by
accident, because the clamp is a behaviour change to `StoreRedis` semantics.

> ### ⚠️ New finding — the clamp will be **inert** in this service
>
> The redis doc's table implies that pinning `≥ v1.10.2` buys clamp protection. **For
> `merchantpg-manager` it buys none.** Task 1 established the manager builds its **own** Redis client
> (`helpers/redis.go:19`, `providers/redis.go:24`) — and the 2026-08-06 pass confirms it calls **zero**
> pchelper Redis wrappers. Its complete pchelper symbol surface is:
>
> `ConfigureLogForwarding`, `Detail`, `InitSentry`, `LogAudittrailData`, `LogAudittrailProcess`,
> `LogD/E/I/W`, `LogForwardConfigFromEnv`, `LogSetLevel`, `LoggerErrorHub`, `Request`,
> `RequestAndResponse`, `SetUpRabbitMq`.
>
> No `StoreRedis`, no `GetRedis`, no `InitializeRedis`, no `AcquireLock`. The clamp guards writes that
> pass through those wrappers; this service's writes do not. **So Track A satisfies TODO-2's version
> requirement without satisfying its intent.** Genuinely closing TODO-2 for this service needs a
> separate decision — route through the pchelper wrappers, or clamp locally in `helpers/redis.go`.
> That is **out of scope here** and is raised as **F-17** in §7b.

---

## 1. Task 1 — Inventory the manager's Redis surface ✅ DONE 2026-07-31

Run and recorded, so Tasks 2–3 are no longer speculative:

```bash
git grep -n "go-redis/redis/v8" -- '*.go'
git grep -n "redis\.Options\|IdleTimeout\|NewClient\|redsync\|goredis" -- '*.go'
git grep -n "pchelper\.[A-Za-z]*(.*redis\." -- '*.go'     # the boundary-crossing gate
```

| Question | Answer |
|---|---|
| Where does the manager use Redis? | **Two files only** — `helpers/redis.go:19` and `providers/redis.go:24` |
| How? | Each builds its **own** client: `redis.NewClient(&redis.Options{…})`. It does **not** use `pchelper.GetRedisPoolClient()` |
| `IdleTimeout` set anywhere? | **No** — so the v9 `ConnMaxIdleTime` rename is a no-op here |
| redsync / distributed locks? | **None** in the manager's own code (redsync is indirect, via pchelper) |
| **Does any value cross the pchelper ↔ redis boundary?** | **No.** The only grep hit is a *commented-out* log line (`providers/redis.go:51`) |

**Two consequences, both simplifying:**

1. **The v8 → v9 change is an import-path edit in two files.** No option renames, no adapter work.
2. **The two-step split buys nothing.** It existed to dodge a mixed-version type mismatch that this
   codebase cannot hit — nothing is handed to pchelper as a `redis.*` type, so bump and swap are free
   to land together. **Tasks 2 and 3 of the earlier revision are now a single Task 3.**

> **Sequencing note (unchanged).** The bulk-import feature *adds* a Redis consumer (its Task 11
> idempotency store) and *removes* another (D2's row staging). Run this migration **after** that
> feature merges, so the surface is migrated once rather than straddled — and re-run the grep above
> afterwards, since Task 11 adds new Redis call sites.
>
> ✅ **Confirmed 2026-07-31 — the inventory above is already stale on `feat/PA-222`.** That branch adds a
> **third** direct `go-redis/v8` import site: `services/merchant-settings/merchant-bulk.go` imports it
> for `redis.Nil` around the idempotency store. So Task 3's import swap touches **three** files, not
> two. The boundary-crossing grep is still clean there (`helpers.GetRedis` / `helpers.StoreRedis`
> wrappers, no `redis.*` value handed to pchelper), so the single-step conclusion holds — but re-run
> both greps at the time of the work rather than trusting this list.

---

## 2. Task 2 — Add `pcauth` to `paycloudhelper` and cut `v1.12.0` — **Track B**

**Status 2026-08-06: not started.** `pcauth` is absent from paycloudhelper `main` (`03f05b5`) and from
every published tag. This is the prerequisite for **Track B only** — it is a prerequisite of the
bulk-import plan's Task 14b Phase A′, which moved here.

**It no longer blocks Track A.** That is the correction pass's structural fix: Task 3 ships on
`v1.11.1` without waiting for this release.

Seed the *algorithm* from `clientpg-manager`'s implementation — the authoritative one, because that is
what the login path verifies (`usecases/auth_login.go:145,454`) — so the scheme is provably unchanged:

```go
// Package pcauth provides PayCloud's canonical password hashing.
//
// The scheme is bcrypt over the plaintext with a per-user UUID appended as a pepper:
//
//	hash = bcrypt(plaintext + salt, cost)
//
// Both values are stored in the clear; the salt is a plain column, not a secret. This mirrors what
// clientpg-manager's login path verifies, so hashes produced here are accepted there. Do not vary
// the concatenation order or the salt shape — existing hashes cannot be re-derived.
package pcauth

// DefaultCost is the target bcrypt cost for newly issued hashes. It matches every hash currently in
// the database ($2a$10$…). BCRYPT_COST overrides it, clamped to [bcrypt.MinCost, 14].
const DefaultCost = 10

func GenerateHashAndSalt(plaintext string) (salt, hash string, err error)
func VerifyAndMaybeRehash(plaintext, salt, hash string) (VerificationResult, error)
```

> ### ⚠️ Correction — "verbatim" is not achievable, and the difference is not cosmetic
>
> **Added 2026-08-06.** Three signatures exist today and no two agree:
>
> | Source | Signature |
> |---|---|
> | `clientpg-manager` `libs/utils.go:189` *(the "canonical" one)* | `GenerateHashAndSalt(data string) (string, string)` — **no error** |
> | `merchantpg-manager` `feat/PA-222` `helpers/password.go` | `GenerateHashAndSalt(data string) (string, string, error)` |
> | `pcauth` as specified above | `GenerateHashAndSalt(plaintext string) (salt, hash string, err error)` |
>
> The canonical implementation **swallows the bcrypt error** (`libs/utils.go:190` discards it). `pcauth`
> returning it is the *right* design — but it means Task 4b is a **signature change at the 11 generate sites**,
> not a drop-in. §4b's "keeping every existing call site behaviourally identical" was wrong as written
> and is corrected there.
>
> **What must stay verbatim is the *scheme*, not the signature:** `bcrypt(plaintext + salt, cost)` with
> a UUID salt and `DefaultCost = 10`, `BCRYPT_COST` clamped to `[bcrypt.MinCost, 14]`. That is what the
> guard test below pins, and it is the only part that can kill deployed hashes.

**Guard test in `paycloudhelper`** — pins the algorithm so a later refactor cannot silently invalidate
every deployed hash:

```go
func TestHashIsCompatibleWithDeployedHashes(t *testing.T) {
	// A real cost-10 pair produced by clientpg-manager's implementation. If this stops verifying,
	// the extraction changed the scheme and every deployed hash is dead.
	const plaintext, salt = "PassQwerty123!", "e07ac3ac-5cc0-4b38-8087-96444a2155c7"
	...
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(plaintext+salt)); err != nil {
		t.Fatalf("scheme drift: %v", err)
	}
}
```

**Release as `v1.12.0`** — additive over **`v1.11.1`**, no breaking change.

**Gate:** `v1.12.0` tagged and resolvable via `go get`.

---

## 3. Task 3 — merchantpg-manager: bump + import swap — **Track A, ready now**

One commit. Task 1 proved the two-step split is unnecessary; `fundtransfer-manager` is the reference
shape.

> **🔧 Retargeted 2026-08-06: `v1.11.1`, not `v1.12.0`.** This task no longer waits on Task 2. The
> Redis debt is a `go get` plus a three-line import swap; deferring it behind an unwritten library
> package bought nothing. `v1.11.1` carries the go-redis v9 alignment from `v1.10.0`, the TTL clamp
> from `v1.10.2` (§0.4), and the `v1.11.1` audit-trail log-volume patch. `pcauth` arrives later in
> Task 4a as a second, trivial bump — the cost of decoupling is exactly one extra `go get`.

**Step 1** — dependencies:

```bash
go get github.com/PayCloud-ID/paycloudhelper@v1.11.1
go get github.com/redis/go-redis/v9@v9.18.0     # match fundtransfer-manager's pin
go mod tidy                                      # go-redis/v8 should drop out or fall to // indirect
```

**Step 2** — swap the import. **Three files on `feat/PA-222`** (`helpers/redis.go:9`,
`providers/redis.go:14`, `services/merchant-settings/merchant-bulk.go:23`); two if this somehow lands
before the bulk-import merge. Re-run Task 1's greps at the time of the work rather than trusting
either count:

```go
// BEFORE
import redis "github.com/go-redis/redis/v8"
// AFTER
import redis "github.com/redis/go-redis/v9"
```

Nothing else in those files changes — no `IdleTimeout`, no redsync adapter, no options renames.

**Step 3** — regression scope for the `v1.9.1 → v1.11.1` span. Scope it from the manager's **actual**
pchelper symbol surface, re-verified 2026-08-06:

`ConfigureLogForwarding`, `Detail`, `InitSentry`, `LogAudittrailData`, `LogAudittrailProcess`,
`LogD/E/I/W`, `LogForwardConfigFromEnv`, `LogSetLevel`, `LoggerErrorHub`, `Request`,
`RequestAndResponse`, `SetUpRabbitMq`.

That is **logging, Sentry, audit-trail, HTTP client and RabbitMQ setup** — no Redis, no middleware, no
JWT. So:

- **`v1.8.x` — `AuditPublisher` worker pool** (functional options, circuit breaker) — **the real
  regression risk.** The manager uses `LogAudittrailData` / `LogAudittrailProcess`, and the
  bulk-import feature's Task 12 leans hard on that path. Verify it still behaves under import load.
- **`v1.11.1` — audit-trail log volume:** per-event echo demoted, publish-success line rate-limited.
  Expect **fewer** audit log lines after deploy — that is the fix, not a regression. Tell whoever
  watches the dashboards.
- **`v1.11.0` — `rmq-autoconnect`:** reconnect/re-init hardening (mutex-protected conn/channel,
  `WaitGroup` on stop, errors instead of panic/fatal) plus **redacted AMQP credentials in logs**. The
  manager calls `SetUpRabbitMq` — exercise a broker restart.
- **`v1.10.0` — Added:** `miniredis` test support, `CheckHealth` + Redis metrics, `phtrace`
  propagator/resource, `ValidateConfiguration` with Redis options. Additive; opportunities, not work.

> ### ⚠️ Correction — the `RevokeToken` regression concern was misattributed
>
> **Removed 2026-08-06.** The earlier revision said *"the manager verifies JWTs via
> `middlewares.VerifyToken` — confirm nothing depended on the panic"* and gated staging on it. **It
> does not.** The manager has its **own** middleware at `handlers/middlewares/token.go`, built on
> `github.com/golang-jwt/jwt/v5` plus a gRPC call to clientpg's `VerifyTokenService`
> (`grpc/pb/client_pg/auth.pb.go`). pchelper appears in that file only as `LogD` / `LogE`.
>
> `RevokeToken` is not in the manager's symbol surface at all, so `v1.11.0`'s JWT panic-vector fix is a
> **no-op here**. The corresponding staging gate tested nothing and has been removed from §6. The fix
> is still real and still matters — for the services that *do* use pchelper's middleware.

**Step 4** — gates:

```bash
go build ./... && go vet ./... && go test ./... && go test -race ./...
git grep -n "go-redis/redis/v8" -- '*.go'    # must be empty
```

**Commit:** `chore(deps): paycloudhelper v1.9.1 -> v1.11.1, go-redis v8 -> v9`

> **The old §3 fallback is now the plan.** It read: *"Fallback if `v1.12.0` slips — bump to `v1.11.0`
> instead… Do **not** let the password work block the Redis work."* The correction pass promoted that
> from contingency to default. Keeping it as a fallback meant the default path still blocked on an
> unwritten package.

---

## 4. Task 4 — Put all three consumers on `pcauth` — **Track B**

This is the F-13/F-14 close-out. Order matters: the two *incompatible* implementations must not be
deleted before the shared one is in place.

### 4a — merchantpg-manager (closes bulk-import Task 14b Phase B)

**Prerequisite:** a second bump, `go get github.com/PayCloud-ID/paycloudhelper@v1.12.0` — trivial,
because Task 3 already crossed every breaking boundary. This is the whole cost of decoupling.

`helpers/password.go` **does not exist on `master`** — it arrives with the bulk-import feature on
`feat/PA-222`, where it already returns `(salt, hash string, err error)` and carries the
`TODO(merchant-bulk-import Task 4a)` marker. So this task's shape is: **match the import path, keep
the signature.** It is the cheapest of the three adoptions.

1. Point `helpers/password.go`'s callers at `pcauth.GenerateHashAndSalt`.
2. **Keep the tests unchanged.** `TestImportCredentialVerifiesWithTheConfiguredDefault` and the cost-10
   assertion are what prove the swap did not change the scheme. **If either needs editing, stop** — the
   extraction has drifted and every deployed hash is at risk.
3. Delete `helpers/password.go` and its `TODO`; drop `golang.org/x/crypto` back to indirect if nothing
   else needs it.
4. Update the bulk-import plan's Task 14b status and `merchantpg-manager/AGENTS.md`.

**Commit:** `refactor(auth): use pcauth.GenerateHashAndSalt, drop the local bcrypt copy`

### 4b — clientpg-manager (the source of truth moves out)

Replace `libs/utils.go`'s `GenerateHashAndSalt` (`:189`) / `VerifyAndMaybeRehash` (`:210`) /
`generateHashAndSaltAtCost` (`:265`) with `pcauth`, **preserving the scheme exactly** at every call
site.

> **🔧 Corrected 2026-08-06 — the call-site list was incomplete (9 listed, 14 actual), and this is a
> signature change, not a drop-in.**
>
> The earlier revision promised "keeping every existing call site behaviourally identical." That is not
> possible: `libs.GenerateHashAndSalt` returns `(string, string)` and discards the bcrypt error;
> `pcauth.GenerateHashAndSalt` returns `(salt, hash string, err error)` (§2). **Every one of the 11
> generate sites gains error handling.** Decide the policy once, up front, and apply it uniformly —
> silently dropping the error at 11 sites reproduces the flaw being extracted away from.

**Full call-site inventory, verified 2026-08-06 on `staging`:**

| File:line | Call | New in this pass |
|---|---|---|
| `usecases/auth_login.go:145` | `VerifyAndMaybeRehash` | |
| `usecases/auth_login.go:454` | `VerifyAndMaybeRehash` | |
| **`controllers/auth_otp_trusted_device.go:246`** | `VerifyAndMaybeRehash` | ⬅ **was missing** |
| `controllers/password.go:79` | `GenerateHashAndSalt` | |
| `controllers/password.go:297` | `GenerateHashAndSalt` | |
| `controllers/forgot.go:317` | `GenerateHashAndSalt` | |
| `controllers/pin.go:100` | `GenerateHashAndSalt` | |
| `controllers/pin.go:201` | `GenerateHashAndSalt` | |
| `usecases/auth_registration_validation.go:99` | `GenerateHashAndSalt` | |
| **`usecases/auth_registration_validation.go:163`** | `GenerateHashAndSalt` | ⬅ **was missing** |
| **`usecases/auth_registration_validation.go:215`** | `GenerateHashAndSalt` | ⬅ **was missing** |
| **`usecases/auth_registration_validation.go:274`** | `GenerateHashAndSalt` | ⬅ **was missing** |
| `usecases/auth_utils.go:19` | `GenerateHashAndSalt` | |
| `usecases/auth_utils.go:85` | `GenerateHashAndSalt` | |

Plus `libs/utils_rehash_test.go` — four tests (`…UpgradesLegacyCost`, `…SkipsCurrentCost`,
`…RejectsWrongPassword`, `TestGenerateHashAndSaltUsesConfiguredCost`) that must keep passing. As in
4a: **if a test needs editing beyond the signature, stop.**

The omission that matters most is `auth_otp_trusted_device.go:246` — a `VerifyAndMaybeRehash` site, so
the same login-adjacent risk class as the two `auth_login.go` sites, and it was invisible to anyone
working the old list.

⚠️ **This service owns login.** A regression here locks every merchant out, so it goes last of the two
managers and gets its own staging soak with a real login + password-reset cycle.

**Commit:** `refactor(auth): adopt pcauth, remove the local implementation`

### 4c — clientpg-module: delete the divergent helper (F-13) — **branch-gated**

`libs/utils.go:106-263` holds the incompatible variant: `DefaultBcryptCost = 12`, a 1–2 KB
`RandomByte` salt that `bcryptInput` then truncates to 72 bytes, and **AES-encrypted** hash *and* salt.
Rows written by it cannot be verified by the login path, and vice versa.

**The gate is the whole task** — it is dead on `develop`/`staging` but **live on `master`** via
`RegisterFullClient` (`controllers/register-user.go:194`, `services/client_service.go:30,182`), so
deleting on the wrong branch breaks the build:

```bash
git fetch origin
for id in GenerateHashAndSalt VerifyAndMaybeRehash PasswordVerificationResult; do
  git grep -n "$id" origin/develop -- '*.go' | grep -v 'libs/utils'   # must be empty
done
git grep -c RegisterFullClient origin/develop -- '*.go'               # must be 0
```

✅ **Gate re-run 2026-08-06: both conditions hold.** No external references on `develop`; zero
`RegisterFullClient` hits there. Still live on `master` at the three sites above.

1. Delete from `develop` first; `go build ./... && go test ./...`.
2. **Do not cherry-pick to `master` yet.** Master must first receive develop's removal of
   `RegisterFullClient` — see the corrected merge check below.
3. Re-run the gate against `origin/master`, then apply the same deletion there.

> ### ⚠️ Corrected 2026-08-06 — step 2 is **no longer a fast-forward**
>
> The earlier revision asserted `origin/master` is a strict ancestor of `origin/develop` and gave this
> gate:
>
> ```bash
> git merge-base --is-ancestor origin/master origin/develop && echo "fast-forward, safe"
> ```
>
> **That command fails today.** Measured 2026-08-06:
>
> | | Commits |
> |---|---|
> | `master` ahead of `develop` | **1** — `d7ea79f` *"docs: point AGENTS.md at paycloud-docs documentation hub"* |
> | `develop` ahead of `master` | 133 |
> | merge base | `7978729` (2026-06-19) |
>
> The divergence is the org-wide AGENTS.md sweep of 2026-08-04, which landed directly on `master` in
> every repo. So it is a **one-commit, docs-only merge**, not a conflict risk — but a reader following
> the plan literally hits a failing gate and stops. Replace step 2's check with:
>
> ```bash
> # Confirm the ONLY thing master has that develop lacks is the docs commit.
> git log --oneline origin/develop..origin/master     # expect: 1 docs-only commit
> git diff --stat origin/develop..origin/master       # expect: AGENTS.md only, no .go files
> ```
>
> If that holds, merge `develop → master` normally. If any `.go` file appears, **stop** — the
> divergence has grown past what this plan assessed and 4c needs re-scoping.

**Commit:** `chore: remove divergent bcrypt helper superseded by pcauth (F-13)`

**Done when:** `pcauth` is the only **password** bcrypt implementation in the org, and F-13/F-14 can
be closed.

> **🔧 Scoped 2026-08-06.** The original read *"the only bcrypt implementation in the org"* — not
> reachable, and not the goal. `clientpg-manager` `helpers/helpers.go:374` also calls
> `bcrypt.GenerateFromPassword`, for **OTP** hashing at `bcrypt.DefaultCost` — a different secret with
> a different lifetime and no salt column. It is deliberately out of scope. Say "password" so nobody
> later reads the done-condition as unmet, or folds OTP hashing into `pcauth` on the strength of it.

---

## 5. Task 5 — Correct the migration skill ✅ DONE 2026-07-30

**File:** `be-services/ai-agents/skills/pchelper-redis-v9-migration/SKILL.md` — committed as `3a87124`
on branch `local` (pushed), 185 → 234 lines, `make sync` run and the corrected description confirmed
live in `~/.agents/skills/` and `~/.claude/skills/`. Authored in the workspace, never in
`~/.claude/skills` (a symlink).

- ✅ Break is at **`v1.10.0`**, not `v2.0.0`; there is no `v2.x`.
- ✅ Module path is `github.com/PayCloud-ID/paycloudhelper`, not bitbucket.
- ✅ §0.3's reference table added.
- ✅ The mixed-version trap + its grep gate added.
- ✅ **Frontmatter `description` rewritten** — the highest-impact fix (§0.2).
- ✅ Tag dates are not in version order: `v1.9.1` (2026-04-29) post-dates `v1.10.0` (2026-04-24).

### 5b — the same error upstream ✅ DONE and MERGED 2026-07-31

`paycloudhelper`'s own consumer docs carried the same `v2.0.0` claim and instructed
`go get …@v2.0.0` — a tag that **404s**, so a reader stopped at the first command.

**Scope, after checking each file** — one skill plus three docs, *not* four skills (`echo-api`,
`scheduler`, `worker` are clean and defer to `core`):

| File | Fix |
|---|---|
| `README.md` §Consumer Migration | retitled `v1.10.0`, `go get` target `v1.11.0`, TOC anchor, back-patch note |
| `.agents/skills/redis-v9-consumer-migration-core/SKILL.md` | `description` + Use-When rescoped to "across `v1.10.0`" |
| `…/core/COMMON_MISTAKES.md` | added "looking for a nonexistent `v2.0.0`" |
| `AGENTS.md` skill index | row rescoped |
| `docs/plans/2026-05-25-redis-provider-migration.md` | inline correction — it told five services to swap "when they bump to v2.x" |

**`CHANGELOG.md` needed no change** — its `v1.10.0` entry already documents the break correctly under
"Changed (Breaking)" and makes no `v2.0.0` claim.

✅ **Merged to `paycloudhelper` `main` on 2026-07-31** — `2e46efa..04cbe42`, fast-forward, no PR (owner
directed), branch deleted locally and on the remote. The `go get …@v2.0.0` instruction that 404s is out
of that repo. Deliberately untouched: the generic deprecation-policy references to a future `v2.0.0`
(`.agents/rules/api-compatibility.md`, `library-maintenance`, copilot-instructions) — correct as policy.

---

## 6. Task 6 — Rollout

The manager is a **request-path Echo API service**, so the skill's Echo profile applies: initialise
Redis before registering middleware, keep key formats byte-identical (running clients depend on them),
and do not change duplicate-idempotency response shapes.

**Track A rollout** (Task 3 — the `v1.11.1` bump + import swap):

| Stage | Gate |
|---|---|
| **dev** | Deploy. Exercise **all three** Redis sites from Task 1. Replay a bulk import twice to prove the idempotency store still returns `X-Idempotent-Replay: true`. Watch cache metrics. |
| **staging** | Deploy and **soak 24 h**. Watch cache hit rate. **Exercise the audit-trail path under import load** — that is the one genuinely changed surface (`AuditPublisher` since `v1.8.x`, log-volume patch in `v1.11.1`); expect *fewer* audit lines, and tell whoever watches the dashboards so the drop is not read as breakage. **Restart the broker once** to exercise `v1.11.0`'s `rmq-autoconnect` hardening, and confirm AMQP credentials are redacted in the logs. |
| **prod** | Roll gradually. ⚠️ **Rollback is a single revert of the Task-3 commit** (pin + imports together). That is the cost of the single-step approach, accepted because Task 1 showed the surface is three files. |

> **🔧 Removed 2026-08-06 — "Verify admin JWT flows, `RevokeToken` changed in `v1.11.0`."** The manager
> does not use pchelper's JWT middleware (§3 Step 3), so that gate exercised nothing. Deleting a gate
> that cannot fail is not a reduction in coverage — a gate that always passes teaches the team to trust
> the whole checklist less. The `v1.11.0` `RevokeToken` fix is real and matters for services that *do*
> consume `middlewares.VerifyToken`.

**Track B rollout** (Tasks 4a–4c — `pcauth` adoption): after **each** adoption, run a **real password
reset + login** on staging for an affected account. For 4a that is an imported merchant; for 4b it is
a merchant login plus a full forgot-password cycle. That round trip is the only end-to-end proof
`pcauth` matches the deployed scheme — a green unit suite is not, because the guard test and the
implementation can drift together.

No env-data or `gitops-web` changes in either track: Redis keys are unchanged and this plan adds no new
environment variables. (The bulk-import feature's three password env vars are separate and already in
its Task 16.)

---

## 7. Risks

| Risk | Mitigation |
|---|---|
| ~~A `redis.*` value crosses the pchelper boundary~~ | **Closed by Task 1** — nothing crosses; the only grep hit is a commented-out line (`providers/redis.go:51`), re-confirmed 2026-08-06 on `master` and `feat/PA-222` |
| ~~`v1.12.0` slips, blocking the Redis work~~ | **Closed 2026-08-06 by decoupling** — Track A ships on `v1.11.1` and never waits on `pcauth` |
| ~~`RevokeToken`'s `v1.11.0` change alters admin-token behaviour~~ | **Not applicable** — the manager does not use pchelper's JWT middleware (§3 Step 3) |
| Redis key format drift silently breaks running clients | Keys are untouched by design; the skill lists this as the top mistake. Diff key-building code in review |
| **Audit-trail behaviour changes across the `v1.9.1 → v1.11.1` span** | The one genuinely-changed surface the manager uses. Exercise `LogAudittrailData` / `LogAudittrailProcess` under import load on staging; expect *fewer* log lines by design (`v1.11.1`) |
| AMQP reconnect regressions from `v1.11.0`'s `rmq-autoconnect` rewrite | The manager calls `SetUpRabbitMq`. Restart the broker once on staging and confirm clean reconnect + redacted credentials |
| **`pcauth` extraction subtly changes the scheme → every deployed hash dies** | Task 2's `TestHashIsCompatibleWithDeployedHashes` pins it; Task 4a's tests must pass **unedited**; staging does a real reset + login |
| **`pcauth`'s new `error` return is silently dropped at 11 sites**, reproducing the flaw being extracted | Decide the error policy **once** in 4b before editing; review the diff for discarded returns. The canonical implementation swallowing this error is precisely why it is being replaced (§2) |
| 4b regresses login for every merchant | clientpg-manager goes last, with its own soak. **14 call sites, not 9** — including `auth_otp_trusted_device.go:246`, a verify site missed by the earlier list |
| 4c deleted on the wrong branch breaks the build | The branch gate **is** the task — `develop` first, then a **one-commit docs-only merge** to `master` (no longer a fast-forward, §4c) |
| Single-step means one revert undoes both pin and imports | Accepted (Task 1: three files on `feat/PA-222`). The alternative bought isolation this codebase does not need |
| Bulk-import and this migration touch Redis at once | Explicit ordering: bulk-import first, then Task 3; re-run Task 1's greps afterwards |
| **TODO-2 is recorded as closed for this service when it is not** | Track A satisfies the *version* requirement but the clamp is inert here (§0.4). Raised as **F-17**; do not tick merchantpg-manager green in the redis doc's table without qualification |

---

## 7b. Task 7 — Close the documentation, or the record stays wrong

Small, and the one task most likely to be skipped — which is exactly why it is written down. Every
item here is a claim that becomes **false** the moment the code lands, in a doc someone will later
trust.

| # | File | Change | Trigger |
|---|---|---|---|
| 1 | bulk-import plan `§4.9` findings table | Mark **F-13** and **F-14** fixed. They currently sit under *"Needing separate tickets"*; after Task 4c they are done | Task 4c merges |
| 2 | bulk-import plan Task 14b | Flip Phase A → superseded, Phase B → done; note `helpers/password.go` is deleted | Task 4a merges |
| 3 | bulk-import plan `§1.4a` / F-13 narrative | It describes `clientpg-module`'s divergent helper in the present tense. Past-tense it, keep the history | Task 4c merges |
| 4 | **This** plan | Mark Tasks 2, 3, 4a–4c done as each lands | each task |
| 5 | `merchantpg-manager/AGENTS.md` | **Re-targeted 2026-08-06** — `master` has *no* bcrypt entry. The line to edit is `feat/PA-222:AGENTS.md:145` ("Compute bcrypt for the default import password in this manager…"), which reaches `master` with the bulk-import merge. Edit it there, or after that merge | Task 4a merges |
| 6 | ~~`merchantpg-module/README.md` §Password hashing~~ | **Dropped 2026-08-06 — no such section exists.** That README contains no bcrypt, `pcauth`, or password-hashing text at all. Nothing to un-hedge | — |
| 7 | `paycloudhelper` README / `AGENTS.md` | Add `pcauth` to the package list and the skill index | Task 2 merges |
| 8 | **`backend/redis/2026-05-23-redis-key-namespacing-and-ttl-fixes.md`** §TODO-2 | **New 2026-08-06.** Move `merchantpg-manager` out of the "❌ no clamp" row — **with the F-17 qualifier** (§0.4): the version requirement is met, the clamp is inert. Do not mark it plainly green | Task 3 merges |
| 9 | `pchelper-redis-v9-migration` skill | ✅ **DONE 2026-08-06** — did not wait for Task 3, since a wrong skill misleads every service, not just this one. Added a *Version boundaries* section (all three lines), retargeted the default pin to `v1.11.1`, four new Common Mistakes rows, and the "clamp is inert if you bypass the wrappers" grep (F-17 generalised). Source edited at `be-services/ai-agents/skills/pchelper-redis-v9-migration/SKILL.md` on branch `local`, `make sync` run | — |

**Also raise as their own tickets (not this plan's to fix):**

- **F-16** — `insert_new_user` and `insert_new_merchant_v2` each exist twice on dev PostgreSQL
  (`varchar` + `text` overloads with different parameter names). Belongs to whoever owns the Postgres
  migration.
- **F-17** *(new 2026-08-06)* — `merchantpg-manager` bypasses pchelper's Redis wrappers entirely, so
  the `v1.10.2+` TTL soft-clamp is compiled in but never applied to its writes (§0.4). Closing
  TODO-2's *intent* for this service means routing through the wrappers or clamping locally in
  `helpers/redis.go`. Sizeable enough to need its own scoping.

**Gate:** `grep -rn "F-13\|F-14" backend/plans/` returns nothing describing them as open, no doc still
promises a local password helper, and the redis doc's TODO-2 table no longer lists
`merchantpg-manager` as un-migrated.

---

## 8. Dependencies

**Corrected 2026-08-06.** The old graph had `Task 2 → Task 3`, which is the coupling this pass removed.

```
TRACK A — Redis (ready now, blocks nothing else)

  bulk-import Task 11 + 14b Phase A ──► Task 3: manager bump v1.9.1→v1.11.1 + import swap
                                              │
                                              └─► dev → staging 24h → prod


TRACK B — pcauth (independent; starts whenever Task 2 is written)

  Task 2: write pcauth, cut v1.12.0
        │
        ├─► Task 4a merchantpg-manager   (needs Task 3 merged: same repo, avoid a straddled bump)
        │
        └─► Task 4b clientpg-manager ──► Task 4c clientpg-module
              (owns login, goes last)      (branch-gated, F-13/14 closed)
```

- **Track A ↔ Track B** share exactly one edge: **4a comes after 3**, and only because both touch
  `merchantpg-manager`'s `go.mod` and sequencing them avoids a straddled bump. 4b and 4c are free to
  proceed the moment `v1.12.0` exists — they do not wait on Track A at all.
- **Task 1** ✅ done — its findings collapsed the old Tasks 2+3 into today's Task 3. Inventory is
  **stale by design**; re-run both greps at execution time.
- **Task 5** ✅ done — whoever executes Tasks 1–4 now reads correct instructions.
- **Task 5b** ✅ done and **merged into `paycloudhelper` `main`** 2026-07-31 (`2e46efa..04cbe42`,
  fast-forward, branch deleted). The `@v2.0.0` 404 trap is out of that repo.
- **Task 7** trails each of Tasks 2, 3, 4a and 4c — it is not a final step, it fires per merge.
- The bulk-import feature is the only upstream blocker on Task 3, and only for sequencing reasons.

---

## 9. Correction pass — 2026-08-06

What changed and why, so the reasoning survives the edit. Every item below was verified against a live
repo, not against another document.

### The structural change

**Tracks A and B were decoupled.** The plan bundled a `go get` plus a three-line import swap with the
design, review and release of a brand-new shared-library package, then sequenced the cheap work behind
the expensive work. The saving was one trivial dependency bump. The cost was an open-ended block on a
package that, six days after the plan was written, still did not exist in any form.

The tell was already in the document: §3 carried a fallback reading *"Do **not** let the password work
block the Redis work"* — correct instinct, wrong status. A rule that important does not belong in a
contingency branch. It is now the default path.

**Rule of thumb this produced:** when one deliverable is a version-string change and the other is a new
library package, they do not share a release. Bundling them prices the cheap thing at the risk of the
expensive one.

### The factual corrections

| # | Correction | Evidence |
|---|---|---|
| 1 | Latest tag is **`v1.11.1`** (2026-08-03), not `v1.11.0`. Track A retargeted | Full tag list on `paycloudhelper` `main` `03f05b5` |
| 2 | `pcauth` **does not exist** — not on `main`, not in any tag. Task 2 is unstarted, not in-progress | `ls pcauth` → absent; grep across the repo → no hits |
| 3 | 4c's **fast-forward claim is false**. `master` is 1 commit ahead (`d7ea79f`, the 2026-08-04 AGENTS.md sweep); the stated gate command fails. Replaced with a docs-only merge check | `git merge-base --is-ancestor` → non-zero; `rev-list --count` → 1 / 133 |
| 4 | `pcauth` **cannot be seeded verbatim** — the canonical implementation returns `(string, string)` and swallows the bcrypt error. 4b is a signature change at 11 of them | `clientpg-manager/libs/utils.go:189` |
| 5 | 4b touches **14 call sites, not 9**. The riskiest omission was `auth_otp_trusted_device.go:246`, a `VerifyAndMaybeRehash` site | `git grep` on `clientpg-manager` `staging` |
| 6 | The **`RevokeToken` regression concern was misattributed** and its staging gate tested nothing. The manager has its own JWT middleware over gRPC | `handlers/middlewares/token.go`; full pchelper symbol surface has no middleware/JWT/Redis symbol |
| 7 | Task 7 items **5 and 6 targeted text that does not exist** on `master`. 5 re-targeted to `feat/PA-222`, 6 dropped | `grep -i bcrypt` on both files → no hits |
| 8 | "Only bcrypt implementation in the org" is unreachable — OTP hashing also uses bcrypt. Scoped to *password* | `clientpg-manager/helpers/helpers.go:374` |

### The cross-document findings

| # | Finding | Where |
|---|---|---|
| 9 | **This migration was already owned by another doc.** The redis key-namespacing doc's TODO-2 names `merchantpg-manager` and frames the fix as *"this is the go-redis v9 migration, not a version bump."* Neither doc cited the other; now cross-linked | §0.4 |
| 10 | **A third version boundary exists** — the TTL soft-clamp lands at `v1.10.2`, not `v1.10.0`. Track A's target clears it, but deliberately rather than accidentally | §0.4 |
| 11 | **The clamp will be inert here** — the manager calls zero pchelper Redis wrappers, so `≥ v1.10.2` buys the version requirement without the protection. Raised as **F-17** | §0.4, §7b |
| 12 | **The `v1.10.0` break line is a GA statement, not a first-appearance one.** go-redis v9 lands at `v1.9.2-beta.1`; that tag and `beta.2` are the only v9-bearing go-1.24 releases, which is why `qoinhubinterface-manager` sits there. The skill's flat claim misleads there | §0.1, Task 7 item 9 |

### What was checked and found correct

§0.1's break location · §0.2 and Tasks 5 / 5b (`04cbe42` confirmed in `main`) · §0.3's reference repos
· §1's boundary grep, still clean on both `master` and `feat/PA-222` · the three-import-site count on
`feat/PA-222` · the single-step conclusion · 4c's `RegisterFullClient` branch gate, which still holds
in both directions · `merchantpg-module` correctly excluded.
