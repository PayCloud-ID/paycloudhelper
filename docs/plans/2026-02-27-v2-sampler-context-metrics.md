# v1.7.0 Update: Sampler Rate Limiting + Context Logger + Metrics Hooks

> **Status:** ✅ COMPLETED — all tasks implemented and tested.

**Goal:** Replace simple 50ms time-window rate limiting with production-grade Initial/Thereafter sampler, add child-context logger, metrics hooks, and custom KeyedLimiter — all backward-compatible (v1.7.0 MINOR bump).

**Architecture:**
- `phlogger/sampler.go` — Initial/Thereafter per-key log sampler using `sync.Map` + `sync/atomic`. Environment-aware defaults via `phhelper.GetAppEnv()`.
- `phlogger/context.go` — `LogContext` child logger attaching request-scoped fields; avoids passing raw loggers.
- `phlogger/metrics.go` — `MetricsHook` callback pattern (no prometheus dependency in library); consumer wires their own counters.
- `phlogger/keyed_limiter.go` — Per-key token bucket via `golang.org/x/time/rate` for precise rate control.
- `phlogger/phlogger.go` — Default `LogI/E/W/D/F` use sampler; `LogIRated` uses sampler with custom key; `LogIRatedW` uses time-window (backward compat).

**Tech Stack:** Go 1.23, `kataras/golog`, `sync/atomic`, `golang.org/x/time/rate` (new direct dep), `phhelper.GetAppEnv()`

---

## Environment-Aware Defaults

| Environment | Initial | Thereafter | Period | Behavior |
|-------------|---------|------------|--------|----------|
| `production` / `prod` | 5 | 50 | 1s | Log first 5/sec, then every 50th |
| `staging` / `stg` | 10 | 10 | 1s | Log first 10/sec, then every 10th |
| `develop` / `dev` / `""` | 0 | 0 | — | No sampling (all logs pass through) |

---

## Task 1: Sampler Config + Core Implementation

**Files:**
- Create: `phlogger/sampler.go`
- Create: `phlogger/sampler_test.go`

### sampler.go

```go
package phlogger

import (
    "sync"
    "sync/atomic"
    "time"

    "bitbucket.org/paycloudid/paycloudhelper/phhelper"
)

// SamplerConfig controls log sampling behavior.
// Initial is the number of log lines per key allowed in each Period.
// After Initial is exhausted, only every Thereafter-th log is emitted.
// If Initial <= 0, sampling is disabled (all logs pass through).
type SamplerConfig struct {
    Initial    int           // log first N per period per key (0 = disabled)
    Thereafter int           // after Initial, log every Nth (0 = drop all after initial)
    Period     time.Duration // sampling window (default: 1s)
}

// SamplerConfigForEnv returns production-tuned defaults based on environment string.
func SamplerConfigForEnv(env string) SamplerConfig {
    switch env {
    case "production", "prod":
        return SamplerConfig{Initial: 5, Thereafter: 50, Period: time.Second}
    case "staging", "stg":
        return SamplerConfig{Initial: 10, Thereafter: 10, Period: time.Second}
    default:
        return SamplerConfig{} // disabled
    }
}

// samplerEntry tracks per-key counter state.
type samplerEntry struct {
    count     atomic.Int64
    resetNano atomic.Int64 // unix nano of last period reset
}

// sampler implements per-key Initial/Thereafter log sampling.
type sampler struct {
    config SamplerConfig
    entries sync.Map // key → *samplerEntry
}

// globalSampler is initialized from APP_ENV during InitializeSampler().
var globalSampler = &sampler{}

// InitializeSampler sets the global sampler config. Called automatically by InitializeLogger.
// Safe to call multiple times — last call wins.
func InitializeSampler(cfg SamplerConfig) {
    if cfg.Period <= 0 {
        cfg.Period = time.Second
    }
    globalSampler = &sampler{config: cfg}
}

// check returns true if the log line for key should be emitted.
// Returns (allowed, suppressed) where suppressed is the count dropped since last emit.
func (s *sampler) check(key string) (allowed bool, suppressed int64) {
    if s.config.Initial <= 0 {
        return true, 0 // sampling disabled
    }

    now := time.Now().UnixNano()
    actual, _ := s.entries.LoadOrStore(key, &samplerEntry{})
    entry := actual.(*samplerEntry)

    // Check if we need to reset (new period).
    lastReset := entry.resetNano.Load()
    if time.Duration(now-lastReset) >= s.config.Period {
        entry.resetNano.Store(now)
        entry.count.Store(1)
        return true, 0
    }

    n := entry.count.Add(1)
    if int(n) <= s.config.Initial {
        return true, 0
    }
    if s.config.Thereafter <= 0 {
        return false, 0
    }
    over := int(n) - s.config.Initial
    if over%s.config.Thereafter == 0 {
        return true, int64(s.config.Thereafter - 1)
    }
    return false, 0
}
```

### sampler_test.go

Tests: disabled config always allows, initial burst, thereafter sampling, independent keys, env config defaults.

---

## Task 2: Wire Sampler into phlogger.go

**Files:**
- Modify: `phlogger/phlogger.go`
- Modify: `phlogger/ratelimit.go` (keep for LogIRatedW backward compat)

Replace `globalRateLimiter.check(format, defaultWindow)` with `globalSampler.check(format)` in all default LogI/E/W/D/F. Keep `rateLimiter` for `LogIRatedW` (explicit time window). Update `LogIRated` to use sampler.

Wire `InitializeSampler(SamplerConfigForEnv(phhelper.GetAppEnv()))` into `InitializeLogger()`.

---

## Task 3: Context Logger

**Files:**
- Create: `phlogger/context.go`
- Create: `phlogger/context_test.go`

### context.go

```go
// LogContext holds request-scoped fields prepended to every log message.
type LogContext struct { prefix string }

func NewLogContext(fields ...string) *LogContext { ... }
func (lc *LogContext) LogI(format string, args ...interface{}) { ... }
func (lc *LogContext) LogE/LogW/LogD/LogF(format, args)
```

---

## Task 4: Metrics Hooks

**Files:**
- Create: `phlogger/metrics.go`
- Create: `phlogger/metrics_test.go`

No prometheus dependency. Callback pattern:
```go
type MetricsHook func(event string, count int64)
func RegisterMetricsHook(hook MetricsHook)
func IncrementMetric(event string)
func IncrementMetricBy(event string, n int64)
```

---

## Task 5: KeyedLimiter (x/time/rate)

**Files:**
- Create: `phlogger/keyed_limiter.go`
- Create: `phlogger/keyed_limiter_test.go`
- Modify: `go.mod` (add `golang.org/x/time`)

```go
type KeyedLimiter struct { ... }
func NewKeyedLimiter(r float64, burst int) *KeyedLimiter
func (kl *KeyedLimiter) Allow(key string) bool
```

---

## Task 6: Root Exports + README + Final

- Update `logger.go` with exports for `NewLogContext`, `InitializeSampler`, `SamplerConfig`, etc.
- Update `README.md` with new API sections.
- `go test ./... -race -count=1`
- Move `v1.7.0` tag.
