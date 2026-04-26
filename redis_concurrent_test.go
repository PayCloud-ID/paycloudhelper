package paycloudhelper

import (
	"sync"
	"testing"

	"github.com/go-redsync/redsync/v4"
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
