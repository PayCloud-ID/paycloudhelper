package paycloudhelper

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// IRqAutoConnect is interface defining method of rabbit mq auto connect
type IRqAutoConnect interface {
	StartConnection(username, password, host, port, vhost string) (c *amqp.Connection, err error)
	DeclareQueues(queues ...string) (err error)
	GetRqChannel() *amqp.Channel
	Stop()
	beforeReconnect() // implement template pattern
	afterReconnect()  // implement template pattern
}

type rMqAutoConnect struct {
	mu sync.Mutex

	conn           *amqp.Connection
	ch             *amqp.Channel
	uriConnection  string
	notifCloseCh   chan *amqp.Error
	ctxReconnect   context.Context
	stopReconnect  context.CancelFunc
	reconnectWg    sync.WaitGroup
	rq             IRqAutoConnect // implement template pattern
	declaredQueues []string       // queues
}

// rmqDialHook allows tests to stub amqp.DialConfig and avoid real network calls.
var rmqDialHook = amqp.DialConfig

// rmqAfterHook allows tests to stub time.After and avoid long sleeps.
var rmqAfterHook = time.After

// rmqConnectMaxTrialsForTest caps connect() retries when > 0 (tests only).
var rmqConnectMaxTrialsForTest atomic.Int32

// rmqChannelHook allows tests to stub conn.Channel().
var rmqChannelHook = func(conn *amqp.Connection) (*amqp.Channel, error) { return conn.Channel() }

// rmqNotifyCloseHook allows tests to stub conn.NotifyClose().
var rmqNotifyCloseHook = func(conn *amqp.Connection, c chan *amqp.Error) <-chan *amqp.Error { return conn.NotifyClose(c) }

// rmqQueueDeclareHook allows tests to stub ch.QueueDeclare().
var rmqQueueDeclareHook = func(ch *amqp.Channel, name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error) {
	return ch.QueueDeclare(name, durable, autoDelete, exclusive, noWait, args)
}

// rmqChannelCloseHook / rmqConnCloseHook allow tests to stub Close() calls inside resetLocked().
var rmqChannelCloseHook = func(ch *amqp.Channel) error { return ch.Close() }
var rmqConnCloseHook = func(conn *amqp.Connection) error { return conn.Close() }

func (r *rMqAutoConnect) resetLocked() {
	_ = rmqChannelCloseHook(r.ch)
	_ = rmqConnCloseHook(r.conn)
	r.ch = nil
	r.conn = nil
}

func (r *rMqAutoConnect) connect(uri string) (*amqp.Connection, error) {
	const (
		maxTrialSecond = 3 // 60 second
		maxTrialMinute = 7 // 10 minute
	)

	LogI("%s try connecting to rabbitmq", buildLogPrefix("RMqAutoConnect.connect"))

	trial := 0
	var lastErr error
	for {
		trial++
		if max := rmqConnectMaxTrialsForTest.Load(); max > 0 && int32(trial) > max {
			return nil, lastErr
		}

		cfg := amqp.Config{
			Properties: amqp.Table{
				"connection_name": os.Getenv("APP_NAME") + "03",
			},
		}

		newConn, err := rmqDialHook(uri, cfg)
		if err != nil {
			lastErr = err
			LogE("%s connection failed err=%s", buildLogPrefix("RMqAutoConnect.connect"), err.Error())

			switch {
			case trial <= maxTrialSecond:
				LogI("%s reconnect delay=30s", buildLogPrefix("RMqAutoConnect.connect"))
				<-rmqAfterHook(time.Duration(30) * time.Second)
			case trial <= maxTrialMinute:
				LogI("%s reconnect delay=10m", buildLogPrefix("RMqAutoConnect.connect"))
				<-rmqAfterHook(time.Duration(10) * time.Minute)
			default:
				LogI("%s reconnect delay=1h", buildLogPrefix("RMqAutoConnect.connect"))
				<-rmqAfterHook(time.Duration(1) * time.Hour)
			}
			continue
		}

		r.mu.Lock()
		r.conn = newConn
		r.mu.Unlock()

		newConn.Config.Heartbeat = time.Duration(5) * time.Second

		LogI("%s opening channel", buildLogPrefix("RMqAutoConnect.connect"))
		ch, err := rmqChannelHook(newConn)
		if err != nil {
			_ = newConn.Close()
			r.mu.Lock()
			r.conn = nil
			r.mu.Unlock()
			LogE("%s channel open failed err=%s", buildLogPrefix("RMqAutoConnect.connect"), err.Error())
			return nil, fmt.Errorf("rmq channel: %w", err)
		}

		r.mu.Lock()
		r.ch = ch
		r.mu.Unlock()

		LogI("%s channel opened successfully", buildLogPrefix("RMqAutoConnect.connect"))
		return newConn, nil
	}
}

