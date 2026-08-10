package paycloudhelper

import (
	"os"
	"testing"

	"github.com/PayCloud-ID/paycloudhelper/pcauth"
	"golang.org/x/crypto/bcrypt"
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

func TestInitializeApp_ConfiguresBcryptCost(t *testing.T) {
	t.Setenv("BCRYPT_COST", "6")
	t.Cleanup(InitializeApp)

	InitializeApp()
	assertGeneratedCost(t, 6)

	// Consumer tests and staged rotations change BCRYPT_COST after package init.
	t.Setenv("BCRYPT_COST", "7")
	assertGeneratedCost(t, 7)
}

func assertGeneratedCost(t *testing.T, want int) {
	t.Helper()
	_, hash, err := pcauth.GenerateHashAndSalt("hunter2")
	if err != nil {
		t.Fatalf("GenerateHashAndSalt: %v", err)
	}
	cost, err := bcrypt.Cost([]byte(hash))
	if err != nil {
		t.Fatalf("bcrypt.Cost: %v", err)
	}
	if cost != want {
		t.Fatalf("bcrypt cost=%d, want %d", cost, want)
	}
}
