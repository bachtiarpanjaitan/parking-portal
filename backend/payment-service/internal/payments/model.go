// Package payments owns the Midtrans-backed payment flow. See ADR-012.
package payments

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Payment is the local DB record for a Midtrans transaction attempt.
type Payment struct {
	ID                        uuid.UUID       `json:"id"`
	InvoiceID                 uuid.UUID       `json:"invoice_id"`
	Amount                    decimal.Decimal `json:"amount"`
	Status                    string          `json:"status"`
	PaymentMethod             *string         `json:"payment_method,omitempty"`
	MidtransOrderID           string          `json:"midtrans_order_id"`
	MidtransSnapToken         *string         `json:"midtrans_snap_token,omitempty"`
	MidtransTransactionID     *string         `json:"midtrans_transaction_id,omitempty"`
	MidtransTransactionStatus *string         `json:"midtrans_transaction_status,omitempty"`
	MidrawResponse            any             `json:"midraw_response,omitempty"`
	CreatedAt                 time.Time       `json:"created_at"`
	UpdatedAt                 time.Time       `json:"updated_at"`
}

// CreateSnapTokenRequest is the body for POST /payments/snap-token.
type CreateSnapTokenRequest struct {
	InvoiceID uuid.UUID `json:"invoice_id" validate:"required"`
}

// CreateSnapTokenResponse is returned to the client.
type CreateSnapTokenResponse struct {
	PaymentID   uuid.UUID  `json:"payment_id"`
	OrderID     string     `json:"order_id"`
	SnapToken   string     `json:"snap_token"`
	RedirectURL string     `json:"redirect_url"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}
