// Package rules manages fine rule versions. See .ai/DATABASE_MAPPING.md and
// .ai/MODULES.md → "Rule Management".
package rules

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Version is a rule version header.
type Version struct {
	ID            uuid.UUID `json:"id"`
	VersionNumber int       `json:"version_number"`
	IsActive      bool      `json:"is_active"`
	PublishedAt   time.Time `json:"published_at"`
	CreatedBy     uuid.UUID `json:"created_by"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Detail is one (rule_version_id, violation_type) row.
type Detail struct {
	ID              uuid.UUID       `json:"id"`
	RuleVersionID   uuid.UUID       `json:"rule_version_id"`
	ViolationType   string          `json:"violation_type"`
	BaseAmount      decimal.Decimal `json:"base_amount"`
	DayMultiplier   decimal.Decimal `json:"day_multiplier"`
	NightMultiplier decimal.Decimal `json:"night_multiplier"`
	Repeat0         decimal.Decimal `json:"repeat_0"`
	Repeat1         decimal.Decimal `json:"repeat_1"`
	Repeat2Plus     decimal.Decimal `json:"repeat_2_plus"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// VersionWithDetails bundles a Version and its 4 Details.
type VersionWithDetails struct {
	Version
	Details []Detail `json:"details"`
}
