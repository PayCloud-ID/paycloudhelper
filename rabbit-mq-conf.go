package paycloudhelper

import (
	"log"
	"time"

	"github.com/streadway/amqp"
)

type RMqAutoConnect struct {
	conn          *amqp.Connection
	ch            *amqp.Channel
	uriConnection string
}

var Conn *amqp.Connection
var Channel *amqp.Channel
var Que *string

// SetUpRabbitMq service must call this func in main function
// NOTE : for audittrail purpose
func SetUpRabbitMq(host, port, vhost, username, password, audittrailQue, appName string) RMqAutoConnect {
	rmq := new(RMqAutoConnect)

	// set connection to rabbit mq
	urlStr := host + ":" + port + "/" + vhost
	LogI("[AMQP] Init %s. queue: %s", urlStr, audittrailQue)
	rmq.uriConnection = "amqp://" + username + ":" + password + "@" + urlStr
	conn, ch, err := rmq.startRQConnection()
	if err != nil {
		LogE("[AMQP] %s ERR open connection to rabbit: %v", audittrailQue, err)
		log.Panicln(err)
	}

	// set global variable
	Conn = conn
	Channel = ch
	Que = &audittrailQue
	if GetAppName() == "" {
		SetAppName(appName)
	}

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

	LogI("[AMQP] open connection to rabbit mq ...")

	retry := 0
	for {
		retry++
		r.conn, err = amqp.Dial(r.uriConnection)
		if err != nil {
			// retry connect to rabbit by sleep time
			switch {
			case retry <= maxTrialSecond:
				LogI("[AMQP] try to reconnect in 30 seconds ...")
				<-time.After(time.Duration(30) * time.Second)
			case retry <= maxTrialMinute:
				LogI("[AMQP] try to reconnect in 10 minutes ...")
				<-time.After(time.Duration(10) * time.Minute)
			default:
				// send notif to sentry
			}
			continue
		}
		break
	}

	LogI("[AMQP] connected to rabbit mq successfully")

	// keep a live
	r.conn.Config.Heartbeat = time.Duration(5) * time.Second

	//declare channel
	LogI("[AMQP] open channel ...")

	r.ch, err = r.conn.Channel()
	if err != nil {
		r.reset()
		log.Panicln(err.Error())
	}

	LogI("[AMQP] opening channel succeed")

	return r.conn, r.ch, nil
}

// set all memory to nil
func (r *RMqAutoConnect) reset() {
	Conn = nil
	Channel = nil
	Que = nil

	r.ch.Close()
	r.conn.Close()
}

// PushMessage push message to audittrail queue
func PushMessage(data interface{}) {

	LogI("[AMQP] Publish message async to queue %s ...", *Que)

	msgBytes, err := jsonMarshalNoEsc(data)
	if err != nil {
		LogE("[AMQP] ERR convert data to byte : %v", err)
		// send sentry error

		return
	}

	// declare que
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
		// send sentry error

		LogE("[AMQP] ERR declaring queue : %v", err)
		return
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
		// send sentry error
		LogE("[AMQP] ERR publish message to queue %s %v", *Que, err)
		return
	}

	LogI("[AMQP] Publish message async to queue %s successfully", *Que)
}
