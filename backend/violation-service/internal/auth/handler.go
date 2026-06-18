package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/parking-portal/backend/pkg/errs"
	"github.com/parking-portal/backend/pkg/httpx"
)

// Handler exposes the auth HTTP endpoints.
type Handler struct {
	svc *Service
	log *zap.Logger
}

func NewHandler(svc *Service, log *zap.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// Register attaches the routes. Login is public (no auth required).
func (h *Handler) Register(r *gin.Engine) {
	r.POST("/api/v1/auth/login", h.Login)
}

func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		panic(errs.New(errs.CodeValidation, "invalid request body"))
	}
	if req.Email == "" {
		panic(errs.New(errs.CodeValidation, "email is required"))
	}
	if req.Password == "" {
		panic(errs.New(errs.CodeValidation, "password is required"))
	}
	resp, err := h.svc.Login(req.Email, req.Password)
	if err != nil {
		panic(err)
	}
	c.JSON(http.StatusOK, httpx.OK(resp))
}
