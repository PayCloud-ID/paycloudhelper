package paycloudhelper

import (
	"sync"
	"testing"
	"time"

	"github.com/go-redsync/redsync/v4"
	"github.com/redis/go-redis/v9"
)

// TestInitRedSyncOnce_concurrent verifies that concurrent calls to InitRedSyncOnce are
// race-safe (sync.Once guarantees single initialisation). Run with go test -race.
func TestInitRedSyncOnce_concurrent(t *testing.T) {
	_ = setupMiniredis(t)

	const goroutines = 20
	var wg sync.WaitGroup
	errs := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := InitRedSyncOnce(); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("InitRedSyncOnce concurrent: %v", err)
	}
}

// TestMutexMap_concurrent verifies that concurrent Store/Get/Remove operations on the
// internal mutex map are race-safe. Run with go test -race.
func TestMutexMap_concurrent(t *testing.T) {
	keys := []string{"key-a", "key-b", "key-c", "key-d", "key-e"}

	const goroutines = 50
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		idx := i
		go func() {
			defer wg.Done()
			key := keys[idx%len(keys)]
			m := &redsync.Mutex{}
			StoreMutex(key, m)
			_ = GetMutex(key)
			RemoveMutex(key)
		}()
	}
	wg.Wait()
	// Reaching here without panic or data race is the success criterion.
}

// TestInitRedSyncOnce_idempotent verifies that calling InitRedSyncOnce multiple times
// sequentially returns nil and does not reinitialise the instance.
func TestInitRedSyncOnce_idempotent(t *testing.T) {
	_ = setupMiniredis(t)

	for i := 0; i < 5; i++ {
		if err := InitRedSyncOnce(); err != nil {
			t.Fatalf("InitRedSyncOnce() call #%d: %v", i+1, err)
		}
	}
	if redisSync == nil {
		t.Fatal("redisSync is nil after InitRedSyncOnce")
	}
}

// TestInitRedisOptions_concurrent reproduces PA-293: InitRedisOptions writes the
// package-level Redis configuration globals while GetRedisOptions and
// GetRedisPoolClient read them, with no synchronisation between the two. The
// dialing failures are irrelevant here — the assertion is the absence of a race.
// Run with go test -race.
func TestInitRedisOptions_concurrent(t *testing.T) {
	resetRedisClientStateForTesting()
	t.Cleanup(resetRedisClientStateForTesting)

	const goroutines = 16
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			// A short DialTimeout keeps the unreachable-port path fast; the
			// test is about the config globals, not connectivity.
			InitRedisOptions(redis.Options{
				Addr:        "127.0.0.1:63799",
				DialTimeout: 20 * time.Millisecond,
			})
		}()
		go func() {
			defer wg.Done()
			_ = GetRedisOptions()
			_, _ = GetRedisPoolClient()
		}()
	}
	wg.Wait()
}

// TestInitializeRedisWithRetry_concurrent is the regression guard for the gap
// v1.12.1 missed: InitializeRedisWithRetry writes redisLockKey *outside*
// InitRedisOptions, so the v1.12.1 mutex did not cover it, while lock
// acquisition reads it. v1.12.1's own test suite did not exercise
// InitializeRedisWithRetry concurrently, which is why it shipped.
// Run with go test -race.
func TestInitializeRedisWithRetry_concurrent(t *testing.T) {
	resetRedisClientStateForTesting()
	t.Cleanup(resetRedisClientStateForTesting)

	const goroutines = 8
	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_ = InitializeRedisWithRetry(RedisInitOptions{
				Options:    redis.Options{Addr: "127.0.0.1:63799", DialTimeout: 20 * time.Millisecond},
				MaxRetries: 1,
				RetryDelay: time.Millisecond,
				FailFast:   true,
			})
		}()
		go func() {
			defer wg.Done()
			_ = getRedisLockKey()
			_ = GetRedisOptions()
		}()
	}
	wg.Wait()
}
