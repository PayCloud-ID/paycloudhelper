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

// globalRateLimiter is the singleton used by all log functions and LogIRated variants.
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
