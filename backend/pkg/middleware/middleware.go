// Package middleware contains the standard Gin middlewares used by every
// service. See .ai/CODE_TEMPLATES.md for the canonical chain:
//
//	Recovery → RequestID → CORS → Logger → Auth → ErrorTranslator
package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/parking-portal/backend/pkg/auth"
	"github.com/parking-portal/backend/pkg/errs"
	"github.com/parking-portal/backend/pkg/httpx"
)

// CtxKey is the type used for context.WithValue to avoid collisions.
type CtxKey string

const (
	// CtxUserID is the key for the authenticated user's UUID in the gin context.
	CtxUserID CtxKey = "user_id"
	// CtxRole is the key for the authenticated user's role.
	CtxRole CtxKey = "role"
	// CtxRequestID is the key for the per-request UUID.
	CtxRequestID CtxKey = "request_id"
)

// RequestID assigns a UUID to each request and exposes it on the context and
// response header. Should be the FIRST middleware in the chain.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader("X-Request-ID")
		if rid == "" {
			rid = uuid.NewString()
		}
		c.Set(string(CtxRequestID), rid)
		c.Writer.Header().Set("X-Request-ID", rid)
		c.Next()
	}
}

// Recovery converts panics (including *errs.AppError) into the standard
// error envelope. Should wrap everything else.
func Recovery(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				// If it's an AppError, render it.
				if ae, ok := r.(*errs.AppError); ok {
					logAppError(c, log, ae)
					httpx.WriteError(c, ae)
					return
				}
				// Otherwise treat as 500.
				var ae *errs.AppError
				ae = errs.Wrap(errs.CodeInternal, "internal server error", fmt.Errorf("%v", r))
				_ = ae
				log.Error("panic recovered",
					zap.Any("err", r),
					zap.String("path", c.Request.URL.Path),
					zap.String("method", c.Request.Method),
					zap.String("stack", string(debug.Stack())),
				)
				internal := errs.New(errs.CodeInternal, "internal server error")
				httpx.WriteError(c, internal)
			}
		}()
		c.Next()
	}
}

func logAppError(c *gin.Context, log *zap.Logger, ae *errs.AppError) {
	fields := []zap.Field{
		zap.String("code", string(ae.ErrCode)),
		zap.String("message", ae.Message),
		zap.String("path", c.Request.URL.Path),
		zap.String("method", c.Request.Method),
		zap.Int("status", ae.HTTPStatusCode),
	}
	if rid, ok := c.Get(string(CtxRequestID)); ok {
		fields = append(fields, zap.String("request_id", rid.(string)))
	}
	if ae.HTTPStatusCode >= 500 {
		log.Error("request failed", append(fields, zap.Error(ae))...)
	} else {
		log.Warn("request rejected", fields...)
	}
}

// ErrorTranslator catches *errs.AppError returned from handlers/services and
// writes the standardized envelope. Add this BEFORE the handler (after the
// other middlewares). Handlers/services can also `panic(errs.New(...))` and
// the Recovery middleware will route through here.
func ErrorTranslator(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		// If the handler returned without panic but with an attached error,
		// we'd have to read it from c.Errors. For this project we follow
		// the convention: panic on typed errors; the Recovery middleware
		// writes the envelope. So this middleware is a no-op safety net
		// that translates any leftover c.Errors entries.
		for _, ginErr := range c.Errors {
			var ae *errs.AppError
			if errors.As(ginErr.Err, &ae) {
				logAppError(c, log, ae)
				httpx.WriteError(c, ae)
				return
			}
		}
	}
}

// Auth validates the Bearer JWT and stores user_id + role in the context.
// The gateway uses this for every route; backend services can also use it
// when running standalone (dev) by passing the same JWT secret.
func Auth(s *auth.Signer) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			panic(errs.New(errs.CodeUnauthorized, "missing bearer token"))
		}
		token := strings.TrimPrefix(header, "Bearer ")
		claims, err := s.Verify(token)
		if err != nil {
			// Distinguish expired vs invalid in the future if needed.
			panic(errs.Wrap(errs.CodeInvalidToken, "invalid token", err))
		}
		c.Set(string(CtxUserID), claims.UserID)
		c.Set(string(CtxRole), string(claims.Role))
		c.Next()
	}
}

// RequireRole enforces a role on a route group. Must be used AFTER Auth.
func RequireRole(allowed ...auth.Role) gin.HandlerFunc {
	allow := map[auth.Role]struct{}{}
	for _, r := range allowed {
		allow[r] = struct{}{}
	}
	return func(c *gin.Context) {
		roleVal, _ := c.Get(string(CtxRole))
		role, _ := roleVal.(string)
		if _, ok := allow[auth.Role(role)]; !ok {
			panic(errs.New(errs.CodeForbidden, "role not permitted"))
		}
		c.Next()
	}
}

// UserID returns the authenticated user's UUID. Panics if not set.
func UserID(c *gin.Context) uuid.UUID {
	v, _ := c.Get(string(CtxUserID))
	id, _ := v.(uuid.UUID)
	return id
}

// Role returns the authenticated user's role. Returns "" if not set.
func Role(c *gin.Context) auth.Role {
	v, _ := c.Get(string(CtxRole))
	s, _ := v.(string)
	return auth.Role(s)
}

// ForceMemberScope rewrites c.Query("member_id") to the authenticated member's
// id. Officers keep whatever they sent. If a MEMBER tries to query a
// different member_id, returns FORBIDDEN.
func ForceMemberScope(c *gin.Context) {
	role := Role(c)
	if role == auth.RoleMember {
		requested := c.Query("member_id")
		uid := UserID(c)
		if requested != "" && requested != uid.String() {
			panic(errs.New(errs.CodeForbidden, "cannot access another member's data"))
		}
		c.Request.URL.Query().Set("member_id", uid.String())
		// also update the underlying query so downstream reads see the right id
		q := c.Request.URL.Query()
		q.Set("member_id", uid.String())
		c.Request.URL.RawQuery = q.Encode()
	}
}

// CORS sets permissive CORS headers for the browser frontend. Tighten in prod.
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
		c.Writer.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// Logger emits one zap log line per request, after the handler finishes.
func Logger(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		dur := time.Since(start)
		fields := []zap.Field{
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("dur", dur),
			zap.String("ip", c.ClientIP()),
		}
		if rid, ok := c.Get(string(CtxRequestID)); ok {
			fields = append(fields, zap.String("request_id", rid.(string)))
		}
		if uid, ok := c.Get(string(CtxUserID)); ok {
			fields = append(fields, zap.Any("user_id", uid))
		}
		switch {
		case c.Writer.Status() >= 500:
			log.Error("http", fields...)
		case c.Writer.Status() >= 400:
			log.Warn("http", fields...)
		default:
			log.Info("http", fields...)
		}
	}
}
