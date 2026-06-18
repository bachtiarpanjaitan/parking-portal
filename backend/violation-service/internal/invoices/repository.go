// Package invoices handles persistence for the invoices module.
package invoices

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/parking-portal/backend/pkg/errs"
)

// Filter for listing invoices. All string filters are case-insensitive
// partial matches (LIKE '%...%') on the joined violation columns. Date
// range filters apply to the violation timestamp (when the violation
// happened), not the invoice creation time — that is what members
// actually want when they say "find my ticket from last Tuesday".
type Filter struct {
	MemberID      *uuid.UUID
	Status        string
	LicensePlate  string
	ViolationType string
	Location      string
	From          *string // ISO 8601 (caller-parsed)
	To            *string // ISO 8601 (caller-parsed)
	Page          int
	PageSize      int
}

// Repository is the persistence interface.
type Repository interface {
	Create(ctx context.Context, inv *Invoice) error
	FindByID(ctx context.Context, id uuid.UUID) (*Invoice, error)
	FindByIDWithLatest(ctx context.Context, id uuid.UUID) (*InvoiceWithLatest, error)
	List(ctx context.Context, f Filter) ([]InvoiceListItem, int, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status string) error
}

type pgRepo struct{ db *pgxpool.Pool }

func NewPGRepository(db *pgxpool.Pool) Repository { return &pgRepo{db: db} }

func (r *pgRepo) Create(ctx context.Context, inv *Invoice) error {
	const q = `INSERT INTO invoices
		(id, violation_id, amount, status, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$5)`
	_, err := r.db.Exec(ctx, q, inv.ID, inv.ViolationID, inv.Amount, inv.Status, inv.CreatedAt)
	if err != nil {
		return errs.Wrap(errs.CodeInternal, "insert invoice", err)
	}
	return nil
}

func (r *pgRepo) FindByID(ctx context.Context, id uuid.UUID) (*Invoice, error) {
	const q = `SELECT i.id, i.violation_id, v.member_id, i.amount, i.status, i.created_at, i.updated_at
		FROM invoices i JOIN violations v ON v.id = i.violation_id
		WHERE i.id = $1`
	var inv Invoice
	err := r.db.QueryRow(ctx, q, id).Scan(
		&inv.ID, &inv.ViolationID, &inv.MemberID, &inv.Amount, &inv.Status, &inv.CreatedAt, &inv.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errs.New(errs.CodeInvoiceNotFound, "invoice not found")
	}
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, "find invoice", err)
	}
	return &inv, nil
}

func (r *pgRepo) FindByIDWithLatest(ctx context.Context, id uuid.UUID) (*InvoiceWithLatest, error) {
	inv, err := r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	// NOTE: the `scenario` column was removed in migration 0011 (replaced by
	// the Midtrans flow). Only the columns that still exist are selected.
	const q = `SELECT id, status, transaction_id, payment_method, created_at
		FROM payments WHERE invoice_id = $1
		ORDER BY created_at DESC LIMIT 1`
	var p LatestPayment
	err = r.db.QueryRow(ctx, q, id).Scan(&p.ID, &p.Status, &p.TransactionID, &p.PaymentMethod, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return &InvoiceWithLatest{Invoice: *inv, LatestPayment: nil}, nil
	}
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, "find latest payment", err)
	}
	return &InvoiceWithLatest{Invoice: *inv, LatestPayment: &p}, nil
}

