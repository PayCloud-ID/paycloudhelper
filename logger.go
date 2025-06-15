package paycloudhelper

import (
	"bitbucket.org/paycloudid/paycloudhelper/phlogger"
	"github.com/kataras/golog"
)

var (
	Log  = phlogger.Log
	LogD = Log.Debugf
	LogI = Log.Infof
	LogW = Log.Warnf
	LogE = Log.Errorf
	LogF = Log.Fatalf
	Logf = Log.Logf
)

var GinLevel golog.Level = phlogger.GinLevel

func InitializeLogger() {
	phlogger.InitializeLogger()
}

func LogSetLevel(levelName string) {
	phlogger.LogSetLevel(levelName)
}

// LogJ JSON
func LogJ(arg interface{}) {
	phlogger.LogJ(arg)
}

// LogJI as JSON with Indent
func LogJI(arg interface{}) {
	phlogger.LogJI(arg)
}

// LogErr Error
func LogErr(err error) {
	phlogger.LogErr(err)
}
