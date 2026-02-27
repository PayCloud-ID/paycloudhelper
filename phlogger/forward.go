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
