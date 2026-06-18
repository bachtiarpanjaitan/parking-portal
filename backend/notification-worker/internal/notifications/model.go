// Package notifications owns the notifications + processed_events tables.
// See .ai/DATABASE_MAPPING.md for the schema.
package notifications

import (
	"time"

	"github.com/google/uuid"
)

// Notification is a row in the `notifications` table.
type Notification struct {
	ID        uuid.UUID  `json:"id"`
	UserID    *uuid.UUID `json:"user_id,omitempty"`
	EventType string     `json:"event_type"`
	Title     string     `json:"title"`
	Message   string     `json:"message"`
	CreatedAt time.Time  `json:"created_at"`
}
