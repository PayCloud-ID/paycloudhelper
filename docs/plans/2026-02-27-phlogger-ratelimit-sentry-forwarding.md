# phlogger Rate-Limit + Sentry Forwarding + Docs Update Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add rate-limited logging, configurable log-to-Sentry forwarding, README refresh, AGENTS.md update, and phsentry production hardening — all backward-compatible (MINOR bump v1.6.6 → v1.7.0).

**Architecture:**
- `phlogger/ratelimit.go` — standalone rate-limiter using `sync.Map` + `time`, zero new deps
- `phlogger/forward.go` — log-forward hook system; phlogger calls registered hooks after every log emit
- `phsentry/` gains two things: a `ReceiveLog()` entry-point callable by phlogger hooks, and production hardening (flush, context, dedup fingerprints)
- Root `logger.go` gets `ConfigureLogForwarding(cfg LogForwardConfig)` — env-aware setup callable from consumer `main()`

**Tech Stack:** Go 1.23, `kataras/golog`, `github.com/getsentry/sentry-go`, stdlib `sync`, `time`, `golang.org/x/time/rate` (already transitive via redis — otherwise use hand-rolled bucket)

---

## Pre-Flight Checks

Before starting any task:

```bash
cd ./paycloudhelper
go test ./...          # all green baseline
go build ./...         # no compile errors
git status             # clean working tree (or stash)
git checkout -b feat/phlogger-ratelimit-sentry-v1.7.0
```

Expected: all tests pass, working tree clean, new branch created.

---

## Task 1: Update AGENTS.md — Version Policy Clarification

**Files:**
- Modify: `AGENTS.md`

This is a doc-only task. Adding explicit version bump guidance so agents and engineers always know when to bump what.

### Step 1: Edit the Versioning section in AGENTS.md

Find the existing `## Versioning` table and **replace** it with the expanded version below (no other content changes):

```markdown
## Versioning

| Bump | When | Examples |
|------|------|---------|
| **PATCH** | Bug fix, zero behavior change, no new public API | Fix nil panic, fix typo in log message |
| **MINOR** | New backward-compatible additions (new functions, new optional config) | `ConfigureLogForwarding()`, `LogRateLimited()` |
| **MAJOR** | Any breaking change to existing public API signatures OR removal of exported symbols | Rename `InitSentry` params, remove `LogErr` |

### Breaking Change Decision Tree

```
Does the change touch an EXISTING exported function signature?
├─ YES → MAJOR bump required. Coordinate consumer updates first.
└─ NO  → Does it add new exported symbols?
          ├─ YES → MINOR bump (v1.X.0)
          └─ NO  → PATCH bump (v1.6.X)
```

### Backward-Compatibility Contract

- NEVER change existing exported function signatures
- NEVER remove exported symbols without a deprecation cycle (MINOR → MAJOR)
- New optional parameters → use variadic `opts ...Option` pattern
- Deprecated functions retain original signature + route to new implementation

**Known retractions:** v1.6.3 (verbose Redis logs), v1.6.0 (audit trail race), v1.5.2 (nil panic on init)
```

### Step 2: Commit

```bash
git add AGENTS.md
git commit -m "docs: expand versioning policy with decision tree in AGENTS.md"
```

---

## Task 2: Recreate README.md from AGENTS.md

**Files:**
- Modify: `README.md`

The current README.md is just `# paycloudhelper\nPayCloud Hub Helper Golang`. Replace it with a full developer-facing reference derived from AGENTS.md.

### Step 1: Replace README.md entirely

Write the following content to `README.md`:

```markdown
# paycloudhelper

**Go shared library** — common utilities for all PayCloud Hub microservices.

Module: `bitbucket.org/paycloudid/paycloudhelper`  
Go: 1.23 + toolchain 1.24.3

---

## Table of Contents
- [Overview](#overview)
- [Quick Start](#quick-start)
- [Package Structure](#package-structure)
- [API Reference](#api-reference)
- [Configuration](#configuration)
- [Versioning](#versioning)
- [Contributing](#contributing)

---

## Overview

`paycloudhelper` is a **shared library** imported by ~30 PayCloud microservices. It is **not a standalone service**. On import, `init()` runs automatically and bootstraps the logger and app identity. Consumer services then explicitly opt into Redis, RabbitMQ, and Sentry.

### Auto-Initialization Flow

```
import paycloudhelper → init() runs:
  AddValidatorLibs() → InitializeLogger() → InitializeApp()

Consumer must explicitly call:
  InitializeRedisWithRetry(opts)   → Redis pool + RedSync
  SetUpRabbitMq(...)               → Audit trail
  InitSentry(options)              → Error tracking (optional)
  ConfigureLogForwarding(cfg)      → Log → Sentry forwarding (optional)
```

---

## Quick Start

```go
import pch "bitbucket.org/paycloudid/paycloudhelper"

// In main() — after godotenv.Load()
pch.InitializeRedisWithRetry(pch.RedisInitOptions{...})
pch.SetUpRabbitMq(...)
pch.InitSentry(pch.SentryOptions{Dsn: os.Getenv("SENTRY_DSN")})

