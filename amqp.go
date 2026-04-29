package paycloudhelper

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/PayCloud-ID/paycloudhelper/phhelper"
)

// Push retry and timeout configuration.
// Package-level vars allow consumer services to override before calling SetUpRabbitMq.
var (
	PushMaxRetries = 3                // max retry attempts for Push()
	PushTimeout    = 15 * time.Second // total timeout for a single Push() call
)

// AmqpClient is the base struct for handling connection recovery, consumption and
// publishing. Note that this struct has an internal mutex to safeguard against
// data races. As you develop and iterate over this example, you may need to add
// further locks, or safeguards, to keep your application safe from data races
type AmqpClient struct {
	m               *sync.Mutex
	connName        string
	queueName       string
	consumerName    string
	infoLog         *log.Logger
	errLog          *log.Logger
	connection      *amqp.Connection
	channel         *amqp.Channel
	done            chan bool
	notifyConnClose chan *amqp.Error
	notifyChanClose chan *amqp.Error
	notifyConfirm   chan amqp.Confirmation
	isReady         bool
	amqpConfig      *amqp.Config
}

func (c *AmqpClient) ConnName() string {
	if c.connName == "" {
		c.connName = "amqp-" + phhelper.GetAppName()
	}
	return c.connName
}

func (c *AmqpClient) AmqpConfig() amqp.Config {
	if c.amqpConfig == nil {
		c.amqpConfig = defaultAmqpConfig()
	}

	if c.connName != "" {
		c.amqpConfig.Properties["connection_name"] = c.connName
	}

	return *c.amqpConfig
}

func (c *AmqpClient) SetAmqpConfig(amqpConfig *amqp.Config) {
	c.amqpConfig = amqpConfig
}

func (c *AmqpClient) Channel() *amqp.Channel {
	return c.channel
}

func (c *AmqpClient) InfoLog() *log.Logger {
	return c.infoLog
}

func (c *AmqpClient) ErrLog() *log.Logger {
	return c.errLog
}

const (
	// When reconnecting to the server after connection failure
	reconnectDelay = 5 * time.Second

	// When setting up the channel after a channel exception
	reInitDelay = 2 * time.Second

	// When resending messages the server didn't confirm
	resendDelay = 5 * time.Second
)

var (
	errNotConnected  = errors.New("not connected to a server")
	errAlreadyClosed = errors.New("already closed: not connected to the server")
	errShutdown      = errors.New("client is shutting down")
)

func defaultAmqpClient() *AmqpClient {
	return &AmqpClient{
		m:       &sync.Mutex{},
		infoLog: log.New(os.Stdout, "[INFO] ", log.LstdFlags|log.Lmsgprefix),
		errLog:  log.New(os.Stderr, "[ERROR] ", log.LstdFlags|log.Lmsgprefix),
		done:    make(chan bool),
	}
}

func defaultAmqpConfig() *amqp.Config {
	return &amqp.Config{
		Properties: amqp.Table{
			"connection_name": fmt.Sprintf("amqp-%s", phhelper.GetAppName()),
		},
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		Heartbeat:       time.Duration(5) * time.Second, // keep a live
	}
}

// NewAmqp creates a new consumer state instance, and automatically
// attempts to connect to the server.
func NewAmqp(addr string, c *AmqpClient) {
	if c != nil {
		if c.amqpConfig == nil {
			amqpConfig := defaultAmqpConfig()
			c.SetAmqpConfig(amqpConfig)
		}

		go c.handleReconnect(addr)
	}
}

// NewAmqpClient creates a new consumer state instance, and automatically
// attempts to connect to the server.
func NewAmqpClient(queueName, connName, addr string, config *amqp.Config) *AmqpClient {
	client := defaultAmqpClient()
	client.queueName = queueName
	client.connName = connName

	if config != nil {
		client.SetAmqpConfig(config)
	}

	go client.handleReconnect(addr)
	return client
}

