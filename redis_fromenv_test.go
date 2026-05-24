package paycloudhelper

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
)

func TestInitRedisFromEnv_noHost(t *testing.T) {
	t.Setenv("REDIS_HOST", "")
	resetRedisClientStateForTesting()
	t.Cleanup(resetRedisClientStateForTesting)

	if err := InitRedisFromEnv(); err != nil {
		t.Fatalf("expected nil when REDIS_HOST unset, got %v", err)
	}
	if RedisEnabled() {
		t.Fatal("RedisEnabled should be false when REDIS_HOST is unset")
	}
}

func TestInitRedisFromEnv_withHost(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(func() {
		resetRedisClientStateForTesting()
		mr.Close()
	})
	resetRedisClientStateForTesting()

	host, port := mr.Host(), mr.Port()
	t.Setenv("REDIS_HOST", host)
	t.Setenv("REDIS_PORT", port)
	t.Setenv("REDIS_PASSWORD", "")
	t.Setenv("REDIS_DB", "0")

	if err := InitRedisFromEnv(); err != nil {
		t.Fatalf("InitRedisFromEnv: %v", err)
	}
	if !RedisEnabled() {
		t.Fatal("RedisEnabled should be true after successful init")
	}
}

func TestInitRedisFromEnv_invalidDB(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(func() {
		resetRedisClientStateForTesting()
		mr.Close()
	})
	resetRedisClientStateForTesting()

	host, port := mr.Host(), mr.Port()
	t.Setenv("REDIS_HOST", host)
	t.Setenv("REDIS_PORT", port)
	t.Setenv("REDIS_DB", "not-a-number") // falls back to 0

	if err := InitRedisFromEnv(); err != nil {
		t.Fatalf("InitRedisFromEnv with bad REDIS_DB: %v", err)
	}
	if !RedisEnabled() {
		t.Fatal("RedisEnabled should be true")
	}
}

func TestInitRedisFromEnv_defaultPort(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(func() {
		resetRedisClientStateForTesting()
		mr.Close()
	})
	resetRedisClientStateForTesting()

	// miniredis listens on a random port; override with explicit REDIS_PORT
	// to exercise the default-port path being overridden by env.
	host, port := mr.Host(), mr.Port()
	t.Setenv("REDIS_HOST", host)
	t.Setenv("REDIS_PORT", port)
	t.Setenv("REDIS_DB", "")

	if err := InitRedisFromEnv(); err != nil {
		t.Fatalf("InitRedisFromEnv: %v", err)
	}
}

func TestInitRedisFromEnv_unreachable(t *testing.T) {
	resetRedisClientStateForTesting()
	t.Cleanup(resetRedisClientStateForTesting)

	t.Setenv("REDIS_HOST", "127.0.0.1")
	t.Setenv("REDIS_PORT", "19999") // nothing listening here

	err := InitRedisFromEnv()
	if err == nil {
		t.Fatal("expected error for unreachable Redis host")
	}
	if RedisEnabled() {
		t.Fatal("RedisEnabled should be false after failed init")
	}
}
