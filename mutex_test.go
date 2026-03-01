package paycloudhelper

import (
	"testing"

	"github.com/go-redsync/redsync/v4"
)

func TestStoreMutex_GetMutex_RemoveMutex(t *testing.T) {
	key := "test-mutex-key-unit"
	RemoveMutex(key)
	defer RemoveMutex(key)

	if got := GetMutex(key); got != nil {
		t.Errorf("GetMutex(%q) before store = %v, want nil", key, got)
	}

	var m *redsync.Mutex
	StoreMutex(key, m)
	got := GetMutex(key)
	if got != nil {
		t.Errorf("GetMutex(%q) after store nil = %v, want nil", key, got)
	}

	StoreMutex(key, &redsync.Mutex{})
	got = GetMutex(key)
	if got == nil {
		t.Error("GetMutex() after store non-nil mutex = nil, want non-nil")
	}

	RemoveMutex(key)
	if got := GetMutex(key); got != nil {
		t.Errorf("GetMutex(%q) after remove = %v, want nil", key, got)
	}
}

func TestMutex_DifferentKeys(t *testing.T) {
	k1, k2 := "mutex-a", "mutex-b"
	RemoveMutex(k1)
	RemoveMutex(k2)
	defer func() { RemoveMutex(k1); RemoveMutex(k2) }()

	StoreMutex(k1, &redsync.Mutex{})
	StoreMutex(k2, nil)

	if GetMutex(k1) == nil {
		t.Error("GetMutex(k1) want non-nil")
	}
	if GetMutex(k2) != nil {
		t.Error("GetMutex(k2) want nil")
	}
}
