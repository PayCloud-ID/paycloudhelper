package paycloudhelper

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindEnvPath_ENV_FILE(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "custom.env")
	if err := os.WriteFile(p, []byte("X=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ENV_FILE", p)
	if got := findEnvPath(); got != p {
		t.Fatalf("findEnvPath() = %q, want %q", got, p)
	}
}

func TestFindEnvPath_CwdDotEnv(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("ENV_FILE", "")
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("Y=2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := findEnvPath(); got == "" {
		t.Fatal("findEnvPath() empty, want .env in cwd")
	}
}
