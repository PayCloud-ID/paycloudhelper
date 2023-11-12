package qoinhubpackageprivate

import (
	"bytes"
	"encoding/json"
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
var AppName *string

// service must call this func in main function
func SetUpRabbitMq(host, port, vhost, username, password, audittrailQue, appName string) RMqAutoConnect {
	rmq := new(RMqAutoConnect)

	// set connection to rabbit mq
	url := host + ":" + port + "/" + vhost
	log.Println("[AMQP] " + url)
	rmq.uriConnection = "amqp://" + username + ":" + password + "@" + url
	conn, ch, err := rmq.startRQConnection()
	if err != nil {
		log.Println("Error open connection to rabbit : ", err)
		log.Panicln(err)
	}

	// set global variable
	Conn = conn
	Channel = ch
	Que = &audittrailQue
	AppName = &appName

	return *rmq
}

// service must call this method in defer func
func (r *RMqAutoConnect) CloseConnection() {
	r.reset()
}

func (r *RMqAutoConnect) startRQConnection() (conn *amqp.Connection, ch *amqp.Channel, err error) {
	const (
		maxTrialSecond = 3 // 30 second
		maxTrialMinute = 5 // 10 minute
	)

	log.Println("open connection to rabbit mq ...")

	retry := 0
	for {
		retry++
		r.conn, err = amqp.Dial(r.uriConnection)
		if err != nil {
			// retry connect to rabbit by sleep time
			switch {
			case retry <= maxTrialSecond:
				log.Println("try to reconnect in 30 seconds ...")
				<-time.After(time.Duration(30) * time.Second)
			case retry <= maxTrialMinute:
				log.Println("try to reconnect in 10 minutes ...")
				<-time.After(time.Duration(10) * time.Minute)
			default:
				// send notif to sentry
			}
			continue
		}
		break
	}

	log.Println("connected to rabbit mq successfully")

	// keep a live
	r.conn.Config.Heartbeat = time.Duration(5) * time.Second

	//declare channel
	log.Println("open channel ...")

	r.ch, err = r.conn.Channel()
	if err != nil {
		r.reset()
		log.Panicln(err.Error())
	}

	log.Println("opening channel succeed")

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

// push message to audittrail queue
func PushMessage(data interface{}) {

	log.Println("Publish message async to queue " + *Que + " ...")

	msgBytes, err := jsonMarshalNoEsc(data)
	if err != nil {
		log.Println("Error convert data to byte : ", err)
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

		log.Println("Error declaring queue : ", err)
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

		log.Println("Error publish message to queue "+*Que+" :", err)
		return
	}

	log.Println("Publish message async to queue " + *Que + " successfully")
}

func jsonMarshalNoEsc(t interface{}) ([]byte, error) {
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(t)
	return buffer.Bytes(), err
}
