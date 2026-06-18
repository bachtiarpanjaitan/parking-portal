// Package errs defines the standardized error envelope used by all HTTP services.
// The ErrorTranslator middleware (see pkg/middleware) catches *AppError returned
// from any layer and writes the JSON response defined in .ai/ERROR_CATALOG.md.
package errs

import (
	"errors"
	"fmt"
	"net/http"
)

// Code is a machine-readable error code. The set of allowed values is defined
// in .ai/ERROR_CATALOG.md. Always uppercase with underscores.
type Code string

const (
	// Generic
	CodeValidation   Code = "VALIDATION_ERROR"
	CodeUnauthorized Code = "UNAUTHORIZED"
	CodeInvalidToken Code = "INVALID_TOKEN"
	CodeTokenExpired Code = "TOKEN_EXPIRED"
	CodeForbidden    Code = "FORBIDDEN"
	CodeNotFound     Code = "RESOURCE_NOT_FOUND"
	CodeConflict     Code = "CONFLICT"
	CodeInternal     Code = "INTERNAL_SERVER_ERROR"
	CodeBusinessRule Code = "BUSINESS_RULE_VIOLATION"

	// Rule management
	CodeRuleNotFound      Code = "RULE_VERSION_NOT_FOUND"
	CodeRuleAlreadyActive Code = "RULE_ALREADY_ACTIVE"
	CodeNoActiveRule      Code = "NO_ACTIVE_RULE"

	// Violations
	CodeViolationNotFound    Code = "VIOLATION_NOT_FOUND"
	CodeInvalidViolationType Code = "INVALID_VIOLATION_TYPE"
	CodeLicensePlateRequired Code = "LICENSE_PLATE_REQUIRED"
	CodePhotoRequired        Code = "PHOTO_REQUIRED"

	// Invoices
	CodeInvoiceNotFound    Code = "INVOICE_NOT_FOUND"
	CodeInvoiceAlreadyPaid Code = "INVOICE_ALREADY_PAID"
	CodeInvalidInvStatus   Code = "INVALID_INVOICE_STATUS"

	// Payments
	CodePaymentFailed   Code = "PAYMENT_FAILED"
	CodePaymentNotFound Code = "PAYMENT_NOT_FOUND"
	CodeInvalidScenario Code = "INVALID_PAYMENT_SCENARIO"

	// Uploads
	CodeFileRequired     Code = "FILE_REQUIRED"
	CodeFileTooLarge     Code = "FILE_TOO_LARGE"
	CodeInvalidFileType  Code = "INVALID_FILE_TYPE"
	CodeFileUploadFailed Code = "FILE_UPLOAD_FAILED"

	// Notifications
	CodeEventPublishFailed    Code = "EVENT_PUBLISH_FAILED"
	CodeEventProcessingFailed Code = "EVENT_PROCESSING_FAILED"
)

// httpStatus maps a Code to its HTTP status. See .ai/ERROR_CATALOG.md.
var httpStatus = map[Code]int{
	CodeValidation:            http.StatusBadRequest,
	CodeUnauthorized:          http.StatusUnauthorized,
	CodeInvalidToken:          http.StatusUnauthorized,
	CodeTokenExpired:          http.StatusUnauthorized,
	CodeForbidden:             http.StatusForbidden,
	CodeNotFound:              http.StatusNotFound,
	CodeConflict:              http.StatusConflict,
	CodeBusinessRule:          http.StatusUnprocessableEntity,
	CodeRuleNotFound:          http.StatusNotFound,
	CodeRuleAlreadyActive:     http.StatusConflict,
	CodeNoActiveRule:          http.StatusUnprocessableEntity,
	CodeViolationNotFound:     http.StatusNotFound,
	CodeInvalidViolationType:  http.StatusBadRequest,
	CodeLicensePlateRequired:  http.StatusBadRequest,
	CodePhotoRequired:         http.StatusBadRequest,
	CodeInvoiceNotFound:       http.StatusNotFound,
	CodeInvoiceAlreadyPaid:    http.StatusConflict,
	CodeInvalidInvStatus:      http.StatusUnprocessableEntity,
	CodePaymentFailed:         http.StatusUnprocessableEntity,
	CodePaymentNotFound:       http.StatusNotFound,
	CodeInvalidScenario:       http.StatusBadRequest,
	CodeFileRequired:          http.StatusBadRequest,
	CodeFileTooLarge:          http.StatusBadRequest,
	CodeInvalidFileType:       http.StatusBadRequest,
	CodeFileUploadFailed:      http.StatusInternalServerError,
	CodeEventPublishFailed:    http.StatusInternalServerError,
	CodeEventProcessingFailed: http.StatusInternalServerError,
	CodeInternal:              http.StatusInternalServerError,
}

// HTTPStatus returns the canonical HTTP status for a Code.
func (c Code) HTTPStatus() int {
	if s, ok := httpStatus[c]; ok {
		return s
	}
	return http.StatusInternalServerError
}

// AppError is a typed error that the ErrorTranslator middleware can inspect
// to produce a standardized JSON response. See .ai/ERROR_CATALOG.md.
type AppError struct {
	HTTPStatusCode int            `json:"-"`
	ErrCode        Code           `json:"code"`
	Message        string         `json:"message"`
	Details        map[string]any `json:"details,omitempty"`
	Cause          error          `json:"-"`
}

// Error implements the error interface. Includes the wrapped cause when present.
func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.ErrCode, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.ErrCode, e.Message)
}

// Unwrap allows errors.Is / errors.As to see the wrapped cause.
func (e *AppError) Unwrap() error { return e.Cause }

// New creates a new AppError with no wrapped cause.
func New(code Code, msg string) *AppError {
	return &AppError{
		HTTPStatusCode: code.HTTPStatus(),
		ErrCode:        code,
		Message:        msg,
	}
}

// Wrap creates a new AppError wrapping an underlying cause.
func Wrap(code Code, msg string, cause error) *AppError {
	return &AppError{
		HTTPStatusCode: code.HTTPStatus(),
		ErrCode:        code,
		Message:        msg,
		Cause:          cause,
	}
}

// WithDetails attaches a details map (e.g. validation field errors).
func (e *AppError) WithDetails(details map[string]any) *AppError {
	e.Details = details
	return e
}

// AsAppError returns the *AppError inside err, or nil if err is not one.
// It also returns true if err is (or wraps) an *AppError.
func AsAppError(err error) (*AppError, bool) {
	var ae *AppError
	if errors.As(err, &ae) {
		return ae, true
	}
	return nil, false
}
