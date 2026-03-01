package paycloudhelper

import (
	"os"
	"testing"
)

func TestSetAppName_GetAppName(t *testing.T) {
	old := GetAppName()
	defer SetAppName(old)

	SetAppName("test-app-name")
	if got := GetAppName(); got != "test-app-name" {
		t.Errorf("GetAppName() = %q, want test-app-name", got)
	}
}

func TestSetAppEnv_GetAppEnv(t *testing.T) {
	old := GetAppEnv()
	defer SetAppEnv(old)

	SetAppEnv("staging")
	if got := GetAppEnv(); got != "staging" {
		t.Errorf("GetAppEnv() = %q, want staging", got)
	}
}

func TestInitializeApp_LoadsEnv(t *testing.T) {
	oldName := os.Getenv("APP_NAME")
	oldEnv := os.Getenv("APP_ENV")
	defer func() {
		os.Setenv("APP_NAME", oldName)
		os.Setenv("APP_ENV", oldEnv)
	}()

	os.Setenv("APP_NAME", "init-test-app")
	os.Setenv("APP_ENV", "develop")
	InitializeApp()
	if got := GetAppName(); got != "init-test-app" {
		t.Errorf("after InitializeApp GetAppName() = %q, want init-test-app", got)
	}
	if got := GetAppEnv(); got != "develop" {
		t.Errorf("after InitializeApp GetAppEnv() = %q, want develop", got)
	}
}
