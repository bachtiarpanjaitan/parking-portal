package history

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

// Handler exposes the history HTTP endpoint.
type Handler struct {
	svc *Service
	log *zap.Logger
}

func NewHandler(svc *Service, log *zap.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// Register attaches the routes. MEMBER can only see their own.
func (h *Handler) Register(g *gin.RouterGroup) {
	g.GET("/history", h.List)
}

func (h *Handler) List(c *gin.Context) {
	f := Filter{
		Page:     atoiD(c.Query("page"), 1),
		PageSize: atoiD(c.Query("page_size"), 20),
	}

	// Role-based scoping: MEMBER → own only.
	role := middleware.Role(c)
	switch role {
	case "MEMBER":
		uid := middleware.UserID(c)
		f.MemberID = &uid
	case "OFFICER":
		// Officer may pass member_id to filter to one member; default is all.
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

	if v := c.Query("from"); v != "" {
		f.From = &v
	}
	if v := c.Query("to"); v != "" {
		f.To = &v
	}

	items, total, err := h.svc.List(c.Request.Context(), f)
	if err != nil {
		panic(err)
	}
	c.JSON(http.StatusOK, httpx.Paginated(items, f.Page, f.PageSize, total))
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
