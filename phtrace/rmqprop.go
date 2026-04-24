package phtrace

import (
	"context"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel/propagation"
)

// AMQPCarrier is a propagation.TextMapCarrier backed by AMQP message headers.
// Use it to inject and extract the W3C traceparent header on the AMQP hot
// path so traces flow seamlessly between publisher and consumer services.
//
// Usage on the publish side:
//
//	headers := amqp.Table{}
//	phtrace.InjectAMQP(ctx, headers)
//	ch.Publish(exchange, routingKey, false, false, amqp.Publishing{
//	    Headers: headers,
//	    Body:    body,
//	})
//
// Usage on the consume side:
//
//	ctx = phtrace.ExtractAMQP(ctx, delivery.Headers)
//	ctx, span := phtrace.Tracer("rmq-consumer").Start(ctx, "process_message")
//	defer span.End()
type AMQPCarrier struct {
	Headers amqp.Table
}

// NewAMQPCarrier wraps an amqp.Table so it can be used as a TextMapCarrier.
// It does not copy the table; mutations to the carrier mutate the table.
func NewAMQPCarrier(h amqp.Table) *AMQPCarrier {
	if h == nil {
		h = amqp.Table{}
	}
	return &AMQPCarrier{Headers: h}
}

// Get returns the value for the given header key, or "" when absent.
// W3C traceparent is commonly stored as a string, but AMQP allows typed
// values, so we normalize non-string values to their string form.
func (c *AMQPCarrier) Get(key string) string {
	if c == nil || c.Headers == nil {
		return ""
	}
	raw, ok := c.Headers[key]
	if !ok {
		return ""
	}
	switch v := raw.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return ""
	}
}

// Set writes a header value. An empty key is ignored per AMQP semantics.
func (c *AMQPCarrier) Set(key, value string) {
	if c == nil || key == "" {
		return
	}
	if c.Headers == nil {
		c.Headers = amqp.Table{}
	}
	c.Headers[key] = value
}

// Keys returns all header keys. Order is not guaranteed — callers that need
// stable ordering must sort the returned slice themselves.
func (c *AMQPCarrier) Keys() []string {
	if c == nil || c.Headers == nil {
		return nil
	}
	keys := make([]string, 0, len(c.Headers))
	for k := range c.Headers {
		keys = append(keys, k)
	}
	return keys
}

// InjectAMQP writes the current span context from ctx into the provided AMQP
// headers using the globally registered propagator. Safe when headers is nil —
// InjectAMQP will not allocate a new Table in that case (caller loses the
// traceparent). Prefer passing a non-nil amqp.Table so propagation actually
// works. Returns the (possibly newly allocated) headers for ergonomic call
// sites: `headers = phtrace.InjectAMQP(ctx, headers)`.
func InjectAMQP(ctx context.Context, headers amqp.Table) amqp.Table {
	if ctx == nil {
		return headers
	}
	if headers == nil {
		headers = amqp.Table{}
	}
	Propagator().Inject(ctx, NewAMQPCarrier(headers))
	return headers
}

// ExtractAMQP returns a new context carrying the span context extracted from
// the given AMQP headers. When headers is nil or does not contain a traceparent,
// it returns ctx unchanged (any new span will become a root span).
func ExtractAMQP(ctx context.Context, headers amqp.Table) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if headers == nil {
		return ctx
	}
	return Propagator().Extract(ctx, NewAMQPCarrier(headers))
}

// compile-time assertion that *AMQPCarrier satisfies propagation.TextMapCarrier.
var _ propagation.TextMapCarrier = (*AMQPCarrier)(nil)
