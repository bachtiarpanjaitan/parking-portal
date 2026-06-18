// Package httpx provides helpers for the standardized success/error envelopes
// defined in .ai/API_CONTRACTS.md and .ai/ERROR_CATALOG.md.
package httpx

import (
	"github.com/gin-gonic/gin"

	"github.com/parking-portal/backend/pkg/errs"
)

// Envelope is the top-level JSON shape for every API response.
type Envelope struct {
	Success bool       `json:"success"`
	Data    any        `json:"data,omitempty"`
	Message string     `json:"message,omitempty"`
	Error   *ErrorBody `json:"error,omitempty"`
	Meta    *Meta      `json:"meta,omitempty"`
}

// ErrorBody is the error sub-object.
type ErrorBody struct {
	Code    errs.Code      `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// Meta is for paginated responses (items live in Data, totals in Meta).
type Meta struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
	Total    int `json:"total"`
}

// Paginated wraps a slice + pagination meta into the standard envelope.
// Use c.JSON(http.StatusOK, httpx.Paginated(items, page, pageSize, total)).
func Paginated(items any, page, pageSize, total int) Envelope {
	return Envelope{
		Success: true,
		Data:    items,
		Meta:    &Meta{Page: page, PageSize: pageSize, Total: total},
	}
}

// OK wraps a successful payload.
func OK(data any) Envelope {
	return Envelope{Success: true, Data: data}
}

// OKWithMessage wraps a successful payload with an extra message.
func OKWithMessage(data any, message string) Envelope {
	return Envelope{Success: true, Data: data, Message: message}
}

// ErrorEnvelope builds the error envelope from a typed error.
func ErrorEnvelope(ae *errs.AppError) Envelope {
	return Envelope{
		Success: false,
		Error: &ErrorBody{
			Code:    ae.ErrCode,
			Message: ae.Message,
			Details: ae.Details,
		},
	}
}

// WriteError is a convenience used by the ErrorTranslator middleware.
// It also Aborts the gin context so no further handlers run.
func WriteError(c *gin.Context, ae *errs.AppError) {
	c.AbortWithStatusJSON(ae.HTTPStatusCode, ErrorEnvelope(ae))
}
