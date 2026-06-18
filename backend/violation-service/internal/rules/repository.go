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

// Filter for listing rule versions.
type Filter struct {
	// IsActive nil = all, true = active only, false = drafts only.
	IsActive *bool
	Page     int
	PageSize int
}

// Repository is the persistence interface.
type Repository interface {
	ListVersions(ctx context.Context, f Filter) ([]Version, int, error)
	GetVersion(ctx context.Context, id uuid.UUID) (*VersionWithDetails, error)
	GetActiveVersion(ctx context.Context) (*VersionWithDetails, error)
	NextVersionNumber(ctx context.Context) (int, error)
	CreateVersion(ctx context.Context, v Version, details []Detail) (*VersionWithDetails, error)
	UpdateVersionDetails(ctx context.Context, versionID uuid.UUID, details []Detail) (*VersionWithDetails, error)
	DeleteVersion(ctx context.Context, id uuid.UUID) error
	ActivateVersion(ctx context.Context, id uuid.UUID) error
}

type pgRepo struct{ db *pgxpool.Pool }

func NewPGRepository(db *pgxpool.Pool) Repository { return &pgRepo{db: db} }

// ListVersions returns a paginated, optionally-filtered list of rule
// versions (newest first by version_number) plus the total count.
func (r *pgRepo) ListVersions(ctx context.Context, f Filter) ([]Version, int, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize <= 0 || f.PageSize > 100 {
		f.PageSize = 20
	}

	args := []any{}
	where := "WHERE 1=1"
	if f.IsActive != nil {
		args = append(args, *f.IsActive)
		where += " AND is_active = $" + itoa(len(args))
	}

	listSQL := `SELECT id, version_number, is_active, published_at,
		COALESCE(created_by, '00000000-0000-0000-0000-000000000000'::uuid),
		created_at, updated_at
		FROM fine_rule_versions ` + where + ` ORDER BY version_number DESC
		LIMIT $` + itoa(len(args)+1) + ` OFFSET $` + itoa(len(args)+2)
	args = append(args, f.PageSize, (f.Page-1)*f.PageSize)

	rows, err := r.db.Query(ctx, listSQL, args...)
	if err != nil {
		return nil, 0, errs.Wrap(errs.CodeInternal, "list versions", err)
	}
	defer rows.Close()
	out := make([]Version, 0, f.PageSize)
	for rows.Next() {
		var v Version
		if err := rows.Scan(&v.ID, &v.VersionNumber, &v.IsActive, &v.PublishedAt,
			&v.CreatedBy, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, 0, errs.Wrap(errs.CodeInternal, "scan version", err)
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, errs.Wrap(errs.CodeInternal, "iterate versions", err)
	}

	countSQL := "SELECT count(*) FROM fine_rule_versions " + where
	var total int
	if err := r.db.QueryRow(ctx, countSQL, args[:len(args)-2]...).Scan(&total); err != nil {
		return nil, 0, errs.Wrap(errs.CodeInternal, "count versions", err)
	}
	return out, total, nil
}

// UpdateVersionDetails replaces the 4 details rows of a draft (non-active)
// version. We delete + reinsert in one transaction so the unique
// (rule_version_id, violation_type) constraint never trips and we can
// return the new full record in a single round-trip.
func (r *pgRepo) UpdateVersionDetails(ctx context.Context, versionID uuid.UUID, details []Detail) (*VersionWithDetails, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, "begin tx", err)
	}
	defer tx.Rollback(ctx)

	// Refuse to mutate an active version: the violation engine reads
	// from the active version and we don't want concurrent violations
	// to see half-applied changes. Create a new draft instead.
	var isActive bool
	if err := tx.QueryRow(ctx,
		`SELECT is_active FROM fine_rule_versions WHERE id = $1 FOR UPDATE`, versionID,
	).Scan(&isActive); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.New(errs.CodeRuleNotFound, "rule version not found")
		}
		return nil, errs.Wrap(errs.CodeInternal, "lookup version", err)
	}
	if isActive {
		return nil, errs.New(errs.CodeBusinessRule, "cannot update an active rule version; create a new draft instead")
	}

	if _, err := tx.Exec(ctx,
		`DELETE FROM fine_rule_details WHERE rule_version_id = $1`, versionID,
	); err != nil {
		return nil, errs.Wrap(errs.CodeInternal, "delete old details", err)
	}

	now := timeNow()
	for _, d := range details {
		if d.ID == uuid.Nil {
			d.ID = uuid.New()
		}
		d.RuleVersionID = versionID
		if err := validateDetailAmounts(d); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO fine_rule_details
				(id, rule_version_id, violation_type, base_amount,
				 day_multiplier, night_multiplier, repeat_0, repeat_1, repeat_2_plus,
				 created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $10)`,
			d.ID, d.RuleVersionID, d.ViolationType, d.BaseAmount,
			d.DayMultiplier, d.NightMultiplier, d.Repeat0, d.Repeat1, d.Repeat2Plus,
			now,
		); err != nil {
			return nil, errs.Wrap(errs.CodeInternal, "insert detail", err)
		}
	}

	if _, err := tx.Exec(ctx,
		`UPDATE fine_rule_versions SET updated_at = $1 WHERE id = $2`, now, versionID,
	); err != nil {
		return nil, errs.Wrap(errs.CodeInternal, "bump version", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, errs.Wrap(errs.CodeInternal, "commit", err)
	}
	return r.GetVersion(ctx, versionID)
}

// DeleteVersion removes a rule version. Active versions cannot be deleted
// because they are the source of truth for the violation engine — first
// publish a different version, then delete the old one.
func (r *pgRepo) DeleteVersion(ctx context.Context, id uuid.UUID) error {
	var isActive bool
	if err := r.db.QueryRow(ctx,
		`SELECT is_active FROM fine_rule_versions WHERE id = $1`, id,
	).Scan(&isActive); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.New(errs.CodeRuleNotFound, "rule version not found")
		}
		return errs.Wrap(errs.CodeInternal, "lookup version", err)
	}
	if isActive {
		return errs.New(errs.CodeBusinessRule, "cannot delete an active rule version; publish a different version first")
	}

	// details cascade via FK ON DELETE CASCADE (see migration 0004).
	tag, err := r.db.Exec(ctx, `DELETE FROM fine_rule_versions WHERE id = $1`, id)
	if err != nil {
		return errs.Wrap(errs.CodeInternal, "delete version", err)
	}
	if tag.RowsAffected() == 0 {
		return errs.New(errs.CodeRuleNotFound, "rule version not found")
	}
	return nil
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

// itoa is a tiny int-to-string helper used to build $N placeholders
// dynamically. Postgres only accepts up to 65535 params, which is far
// more than the 4-5 we ever build here.
func itoa(i int) string {
	const digits = "0123456789"
	if i < 10 {
		return string(digits[i])
	}
	return itoa(i/10) + string(digits[i%10])
}
