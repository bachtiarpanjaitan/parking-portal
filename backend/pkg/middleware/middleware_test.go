package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/parking-portal/backend/pkg/auth"
	"github.com/parking-portal/backend/pkg/errs"
)

func newTestEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID(), Recovery(zap.NewNop()), CORS(), Logger(zap.NewNop()))
	return r
}

func TestRequestID_GeneratesWhenMissing(t *testing.T) {
	r := newTestEngine()
	r.GET("/x", func(c *gin.Context) { c.Status(200) })
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/x", nil)
	r.ServeHTTP(w, req)
	if w.Header().Get("X-Request-ID") == "" {
		t.Errorf("X-Request-ID header missing")
	}
}

func TestRequestID_PassesThrough(t *testing.T) {
	r := newTestEngine()
	r.GET("/x", func(c *gin.Context) { c.Status(200) })
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/x", nil)
	req.Header.Set("X-Request-ID", "abc-123")
	r.ServeHTTP(w, req)
	if got := w.Header().Get("X-Request-ID"); got != "abc-123" {
		t.Errorf("X-Request-ID = %q, want abc-123", got)
	}
}

func TestRecovery_ConvertsAppErrorPanic(t *testing.T) {
	r := newTestEngine()
	r.GET("/x", func(c *gin.Context) { panic(errs.New(errs.CodeForbidden, "nope")) })
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/x", nil)
	r.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Errorf("status = %d, want 403", w.Code)
	}
	if !strings.Contains(w.Body.String(), "FORBIDDEN") {
		t.Errorf("body missing FORBIDDEN: %s", w.Body.String())
	}
}

func TestRequireRole(t *testing.T) {
	s, err := auth.NewSigner("supersecretkey-must-be-long-enough-32", 1, "test")
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	uid := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	tok, err := s.Sign(uid, auth.RoleOfficer)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID(), Recovery(zap.NewNop()))
	r.GET("/admin", Auth(s), RequireRole(auth.RoleOfficer), func(c *gin.Context) {
		c.Status(200)
	})

	// no token
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/admin", nil)
	r.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Errorf("no token: status = %d, want 401", w.Code)
	}

	// with token
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/admin", nil)
	req2.Header.Set("Authorization", "Bearer "+tok)
	r.ServeHTTP(w2, req2)
	if w2.Code != 200 {
		t.Errorf("with token: status = %d, want 200", w2.Code)
	}
}
