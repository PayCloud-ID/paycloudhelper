package paycloudhelper

import (
	"sync"
	"testing"
)

func TestCheckHealth_UninitializedComponents(t *testing.T) {
	hc := CheckHealth()
	if hc == nil {
		t.Fatal("CheckHealth() returned nil")
	}
	if hc.AppName == "" && hc.AppEnv == "" {
		t.Log("app name/env may be empty in test env")
	}
	if hc.Overall != "unhealthy" && hc.Overall != "degraded" && hc.Overall != "healthy" {
		t.Errorf("unexpected Overall=%q", hc.Overall)
	}
	if len(hc.Checks) < 3 {
		t.Fatalf("expected redis+rabbitmq+sentry checks, got %d", len(hc.Checks))
	}
}

func TestGetRedisPoolStats_NilClient(t *testing.T) {
	if GetRedisPoolStats() != nil {
		t.Log("redis client unexpectedly initialized in test")
	}
	if GetRedisMetrics() != nil {
		t.Log("redis client unexpectedly initialized in test")
	}
	if d := GetRedisMetricsDetailed(); d != nil {
		t.Fatalf("GetRedisMetricsDetailed() = %v, want nil when redis unset", d)
	}
}

func TestCheckHealth_redisHealthyWithMiniredis(t *testing.T) {
	_ = setupMiniredis(t)
	hc := CheckHealth()
	if hc == nil {
		t.Fatal("nil health")
	}
	var redisStatus string
	for _, c := range hc.Checks {
		if c.Component == "redis" {
			redisStatus = c.Status
			break
		}
	}
	if redisStatus != "healthy" {
		t.Fatalf("redis check = %q want healthy", redisStatus)
	}
}

func TestGetRedisMetricsDetailed_withPool(t *testing.T) {
	_ = setupMiniredis(t)
	ctx := t.Context()
	if c, err := GetRedisPoolClient(); err == nil {
		_ = c.Ping(ctx).Err()
	}
	d := GetRedisMetricsDetailed()
	if d == nil || d.PoolStats == nil {
		t.Fatalf("GetRedisMetricsDetailed = %v", d)
	}
	if d.HitRate < 0 {
		t.Fatal("negative hit rate")
	}
}

func TestGetRedisPoolStats_nonNilWhenInitialized(t *testing.T) {
	_ = setupMiniredis(t)
	s := GetRedisPoolStats()
	if s == nil {
		t.Fatal("expected pool stats")
	}
	if s.TotalConns < 0 {
		t.Fatal("invalid stats")
	}
}

func TestCheckHealth_rabbitmqDegradedWhenNotReady(t *testing.T) {
	orig := auditTrailMqClient.Load()
	defer auditTrailMqClient.Store(orig)
	auditTrailMqClient.Store(&AmqpClient{m: &sync.Mutex{}, isReady: false})

	hc := CheckHealth()
	for _, c := range hc.Checks {
		if c.Component != "rabbitmq" {
			continue
		}
		if c.Status != "degraded" || c.Message != "connection not ready" {
			t.Fatalf("rabbitmq check = %#v want degraded/connection not ready", c)
		}
		return
	}
	t.Fatal("rabbitmq check missing")
}

func TestCheckHealth_rabbitmqHealthyWhenReady(t *testing.T) {
	orig := auditTrailMqClient.Load()
	defer auditTrailMqClient.Store(orig)
	auditTrailMqClient.Store(&AmqpClient{m: &sync.Mutex{}, isReady: true})

	hc := CheckHealth()
	for _, c := range hc.Checks {
		if c.Component != "rabbitmq" {
			continue
		}
		if c.Status != "healthy" {
			t.Fatalf("rabbitmq check = %#v want healthy", c)
		}
		return
	}
	t.Fatal("rabbitmq check missing")
}
