package phaudittrailv0

import (
	"crypto/tls"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"bitbucket.org/paycloudid/paycloudhelper/phhelper"
	"bitbucket.org/paycloudid/paycloudhelper/phlogger"
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
		r.conn, err = amqp.DialConfig(r.uriConnection, cfg)
		if err != nil {
			// retry connect to rabbit by sleep time
			switch {
			case retry <= maxTrialSecond:
				phlogger.LogI("%s reconnect delay=30s", phhelper.BuildLogPrefix("startRQConnection"))
				<-time.After(time.Duration(30) * time.Second)
			case retry <= maxTrialMinute:
				phlogger.LogI("%s reconnect delay=10m", phhelper.BuildLogPrefix("startRQConnection"))
				<-time.After(time.Duration(10) * time.Minute)
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

	r.ch, err = r.conn.Channel()
	if err != nil {
		r.reset()
		log.Panicln(err.Error())
	}

	phlogger.LogI("%s channel opened successfully", phhelper.BuildLogPrefix("startRQConnection"))

	return r.conn, r.ch, nil
}

// set all memory to nil
func (r *RMqAutoConnect) reset() {
	Conn = nil
	Channel = nil
	Que = nil

	if err := r.ch.Close(); err != nil {
		return
	}
	if err := r.conn.Close(); err != nil {
		return
	}
}

func checkIfQueueExists(channel *amqp.Channel, queueName string) (bool, error) {
	_, err := channel.QueueDeclarePassive(
		queueName,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)

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
		_, err = Channel.QueueDeclare(
			*Que,
			true,
			false,
			false,
			false,
			nil,
		)
		if err != nil {
			// TODO : send sentry error
			phlogger.LogE("%s declaring queue failed err=%v", phhelper.BuildLogPrefix("PushMessage"), err)
			return
		}
	}

	err = Channel.Publish(
		"",
		*Que,
		false,
		false,
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        msgBytes,
			Expiration:  "60000",
		},
	)

	if err != nil {
		// TODO : send sentry error
		phlogger.LogE("%s publish message failed queue=%s err=%v", phhelper.BuildLogPrefix("PushMessage"), *Que, err)
		return
	}

	phlogger.LogI("%s publish message async success queue=%s", phhelper.BuildLogPrefix("PushMessage"), *Que)
}
