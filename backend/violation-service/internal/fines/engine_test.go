package fines

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

// Standard rule from the assignment (BASE_RULE) used by the test cases.
func baseRule() Rule {
	r := Rule{
		BaseAmount:      decimal.NewFromInt(150000),
		DayMultiplier:   decimal.NewFromFloat(1.0),
		NightMultiplier: decimal.NewFromFloat(1.5),
		Repeat0:         decimal.NewFromFloat(1.0),
		Repeat1:         decimal.NewFromFloat(1.5),
		Repeat2Plus:     decimal.NewFromFloat(2.0),
	}
	return r
}

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.Parse("2006-01-02T15:04:05", s)
	if err != nil {
		t.Fatalf("bad time: %v", err)
	}
	return v
}

func TestTimeMultiplier_Day(t *testing.T) {
	cases := []string{"06:00:00", "10:00:00", "12:30:00", "21:59:59"}
	for _, c := range cases {
		tt := mustTime(t, "2026-01-01T"+c)
		mult, w := TimeMultiplier(tt)
		one := decimal.NewFromFloat(1.0)
		if !mult.Equal(one) || w != WindowDay {
			t.Errorf("%s: got (%s, %s), want (1.0, DAY)", c, mult.String(), w)
		}
	}
}

func TestTimeMultiplier_Night(t *testing.T) {
	cases := []string{"22:00:00", "23:30:00", "00:00:00", "03:00:00", "05:59:59"}
	for _, c := range cases {
		tt := mustTime(t, "2026-01-01T"+c)
		mult, w := TimeMultiplier(tt)
		one5 := decimal.NewFromFloat(1.5)
		if !mult.Equal(one5) || w != WindowNight {
			t.Errorf("%s: got (%s, %s), want (1.5, NIGHT)", c, mult.String(), w)
		}
	}
}

func TestCalculate_DayNoRepeat(t *testing.T) {
	// TESTING_STRATEGY.md "Daytime Fine": 150000 × 1.0 × 1.0 = 150000
	now := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	got := Calculate(baseRule(), "rv-uuid", 1, "no_parking_zone",
		mustTime(t, "2026-01-01T10:00:00"), 0, now)
	if got.CalculatedFine != 150000 {
		t.Errorf("fine = %v, want 150000", got.CalculatedFine)
	}
	if got.TimeWindow != "DAY" || got.TimeMultiplier != 1.0 {
		t.Errorf("time = %v/%v, want DAY/1.0", got.TimeWindow, got.TimeMultiplier)
	}
}

func TestCalculate_NightNoRepeat(t *testing.T) {
	// 150000 × 1.5 × 1.0 = 225000
	now := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	got := Calculate(baseRule(), "rv-uuid", 1, "no_parking_zone",
		mustTime(t, "2026-01-01T23:00:00"), 0, now)
	if got.CalculatedFine != 225000 {
		t.Errorf("fine = %v, want 225000", got.CalculatedFine)
	}
	if got.TimeWindow != "NIGHT" {
		t.Errorf("window = %v, want NIGHT", got.TimeWindow)
	}
}

func TestCalculate_Repeat1(t *testing.T) {
	// 150000 × 1.0 × 1.5 = 225000
	now := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	got := Calculate(baseRule(), "rv-uuid", 1, "no_parking_zone",
		mustTime(t, "2026-01-01T10:00:00"), 1, now)
	if got.CalculatedFine != 225000 {
		t.Errorf("fine = %v, want 225000", got.CalculatedFine)
	}
}

func TestCalculate_Repeat2Plus(t *testing.T) {
	// 150000 × 1.0 × 2.0 = 300000
	now := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	got := Calculate(baseRule(), "rv-uuid", 1, "no_parking_zone",
		mustTime(t, "2026-01-01T10:00:00"), 2, now)
	if got.CalculatedFine != 300000 {
		t.Errorf("fine = %v, want 300000", got.CalculatedFine)
	}
}

func TestCalculate_NightRepeat2Plus(t *testing.T) {
	// 250000 × 1.5 × 2.0 = 750000 (using repeat_2_plus because priorUnpaid=2)
	r := baseRule()
	r.BaseAmount = decimal.NewFromInt(250000)
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	got := Calculate(r, "rv-uuid", 1, "blocking_hydrant",
		mustTime(t, "2026-05-01T23:30:00"), 2, now)
	if got.CalculatedFine != 750000 {
		t.Errorf("fine = %v, want 750000", got.CalculatedFine)
	}
}

func TestCalculate_NightRepeat1(t *testing.T) {
	// 250000 × 1.5 × 1.5 = 562500 (repeat_1 because priorUnpaid=1)
	r := baseRule()
	r.BaseAmount = decimal.NewFromInt(250000)
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	got := Calculate(r, "rv-uuid", 1, "blocking_hydrant",
		mustTime(t, "2026-05-01T23:30:00"), 1, now)
	if got.CalculatedFine != 562500 {
		t.Errorf("fine = %v, want 562500", got.CalculatedFine)
	}
}

func TestResult_MarshalsToJSON(t *testing.T) {
	now := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	r := Calculate(baseRule(), "rv-uuid", 1, "no_parking_zone",
		mustTime(t, "2026-01-01T10:00:00"), 0, now)
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(b), `"time_window":"DAY"`) {
		t.Errorf("missing DAY in JSON: %s", b)
	}
}
