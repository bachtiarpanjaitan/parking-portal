// Package history exposes the aggregated history view.
//
// Per .ai/MODULES.md → "History Module" and .ai/MODULE_WORKFLOW.md → Flow 5:
// the history endpoint joins violations ⨝ invoices ⨝ latest payment ⨝
// fine_rule_versions and returns one row per violation with the full
// calculation_snapshot, the rule version that was applied, and the
// invoice + payment status.
//
// The history view is the **assignment Flow 5**.
package history

import "time"

// Entry is the shape returned by GET /history. Every field is a snapshot
// of the state at the time of the violation (rule version is frozen; the
// calculation_snapshot never changes).
//
// PaymentTxStatus is the Midtrans `transaction_status` of the latest
// payment attempt: capture / settlement / pending / deny / cancel /
// expire / refund. Replaces the old pre-Midtrans `scenario` field.
type Entry struct {
	ViolationID         string    `json:"violation_id"`
	MemberID            string    `json:"member_id"`
	LicensePlate        string    `json:"license_plate"`
	ViolationType       string    `json:"violation_type"`
	Location            string    `json:"location"`
	ViolationTS         time.Time `json:"violation_timestamp"`
	FineAmount          float64   `json:"fine_amount"`
	PhotoURL            string    `json:"photo_url"`
	RuleVersionID       string    `json:"rule_version_id"`
	RuleVersionNumber   int       `json:"rule_version_number"`
	InvoiceID           string    `json:"invoice_id"`
	InvoiceStatus       string    `json:"invoice_status"`
	InvoiceAmount       float64   `json:"invoice_amount"`
	PaymentStatus       string    `json:"payment_status,omitempty"`
	PaymentTxStatus     string    `json:"payment_tx_status,omitempty"`
	CalculationSnapshot any       `json:"calculation_snapshot"`
}
