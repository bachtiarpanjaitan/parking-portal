// Package invoices manages invoices. See .ai/MODULES.md and Flow 1.
package invoices

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Invoice is a bill generated from a violation.
type Invoice struct {
	ID          uuid.UUID       `json:"id"`
	ViolationID uuid.UUID       `json:"violation_id"`
	MemberID    uuid.UUID       `json:"member_id"`
	Amount      decimal.Decimal `json:"amount"`
	Status      string          `json:"status"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// LatestPayment is the latest payment attempt for a given invoice.
// (replaces the old `Scenario` field with `PaymentMethod` after the
// Midtrans migration).
type LatestPayment struct {
	ID            uuid.UUID `json:"id"`
	Status        string    `json:"status"`
	TransactionID string    `json:"transaction_id"`
	PaymentMethod *string   `json:"payment_method,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// InvoiceWithLatest bundles an Invoice with its latest payment (may be nil).
type InvoiceWithLatest struct {
	Invoice
	LatestPayment *LatestPayment `json:"latest_payment,omitempty"`
}

// InvoiceListItem is the row shape returned by GET /invoices. It embeds
// Invoice and flattens the joined violation fields the UI needs to render
// the table (license plate, violation type, location, photo, etc.) so the
// frontend does not have to N+1 a second request per row.
type InvoiceListItem struct {
	Invoice
	// Violation snapshot — joined from the violations table.
	LicensePlate        string    `json:"license_plate"`
	ViolationType       string    `json:"violation_type"`
	Location            string    `json:"location"`
	ViolationTimestamp  time.Time `json:"violation_timestamp"`
	PhotoURL            string    `json:"photo_url"`
	RuleVersionNumber   int       `json:"rule_version_number"`
}
