// Package decimal provides helpers for the shopspring/decimal library so we
// have a single place for money math conventions.
package decimal

import (
	"github.com/shopspring/decimal"
)

// Zero is exposed for convenience.
var Zero = decimal.Zero

// New parses a string into a decimal. Returns an error for invalid input.
func New(s string) (decimal.Decimal, error) {
	return decimal.NewFromString(s)
}

// MustNew panics on parse error. Use only with hard-coded literals.
func MustNew(s string) decimal.Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		panic(err)
	}
	return d
}

// ToIDRInt converts a decimal to the int64 IDR representation stored in
// the DB (no subunits). Truncates fractional amounts.
func ToIDRInt(d decimal.Decimal) int64 {
	return d.IntPart()
}

// FromIDRInt converts an int64 IDR value back to a decimal.
func FromIDRInt(v int64) decimal.Decimal {
	return decimal.NewFromInt(v)
}

// Round2 rounds a decimal to 2 decimal places (banker's rounding).
func Round2(d decimal.Decimal) decimal.Decimal {
	return d.Round(2)
}
