package fines

import (
	"time"

	"github.com/shopspring/decimal"
)

// Rule is the subset of FineRuleDetail needed by the engine.
// Defined as a local type so the engine has no DB import.
type Rule struct {
	BaseAmount      decimal.Decimal
	DayMultiplier   decimal.Decimal
	NightMultiplier decimal.Decimal
	Repeat0         decimal.Decimal
	Repeat1         decimal.Decimal
	Repeat2Plus     decimal.Decimal
}

// Result is the output of a single fine calculation. Persisted in
// `violations.calculation_snapshot` as JSON.
type Result struct {
	RuleVersionID     string  `json:"rule_version_id"`
	RuleVersionNumber int     `json:"rule_version_number"`
	ViolationType     string  `json:"violation_type"`
	BaseAmount        float64 `json:"base_amount"`
	TimeMultiplier    float64 `json:"time_multiplier"`
	TimeWindow        string  `json:"time_window"`
	RepeatMultiplier  float64 `json:"repeat_multiplier"`
	PriorUnpaidCount  int     `json:"prior_unpaid_count"`
	CalculatedFine    float64 `json:"calculated_fine"`
	CalculatedAt      string  `json:"calculated_at"`
}

// RepeatMultiplier returns the multiplier to apply for a given prior-unpaid count.
// 0 → 1.0, 1 → 1.5, 2+ → 2.0.
func (r Rule) RepeatMultiplier(priorUnpaid int) decimal.Decimal {
	switch {
	case priorUnpaid <= 0:
		return r.Repeat0
	case priorUnpaid == 1:
		return r.Repeat1
	default:
		return r.Repeat2Plus
	}
}

// Calculate computes the fine for a violation. The localTime parameter is the
// violation_timestamp in local time (the engine evaluates the time window
// against local time, see BUSINESS_RULES.md).
func Calculate(rule Rule, ruleVersionID string, ruleVersionNumber int,
	violationType string, localTime time.Time, priorUnpaid int, now time.Time) Result {

	timeMult, window := TimeMultiplier(localTime)
	repeatMult := rule.RepeatMultiplier(priorUnpaid)

	// fine = base * time_mult * repeat_mult, rounded to 2 dp
	fine := rule.BaseAmount.
		Mul(timeMult).
		Mul(repeatMult).
		Round(2)

	return Result{
		RuleVersionID:     ruleVersionID,
		RuleVersionNumber: ruleVersionNumber,
		ViolationType:     violationType,
		BaseAmount:        decToF(rule.BaseAmount),
		TimeMultiplier:    decToF(timeMult),
		TimeWindow:        string(window),
		RepeatMultiplier:  decToF(repeatMult),
		PriorUnpaidCount:  priorUnpaid,
		CalculatedFine:    decToF(fine),
		CalculatedAt:      now.UTC().Format(time.RFC3339),
	}
}

// decToF is a small helper to keep the Result struct using float64
// for JSON readability. (We keep decimal.Decimal internally for math safety.)
func decToF(d decimal.Decimal) float64 {
	f, _ := d.Float64()
	return f
}
