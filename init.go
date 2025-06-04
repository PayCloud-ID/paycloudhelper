package paycloudhelper

import "os"

func init() {
	AddValidatorLibs()
	InitializeLogger()
	InitializeApp()
}

var globAppName string
var globAppEnv string

func InitializeApp() {
	if appName := os.Getenv("APP_NAME"); appName != "" {
		SetAppName(appName)
	}
	if appEnv := os.Getenv("APP_ENV"); appEnv != "" {
		SetAppName(appEnv)
	}
}

func GetAppName() string {
	return globAppName
}

func GetAppEnv() string {
	return globAppEnv
}

func SetAppName(v string) {
	globAppName = v
}

func SetAppEnv(v string) {
	globAppEnv = v
}
