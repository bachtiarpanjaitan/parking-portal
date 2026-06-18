// Package midtrans is a thin HTTP client for the Midtrans Snap API.
// See ADR-012 for the design.
//
// We support two modes:
//  1. Real Midtrans (server key starts with "SB-Mid-server-" or "Mid-server-")
//  2. Mock fallback (server key starts with "MOCK_") — returns a fake token
//     so the rest of the flow can be exercised without hitting Midtrans.
//
// Endpoints used:
//   - POST  https://{snap-host}/snap/v1/transactions      (create Snap token)
//   - GET   https://{api-host}/v2/{order_id}/status        (check status)
package midtrans

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Client is the Midtrans HTTP client. Safe for concurrent use.
type Client struct {
	serverKey       string
	env             string // "sandbox" or "production"
	snapBaseURL     string
	apiBaseURL      string
	http            *http.Client
	methods         []string
	notificationURL string
	returnURL       string
}

// SnapRequest is the body of POST /snap/v1/transactions.
type SnapRequest struct {
	TransactionDetails TransactionDetails `json:"transaction_details"`
	EnabledPayments    []string           `json:"enabled_payments"`
	Callbacks          *Callbacks         `json:"callbacks,omitempty"`
}

type TransactionDetails struct {
	OrderID  string `json:"order_id"`
	GrossAmt int64  `json:"gross_amount"` // IDR, no subunits
}

type Callbacks struct {
	Finish string `json:"finish,omitempty"` // redirect URL
}

// SnapResponse is the success body from Midtrans.
type SnapResponse struct {
	Token       string `json:"token"`
	RedirectURL string `json:"redirect_url"`
	OrderID     string `json:"order_id"`
}

// StatusResponse is the body of GET /v2/{order_id}/status.
type StatusResponse struct {
	OrderID           string `json:"order_id"`
	TransactionID     string `json:"transaction_id"`
	TransactionStatus string `json:"transaction_status"` // capture|settlement|pending|deny|cancel|expire|refund
	StatusCode        string `json:"status_code"`
	PaymentType       string `json:"payment_type"` // gopay, qris, etc.
	GrossAmount       string `json:"gross_amount"`
	FraudStatus       string `json:"fraud_status"`
}

// NewClient constructs a Midtrans client. If serverKey starts with "MOCK_",
// the client operates in mock mode (no HTTP calls).
func NewClient(serverKey, env, notificationURL, returnURL string, methods []string, timeout time.Duration) *Client {
	c := &Client{
		serverKey:       serverKey,
		env:             env,
		http:            &http.Client{Timeout: timeout},
		methods:         methods,
		notificationURL: notificationURL,
		returnURL:       returnURL,
	}
	if env == "production" {
		c.snapBaseURL = "https://app.midtrans.com"
		c.apiBaseURL = "https://api.midtrans.com"
	} else {
		c.snapBaseURL = "https://app.sandbox.midtrans.com"
		c.apiBaseURL = "https://api.sandbox.midtrans.com"
	}
	return c
}

// IsMock returns true if the client is running in mock mode.
func (c *Client) IsMock() bool { return strings.HasPrefix(c.serverKey, "MOCK_") }

// EnabledMethods returns the configured enabled payment methods.
func (c *Client) EnabledMethods() []string { return c.methods }

// CreateSnapToken creates a Snap transaction. Returns token, redirect URL,
// and the order_id we sent (which Midtrans echoes back).
func (c *Client) CreateSnapToken(ctx context.Context, orderID string, grossAmount int64) (*SnapResponse, error) {
	if c.IsMock() {
		return &SnapResponse{
			Token:       "MOCK-snap-token-" + uuid.NewString(),
			RedirectURL: c.returnURL + "?order_id=" + orderID,
			OrderID:     orderID,
		}, nil
	}
	body := SnapRequest{
		TransactionDetails: TransactionDetails{OrderID: orderID, GrossAmt: grossAmount},
		EnabledPayments:    c.methods,
	}
	if c.notificationURL != "" || c.returnURL != "" {
		body.Callbacks = &Callbacks{Finish: c.returnURL}
	}
	bs, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal snap request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST",
		c.snapBaseURL+"/snap/v1/transactions", bytes.NewReader(bs))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth(c.serverKey, "")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("snap request: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("snap error %d: %s", resp.StatusCode, string(respBody))
	}
	var out SnapResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("decode snap response: %w", err)
	}
	return &out, nil
}

// GetStatus fetches the current transaction status from Midtrans.
// In mock mode, returns a fake "settlement" response.
func (c *Client) GetStatus(ctx context.Context, orderID string) (*StatusResponse, error) {
	if c.IsMock() {
		return &StatusResponse{
			OrderID:           orderID,
			TransactionID:     "MOCK-trx-" + uuid.NewString(),
			TransactionStatus: "settlement",
			StatusCode:        "200",
			PaymentType:       "qris",
			GrossAmount:       "0",
			FraudStatus:       "accept",
		}, nil
	}
	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/v2/%s/status", c.apiBaseURL, orderID), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth(c.serverKey, "")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("status request: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("status error %d: %s", resp.StatusCode, string(respBody))
	}
	var out StatusResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("decode status response: %w", err)
	}
	return &out, nil
}
