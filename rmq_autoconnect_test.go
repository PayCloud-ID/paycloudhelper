package paycloudhelper

import (
	"errors"
	"strings"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/PayCloud-ID/paycloudhelper/phlogger"
)

type stubRqAutoConnect struct {
	beforeN int
	afterN  int
}

func (s *stubRqAutoConnect) StartConnection(string, string, string, string, string) (*amqp.Connection, error) {
	return nil, nil
}
func (s *stubRqAutoConnect) DeclareQueues(...string) error { return nil }
func (s *stubRqAutoConnect) GetRqChannel() *amqp.Channel   { return nil }
func (s *stubRqAutoConnect) Stop()                         {}
func (s *stubRqAutoConnect) beforeReconnect()              { s.beforeN++ }
func (s *stubRqAutoConnect) afterReconnect()               { s.afterN++ }

func TestRMqAutoConnect_connect_respectsMaxTrialsForTest(t *testing.T) {
	r := &rMqAutoConnect{}
	prevDial := rmqDialHook
	prevAfter := rmqAfterHook
	prevChannel := rmqChannelHook
	prevMax := rmqConnectMaxTrialsForTest.Load()
	t.Cleanup(func() {
		rmqDialHook = prevDial
		rmqAfterHook = prevAfter
		rmqChannelHook = prevChannel
		rmqConnectMaxTrialsForTest.Store(prevMax)
	})

	var dials int
	rmqDialHook = func(string, amqp.Config) (*amqp.Connection, error) {
		dials++
		return nil, errors.New("dial failed")
	}
	rmqChannelHook = func(*amqp.Connection) (*amqp.Channel, error) {
		return &amqp.Channel{}, nil
	}
	rmqAfterHook = func(time.Duration) <-chan time.Time {
		ch := make(chan time.Time)
		close(ch)
		return ch
	}
	rmqConnectMaxTrialsForTest.Store(3)

	_, err := r.connect("amqp://u:p@host:5672/v")
	if err == nil {
		t.Fatal("expected error")
	}
	if dials != 3 {
		t.Fatalf("dials=%d want 3", dials)
	}
}

func TestRMqAutoConnect_stop_recoversOnNilConnAndChannel(t *testing.T) {
	r := &rMqAutoConnect{}
	// stop() wraps reset() with a recover; with nil conn/ch it must not panic.
	r.stop()
}

func TestRMqAutoConnect_connect_exercisesBackoffBranches(t *testing.T) {
	r := &rMqAutoConnect{}
	prevDial := rmqDialHook
	prevAfter := rmqAfterHook
	prevMax := rmqConnectMaxTrialsForTest.Load()
	t.Cleanup(func() {
		rmqDialHook = prevDial
		rmqAfterHook = prevAfter
		rmqConnectMaxTrialsForTest.Store(prevMax)
	})

	var dials int
	rmqDialHook = func(string, amqp.Config) (*amqp.Connection, error) {
		dials++
		return nil, errors.New("dial failed")
	}
	rmqAfterHook = func(time.Duration) <-chan time.Time {
		ch := make(chan time.Time)
		close(ch)
		return ch
	}

	// maxTrialSecond=3, maxTrialMinute=7; running 9 trials exercises:
	// - 30s branch (1-3), 10m branch (4-7), 1h branch (8+).
	rmqConnectMaxTrialsForTest.Store(9)
	_, err := r.connect("amqp://u:p@host:5672/v")
	if err == nil {
		t.Fatal("expected error")
	}
	if dials != 9 {
		t.Fatalf("dials=%d want 9", dials)
	}
}

func TestRMqAutoConnect_DeclareQueues_successAndError(t *testing.T) {
	r := &rMqAutoConnect{ch: &amqp.Channel{}}
	prev := rmqQueueDeclareHook
	t.Cleanup(func() { rmqQueueDeclareHook = prev })

	var calls int
	rmqQueueDeclareHook = func(*amqp.Channel, string, bool, bool, bool, bool, amqp.Table) (amqp.Queue, error) {
		calls++
		return amqp.Queue{}, nil
	}
	if err := r.DeclareQueues("q1", "q2"); err != nil {
		t.Fatalf("DeclareQueues: %v", err)
	}
	if calls != 2 {
		t.Fatalf("calls=%d want 2", calls)
	}

	rmqQueueDeclareHook = func(*amqp.Channel, string, bool, bool, bool, bool, amqp.Table) (amqp.Queue, error) {
		return amqp.Queue{}, errors.New("declare failed")
	}
	if err := r.DeclareQueues("q3"); err == nil {
		t.Fatal("expected error")
	}
}

func TestRMqAutoConnect_GetRqChannel_beforeAfter(t *testing.T) {
	r := &rMqAutoConnect{ch: &amqp.Channel{}}
	if r.GetRqChannel() == nil {
		t.Fatal("expected channel")
	}
	st := &stubRqAutoConnect{}
	r.rq = st
	r.beforeReconnect()
	r.afterReconnect()
	if st.beforeN != 1 || st.afterN != 1 {
		t.Fatalf("before=%d after=%d", st.beforeN, st.afterN)
	}
}