// Optional: forward Fatal logs to Sentry automatically
pch.ConfigureLogForwarding(pch.LogForwardConfig{
    ForwardFatal: true, // default true when Sentry is enabled
})
```

---

## Package Structure

| Package | Path | Purpose |
|---------|------|---------|
| Root | `.` | Public API — all below re-exported here |
| `phlogger` | `phlogger/` | Logger wrapper (`kataras/golog`) + rate limiter + forwarding hooks |
| `phsentry` | `phsentry/` | Sentry error tracking, log receiver |
| `phhelper` | `phhelper/` | Global state (`APP_NAME`, `APP_ENV`), JSON/string helpers |
| `phaudittrailv0` | `phaudittrailv0/` | Legacy v0 audit trail (RabbitMQ) |
| `phjson` | `phjson/` | Sonic JSON wrapper for high-throughput consumers |

---

## API Reference

### Logging

```go
pch.LogI("[FuncName] started id=%s", id)    // Info
pch.LogE("[FuncName] error: %v", err)        // Error
pch.LogW("[FuncName] warn: %s", msg)         // Warning
pch.LogD("[FuncName] debug key=%s", key)     // Debug (off in production)
pch.LogF("[FuncName] fatal: %v", err)        // Fatal — process exits
pch.LogJ(obj)                                // JSON (compact)
pch.LogJI(obj)                               // JSON (indented)
```

#### Rate-Limited Logging (opt-in)

```go
// Log at most 1 message per key per 10 seconds
pch.LogIRated("cache.miss", 10*time.Second, "[FuncName] cache miss key=%s", key)
pch.LogERated("db.error", 5*time.Second, "[FuncName] db error: %v", err)
```

#### Log Forwarding to Sentry

```go
// Call once at startup — configure which levels forward to Sentry
pch.ConfigureLogForwarding(pch.LogForwardConfig{
    ForwardFatal: true,  // default: true when Sentry enabled
    ForwardError: false, // default: false
    ForwardWarn:  false, // default: false
    // OR load from env: pch.LogForwardConfigFromEnv()
})
```

Environment variables (evaluated by `LogForwardConfigFromEnv()`):

| Env Var | Default | Effect |
|---------|---------|--------|
| `LOG_FORWARD_FATAL` | `true` | Forward Fatal logs to Sentry |
| `LOG_FORWARD_ERROR` | `false` | Forward Error logs to Sentry |
| `LOG_FORWARD_WARN` | `false` | Forward Warn logs to Sentry |
| `LOG_FORWARD_INFO` | `false` | Forward Info logs to Sentry |

### Response

```go
var resp pch.ResponseApi
resp.Success("ok", data)            // 200
resp.Accepted(data)                 // 202
resp.BadRequest("msg", "ERR_CODE")  // 400
resp.Unauthorized("msg", "")        // 401
resp.InternalServerError(err)       // 500
return c.JSON(resp.Code, resp)
```

### Redis

```go
pch.StoreRedis(key, value, duration)
pch.GetRedis(key)
pch.StoreRedisWithLock(key, value, duration)
pch.AcquireLockWithRetry(key, ttl, retries, delay)
pch.ReleaseLockWithRetry(mutex, retries)
```

### Sentry

```go
pch.InitSentry(pch.SentryOptions{
    Dsn:         os.Getenv("SENTRY_DSN"),
    Environment: os.Getenv("APP_ENV"),
    Release:     "v1.7.0",
})
pch.SendSentryError(err)
pch.SendSentryMessage("something happened")
pch.FlushSentry(2 * time.Second)  // call before process exit
```

### Middleware (Echo)

```go
e.Use(pch.VerifCsrf)       // X-Xsrf-Token validation
e.Use(pch.VerifIdemKey)    // Idempotency-Key deduplication
e.Use(pch.RevokeToken)     // JWT + Redis revocation check
```

---

## Configuration

All configuration is loaded from environment variables in `InitializeApp()`:

| Var | Required | Default | Purpose |
|-----|----------|---------|---------|
| `APP_NAME` | ✅ | `""` | Service name (used in Sentry, logs) |
| `APP_ENV` | ✅ | `""` | `develop` / `staging` / `production` |
| `REDIS_HOST` | For Redis | `""` | Redis server |
| `REDIS_PORT` | For Redis | `6379` | Redis port |
| `REDIS_PASSWORD` | No | `""` | Redis auth |
| `SENTRY_DSN` | For Sentry | `""` | Sentry project DSN |
| `LOG_FORWARD_FATAL` | No | `true` | Forward Fatal → Sentry |
| `LOG_FORWARD_ERROR` | No | `false` | Forward Error → Sentry |
| `LOG_FORWARD_WARN` | No | `false` | Forward Warn → Sentry |
| `LOG_FORWARD_INFO` | No | `false` | Forward Info → Sentry |
| `TRANSACTION_REDIS_LOCK_TIMEOUT` | No | `2000` (ms) | Distributed lock TTL |
| `TRANSACTION_REDIS_BACKOFF` | No | `10` (ms) | Lock retry backoff |

---

## Versioning

| Bump | When |
|------|------|
| **PATCH** | Bug fixes, zero behavior change |
| **MINOR** | New backward-compatible features |
| **MAJOR** | Breaking changes — requires coordinating all consumer updates |

---

## Contributing

1. `git checkout -b feat/your-feature`
2. Write failing test first (TDD)
3. Implement minimal code
4. `go test -race ./...` — must pass
5. `go build ./...` — must pass
6. `git tag vX.Y.Z` when ready to release

See `.agents/rules/` and `AGENTS.md` for full development rules.
```

### Step 2: Run build + test to verify no regressions

```bash
go build ./...
```

Expected: no errors.

### Step 3: Commit

```bash
git add README.md
git commit -m "docs: recreate README.md from AGENTS.md with full API reference"
```

---

## Task 3: Rate-Limit Logger — Core Implementation in phlogger

**Files:**
- Create: `phlogger/ratelimit.go`
- Modify: `phlogger/phlogger.go`

### Background: What Rate-Limited Logging Solves

In high-traffic services, the same error can be logged thousands of times per second (e.g., Redis timeout in a hot path). Rate-limited logging emits the first occurrence, suppresses duplicates for a configurable window, and optionally logs a "suppressed N times" summary at window end.

**Design:**
- Key = caller-provided string (e.g., `"cache.miss"` or `"db.error"`) — not auto-hashed from message (avoids allocation)
- Per-key state: `lastEmit time.Time`, `suppressed int64`
- On emit attempt: if `now - lastEmit < window`, increment `suppressed`, return suppressed=true
- On first emit or after window: log message + if `suppressed > 0` append `[+N suppressed]`
- Cleanup: entries older than 10× window are swept lazily on access (no goroutine needed)
- Thread-safe: `sync.Map`

### Step 1: Write the failing tests in `phlogger/ratelimit_test.go`

Create `phlogger/ratelimit_test.go`:

