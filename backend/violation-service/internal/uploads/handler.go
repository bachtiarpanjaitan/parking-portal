package uploads

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/parking-portal/backend/pkg/errs"
	"github.com/parking-portal/backend/pkg/httpx"
	"github.com/parking-portal/backend/pkg/middleware"
)

// Handler exposes the uploads HTTP endpoints.
type Handler struct {
	svc *Service
	log *zap.Logger
}

func NewHandler(svc *Service, log *zap.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// Register attaches the routes. OFFICER only.
func (h *Handler) Register(g *gin.RouterGroup) {
	g.POST("/uploads/violations", h.Upload)
}

func (h *Handler) Upload(c *gin.Context) {
	if middleware.Role(c) != "OFFICER" {
		panic(errs.New(errs.CodeForbidden, "only officer can upload photos"))
	}
	// MaxBytesReader is set globally by the middleware; here we just parse.
	file, err := c.FormFile("file")
	if err != nil {
		panic(errs.New(errs.CodeFileRequired, "file is required (multipart field 'file')"))
	}
	res, err := h.svc.Save(file)
	if err != nil {
		// Translate known error markers into AppErrors
		msg := err.Error()
		switch {
		case startsWith(msg, "FILE_REQUIRED"):
			panic(errs.New(errs.CodeFileRequired, "file is required"))
		case startsWith(msg, "FILE_TOO_LARGE"):
			panic(errs.New(errs.CodeFileTooLarge, "file exceeds maximum allowed size"))
		case startsWith(msg, "INVALID_FILE_TYPE"):
			panic(errs.New(errs.CodeInvalidFileType, "unsupported file type (allowed: jpg, jpeg, png, webp)"))
		default:
			panic(errs.Wrap(errs.CodeFileUploadFailed, "save upload", err))
		}
	}
	c.JSON(http.StatusCreated, httpx.OK(res))
}

func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
