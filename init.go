package paycloudhelper

import (
	"bitbucket.org/paycloudid/paycloudhelper/phhelper"
	"log"
	"os"

	"github.com/joho/godotenv"
)

func init() {
	AddValidatorLibs()
	InitializeLogger()
	InitializeApp()
}

func InitializeApp() {
	if err := godotenv.Load(); err != nil {
		log.Println("PCHELPER ERR godotenv.Load", err)
	}

	if appName := os.Getenv("APP_NAME"); appName != "" {
		phhelper.SetAppName(appName)
	}
	if appEnv := os.Getenv("APP_ENV"); appEnv != "" {
		phhelper.SetAppEnv(appEnv)
	}
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