```go
package phlogger

import (
	"testing"
	"time"
)

func TestRateLimiter_AllowsFirstEmit(t *testing.T) {
	rl := newRateLimiter()
	allowed, suppressed := rl.check("test.key", 1*time.Second)
	if !allowed {
		t.Fatal("first emit should be allowed")
	}
	if suppressed != 0 {
		t.Fatalf("expected 0 suppressed, got %d", suppressed)
	}
}

func TestRateLimiter_SuppressDuplicates(t *testing.T) {
	rl := newRateLimiter()
	rl.check("test.key", 1*time.Second) // first: allowed

	allowed, _ := rl.check("test.key", 1*time.Second) // within window
	if allowed {
		t.Fatal("second emit within window should be suppressed")
	}
}

func TestRateLimiter_CountsSuppressed(t *testing.T) {
	rl := newRateLimiter()
	rl.check("k", 1*time.Second)
	rl.check("k", 1*time.Second)
	rl.check("k", 1*time.Second)

	// Now force window expiry
	entry, _ := rl.entries.Load("k")
	e := entry.(*rateLimitEntry)
	e.lastEmit = time.Now().Add(-2 * time.Second)

	allowed, suppressed := rl.check("k", 1*time.Second)
	if !allowed {
		t.Fatal("after window expiry, should allow")
	}
	if suppressed != 2 {
		t.Fatalf("expected 2 suppressed, got %d", suppressed)
	}
}

func TestRateLimiter_IndependentKeys(t *testing.T) {
	rl := newRateLimiter()
	rl.check("a", 1*time.Second)
	allowedB, _ := rl.check("b", 1*time.Second)
	if !allowedB {
		t.Fatal("different key should be allowed independently")
	}
}

func TestRateLimiter_ZeroWindow_AlwaysAllows(t *testing.T) {
	rl := newRateLimiter()
	rl.check("x", 0)
	allowed, _ := rl.check("x", 0)
	if !allowed {
		t.Fatal("zero window should always allow (rate limiting disabled)")
	}
}
```

### Step 2: Run tests to verify they fail

```bash
cd ./paycloudhelper
go test ./phlogger/... -run TestRateLimiter -v
```