// handleReconnect will wait for a connection error on
// notifyConnClose, and then continuously attempt to reconnect.
func (c *AmqpClient) handleReconnect(addr string) {
	for {
		c.m.Lock()
		c.isReady = false
		c.m.Unlock()

		c.infoLog.Println("[AMQP] attempting to connect")

		conn, err := c.connect(addr)

		if err != nil {
			c.errLog.Println("[AMQP] failed to connect. Retrying...")

			select {
			case <-c.done:
				return
			case <-time.After(reconnectDelay):
			}
			continue
		}

		if done := c.handleReInit(conn); done {
			break
		}
	}
}

// connect will create a new AMQP connection
func (c *AmqpClient) connect(addr string) (*amqp.Connection, error) {
	conn, err := amqp.DialConfig(addr, c.AmqpConfig())

	if err != nil {
		return nil, err
	}

	c.changeConnection(conn)
	c.infoLog.Println("[AMQP] connected")
	return conn, nil
}

// handleReInit will wait for a channel error
// and then continuously attempt to re-initialize both channels
func (c *AmqpClient) handleReInit(conn *amqp.Connection) bool {
	for {
		c.m.Lock()
		c.isReady = false
		c.m.Unlock()

		err := c.init(conn)
		if err != nil {
			c.errLog.Println("[AMQP] failed to initialize channel, retrying...")

			select {
			case <-c.done:
				return true
			case <-c.notifyConnClose:
				c.infoLog.Println("[AMQP] connection closed, reconnecting...")
				return false
			case <-time.After(reInitDelay):
			}
			continue
		}

		select {
		case <-c.done:
			return true
		case <-c.notifyConnClose:
			c.infoLog.Println("[AMQP] connection closed, reconnecting...")
			return false
		case <-c.notifyChanClose:
			c.infoLog.Println("[AMQP] channel closed, re-running init...")
		}
	}
}

func (c *AmqpClient) checkIfQueueExists(ch *amqp.Channel) (bool, error) {
	if ch == nil {
		return false, errors.New("[AMQP] failed to open channel")
	}

	_, err := ch.QueueDeclarePassive(
		c.queueName,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)

	if err != nil {
		// queue does not exists
		c.errLog.Printf("[AMQP] ERR queue does not exists queue=%s\n", c.queueName)
		return false, err
	}

	return true, nil
}

// init will initialize channel & declare queue
func (c *AmqpClient) init(conn *amqp.Connection) error {
	ch, err := conn.Channel()
	if err != nil {
		return err
	}

	err = ch.Confirm(false)
	if err != nil {
		return err
	}

	if ex, errEx := c.checkIfQueueExists(ch); !ex || errEx != nil {
		c.infoLog.Printf("[AMQP] queue does not exist, declaring queue=%s\n", c.queueName)

		// QueueDeclarePassive causes the broker to close the channel on failure
		// (AMQP 404/channel-error). Open a fresh channel before declaring the queue.
		ch, err = conn.Channel()
		if err != nil {
			return err
		}
		err = ch.Confirm(false)
		if err != nil {
			return err
		}

		_, err = ch.QueueDeclare(
			c.queueName,
			true,  // Durable
			false, // Delete when unused
			false, // Exclusive
			false, // No-wait
			nil,   // Arguments
		)
		if err != nil {
			c.errLog.Printf("[AMQP] failed to declare queue=%s err=%v\n", c.queueName, err)
			return err
		}
	}

	c.changeChannel(ch)
	c.m.Lock()
	c.isReady = true
	c.m.Unlock()
	c.infoLog.Println("[AMQP] client init done")

	return nil
}

// changeConnection takes a new connection to the queue,
// and updates the close listener to reflect this.
func (c *AmqpClient) changeConnection(connection *amqp.Connection) {
	c.connection = connection
	c.notifyConnClose = make(chan *amqp.Error, 1)
	c.connection.NotifyClose(c.notifyConnClose)
}

// changeChannel takes a new channel to the queue,
// and updates the channel listeners to reflect this.
func (c *AmqpClient) changeChannel(channel *amqp.Channel) {
	c.channel = channel
	c.notifyChanClose = make(chan *amqp.Error, 1)
	c.notifyConfirm = make(chan amqp.Confirmation, 1)
	c.channel.NotifyClose(c.notifyChanClose)
	c.channel.NotifyPublish(c.notifyConfirm)
}

