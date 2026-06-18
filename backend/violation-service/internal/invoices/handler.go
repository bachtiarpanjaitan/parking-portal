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
}

func (h *Handler) List(c *gin.Context) {
	f := Filter{
		Page:     atoiD(c.Query("page"), 1),
		PageSize: atoiD(c.Query("page_size"), 20),
		Status:   c.Query("status"),
	}
	role := middleware.Role(c)
	if role == "MEMBER" {
		uid := middleware.UserID(c)
		f.MemberID = &uid
	} else if role != "OFFICER" {
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
