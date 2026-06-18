package rules

import (
	"net/http"

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
	// All routes are OFFICER-only (members cannot see rules).
	g.GET("/rules", h.List)
	g.GET("/rules/active", h.GetActive)
	g.GET("/rules/:id", h.Get)
	g.POST("/rules", h.Create)
	g.POST("/rules/:id/publish", h.Publish)
}

func (h *Handler) mustOfficer(c *gin.Context) {
	if middleware.Role(c) != "OFFICER" {
		panic(errs.New(errs.CodeForbidden, "only officer can access rules"))
	}
}

func (h *Handler) List(c *gin.Context) {
	h.mustOfficer(c)
	vs, err := h.svc.List(c.Request.Context())
	if err != nil {
		panic(err)
	}
	c.JSON(http.StatusOK, httpx.OK(vs))
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
