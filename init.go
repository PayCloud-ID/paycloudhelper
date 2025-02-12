package paycloudhelper

import "os"

func init() {
	AddValidatorLibs()
	InitializeLogger()
	InitializeAppName()
}

var globAppName string

func InitializeAppName() {
	if appName := os.Getenv("APP_NAME"); appName != "" {
		SetAppName(appName)
	}
}

func GetAppName() string {
	return globAppName
}

func SetAppName(v string) {
	globAppName = v
}
