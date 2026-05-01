package phaudittrailv0

import (
	"crypto/tls"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/PayCloud-ID/paycloudhelper/phhelper"
	"github.com/PayCloud-ID/paycloudhelper/phlogger"
)

type RMqAutoConnect struct {
	conn          *amqp.Connection
	ch            *amqp.Channel
	uriConnection string
	Initialized   bool
}

var Conn *amqp.Connection
var Channel *amqp.Channel
var Que *string

// Test hooks (default to real implementations). These allow deterministic unit tests
// without a live RabbitMQ and without long sleeps.
var auditV0DialHook = amqp.DialConfig
var auditV0AfterHook = time.After
var auditV0ChannelHook = func(conn *amqp.Connection) (*amqp.Channel, error) { return conn.Channel() }
var auditV0ChannelCloseHook = func(ch *amqp.Channel) error { return ch.Close() }
var auditV0ConnCloseHook = func(conn *amqp.Connection) error { return conn.Close() }

var auditV0QueuePassiveHook = func(ch *amqp.Channel, name string) (amqp.Queue, error) {
	return ch.QueueDeclarePassive(name, true, false, false, false, nil)
}
var auditV0QueueDeclareHook = func(ch *amqp.Channel, name string) (amqp.Queue, error) {
	return ch.QueueDeclare(name, true, false, false, false, nil)
}
var auditV0PublishHook = func(ch *amqp.Channel, queueName string, body []byte) error {
	return ch.Publish("", queueName, false, false, amqp.Publishing{
		ContentType: "text/plain",
		Body:        body,
		Expiration:  "60000",
	})
}

// auditV0MaxTrialsForTest, when > 0, caps retry loops in startRQConnection (tests only).
var auditV0MaxTrialsForTest int

// SetUpRabbitMq service must call this func in main function
// NOTE : for audittrail purpose
func SetUpRabbitMq(host, port, vhost, username, password, auditTrailQue, appName string) RMqAutoConnect {
	rmq := new(RMqAutoConnect)

	// set connection to rabbit mq
	urlStr := host + ":" + port + "/" + vhost
	phlogger.LogI("%s init url=%s queue=%s", phhelper.BuildLogPrefix("SetUpRabbitMq"), urlStr, auditTrailQue)
	rmq.uriConnection = "amqp://" + username + ":" + password + "@" + urlStr
	conn, ch, err := rmq.startRQConnection()
	if err != nil {
		phlogger.LogE("%s open connection failed queue=%s err=%v", phhelper.BuildLogPrefix("SetUpRabbitMq"), auditTrailQue, err)
		return *rmq
	}

	// set global variable
	Conn = conn
	Channel = ch
	Que = &auditTrailQue
	if phhelper.GetAppName() == "" {
		phhelper.SetAppName(appName)
	}
	rmq.Initialized = true

	return *rmq
}

// CloseConnection service must call this method in defer func
func (r *RMqAutoConnect) CloseConnection() {
	r.reset()
}

func (r *RMqAutoConnect) startRQConnection() (conn *amqp.Connection, ch *amqp.Channel, err error) {
	const (
		maxTrialSecond = 3 // 30 second
		maxTrialMinute = 5 // 10 minute
	)

	phlogger.LogI("%s opening connection to rabbitmq", phhelper.BuildLogPrefix("startRQConnection"))
	cfg := amqp.Config{
		Properties: amqp.Table{
			"connection_name": "audit-trail-" + phhelper.GetAppName(),
		},
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		Heartbeat:       time.Duration(5) * time.Second, // keep a live
	}

	retry := 0
	for {
		retry++
		if auditV0MaxTrialsForTest > 0 && retry > auditV0MaxTrialsForTest {
			return nil, nil, err
		}
		r.conn, err = auditV0DialHook(r.uriConnection, cfg)
		if err != nil {
			// retry connect to rabbit by sleep time
			switch {
			case retry <= maxTrialSecond:
				phlogger.LogI("%s reconnect delay=30s", phhelper.BuildLogPrefix("startRQConnection"))
				<-auditV0AfterHook(time.Duration(30) * time.Second)
			case retry <= maxTrialMinute:
				phlogger.LogI("%s reconnect delay=10m", phhelper.BuildLogPrefix("startRQConnection"))
				<-auditV0AfterHook(time.Duration(10) * time.Minute)
			default:
				// send notif to sentry
			}
			continue
		}
		break
	}

	phlogger.LogI("%s connected to rabbitmq successfully", phhelper.BuildLogPrefix("startRQConnection"))

	//declare channel
	phlogger.LogI("%s opening channel", phhelper.BuildLogPrefix("startRQConnection"))

	r.ch, err = auditV0ChannelHook(r.conn)
	if err != nil {
		r.reset()
		return nil, nil, fmt.Errorf("audit trail channel: %w", err)
	}

	phlogger.LogI("%s channel opened successfully", phhelper.BuildLogPrefix("startRQConnection"))

	return r.conn, r.ch, nil
}

// set all memory to nil
func (r *RMqAutoConnect) reset() {
	Conn = nil
	Channel = nil
	Que = nil
	if r.ch != nil {
		_ = auditV0ChannelCloseHook(r.ch)
	}
	if r.conn != nil {
		_ = auditV0ConnCloseHook(r.conn)
	}
}

func checkIfQueueExists(channel *amqp.Channel, queueName string) (bool, error) {
	_, err := auditV0QueuePassiveHook(channel, queueName)

	if err != nil {
		// queue does not exists
		phlogger.LogE("%s queue does not exist queue=%s", phhelper.BuildLogPrefix("checkIfQueueExists"), queueName)
		return false, err
	}

	return true, nil
}

// PushMessage push message to audittrail queue
func PushMessage(data interface{}) {
	if Que == nil {
		phlogger.LogE("%s queue does not exist", phhelper.BuildLogPrefix("PushMessage"))
		// TODO : send sentry error
		return
	}

	msgBytes, err := phhelper.JsonMarshalNoEsc(data)
	if err != nil {
		phlogger.LogE("%s convert data to bytes failed err=%v", phhelper.BuildLogPrefix("PushMessage"), err)
		// TODO : send sentry error
		return
	}

	// declare que if does not exists
	if queueExists, _ := checkIfQueueExists(Channel, *Que); !queueExists {
		// declaring creates a queue if it doesn't already exist, or ensures that an existing queue matches the same parameters.
		_, err = auditV0QueueDeclareHook(Channel, *Que)
		if err != nil {
			// TODO : send sentry error
			phlogger.LogE("%s declaring queue failed err=%v", phhelper.BuildLogPrefix("PushMessage"), err)
			return
		}
	}

	err = auditV0PublishHook(Channel, *Que, msgBytes)

	if err != nil {
		// TODO : send sentry error
		phlogger.LogE("%s publish message failed queue=%s err=%v", phhelper.BuildLogPrefix("PushMessage"), *Que, err)
		return
	}

	phlogger.LogI("%s publish message async success queue=%s", phhelper.BuildLogPrefix("PushMessage"), *Que)
}
