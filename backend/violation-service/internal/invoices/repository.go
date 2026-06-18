package invoices

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/parking-portal/backend/pkg/errs"
)

// Filter for listing invoices.
type Filter struct {
	MemberID *uuid.UUID
	Status   string
	Page     int
	PageSize int
}

// Repository is the persistence interface.
type Repository interface {
	Create(ctx context.Context, inv *Invoice) error
	FindByID(ctx context.Context, id uuid.UUID) (*Invoice, error)
	FindByIDWithLatest(ctx context.Context, id uuid.UUID) (*InvoiceWithLatest, error)
	List(ctx context.Context, f Filter) ([]Invoice, int, error)
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

func (r *pgRepo) List(ctx context.Context, f Filter) ([]Invoice, int, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize <= 0 || f.PageSize > 100 {
		f.PageSize = 20
	}
	args := []any{}
	where := "WHERE 1=1"
	if f.MemberID != nil {
		args = append(args, *f.MemberID)
		where += " AND v.member_id = $" + itoa(len(args))
	}
	if f.Status != "" {
		args = append(args, f.Status)
		where += " AND i.status = $" + itoa(len(args))
	}
	listSQL := `SELECT i.id, i.violation_id, v.member_id, i.amount, i.status, i.created_at, i.updated_at
		FROM invoices i JOIN violations v ON v.id = i.violation_id
		` + where + ` ORDER BY i.created_at DESC
		LIMIT $` + itoa(len(args)+1) + ` OFFSET $` + itoa(len(args)+2)
	args = append(args, f.PageSize, (f.Page-1)*f.PageSize)
	rows, err := r.db.Query(ctx, listSQL, args...)
	if err != nil {
		return nil, 0, errs.Wrap(errs.CodeInternal, "list invoices", err)
	}
	defer rows.Close()
	out := make([]Invoice, 0, f.PageSize)
	for rows.Next() {
		var inv Invoice
		if err := rows.Scan(&inv.ID, &inv.ViolationID, &inv.MemberID, &inv.Amount, &inv.Status,
			&inv.CreatedAt, &inv.UpdatedAt); err != nil {
			return nil, 0, errs.Wrap(errs.CodeInternal, "scan invoice", err)
		}
		out = append(out, inv)
	}
	countSQL := "SELECT count(*) FROM invoices i JOIN violations v ON v.id = i.violation_id " + where
	var total int
	if err := r.db.QueryRow(ctx, countSQL, args[:len(args)-2]...).Scan(&total); err != nil {
		return nil, 0, errs.Wrap(errs.CodeInternal, "count invoices", err)
	}
	return out, total, nil
}

func itoa(i int) string {
	const digits = "0123456789"
	if i < 10 {
		return string(digits[i])
	}
	return itoa(i/10) + string(digits[i%10])
}
