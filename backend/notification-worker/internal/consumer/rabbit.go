// Package consumer wires the RabbitMQ AMQP channel + queue + dispatch loop.
//
// One queue (`RABBITMQ_NOTIFICATION_QUEUE`) is bound to the topic exchange
// (`parking.events`) with a catch-all routing key (`#`). Each message
// is deserialized into a `pkg/events.Envelope` and passed to the worker.
package consumer

import (
	"context"
	"fmt"
	"log"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"

	pkgEvents "github.com/parking-portal/backend/pkg/events"
)

// Consumer is a single AMQP connection + channel.
type Consumer struct {
	url      string
	exchange string
	queue    string
	conn     *amqp.Connection
	ch       *amqp.Channel

	mu      sync.Mutex
	closed  bool
	stopped chan struct{}
}

// New connects to RabbitMQ, declares the exchange (idempotent) and the
// queue (idempotent), and binds the queue to the exchange with `#` so
// all routing keys are captured.
func New(url, exchange, queue string) (*Consumer, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("channel: %w", err)
	}
	// QoS: process one at a time per consumer (worker Concurrency handled
	// at the worker-pool level).
	if err := ch.Qos(8, 0, false); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("qos: %w", err)
	}
	if err := ch.ExchangeDeclare(exchange, "topic", true, false, false, false, nil); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("exchange declare: %w", err)
	}
	if _, err := ch.QueueDeclare(queue, true, false, false, false, nil); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("queue declare: %w", err)
	}
	if err := ch.QueueBind(queue, "#", exchange, false, nil); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("queue bind: %w", err)
	}
	return &Consumer{
		url: url, exchange: exchange, queue: queue,
		conn: conn, ch: ch, stopped: make(chan struct{}),
	}, nil
}

// Handler processes one envelope. Returns an error to NACK the message
// (it will be re-queued) or nil to ACK.
type Handler func(ctx context.Context, env *pkgEvents.Envelope) error

// Run starts consuming. It blocks until ctx is cancelled or the channel
// closes. Returns when all in-flight handlers complete.
func (c *Consumer) Run(ctx context.Context, h Handler) error {
	deliveries, err := c.ch.Consume(c.queue, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consume: %w", err)
	}
	var wg sync.WaitGroup
	go func() {
		<-ctx.Done()
		c.mu.Lock()
		if !c.closed {
			c.closed = true
			_ = c.ch.Cancel("", false)
		}
		c.mu.Unlock()
	}()
	for d := range deliveries {
		wg.Add(1)
		go func(d amqp.Delivery) {
			defer wg.Done()
			env, err := pkgEvents.DecodeEnvelope(d.Body)
			if err != nil {
				log.Printf("[worker] bad envelope: %v", err)
				_ = d.Nack(false, false) // drop
				return
			}
			if err := h(ctx, env); err != nil {
				log.Printf("[worker] handler error: %v (requeue)", err)
				_ = d.Nack(false, true) // requeue
				return
			}
			_ = d.Ack(false)
		}(d)
	}
	wg.Wait()
	close(c.stopped)
	return nil
}

// Close cleans up the channel and connection.
func (c *Consumer) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.mu.Unlock()
	if c.ch != nil {
		_ = c.ch.Close()
	}
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
