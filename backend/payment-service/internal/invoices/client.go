// Package invoices is a thin HTTP client for the Violation Service's
// invoice endpoints. The Payment Service uses it to:
//  1. GET /invoices/{id}        — fetch amount + status + member_id (auth check)
//  2. PUT /invoices/{id}/status — set status to PAID or FAILED after webhook
package invoices

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Invoice is the shape we get back from the violation-service.
// Mirrors the Invoice model there (kept in sync manually).
//
// The violation-service serializes `amount` as a JSON string
// (because it's shopspring/decimal.Decimal). We accept it as decimal
// and convert to int64 (IDR has no subunits) when needed.
type Invoice struct {
	ID        uuid.UUID       `json:"id"`
	MemberID  uuid.UUID       `json:"member_id"`
	Amount    decimal.Decimal `json:"amount"`
	Status    string          `json:"status"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// AmountInt returns the amount as int64 (IDR has no subunits).
func (i *Invoice) AmountInt() int64 { return i.Amount.IntPart() }

// Client wraps the HTTP client and the base URL.
type Client struct {
	baseURL string
	http    *http.Client
	token   string // pre-minted service-to-service JWT
}

// NewClient returns a new client. token is a JWT signed with the same
// JWT_SECRET the violation-service uses. The simplest way to mint one is
// to call POST /auth/login with a dedicated service user; for the slice
// we accept whatever the caller provides.
func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL: baseURL,
		http:    &http.Client{Timeout: 10 * time.Second},
		token:   token,
	}
}

// GetInvoice calls GET /api/v1/invoices/{id}.
func (c *Client) GetInvoice(ctx context.Context, id uuid.UUID) (*Invoice, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/v1/invoices/"+id.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get invoice: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("invoice not found")
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("get invoice %d: %s", resp.StatusCode, string(body))
	}
	// Response envelope: { success, data: { id, member_id, amount, status, ... } }
	var env struct {
		Success bool     `json:"success"`
		Data    *Invoice `json:"data"`
		Error   *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("decode invoice: %w", err)
	}
	if !env.Success || env.Data == nil {
		if env.Error != nil {
			return nil, fmt.Errorf("invoice api: %s: %s", env.Error.Code, env.Error.Message)
		}
		return nil, fmt.Errorf("invoice api: unknown error: %s", string(body))
	}
	return env.Data, nil
}

// SetStatus calls PATCH /api/v1/invoices/{id}/status with a body of
// {"status": "<PENDING|PAID|FAILED|CANCELLED>"}.
//
// The endpoint is implemented by the violation-service and is gated to
// OFFICER-role tokens; the payment service mints a service-to-service JWT
// in main.go for exactly this purpose.
//
// 409 (CONFLICT) responses — typically "PAID → PAID is no-op" or
// "cannot change a PAID invoice" — are treated as soft-success: the local
// payment row already reflects the correct outcome, so we don't fail the
// webhook / refresh.
func (c *Client) SetStatus(ctx context.Context, id uuid.UUID, status string) error {
	body, _ := json.Marshal(map[string]string{"status": status})
	req, err := http.NewRequestWithContext(ctx, "PATCH",
		c.baseURL+"/api/v1/invoices/"+id.String()+"/status", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("set status: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return nil
	case resp.StatusCode == 409:
		// Already PAID, or invalid transition — soft success.
		return nil
	default:
		return fmt.Errorf("set status %d: %s", resp.StatusCode, string(respBody))
	}
}
