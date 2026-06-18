package rules

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/parking-portal/backend/pkg/errs"
)

// Repository is the persistence interface.
type Repository interface {
	ListVersions(ctx context.Context) ([]Version, error)
	GetVersion(ctx context.Context, id uuid.UUID) (*VersionWithDetails, error)
	GetActiveVersion(ctx context.Context) (*VersionWithDetails, error)
	NextVersionNumber(ctx context.Context) (int, error)
	CreateVersion(ctx context.Context, v Version, details []Detail) (*VersionWithDetails, error)
	ActivateVersion(ctx context.Context, id uuid.UUID) error
}

type pgRepo struct{ db *pgxpool.Pool }

func NewPGRepository(db *pgxpool.Pool) Repository { return &pgRepo{db: db} }

func (r *pgRepo) ListVersions(ctx context.Context) ([]Version, error) {
	const q = `SELECT id, version_number, is_active, published_at,
		COALESCE(created_by, '00000000-0000-0000-0000-000000000000'::uuid),
		created_at, updated_at
		FROM fine_rule_versions ORDER BY version_number DESC`
	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, "list versions", err)
	}
	defer rows.Close()
	var out []Version
	for rows.Next() {
		var v Version
		if err := rows.Scan(&v.ID, &v.VersionNumber, &v.IsActive, &v.PublishedAt,
			&v.CreatedBy, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, errs.Wrap(errs.CodeInternal, "scan version", err)
		}
		out = append(out, v)
	}
	return out, nil
}

func (r *pgRepo) GetVersion(ctx context.Context, id uuid.UUID) (*VersionWithDetails, error) {
	v, err := r.scanVersion(ctx, "WHERE v.id = $1", id)
	if err != nil {
		return nil, err
	}
	d, err := r.scanDetails(ctx, "WHERE rule_version_id = $1", id)
	if err != nil {
		return nil, err
	}
	v.Details = d
	return v, nil
}

func (r *pgRepo) GetActiveVersion(ctx context.Context) (*VersionWithDetails, error) {
	v, err := r.scanVersion(ctx, "WHERE v.is_active = true", nil)
	if err != nil {
		if ae, ok := errs.AsAppError(err); ok && ae.ErrCode == errs.CodeNotFound {
			return nil, errs.New(errs.CodeNoActiveRule, "no active rule version")
		}
		return nil, err
	}
	d, err := r.scanDetails(ctx, "WHERE rule_version_id = $1", v.ID)
	if err != nil {
		return nil, err
	}
	v.Details = d
	return v, nil
}

func (r *pgRepo) scanVersion(ctx context.Context, where string, idArg any) (*VersionWithDetails, error) {
	q := `SELECT v.id, v.version_number, v.is_active, v.published_at,
		COALESCE(v.created_by, '00000000-0000-0000-0000-000000000000'::uuid),
		v.created_at, v.updated_at
		FROM fine_rule_versions v ` + where + ` LIMIT 1`
	var v VersionWithDetails
	var err error
	if idArg == nil {
		err = r.db.QueryRow(ctx, q).Scan(&v.ID, &v.VersionNumber, &v.IsActive,
			&v.PublishedAt, &v.CreatedBy, &v.CreatedAt, &v.UpdatedAt)
	} else {
		err = r.db.QueryRow(ctx, q, idArg).Scan(&v.ID, &v.VersionNumber, &v.IsActive,
			&v.PublishedAt, &v.CreatedBy, &v.CreatedAt, &v.UpdatedAt)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errs.New(errs.CodeRuleNotFound, "rule version not found")
	}
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, "scan version", err)
	}
	return &v, nil
}

