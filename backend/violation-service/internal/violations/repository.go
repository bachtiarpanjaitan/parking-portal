package violations

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/parking-portal/backend/pkg/errs"
)

// Filter holds the list query options.
type Filter struct {
	MemberID     *uuid.UUID
	LicensePlate string
	From         *time.Time
	To           *time.Time
	Page         int
	PageSize     int
	Sort         string // "violation_timestamp" | "created_at" | "fine_amount"
	Order        string // "asc" | "desc"
}

// Repository is the persistence interface.
type Repository interface {
	Create(ctx context.Context, v *Violation, invoiceID uuid.UUID) error
	FindByID(ctx context.Context, id uuid.UUID) (*Violation, error)
	List(ctx context.Context, f Filter) ([]Violation, int, error)
	CountPriorUnpaid(ctx context.Context, plate string, before time.Time) (int, error)
}

type pgRepo struct{ db *pgxpool.Pool }

func NewPGRepository(db *pgxpool.Pool) Repository { return &pgRepo{db: db} }

func (r *pgRepo) Create(ctx context.Context, v *Violation, invoiceID uuid.UUID) error {
	const q = `INSERT INTO violations
		(id, member_id, rule_version_id, license_plate, violation_type,
		 location, violation_timestamp, photo_url, fine_amount,
		 calculation_snapshot, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::jsonb,$11,$11)`
	snap, err := json.Marshal(v.CalculationSnapshot)
	if err != nil {
		return errs.Wrap(errs.CodeInternal, "marshal snapshot", err)
	}
	_, err = r.db.Exec(ctx, q,
		v.ID, v.MemberID, v.RuleVersionID, v.LicensePlate, v.ViolationType,
		v.Location, v.ViolationTimestamp, v.PhotoURL, v.FineAmount,
		string(snap), v.CreatedAt,
	)
	if err != nil {
		return errs.Wrap(errs.CodeInternal, "insert violation", err)
	}
	return nil
}

func (r *pgRepo) FindByID(ctx context.Context, id uuid.UUID) (*Violation, error) {
	const q = `SELECT v.id, v.member_id, v.rule_version_id, v.license_plate,
		v.violation_type, v.location, v.violation_timestamp, v.photo_url,
		v.fine_amount, v.calculation_snapshot, v.created_at, v.updated_at,
		COALESCE(i.id, '00000000-0000-0000-0000-000000000000'::uuid),
		COALESCE(i.status, '')
		FROM violations v
		LEFT JOIN invoices i ON i.violation_id = v.id
		WHERE v.id = $1`
	var v Violation
	var snapRaw []byte
	var invID uuid.UUID
	var invStatus string
	err := r.db.QueryRow(ctx, q, id).Scan(
		&v.ID, &v.MemberID, &v.RuleVersionID, &v.LicensePlate, &v.ViolationType,
		&v.Location, &v.ViolationTimestamp, &v.PhotoURL, &v.FineAmount,
		&snapRaw, &v.CreatedAt, &v.UpdatedAt, &invID, &invStatus,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errs.New(errs.CodeViolationNotFound, "violation not found")
	}
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, "scan violation", err)
	}
	if invID.String() != "00000000-0000-0000-0000-000000000000" {
		v.InvoiceID = &invID
		v.InvoiceStatus = invStatus
	}
	_ = json.Unmarshal(snapRaw, &v.CalculationSnapshot)
	return &v, nil
}

func (r *pgRepo) List(ctx context.Context, f Filter) ([]Violation, int, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize <= 0 || f.PageSize > 100 {
		f.PageSize = 20
	}
	sort := "v.violation_timestamp"
	switch f.Sort {
	case "created_at":
		sort = "v.created_at"
	case "fine_amount":
		sort = "v.fine_amount"
	}
	order := "DESC"
	if f.Order == "asc" {
		order = "ASC"
	}

	args := []any{}
	where := "WHERE 1=1"
	if f.MemberID != nil {
		args = append(args, *f.MemberID)
		where += " AND v.member_id = $" + itoa(len(args))
	}
	if f.LicensePlate != "" {
		args = append(args, f.LicensePlate)
		where += " AND v.license_plate = $" + itoa(len(args))
	}
	if f.From != nil {
		args = append(args, *f.From)
		where += " AND v.violation_timestamp >= $" + itoa(len(args))
	}
	if f.To != nil {
		args = append(args, *f.To)
		where += " AND v.violation_timestamp < $" + itoa(len(args))
	}

	listSQL := `SELECT v.id, v.member_id, v.rule_version_id, v.license_plate,
		v.violation_type, v.location, v.violation_timestamp, v.photo_url,
		v.fine_amount, v.calculation_snapshot, v.created_at, v.updated_at,
		COALESCE(i.id, '00000000-0000-0000-0000-000000000000'::uuid),
		COALESCE(i.status, '')
		FROM violations v
		LEFT JOIN invoices i ON i.violation_id = v.id
		` + where + ` ORDER BY ` + sort + ` ` + order + `
		LIMIT $` + itoa(len(args)+1) + ` OFFSET $` + itoa(len(args)+2)
	args = append(args, f.PageSize, (f.Page-1)*f.PageSize)

	rows, err := r.db.Query(ctx, listSQL, args...)
	if err != nil {
		return nil, 0, errs.Wrap(errs.CodeInternal, "list violations", err)
	}
	defer rows.Close()
	out := make([]Violation, 0, f.PageSize)
	for rows.Next() {
		var v Violation
		var snapRaw []byte
		var invID uuid.UUID
		var invStatus string
		if err := rows.Scan(
			&v.ID, &v.MemberID, &v.RuleVersionID, &v.LicensePlate, &v.ViolationType,
			&v.Location, &v.ViolationTimestamp, &v.PhotoURL, &v.FineAmount,
			&snapRaw, &v.CreatedAt, &v.UpdatedAt, &invID, &invStatus,
		); err != nil {
			return nil, 0, errs.Wrap(errs.CodeInternal, "scan row", err)
		}
		_ = json.Unmarshal(snapRaw, &v.CalculationSnapshot)
		if invID.String() != "00000000-0000-0000-0000-000000000000" {
			v.InvoiceID = &invID
			v.InvoiceStatus = invStatus
		}
		out = append(out, v)
	}

	countSQL := "SELECT count(*) FROM violations v " + where
	var total int
	if err := r.db.QueryRow(ctx, countSQL, args[:len(args)-2]...).Scan(&total); err != nil {
		return nil, 0, errs.Wrap(errs.CodeInternal, "count", err)
	}
	return out, total, nil
}

// CountPriorUnpaid returns the count of unpaid (PENDING|FAILED) invoices for
// the given plate whose violation_timestamp is within (now - 90 days, now).
func (r *pgRepo) CountPriorUnpaid(ctx context.Context, plate string, before time.Time) (int, error) {
	const q = `SELECT count(*) FROM violations v
		JOIN invoices i ON i.violation_id = v.id
		WHERE v.license_plate = $1
		  AND i.status IN ('PENDING','FAILED')
		  AND v.violation_timestamp < $2
		  AND v.violation_timestamp >= $2 - INTERVAL '90 days'`
	var n int
	if err := r.db.QueryRow(ctx, q, plate, before).Scan(&n); err != nil {
		return 0, errs.Wrap(errs.CodeInternal, "count prior unpaid", err)
	}
	return n, nil
}

func itoa(i int) string {
	const digits = "0123456789"
	if i < 10 {
		return string(digits[i])
	}
	return itoa(i/10) + string(digits[i%10])
}
