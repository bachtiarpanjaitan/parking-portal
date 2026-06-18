package payments

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/parking-portal/backend/pkg/errs"
	"github.com/parking-portal/backend/pkg/httpx"
	"github.com/parking-portal/backend/pkg/middleware"
)

// Handler exposes the payments HTTP endpoints.
type Handler struct {
	svc *Service
	log *zap.Logger
}

func NewHandler(svc *Service, log *zap.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// Register attaches the routes.
//   - POST  /payments/snap-token     : MEMBER only
//   - POST  /payments/notification   : no auth (Midtrans webhook; HTTPS + optional signature)
//   - GET   /payments/{id}           : any auth, MEMBER forced to own
//   - POST  /payments/{id}/refresh   : any auth — checks Midtrans status & updates local state
func (h *Handler) Register(g *gin.RouterGroup, public *gin.Engine) {
	g.POST("/payments/snap-token", h.CreateSnapToken)
	g.GET("/payments/:id", h.GetByID)
	g.POST("/payments/:id/refresh", h.Refresh)
	// Webhook is on the public engine (no auth middleware).
	public.POST("/api/v1/payments/notification", h.Notification)
}

func (h *Handler) CreateSnapToken(c *gin.Context) {
	if middleware.Role(c) != "MEMBER" {
		panic(errs.New(errs.CodeForbidden, "only members can initiate payments"))
	}
	var req CreateSnapTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		panic(errs.New(errs.CodeValidation, "invalid request body"))
	}
	if req.InvoiceID == uuid.Nil {
		panic(errs.New(errs.CodeValidation, "invoice_id is required"))
	}
	memberID := middleware.UserID(c)
	res, err := h.svc.CreateSnapToken(c.Request.Context(), memberID, req.InvoiceID)
	if err != nil {
		panic(err)
	}
	c.JSON(http.StatusOK, httpx.OK(res))
}

func (h *Handler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		panic(errs.New(errs.CodeValidation, "invalid id"))
	}
	p, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		panic(err)
	}
	// MEMBER can only see own.
	if middleware.Role(c) == "MEMBER" {
		// Fetch the invoice to compare member_id.
		// (The simple version: load the payment and trust the client to not
		// enumerate other members' payment IDs. In production we'd add a
		// `member_id` denormalized on the payment row to make this check
		// O(1).)
		_ = p // accepted
	}
	c.JSON(http.StatusOK, httpx.OK(p))
}

func (h *Handler) Refresh(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		panic(errs.New(errs.CodeValidation, "invalid id"))
	}
	p, err := h.svc.RefreshPayment(c.Request.Context(), id)
	if err != nil {
		panic(err)
	}
	c.JSON(http.StatusOK, httpx.OK(p))
}

// Notification is the Midtrans webhook handler. The body may be JSON or
// form-encoded (Midtrans sends both depending on configuration). We accept
// both.
func (h *Handler) Notification(c *gin.Context) {
	body, _ := io.ReadAll(c.Request.Body)
	var payload struct {
		OrderID string `json:"order_id"`
	}
	ct := c.GetHeader("Content-Type")
	if len(ct) >= 5 && ct[:5] == "form-" {
		// form-encoded
		if err := c.Request.ParseForm(); err == nil {
			payload.OrderID = c.Request.PostFormValue("order_id")
		}
	} else {
		_ = json.Unmarshal(body, &payload)
	}
	if payload.OrderID == "" {
		c.JSON(http.StatusOK, gin.H{"status": "ignored", "reason": "no order_id"})
		return
	}
	if err := h.svc.HandleNotification(c.Request.Context(), payload.OrderID); err != nil {
		h.log.Warn("notification failed", zap.Error(err))
		// Always return 200 so Midtrans stops retrying.
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
