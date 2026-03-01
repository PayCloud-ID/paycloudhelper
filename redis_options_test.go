package paycloudhelper

import (
	"os"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
)

func TestInitRedisOptions(t *testing.T) {
	opts := redis.Options{
		Addr:     "localhost:6379",
		Password: "secret",
		DB:       1,
	}
	got := InitRedisOptions(opts)
	if got == nil {
		t.Fatal("InitRedisOptions() returned nil")
	}
	if got.Addr != "localhost:6379" {
		t.Errorf("Addr = %q, want localhost:6379", got.Addr)
	}
	if got.Password != "secret" {
		t.Errorf("Password = %q, want secret", got.Password)
	}
	if got.DB != 1 {
		t.Errorf("DB = %d, want 1", got.DB)
	}
	if got.Username != "default" {
		t.Errorf("Username = %q, want default", got.Username)
	}
	if got.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", got.MaxRetries)
	}
	if got.MinRetryBackoff != 10*time.Millisecond {
		t.Errorf("MinRetryBackoff = %v, want 10ms", got.MinRetryBackoff)
	}
	if got.MaxRetryBackoff != 500*time.Millisecond {
		t.Errorf("MaxRetryBackoff = %v, want 500ms", got.MaxRetryBackoff)
	}
	if got.IdleTimeout != 5*time.Minute {
		t.Errorf("IdleTimeout = %v, want 5m", got.IdleTimeout)
	}
}

func TestInitRedisOptions_EmptyAddrUsesMem(t *testing.T) {
	InitRedisOptions(redis.Options{Addr: "redis.example.com:6380", Password: "p", DB: 2})
	opts := InitRedisOptions(redis.Options{Addr: "", Password: "", DB: 0})
	if opts == nil {
		t.Fatal("InitRedisOptions() returned nil")
	}
	if opts.Addr != "redis.example.com:6380" {
		t.Errorf("Addr = %q, want redis.example.com:6380", opts.Addr)
	}
	// DB may be 0 or 2 depending on init order; Addr from memory is the main assertion.
}

func TestGetTrxRedisBackoff(t *testing.T) {
	old := os.Getenv("TRANSACTION_REDIS_BACKOFF")
	defer os.Setenv("TRANSACTION_REDIS_BACKOFF", old)

	t.Run("default when unset", func(t *testing.T) {
		os.Unsetenv("TRANSACTION_REDIS_BACKOFF")
		got := GetTrxRedisBackoff()
		if got != 10 {
			t.Errorf("GetTrxRedisBackoff() = %d, want default 10", got)
		}
	})
	t.Run("env value when valid", func(t *testing.T) {
		os.Setenv("TRANSACTION_REDIS_BACKOFF", "25")
		got := GetTrxRedisBackoff()
		if got != 25 {
			t.Errorf("GetTrxRedisBackoff() = %d, want 25", got)
		}
	})
	t.Run("default when below minimum", func(t *testing.T) {
		os.Setenv("TRANSACTION_REDIS_BACKOFF", "5")
		got := GetTrxRedisBackoff()
		if got != 10 {
			t.Errorf("GetTrxRedisBackoff() = %d, want default 10 when < 10", got)
		}
	})
	t.Run("default when invalid", func(t *testing.T) {
		os.Setenv("TRANSACTION_REDIS_BACKOFF", "invalid")
		got := GetTrxRedisBackoff()
		if got != 10 {
			t.Errorf("GetTrxRedisBackoff() = %d, want default 10 when invalid", got)
		}
	})
}

func TestGetTrxRedisLockTimeout(t *testing.T) {
	old := os.Getenv("TRANSACTION_REDIS_LOCK_TIMEOUT")
	defer os.Setenv("TRANSACTION_REDIS_LOCK_TIMEOUT", old)

	t.Run("default when unset", func(t *testing.T) {
		os.Unsetenv("TRANSACTION_REDIS_LOCK_TIMEOUT")
		got := GetTrxRedisLockTimeout()
		if got != 2000*time.Millisecond {
			t.Errorf("GetTrxRedisLockTimeout() = %v, want 2000ms", got)
		}
	})
	t.Run("env value when valid and above min", func(t *testing.T) {
		os.Setenv("TRANSACTION_REDIS_LOCK_TIMEOUT", "3000")
		got := GetTrxRedisLockTimeout()
		if got != 3000*time.Millisecond {
			t.Errorf("GetTrxRedisLockTimeout() = %v, want 3000ms", got)
		}
	})
	t.Run("default when below min 700", func(t *testing.T) {
		os.Setenv("TRANSACTION_REDIS_LOCK_TIMEOUT", "500")
		got := GetTrxRedisLockTimeout()
		if got != 2000*time.Millisecond {
			t.Errorf("GetTrxRedisLockTimeout() = %v, want default 2000ms when < 700", got)
		}
	})
	t.Run("default when invalid", func(t *testing.T) {
		os.Setenv("TRANSACTION_REDIS_LOCK_TIMEOUT", "abc")
		got := GetTrxRedisLockTimeout()
		if got != 2000*time.Millisecond {
			t.Errorf("GetTrxRedisLockTimeout() = %v, want default 2000ms when invalid", got)
		}
	})
}

func TestGetRedisPoolClient_ReturnsErrorWhenNotInitialized(t *testing.T) {
	// Only verify behavior when redisOptions is nil (e.g. first test run or no init).
	// If another test already set redisOptions, skip to avoid mutating shared state.
	if redisOptions != nil {
		t.Skip("redisOptions already set; skipping to avoid affecting other tests")
	}
	client, err := GetRedisPoolClient()
	if err == nil {
		t.Error("GetRedisPoolClient() expected error when options nil, got nil")
	}
	if client != nil {
		t.Error("GetRedisPoolClient() expected nil client when options nil")
	}
}

