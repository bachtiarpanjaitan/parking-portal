// Package violations manages parking violations. See .ai/MODULES.md and
// .ai/MODULE_WORKFLOW.md → Flow 1.
package violations

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Violation is a recorded parking violation.
type Violation struct {
	ID                  uuid.UUID       `json:"id"`
	MemberID            uuid.UUID       `json:"member_id"`
	RuleVersionID       uuid.UUID       `json:"rule_version_id"`
	RuleVersionNumber   int             `json:"rule_version_number"`
	LicensePlate        string          `json:"license_plate"`
	ViolationType       string          `json:"violation_type"`
	Location            string          `json:"location"`
	ViolationTimestamp  time.Time       `json:"violation_timestamp"`
	PhotoURL            string          `json:"photo_url"`
	FineAmount          decimal.Decimal `json:"fine_amount"`
	CalculationSnapshot map[string]any  `json:"calculation_snapshot"`
	InvoiceID           *uuid.UUID      `json:"invoice_id,omitempty"`
	InvoiceStatus       string          `json:"invoice_status,omitempty"`
	CreatedAt           time.Time       `json:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at"`
}
