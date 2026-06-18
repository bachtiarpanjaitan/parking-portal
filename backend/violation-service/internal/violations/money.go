package violations

import "github.com/shopspring/decimal"

// decimalFromFloat converts a float64 fine amount to a decimal for storage.
func decimalFromFloat(f float64) decimal.Decimal {
	return decimal.NewFromFloat(f)
}