func (r *pgRepo) scanDetails(ctx context.Context, where string, idArg any) ([]Detail, error) {
	q := `SELECT id, rule_version_id, violation_type, base_amount,
		day_multiplier, night_multiplier, repeat_0, repeat_1, repeat_2_plus,
		created_at, updated_at
		FROM fine_rule_details ` + where
	var rows pgx.Rows
	var err error
	if idArg == nil {
		rows, err = r.db.Query(ctx, q)
	} else {
		rows, err = r.db.Query(ctx, q, idArg)
	}
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, "scan details query", err)
	}
	defer rows.Close()
	var out []Detail
	for rows.Next() {
		var d Detail
		if err := rows.Scan(&d.ID, &d.RuleVersionID, &d.ViolationType, &d.BaseAmount,
			&d.DayMultiplier, &d.NightMultiplier, &d.Repeat0, &d.Repeat1, &d.Repeat2Plus,
			&d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, errs.Wrap(errs.CodeInternal, "scan detail", err)
		}
		out = append(out, d)
	}
	return out, nil
}

func (r *pgRepo) NextVersionNumber(ctx context.Context) (int, error) {
	const q = `SELECT COALESCE(MAX(version_number), 0) + 1 FROM fine_rule_versions`
	var n int
	if err := r.db.QueryRow(ctx, q).Scan(&n); err != nil {
		return 0, errs.Wrap(errs.CodeInternal, "next version", err)
	}
	return n, nil
}

// CreateVersion creates a new draft version. Caller passes the v.VersionNumber
// (0 for "auto"), and a slice of 4 Details.
func (r *pgRepo) CreateVersion(ctx context.Context, v Version, details []Detail) (*VersionWithDetails, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, "begin tx", err)
	}
	defer tx.Rollback(ctx)

	nextNum, err := r.NextVersionNumber(ctx)
	if err != nil {
		return nil, err
	}
	if v.VersionNumber == 0 {
		v.VersionNumber = nextNum
	}
	v.IsActive = false
	v.PublishedAt = timeNow()

	const insV = `INSERT INTO fine_rule_versions
		(id, version_number, is_active, published_at, created_by, created_at, updated_at)
		VALUES ($1, $2, false, $3, $4, $3, $3)`
	if _, err := tx.Exec(ctx, insV, v.ID, v.VersionNumber, v.PublishedAt, v.CreatedBy); err != nil {
		return nil, errs.Wrap(errs.CodeInternal, "insert version", err)
	}
	for _, d := range details {
		if err := validateDetailAmounts(d); err != nil {
			return nil, err
		}
		const insD = `INSERT INTO fine_rule_details
			(id, rule_version_id, violation_type, base_amount,
			 day_multiplier, night_multiplier, repeat_0, repeat_1, repeat_2_plus,
			 created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $10)`
		if _, err := tx.Exec(ctx, insD, d.ID, v.ID, d.ViolationType, d.BaseAmount,
			d.DayMultiplier, d.NightMultiplier, d.Repeat0, d.Repeat1, d.Repeat2Plus,
			v.PublishedAt); err != nil {
			return nil, errs.Wrap(errs.CodeInternal, "insert detail", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, errs.Wrap(errs.CodeInternal, "commit", err)
	}
	return r.GetVersion(ctx, v.ID)
}

func (r *pgRepo) ActivateVersion(ctx context.Context, id uuid.UUID) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return errs.Wrap(errs.CodeInternal, "begin tx", err)
	}
	defer tx.Rollback(ctx)

	// verify exists
	var exists bool
	if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM fine_rule_versions WHERE id=$1)", id).Scan(&exists); err != nil {
		return errs.Wrap(errs.CodeInternal, "verify version", err)
	}
	if !exists {
		return errs.New(errs.CodeRuleNotFound, "rule version not found")
	}
	if _, err := tx.Exec(ctx, "UPDATE fine_rule_versions SET is_active=false, updated_at=now() WHERE is_active=true"); err != nil {
		return errs.Wrap(errs.CodeInternal, "deactivate all", err)
	}
	if _, err := tx.Exec(ctx, "UPDATE fine_rule_versions SET is_active=true, published_at=now(), updated_at=now() WHERE id=$1", id); err != nil {
		return errs.Wrap(errs.CodeInternal, "activate", err)
	}
	return tx.Commit(ctx)
}

func validateDetailAmounts(d Detail) error {
	if d.BaseAmount.LessThanOrEqual(decimal.Zero) {
		return errs.New(errs.CodeValidation, "base_amount must be > 0")
	}
	return nil
}
