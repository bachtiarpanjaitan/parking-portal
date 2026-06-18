package invoices

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/parking-portal/backend/pkg/errs"
	"github.com/parking-portal/backend/pkg/httpx"
	"github.com/parking-portal/backend/pkg/middleware"
)

// Handler exposes the invoices HTTP endpoints.
type Handler struct {
	svc *Service
	log *zap.Logger
}

func NewHandler(svc *Service, log *zap.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// Register attaches the routes. MEMBER can only see their own.
func (h *Handler) Register(g *gin.RouterGroup) {
	g.GET("/invoices", h.List)
	g.GET("/invoices/:id", h.Get)
	// Internal: called by the payment service after Midtrans settles a payment.
	// Only OFFICER (or the service-to-service JWT minted by payment-service)
	// is allowed to mutate status — never MEMBER.
	g.PATCH("/invoices/:id/status", h.SetStatus)
}

func (h *Handler) List(c *gin.Context) {
	f := Filter{
		Page:          atoiD(c.Query("page"), 1),
		PageSize:      atoiD(c.Query("page_size"), 20),
		Status:        c.Query("status"),
		LicensePlate:  c.Query("license_plate"),
		ViolationType: c.Query("violation_type"),
		Location:      c.Query("location"),
	}
	if v := c.Query("from"); v != "" {
		f.From = &v
	}
	if v := c.Query("to"); v != "" {
		f.To = &v
	}

	role := middleware.Role(c)
	switch role {
	case "MEMBER":
		// MEMBER is always scoped to their own invoices. A `member_id`
		// query param sent by a member is ignored (we never trust the
		// client to widen their own scope).
		uid := middleware.UserID(c)
		f.MemberID = &uid
	case "OFFICER":
		// OFFICER may pass `member_id` to drill into one member's
		// invoices, or omit it to see everyone's.
		if mid := c.Query("member_id"); mid != "" {
			u, err := uuid.Parse(mid)
			if err != nil {
				panic(errs.New(errs.CodeValidation, "invalid member_id"))
			}
			f.MemberID = &u
		}
	default:
		panic(errs.New(errs.CodeForbidden, "unknown role"))
	}

	items, total, err := h.svc.List(c.Request.Context(), f)
	if err != nil {
		panic(err)
	}
	c.JSON(http.StatusOK, httpx.Paginated(items, f.Page, f.PageSize, total))
}

func (h *Handler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		panic(errs.New(errs.CodeValidation, "invalid id"))
	}
	inv, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		panic(err)
	}
	if middleware.Role(c) == "MEMBER" && inv.MemberID != middleware.UserID(c) {
		panic(errs.New(errs.CodeForbidden, "cannot access another member's invoice"))
	}
	c.JSON(http.StatusOK, httpx.OK(inv))
}

// SetStatusRequest is the body for PATCH /invoices/:id/status.
type SetStatusRequest struct {
	Status string `json:"status"`
}

// SetStatus is called by the payment service after a Midtrans webhook or
// client-side refresh settles a payment. Only OFFICER-role tokens may call
// this endpoint (the payment service mints an OFFICER service-to-service JWT
// in main.go). MEMBER tokens are rejected with FORBIDDEN.
func (h *Handler) SetStatus(c *gin.Context) {
	if middleware.Role(c) != "OFFICER" {
		panic(errs.New(errs.CodeForbidden, "only officers may mutate invoice status"))
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		panic(errs.New(errs.CodeValidation, "invalid id"))
	}
	var req SetStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		panic(errs.New(errs.CodeValidation, "invalid request body"))
	}
	switch req.Status {
	case "PENDING", "PAID", "FAILED", "CANCELLED":
		// ok
	default:
		panic(errs.New(errs.CodeValidation, "status must be one of PENDING|PAID|FAILED|CANCELLED"))
	}
	if err := h.svc.SetStatus(c.Request.Context(), id, req.Status); err != nil {
		panic(err)
	}
	inv, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		panic(err)
	}
	c.JSON(http.StatusOK, httpx.OK(inv))
}

func atoiD(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}
