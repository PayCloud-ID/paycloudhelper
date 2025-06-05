package paycloudhelper

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

func init() {
	AddValidatorLibs()
	InitializeLogger()
	InitializeApp()
}

var globAppName string
var globAppEnv string

func InitializeApp() {
	if err := godotenv.Load(); err != nil {
		log.Println("PCHELPER ERR godotenv.Load", err)
	}

	if appName := os.Getenv("APP_NAME"); appName != "" {
		SetAppName(appName)
	}
	if appEnv := os.Getenv("APP_ENV"); appEnv != "" {
		SetAppEnv(appEnv)
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
