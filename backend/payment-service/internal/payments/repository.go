package payments

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

// Repository is the persistence interface for payments.
type Repository interface {
	Insert(ctx context.Context, p *Payment) error
	FindByID(ctx context.Context, id uuid.UUID) (*Payment, error)
	FindByMidtransOrderID(ctx context.Context, orderID string) (*Payment, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status string, method *string, mtTxID, mtTxStatus *string, raw any) error
}

type pgRepo struct{ db *pgxpool.Pool }

func NewPGRepository(db *pgxpool.Pool) Repository { return &pgRepo{db: db} }

func (r *pgRepo) Insert(ctx context.Context, p *Payment) error {
	rawJSON, _ := json.Marshal(p.MidrawResponse)
	// transaction_id is NOT NULL in the DB; for a freshly created Snap
	// payment we don't have a real Midtrans transaction_id yet, so we
	// store a placeholder of "PENDING-<order_id>". The webhook will overwrite
	// it with the real value.
	txID := "PENDING-" + p.MidtransOrderID
	if p.MidtransTransactionID != nil {
		txID = *p.MidtransTransactionID
	}
	const q = `
		INSERT INTO payments
			(id, invoice_id, amount, transaction_id, status, payment_method,
			 midtrans_order_id, midtrans_snap_token, midtrans_transaction_id,
			 midtrans_transaction_status, midraw_response, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11::jsonb,$12,$13)`
	_, err := r.db.Exec(ctx, q,
		p.ID, p.InvoiceID, p.Amount, txID, p.Status, p.PaymentMethod,
		p.MidtransOrderID, p.MidtransSnapToken, p.MidtransTransactionID,
		p.MidtransTransactionStatus, string(rawJSON), p.CreatedAt, p.UpdatedAt,
	)
	if err != nil {
		return errs.Wrap(errs.CodeInternal, "insert payment", err)
	}
	return nil
}

func (r *pgRepo) FindByID(ctx context.Context, id uuid.UUID) (*Payment, error) {
	return r.scanOne(ctx,
		`SELECT id, invoice_id, amount, transaction_id, status, payment_method,
		        midtrans_order_id, midtrans_snap_token, midtrans_transaction_id,
		        midtrans_transaction_status, midraw_response, created_at, updated_at
		 FROM payments WHERE id = $1`, id)
}

func (r *pgRepo) FindByMidtransOrderID(ctx context.Context, orderID string) (*Payment, error) {
	return r.scanOne(ctx,
		`SELECT id, invoice_id, amount, transaction_id, status, payment_method,
		        midtrans_order_id, midtrans_snap_token, midtrans_transaction_id,
		        midtrans_transaction_status, midraw_response, created_at, updated_at
		 FROM payments WHERE midtrans_order_id = $1`, orderID)
}

// scanOne uses the SELECT order: id, invoice_id, amount, transaction_id, status,
// payment_method, midtrans_order_id, midtrans_snap_token, midtrans_transaction_id,
// midtrans_transaction_status, midraw_response, created_at, updated_at.
//
// The Scan target list MUST match exactly.
func (r *pgRepo) scanOne(ctx context.Context, q string, args ...any) (*Payment, error) {
	var p Payment
	var rawJSON []byte
	err := r.db.QueryRow(ctx, q, args...).Scan(
		&p.ID,                        // 1. id
		&p.InvoiceID,                 // 2. invoice_id
		&p.Amount,                    // 3. amount
		new(string),                  // 4. transaction_id (placeholder, not in model)
		&p.Status,                    // 5. status
		&p.PaymentMethod,             // 6. payment_method
		&p.MidtransOrderID,           // 7. midtrans_order_id
		&p.MidtransSnapToken,         // 8. midtrans_snap_token
		&p.MidtransTransactionID,     // 9. midtrans_transaction_id
		&p.MidtransTransactionStatus, // 10. midtrans_transaction_status
		&rawJSON,                     // 11. midraw_response (jsonb)
		&p.CreatedAt,                 // 12. created_at
		&p.UpdatedAt,                 // 13. updated_at
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errs.New(errs.CodePaymentNotFound, "payment not found")
	}
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, "scan payment", err)
	}
	if len(rawJSON) > 0 {
		_ = json.Unmarshal(rawJSON, &p.MidrawResponse)
	}
	return &p, nil
}

// UpdateStatus updates the Midtrans-driven fields on a payment row.
// `raw` is the full Midtrans StatusResponse (stored as JSONB for debugging).
func (r *pgRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status string,
	method *string, mtTxID, mtTxStatus *string, raw any) error {
	rawJSON, _ := json.Marshal(raw)
	now := time.Now().UTC()
	const q = `
		UPDATE payments
		SET status = $1,
		    payment_method = COALESCE($2, payment_method),
		    midtrans_transaction_id = COALESCE($3, midtrans_transaction_id),
		    midtrans_transaction_status = COALESCE($4, midtrans_transaction_status),
		    midraw_response = $5::jsonb,
		    updated_at = $6
		WHERE id = $7`
	tag, err := r.db.Exec(ctx, q, status, method, mtTxID, mtTxStatus, string(rawJSON), now, id)
	if err != nil {
		return errs.Wrap(errs.CodeInternal, "update payment", err)
	}
	if tag.RowsAffected() == 0 {
		return errs.New(errs.CodePaymentNotFound, "payment not found")
	}
	return nil
}