func TestRedactAMQPURIForLog(t *testing.T) {
	got := redactAMQPURIForLog("broker.example", "5672", "myvhost")
	want := "amqp://***:***@broker.example:5672/myvhost"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRMqAutoConnect_startConnection_logDoesNotLeakCredentials(t *testing.T) {
	phlogger.ClearLogHooks()
	var msgs []string
	phlogger.RegisterLogHook("info", func(_, msg string) {
		msgs = append(msgs, msg)
	})
	t.Cleanup(phlogger.ClearLogHooks)

	r := &rMqAutoConnect{}
	prevDial := rmqDialHook
	prevAfter := rmqAfterHook
	prevChannel := rmqChannelHook
	prevNotify := rmqNotifyCloseHook
	prevCloseCh := rmqChannelCloseHook
	prevCloseConn := rmqConnCloseHook
	t.Cleanup(func() {
		rmqDialHook = prevDial
		rmqAfterHook = prevAfter
		rmqChannelHook = prevChannel
		rmqNotifyCloseHook = prevNotify
		rmqChannelCloseHook = prevCloseCh
		rmqConnCloseHook = prevCloseConn
	})

	rmqDialHook = func(string, amqp.Config) (*amqp.Connection, error) { return &amqp.Connection{}, nil }
	rmqAfterHook = func(time.Duration) <-chan time.Time { ch := make(chan time.Time); close(ch); return ch }
	rmqChannelHook = func(*amqp.Connection) (*amqp.Channel, error) { return &amqp.Channel{}, nil }
	rmqChannelCloseHook = func(*amqp.Channel) error { return nil }
	rmqConnCloseHook = func(*amqp.Connection) error { return nil }

	notify := make(chan *amqp.Error, 1)
	rmqNotifyCloseHook = func(*amqp.Connection, chan *amqp.Error) <-chan *amqp.Error { return notify }

	st := &stubRqAutoConnect{}
	r.rq = st
	secretPass := "secret-password-do-not-log"
	secretUser := "svc_audit_user"
	if err := r.startConnection(secretUser, secretPass, "h.example", "5672", "vh"); err != nil {
		t.Fatalf("startConnection: %v", err)
	}
	r.stop()

	for _, m := range msgs {
		if strings.Contains(m, secretPass) || strings.Contains(m, secretUser) {
			t.Fatalf("log message must not contain credentials: %q", m)
		}
	}
	var sawRedacted bool
	for _, m := range msgs {
		if strings.Contains(m, "***:***@") && strings.Contains(m, "h.example") {
			sawRedacted = true
			break
		}
	}
	if !sawRedacted {
		t.Fatalf("expected redacted connection log; msgs=%v", msgs)
	}
}

func TestRMqAutoConnect_startConnection_setsURIAndStartsReconnectLoop(t *testing.T) {
	r := &rMqAutoConnect{}
	prevDial := rmqDialHook
	prevAfter := rmqAfterHook
	prevChannel := rmqChannelHook
	prevNotify := rmqNotifyCloseHook
	prevCloseCh := rmqChannelCloseHook
	prevCloseConn := rmqConnCloseHook
	t.Cleanup(func() {
		rmqDialHook = prevDial
		rmqAfterHook = prevAfter
		rmqChannelHook = prevChannel
		rmqNotifyCloseHook = prevNotify
		rmqChannelCloseHook = prevCloseCh
		rmqConnCloseHook = prevCloseConn
	})

	rmqDialHook = func(string, amqp.Config) (*amqp.Connection, error) { return &amqp.Connection{}, nil }
	rmqAfterHook = func(time.Duration) <-chan time.Time { ch := make(chan time.Time); close(ch); return ch }
	rmqChannelHook = func(*amqp.Connection) (*amqp.Channel, error) { return &amqp.Channel{}, nil }
	rmqChannelCloseHook = func(*amqp.Channel) error { return nil }
	rmqConnCloseHook = func(*amqp.Connection) error { return nil }

	notify := make(chan *amqp.Error, 1)
	rmqNotifyCloseHook = func(*amqp.Connection, chan *amqp.Error) <-chan *amqp.Error { return notify }

	st := &stubRqAutoConnect{}
	r.rq = st
	if err := r.startConnection("u", "p", "h", "5672", "v"); err != nil {
		t.Fatalf("startConnection: %v", err)
	}
	if r.uriConnection == "" {
		t.Fatal("expected uriConnection to be set")
	}

	notify <- &amqp.Error{Code: 320, Reason: "closed"}
	time.Sleep(5 * time.Millisecond)
	r.stop()

	if st.beforeN == 0 || st.afterN == 0 {
		t.Fatalf("expected before/after reconnect to run at least once, got before=%d after=%d", st.beforeN, st.afterN)
	}
}
