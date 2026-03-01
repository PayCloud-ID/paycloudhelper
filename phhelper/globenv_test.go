package phhelper

import (
	"testing"
)

func TestGetAppName_SetAppName(t *testing.T) {
	old := GetAppName()
	defer SetAppName(old)

	SetAppName("my-service")
	if got := GetAppName(); got != "my-service" {
		t.Errorf("GetAppName() = %q, want my-service", got)
	}
}

func TestGetAppEnv_SetAppEnv(t *testing.T) {
	old := GetAppEnv()
	defer SetAppEnv(old)

	SetAppEnv("production")
	if got := GetAppEnv(); got != "production" {
		t.Errorf("GetAppEnv() = %q, want production", got)
	}
}

func TestSetAppName_Empty(t *testing.T) {
	SetAppName("")
	if got := GetAppName(); got != "" {
		t.Errorf("GetAppName() after SetAppName(\"\") = %q, want empty", got)
	}
}

func TestSetAppEnv_Empty(t *testing.T) {
	SetAppEnv("")
	if got := GetAppEnv(); got != "" {
		t.Errorf("GetAppEnv() after SetAppEnv(\"\") = %q, want empty", got)
	}
}
