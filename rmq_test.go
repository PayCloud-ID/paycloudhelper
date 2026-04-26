package paycloudhelper

import "testing"

func TestRabbitMQConnection_GlobalConn_ReadsEnv(t *testing.T) {
	t.Setenv("RQ_HOST", "h1")
	t.Setenv("RQ_PORT", "5672")
	t.Setenv("RQ_USERNAME", "u1")
	t.Setenv("RQ_PASSWORD", "p1")
	t.Setenv("RQ_VHOST", "/v")
	t.Setenv("RQ_QUEUE", "q-global")

	var c RabbitMQConnection
	c.GlobalConn()

	if c.Host != "h1" || c.Port != "5672" || c.Username != "u1" || c.Password != "p1" || c.VirtualHost != "/v" || c.QueueName != "q-global" {
		t.Fatalf("unexpected connection: %+v", c)
	}
}

func TestRabbitMQConnection_QueueConn_ReadsEnvAndQueue(t *testing.T) {
	t.Setenv("RQ_HOST", "h2")
	t.Setenv("RQ_PORT", "5673")
	t.Setenv("RQ_USERNAME", "u2")
	t.Setenv("RQ_PASSWORD", "p2")
	t.Setenv("RQ_VHOST", "v2")
	t.Setenv("RQ_QUEUE", "ignored-for-queue-conn")

	var c RabbitMQConnection
	c.QueueConn("my-queue")

	if c.Host != "h2" || c.QueueName != "my-queue" {
		t.Fatalf("unexpected connection: %+v", c)
	}
}
