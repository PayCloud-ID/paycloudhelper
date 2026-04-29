package phlogger

import (
	"testing"
	"time"

	"github.com/PayCloud-ID/paycloudhelper/phhelper"
)

func TestSampler_DisabledConfig_AlwaysAllows(t *testing.T) {
	s := &sampler{config: SamplerConfig{}} // Initial=0 → disabled
	for i := 0; i < 100; i++ {
		allowed, _ := s.check("key")
		if !allowed {
			t.Fatalf("disabled sampler should allow all, blocked at iteration %d", i)
		}
	}
}

func TestSampler_InitialBurst(t *testing.T) {
	s := &sampler{config: SamplerConfig{Initial: 3, Thereafter: 0, Period: time.Second}}
	for i := 1; i <= 3; i++ {
		allowed, _ := s.check("key")
		if !allowed {
			t.Fatalf("call %d within initial burst should be allowed", i)
		}
	}
	// 4th call should be blocked (Thereafter=0 means drop all after)
	allowed, _ := s.check("key")
	if allowed {
		t.Fatal("call beyond initial burst should be blocked when Thereafter=0")
	}
}

func TestSampler_ThereafterSampling(t *testing.T) {
	cfg := SamplerConfig{Initial: 2, Thereafter: 5, Period: 10 * time.Second}
	s := &sampler{config: cfg}

	// First 2 allowed (initial)
	s.check("key")
	s.check("key")

	// Next 4 suppressed, 5th allowed
	for i := 0; i < 4; i++ {
		allowed, _ := s.check("key")
		if allowed {
			t.Fatalf("call %d after initial should be suppressed", i+3)
		}
	}
	allowed, suppressed := s.check("key") // call 7 = 5th over initial
	if !allowed {
		t.Fatal("every Thereafter-th call should be allowed")
	}
	if suppressed != 4 {
		t.Fatalf("expected 4 suppressed, got %d", suppressed)
	}
}

func TestSampler_IndependentKeys(t *testing.T) {
	s := &sampler{config: SamplerConfig{Initial: 1, Thereafter: 0, Period: time.Second}}
	a1, _ := s.check("a")
	b1, _ := s.check("b")
	if !a1 || !b1 {
		t.Fatal("different keys should be independent")
	}
	a2, _ := s.check("a")
	b2, _ := s.check("b")
	if a2 || b2 {
		t.Fatal("second call for each key should be blocked (Initial=1, Thereafter=0)")
	}
}

func TestSampler_PeriodReset(t *testing.T) {
	cfg := SamplerConfig{Initial: 1, Thereafter: 0, Period: 50 * time.Millisecond}
	s := &sampler{config: cfg}

	s.check("key") // allowed
	allowed, _ := s.check("key")
	if allowed {
		t.Fatal("second call in same period should be blocked")
	}

	time.Sleep(60 * time.Millisecond) // wait for period to elapse

	allowed, _ = s.check("key")
	if !allowed {
		t.Fatal("first call in new period should be allowed")
	}
}

func TestSamplerConfigForEnv_Production(t *testing.T) {
	cfg := SamplerConfigForEnv("production")
	if cfg.Initial != 5 || cfg.Thereafter != 50 || cfg.Period != time.Second {
		t.Errorf("production config mismatch: %+v", cfg)
	}
	cfg2 := SamplerConfigForEnv("prod")
	if cfg2.Initial != 5 {
		t.Error("prod should match production")
	}
}

func TestSamplerConfigForEnv_Staging(t *testing.T) {
	cfg := SamplerConfigForEnv("staging")
	if cfg.Initial != 10 || cfg.Thereafter != 10 || cfg.Period != time.Second {
		t.Errorf("staging config mismatch: %+v", cfg)
	}
}

func TestSamplerConfigForEnv_Dev(t *testing.T) {
	cfg := SamplerConfigForEnv("develop")
	if cfg.Initial != 20 {
		t.Errorf("dev should disable sampling, got Initial=%d", cfg.Initial)
	}
	cfg2 := SamplerConfigForEnv("")
	if cfg2.Initial != 0 {
		t.Error("empty env should disable sampling")
	}
}

func TestSamplerConfigFromAppEnv(t *testing.T) {
	original := phhelper.GetAppEnv()
	defer phhelper.SetAppEnv(original)

	phhelper.SetAppEnv("production")
	cfg := SamplerConfigFromAppEnv()
	if cfg.Initial != 5 {
		t.Error("SamplerConfigFromAppEnv should use phhelper.GetAppEnv()")
	}
}

func TestInitializeSampler_DefaultsPeriod(t *testing.T) {
	InitializeSampler(SamplerConfig{Initial: 5, Thereafter: 10})
	if globalSampler.config.Period != time.Second {
		t.Error("InitializeSampler should default Period to 1s when omitted")
	}
	// Reset
	InitializeSampler(SamplerConfig{})
}
