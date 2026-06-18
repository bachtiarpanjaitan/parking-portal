package violations

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/parking-portal/backend/pkg/errs"
	"github.com/parking-portal/backend/pkg/httpx"
	"github.com/parking-portal/backend/pkg/middleware"
)

// Handler exposes the violations HTTP endpoints.
type Handler struct {
	svc *Service
	log *zap.Logger
}

func NewHandler(svc *Service, log *zap.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// Register attaches the routes.
func (h *Handler) Register(g *gin.RouterGroup) {
	g.POST("/violations", h.Create)
	g.GET("/violations", h.List)
	g.GET("/violations/:id", h.Get)
}

func (h *Handler) Create(c *gin.Context) {
	if middleware.Role(c) != "OFFICER" {
		panic(errs.New(errs.CodeForbidden, "only officer can create violations"))
	}
	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		panic(errs.New(errs.CodeValidation, "invalid request body"))
	}
	if req.LicensePlate == "" {
		panic(errs.New(errs.CodeLicensePlateRequired, "license_plate is required"))
	}
	if req.PhotoURL == "" {
		panic(errs.New(errs.CodePhotoRequired, "photo_url is required"))
	}
	res, err := h.svc.Create(c.Request.Context(), req)
	if err != nil {
		panic(err)
	}
	c.JSON(http.StatusCreated, httpx.OK(res))
}

func (h *Handler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		panic(errs.New(errs.CodeValidation, "invalid id"))
	}
	v, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		panic(err)
	}
	// MEMBER can only see their own
	if middleware.Role(c) == "MEMBER" {
		if v.MemberID != middleware.UserID(c) {
			panic(errs.New(errs.CodeForbidden, "cannot access another member's violation"))
		}
	}
	c.JSON(http.StatusOK, httpx.OK(v))
}

func (h *Handler) List(c *gin.Context) {
	f := Filter{
		LicensePlate:  c.Query("license_plate"),
		ViolationType: c.Query("violation_type"),
		Location:      c.Query("location"),
		Page:          atoiDefault(c.Query("page"), 1),
		PageSize:      atoiDefault(c.Query("page_size"), 20),
		Sort:          c.DefaultQuery("sort", "violation_timestamp"),
		Order:         c.DefaultQuery("order", "desc"),
	}

	// Date range: parse both params once. We tolerate malformed values
	// silently (ignore them) so a typo in the UI doesn't 500 the page;
	// the underlying repository will still return all rows when f.From
	// and f.To stay nil.
	if v := c.Query("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.From = &t
		}
	}
	if v := c.Query("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.To = &t
		}
	}

	// Role-based scoping: MEMBER → own only. OFFICER may pass member_id
	// to drill into one member's violations, or omit it to see everyone's.
	role := middleware.Role(c)
	switch role {
	case "MEMBER":
		uid := middleware.UserID(c)
		f.MemberID = &uid
	case "OFFICER":
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

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}
