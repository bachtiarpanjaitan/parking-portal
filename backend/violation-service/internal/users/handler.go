package users

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/parking-portal/backend/pkg/errs"
	"github.com/parking-portal/backend/pkg/httpx"
	"github.com/parking-portal/backend/pkg/middleware"
)

// Handler exposes the users HTTP endpoints.
type Handler struct {
	repo Repository
	log  *zap.Logger
}

func NewHandler(repo Repository, log *zap.Logger) *Handler {
	return &Handler{repo: repo, log: log}
}

// Register attaches the routes to the given group. Auth is required.
// Use only OFFICER can call these (members cannot list other members).
func (h *Handler) Register(g *gin.RouterGroup) {
	g.GET("/members", h.ListMembers)
	g.GET("/members/:id", h.GetMember)
}

func (h *Handler) ListMembers(c *gin.Context) {
	if middleware.Role(c) != "OFFICER" {
		panic(errs.New(errs.CodeForbidden, "only officer can list members"))
	}
	q := c.Query("q")
	limit, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit
	items, total, err := h.repo.List(c.Request.Context(), q, limit, offset)
	if err != nil {
		panic(err)
	}
	c.JSON(http.StatusOK, httpx.Paginated(items, page, limit, total))
}

func (h *Handler) GetMember(c *gin.Context) {
	id, err := uuidFromParam(c, "id")
	if err != nil {
		panic(errs.New(errs.CodeValidation, "invalid id"))
	}
	u, err := h.repo.FindByID(c.Request.Context(), id)
	if err != nil {
		panic(err)
	}
	c.JSON(http.StatusOK, httpx.OK(u))
}
