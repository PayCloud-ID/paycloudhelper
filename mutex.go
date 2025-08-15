package paycloudhelper

import (
	"sync"

	"github.com/go-redsync/redsync/v4"
)

var (
	// Other vars...
	mutexMap     = make(map[string]*redsync.Mutex)
	mutexMapLock = &sync.Mutex{}
)

// StoreMutex stores a mutex in the map for later release
func StoreMutex(key string, mutex *redsync.Mutex) {
	mutexMapLock.Lock()
	defer mutexMapLock.Unlock()
	mutexMap[key] = mutex
}

// GetMutex retrieves a mutex from the map
func GetMutex(key string) *redsync.Mutex {
	mutexMapLock.Lock()
	defer mutexMapLock.Unlock()
	mutex, exists := mutexMap[key]
	if !exists {
		return nil
	}
	return mutex
}

// RemoveMutex removes a mutex from the map
func RemoveMutex(key string) {
	mutexMapLock.Lock()
	defer mutexMapLock.Unlock()
	delete(mutexMap, key)
}
