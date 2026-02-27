package phlogger

import (
	"time"

	"bitbucket.org/paycloudid/paycloudhelper/phhelper"
	"github.com/kataras/golog"
	"github.com/kataras/pio"
)

var (
	Log  = golog.New()
	LogD = Log.Debugf
	LogI = Log.Infof
	LogW = Log.Warnf
	LogE = Log.Errorf
	LogF = Log.Fatalf
	Logf = Log.Logf
)

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
