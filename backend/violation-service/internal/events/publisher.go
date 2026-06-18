// Package events publishes domain events to RabbitMQ. Best-effort: if the
// broker is down, the call returns an error and the calling service decides
// what to do (usually: log and continue, per ADR-011).
package events

import (
	"context"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"

	ev "github.com/parking-portal/backend/pkg/events"
)

// Publisher wraps an amqp channel and the exchange name.
type Publisher struct {
	ch       *amqp.Channel
	exchange string
	log      *zap.Logger
}

// NewPublisher dials RabbitMQ and declares the exchange.
func NewPublisher(amqpURL, exchange string, log *zap.Logger) (*Publisher, error) {
	conn, err := amqp.Dial(amqpURL)
	if err != nil {
		return nil, err
	}
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, err
	}
	if err := ch.ExchangeDeclare(exchange, "topic", true, false, false, false, nil); err != nil {
		ch.Close()
		conn.Close()
		return nil, err
	}
	log.Info("rabbitmq publisher ready", zap.String("exchange", exchange))
	return &Publisher{ch: ch, exchange: exchange, log: log}, nil
}

// Close shuts down the channel (and the underlying connection, indirectly).
func (p *Publisher) Close() {
	if p.ch != nil {
		_ = p.ch.Close()
	}
}

// publish sends one event. Errors are returned for the caller to log.
func (p *Publisher) publish(ctx context.Context, routingKey string, payload any) error {
	env := ev.New(routingKey, payload)
	body, _ := jsonMarshal(env)
	return p.ch.PublishWithContext(ctx, p.exchange, routingKey, false, false,
		amqp.Publishing{ContentType: "application/json", Body: body})
}

// PublishViolationCreated is a thin wrapper for the violation-created event.
func (p *Publisher) PublishViolationCreated(ctx context.Context, payload any) error {
	return p.publish(ctx, ev.RoutingViolationCreated, payload)
}

// PublishInvoiceCreated is a thin wrapper for the invoice-created event.
func (p *Publisher) PublishInvoiceCreated(ctx context.Context, payload any) error {
	return p.publish(ctx, ev.RoutingInvoiceCreated, payload)
}

// PublishPaymentSucceeded is used by the payment service (not violation-svc).
func (p *Publisher) PublishPaymentSucceeded(ctx context.Context, payload any) error {
	return p.publish(ctx, ev.RoutingPaymentSucceeded, payload)
}

// PublishPaymentFailed is used by the payment service.
func (p *Publisher) PublishPaymentFailed(ctx context.Context, payload any) error {
	return p.publish(ctx, ev.RoutingPaymentFailed, payload)
}

// PublishRulePublished is used by the rule publish flow.
func (p *Publisher) PublishRulePublished(ctx context.Context, payload any) error {
	return p.publish(ctx, ev.RoutingRulePublished, payload)
}
