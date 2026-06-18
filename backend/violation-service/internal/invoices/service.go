package invoices

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/parking-portal/backend/pkg/errs"
	"github.com/parking-portal/backend/violation-service/internal/violations"
)

// Service manages invoice creation, listing, and retrieval.
type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service { return &Service{repo: repo} }

// CreateForViolation creates a PENDING invoice for the given violation.
// Implements the violations.InvoiceCreator interface.
func (s *Service) CreateForViolation(ctx context.Context, v *violations.Violation) (uuid.UUID, error) {
	inv := &Invoice{
		ID:          uuid.New(),
		ViolationID: v.ID,
		Amount:      v.FineAmount,
		Status:      "PENDING",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if err := s.repo.Create(ctx, inv); err != nil {
		return uuid.Nil, err
	}
	return inv.ID, nil
}

// Get returns one invoice with its latest payment (if any).
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*InvoiceWithLatest, error) {
	return s.repo.FindByIDWithLatest(ctx, id)
}

// List returns paginated invoices, optionally filtered.
func (s *Service) List(ctx context.Context, f Filter) ([]Invoice, int, error) {
	return s.repo.List(ctx, f)
}

// SetStatus updates the invoice status. Called by the payment service via an
// internal HTTP call after a Midtrans webhook / refresh settles the payment.
// PAID is terminal — we refuse to mutate an already-PAID invoice.
func (s *Service) SetStatus(ctx context.Context, id uuid.UUID, status string) error {
	inv, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if inv.Status == "PAID" {
		// Idempotent: PAID → PAID is a no-op (e.g. duplicate webhook delivery).
		if status == "PAID" {
			return nil
		}
		return errs.New(errs.CodeInvalidInvStatus, "cannot change a PAID invoice")
	}
	if err := s.repo.UpdateStatus(ctx, id, status); err != nil {
		return err
	}
	return nil
}
