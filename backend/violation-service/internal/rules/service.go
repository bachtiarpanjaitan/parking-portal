package rules

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/parking-portal/backend/pkg/errs"
)

// CreateRequest is the body for POST /rules.
type CreateRequest struct {
	Rules []DetailInput `json:"rules" validate:"required,min=4,max=4,dive"`
}

// DetailInput is one row of the rules array.
type DetailInput struct {
	ViolationType   string          `json:"violation_type" validate:"required,oneof=expired_meter no_parking_zone blocking_hydrant disabled_spot"`
	BaseAmount      decimal.Decimal `json:"base_amount" validate:"required"`
	DayMultiplier   decimal.Decimal `json:"day_multiplier" validate:"required"`
	NightMultiplier decimal.Decimal `json:"night_multiplier" validate:"required"`
	Repeat0         decimal.Decimal `json:"repeat_0" validate:"required"`
	Repeat1         decimal.Decimal `json:"repeat_1" validate:"required"`
	Repeat2Plus     decimal.Decimal `json:"repeat_2_plus" validate:"required"`
}

// Service coordinates rule creation, publication, and reading.
type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service { return &Service{repo: repo} }

// List returns all rule versions, newest first.
func (s *Service) List(ctx context.Context) ([]Version, error) {
	return s.repo.ListVersions(ctx)
}

// Get returns one version with its details.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*VersionWithDetails, error) {
	return s.repo.GetVersion(ctx, id)
}

// GetActive returns the currently active version.
func (s *Service) GetActive(ctx context.Context) (*VersionWithDetails, error) {
	return s.repo.GetActiveVersion(ctx)
}

// CreateDraft creates a new rule version in draft state. Not yet active.
func (s *Service) CreateDraft(ctx context.Context, createdBy uuid.UUID, req CreateRequest) (*VersionWithDetails, error) {
	if len(req.Rules) != 4 {
		return nil, errs.New(errs.CodeValidation, "exactly 4 rule details required")
	}
	v := Version{
		ID:            uuid.New(),
		VersionNumber: 0, // 0 → auto
		CreatedBy:     createdBy,
		PublishedAt:   time.Now().UTC(),
	}
	details := make([]Detail, 0, len(req.Rules))
	for _, r := range req.Rules {
		if err := validateOne(r); err != nil {
			return nil, err
		}
		details = append(details, Detail{
			ID:              uuid.New(),
			RuleVersionID:   v.ID,
			ViolationType:   r.ViolationType,
			BaseAmount:      r.BaseAmount,
			DayMultiplier:   r.DayMultiplier,
			NightMultiplier: r.NightMultiplier,
			Repeat0:         r.Repeat0,
			Repeat1:         r.Repeat1,
			Repeat2Plus:     r.Repeat2Plus,
		})
	}
	return s.repo.CreateVersion(ctx, v, details)
}

// Publish atomically activates the version and deactivates all others.
func (s *Service) Publish(ctx context.Context, id uuid.UUID) error {
	return s.repo.ActivateVersion(ctx, id)
}

func validateOne(r DetailInput) error {
	if r.BaseAmount.LessThanOrEqual(decimal.Zero) {
		return errs.New(errs.CodeValidation, "base_amount must be > 0")
	}
	if r.DayMultiplier.LessThanOrEqual(decimal.Zero) ||
		r.NightMultiplier.LessThanOrEqual(decimal.Zero) ||
		r.Repeat0.LessThanOrEqual(decimal.Zero) ||
		r.Repeat1.LessThanOrEqual(decimal.Zero) ||
		r.Repeat2Plus.LessThanOrEqual(decimal.Zero) {
		return errs.New(errs.CodeValidation, "multipliers must be > 0")
	}
	return nil
}
