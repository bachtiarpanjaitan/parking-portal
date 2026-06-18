package payments

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/parking-portal/backend/payment-service/internal/invoices"
	"github.com/parking-portal/backend/payment-service/internal/midtrans"
	"github.com/parking-portal/backend/pkg/errs"
)

// InvoiceFetcher is implemented by *invoices.Client.
type InvoiceFetcher interface {
	GetInvoice(ctx context.Context, id uuid.UUID) (*invoices.Invoice, error)
	SetStatus(ctx context.Context, id uuid.UUID, status string) error
}

// EventPublisher publishes PaymentSucceeded/Failed to RabbitMQ.
type EventPublisher interface {
	PublishPaymentSucceeded(ctx context.Context, payload any) error
	PublishPaymentFailed(ctx context.Context, payload any) error
}

// Service is the heart of the Midtrans integration.
type Service struct {
	repo   Repository
	inv    InvoiceFetcher
	mid    *midtrans.Client
	events EventPublisher
}

func NewService(repo Repository, inv InvoiceFetcher, mid *midtrans.Client, ev EventPublisher) *Service {
	return &Service{repo: repo, inv: inv, mid: mid, events: ev}
}

// CreateSnapToken implements POST /payments/snap-token.
//
// Defensive ordering (the cross-service invoice status can lag the local
// payment DB by a few hundred ms after a webhook/refresh settles the
// payment, but the *local* payment row is the source of truth):
//
//  1. Look up the local payment for this invoice first.
//     - If a PAID payment exists → return its id (no snap_token) so the
//     client refetches and shows PAID. We never mint a second order.
//     - If a PENDING payment exists → re-sync with Midtrans and either
//     reuse the snap_token or return the new terminal status.
//     - If FAILED/EXPIRED → fall through to create a new payment.
//  2. Fetch the invoice to authorize the member and validate the status.
//     - PENDING / FAILED are payable. PAID → 409. CANCELLED → 422.
//  3. Mint a fresh Midtrans order_id + snap_token, persist a new payment.
func (s *Service) CreateSnapToken(ctx context.Context, memberID uuid.UUID, invoiceID uuid.UUID) (*CreateSnapTokenResponse, error) {
	// 1. Existing-payment guard (source of truth).
	existing, err := s.repo.FindByInvoiceID(ctx, invoiceID)
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return nil, err
	}
	if existing != nil {
		switch existing.Status {
		case "PAID":
			// Idempotent: don't mint a new order, just tell the client to refetch.
			return &CreateSnapTokenResponse{
				PaymentID: existing.ID,
				OrderID:   existing.MidtransOrderID,
			}, nil
		case "PENDING":
			// Refresh from Midtrans and return existing token (or new status).
			refreshed, rErr := s.RefreshPayment(ctx, existing.ID)
			if rErr != nil {
				return nil, rErr
			}
			if refreshed.Status == "PENDING" && refreshed.MidtransSnapToken != nil {
				return &CreateSnapTokenResponse{
					PaymentID: refreshed.ID,
					OrderID:   refreshed.MidtransOrderID,
					SnapToken: *refreshed.MidtransSnapToken,
				}, nil
			}
			if refreshed.Status == "PAID" {
				return &CreateSnapTokenResponse{
					PaymentID: refreshed.ID,
					OrderID:   refreshed.MidtransOrderID,
				}, nil
			}
			// FAILED/EXPIRED/CANCELLED → fall through to create new.
		}
	}

	// 2. Fetch the invoice (only if we are about to create a new payment).
	inv, err := s.inv.GetInvoice(ctx, invoiceID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, errs.New(errs.CodeInvoiceNotFound, "invoice not found")
		}
		return nil, errs.Wrap(errs.CodeInternal, "fetch invoice", err)
	}
	// 2a. Authorization: invoice must belong to the calling member.
	if inv.MemberID != memberID {
		return nil, errs.New(errs.CodeForbidden, "cannot pay another member's invoice")
	}
	// 2b. Status: PENDING or FAILED is payable; PAID / CANCELLED are not.
	switch inv.Status {
	case "PAID":
		return nil, errs.New(errs.CodeInvoiceAlreadyPaid, "invoice already paid")
	case "CANCELLED":
		return nil, errs.New(errs.CodeInvalidInvStatus, "invoice is cancelled")
	}

	// 3. Mint a fresh Midtrans order_id (Midtrans rejects duplicate
	//    order_ids, so we always generate a new one for new attempts).
	orderID := fmt.Sprintf("ORD-%s-%s", invoiceID.String()[:8], uuid.NewString()[:6])

	// 4. Create the Snap token via Midtrans.
	snap, err := s.mid.CreateSnapToken(ctx, orderID, inv.AmountInt())
	if err != nil {
		return nil, errs.New(errs.CodeInternal, "midtrans snap: "+err.Error())
	}

	// 5. Persist the new payment row. If a FAILED/EXPIRED/CANCELLED row
	//    already exists for this invoice, replace it so we never end up
	//    with >1 row per invoice.
	now := time.Now().UTC()
	token := snap.Token
	p := &Payment{
		ID:                uuid.New(),
		InvoiceID:         invoiceID,
		Amount:            inv.Amount,
		Status:            "PENDING",
		MidtransOrderID:   snap.OrderID,
		MidtransSnapToken: &token,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	p.MidrawResponse = snap
	if existing != nil {
		if err := s.repo.ReplaceByInvoiceID(ctx, p); err != nil {
			return nil, err
		}
	} else {
		if err := s.repo.Insert(ctx, p); err != nil {
			return nil, err
		}
	}

	return &CreateSnapTokenResponse{
		PaymentID:   p.ID,
		OrderID:     p.MidtransOrderID,
		SnapToken:   snap.Token,
		RedirectURL: snap.RedirectURL,
	}, nil
}

