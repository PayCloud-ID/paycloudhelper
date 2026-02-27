package phlogger

import (
	"sync"

	"golang.org/x/time/rate"
)

// KeyedLimiter provides per-key token bucket rate limiting using x/time/rate.
// Each unique key gets its own independent limiter with identical rate and burst.
//
// Use this when you need precise rate control (e.g. max N events/second per error type)
// rather than the sampling approach of the global sampler.
//
// Thread-safe: safe for concurrent use from multiple goroutines.
//
// Example:
//
//	limiter := NewKeyedLimiter(10, 1)  // 10 events/sec, burst of 1
//	if limiter.Allow("db.timeout") {
//	    phlogger.LogE("database timeout on host=%s", host)
//	}
type KeyedLimiter struct {
	limiters sync.Map // key string → *rate.Limiter
	r        rate.Limit
	burst    int
}

// NewKeyedLimiter creates a limiter allowing `r` events per second with `burst`
// capacity per key. A burst of 1 provides strict per-second limiting.
//
// Parameters:
//   - r: sustained events per second (e.g. 10.0 = ten events/sec per key)
//   - burst: maximum burst size (events allowed in a single instant)
func NewKeyedLimiter(r float64, burst int) *KeyedLimiter {
	return &KeyedLimiter{r: rate.Limit(r), burst: burst}
}

// Allow reports whether an event for the given key should be permitted.
// Returns true if within rate limit, false if the event should be dropped.
//
// Creates a new limiter for unseen keys automatically (lazy initialization).
func (kl *KeyedLimiter) Allow(key string) bool {
	actual, _ := kl.limiters.LoadOrStore(key, rate.NewLimiter(kl.r, kl.burst))
	return actual.(*rate.Limiter).Allow()
}
