package phlogger

import (
	"fmt"
	"time"

	"bitbucket.org/paycloudid/paycloudhelper/phhelper"
	"github.com/kataras/golog"
	"github.com/kataras/pio"
)

var (
	Log  = golog.New()
	Logf = Log.Logf
)

// ── Internal emit helpers ────────────────────────────────────────────────────
// These bypass rate limiting and are the single point that calls golog + hooks.

func emitD(format string, args ...interface{}) {
	Log.Debugf(format, args...)
	fireHooks("debug", fmt.Sprintf(format, args...))
}

func emitI(format string, args ...interface{}) {
	Log.Infof(format, args...)
	fireHooks("info", fmt.Sprintf(format, args...))
}

func emitW(format string, args ...interface{}) {
	Log.Warnf(format, args...)
	fireHooks("warn", fmt.Sprintf(format, args...))
}

func emitE(format string, args ...interface{}) {
	Log.Errorf(format, args...)
	fireHooks("error", fmt.Sprintf(format, args...))
}

func emitF(format string, args ...interface{}) {
	// Hooks fire BEFORE Fatalf because Fatalf calls os.Exit.
	fireHooks("fatal", fmt.Sprintf(format, args...))
	Log.Fatalf(format, args...)
}

// ── Public log functions (rate-limited by default) ───────────────────────────
// Each public function rate-limits using the format string as key and
// defaultWindow (50ms). Duplicate lines within 50ms are suppressed silently.

// LogD logs at Debug level.
func LogD(format string, args ...interface{}) {
	allowed, suppressed := globalRateLimiter.check(format, defaultWindow)
	if !allowed {
		return
	}
	if suppressed > 0 {
		format += " [+%d suppressed]"
		args = append(args, suppressed)
	}
	emitD(format, args...)
}

// LogI logs at Info level.
func LogI(format string, args ...interface{}) {
	allowed, suppressed := globalRateLimiter.check(format, defaultWindow)
	if !allowed {
		return
	}
	if suppressed > 0 {
		format += " [+%d suppressed]"
		args = append(args, suppressed)
	}
	emitI(format, args...)
}

// LogW logs at Warning level.
func LogW(format string, args ...interface{}) {
	allowed, suppressed := globalRateLimiter.check(format, defaultWindow)
	if !allowed {
		return
	}
	if suppressed > 0 {
		format += " [+%d suppressed]"
		args = append(args, suppressed)
	}
	emitW(format, args...)
}

// LogE logs at Error level.
func LogE(format string, args ...interface{}) {
	allowed, suppressed := globalRateLimiter.check(format, defaultWindow)
	if !allowed {
		return
	}
	if suppressed > 0 {
		format += " [+%d suppressed]"
		args = append(args, suppressed)
	}
	emitE(format, args...)
}

// LogF logs at Fatal level. Rate-limited by format string key.
// Hooks fire synchronously before os.Exit.
func LogF(format string, args ...interface{}) {
	allowed, suppressed := globalRateLimiter.check(format, defaultWindow)
	if !allowed {
		return
	}
	if suppressed > 0 {
		format += " [+%d suppressed]"
		args = append(args, suppressed)
	}
	emitF(format, args...)
}

var GinLevel golog.Level = 6

func InitializeLogger() {
	Log.SetTimeFormat("2006-01-02 15:04:05.000")
	Log.SetLevel("info")

	golog.Levels[GinLevel] = &golog.LevelMetadata{
		Name:             "gin",
		AlternativeNames: []string{"http-server"},
		Title:            "[GIN]",
		ColorCode:        pio.Green,
	}
}

func LogSetLevel(levelName string) {
	Log.SetLevel(levelName)
}

// LogJ logs arg as compact JSON.
func LogJ(arg interface{}) {
	data := phhelper.ToJson(arg)
	LogI("%s", data)
}

// LogJI logs arg as indented JSON.
func LogJI(arg interface{}) {
	data := phhelper.ToJsonIndent(arg)
	LogI("%s", data)
}

// LogErr logs an error value.
func LogErr(err error) {
	Log.Error(err)
}

// ── Rate-limited variants with custom key ─────────────────────────────────────
// Use these when the format string is not a stable identifier (e.g. it contains
// a variable value). key should be a stable string like "cache.miss".

// LogIRated logs at Info level with rate limiting using key and the default 50ms window.
func LogIRated(key string, format string, args ...interface{}) {
	logRated(key, defaultWindow, emitI, format, args...)
}

// LogERated logs at Error level with rate limiting using key and the default 50ms window.
func LogERated(key string, format string, args ...interface{}) {
	logRated(key, defaultWindow, emitE, format, args...)
}

// LogWRated logs at Warning level with rate limiting using key and the default 50ms window.
func LogWRated(key string, format string, args ...interface{}) {
	logRated(key, defaultWindow, emitW, format, args...)
}

// LogDRated logs at Debug level with rate limiting using key and the default 50ms window.
func LogDRated(key string, format string, args ...interface{}) {
	logRated(key, defaultWindow, emitD, format, args...)
}

// LogIRatedW logs at Info level with rate limiting using key and an explicit window.
func LogIRatedW(key string, window time.Duration, format string, args ...interface{}) {
	logRated(key, window, emitI, format, args...)
}

// LogERatedW logs at Error level with rate limiting using key and an explicit window.
func LogERatedW(key string, window time.Duration, format string, args ...interface{}) {
	logRated(key, window, emitE, format, args...)
}

// LogWRatedW logs at Warning level with rate limiting using key and an explicit window.
func LogWRatedW(key string, window time.Duration, format string, args ...interface{}) {
	logRated(key, window, emitW, format, args...)
}

// LogDRatedW logs at Debug level with rate limiting using key and an explicit window.
func LogDRatedW(key string, window time.Duration, format string, args ...interface{}) {
	logRated(key, window, emitD, format, args...)
}

// logRated is the shared rate-limiting + emit logic for all *Rated variants.
func logRated(key string, window time.Duration, emit func(string, ...interface{}), format string, args ...interface{}) {
	allowed, suppressed := globalRateLimiter.check(key, window)
	if !allowed {
		return
	}
	if suppressed > 0 {
		format += " [+%d suppressed]"
		args = append(args, suppressed)
	}
	emit(format, args...)
}
