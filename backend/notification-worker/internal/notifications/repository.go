package notifications

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository is the persistence interface.
type Repository interface {
	// MarkProcessed inserts the event_id into processed_events.
	// Returns (true, nil) if it was new, (false, nil) if it was already
	// there (idempotent re-delivery), (false, err) on DB failure.
	MarkProcessed(ctx context.Context, eventID uuid.UUID) (bool, error)
	// InsertNotification inserts a row in `notifications`.
	InsertNotification(ctx context.Context, n *Notification) error
}

type pgRepo struct{ db *pgxpool.Pool }

func NewPGRepository(db *pgxpool.Pool) Repository { return &pgRepo{db: db} }

func (r *pgRepo) MarkProcessed(ctx context.Context, eventID uuid.UUID) (bool, error) {
	const q = `INSERT INTO processed_events (event_id, processed_at) VALUES ($1, now())
		ON CONFLICT (event_id) DO NOTHING`
	tag, err := r.db.Exec(ctx, q, eventID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (r *pgRepo) InsertNotification(ctx context.Context, n *Notification) error {
	const q = `INSERT INTO notifications (id, user_id, event_type, title, message, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := r.db.Exec(ctx, q, n.ID, n.UserID, n.EventType, n.Title, n.Message, n.CreatedAt)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	return nil
}
