package phaudittrailv0

import "testing"

// PushMessage with no queue configured should return without panicking (early exit).
func TestPushMessage_nilQueue(t *testing.T) {
	Que = nil
	Channel = nil
	t.Cleanup(func() {
		Que = nil
		Channel = nil
	})
	PushMessage(map[string]string{"event": "noop"})
}
