package phlogger

import (
	"sync"
	"sync/atomic"
	"time"

	"bitbucket.org/paycloudid/paycloudhelper/phhelper"
)

// SamplerConfig controls log sampling behavior per key per period.
//
// Initial is the number of log lines allowed per key in each Period.
// After Initial is exhausted, only every Thereafter-th message is emitted.
// If Initial <= 0, sampling is disabled and all logs pass through.
//
// Environment defaults:
//   - production: Initial=5, Thereafter=50, Period=1s
//   - staging:    Initial=10, Thereafter=10, Period=1s
//   - develop:    disabled (all logs pass through)
type SamplerConfig struct {
	Initial    int           // log first N per period per key (0 = disabled)
	Thereafter int           // after Initial, log every Nth (0 = drop all after initial)
	Period     time.Duration // sampling window (default: 1s)
}

// SamplerConfigForEnv returns production-tuned defaults based on environment string.
// Uses the same env values as phhelper.GetAppEnv(): "production", "prod", "staging", "stg".
func SamplerConfigForEnv(env string) SamplerConfig {
	switch env {
	case "production", "prod":
		return SamplerConfig{Initial: 5, Thereafter: 50, Period: time.Second}
	case "staging", "stg":
		return SamplerConfig{Initial: 10, Thereafter: 10, Period: time.Second}
	case "develop", "developement", "dev":
		return SamplerConfig{Initial: 20, Thereafter: 20, Period: time.Second}
	default:
		return SamplerConfig{} // disabled — all logs pass through
	}
}

// SamplerConfigFromAppEnv returns a SamplerConfig based on the current APP_ENV.
// Convenience wrapper: reads phhelper.GetAppEnv() and calls SamplerConfigForEnv.
func SamplerConfigFromAppEnv() SamplerConfig {
	return SamplerConfigForEnv(phhelper.GetAppEnv())
}

// samplerEntry tracks per-key counter state within a period.
type samplerEntry struct {
	count     atomic.Int64
	resetNano atomic.Int64 // unix nano of last period reset
}

// sampler implements per-key Initial/Thereafter log sampling.
// Thread-safe via sync.Map and atomic operations.
type sampler struct {
	config  SamplerConfig
	entries sync.Map // key string → *samplerEntry
}

// globalSampler is initialized from APP_ENV during InitializeLogger().
// Starts disabled so pre-init log calls always pass through.
var globalSampler = &sampler{}

// InitializeSampler sets the global sampler config.
// Called automatically by InitializeLogger with env-aware defaults.
// Safe to call multiple times — last call wins.
func InitializeSampler(cfg SamplerConfig) {
	if cfg.Period <= 0 && cfg.Initial > 0 {
		cfg.Period = time.Second
	}
	globalSampler = &sampler{config: cfg}
}

// check returns true if the log line for key should be emitted.
//
//   - Sampling disabled (Initial <= 0): always returns (true, 0).
//   - Within Initial burst: returns (true, 0).
//   - After Initial, on every Thereafter-th call: returns (true, suppressed).
//   - Otherwise: returns (false, 0).
func (s *sampler) check(key string) (allowed bool, suppressed int64) {
	if s.config.Initial <= 0 {
		return true, 0
	}

	now := time.Now().UnixNano()
	actual, _ := s.entries.LoadOrStore(key, &samplerEntry{})
	entry := actual.(*samplerEntry)

	// Reset counter if period has elapsed.
	lastReset := entry.resetNano.Load()
	if time.Duration(now-lastReset) >= s.config.Period {
		entry.resetNano.Store(now)
		entry.count.Store(1)
		return true, 0
	}

	n := entry.count.Add(1)

	// Within initial burst — allow.
	if int(n) <= s.config.Initial {
		return true, 0
	}

	// After initial: allow every Thereafter-th, suppress the rest.
	if s.config.Thereafter <= 0 {
		return false, 0
	}
	over := int(n) - s.config.Initial
	if over%s.config.Thereafter == 0 {
		return true, int64(s.config.Thereafter - 1)
	}
	return false, 0
}
