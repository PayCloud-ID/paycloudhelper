package phlogger

import (
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

// LogJ JSON
func LogJ(arg interface{}) {
	data := phhelper.ToJson(arg)
	LogD("%s", data)
}

// LogJI as JSON with Indent
func LogJI(arg interface{}) {
	data := phhelper.ToJsonIndent(arg)
	LogD("%s", data)
}

// LogErr Error
func LogErr(err error) {
	Log.Error(err)
}
