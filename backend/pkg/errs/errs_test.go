package errs

import (
	"errors"
	"net/http"
	"testing"
)

func TestCode_HTTPStatus(t *testing.T) {
	tests := []struct {
		code Code
		want int
	}{
		{CodeValidation, http.StatusBadRequest},
		{CodeUnauthorized, http.StatusUnauthorized},
		{CodeForbidden, http.StatusForbidden},
		{CodeViolationNotFound, http.StatusNotFound},
		{CodeInvoiceAlreadyPaid, http.StatusConflict},
		{CodeNoActiveRule, http.StatusUnprocessableEntity},
		{CodeInternal, http.StatusInternalServerError},
		{Code("UNKNOWN"), http.StatusInternalServerError},
	}
	for _, tt := range tests {
		if got := tt.code.HTTPStatus(); got != tt.want {
			t.Errorf("Code(%s).HTTPStatus() = %d, want %d", tt.code, got, tt.want)
		}
	}
}

func TestNew(t *testing.T) {
	e := New(CodeValidation, "bad input")
	if e.ErrCode != CodeValidation || e.Message != "bad input" {
		t.Errorf("unexpected: %+v", e)
	}
	if e.HTTPStatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", e.HTTPStatusCode)
	}
}

func TestWrap_Unwrap(t *testing.T) {
	cause := errors.New("db down")
	e := Wrap(CodeInternal, "internal", cause)
	if errors.Unwrap(e) != cause {
		t.Errorf("Unwrap did not return cause")
	}
}

func TestAsAppError(t *testing.T) {
	e := New(CodeForbidden, "x")
	var err error = e
	got, ok := AsAppError(err)
	if !ok || got != e {
		t.Errorf("AsAppError failed")
	}
	_, ok = AsAppError(errors.New("plain"))
	if ok {
		t.Errorf("AsAppError on plain error should return false")
	}
}

func TestWithDetails(t *testing.T) {
	e := New(CodeValidation, "bad").WithDetails(map[string]any{
		"license_plate": []string{"required"},
	})
	if len(e.Details) != 1 {
		t.Errorf("details not set")
	}
}
