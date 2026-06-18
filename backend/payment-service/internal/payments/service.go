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
func (s *Service) CreateSnapToken(ctx context.Context, memberID uuid.UUID, invoiceID uuid.UUID) (*CreateSnapTokenResponse, error) {
	// 1. Fetch the invoice from the violation-service to validate it.
	inv, err := s.inv.GetInvoice(ctx, invoiceID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, errs.New(errs.CodeInvoiceNotFound, "invoice not found")
		}
		return nil, errs.Wrap(errs.CodeInternal, "fetch invoice", err)
	}
	// 2. Authorization: invoice must belong to the calling member.
	if inv.MemberID != memberID {
		return nil, errs.New(errs.CodeForbidden, "cannot pay another member's invoice")
	}
	// 3. Status: PENDING or FAILED is payable; PAID is not.
	switch inv.Status {
	case "PAID":
		return nil, errs.New(errs.CodeInvoiceAlreadyPaid, "invoice already paid")
	case "CANCELLED":
		return nil, errs.New(errs.CodeInvalidInvStatus, "invoice is cancelled")
	}
	// 4. Build order_id (deterministic from invoice_id, so a member retrying
	// the same invoice gets a new Snap token but the same order_id — this
	// is the Midtrans-recommended way to handle retries).
	orderID := "ORDER-" + invoiceID.String()

	// 5. Create the Snap token via Midtrans.
	snap, err := s.mid.CreateSnapToken(ctx, orderID, inv.AmountInt())
	if err != nil {
		return nil, errs.New(errs.CodeInternal, "midtrans snap: "+err.Error())
	}

	// 6. Persist the payment row in PENDING state.
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
	// Stash the snap response for debugging
	p.MidrawResponse = snap
	if err := s.repo.Insert(ctx, p); err != nil {
		return nil, err
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
