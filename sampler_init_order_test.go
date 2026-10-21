package paycloudhelper

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// The sampler default depends on APP_ENV, which the package init() only learns
// inside InitializeApp(). These tests run the real init() in a child process
// (APP_ENV is in the process env before the package loads) and count how many
// of a burst of identical rate-limited log lines survive.

const (
	samplerChildEnv   = "PCH_SAMPLER_CHILD"
	samplerBurstLines = 200
	samplerMarker     = "SAMPCHECK-LINE"
)

// runSamplerChild re-executes this test binary running only the named test in
// child mode and returns how many marker lines the child emitted.
func runSamplerChild(t *testing.T, testName, mode, appEnv string) int {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^"+testName+"$")
	cmd.Env = append(os.Environ(), samplerChildEnv+"="+mode, "APP_ENV="+appEnv)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("child process failed: %v\n%s", err, out)
	}
	return strings.Count(string(out), samplerMarker)
}

func emitSamplerBurst() {
	for i := 0; i < samplerBurstLines; i++ {
		LogI(samplerMarker+" %d", i)
	}
}

func TestSamplerEnabledWhenAppEnvIsInProcessEnv(t *testing.T) {
	if os.Getenv(samplerChildEnv) == "default" {
		emitSamplerBurst()
		return
	}
	n := runSamplerChild(t, "TestSamplerEnabledWhenAppEnvIsInProcessEnv", "default", "staging")
	if n == 0 {
		t.Fatalf("child emitted no marker lines; the check is not observing log output")
	}
	if n >= samplerBurstLines {
		t.Fatalf("sampler disabled: %d of %d identical lines were emitted, want fewer", n, samplerBurstLines)
	}
}

// Re-running InitializeApp (public API, may be called by services) must not
// clobber a sampler config the service chose after package init.
func TestSamplerInitRunsOnceAndKeepsExplicitConfig(t *testing.T) {
	if os.Getenv(samplerChildEnv) == "explicit" {
		InitializeSampler(SamplerConfig{}) // service opts out of sampling
		InitializeApp()                    // must not re-enable it
		emitSamplerBurst()
		return
	}
	n := runSamplerChild(t, "TestSamplerInitRunsOnceAndKeepsExplicitConfig", "explicit", "staging")
	if n != samplerBurstLines {
		t.Fatalf("explicit InitializeSampler(SamplerConfig{}) was overridden: %d of %d lines emitted, want all", n, samplerBurstLines)
	}
}

// Environments with a disabled sampler default stay unsampled.
func TestSamplerStaysDisabledForLocalEnv(t *testing.T) {
	if os.Getenv(samplerChildEnv) == "local" {
		emitSamplerBurst()
		return
	}
	n := runSamplerChild(t, "TestSamplerStaysDisabledForLocalEnv", "local", "local")
	if n != samplerBurstLines {
		t.Fatalf("APP_ENV=local must not sample: %d of %d lines emitted", n, samplerBurstLines)
	}
}
