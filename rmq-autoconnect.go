package paycloudhelper

import (
	"context"
	"fmt"
	"log"
	"os"
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
	conn           *amqp.Connection
	ch             *amqp.Channel
	uriConnection  string
	notifCloseCh   chan *amqp.Error
	ctxReconnect   context.Context
	stopReconnect  context.CancelFunc
	rq             IRqAutoConnect // implement template pattern
	declaredQueues []string       // queues
}

func (r *rMqAutoConnect) reset() {
	r.ch.Close()
	r.conn.Close()
}

func (r *rMqAutoConnect) connect(uri string) (c *amqp.Connection, err error) {
	const (
		maxTrialSecond = 3 // 60 second
		maxTrialMinute = 7 // 10 minute
	)
	// connect to rabbit mq
	LogI("%s try connecting to rabbitmq", buildLogPrefix("RMqAutoConnect.connect"))
	trial := 0
	for {
		trial++
		cfg := amqp.Config{
			Properties: amqp.Table{
				"connection_name": os.Getenv("APP_NAME") + "03",
			},
		}
		r.conn, err = amqp.DialConfig(uri, cfg)
		if err != nil {
			LogE("%s connection failed err=%s", buildLogPrefix("RMqAutoConnect.connect"), err.Error())
			switch {
			case trial <= maxTrialSecond:
				LogI("%s reconnect delay=30s", buildLogPrefix("RMqAutoConnect.connect"))
				<-time.After(time.Duration(30) * time.Second)
			case trial <= maxTrialMinute:
				LogI("%s reconnect delay=10m", buildLogPrefix("RMqAutoConnect.connect"))
				<-time.After(time.Duration(10) * time.Minute)
			default:
				LogI("%s reconnect delay=1h", buildLogPrefix("RMqAutoConnect.connect"))
				<-time.After(time.Duration(1) * time.Hour)
			}
			continue
		}
		break
	}
	LogI("%s connected to rabbitmq successfully", buildLogPrefix("RMqAutoConnect.connect"))
	// keep a live
	r.conn.Config.Heartbeat = time.Duration(5) * time.Second
	//declare channel
	LogI("%s opening channel", buildLogPrefix("RMqAutoConnect.connect"))
	r.ch, err = r.conn.Channel()
	if err != nil {
		r.conn.Close()
		LogF("%s channel open failed err=%s", buildLogPrefix("RMqAutoConnect.connect"), err.Error())
	}
	LogI("%s channel opened successfully", buildLogPrefix("RMqAutoConnect.connect"))
	return r.conn, nil
}

func (r *rMqAutoConnect) DeclareQueues(queues ...string) (err error) {
	r.declaredQueues = queues
	//declare queues
	for _, queue := range queues {
		LogI("%s declare queue=%s", buildLogPrefix("RMqAutoConnect.DeclareQueues"), queue)
		_, err = r.ch.QueueDeclare(
			queue, //name
			//true,  //durable
			false, //durable
			false, //auto delte
			false, //exclusive
			false, //no wait
			func() (out amqp.Table) {
				return
			}(), //args
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
	r.reset()
}

func (r *rMqAutoConnect) GetRqChannel() *amqp.Channel {
	return r.ch
}

func (r *rMqAutoConnect) beforeReconnect() { // implement template pattern
	r.rq.beforeReconnect()
}

func (r *rMqAutoConnect) afterReconnect() { // implement template pattern
	r.rq.afterReconnect()
}

func (r *rMqAutoConnect) startConnection(username, password, host, port, vhost string) (err error) {
	// set uri parameter to connect to rabbit mq
	connection := fmt.Sprintf("amqp://%s:%s@%s:%s/%s", username, password, host, port, vhost)
	LogI("%s connection=%s", buildLogPrefix("RMqAutoConnect.startConnection"), connection)
	r.uriConnection = connection
	r.conn, err = r.connect(r.uriConnection)
	if err != nil {
		log.Panicln(err.Error())
	}
	// try to reconnect
	r.reconnect()

	return
}

func (r *rMqAutoConnect) getConnection() *amqp.Connection {
	return r.conn
}

func (r *rMqAutoConnect) reconnect() {
	LogI("%s auto reconnect started", buildLogPrefix("RMqAutoConnect.reconnect"))
	r.ctxReconnect, r.stopReconnect = context.WithCancel(context.Background()) // prepare context
	LogI("%s creating notify-close channel", buildLogPrefix("RMqAutoConnect.reconnect"))
	r.notifCloseCh = make(chan *amqp.Error)
	LogI("%s notify-close channel created", buildLogPrefix("RMqAutoConnect.reconnect"))
	go func() {
		for {
			LogI("%s waiting for connection close signal", buildLogPrefix("RMqAutoConnect.reconnect"))
			select {
			case <-r.ctxReconnect.Done():
				LogI("%s stop reconnect listener", buildLogPrefix("RMqAutoConnect.reconnect"))
				return
			case <-r.getConnection().NotifyClose(r.notifCloseCh):
				r.beforeReconnect()
				LogI("%s connection closed, reconnecting", buildLogPrefix("RMqAutoConnect.reconnect"))
				r.reset()
				r.connect(r.uriConnection)
				r.DeclareQueues(r.declaredQueues...)
				r.afterReconnect()
				r.notifCloseCh = make(chan *amqp.Error)
			}
		}
	}()
}
