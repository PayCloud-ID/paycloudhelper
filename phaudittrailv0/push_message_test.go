package phaudittrailv0

import (
	"errors"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

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

func TestSetUpRabbitMq_success_setsGlobals(t *testing.T) {
	prevDial := auditV0DialHook
	prevAfter := auditV0AfterHook
	prevCh := auditV0ChannelHook
	prevCloseCh := auditV0ChannelCloseHook
	prevCloseConn := auditV0ConnCloseHook
	prevMax := auditV0MaxTrialsForTest
	t.Cleanup(func() {
		auditV0DialHook = prevDial
		auditV0AfterHook = prevAfter
		auditV0ChannelHook = prevCh
		auditV0ChannelCloseHook = prevCloseCh
		auditV0ConnCloseHook = prevCloseConn
		auditV0MaxTrialsForTest = prevMax
		Conn = nil
		Channel = nil
		Que = nil
	})

	auditV0DialHook = func(string, amqp.Config) (*amqp.Connection, error) { return &amqp.Connection{}, nil }
	auditV0AfterHook = func(time.Duration) <-chan time.Time { ch := make(chan time.Time); close(ch); return ch }
	auditV0ChannelHook = func(*amqp.Connection) (*amqp.Channel, error) { return &amqp.Channel{}, nil }
	auditV0ChannelCloseHook = func(*amqp.Channel) error { return nil }
	auditV0ConnCloseHook = func(*amqp.Connection) error { return nil }

	r := SetUpRabbitMq("h", "5672", "v", "u", "p", "q", "app")
	if !r.Initialized || Conn == nil || Channel == nil || Que == nil || *Que != "q" {
		t.Fatalf("unexpected state Initialized=%v Conn=%v Channel=%v Que=%v", r.Initialized, Conn, Channel, Que)
	}
}

func TestStartRQConnection_retryCapped_returnsError(t *testing.T) {
	r := &RMqAutoConnect{uriConnection: "amqp://x"}
	prevDial := auditV0DialHook
	prevAfter := auditV0AfterHook
	prevMax := auditV0MaxTrialsForTest
	t.Cleanup(func() {
		auditV0DialHook = prevDial
		auditV0AfterHook = prevAfter
		auditV0MaxTrialsForTest = prevMax
	})

	var dials int
	auditV0DialHook = func(string, amqp.Config) (*amqp.Connection, error) {
		dials++
		return nil, errors.New("dial failed")
	}
	auditV0AfterHook = func(time.Duration) <-chan time.Time { ch := make(chan time.Time); close(ch); return ch }
	auditV0MaxTrialsForTest = 3

	_, _, err := r.startRQConnection()
	if err == nil {
		t.Fatal("expected error")
	}
	if dials != 3 {
		t.Fatalf("dials=%d want 3", dials)
	}
}

func TestStartRQConnection_exercisesBackoffBranches(t *testing.T) {
	r := &RMqAutoConnect{uriConnection: "amqp://x"}
	prevDial := auditV0DialHook
	prevAfter := auditV0AfterHook
	prevMax := auditV0MaxTrialsForTest
	t.Cleanup(func() {
		auditV0DialHook = prevDial
		auditV0AfterHook = prevAfter
		auditV0MaxTrialsForTest = prevMax
	})

	var dials int
	auditV0DialHook = func(string, amqp.Config) (*amqp.Connection, error) {
		dials++
		return nil, errors.New("dial failed")
	}
	auditV0AfterHook = func(time.Duration) <-chan time.Time { ch := make(chan time.Time); close(ch); return ch }

	// maxTrialSecond=3, maxTrialMinute=5; running 7 trials exercises:
	// - 30s branch (1-3), 10m branch (4-5), default branch (6+).
	auditV0MaxTrialsForTest = 7
	_, _, err := r.startRQConnection()
	if err == nil {
		t.Fatal("expected error")
	}
	if dials != 7 {
		t.Fatalf("dials=%d want 7", dials)
	}
}

func TestReset_nilSafe(t *testing.T) {
	r := &RMqAutoConnect{}
	r.reset() // should not panic
}

func TestCheckIfQueueExists_hookPaths(t *testing.T) {
	prev := auditV0QueuePassiveHook
	t.Cleanup(func() { auditV0QueuePassiveHook = prev })

	auditV0QueuePassiveHook = func(*amqp.Channel, string) (amqp.Queue, error) {
		return amqp.Queue{}, nil
	}
	ok, err := checkIfQueueExists(&amqp.Channel{}, "q")
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}

	auditV0QueuePassiveHook = func(*amqp.Channel, string) (amqp.Queue, error) {
		return amqp.Queue{}, errors.New("missing")
	}
	ok2, err2 := checkIfQueueExists(&amqp.Channel{}, "q")
	if ok2 || err2 == nil {
		t.Fatalf("expected missing queue, ok=%v err=%v", ok2, err2)
	}
}

