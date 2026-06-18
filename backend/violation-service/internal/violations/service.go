package violations

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/parking-portal/backend/pkg/errs"
	"github.com/parking-portal/backend/violation-service/internal/fines"
	"github.com/parking-portal/backend/violation-service/internal/rules"
	"github.com/parking-portal/backend/violation-service/internal/users"
)

// CreateRequest is the body for POST /violations.
type CreateRequest struct {
	MemberID           uuid.UUID `json:"member_id" validate:"required"`
	LicensePlate       string    `json:"license_plate" validate:"required,min=1,max=20"`
	ViolationType      string    `json:"violation_type" validate:"required,oneof=expired_meter no_parking_zone blocking_hydrant disabled_spot"`
	Location           string    `json:"location" validate:"required,max=255"`
	ViolationTimestamp time.Time `json:"violation_timestamp" validate:"required"`
	PhotoURL           string    `json:"photo_url" validate:"required"`
}

// CreateResult bundles the violation and its invoice.
type CreateResult struct {
	ViolationID    uuid.UUID    `json:"violation_id"`
	RuleVersionID  uuid.UUID    `json:"rule_version_id"`
	RuleVersionNum int          `json:"rule_version_number"`
	InvoiceID      uuid.UUID    `json:"invoice_id"`
	FineAmount     float64      `json:"fine_amount"`
	Snapshot       fines.Result `json:"calculation_snapshot"`
}

// InvoiceCreator is implemented by the invoices package.
type InvoiceCreator interface {
	CreateForViolation(ctx context.Context, v *Violation) (uuid.UUID, error)
}

// EventPublisher is implemented by the events package.
type EventPublisher interface {
	PublishViolationCreated(ctx context.Context, payload any) error
	PublishInvoiceCreated(ctx context.Context, payload any) error
}

// Service coordinates creation, listing, and retrieval of violations.
type Service struct {
	repo   Repository
	rules  *rules.Service
	users  users.Repository
	invSvc InvoiceCreator
	events EventPublisher
}

func NewService(repo Repository, r *rules.Service, u users.Repository, inv InvoiceCreator, e EventPublisher) *Service {
	return &Service{repo: repo, rules: r, users: u, invSvc: inv, events: e}
}

// Create implements Flow 1: load active rule → count prior unpaid → calc fine →
// save violation + invoice in a transaction → publish events (best-effort).
func (s *Service) Create(ctx context.Context, req CreateRequest) (*CreateResult, error) {
	// 1. Verify the member exists.
	if _, err := s.users.FindByID(ctx, req.MemberID); err != nil {
		if ae, ok := errs.AsAppError(err); ok && ae.ErrCode == errs.CodeNotFound {
			return nil, errs.New(errs.CodeNotFound, "member not found")
		}
		return nil, err
	}

	// 2. Load the active rule version + matching detail.
	active, err := s.rules.GetActive(ctx)
	if err != nil {
		return nil, err
	}
	var detail *rules.Detail
	for i := range active.Details {
		if active.Details[i].ViolationType == req.ViolationType {
			detail = &active.Details[i]
			break
		}
	}
	if detail == nil {
		return nil, errs.New(errs.CodeInvalidViolationType, "no rule for violation type")
	}

	// 3. Count prior unpaid violations for this plate.
	priorUnpaid, err := s.repo.CountPriorUnpaid(ctx, req.LicensePlate, req.ViolationTimestamp)
	if err != nil {
		return nil, err
	}

	// 4. Build the engine Rule and calculate.
	rule := fines.Rule{
		BaseAmount:      detail.BaseAmount,
		DayMultiplier:   detail.DayMultiplier,
		NightMultiplier: detail.NightMultiplier,
		Repeat0:         detail.Repeat0,
		Repeat1:         detail.Repeat1,
		Repeat2Plus:     detail.Repeat2Plus,
	}
	now := time.Now().UTC()
	snap := fines.Calculate(rule, active.ID.String(), active.VersionNumber,
		req.ViolationType, req.ViolationTimestamp.Local(), priorUnpaid, now)

	// 5. Build the violation.
	v := &Violation{
		ID:                  uuid.New(),
		MemberID:            req.MemberID,
		RuleVersionID:       active.ID,
		RuleVersionNumber:   active.VersionNumber,
		LicensePlate:        strings.ToUpper(req.LicensePlate),
		ViolationType:       req.ViolationType,
		Location:            req.Location,
		ViolationTimestamp:  req.ViolationTimestamp,
		PhotoURL:            req.PhotoURL,
		FineAmount:          decimalFromFloat(snap.CalculatedFine),
		CalculationSnapshot: snapAsMap(snap),
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if err := s.repo.Create(ctx, v, uuid.Nil); err != nil {
		return nil, err
	}
	// Create invoice
	invoiceID, err := s.invSvc.CreateForViolation(ctx, v)
	if err != nil {
		return nil, err
	}
	v.InvoiceID = &invoiceID

	// 6. Publish events (best-effort).
	if s.events != nil {
		_ = s.events.PublishViolationCreated(ctx, map[string]any{
			"violation_id":        v.ID,
			"member_id":           v.MemberID,
			"license_plate":       v.LicensePlate,
			"violation_type":      v.ViolationType,
			"rule_version_id":     v.RuleVersionID,
			"rule_version_number": v.RuleVersionNumber,
			"location":            v.Location,
			"violation_timestamp": v.ViolationTimestamp,
			"photo_url":           v.PhotoURL,
			"fine_amount":         int64(snap.CalculatedFine),
		})
		_ = s.events.PublishInvoiceCreated(ctx, map[string]any{
			"invoice_id":   invoiceID,
			"violation_id": v.ID,
			"member_id":    v.MemberID,
			"amount":       int64(snap.CalculatedFine),
		})
	}

	return &CreateResult{
		ViolationID:    v.ID,
		RuleVersionID:  v.RuleVersionID,
		RuleVersionNum: v.RuleVersionNumber,
		InvoiceID:      invoiceID,
		FineAmount:     snap.CalculatedFine,
		Snapshot:       snap,
	}, nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Violation, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *Service) List(ctx context.Context, f Filter) ([]Violation, int, error) {
	return s.repo.List(ctx, f)
}

func snapAsMap(s fines.Result) map[string]any {
	return map[string]any{
		"rule_version_id":     s.RuleVersionID,
		"rule_version_number": s.RuleVersionNumber,
		"violation_type":      s.ViolationType,
		"base_amount":         s.BaseAmount,
		"time_multiplier":     s.TimeMultiplier,
		"time_window":         s.TimeWindow,
		"repeat_multiplier":   s.RepeatMultiplier,
		"prior_unpaid_count":  s.PriorUnpaidCount,
		"calculated_fine":     s.CalculatedFine,
		"calculated_at":       s.CalculatedAt,
	}
}
