package paycloudhelper

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestInitializeRedis_miniredisConnects(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(func() {
		resetRedisClientStateForTesting()
		mr.Close()
	})
	resetRedisClientStateForTesting()

	InitializeRedis(redis.Options{Addr: mr.Addr()})

	c, err := GetRedisPoolClient()
	if err != nil {
		t.Fatalf("GetRedisPoolClient: %v", err)
	}
	if err := c.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func TestGetRedisClient_miniredis(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(func() {
		resetRedisClientStateForTesting()
		mr.Close()
	})
	resetRedisClientStateForTesting()

	host, port, err := net.SplitHostPort(mr.Addr())
	if err != nil {
		t.Fatal(err)
	}
	if err := GetRedisClient(host, port, "", 0); err != nil {
		t.Fatalf("GetRedisClient: %v", err)
	}
	c, err := GetRedisPoolClient()
	if err != nil {
		t.Fatalf("GetRedisPoolClient: %v", err)
	}
	if err := c.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func TestDeleteRedis_miniredis(t *testing.T) {
	_ = setupMiniredis(t)
	key := "del_ext_" + t.Name()
	if err := StoreRedis(key, "v1", time.Minute); err != nil {
		t.Fatalf("StoreRedis: %v", err)
	}
	if err := DeleteRedis(key); err != nil {
		t.Fatalf("DeleteRedis: %v", err)
	}
	_, err := GetRedis(key)
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestDeleteRedisWithContext_miniredis(t *testing.T) {
	_ = setupMiniredis(t)
	key := "del_ctx_" + t.Name()
	if err := StoreRedis(key, "v2", time.Minute); err != nil {
		t.Fatalf("StoreRedis: %v", err)
	}
	if err := DeleteRedisWithContext(context.Background(), key); err != nil {
		t.Fatalf("DeleteRedisWithContext: %v", err)
	}
}

func TestStoreRedisWithLock_miniredis(t *testing.T) {
	_ = setupMiniredis(t)
	key := "lock_kv_" + t.Name()
	if err := StoreRedisWithLock(key, map[string]int{"n": 42}, time.Minute); err != nil {
		t.Fatalf("StoreRedisWithLock: %v", err)
	}
	got, err := GetRedis(key)
	if err != nil {
		t.Fatalf("GetRedis: %v", err)
	}
	if got == "" {
		t.Fatal("expected stored JSON payload")
	}
}

func TestReleaseLock_unknownKey(t *testing.T) {
	resetRedisClientStateForTesting()
	t.Cleanup(resetRedisClientStateForTesting)

	err := ReleaseLock("no-such-lock-key-" + t.Name())
	if err == nil {
		t.Fatal("expected error")
	}
	var le *LockError
	if !errors.As(err, &le) {
		t.Fatalf("want *LockError, got %T: %v", err, err)
	}
}

func TestAcquireLock_ReleaseLock_miniredis(t *testing.T) {
	_ = setupMiniredis(t)
	key := "acq_rel_" + t.Name()
	ok, err := AcquireLock(key, 2*time.Second)
	if err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}
	if !ok {
		t.Fatal("expected lock acquired")
	}
	if err := ReleaseLock(key); err != nil {
		t.Fatalf("ReleaseLock: %v", err)
	}
}

func TestAcquireLockWithRetry_ReleaseLockWithRetry_miniredis(t *testing.T) {
	_ = setupMiniredis(t)
	key := "acq_retry_" + t.Name()
	mu, got, err := AcquireLockWithRetry(key, 2*time.Second, 3, 5*time.Millisecond)
	if err != nil {
		t.Fatalf("AcquireLockWithRetry: %v", err)
	}
	if !got || mu == nil {
		t.Fatalf("expected acquired mutex, got=%v mu=%v", got, mu)
	}
	if err := ReleaseLockWithRetry(mu, 3); err != nil {
		t.Fatalf("ReleaseLockWithRetry: %v", err)
	}
}

// --- v9 context-aware API tests ---

// TestStoreAndGetRedisWithContext_roundTrip verifies the v9 context-aware store/get path
// using an explicit context, confirming JSON serialisation round-trips cleanly.
func TestStoreAndGetRedisWithContext_roundTrip(t *testing.T) {
	_ = setupMiniredis(t)
	ctx := context.Background()
	key := "ctx_rt_" + t.Name()
	payload := map[string]string{"lib": "go-redis/v9"}

	if err := StoreRedisWithContext(ctx, key, payload, time.Minute); err != nil {
		t.Fatalf("StoreRedisWithContext: %v", err)
	}
	raw, err := GetRedisWithContext(ctx, key)
	if err != nil {
		t.Fatalf("GetRedisWithContext: %v", err)
	}
	var got map[string]string
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("unmarshal: %v (raw=%q)", err, raw)
	}
	if got["lib"] != "go-redis/v9" {
		t.Errorf("GetRedisWithContext round-trip = %v, want lib=go-redis/v9", got)
	}
}

// TestStoreRedisWithContext_cancelledContext verifies that a pre-cancelled context is
// propagated to the Redis client (v9 checks ctx.Err() before executing commands).
func TestStoreRedisWithContext_cancelledContext(t *testing.T) {
	_ = setupMiniredis(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before any operation

	err := StoreRedisWithContext(ctx, "cancel_store_"+t.Name(), "any", time.Minute)
	if err == nil {
		t.Fatal("StoreRedisWithContext with cancelled context: want error, got nil")
	}
}

// TestGetRedisWithContext_cancelledContext verifies that a pre-cancelled context is
// propagated on the read path.
func TestGetRedisWithContext_cancelledContext(t *testing.T) {
	_ = setupMiniredis(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before any operation

	_, err := GetRedisWithContext(ctx, "cancel_get_"+t.Name())
	if err == nil {
		t.Fatal("GetRedisWithContext with cancelled context: want error, got nil")
	}
}

// TestAcquireLockWithRetry_contention_returnsFalseNilError verifies that when a lock is
// already held, AcquireLockWithRetry with tries=1 returns (nil, false, nil) per the
// redsync.ErrFailed contract (v9 LockContext API).
func TestAcquireLockWithRetry_contention_returnsFalseNilError(t *testing.T) {
	_ = setupMiniredis(t)
	key := "contention_" + t.Name()

	// Hold the lock.
	mu, ok, err := AcquireLockWithRetry(key, 10*time.Second, 3, 5*time.Millisecond)
	if err != nil || !ok || mu == nil {
		t.Fatalf("first AcquireLockWithRetry: ok=%v err=%v", ok, err)
	}
	t.Cleanup(func() { _ = ReleaseLockWithRetry(mu, 1) })

	// Second acquisition with tries=1 should fail immediately with ErrFailed,
	// which our implementation maps to (nil, false, nil).
	mu2, ok2, err2 := AcquireLockWithRetry(key, 10*time.Second, 1, 5*time.Millisecond)
	if err2 != nil {
		t.Fatalf("contended AcquireLockWithRetry: want nil error for ErrFailed, got: %v", err2)
	}
	if ok2 || mu2 != nil {
		t.Fatalf("contended AcquireLockWithRetry: want acquired=false, got ok=%v mu=%v", ok2, mu2)
	}
}

// TestReleaseLockWithRetry_nilMutex verifies that passing a nil mutex returns an error.
func TestReleaseLockWithRetry_nilMutex(t *testing.T) {
	err := ReleaseLockWithRetry(nil, 1)
	if err == nil {
		t.Fatal("ReleaseLockWithRetry(nil, 1): want error, got nil")
	}
}

// TestInitializeRedisWithRetry_FailFast_badAddr verifies FailFast=true propagates
// connection errors (tests the retry and failure path without live Redis).
func TestInitializeRedisWithRetry_FailFast_badAddr(t *testing.T) {
	t.Cleanup(resetRedisClientStateForTesting)
	resetRedisClientStateForTesting()

	err := InitializeRedisWithRetry(RedisInitOptions{
		Options:    redis.Options{Addr: "127.0.0.1:1"}, // unreachable port
		MaxRetries: 1,
		RetryDelay: 1 * time.Millisecond,
		FailFast:   true,
	})
	if err == nil {
		t.Fatal("InitializeRedisWithRetry with bad addr and FailFast=true: want error, got nil")
	}
}

// TestAcquireLockWithRetry_uninitializedRedsync verifies the error path when redsync
// is not yet initialized (no Redis client set up).
func TestAcquireLockWithRetry_uninitializedRedsync(t *testing.T) {
	resetRedisClientStateForTesting()
	t.Cleanup(resetRedisClientStateForTesting)

	_, ok, err := AcquireLockWithRetry("some-key", time.Second, 1, 0)
	if err == nil {
		t.Fatal("AcquireLockWithRetry without Redis init: want error, got nil")
	}
	if ok {
		t.Fatal("AcquireLockWithRetry without Redis init: want acquired=false")
	}
}