func TestPushMessage_branches(t *testing.T) {
	prevPassive := auditV0QueuePassiveHook
	prevDeclare := auditV0QueueDeclareHook
	prevPublish := auditV0PublishHook
	t.Cleanup(func() {
		auditV0QueuePassiveHook = prevPassive
		auditV0QueueDeclareHook = prevDeclare
		auditV0PublishHook = prevPublish
		Que = nil
		Channel = nil
	})

	q := "q"
	Que = &q
	Channel = &amqp.Channel{}

	// marshal error
	PushMessage(func() {})

	// queue exists, publish success
	auditV0QueuePassiveHook = func(*amqp.Channel, string) (amqp.Queue, error) { return amqp.Queue{}, nil }
	auditV0PublishHook = func(*amqp.Channel, string, []byte) error { return nil }
	PushMessage(map[string]string{"k": "v"})

	// queue missing -> declare -> publish error
	var declared int
	auditV0QueuePassiveHook = func(*amqp.Channel, string) (amqp.Queue, error) { return amqp.Queue{}, errors.New("missing") }
	auditV0QueueDeclareHook = func(*amqp.Channel, string) (amqp.Queue, error) { declared++; return amqp.Queue{}, nil }
	auditV0PublishHook = func(*amqp.Channel, string, []byte) error { return errors.New("publish failed") }
	PushMessage(map[string]string{"k": "v"})
	if declared != 1 {
		t.Fatalf("declare calls=%d want 1", declared)
	}

	// declare fails
	auditV0QueueDeclareHook = func(*amqp.Channel, string) (amqp.Queue, error) { return amqp.Queue{}, errors.New("declare failed") }
	PushMessage(map[string]string{"k": "v"})
}

func TestSetUpRabbitMq_error_doesNotSetGlobals(t *testing.T) {
	prevDial := auditV0DialHook
	prevAfter := auditV0AfterHook
	prevMax := auditV0MaxTrialsForTest
	t.Cleanup(func() {
		auditV0DialHook = prevDial
		auditV0AfterHook = prevAfter
		auditV0MaxTrialsForTest = prevMax
		Conn = nil
		Channel = nil
		Que = nil
	})

	auditV0DialHook = func(string, amqp.Config) (*amqp.Connection, error) { return nil, errors.New("dial failed") }
	auditV0AfterHook = func(time.Duration) <-chan time.Time { ch := make(chan time.Time); close(ch); return ch }
	auditV0MaxTrialsForTest = 1

	r := SetUpRabbitMq("h", "5672", "v", "u", "p", "q", "app")
	if r.Initialized {
		t.Fatal("expected not initialized")
	}
	if Conn != nil || Channel != nil || Que != nil {
		t.Fatalf("globals should remain nil: Conn=%v Channel=%v Que=%v", Conn, Channel, Que)
	}
}

func TestStartRQConnection_channelError_returnsWhenTestFlagSet(t *testing.T) {
	r := &RMqAutoConnect{uriConnection: "amqp://x"}
	prevDial := auditV0DialHook
	prevAfter := auditV0AfterHook
	prevCh := auditV0ChannelHook
	prevCloseCh := auditV0ChannelCloseHook
	prevCloseConn := auditV0ConnCloseHook
	prevMax := auditV0MaxTrialsForTest
	prevFlag := auditV0ReturnChannelErrorForTest
	t.Cleanup(func() {
		auditV0DialHook = prevDial
		auditV0AfterHook = prevAfter
		auditV0ChannelHook = prevCh
		auditV0ChannelCloseHook = prevCloseCh
		auditV0ConnCloseHook = prevCloseConn
		auditV0MaxTrialsForTest = prevMax
		auditV0ReturnChannelErrorForTest = prevFlag
	})

	auditV0DialHook = func(string, amqp.Config) (*amqp.Connection, error) { return &amqp.Connection{}, nil }
	auditV0AfterHook = func(time.Duration) <-chan time.Time { ch := make(chan time.Time); close(ch); return ch }
	auditV0ChannelHook = func(*amqp.Connection) (*amqp.Channel, error) { return nil, errors.New("channel failed") }
	auditV0ChannelCloseHook = func(*amqp.Channel) error { return nil }
	auditV0ConnCloseHook = func(*amqp.Connection) error { return nil }
	auditV0ReturnChannelErrorForTest = true
	auditV0MaxTrialsForTest = 1

	_, _, err := r.startRQConnection()
	if err == nil {
		t.Fatal("expected channel error")
	}
}
