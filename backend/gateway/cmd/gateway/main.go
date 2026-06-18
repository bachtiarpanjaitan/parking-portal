// Package main is the entrypoint for the API gateway.
//
// Routes (per ADR-009):
//
//	GET  /healthz                                                -> gateway
//	POST /api/v1/auth/login                                      -> violation
//	ANY  /api/v1/uploads/*path                                    -> violation
//	ANY  /api/v1/violations                                       -> violation
//	ANY  /api/v1/violations/*path                                 -> violation
//	ANY  /api/v1/invoices                                         -> violation
//	ANY  /api/v1/invoices/*path                                   -> violation
//	ANY  /api/v1/rules                                            -> violation
//	ANY  /api/v1/rules/*path                                      -> violation
//	ANY  /api/v1/members                                          -> violation
//	ANY  /api/v1/members/*path                                    -> violation
//	ANY  /api/v1/history                                          -> violation
//	ANY  /api/v1/history/*path                                    -> violation
//	ANY  /api/v1/payments                                         -> payment
//	ANY  /api/v1/payments/*path                                   -> payment
//	GET  /uploads/*path                                          -> violation
//
// Gin's `*path` catch-all only matches when there's content after the
// prefix, so we register an explicit route for every "list" endpoint
// (no trailing slash) plus the catch-all for "detail" endpoints.
//
// All routes (except /healthz and /api/v1/auth/login) require a valid
// JWT. The gateway validates the token and forwards the request to the
// downstream service. The downstream service re-validates (defense in
// depth).
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/parking-portal/backend/gateway/internal/config"
	"github.com/parking-portal/backend/gateway/internal/proxy"
	"github.com/parking-portal/backend/pkg/auth"
	"github.com/parking-portal/backend/pkg/middleware"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	zl, _ := zap.NewDevelopment()
	defer zl.Sync()

	signer, err := auth.NewSigner(cfg.JWTSecret, cfg.JWTExpirationHrs, cfg.AppName)
	if err != nil {
		zl.Fatal("auth signer", zap.Error(err))
	}

	timeout := time.Duration(cfg.UpstreamTimeoutSeconds) * time.Second
	violationProxy, err := proxy.New(cfg.ViolationServiceURL, timeout)
	if err != nil {
		zl.Fatal("violation proxy", zap.Error(err))
	}
	paymentProxy, err := proxy.New(cfg.PaymentServiceURL, timeout)
	if err != nil {
		zl.Fatal("payment proxy", zap.Error(err))
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(
		middleware.RequestID(),
		middleware.Recovery(zl),
		middleware.CORS(),
		middleware.Logger(zl),
	)

	// Gateway-owned health endpoint
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "api-gateway",
			"upstream": gin.H{
				"violation": cfg.ViolationServiceURL,
				"payment":   cfg.PaymentServiceURL,
			},
		})
	})

	// Public auth (login is on violation-service)
	v1Public := r.Group("/api/v1")
	v1Public.POST("/auth/login", violationProxy)

	// All other /api/v1 routes require JWT.
	authMW := middleware.Auth(signer)
	v1 := r.Group("/api/v1")
	v1.Use(authMW)

	// Helper: register both "list" (no trailing slash) and "detail"
	// (with trailing content) routes for one upstream.
	bind := func(prefix string, p gin.HandlerFunc) {
		v1.Any(prefix, p)          // list endpoint (e.g. /api/v1/invoices)
		v1.Any(prefix+"/*path", p) // detail endpoint (e.g. /api/v1/invoices/123)
	}

	// Violation-service routes
	bind("/uploads", violationProxy)
	bind("/violations", violationProxy)
	bind("/invoices", violationProxy)
	bind("/rules", violationProxy)
	bind("/members", violationProxy)
	bind("/history", violationProxy)
	// Payment-service routes
	bind("/payments", paymentProxy)

	// Static uploads: proxy /uploads/* to violation-service
	r.GET("/uploads/*path", violationProxy)

	// HTTP server with graceful shutdown
	srv := &http.Server{
		Addr:         ":" + cfg.AppPort,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: timeout,
	}
	go func() {
		zl.Info("api-gateway listening", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			zl.Fatal("listen", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	zl.Info("shutdown signal received")
	ctx, c2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer c2()
	if err := srv.Shutdown(ctx); err != nil {
		zl.Error("graceful shutdown", zap.Error(err))
	}
	fmt.Println("bye")
}