// Push will push data onto the queue, and wait for a confirmation.
// Retries up to PushMaxRetries times with a total timeout of PushTimeout.
// Returns an error if all retries are exhausted or the timeout is reached.
func (c *AmqpClient) Push(data []byte) error {
	c.m.Lock()
	if !c.isReady {
		c.m.Unlock()
		return errors.New("[AMQP] failed to push: not connected")
	}
	c.m.Unlock()

	deadline := time.After(PushTimeout)
	for attempt := 0; attempt < PushMaxRetries; attempt++ {
		err := c.UnsafePush(data)
		if err != nil {
			select {
			case <-c.done:
				return errShutdown
			case <-deadline:
				return fmt.Errorf("[AMQP] push timeout after %v: %w", PushTimeout, err)
			case <-time.After(resendDelay):
			}
			continue
		}
		select {
		case confirm := <-c.notifyConfirm:
			if confirm.Ack {
				return nil
			}
		case <-deadline:
			return fmt.Errorf("[AMQP] push confirmation timeout after %v", PushTimeout)
		case <-c.done:
			return errShutdown
		}
	}
	return fmt.Errorf("[AMQP] push failed after %d retries", PushMaxRetries)
}

// UnsafePush will push to the queue without checking for
// confirmation. It returns an error if it fails to connect.
// No guarantees are provided for whether the server will
// receive the message.
func (c *AmqpClient) UnsafePush(data []byte) error {
	return c.PushWithTTL(data, "60000")
}

// PushWithTTL pushes data to the queue with a configurable TTL.
// An empty ttl means the message never expires in the queue.
func (c *AmqpClient) PushWithTTL(data []byte, ttl string) error {
	c.m.Lock()
	if !c.isReady {
		c.m.Unlock()
		return errNotConnected
	}
	c.m.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pub := amqp.Publishing{
		ContentType: "application/json",
		Body:        data,
	}
	if ttl != "" {
		pub.Expiration = ttl
	}

	return c.channel.PublishWithContext(
		ctx,
		"",          // Exchange
		c.queueName, // Routing key
		false,       // Mandatory
		false,       // Immediate
		pub,
	)
}

// IsReady returns true if the AMQP client has an active connection and channel.
func (c *AmqpClient) IsReady() bool {
	c.m.Lock()
	defer c.m.Unlock()
	return c.isReady
}

// WaitForReady blocks until the client is ready or the timeout expires.
// Returns true if ready, false if timed out.
func (c *AmqpClient) WaitForReady(timeout time.Duration) bool {
	deadline := time.After(timeout)
	for {
		if c.IsReady() {
			return true
		}
		select {
		case <-deadline:
			return false
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// Consume will continuously put queue items on the channel.
// It is required to call delivery.Ack when it has been
// successfully processed, or delivery.Nack when it fails.
// Ignoring this will cause data to build up on the server.
func (c *AmqpClient) Consume() (<-chan amqp.Delivery, error) {
	c.m.Lock()
	if !c.isReady {
		c.m.Unlock()
		return nil, errNotConnected
	}
	c.m.Unlock()

	if err := c.channel.Qos(
		1,     // prefetchCount
		0,     // prefetchSize
		false, // global
	); err != nil {
		return nil, err
	}

	return c.channel.Consume(
		c.queueName,
		c.consumerName, // Consumer
		false,          // Auto-Ack
		false,          // Exclusive
		false,          // No-local
		false,          // No-Wait
		nil,            // Args
	)
}

// Close will cleanly shut down the channel and connection.
func (c *AmqpClient) Close() error {
	c.m.Lock()
	// we read and write isReady in two locations, so we grab the lock and hold onto
	// it until we are finished
	defer c.m.Unlock()

	if !c.isReady {
		return errAlreadyClosed
	}
	close(c.done)
	err := c.channel.Close()
	if err != nil {
		return err
	}
	err = c.connection.Close()
	if err != nil {
		return err
	}

	c.isReady = false
	return nil
}

// Cc For debug purpose
// TODO : debugging close channel
func (c *AmqpClient) Cc() error {
	_ = c.channel.Close()
	_ = c.connection.Close()
	return nil
}
