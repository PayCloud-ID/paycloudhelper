package phlogger

import "sync"

// MetricsHook is a callback invoked when a metric event is recorded.
// Consumers wire their own counter backends (prometheus, statsd, etc.).
//
// Parameters:
//   - event: stable identifier string (e.g. "cache.miss", "db.timeout")
//   - count: number of occurrences to add (usually 1)
//
// Example with prometheus:
//
//	RegisterMetricsHook(func(event string, count int64) {
//	    myCounter.WithLabelValues(event).Add(float64(count))
//	})
type MetricsHook func(event string, count int64)

var (
	metricsHookMu sync.RWMutex
	metricsHook   MetricsHook
)

// RegisterMetricsHook sets the global metrics callback. Call once at startup.
// Passing nil disables metrics forwarding.
//
// Only one hook is active at a time — subsequent calls replace the previous hook.
func RegisterMetricsHook(hook MetricsHook) {
	metricsHookMu.Lock()
	defer metricsHookMu.Unlock()
	metricsHook = hook
}

// IncrementMetric records one occurrence of a named event via the registered hook.
// If no hook is registered, the call is a no-op (zero overhead).
//
// Use this instead of logging for events that happen thousands of times per second:
//
//	phlogger.IncrementMetric("cache.miss")
func IncrementMetric(event string) {
	IncrementMetricBy(event, 1)
}

// IncrementMetricBy records `n` occurrences of a named event via the registered hook.
// If no hook is registered, the call is a no-op.
func IncrementMetricBy(event string, n int64) {
	metricsHookMu.RLock()
	h := metricsHook
	metricsHookMu.RUnlock()
	if h != nil {
		h(event, n)
	}
}