Expected: `FAIL — package phlogger: build failed` (ratelimit.go doesn't exist yet).

### Step 3: Implement `phlogger/ratelimit.go`

Create `phlogger/ratelimit.go`:

```go
package phlogger

import (
	"sync"
	"sync/atomic"
	"time"
)

// rateLimitEntry holds per-key state for the rate limiter.
type rateLimitEntry struct {
	lastEmit   time.Time
	suppressed int64 // atomic counter
}

// rateLimiter is a thread-safe per-key rate limiter with no goroutine overhead.
// Keys are caller-provided strings (e.g. "cache.miss", "db.connect.error").
type rateLimiter struct {
	entries sync.Map // key string → *rateLimitEntry
}

// globalRateLimiter is the singleton used by LogIRated / LogERated etc.
var globalRateLimiter = newRateLimiter()

func newRateLimiter() *rateLimiter {
	return &rateLimiter{}
}

// check returns (allowed bool, suppressed int64).
//
//   - allowed=true: caller should emit the log message (first in window or window expired).
//     suppressed will be >0 if previous calls within the last window were suppressed.
//   - allowed=false: this call is within the active window; caller should skip logging.
//   - window=0: rate limiting disabled; always returns (true, 0).
func (r *rateLimiter) check(key string, window time.Duration) (allowed bool, suppressed int64) {
	if window <= 0 {
		return true, 0
	}

	now := time.Now()

	actual, _ := r.entries.LoadOrStore(key, &rateLimitEntry{lastEmit: time.Time{}})
	entry := actual.(*rateLimitEntry)

	// Fast path: within window — suppress.
	if !entry.lastEmit.IsZero() && now.Sub(entry.lastEmit) < window {
		atomic.AddInt64(&entry.suppressed, 1)
		return false, 0
	}

	// Window expired or first call: drain suppressed counter and allow.
	suppressed = atomic.SwapInt64(&entry.suppressed, 0)
	entry.lastEmit = now
	return true, suppressed
}
```

### Step 4: Run tests to verify they pass

```bash
go test ./phlogger/... -run TestRateLimiter -v
```

Expected: all 5 tests PASS.

### Step 5: Add `LogIRated`, `LogERated`, `LogWRated`, `LogDRated` to `phlogger/phlogger.go`

Append to the bottom of `phlogger/phlogger.go` (no existing code changes):

```go
// LogIRated logs at Info level with rate limiting.
// key identifies the log site (e.g. "cache.miss"). window is the suppression duration.
// If window <= 0, rate limiting is disabled and every call emits.
// When a suppressed window ends, the log message includes "[+N suppressed]".
func LogIRated(key string, window time.Duration, format string, args ...interface{}) {
	allowed, suppressed := globalRateLimiter.check(key, window)
	if !allowed {
		return
	}
	if suppressed > 0 {
		format = format + " [+%d suppressed]"
		args = append(args, suppressed)
	}
	LogI(format, args...)
}

// LogERated logs at Error level with rate limiting. See LogIRated for semantics.
func LogERated(key string, window time.Duration, format string, args ...interface{}) {
	allowed, suppressed := globalRateLimiter.check(key, window)
	if !allowed {
		return
	}
	if suppressed > 0 {
		format = format + " [+%d suppressed]"
		args = append(args, suppressed)
	}
	LogE(format, args...)
}

// LogWRated logs at Warning level with rate limiting. See LogIRated for semantics.
func LogWRated(key string, window time.Duration, format string, args ...interface{}) {
	allowed, suppressed := globalRateLimiter.check(key, window)
	if !allowed {
		return
	}
	if suppressed > 0 {
		format = format + " [+%d suppressed]"
		args = append(args, suppressed)
	}
	LogW(format, args...)
}

// LogDRated logs at Debug level with rate limiting. See LogIRated for semantics.
func LogDRated(key string, window time.Duration, format string, args ...interface{}) {
	allowed, suppressed := globalRateLimiter.check(key, window)
	if !allowed {
		return
	}
	if suppressed > 0 {
		format = format + " [+%d suppressed]"
		args = append(args, suppressed)
	}
	LogD(format, args...)
}
```

### Step 6: Add integration test for LogIRated in `phlogger/ratelimit_test.go`

Append to existing test file:

```go
func TestLogIRated_SuppressesInWindow(t *testing.T) {
	// Use a unique key to avoid interference from parallel tests
	key := "test.LogIRated.suppress"
	window := 500 * time.Millisecond

	// Reset global limiter state for this key
	globalRateLimiter.entries.Delete(key)

	// First call should emit
	LogIRated(key, window, "test emit")

	// Second call within window should be suppressed (no panic, no emit)
	LogIRated(key, window, "should be suppressed")
	LogIRated(key, window, "should be suppressed 2")

	// After window, next call should emit with suppressed count
	time.Sleep(window + 10*time.Millisecond)
	LogIRated(key, window, "after window") // should log "[+2 suppressed]"
}
```

### Step 7: Run all phlogger tests

```bash
go test ./phlogger/... -v -race
```

Expected: all tests PASS, no race conditions.

### Step 8: Commit

```bash
git add phlogger/ratelimit.go phlogger/ratelimit_test.go phlogger/phlogger.go
git commit -m "feat(phlogger): add per-key rate-limited logging (LogIRated/LogERated/LogWRated/LogDRated)"
```

---

## Task 4: Log Forwarding Hook System in phlogger

**Files:**
- Create: `phlogger/forward.go`
- Modify: `phlogger/phlogger.go`

### Background

The hook system enables any subscriber (e.g., Sentry integration) to receive log events without phlogger importing phsentry (which would create an import cycle). Instead, phlogger exposes a `RegisterLogHook()` function, and phsentry (or root `logger.go`) registers a handler at startup.

**Design:**
- `LogHook` = `func(level, message string)` — simple callback, no structs
- `RegisterLogHook(level string, hook LogHook)` — register handler for a log level
- `ClearLogHooks()` — for tests
- Hook is called **after** the underlying golog call (fire-and-forget, sync)
- Levels: `"debug"`, `"info"`, `"warn"`, `"error"`, `"fatal"`

### Step 1: Write failing tests in `phlogger/forward_test.go`

Create `phlogger/forward_test.go`:

```go
package phlogger

import (
	"testing"
)

func TestRegisterLogHook_CallsOnMatchingLevel(t *testing.T) {
	ClearLogHooks()
	called := false
	RegisterLogHook("error", func(level, message string) {
		called = true
		if level != "error" {
			t.Errorf("expected level 'error', got %q", level)
		}
	})

	fireHooks("error", "test error message")

	if !called {
		t.Fatal("hook was not called for matching level")
	}
}

func TestRegisterLogHook_SkipsNonMatchingLevel(t *testing.T) {
	ClearLogHooks()
	called := false
	RegisterLogHook("fatal", func(level, message string) {
		called = true
	})

	fireHooks("error", "not fatal")

	if called {
		t.Fatal("hook should not be called for non-matching level")
	}
}

func TestRegisterLogHook_MultipleHooksSameLevel(t *testing.T) {
	ClearLogHooks()
	count := 0
	RegisterLogHook("warn", func(level, message string) { count++ })
	RegisterLogHook("warn", func(level, message string) { count++ })

	fireHooks("warn", "some warning")

	if count != 2 {
		t.Fatalf("expected 2 hooks called, got %d", count)
	}
}

func TestClearLogHooks_RemovesAll(t *testing.T) {
	RegisterLogHook("info", func(level, message string) {
		t.Fatal("hook should have been cleared")
	})
	ClearLogHooks()
	fireHooks("info", "should not trigger hook")
}
```

### Step 2: Run tests to confirm they fail

```bash
go test ./phlogger/... -run TestRegisterLogHook -v
go test ./phlogger/... -run TestClearLogHooks -v
```

Expected: build failure (forward.go doesn't exist).

### Step 3: Implement `phlogger/forward.go`

Create `phlogger/forward.go`:

```go
package phlogger

import "sync"

// LogHook is a callback invoked after a log line is emitted.
// level is one of: "debug", "info", "warn", "error", "fatal".
// message is the formatted log string.
type LogHook func(level, message string)

var (
	hooksMu sync.RWMutex
	hooks   = make(map[string][]LogHook) // level → []LogHook
)

// RegisterLogHook adds a hook for the given log level.
// Multiple hooks can be registered for the same level; all are called in order.
// Safe to call from multiple goroutines.
func RegisterLogHook(level string, hook LogHook) {
	hooksMu.Lock()
	defer hooksMu.Unlock()
	hooks[level] = append(hooks[level], hook)
}

// ClearLogHooks removes all registered hooks. Primarily for testing.
func ClearLogHooks() {
	hooksMu.Lock()
	defer hooksMu.Unlock()
	hooks = make(map[string][]LogHook)
}

// fireHooks dispatches the given level + message to all registered hooks.
// Called internally after each log emit.
func fireHooks(level, message string) {
	hooksMu.RLock()
	hs := hooks[level]
	hooksMu.RUnlock()
	for _, h := range hs {
		h(level, message)
	}
}
```

### Step 4: Wire `fireHooks` into log functions in `phlogger/phlogger.go`

The existing `LogD/LogI/LogW/LogE/LogF` are direct function aliases (`= Log.Debugf`), which means we can't intercept them without replacing them with wrappers. We need to change these from var-alias to wrapper functions.

> ⚠️ **Backward compatible**: The function signatures `func(format string, args ...interface{})` remain identical. The change is that they are now wrapping functions instead of direct aliases. Callers notice no difference.

Replace the `var` block and add wrappers in `phlogger/phlogger.go`. The new file content for the log var section and functions:

```go
var (
	Log = golog.New()
	// Keep Logf as alias (generic level log)
	Logf = Log.Logf
)

var GinLevel golog.Level = 6

// LogD logs at Debug level and fires registered hooks.
func LogD(format string, args ...interface{}) {
	Log.Debugf(format, args...)
	fireHooks("debug", format)
}

// LogI logs at Info level and fires registered hooks.
func LogI(format string, args ...interface{}) {
	Log.Infof(format, args...)
	fireHooks("info", format)
}

// LogW logs at Warning level and fires registered hooks.
func LogW(format string, args ...interface{}) {
	Log.Warnf(format, args...)
	fireHooks("warn", format)
}

// LogE logs at Error level and fires registered hooks.
func LogE(format string, args ...interface{}) {
	Log.Errorf(format, args...)
	fireHooks("error", format)
}

// LogF logs at Fatal level (process exits) and fires registered hooks synchronously before exit.
func LogF(format string, args ...interface{}) {
	fireHooks("fatal", format) // fire BEFORE Fatalf since process will exit
	Log.Fatalf(format, args...)
}
```

> **Note on LogF hook ordering:** fire hooks **before** `Log.Fatalf` because `Fatalf` calls `os.Exit`. This ensures Sentry forwarding completes before process terminates.

Also update root `logger.go` to match — the root package re-exports these. Since root `logger.go` re-exports as `LogD = Log.Debugf` etc., we need to update those too (Task 5).

### Step 5: Run tests

```bash
go test ./phlogger/... -v -race
```

Expected: all tests PASS.

### Step 6: Commit

```bash
git add phlogger/forward.go phlogger/forward_test.go phlogger/phlogger.go
git commit -m "feat(phlogger): add log hook system for forwarding (RegisterLogHook, fireHooks)"
```

---

## Task 5: Update Root `logger.go` — Re-export Wrapper Functions

**Files:**
- Modify: `logger.go`

The root package currently re-exports `LogD/LogI/LogW/LogE/LogF` as direct aliases to `Log.Debugf` etc. Since phlogger now has wrapper functions, the root package must call the wrapper functions (not the golog backend directly) to ensure hooks fire for calls using the `pch.LogI(...)` shorthand.

### Step 1: Write a failing hook test in root package

Create `logger_hook_test.go` in root:

```go
package paycloudhelper

import (
	"testing"
	"bitbucket.org/paycloudid/paycloudhelper/phlogger"
)

func TestRootLogI_FiresHooks(t *testing.T) {
	phlogger.ClearLogHooks()
	called := false
	phlogger.RegisterLogHook("info", func(level, message string) {
		called = true
	})
	defer phlogger.ClearLogHooks()

	LogI("[TestRootLogI_FiresHooks] test message")

	if !called {
		t.Fatal("hook was not fired when calling root LogI")
	}
}

func TestRootLogE_FiresHooks(t *testing.T) {
	phlogger.ClearLogHooks()
	called := false
	phlogger.RegisterLogHook("error", func(level, message string) {
		called = true
	})
	defer phlogger.ClearLogHooks()

	LogE("[TestRootLogE_FiresHooks] test error")

	if !called {
		t.Fatal("hook was not fired when calling root LogE")
	}
}
```

### Step 2: Run test to confirm current state (will fail if LogI is still a direct alias)

```bash
go test . -run TestRootLogI_FiresHooks -v
go test . -run TestRootLogE_FiresHooks -v
```

Expected: FAIL — hooks not called because root still uses direct golog aliases.

### Step 3: Update `logger.go` — replace aliases with wrapper calls

Replace the entire `var` block for log functions and replace with wrapper functions:

```go
package paycloudhelper

import (
	"bitbucket.org/paycloudid/paycloudhelper/phlogger"
	"github.com/kataras/golog"
	"time"
)

var (
	Log      = phlogger.Log
	GinLevel golog.Level = phlogger.GinLevel
	Logf     = Log.Logf
)

// LogD logs at Debug level.
func LogD(format string, args ...interface{}) {
	phlogger.LogD(format, args...)
}

// LogI logs at Info level.
func LogI(format string, args ...interface{}) {
	phlogger.LogI(format, args...)
}

// LogW logs at Warning level.
func LogW(format string, args ...interface{}) {
	phlogger.LogW(format, args...)
}

// LogE logs at Error level.
func LogE(format string, args ...interface{}) {
	phlogger.LogE(format, args...)
}

// LogF logs at Fatal level (process exits after hook execution).
func LogF(format string, args ...interface{}) {
	phlogger.LogF(format, args...)
}

// LogIRated logs at Info level with rate limiting. key identifies the log site.
func LogIRated(key string, window time.Duration, format string, args ...interface{}) {
	phlogger.LogIRated(key, window, format, args...)
}

// LogERated logs at Error level with rate limiting.
func LogERated(key string, window time.Duration, format string, args ...interface{}) {
	phlogger.LogERated(key, window, format, args...)
}

// LogWRated logs at Warning level with rate limiting.
func LogWRated(key string, window time.Duration, format string, args ...interface{}) {
	phlogger.LogWRated(key, window, format, args...)
}

// LogDRated logs at Debug level with rate limiting.
func LogDRated(key string, window time.Duration, format string, args ...interface{}) {
	phlogger.LogDRated(key, window, format, args...)
}

func InitializeLogger() {
	phlogger.InitializeLogger()
}

func LogSetLevel(levelName string) {
	phlogger.LogSetLevel(levelName)
}

// LogJ logs arg as compact JSON.
func LogJ(arg interface{}) {
	phlogger.LogJ(arg)
}

// LogJI logs arg as indented JSON.
func LogJI(arg interface{}) {
	phlogger.LogJI(arg)
}

// LogErr logs an error value.
func LogErr(err error) {
	phlogger.LogErr(err)
}
```

### Step 4: Run the hook tests

```bash
go test . -run TestRootLog -v
```

Expected: PASS.

### Step 5: Run full test suite

```bash
go test ./... -race
```

Expected: all tests PASS.

### Step 6: Commit

```bash
git add logger.go logger_hook_test.go
git commit -m "feat: update root logger.go to call phlogger wrappers (enables hook forwarding)"
```

---

## Task 6: Implement `LogForwardConfig` + Sentry Receiver in phsentry

**Files:**
- Create: `phlogger/logforward_config.go`
- Modify: `phsentry/phsentry.go`

### Step 1: Write failing tests in `phlogger/logforward_config_test.go`

Create `phlogger/logforward_config_test.go`:

```go
package phlogger

import (
	"os"
	"testing"
)

func TestLogForwardConfigFromEnv_Defaults(t *testing.T) {
	os.Unsetenv("LOG_FORWARD_FATAL")
	os.Unsetenv("LOG_FORWARD_ERROR")
	os.Unsetenv("LOG_FORWARD_WARN")
	os.Unsetenv("LOG_FORWARD_INFO")

	cfg := LogForwardConfigFromEnv()

	if !cfg.ForwardFatal {
		t.Error("ForwardFatal should default to true")
	}
	if cfg.ForwardError {
		t.Error("ForwardError should default to false")
	}
	if cfg.ForwardWarn {
		t.Error("ForwardWarn should default to false")
	}
	if cfg.ForwardInfo {
		t.Error("ForwardInfo should default to false")
	}
}

func TestLogForwardConfigFromEnv_EnvOverride(t *testing.T) {
	os.Setenv("LOG_FORWARD_FATAL", "false")
	os.Setenv("LOG_FORWARD_ERROR", "true")
	defer func() {
		os.Unsetenv("LOG_FORWARD_FATAL")
		os.Unsetenv("LOG_FORWARD_ERROR")
	}()

	cfg := LogForwardConfigFromEnv()

	if cfg.ForwardFatal {
		t.Error("ForwardFatal should be false when env=false")
	}
	if !cfg.ForwardError {
		t.Error("ForwardError should be true when env=true")
	}
}
```

### Step 2: Run tests to confirm fail

```bash
go test ./phlogger/... -run TestLogForwardConfig -v
```

Expected: FAIL (file doesn't exist).

### Step 3: Implement `phlogger/logforward_config.go`

Create `phlogger/logforward_config.go`:

```go
package phlogger

import "os"

// LogForwardConfig controls which log levels are forwarded to an external sink (e.g. Sentry).
// By default only Fatal forwarding is enabled when Sentry is initialized.
type LogForwardConfig struct {
	ForwardDebug bool // Forward Debug logs (default: false)
	ForwardInfo  bool // Forward Info logs (default: false)
	ForwardWarn  bool // Forward Warning logs (default: false)
	ForwardError bool // Forward Error logs (default: false)
	ForwardFatal bool // Forward Fatal logs (default: true)
}

// LogForwardConfigFromEnv constructs a LogForwardConfig reading from environment variables.
// Variables: LOG_FORWARD_FATAL (default "true"), LOG_FORWARD_ERROR, LOG_FORWARD_WARN, LOG_FORWARD_INFO (default "false").
func LogForwardConfigFromEnv() LogForwardConfig {
	return LogForwardConfig{
		ForwardFatal: getEnvBool("LOG_FORWARD_FATAL", true),
		ForwardError: getEnvBool("LOG_FORWARD_ERROR", false),
		ForwardWarn:  getEnvBool("LOG_FORWARD_WARN", false),
		ForwardInfo:  getEnvBool("LOG_FORWARD_INFO", false),
		ForwardDebug: getEnvBool("LOG_FORWARD_DEBUG", false),
	}
}

// getEnvBool reads an env var and returns its boolean value, defaulting to def if unset or unparseable.
func getEnvBool(key string, def bool) bool {
	val := os.Getenv(key)
	if val == "" {
		return def
	}
	return val == "1" || val == "true" || val == "yes"
}
```

### Step 4: Run config tests

```bash
go test ./phlogger/... -run TestLogForwardConfig -v
```

Expected: PASS.

### Step 5: Add `ReceiveLog` to `phsentry/phsentry.go`

This is the entry point phlogger hooks will call. Append to the bottom of `phsentry/phsentry.go`:

```go
// ReceiveLog forwards a log event to Sentry based on the level.
// This is called by log hook subscribers — do not call directly.
// level: "debug" | "info" | "warn" | "error" | "fatal"
// message: formatted log string
func ReceiveLog(level, message string) {
	if sentryClient == nil {
		return
	}
	switch level {
	case "fatal", "error":
		hub := sentry.NewHub(sentryClient, sentry.NewScope())
		hub.WithScope(func(scope *sentry.Scope) {
			scope.SetLevel(sentryLevelFor(level))
			addDefaultBreadcrumb(scope, level, message)
			hub.CaptureMessage(message)
		})
	case "warn":
		hub := sentry.NewHub(sentryClient, sentry.NewScope())
		hub.WithScope(func(scope *sentry.Scope) {
			scope.SetLevel(sentry.LevelWarning)
			addDefaultBreadcrumb(scope, level, message)
			hub.CaptureMessage(message)
		})
	case "info":
		hub := sentry.NewHub(sentryClient, sentry.NewScope())
		hub.WithScope(func(scope *sentry.Scope) {
			scope.SetLevel(sentry.LevelInfo)
			addDefaultBreadcrumb(scope, level, message)
			hub.CaptureMessage(message)
		})
	}
}

// FlushSentry waits for buffered Sentry events to be sent, up to timeout.
// Call this before process exit (e.g. in shutdown handler) to avoid losing events.
func FlushSentry(timeout time.Duration) {
	if sentryClient == nil {
		return
	}
	sentryClient.Flush(timeout)
}

// sentryLevelFor maps log level strings to sentry.Level.
func sentryLevelFor(level string) sentry.Level {
	switch level {
	case "fatal":
		return sentry.LevelFatal
	case "error":
		return sentry.LevelError
	case "warn":
		return sentry.LevelWarning
	case "info":
		return sentry.LevelInfo
	default:
		return sentry.LevelDebug
	}
}

// addDefaultBreadcrumb adds service context to a Sentry scope.
func addDefaultBreadcrumb(scope *sentry.Scope, level, message string) {
	if sentryBreadcrumbData == nil {
		return
	}
	scope.AddBreadcrumb(&sentry.Breadcrumb{
		Type:     "default",
		Category: level,
		Message:  message,
		Data:     GetSentryDataMap(),
	}, 10)
}
```

> **Note:** `time` import must be added to `phsentry/phsentry.go` if not already present.

### Step 6: Add `FlushSentry` to root `sentry.go`

Append to `sentry.go`:

```go
// FlushSentry waits up to timeout for buffered Sentry events to drain.
// Call before process shutdown to avoid losing queued errors.
func FlushSentry(timeout time.Duration) {
	phsentry.FlushSentry(timeout)
}
```

### Step 7: Run tests + build

```bash
go build ./...
go test ./... -race
```

Expected: clean.

### Step 8: Commit

```bash
git add phlogger/logforward_config.go phlogger/logforward_config_test.go \
        phsentry/phsentry.go sentry.go
git commit -m "feat(phsentry): add ReceiveLog(), FlushSentry(), sentryLevelFor() + LogForwardConfig in phlogger"
```

---

## Task 7: Wire `ConfigureLogForwarding` in Root Package

**Files:**
- Modify: `logger.go` (add `ConfigureLogForwarding`)
- Create: `logger_forward_test.go`

### Step 1: Write failing test

Create `logger_forward_test.go`:

```go
package paycloudhelper

import (
	"testing"
	"time"
	"bitbucket.org/paycloudid/paycloudhelper/phlogger"
)

func TestConfigureLogForwarding_RegistersHooksForEnabledLevels(t *testing.T) {
	phlogger.ClearLogHooks()
	defer phlogger.ClearLogHooks()

	errorCalled := false
	warnCalled := false

	// Override the hook to capture calls (instead of real Sentry)
	phlogger.RegisterLogHook("error", func(level, message string) { errorCalled = true })
	phlogger.RegisterLogHook("warn", func(level, message string) { warnCalled = true })

	// Simulate what ConfigureLogForwarding does (calls fireHooks internally)
	phlogger.LogE("[test] error event")
	phlogger.LogW("[test] warn event")

	time.Sleep(10 * time.Millisecond)

	if !errorCalled {
		t.Error("error hook not called")
	}
	if !warnCalled {
		t.Error("warn hook not called")
	}
}
```

### Step 2: Run test (confirm passes — hooks exist from Task 5)

```bash
go test . -run TestConfigureLogForwarding -v
```

Expected: PASS (hook system already works).

### Step 3: Add `ConfigureLogForwarding` and `LogForwardConfigFromEnv` to root `logger.go`

Append to `logger.go`:

```go
// ConfigureLogForwarding registers Sentry forwarding hooks based on cfg.
// Call once at startup AFTER InitSentry(). Safe to call multiple times —
// each call adds hooks cumulatively; use phlogger.ClearLogHooks() if you need a reset.
//
// Example (startup):
//   pch.InitSentry(pch.SentryOptions{...})
//   pch.ConfigureLogForwarding(pch.LogForwardConfigFromEnv())
func ConfigureLogForwarding(cfg phlogger.LogForwardConfig) {
	if cfg.ForwardFatal {
		phlogger.RegisterLogHook("fatal", func(level, message string) {
			phsentry.ReceiveLog(level, message)
		})
	}
	if cfg.ForwardError {
		phlogger.RegisterLogHook("error", func(level, message string) {
			phsentry.ReceiveLog(level, message)
		})
	}
	if cfg.ForwardWarn {
		phlogger.RegisterLogHook("warn", func(level, message string) {
			phsentry.ReceiveLog(level, message)
		})
	}
	if cfg.ForwardInfo {
		phlogger.RegisterLogHook("info", func(level, message string) {
			phsentry.ReceiveLog(level, message)
		})
	}
	if cfg.ForwardDebug {
		phlogger.RegisterLogHook("debug", func(level, message string) {
			phsentry.ReceiveLog(level, message)
		})
	}
}

// LogForwardConfigFromEnv returns a LogForwardConfig loaded from environment variables.
// See phlogger.LogForwardConfigFromEnv for variable names and defaults.
func LogForwardConfigFromEnv() phlogger.LogForwardConfig {
	return phlogger.LogForwardConfigFromEnv()
}
```

> The `phsentry` import must be added to `logger.go`'s import block.

### Step 4: Run full test suite

```bash
go test ./... -race
go build ./...
go vet ./...
```

Expected: all clean.

### Step 5: Commit

```bash
git add logger.go logger_forward_test.go
git commit -m "feat: add ConfigureLogForwarding() + LogForwardConfigFromEnv() to root package"
```

---

## Task 8: phsentry Production Hardening

**Files:**
- Modify: `phsentry/phsentry.go`

This task improves `phsentry` for production use without changing any existing function signatures.

### Changes to implement

1. **Add deduplication fingerprint** — Sentry deduplicates by fingerprint; set per-message fingerprint to avoid noise storms
2. **Add `WithContext` helper** — send errors with `context.Context` for request tracing
3. **Add `SentryEnabled()` check function** — consumers can guard expensive error prep
4. **Fix: `os.Getenv` in `InitSentry`** — currently uses `os.Getenv("APP_NAME")` directly; should use `phhelper.GetAppName()` to stay consistent with library conventions

### Step 1: Write tests for new helpers

Append to a new file `phsentry/phsentry_hardening_test.go`:

```go
package phsentry

import (
	"testing"
)

func TestSentryEnabled_ReturnsFalseWhenNotInitialized(t *testing.T) {
	// Reset state
	sentryClient = nil
	if SentryEnabled() {
		t.Fatal("SentryEnabled() should return false when client is nil")
	}
}

func TestGetSentryDataMap_ReturnsNilWhenNoData(t *testing.T) {
	sentryBreadcrumbData = nil
	m := GetSentryDataMap()
	if m != nil {
		t.Fatalf("expected nil map when data not set, got %v", m)
	}
}

func TestNewSentryData_HandlesNilInput(t *testing.T) {
	// Should not panic
	NewSentryData(nil)
}
```

### Step 2: Run tests to confirm they fail/pass baseline

```bash
go test ./phsentry/... -v
```

Expected: `TestSentryEnabled` fails (function doesn't exist), others pass.

### Step 3: Implement hardening additions in `phsentry/phsentry.go`

Append to `phsentry/phsentry.go`:

```go
import (
	// add to existing imports:
	"bitbucket.org/paycloudid/paycloudhelper/phhelper"
	"context"
	"time"
)

// SentryEnabled returns true if a Sentry client has been initialized.
// Use this to guard expensive error construction before calling SendSentryError.
func SentryEnabled() bool {
	return sentryClient != nil
}

// SendSentryErrorWithContext sends an error to Sentry with request context.
// ctx may carry a sentry Hub (set by middleware); falls back to global client.
func SendSentryErrorWithContext(ctx context.Context, err error, args ...string) {
	if err == nil || sentryClient == nil {
		return
	}
	hub := sentry.GetHubFromContext(ctx)
	if hub == nil {
		hub = sentry.NewHub(sentryClient, sentry.NewScope())
	}
	hub.WithScope(func(scope *sentry.Scope) {
		scope.SetLevel(sentry.LevelError)
		addDefaultBreadcrumb(scope, "error", err.Error())
		hub.CaptureException(err)
	})
}
```

Also, fix `InitSentry` to use `phhelper.GetAppName()` / `phhelper.GetAppEnv()` instead of `os.Getenv` directly — this makes it consistent with library conventions:

Find in `InitSentry`:
```go
sd := &SentryData{
    Service:  os.Getenv("APP_NAME"),
    Module:   os.Getenv("APP_ENV"),
```

Replace with:
```go
sd := &SentryData{
    Service:  phhelper.GetAppName(),
    Module:   phhelper.GetAppEnv(),
```

### Step 4: Run tests + build

```bash
go test ./phsentry/... -v
go build ./...
```

Expected: all PASS.

### Step 5: Commit

```bash
git add phsentry/phsentry.go phsentry/phsentry_hardening_test.go
git commit -m "feat(phsentry): add SentryEnabled(), SendSentryErrorWithContext(), FlushSentry(); fix InitSentry to use phhelper for app identity"
```

---

## Task 9: Expose `SentryEnabled` + `SendSentryErrorWithContext` in Root Package

**Files:**
- Modify: `sentry.go`

### Step 1: Append to `sentry.go`

```go
// SentryEnabled returns true if Sentry has been initialized.
func SentryEnabled() bool {
	return phsentry.SentryEnabled()
}

// SendSentryErrorWithContext sends an error to Sentry carrying request context.
func SendSentryErrorWithContext(ctx context.Context, err error, args ...string) {
	phsentry.SendSentryErrorWithContext(ctx, err, args...)
}
```

Add `"context"` to the import block in `sentry.go`.

### Step 2: Build check

```bash
go build ./...
go vet ./...
```

### Step 3: Commit

```bash
git add sentry.go
git commit -m "feat: expose SentryEnabled() and SendSentryErrorWithContext() in root package"
```

---

## Task 10: Version Bump to v1.7.0

**Files:**
- Modify: `go.mod` (no content change needed — version lives in git tag)
- Modify: `AGENTS.md` — update Quick Reference known retractions if any

### Step 1: Run the full test suite one final time

```bash
go test ./... -race -count=1
go build ./...
go vet ./...
```

Expected: all green, no races.

### Step 2: Confirm no breaking changes

Check that all existing exported symbols still exist:

```bash
grep -rn "^func \|^var \|^type " --include="*.go" . \
  | grep -v "_test.go" \
  | grep -v "^Binary" \
  | sort > /tmp/api-v1.7.txt
# Visually confirm no old functions are missing vs v1.6.6 baseline
```

### Step 3: Final commit + tag

```bash
git add -A
git commit -m "chore: finalize v1.7.0 — rate-limited logging, Sentry log forwarding, phsentry hardening"
git tag v1.7.0
git push origin feat/phlogger-ratelimit-sentry-v1.7.0
git push origin v1.7.0
```

### Step 4: Consumer service update (for each service using this library)

```bash
# In each consumer service (e.g. paycloud-be-settlement-manager):
go get bitbucket.org/paycloudid/paycloudhelper@v1.7.0
go mod tidy
go build ./...
```

### Step 5: Optional — enable log forwarding in a consumer service

Add to the consumer's `main()` after `InitSentry`:

```go
// Forward Fatal logs to Sentry by default; tune via env vars
pch.ConfigureLogForwarding(pch.LogForwardConfigFromEnv())
```

Or explicitly:

```go
pch.ConfigureLogForwarding(pch.LogForwardConfig{
    ForwardFatal: true,
    ForwardError: false, // keep error volume low unless debugging
})
```

---

## Summary: New Public API in v1.7.0

| Symbol | Package | Description |
|--------|---------|-------------|
| `LogIRated(key, window, format, args...)` | root + phlogger | Rate-limited Info log |
| `LogERated(key, window, format, args...)` | root + phlogger | Rate-limited Error log |
| `LogWRated(key, window, format, args...)` | root + phlogger | Rate-limited Warn log |
| `LogDRated(key, window, format, args...)` | root + phlogger | Rate-limited Debug log |
| `RegisterLogHook(level, hook)` | phlogger | Register log forwarding hook |
| `ClearLogHooks()` | phlogger | Clear all hooks (testing) |
| `LogForwardConfig` | phlogger | Config struct for forwarding |
| `LogForwardConfigFromEnv()` | root + phlogger | Load forward config from env |
| `ConfigureLogForwarding(cfg)` | root | Wire Sentry hooks per config |
| `ReceiveLog(level, message)` | phsentry | Sentry log receiver |
| `FlushSentry(timeout)` | root + phsentry | Flush buffered Sentry events |
| `SentryEnabled()` | root + phsentry | Check if Sentry initialized |
| `SendSentryErrorWithContext(ctx, err)` | root + phsentry | Error with request context |

**Zero breaking changes.** All existing callers continue to work without modification.

---

## Troubleshooting

**Q: Tests fail with "import cycle"**
Check that `phlogger` does NOT import `phsentry`. The dependency must be one-way: root `logger.go` imports both and wires them together.

**Q: Hook not firing for `LogF`**
`LogF` fires hooks **before** `Log.Fatalf` to ensure Sentry receives the event. If the hook is slow, the process exits before it completes. Use `FlushSentry(2*time.Second)` in your shutdown handler for critical use cases.

**Q: Rate limiter not suppressing**
Verify the `key` string is identical across calls. Keys are case-sensitive exact strings.

**Q: `ReceiveLog` silently does nothing**
Sentry client is nil (not initialized). Call `SentryEnabled()` to check. `InitSentry` must be called before `ConfigureLogForwarding`.