// buildWhere assembles the dynamic WHERE fragment + the matching args
// slice. The same fragment is reused for the list and count queries so
// they always agree on the filter set.
func buildWhere(f Filter) (string, []any) {
	var sb strings.Builder
	var args []any
	if f.MemberID != nil {
		args = append(args, *f.MemberID)
		sb.WriteString(fmt.Sprintf(" AND v.member_id = $%d", len(args)))
	}
	if f.Status != "" {
		args = append(args, f.Status)
		sb.WriteString(fmt.Sprintf(" AND i.status = $%d", len(args)))
	}
	if f.LicensePlate != "" {
		args = append(args, "%"+f.LicensePlate+"%")
		sb.WriteString(fmt.Sprintf(" AND v.license_plate ILIKE $%d", len(args)))
	}
	if f.ViolationType != "" {
		args = append(args, f.ViolationType)
		sb.WriteString(fmt.Sprintf(" AND v.violation_type = $%d", len(args)))
	}
	if f.Location != "" {
		args = append(args, "%"+f.Location+"%")
		sb.WriteString(fmt.Sprintf(" AND v.location ILIKE $%d", len(args)))
	}
	if f.From != nil && *f.From != "" {
		args = append(args, *f.From)
		sb.WriteString(fmt.Sprintf(" AND v.violation_timestamp >= $%d::timestamptz", len(args)))
	}
	if f.To != nil && *f.To != "" {
		args = append(args, *f.To)
		sb.WriteString(fmt.Sprintf(" AND v.violation_timestamp <  $%d::timestamptz", len(args)))
	}
	return sb.String(), args
}

func (r *pgRepo) List(ctx context.Context, f Filter) ([]InvoiceListItem, int, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize <= 0 || f.PageSize > 100 {
		f.PageSize = 20
	}

	whereSQL, whereArgs := buildWhere(f)

	// We join fine_rule_versions so the UI can show the rule version
	// number that produced the fine, matching what /history returns.
	const selectCols = `i.id, i.violation_id, v.member_id, i.amount, i.status,
		i.created_at, i.updated_at,
		v.license_plate, v.violation_type, v.location, v.violation_timestamp,
		v.photo_url, COALESCE(frv.version_number, 0)`

	listSQL := `SELECT ` + selectCols + `
		FROM invoices i
		JOIN violations v ON v.id = i.violation_id
		LEFT JOIN fine_rule_versions frv ON frv.id = v.rule_version_id
		WHERE 1=1` + whereSQL + `
		ORDER BY v.violation_timestamp DESC
		LIMIT $` + itoa(len(whereArgs)+1) + ` OFFSET $` + itoa(len(whereArgs)+2)

	listArgs := append([]any{}, whereArgs...)
	listArgs = append(listArgs, f.PageSize, (f.Page-1)*f.PageSize)

	rows, err := r.db.Query(ctx, listSQL, listArgs...)
	if err != nil {
		return nil, 0, errs.Wrap(errs.CodeInternal, "list invoices", err)
	}
	defer rows.Close()
	out := make([]InvoiceListItem, 0, f.PageSize)
	for rows.Next() {
		var it InvoiceListItem
		if err := rows.Scan(
			&it.ID, &it.ViolationID, &it.MemberID, &it.Amount, &it.Status,
			&it.CreatedAt, &it.UpdatedAt,
			&it.LicensePlate, &it.ViolationType, &it.Location, &it.ViolationTimestamp,
			&it.PhotoURL, &it.RuleVersionNumber,
		); err != nil {
			return nil, 0, errs.Wrap(errs.CodeInternal, "scan invoice", err)
		}
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, errs.Wrap(errs.CodeInternal, "iterate invoices", err)
	}

	countSQL := `SELECT count(*)
		FROM invoices i
		JOIN violations v ON v.id = i.violation_id
		WHERE 1=1` + whereSQL
	var total int
	if err := r.db.QueryRow(ctx, countSQL, whereArgs...).Scan(&total); err != nil {
		return nil, 0, errs.Wrap(errs.CodeInternal, "count invoices", err)
	}
	return out, total, nil
}

// UpdateStatus mutates the invoice status. PAID is terminal: callers cannot
// revert it (the DB CHECK constraint also enforces this — see migration 0006).
// Returns INVOICE_NOT_FOUND if no row matches.
func (r *pgRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE invoices SET status = $1, updated_at = now() WHERE id = $2`,
		status, id,
	)
	if err != nil {
		return errs.Wrap(errs.CodeInternal, "update invoice status", err)
	}
	if tag.RowsAffected() == 0 {
		return errs.New(errs.CodeInvoiceNotFound, "invoice not found")
	}
	return nil
}

func itoa(i int) string {
	const digits = "0123456789"
	if i < 10 {
		return string(digits[i])
	}
	return itoa(i/10) + string(digits[i%10])
}
