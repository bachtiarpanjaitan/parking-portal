// Package events publishes payment events to RabbitMQ. Reuses the
// pkg/events envelope + routing keys. See .ai/NOTIFICATIONS.md.
package events

import (
	"context"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"

	pkgEvents "github.com/parking-portal/backend/pkg/events"
)

// Publisher wraps an amqp.Channel and the exchange name.
type Publisher struct {
	ch       *amqp.Channel
	exchange string
	log      *zap.Logger
}

// NewPublisher dials RabbitMQ and declares the exchange.
// Returns nil if amqpURL is empty (caller can fall back to log-only events).
func NewPublisher(amqpURL, exchange string, log *zap.Logger) (*Publisher, error) {
	conn, err := amqp.Dial(amqpURL)
	if err != nil {
		return nil, err
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err := ch.ExchangeDeclare(exchange, "topic", true, false, false, false, nil); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, err
	}
	return &Publisher{ch: ch, exchange: exchange, log: log}, nil
}

func (p *Publisher) Close() {
	if p != nil && p.ch != nil {
		_ = p.ch.Close()
	}
}

func (p *Publisher) publish(ctx context.Context, routingKey string, payload any) error {
	if p == nil || p.ch == nil {
		return nil
	}
	env := pkgEvents.New(routingKey, payload)
	body, _ := pkgMarshal(env)
	return p.ch.PublishWithContext(ctx, p.exchange, routingKey, false, false,
		amqp.Publishing{ContentType: "application/json", Body: body})
}

func (p *Publisher) PublishPaymentSucceeded(ctx context.Context, payload any) error {
	return p.publish(ctx, pkgEvents.RoutingPaymentSucceeded, payload)
}

func (p *Publisher) PublishPaymentFailed(ctx context.Context, payload any) error {
	return p.publish(ctx, pkgEvents.RoutingPaymentFailed, payload)
}

// small indirection to avoid importing encoding/json in many places
func pkgMarshal(v any) ([]byte, error) { return jsonMarshal(v) }
