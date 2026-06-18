// Package fines implements the fine calculation engine.
// It is a pure-function module: no DB, no HTTP. The violations module
// calls Calculate(...) and persists the result. See .ai/BUSINESS_RULES.md
// for the formulas and .ai/TESTING_STRATEGY.md for the test cases.
package fines

import (
	"time"

	"github.com/shopspring/decimal"
)

// TimeWindow is the human-readable label for the time multiplier decision.
type TimeWindow string

const (
	WindowDay   TimeWindow = "DAY"
	WindowNight TimeWindow = "NIGHT"
)

// TimeMultiplier returns the multiplier to apply for the local time portion
// of the violation. Uses the half-open intervals documented in
// BUSINESS_RULES.md: 06:00:00–21:59:59 is DAY (1.0), 22:00:00–05:59:59 is NIGHT (1.5).
//
// This supersedes the ambiguous `06:00 – 22:00` wording in the assignment PDF.
func TimeMultiplier(localTime time.Time) (decimal.Decimal, TimeWindow) {
	h, m, s := localTime.Hour(), localTime.Minute(), localTime.Second()
	total := h*3600 + m*60 + s
	if total >= 6*3600 && total < 22*3600 {
		return decimal.NewFromFloat(1.0), WindowDay
	}
	return decimal.NewFromFloat(1.5), WindowNight
}
