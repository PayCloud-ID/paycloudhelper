package paycloudhelper

import (
	"errors"
	"strings"
	"testing"
)

func TestLockError_Error(t *testing.T) {
	tests := []struct {
		name   string
		err    *LockError
		substr []string
	}{
		{
			name: "with underlying error",
			err: &LockError{
				Key:    "lock:key1",
				Op:     "acquire",
				Reason: "redsync failed",
				Err:    errors.New("connection refused"),
			},
			substr: []string{"acquire", "lock:key1", "redsync failed", "connection refused"},
		},
		{
			name: "without underlying error",
			err: &LockError{
				Key:    "lock:key2",
				Op:     "release",
				Reason: "no mutex found",
				Err:    nil,
			},
			substr: []string{"release", "lock:key2", "no mutex found"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			for _, s := range tt.substr {
				if !strings.Contains(got, s) {
					t.Errorf("LockError.Error() = %q, want to contain %q", got, s)
				}
			}
		})
	}
}

func TestLockError_Unwrap(t *testing.T) {
	inner := errors.New("inner error")
	err := &LockError{Key: "k", Op: "acquire", Reason: "r", Err: inner}
	if got := err.Unwrap(); got != inner {
		t.Errorf("Unwrap() = %v, want %v", got, inner)
	}
	errNil := &LockError{Key: "k", Op: "release", Reason: "r", Err: nil}
	if got := errNil.Unwrap(); got != nil {
		t.Errorf("Unwrap() with nil Err = %v, want nil", got)
	}
}
