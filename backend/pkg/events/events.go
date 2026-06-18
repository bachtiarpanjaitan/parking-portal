// Package events defines the standard event envelope and payload types
// published to RabbitMQ. See .ai/NOTIFICATIONS.md.
package events

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Envelope is the JSON shape for every event on the bus.
type Envelope struct {
	EventID    uuid.UUID `json:"event_id"`
	EventType  string    `json:"event_type"`
	OccurredAt time.Time `json:"occurred_at"`
	Payload    any       `json:"payload"`
}

// New builds an Envelope with a fresh UUID and current UTC timestamp.
func New(eventType string, payload any) Envelope {
	return Envelope{
		EventID:    uuid.New(),
		EventType:  eventType,
		OccurredAt: time.Now().UTC(),
		Payload:    payload,
	}
}

// DecodeEnvelope parses a raw AMQP body into an Envelope.
func DecodeEnvelope(body []byte) (*Envelope, error) {
	var e Envelope
	if err := json.Unmarshal(body, &e); err != nil {
		return nil, err
	}
	return &e, nil
}

// PayloadMap returns env.Payload as a map[string]any, or an empty map if
// it isn't one. Use this in handlers that need to inspect individual
// fields without writing type assertions everywhere.
func (e *Envelope) PayloadMap() map[string]any {
	if m, ok := e.Payload.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

// Routing keys (see .ai/NOTIFICATIONS.md).
const (
	RoutingViolationCreated = "violation.created"
	RoutingInvoiceCreated   = "invoice.created"
	RoutingPaymentSucceeded = "payment.succeeded"
	RoutingPaymentFailed    = "payment.failed"
	RoutingRulePublished    = "rule.published"
)

// Exchange name (see .ai/ENVVAR_CONFIG.md → RABBITMQ_EXCHANGE).
const Exchange = "parking.events"

// Payload types — all fields are JSON-serializable.

type ViolationCreated struct {
	ViolationID    uuid.UUID `json:"violation_id"`
	MemberID       uuid.UUID `json:"member_id"`
	LicensePlate   string    `json:"license_plate"`
	ViolationType  string    `json:"violation_type"`
	RuleVersionID  uuid.UUID `json:"rule_version_id"`
	RuleVersionNum int       `json:"rule_version_number"`
	Location       string    `json:"location"`
	ViolationTS    time.Time `json:"violation_timestamp"`
	PhotoURL       string    `json:"photo_url"`
	FineAmount     int64     `json:"fine_amount"` // IDR, no subunits
}

type InvoiceCreated struct {
	InvoiceID   uuid.UUID `json:"invoice_id"`
	ViolationID uuid.UUID `json:"violation_id"`
	MemberID    uuid.UUID `json:"member_id"`
	Amount      int64     `json:"amount"`
}

type PaymentSucceeded struct {
	PaymentID     uuid.UUID `json:"payment_id"`
	InvoiceID     uuid.UUID `json:"invoice_id"`
	TransactionID string    `json:"transaction_id"`
	Amount        int64     `json:"amount"`
	Scenario      string    `json:"scenario"`
}

type PaymentFailed struct {
	PaymentID     uuid.UUID `json:"payment_id"`
	InvoiceID     uuid.UUID `json:"invoice_id"`
	TransactionID string    `json:"transaction_id"`
	Amount        int64     `json:"amount"`
	Scenario      string    `json:"scenario"`
}

type RulePublished struct {
	RuleVersionID  uuid.UUID `json:"rule_version_id"`
	VersionNumber  int       `json:"version_number"`
	PublishedBy    uuid.UUID `json:"published_by"`
	PublishedAtUTC time.Time `json:"published_at"`
}