// HandleNotification implements POST /payments/notification (Midtrans webhook).
// The handler reads the body and forwards the order_id here.
func (s *Service) HandleNotification(ctx context.Context, orderID string) error {
	// 1. Find local payment by order_id.
	p, err := s.repo.FindByMidtransOrderID(ctx, orderID)
	if err != nil {
		return err
	}
	// 2. Verify with Midtrans (the webhook body is a hint, not source of truth).
	status, err := s.mid.GetStatus(ctx, orderID)
	if err != nil {
		return errs.Wrap(errs.CodeInternal, "midtrans get status", err)
	}
	// 3. Map Midtrans transaction_status → our enum.
	newStatus, invStatus := s.mapStatus(status.TransactionStatus)
	method := status.PaymentType
	txID := status.TransactionID
	txStatus := status.TransactionStatus
	// 4. Update the local payment row.
	if err := s.repo.UpdateStatus(ctx, p.ID, newStatus, &method, &txID, &txStatus, status); err != nil {
		return err
	}
	// 5. Update the invoice status via the violation-service (best-effort).
	if err := s.inv.SetStatus(ctx, p.InvoiceID, invStatus); err != nil {
		// Don't fail the webhook on a downstream error; we already updated locally.
		// (In production we'd queue a retry.)
		fmt.Println("warn: set invoice status:", err)
	}
	// 6. Publish event.
	if s.events != nil {
		payload := map[string]any{
			"payment_id":     p.ID,
			"invoice_id":     p.InvoiceID,
			"transaction_id": txID,
			"amount":         p.Amount.IntPart(),
			"scenario":       status.TransactionStatus,
		}
		var perr error
		if newStatus == "PAID" {
			perr = s.events.PublishPaymentSucceeded(ctx, payload)
		} else if newStatus == "FAILED" {
			perr = s.events.PublishPaymentFailed(ctx, payload)
		}
		if perr != nil {
			fmt.Println("warn: publish payment event:", perr)
		}
	}
	return nil
}

// mapStatus translates a Midtrans transaction_status to:
//   - our local Payment.status (PENDING|PAID|FAILED|CANCELLED|EXPIRED)
//   - the invoice status to set ("PAID" or "PENDING")
func (s *Service) mapStatus(txStatus string) (string, string) {
	switch txStatus {
	case "capture", "settlement":
		return "PAID", "PAID"
	case "pending":
		return "PENDING", "PENDING"
	case "cancel", "deny":
		return "CANCELLED", "PENDING"
	case "expire":
		return "EXPIRED", "PENDING"
	case "refund":
		return "FAILED", "PENDING"
	default:
		return "FAILED", "PENDING"
	}
}

// Get returns one payment.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Payment, error) {
	return s.repo.FindByID(ctx, id)
}

// RefreshPayment fetches the latest transaction status from Midtrans and
// updates the local payment + invoice status. This is the client-side
// equivalent of the Midtrans webhook (useful for local dev where the
// webhook cannot reach localhost).
func (s *Service) RefreshPayment(ctx context.Context, paymentID uuid.UUID) (*Payment, error) {
	p, err := s.repo.FindByID(ctx, paymentID)
	if err != nil {
		return nil, err
	}
	// If no midtrans_order_id (e.g. seed data), nothing to refresh.
	if p.MidtransOrderID == "" {
		return p, nil
	}

	// Ask Midtrans for the current transaction status.
	status, err := s.mid.GetStatus(ctx, p.MidtransOrderID)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, "midtrans get status", err)
	}

	newStatus, invStatus := s.mapStatus(status.TransactionStatus)
	method := status.PaymentType
	txID := status.TransactionID
	txStatus := status.TransactionStatus

	// Update the local payment row.
	if err := s.repo.UpdateStatus(ctx, p.ID, newStatus, &method, &txID, &txStatus, status); err != nil {
		return nil, err
	}
	// Update the invoice status via the violation-service (best-effort).
	if err := s.inv.SetStatus(ctx, p.InvoiceID, invStatus); err != nil {
		fmt.Println("warn: refresh set invoice status:", err)
	}

	// Return the updated payment.
	return s.repo.FindByID(ctx, paymentID)
}
