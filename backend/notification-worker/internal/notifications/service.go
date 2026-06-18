// Package notifications — service layer.
//
// Handle processes one event envelope. It is idempotent: a duplicate
// event_id is detected via the `processed_events` table and the handler
// becomes a no-op.
package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"

	pkgEvents "github.com/parking-portal/backend/pkg/events"
)

// Service is the worker-side business logic.
type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service { return &Service{repo: repo} }

// Handle processes a single envelope. Returns nil on success or duplicate;
// returns an error if the event should be re-queued.
func (s *Service) Handle(ctx context.Context, env *pkgEvents.Envelope) error {
	// 1. Idempotency check
	first, err := s.repo.MarkProcessed(ctx, env.EventID)
	if err != nil {
		return fmt.Errorf("mark processed: %w", err)
	}
	if !first {
		log.Printf("[worker] duplicate event %s, skipping", env.EventID)
		return nil
	}

	// 2. Build a Notification row
	userID, title, message := buildNotification(env)
	now := time.Now().UTC()
	n := &Notification{
		ID:        uuid.New(),
		UserID:    userID,
		EventType: env.EventType,
		Title:     title,
		Message:   message,
		CreatedAt: now,
	}
	if err := s.repo.InsertNotification(ctx, n); err != nil {
		return fmt.Errorf("insert notification: %w", err)
	}
	log.Printf("[worker] processed %s (event=%s)", env.EventType, env.EventID)
	return nil
}

// buildNotification inspects the event payload and produces a human-readable
// title + message + optional user_id. Per ADR-008 the worker never modifies
// business data; it only produces a notification record.
func buildNotification(env *pkgEvents.Envelope) (*uuid.UUID, string, string) {
	p := env.PayloadMap()
	switch env.EventType {
	case "violation.created":
		plate := stringOf(p["license_plate"])
		fine := numberOf(p["fine_amount"])
		title := "New violation recorded"
		msg := fmt.Sprintf("Plate %s received a new violation (IDR %.0f).", plate, fine)
		return nil, title, msg
	case "invoice.created":
		invID := stringOf(p["invoice_id"])
		title := "New invoice issued"
		msg := fmt.Sprintf("Invoice %s has been created.", invID)
		return nil, title, msg
	case "payment.succeeded":
		invID := stringOf(p["invoice_id"])
		title := "Payment successful"
		msg := fmt.Sprintf("Invoice %s has been paid successfully.", invID)
		return nil, title, msg
	case "payment.failed":
		invID := stringOf(p["invoice_id"])
		title := "Payment failed"
		msg := fmt.Sprintf("Payment for invoice %s failed. Please retry.", invID)
		return nil, title, msg
	case "rule.published":
		title := "Fine rule updated"
		msg := "A new fine rule version has been published. Past violations are not affected."
		return nil, title, msg
	default:
		title := "Event"
		msg := fmt.Sprintf("Event type: %s", env.EventType)
		if b, err := json.Marshal(p); err == nil {
			msg = string(b)
		}
		return nil, title, msg
	}
}

// stringOf safely extracts a string from any.
func stringOf(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// numberOf safely extracts a float64 from any (JSON numbers are float64).
func numberOf(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	}
	return 0
}
