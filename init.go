package paycloudhelper

import (
	"os"

	"bitbucket.org/paycloudid/paycloudhelper/phhelper"

	"github.com/joho/godotenv"
)

func init() {
	AddValidatorLibs()
	InitializeLogger()
	InitializeApp()
}

func InitializeApp() {
	if err := godotenv.Load(); err != nil {
		LogI("PCHELPER ERR godotenv.Load err_message=%s", err)
	}

	if appName := os.Getenv("APP_NAME"); appName != "" {
		phhelper.SetAppName(appName)
	}
	if appEnv := os.Getenv("APP_ENV"); appEnv != "" {
		phhelper.SetAppEnv(appEnv)
	}

	// Validate configuration and log warnings
	LogConfigurationWarnings()
}

func GetAppName() string {
	return phhelper.GetAppName()
}

func GetAppEnv() string {
	return phhelper.GetAppEnv()
}

func SetAppName(v string) {
	phhelper.SetAppName(v)
}

func SetAppEnv(v string) {
	phhelper.SetAppEnv(v)
}
