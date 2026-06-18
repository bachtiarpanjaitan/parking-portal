package rules

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

// Handler exposes the rules HTTP endpoints.
type Handler struct {
	svc *Service
	log *zap.Logger
}

func NewHandler(svc *Service, log *zap.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// Register attaches the routes. Only OFFICER can write; everyone with a JWT
// can read. (See MODULES.md → role matrix.)
func (h *Handler) Register(g *gin.RouterGroup) {
	g.GET("/rules", h.List)
	g.GET("/rules/active", h.GetActive)
	g.GET("/rules/:id", h.Get)
	g.POST("/rules", h.Create)
	// PUT replaces the 4 details of a draft version. The PATCH /rules/:id/publish
	// endpoint activates a draft (see Publish below).
	g.PUT("/rules/:id", h.Update)
	g.DELETE("/rules/:id", h.Delete)
	g.POST("/rules/:id/publish", h.Publish)
}

func (h *Handler) mustOfficer(c *gin.Context) {
	if middleware.Role(c) != "OFFICER" {
		panic(errs.New(errs.CodeForbidden, "only officer can access rules"))
	}
}

// List returns paginated rule versions, optionally filtered by is_active.
// Query params:
//   - page (default 1), page_size (default 20)
//   - status: "active" | "draft" | "all" (default "all")
func (h *Handler) List(c *gin.Context) {
	h.mustOfficer(c)
	f := Filter{
		Page:     atoiD(c.Query("page"), 1),
		PageSize: atoiD(c.Query("page_size"), 20),
	}
	switch c.Query("status") {
	case "active":
		v := true
		f.IsActive = &v
	case "draft":
		v := false
		f.IsActive = &v
	}
	vs, total, err := h.svc.List(c.Request.Context(), f)
	if err != nil {
		panic(err)
	}
	c.JSON(http.StatusOK, httpx.Paginated(vs, f.Page, f.PageSize, total))
}

func (h *Handler) GetActive(c *gin.Context) {
	h.mustOfficer(c)
	v, err := h.svc.GetActive(c.Request.Context())
	if err != nil {
		panic(err)
	}
	c.JSON(http.StatusOK, httpx.OK(v))
}

func (h *Handler) Get(c *gin.Context) {
	h.mustOfficer(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		panic(errs.New(errs.CodeValidation, "invalid id"))
	}
	v, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		panic(err)
	}
	c.JSON(http.StatusOK, httpx.OK(v))
}

func (h *Handler) Create(c *gin.Context) {
	h.mustOfficer(c)
	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		panic(errs.New(errs.CodeValidation, "invalid request body"))
	}
	createdBy := middleware.UserID(c)
	v, err := h.svc.CreateDraft(c.Request.Context(), createdBy, req)
	if err != nil {
		panic(err)
	}
	c.JSON(http.StatusCreated, httpx.OKWithMessage(v, "Rule draft created; publish to activate"))
}

// Update replaces the 4 details of a draft rule version. Refuses to
// modify an active version.
func (h *Handler) Update(c *gin.Context) {
	h.mustOfficer(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		panic(errs.New(errs.CodeValidation, "invalid id"))
	}
	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		panic(errs.New(errs.CodeValidation, "invalid request body"))
	}
	v, err := h.svc.UpdateDraft(c.Request.Context(), id, req)
	if err != nil {
		panic(err)
	}
	c.JSON(http.StatusOK, httpx.OKWithMessage(v, "Rule draft updated"))
}

// Delete removes a draft rule version. Active versions cannot be deleted.
func (h *Handler) Delete(c *gin.Context) {
	h.mustOfficer(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		panic(errs.New(errs.CodeValidation, "invalid id"))
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		panic(err)
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) Publish(c *gin.Context) {
	h.mustOfficer(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		panic(errs.New(errs.CodeValidation, "invalid id"))
	}
	if err := h.svc.Publish(c.Request.Context(), id); err != nil {
		panic(err)
	}
	c.JSON(http.StatusOK, httpx.OKWithMessage(map[string]any{"id": id}, "Rule published"))
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