func (r *rMqAutoConnect) DeclareQueues(queues ...string) (err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.declaredQueues = queues
	ch := r.ch
	if ch == nil {
		return errors.New("rmq: no channel")
	}

	for _, queue := range queues {
		LogI("%s declare queue=%s", buildLogPrefix("RMqAutoConnect.DeclareQueues"), queue)
		_, err = rmqQueueDeclareHook(
			ch,
			queue,
			false,
			false,
			false,
			false,
			func() (out amqp.Table) {
				return
			}(),
		)
		if err != nil {
			LogE("%s queue declare failed err=%s", buildLogPrefix("RMqAutoConnect.DeclareQueues"), err.Error())
			return
		}
		LogI("%s queue declared=%s", buildLogPrefix("RMqAutoConnect.DeclareQueues"), queue)
	}
	return
}

func (r *rMqAutoConnect) stop() {
	defer func() {
		if it := recover(); it != nil {
			LogI("%s panic recovered=%v", buildLogPrefix("RMqAutoConnect.stop"), it)
		}
	}()
	if r.stopReconnect != nil {
		r.stopReconnect()
	}
	r.reconnectWg.Wait()

	r.mu.Lock()
	defer r.mu.Unlock()
	r.resetLocked()
}

func (r *rMqAutoConnect) GetRqChannel() *amqp.Channel {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.ch
}

func (r *rMqAutoConnect) beforeReconnect() { // implement template pattern
	r.rq.beforeReconnect()
}

func (r *rMqAutoConnect) afterReconnect() { // implement template pattern
	r.rq.afterReconnect()
}

// redactAMQPURIForLog formats host/port/vhost for logs without credentials (SEC: avoid leaking secrets to log sinks).
func redactAMQPURIForLog(host, port, vhost string) string {
	return fmt.Sprintf("amqp://***:***@%s:%s/%s", host, port, vhost)
}

func (r *rMqAutoConnect) startConnection(username, password, host, port, vhost string) error {
	connection := fmt.Sprintf("amqp://%s:%s@%s:%s/%s", username, password, host, port, vhost)
	LogI("%s connection=%s", buildLogPrefix("RMqAutoConnect.startConnection"), redactAMQPURIForLog(host, port, vhost))

	r.mu.Lock()
	r.uriConnection = connection
	r.mu.Unlock()

	if _, err := r.connect(connection); err != nil {
		return err
	}
	r.reconnect()
	return nil
}

func (r *rMqAutoConnect) reconnect() {
	LogI("%s auto reconnect started", buildLogPrefix("RMqAutoConnect.reconnect"))
	r.ctxReconnect, r.stopReconnect = context.WithCancel(context.Background())

	r.mu.Lock()
	if r.notifCloseCh == nil {
		r.notifCloseCh = make(chan *amqp.Error, 1)
	}
	r.mu.Unlock()

	r.reconnectWg.Add(1)
	go func() {
		defer r.reconnectWg.Done()
		for {
			r.mu.Lock()
			conn := r.conn
			notifCh := r.notifCloseCh
			r.mu.Unlock()

			if conn == nil || notifCh == nil {
				select {
				case <-r.ctxReconnect.Done():
					LogI("%s stop reconnect listener", buildLogPrefix("RMqAutoConnect.reconnect"))
					return
				case <-time.After(50 * time.Millisecond):
				}
				continue
			}

			LogI("%s waiting for connection close signal", buildLogPrefix("RMqAutoConnect.reconnect"))
			select {
			case <-r.ctxReconnect.Done():
				LogI("%s stop reconnect listener", buildLogPrefix("RMqAutoConnect.reconnect"))
				return
			case <-rmqNotifyCloseHook(conn, notifCh):
				r.beforeReconnect()
				LogI("%s connection closed, reconnecting", buildLogPrefix("RMqAutoConnect.reconnect"))

				r.mu.Lock()
				r.resetLocked()
				r.mu.Unlock()

				if _, err := r.connect(r.uriConnection); err != nil {
					LogE("%s reconnect connect failed err=%v", buildLogPrefix("RMqAutoConnect.reconnect"), err)
					continue
				}
				if err := r.DeclareQueues(r.declaredQueues...); err != nil {
					LogE("%s reconnect declare queues err=%v", buildLogPrefix("RMqAutoConnect.reconnect"), err)
				}
				r.afterReconnect()

				r.mu.Lock()
				r.notifCloseCh = make(chan *amqp.Error, 1)
				r.mu.Unlock()
			}
		}
	}()
}
