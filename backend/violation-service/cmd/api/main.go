// Package main is the entrypoint for the violation service.
// It wires: config → logger → postgres pool → gin engine → middlewares →
// register all modules → graceful shutdown.
//
// See .ai/CODE_TEMPLATES.md for the canonical pattern.
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

	"github.com/parking-portal/backend/pkg/auth"
	"github.com/parking-portal/backend/pkg/middleware"
	vsauth "github.com/parking-portal/backend/violation-service/internal/auth"
	"github.com/parking-portal/backend/violation-service/internal/config"
	"github.com/parking-portal/backend/violation-service/internal/database"
	"github.com/parking-portal/backend/violation-service/internal/events"
	"github.com/parking-portal/backend/violation-service/internal/history"
	"github.com/parking-portal/backend/violation-service/internal/invoices"
	"github.com/parking-portal/backend/violation-service/internal/rules"
	"github.com/parking-portal/backend/violation-service/internal/uploads"
	"github.com/parking-portal/backend/violation-service/internal/users"
	"github.com/parking-portal/backend/violation-service/internal/violations"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	zl, err := loggerBuild(cfg.AppEnv)
	if err != nil {
		log.Fatalf("logger: %v", err)
	}
	defer zl.Sync()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Postgres pool
	db, err := database.NewPostgres(ctx, cfg, zl)
	if err != nil {
		zl.Fatal("postgres", zap.Error(err))
	}
	defer db.Close()

	// Auth signer
	signer, err := auth.NewSigner(cfg.JWTSecret, cfg.JWTExpirationHrs, cfg.AppName)
	if err != nil {
		zl.Fatal("auth signer", zap.Error(err))
	}

	// RabbitMQ publisher (optional; nil if URL empty → events are dropped)
	var pub *events.Publisher
	if cfg.RabbitMQURL != "" {
		p, err := events.NewPublisher(cfg.RabbitMQURL, cfg.RabbitMQExchange, zl)
		if err != nil {
			zl.Warn("rabbitmq unavailable, events will be dropped", zap.Error(err))
			pub = nil
		} else {
			pub = p
			defer pub.Close()
		}
	} else {
		zl.Warn("RABBITMQ_URL empty, events will be dropped")
	}

	// Repos
	usersRepo := users.NewPGRepository(db)
	rulesRepo := rules.NewPGRepository(db)
	invoicesRepo := invoices.NewPGRepository(db)
	violationsRepo := violations.NewPGRepository(db)
	historyRepo := history.NewPGRepository(db)

	// Services
	authSvc := vsauth.NewService(usersRepo, signer)
	rulesSvc := rules.NewService(rulesRepo)
	invoicesSvc := invoices.NewService(invoicesRepo)
	violationsSvc := violations.NewService(violationsRepo, rulesSvc, usersRepo, invoicesSvc, pub)
	uploadsSvc := uploads.NewService(cfg.StoragePath, cfg.PublicUploadURL, cfg.MaxUploadSizeMB)
	historySvc := history.NewService(historyRepo)

	// Handlers
	authH := vsauth.NewHandler(authSvc, zl)
	usersH := users.NewHandler(usersRepo, zl)
	rulesH := rules.NewHandler(rulesSvc, zl)
	invoicesH := invoices.NewHandler(invoicesSvc, zl)
	violationsH := violations.NewHandler(violationsSvc, zl)
	uploadsH := uploads.NewHandler(uploadsSvc, zl)
	historyH := history.NewHandler(historySvc, zl)

	// Gin engine + middlewares
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(
		middleware.RequestID(),
		middleware.Recovery(zl),
		middleware.CORS(),
		middleware.Logger(zl),
		// Global max multipart memory: covers the body limit enforced by uploads.
		// The per-file size check in uploads.Service is the real guard.
	)

	// Serve uploaded photos as static files.
	// Example: GET /uploads/violations/<uuid>.jpg → ./storage/violations/<uuid>.jpg
	r.Static(cfg.PublicUploadURL, cfg.StoragePath)

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "violation-service"})
	})

	// Public auth endpoint
	authH.Register(r)

	// All other routes are JWT-protected.
	authMW := middleware.Auth(signer)
	v1 := r.Group("/api/v1")
	v1.Use(authMW)

	usersH.Register(v1)
	rulesH.Register(v1)
	invoicesH.Register(v1)
	violationsH.Register(v1)
	uploadsH.Register(v1)
	historyH.Register(v1)

	// HTTP server with graceful shutdown
	srv := &http.Server{Addr: ":" + cfg.AppPort, Handler: r}
	go func() {
		zl.Info("violation-service listening", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			zl.Fatal("listen", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	zl.Info("shutting down...")
	shutCtx, c2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer c2()
	_ = srv.Shutdown(shutCtx)
	fmt.Println("bye")
}

// loggerBuild is a tiny shim so this file doesn't import logger.
func loggerBuild(env string) (*zap.Logger, error) {
	return loggerNew(env)
}
